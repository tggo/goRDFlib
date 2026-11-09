package shacl

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// Custom functions in SPARQL (SHACL-SPARQL 1.2, "Declaring SPARQL Functions
// based on Node Expressions").
//
// A shapes graph declares a function as an instance of
// sh:ListParameterExpressionFunction; node_expr_custom.go reads the
// declaration and evaluates a body that is a node expression. This file makes
// the same declarations callable from the SPARQL queries the shapes graph
// supplies (sh:sparql, SPARQL-based validators, sh:select and sh:sparqlExpr
// expressions), and evaluates the two bodies that are SPARQL themselves.
//
// Like the SHACL-AF functions they are bound to one query
// (sparql.ParsedQuery.BindContextFunctions), never registered globally: the
// definitions belong to the shapes graph being validated. They need no option;
// SHACL-AF is a different mechanism (sh:SPARQLFunction).

// customSPARQLFunctions adds the custom functions of the shapes graph to table
// (the SHACL-AF functions, possibly nil) and returns the result. depth is the
// nesting level of the query being bound, which bounds a function that calls
// itself. A function that table already holds is kept.
func (ctx *evalContext) customSPARQLFunctions(table map[string]sparql.ContextFunction, depth int) map[string]sparql.ContextFunction {
	if ctx == nil || ctx.shapesGraph == nil || depth > maxFunctionDepth {
		return table
	}
	fns := declaredFunctions(ctx.shapesGraph)
	if fns == nil || len(fns.list) == 0 {
		return table
	}
	if table == nil {
		table = make(map[string]sparql.ContextFunction, len(fns.list))
	}
	for iri := range fns.list {
		if _, taken := table[iri]; taken {
			continue
		}
		iri := iri
		table[iri] = func(goctx context.Context, args []term.Term) (term.Term, error) {
			return ctx.callCustomSPARQLFunction(goctx, iri, args, depth)
		}
	}
	return table
}

// callCustomSPARQLFunction evaluates a call of the custom function iri made by
// a SPARQL expression. Per the spec the call is a SPARQL error, so the
// expression is unbound, when an argument is unbound or a blank node, when the
// body fails, or when it does not produce exactly one node. A query the engine
// could not run is also the failure of the validation run.
func (ctx *evalContext) callCustomSPARQLFunction(goctx context.Context, iri string, args []term.Term, depth int) (term.Term, error) {
	shArgs := make([]Term, len(args))
	for i, a := range args {
		if a != nil {
			shArgs[i] = fromRDFLib(a)
		}
	}

	var out []Term
	var err error
	fns := declaredFunctions(ctx.shapesGraph)
	if fns == nil || fns.list[iri] == nil {
		return nil, nil
	}
	fn := fns.list[iri]
	if sp := parseSPARQLValues(ctx.shapesGraph, fn.body, IRI(SH+"select")); sp != nil {
		out, err = ctx.callSPARQLBody(goctx, fn, sp, shArgs, depth)
	} else {
		out, err = callCustomFunction(ctx, iri, shArgs)
	}
	if err != nil {
		if errors.Is(err, ErrQueryFailed) {
			ctx.guard.fail(err)
		}
		return nil, nil // SPARQL error: the expression stays unbound
	}
	if len(out) != 1 {
		return nil, nil
	}
	return toTerm(out[0]), nil
}

// callSPARQLBody evaluates a body written as sh:select or sh:sparqlExpr
// (SHACL-SPARQL 1.2, "Select Expressions" and "SPARQL Expr Expressions"): the
// function IRI is $this and the arguments are pre-bound as $arg0, $arg1, ...
// An unbound optional argument is left unbound.
func (ctx *evalContext) callSPARQLBody(goctx context.Context, fn *customFunction, sp *SPARQLValues, args []Term, depth int) ([]Term, error) {
	bindings := map[string]term.Term{"this": toTerm(fn.iri)}
	for i, a := range args {
		switch {
		case a.IsNone():
			if !fn.optional[i] {
				return nil, fmt.Errorf("argument %d of %s is unbound and its parameter is not sh:optional", i, fn.iri)
			}
		case a.IsBlank():
			return nil, fmt.Errorf("argument %d of %s is a blank node, which a function cannot take", i, fn.iri)
		default:
			bindings["arg"+strconv.Itoa(i)] = toTerm(a)
		}
	}

	query := sp.Select
	if query == "" {
		query = "SELECT (" + sp.Expr + " AS ?result) WHERE { }"
	}
	query = normalizeDollarVars(sp.Prefixes + query)
	if err := checkPreBound(query, nil, bindings); err != nil {
		return nil, err
	}
	rows, err := executeSPARQL(goctx, ctx.dataGraph, query, bindings, nil, ctx.sparqlFuncsAt(depth+1))
	if err != nil {
		return nil, err
	}
	var out []Term
	for _, row := range rows {
		for _, v := range row { // exactly one variable is projected
			if !v.IsNone() {
				out = append(out, v)
			}
			break
		}
	}
	return out, nil
}
