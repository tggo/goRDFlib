package shacl

import (
	"errors"
	"strconv"
	"strings"
)

// Custom node expression functions (SHACL 1.2 Node Expressions, "Custom Node
// Expressions"): a shapes graph declares a function as an instance of
// sh:NamedParameterExpressionFunction or sh:ListParameterExpressionFunction
// with an sh:bodyExpression, and an expression that calls it is evaluated by
// evaluating the body with the call's arguments as its scope (shnex:arg).

// customFunction is a function declared in the shapes graph.
type customFunction struct {
	iri  Term
	body Term // sh:bodyExpression; the zero Term when the declaration has none

	// optional[i] reports whether the list parameter with index i
	// (sh:path shnex:argI) is sh:optional.
	optional map[int]bool
}

// customFunctions are the functions a shapes graph declares.
type customFunctions struct {
	// named maps the IRI of a key parameter's sh:path to the function it
	// identifies.
	named map[string]*customFunction
	// namedParams lists, for each named function, the sh:path of all its
	// parameters.
	namedParams map[*customFunction][]Term
	// list maps the function IRI, which is the list parameter property of a
	// call, to the function.
	list map[string]*customFunction
}

// declaredFunctions reads the custom function declarations of g.
func declaredFunctions(g *Graph) *customFunctions {
	namedClass := IRI(SH + "NamedParameterExpressionFunction")
	listClass := IRI(SH + "ListParameterExpressionFunction")
	named := g.Subjects(IRI(RDFType), namedClass)
	lists := g.Subjects(IRI(RDFType), listClass)
	if len(named) == 0 && len(lists) == 0 {
		return nil
	}
	fns := &customFunctions{
		named:       make(map[string]*customFunction, len(named)),
		namedParams: make(map[*customFunction][]Term, len(named)),
		list:        make(map[string]*customFunction, len(lists)),
	}
	declare := func(iri Term) *customFunction {
		fn := &customFunction{iri: iri, optional: map[int]bool{}}
		for _, p := range g.Objects(iri, IRI(SH+"parameter")) {
			path := g.Objects(p, IRI(SH+"path"))
			if len(path) == 0 {
				continue
			}
			if idx, ok := strings.CutPrefix(path[0].Value(), SHNEX+"arg"); ok {
				if i, err := strconv.Atoi(idx); err == nil {
					opt := g.Objects(p, IRI(SH+"optional"))
					fn.optional[i] = len(opt) > 0 && opt[0].Value() == "true"
				}
			}
		}
		if b := g.Objects(iri, IRI(SH+"bodyExpression")); len(b) > 0 {
			fn.body = b[0]
		}
		return fn
	}
	for _, f := range lists {
		if f.IsIRI() {
			fns.list[f.Value()] = declare(f)
		}
	}
	for _, f := range named {
		fn := declare(f)
		for _, p := range g.Objects(f, IRI(SH+"parameter")) {
			path := g.Objects(p, IRI(SH+"path"))
			if len(path) == 0 || !path[0].IsIRI() {
				continue
			}
			fns.namedParams[fn] = append(fns.namedParams[fn], path[0])
			if isKey := g.Objects(p, IRI(SH+"keyParameter")); len(isKey) > 0 && isKey[0].Value() == "true" {
				fns.named[path[0].Value()] = fn
			}
		}
	}
	return fns
}

// ---------- custom named parameter expressions ----------

// CustomNamedExpr is a call of a custom named parameter function: a blank node
// with a value for one of its key parameters.
type CustomNamedExpr struct {
	Function Term
	Body     Term
	// Args maps each parameter the call gives a value to, by argKey of its
	// sh:path, to the argument expression. The arguments are not evaluated
	// by the call; the body's shnex:arg evaluates them.
	Args map[string]NodeExpr
}

func (e *CustomNamedExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *CustomNamedExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	body, err := customBody(ctx, e.Function, e.Body)
	if err != nil {
		return nil, err
	}
	inner := *ctx
	inner.guard = ctx.sharedGuard()
	inner.vars = nil
	inner.args = e.Args
	inner.depth++
	return evalNodeExpr(&inner, body)
}

// ---------- custom list parameter expressions ----------

// CustomListExpr is a call of a custom list parameter function.
type CustomListExpr struct {
	Function Term
	Body     Term
	Args     []NodeExpr
}

func (e *CustomListExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *CustomListExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	// The arguments are evaluated first, in the scope of the caller, like the
	// arguments of every list parameter function; a failing argument takes
	// precedence over one without a value.
	args := evalListArguments(ctx, e.Args)
	if err := firstArgumentFailure(args); err != nil {
		return nil, err
	}
	if !allBound(args) {
		return nil, nil
	}
	body, err := customBody(ctx, e.Function, e.Body)
	if err != nil {
		return nil, err
	}
	scope := make(map[string]NodeExpr, len(args))
	for i, a := range args {
		scope["#"+strconv.Itoa(i)] = &ConstantExpr{Value: a.node}
	}
	inner := *ctx
	inner.guard = ctx.sharedGuard()
	inner.vars = nil
	inner.args = scope
	inner.depth++
	out, err := evalNodeExpr(&inner, body)
	if err != nil {
		return nil, err
	}
	if len(out) > 1 {
		return nil, failf("the body of %s produced %d output nodes, a list parameter function produces at most one", e.Function, len(out))
	}
	return out, nil
}

// customBody parses the body expression of a custom function, refusing to go
// deeper than maxCustomDepth so that a function calling itself fails.
func customBody(ctx *nodeExprContext, fn, body Term) (NodeExpr, error) {
	if ctx.depth >= maxCustomDepth {
		return nil, failf("custom node expression function %s is nested more than %d levels deep (does it call itself?)", fn, maxCustomDepth)
	}
	if body.IsNone() {
		return nil, failf("custom node expression function %s has no sh:bodyExpression", fn)
	}
	return parseNodeExpr(ctx.shapeDefinitions(), body), nil
}

// ErrNotNodeExprFunction is returned by callCustomFunction for an IRI that the
// shapes graph does not declare as a sh:ListParameterExpressionFunction.
var ErrNotNodeExprFunction = errors.New("shacl: not a declared node expression function")

// callCustomFunction calls the list parameter function iri, declared in the
// shapes graph of ctx, with args, and returns its output nodes (at most one).
//
// It is how SPARQL reaches a custom function: args are the argument values,
// where a zero Term is an unbound argument. An unbound argument of a
// parameter that is not sh:optional, a blank node argument, a body that
// fails, and a body with several output nodes are errors wrapping
// ErrNodeExprFailure; an IRI that is no declared function is
// ErrNotNodeExprFunction. The body is evaluated with the function IRI as its
// focus node, ctx.dataGraph as the focus graph and the arguments as
// [ shnex:arg i ] and as the scope variables "arg0", "arg1", ...
//
// A body that is not a node expression (sh:select, sh:sparqlExpr) is not
// handled here: the caller evaluates it.
func callCustomFunction(ctx *evalContext, iri string, args []Term) ([]Term, error) {
	fns := declaredFunctions(ctx.shapesGraph)
	if fns == nil {
		return nil, ErrNotNodeExprFunction
	}
	fn, ok := fns.list[iri]
	if !ok {
		return nil, ErrNotNodeExprFunction
	}
	scope := make(map[string]NodeExpr, len(args))
	vars := make(map[string]Term, len(args))
	for i, a := range args {
		switch {
		case a.IsNone():
			if !fn.optional[i] {
				return nil, failf("argument %d of %s is unbound and its parameter is not sh:optional", i, fn.iri)
			}
		case a.IsBlank():
			return nil, failf("argument %d of %s is the blank node %s, which a node expression function cannot take", i, fn.iri, a)
		default:
			scope["#"+strconv.Itoa(i)] = &ConstantExpr{Value: a}
			vars["arg"+strconv.Itoa(i)] = a
		}
	}
	nctx := &nodeExprContext{
		dataGraph:   ctx.dataGraph,
		shapesGraph: ctx.shapesGraph,
		shapesMap:   ctx.shapesMap,
		focusNode:   fn.iri,
		vars:        vars,
		af:          ctx.af,
		guard:       ctx.sharedGuard(),
	}
	body, err := customBody(nctx, fn.iri, fn.body)
	if err != nil {
		return nil, err
	}
	nctx.args = scope
	nctx.depth++
	out, err := evalNodeExpr(nctx, body)
	if err != nil {
		return nil, err
	}
	if len(out) > 1 {
		return nil, failf("the body of %s produced %d output nodes, a function produces at most one", fn.iri, len(out))
	}
	return out, nil
}
