package sparql

import (
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

var (
	extFuncMu sync.RWMutex
	extFuncs  = make(map[string]Function)
)

// RegisterFunction binds fn to iri. It returns ErrFunctionRegistered if the IRI
// is already bound and ErrInvalidFunctionIRI if the IRI is empty or relative.
func RegisterFunction(iri string, fn Function) error {
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

// LookupFunction returns the function bound to iri, if any.
func LookupFunction(iri string) (Function, bool) {
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
func evalExtensionFunc(e *FuncExpr, bindings map[string]rdflibgo.Term, prefixes map[string]string) (rdflibgo.Term, bool) {
	fn := e.Fn
	if fn == nil {
		if e.IRI == "" {
			return nil, false
		}
		var ok bool
		fn, ok = LookupFunction(e.IRI)
		if !ok {
			return nil, false
		}
	}
	args := make([]rdflibgo.Term, len(e.Args))
	for i, a := range e.Args {
		args[i] = evalExpr(a, bindings, prefixes)
	}
	result, err := fn(args)
	if err != nil {
		// SPARQL expression error → unbound. See Function's documentation.
		return nil, true
	}
	return result, true
}
