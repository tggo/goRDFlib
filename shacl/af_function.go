package shacl

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// SHACL functions (SHACL-AF §5).
//
// A sh:SPARQLFunction declares an IRI-named function whose body is a SPARQL
// SELECT or ASK query and whose arguments are declared with sh:parameter. It is
// callable from two places: from a node expression as a function expression,
// and from inside any SPARQL query the shapes graph supplies — a sh:sparql
// constraint, a sh:construct rule, a SPARQL-based target.
//
// The second call site is why these are bound per query rather than registered
// with sparql.RegisterFunction: the definitions come from the shapes graph
// being validated, and two shapes graphs may define the same IRI differently.

// maxFunctionDepth bounds recursion through SHACL functions. A function whose
// body calls itself is legal to write and never terminates; without a bound the
// stack, not the validator, would decide what happens.
const maxFunctionDepth = 32

// shaclFunction is one sh:SPARQLFunction from the shapes graph.
type shaclFunction struct {
	iri    string
	params []shaclParameter // in call order
	query  string           // sh:select or sh:ask, prefixes already applied
	isAsk  bool
}

// shaclParameter is one sh:parameter of a function. The name is the local name
// of the parameter's sh:path, which is what the function body refers to: a
// parameter with sh:path ex:op1 is written $op1 in the query.
type shaclParameter struct {
	name     string
	optional bool
	order    float64
	hasOrder bool
}

// loadFunctions reads every sh:SPARQLFunction in the shapes graph.
//
// A malformed function is reported but does not stop the others from loading:
// one bad declaration should not disable a whole shapes graph, and the caller
// sees the error through the error handler or from ApplyRules.
func loadFunctions(shapesGraph *Graph) (map[string]*shaclFunction, error) {
	typePred := IRI(RDFType)
	fnType := IRI(SHSPARQLFunction)
	nodes := shapesGraph.Subjects(typePred, fnType)
	if len(nodes) == 0 {
		return nil, nil
	}

	funcs := make(map[string]*shaclFunction, len(nodes))
	var firstErr error
	for _, node := range nodes {
		fn, err := parseFunction(shapesGraph, node)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		funcs[fn.iri] = fn
	}
	return funcs, firstErr
}

func parseFunction(g *Graph, node Term) (*shaclFunction, error) {
	if !node.IsIRI() {
		return nil, fmt.Errorf("%w: a sh:SPARQLFunction must be named by an IRI, got %s", ErrMalformedFunction, node)
	}

	selects := g.Objects(node, IRI(SH+"select"))
	asks := g.Objects(node, IRI(SH+"ask"))
	switch {
	case len(selects) > 0 && len(asks) > 0:
		return nil, fmt.Errorf("%w: %s declares both sh:select and sh:ask; it must declare exactly one", ErrMalformedFunction, node)
	case len(selects) == 0 && len(asks) == 0:
		return nil, fmt.Errorf("%w: %s declares neither sh:select nor sh:ask", ErrMalformedFunction, node)
	}

	fn := &shaclFunction{iri: node.Value(), isAsk: len(asks) > 0}
	body := selects
	if fn.isAsk {
		body = asks
	}
	fn.query = resolvePrefixes(g, node) + normalizeDollarVars(body[0].Value())

	for _, p := range g.Objects(node, IRI(SH+"parameter")) {
		param, err := parseFunctionParameter(g, node, p)
		if err != nil {
			return nil, err
		}
		fn.params = append(fn.params, param)
	}
	sortParameters(fn.params)
	return fn, nil
}

func parseFunctionParameter(g *Graph, fnNode, p Term) (shaclParameter, error) {
	paths := g.Objects(p, IRI(SH+"path"))
	if len(paths) == 0 {
		return shaclParameter{}, fmt.Errorf("%w: a sh:parameter of %s has no sh:path, so its argument has no name", ErrMalformedFunction, fnNode)
	}
	name := localName(paths[0].Value())
	if name == "" {
		return shaclParameter{}, fmt.Errorf("%w: the sh:parameter path %s of %s has no local name to use as a variable", ErrMalformedFunction, paths[0], fnNode)
	}

	param := shaclParameter{name: name}
	if opts := g.Objects(p, IRI(SHOptional)); len(opts) > 0 {
		param.optional = opts[0].Value() == "true"
	}
	if orders := g.Objects(p, IRI(SHOrder)); len(orders) > 0 {
		if v, err := strconv.ParseFloat(orders[0].Value(), 64); err == nil {
			param.order, param.hasOrder = v, true
		}
	}
	return param, nil
}

// sortParameters puts the parameters in the order a caller passes arguments.
//
// SHACL-AF orders by sh:order, but sh:order is optional and a function that
// declares it on only some parameters has no meaningful numeric order. In that
// case the established behaviour — pySHACL's, and what the DASH tests rely on —
// is to fall back to sorting by parameter name, which is stable and matches the
// order arguments are written in the examples.
func sortParameters(params []shaclParameter) {
	allOrdered := true
	for _, p := range params {
		if !p.hasOrder {
			allOrdered = false
			break
		}
	}
	sort.SliceStable(params, func(i, j int) bool {
		if allOrdered && params[i].order != params[j].order {
			return params[i].order < params[j].order
		}
		if allOrdered {
			return false
		}
		return params[i].name < params[j].name
	})
}

// call invokes the function against the data graph with the given arguments.
//
// A nil argument is an unbound value: SPARQL evaluated the expression to
// nothing, or a node expression produced no nodes. It is passed through only
// for an optional parameter; for a required one the call yields no result,
// which is what SPARQL does with an expression error.
// goctx is the context of the query that called the function; the function's
// own query runs with it.
func (fn *shaclFunction) call(goctx context.Context, ctx *afContext, args []term.Term, depth int) (term.Term, error) {
	if depth > maxFunctionDepth {
		return nil, fmt.Errorf("%w: %s recursed more than %d levels deep; a function whose body calls itself does not terminate",
			ErrMalformedExpression, fn.iri, maxFunctionDepth)
	}
	if len(args) != len(fn.params) {
		return nil, fmt.Errorf("%w: %s takes %d argument(s), got %d",
			ErrMalformedExpression, fn.iri, len(fn.params), len(args))
	}

	bindings := make(map[string]term.Term, len(args))
	for i, p := range fn.params {
		if args[i] == nil {
			if !p.optional {
				return nil, nil
			}
			continue
		}
		bindings[p.name] = args[i]
	}

	if fn.isAsk {
		ok, err := executeSPARQLAsk(goctx, ctx.dataGraph, fn.query, bindings, nil, ctx.functionsAtDepth(depth+1))
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrMalformedExpression, fn.iri, err)
		}
		return term.NewLiteral(strconv.FormatBool(ok), term.WithDatatype(term.MustURIRef(XSD+"boolean"))), nil
	}

	rows, err := executeSPARQL(goctx, ctx.dataGraph, fn.query, bindings, nil, ctx.functionsAtDepth(depth+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrMalformedExpression, fn.iri, err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	// SHACL-AF §5.1: the function's value is the first variable of the first
	// solution. Ordering within a row is not defined by a Go map, so the
	// projection order is recovered from the query text.
	name := firstProjectedVar(fn.query)
	if v, ok := rows[0][name]; ok {
		return toTerm(v), nil
	}
	if name == "" && len(rows[0]) == 1 {
		for _, v := range rows[0] {
			return toTerm(v), nil
		}
	}
	return nil, nil
}

// firstProjectedVar returns the name of the first variable projected by a
// SELECT query, without the leading marker.
//
// The bindings a row carries are an unordered map, so "the first variable" has
// to come from the query text. Only the top-level projection is scanned; an
// expression projected as (expr AS ?v) contributes ?v, which is what appears in
// the row.
func firstProjectedVar(query string) string {
	upper := strings.ToUpper(query)
	sel := strings.Index(upper, "SELECT")
	if sel < 0 {
		return ""
	}
	rest := query[sel+len("SELECT"):]
	if end := strings.Index(strings.ToUpper(rest), "WHERE"); end >= 0 {
		rest = rest[:end]
	}

	// Inside (expr AS ?v) the projected name is the last variable, not the
	// first, so a parenthesised group is scanned to its end before deciding.
	depth := 0
	var lastInGroup string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '(':
			depth++
			lastInGroup = ""
			continue
		case ')':
			depth--
			if depth <= 0 && lastInGroup != "" {
				return lastInGroup
			}
			continue
		case '?', '$':
			j := i + 1
			for j < len(rest) && isVarChar(rest[j]) {
				j++
			}
			if j == i+1 {
				continue
			}
			name := rest[i+1 : j]
			if depth == 0 {
				return name
			}
			lastInGroup = name
			i = j - 1
		}
	}
	return ""
}

// sparqlFunctions turns the loaded functions into a table that can be bound to
// a parsed query with sparql.ParsedQuery.BindFunctions.
//
// depth is the nesting level of the query being bound: a function called from
// inside another function's body must not be able to recurse without bound.
func (ctx *afContext) functionsAtDepth(depth int) map[string]sparql.ContextFunction {
	if len(ctx.funcs) == 0 || depth > maxFunctionDepth {
		return nil
	}
	table := make(map[string]sparql.ContextFunction, len(ctx.funcs))
	for iri, fn := range ctx.funcs {
		fn := fn
		table[iri] = func(goctx context.Context, args []term.Term) (term.Term, error) {
			return fn.call(goctx, ctx, args, depth)
		}
	}
	return table
}
