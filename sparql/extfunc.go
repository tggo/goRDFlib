package sparql

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Extension functions (SPARQL 1.1 §17.6) let callers add IRI-named functions to
// the query engine — the Go counterpart of rdflib's
// rdflib.plugins.sparql.operators.register_custom_function.
//
// A registered function is invoked when a query calls its IRI, either directly
// as <http://example.org/fn>(?x) or through a prefixed name that resolves to
// the same IRI:
//
//	sparql.MustRegisterFunction("http://example.org/fn#double", func(args []term.Term) (term.Term, error) {
//		if len(args) != 1 {
//			return nil, fmt.Errorf("double: want 1 argument, got %d", len(args))
//		}
//		lit, ok := args[0].(term.Literal)
//		if !ok {
//			return nil, fmt.Errorf("double: want a literal, got %T", args[0])
//		}
//		n, err := strconv.ParseFloat(lit.Lexical(), 64)
//		if err != nil {
//			return nil, fmt.Errorf("double: %w", err)
//		}
//		return term.NewLiteral(n * 2), nil
//	})
//
// The registry is global and safe for concurrent use. Registration is normally
// done once at init time; registering while queries are running is allowed but
// only affects queries evaluated afterwards.

// ErrFunctionRegistered is returned by RegisterFunction when the IRI already
// has a function bound to it. Use ReplaceFunction to override deliberately.
var ErrFunctionRegistered = errors.New("sparql: extension function already registered")

// ErrInvalidFunctionIRI is returned when the IRI is empty or not absolute.
var ErrInvalidFunctionIRI = errors.New("sparql: invalid extension function IRI")

// Function is the signature of a SPARQL extension function. Arguments arrive
// already evaluated, in the order written in the query; an argument that
// evaluated to an error or to an unbound variable is nil.
//
// Returning an error produces a SPARQL expression error: the call evaluates to
// unbound, which makes an enclosing FILTER reject the row and leaves an
// enclosing BIND without a value. That matches how built-in functions signal
// bad input; it is not a query-level failure.
//
// Implementations must be safe for concurrent use — the engine may call them
// from several goroutines for one query.
type Function func(args []rdflibgo.Term) (rdflibgo.Term, error)

// ContextFunction is a Function that also receives the context the query is
// evaluated with: the ctx passed to QueryContext or EvalQueryContext, and
// context.Background for Query and EvalQuery. Use it for a function that calls
// out (a store, a service, a nested query) and should carry the caller's
// tracing span or deadline, or stop when the query is cancelled.
//
// Errors mean the same as for Function. A function that gives up because ctx
// is done may return any error; the query then reports the cancellation.
type ContextFunction func(ctx context.Context, args []rdflibgo.Term) (rdflibgo.Term, error)

var (
	extFuncMu sync.RWMutex
	extFuncs  = make(map[string]ContextFunction)
)

// withoutContext adapts a Function to the registry, which holds ContextFunction.
func withoutContext(fn Function) ContextFunction {
	return func(_ context.Context, args []rdflibgo.Term) (rdflibgo.Term, error) { return fn(args) }
}

// RegisterFunction binds fn to iri. It returns ErrFunctionRegistered if the IRI
// is already bound and ErrInvalidFunctionIRI if the IRI is empty or relative.
func RegisterFunction(iri string, fn Function) error {
	if fn == nil {
		return RegisterContextFunction(iri, nil)
	}
	return RegisterContextFunction(iri, withoutContext(fn))
}

// RegisterContextFunction is RegisterFunction for a function that receives the
// query's context. Function and ContextFunction share one registry: an IRI is
// bound to at most one function of either kind.
func RegisterContextFunction(iri string, fn ContextFunction) error {
	if err := validateFunctionIRI(iri); err != nil {
		return err
	}
	if fn == nil {
		return fmt.Errorf("%w: %s has a nil function", ErrInvalidFunctionIRI, iri)
	}
	extFuncMu.Lock()
	defer extFuncMu.Unlock()
	if _, exists := extFuncs[iri]; exists {
		return fmt.Errorf("%w: %s (use ReplaceFunction to override)", ErrFunctionRegistered, iri)
	}
	extFuncs[iri] = fn
	return nil
}

// MustRegisterFunction is RegisterFunction, panicking on error. Intended for
// package-level init, where a duplicate registration is a programming bug.
func MustRegisterFunction(iri string, fn Function) {
	if err := RegisterFunction(iri, fn); err != nil {
		panic(err)
	}
}

// ReplaceFunction binds fn to iri whether or not the IRI is already bound. It
// reports whether an existing binding was overwritten.
func ReplaceFunction(iri string, fn Function) (replaced bool, err error) {
	if fn == nil {
		return ReplaceContextFunction(iri, nil)
	}
	return ReplaceContextFunction(iri, withoutContext(fn))
}

// MustRegisterContextFunction is RegisterContextFunction, panicking on error.
func MustRegisterContextFunction(iri string, fn ContextFunction) {
	if err := RegisterContextFunction(iri, fn); err != nil {
		panic(err)
	}
}

// ReplaceContextFunction is ReplaceFunction for a function that receives the
// query's context.
func ReplaceContextFunction(iri string, fn ContextFunction) (replaced bool, err error) {
	if err := validateFunctionIRI(iri); err != nil {
		return false, err
	}
	if fn == nil {
		return false, fmt.Errorf("%w: %s has a nil function", ErrInvalidFunctionIRI, iri)
	}
	extFuncMu.Lock()
	defer extFuncMu.Unlock()
	_, replaced = extFuncs[iri]
	extFuncs[iri] = fn
	return replaced, nil
}

// UnregisterFunction removes the binding for iri. It reports whether one
// existed.
func UnregisterFunction(iri string) bool {
	extFuncMu.Lock()
	defer extFuncMu.Unlock()
	_, existed := extFuncs[iri]
	delete(extFuncs, iri)
	return existed
}

// LookupFunction returns the function bound to iri, if any. A function
// registered with a context is returned calling it with context.Background.
func LookupFunction(iri string) (Function, bool) {
	fn, ok := LookupContextFunction(iri)
	if !ok {
		return nil, false
	}
	return func(args []rdflibgo.Term) (rdflibgo.Term, error) { return fn(context.Background(), args) }, true
}

// LookupContextFunction returns the function bound to iri, if any, whichever
// way it was registered.
func LookupContextFunction(iri string) (ContextFunction, bool) {
	extFuncMu.RLock()
	defer extFuncMu.RUnlock()
	fn, ok := extFuncs[iri]
	return fn, ok
}

// RegisteredFunctions returns the registered IRIs in sorted order.
func RegisteredFunctions() []string {
	extFuncMu.RLock()
	iris := make([]string, 0, len(extFuncs))
	for iri := range extFuncs {
		iris = append(iris, iri)
	}
	extFuncMu.RUnlock()
	sort.Strings(iris)
	return iris
}

func validateFunctionIRI(iri string) error {
	if iri == "" {
		return fmt.Errorf("%w: IRI is empty", ErrInvalidFunctionIRI)
	}
	// Absolute IRIs only: a relative reference cannot be written as a call in a
	// query, so accepting one would silently register something unreachable.
	scheme := strings.IndexByte(iri, ':')
	if scheme <= 0 {
		return fmt.Errorf("%w: %q is not an absolute IRI", ErrInvalidFunctionIRI, iri)
	}
	if strings.ContainsAny(iri, " \t\r\n<>\"{}|^`") {
		return fmt.Errorf("%w: %q contains characters not allowed in an IRI", ErrInvalidFunctionIRI, iri)
	}
	return nil
}

// evalExtensionFunc dispatches a call to an extension function. The second
// return value reports whether a function was found at all; when it is false
// the caller falls through to the built-in function table.
//
// A function bound to this call site by ParsedQuery.BindFunctions wins over one
// registered globally for the same IRI, so a caller-supplied vocabulary never
// silently picks up a process-wide definition.
func evalExtensionFunc(ec *evalCtx, e *FuncExpr, bindings map[string]rdflibgo.Term, prefixes map[string]string, g *rdflibgo.Graph, namedGraphs map[string]*rdflibgo.Graph) (rdflibgo.Term, bool) {
	fn := e.FnCtx
	if fn == nil && e.Fn == nil {
		if e.IRI == "" {
			return nil, false
		}
		var ok bool
		fn, ok = LookupContextFunction(e.IRI)
		if !ok {
			return nil, false
		}
	}
	args := make([]rdflibgo.Term, len(e.Args))
	for i, a := range e.Args {
		args[i] = evalExprWithGraph(ec, a, bindings, prefixes, g, namedGraphs)
	}
	var result rdflibgo.Term
	var err error
	if fn != nil {
		result, err = fn(ec.context(), args)
	} else {
		result, err = e.Fn(args)
	}
	if err != nil {
		// SPARQL expression error → unbound. See Function's documentation.
		return nil, true
	}
	return result, true
}
