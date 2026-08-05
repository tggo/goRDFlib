package shacl

import (
	"fmt"

	"github.com/tggo/goRDFlib/term"
)

// SHACL-AF node expressions (§6).
//
// A node expression maps a focus node to a set of nodes. The grammar is small:
//
//	sh:this                        the focus node
//	<iri> or a literal             itself
//	[ sh:union ( e1 e2 ) ]         the union of the parts
//	[ sh:intersection ( e1 e2 ) ]  the intersection of the parts
//	[ sh:path p ]                  the values of p, from sh:nodes or the focus node
//	[ sh:filterShape s ]           those of sh:nodes that conform to s
//	[ ex:fn ( a1 a2 ) ]            ex:fn applied to the arguments
//
// The result is a set, but the spec does not fix an iteration order and an
// arbitrary one would make rule output non-reproducible. Every expression here
// therefore de-duplicates while preserving the order nodes were first produced.
//
// This vocabulary is disjoint from SHACL 1.2's shnex: node expressions, which
// are parsed separately in node_expr.go. Nothing here is read unless SHACL-AF
// is enabled.

// afExprMaxDepth bounds nesting. A node expression can reference a shape whose
// own expressions reference it back; the parser follows blank nodes, so a
// cyclic shapes graph would otherwise not terminate.
const afExprMaxDepth = 32

// afThisExpr is sh:this: the focus node.
type afThisExpr struct{}

func (e *afThisExpr) Eval(ctx *nodeExprContext) []Term {
	if ctx.focusNode.IsNone() {
		return nil
	}
	return []Term{ctx.focusNode}
}

// afUnionExpr is [ sh:union ( ... ) ].
type afUnionExpr struct{ Parts []NodeExpr }

func (e *afUnionExpr) Eval(ctx *nodeExprContext) []Term {
	var out []Term
	seen := make(map[string]bool)
	for _, p := range e.Parts {
		for _, t := range p.Eval(ctx) {
			if k := t.TermKey(); !seen[k] {
				seen[k] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// afIntersectionExpr is [ sh:intersection ( ... ) ].
type afIntersectionExpr struct{ Parts []NodeExpr }

func (e *afIntersectionExpr) Eval(ctx *nodeExprContext) []Term {
	if len(e.Parts) == 0 {
		return nil
	}
	// The first part fixes both the candidate set and the output order; every
	// later part can only remove from it.
	var out []Term
	seen := make(map[string]bool)
	for _, t := range e.Parts[0].Eval(ctx) {
		if k := t.TermKey(); !seen[k] {
			seen[k] = true
			out = append(out, t)
		}
	}
	for _, p := range e.Parts[1:] {
		if len(out) == 0 {
			return nil
		}
		keep := make(map[string]bool)
		for _, t := range p.Eval(ctx) {
			keep[t.TermKey()] = true
		}
		filtered := out[:0]
		for _, t := range out {
			if keep[t.TermKey()] {
				filtered = append(filtered, t)
			}
		}
		out = filtered
	}
	return out
}

// afPathExpr is [ sh:path p ], optionally with sh:nodes to supply the nodes the
// path starts from instead of the focus node.
type afPathExpr struct {
	Path  *PropertyPath
	Nodes NodeExpr // nil = start from the focus node
}

func (e *afPathExpr) Eval(ctx *nodeExprContext) []Term {
	starts := []Term{ctx.focusNode}
	if e.Nodes != nil {
		starts = e.Nodes.Eval(ctx)
	}
	var out []Term
	seen := make(map[string]bool)
	for _, s := range starts {
		if s.IsNone() {
			continue
		}
		for _, t := range evalPath(ctx.dataGraph, e.Path, s) {
			if k := t.TermKey(); !seen[k] {
				seen[k] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// afFilterShapeExpr is [ sh:filterShape s ; sh:nodes e ]: those nodes produced
// by e that conform to s.
type afFilterShapeExpr struct {
	Nodes NodeExpr
	Shape Term
}

func (e *afFilterShapeExpr) Eval(ctx *nodeExprContext) []Term {
	if e.Nodes == nil {
		return nil
	}
	// A shape used as a filter is evaluated on its own terms: only whether the
	// node conforms matters, so its own targets are irrelevant.
	evalCtx := &evalContext{
		dataGraph:      ctx.dataGraph,
		shapesGraph:    ctx.shapesGraph,
		shapesMap:      ctx.shapesMap,
		classInstances: ctx.classInstances,
		af:             ctx.af,
	}
	shape := resolveShape(evalCtx, e.Shape)
	if shape == nil {
		return nil
	}
	var out []Term
	for _, n := range e.Nodes.Eval(ctx) {
		if len(validateShapeOnNode(evalCtx, shape, n)) == 0 {
			out = append(out, n)
		}
	}
	return out
}

// afFunctionExpr is [ ex:fn ( a1 a2 ) ]: a SHACL function applied to the
// arguments.
type afFunctionExpr struct {
	Fn   *shaclFunction
	Args []NodeExpr
}

// Eval applies the function to every combination of argument values.
//
// Each argument is itself a node expression and so yields a set. SHACL-AF
// defines the result as the function applied to each element of the cartesian
// product of those sets, which is why a single-valued call is the common case
// but not the only one.
func (e *afFunctionExpr) Eval(ctx *nodeExprContext) []Term {
	if ctx.af == nil {
		return nil
	}
	argSets := make([][]term.Term, len(e.Args))
	for i, a := range e.Args {
		for _, t := range a.Eval(ctx) {
			argSets[i] = append(argSets[i], toTerm(t))
		}
		if len(argSets[i]) == 0 {
			// An argument that produced nothing is unbound. The function
			// decides whether that is acceptable, based on sh:optional.
			argSets[i] = []term.Term{nil}
		}
	}

	var out []Term
	seen := make(map[string]bool)
	for _, args := range cartesian(argSets) {
		res, err := e.Fn.call(ctx.af, args, 1)
		if err != nil {
			ctx.af.cfg.report(err)
			continue
		}
		if res == nil {
			continue
		}
		t := fromRDFLib(res)
		if k := t.TermKey(); !seen[k] {
			seen[k] = true
			out = append(out, t)
		}
	}
	return out
}

// cartesian returns every combination taking one element from each set, in
// row-major order so the result is deterministic.
func cartesian(sets [][]term.Term) [][]term.Term {
	if len(sets) == 0 {
		return [][]term.Term{{}}
	}
	total := 1
	for _, s := range sets {
		total *= len(s)
		if total == 0 {
			return nil
		}
	}
	out := make([][]term.Term, 0, total)
	idx := make([]int, len(sets))
	for {
		row := make([]term.Term, len(sets))
		for i, s := range sets {
			row[i] = s[idx[i]]
		}
		out = append(out, row)

		i := len(sets) - 1
		for i >= 0 {
			idx[i]++
			if idx[i] < len(sets[i]) {
				break
			}
			idx[i] = 0
			i--
		}
		if i < 0 {
			return out
		}
	}
}

// parseExpressionFor picks the vocabulary a sh:expression is written in.
//
// SHACL 1.2's shnex: expressions and SHACL-AF's sh: expressions both appear as
// the value of sh:expression, so the two have to be told apart. A blank node
// carrying any shnex: property is 1.2 and is never read as AF. Otherwise, with
// AF enabled, AF is tried first and 1.2 is the fallback — an expression AF
// cannot read is more likely to be a 1.2 one than a mistake.
func parseExpressionFor(ctx *evalContext, node Term) NodeExpr {
	if ctx.af == nil || hasSHNEXProperty(ctx.shapesGraph, node) {
		return parseNodeExpr(ctx.shapesGraph, node)
	}
	expr, err := parseAFNodeExpr(ctx.af, node, 0)
	if err != nil {
		ctx.af.cfg.report(err)
		return parseNodeExpr(ctx.shapesGraph, node)
	}
	return expr
}

// hasSHNEXProperty reports whether node is described with SHACL 1.2 node
// expression vocabulary.
func hasSHNEXProperty(g *Graph, node Term) bool {
	if !node.IsBlank() {
		return false
	}
	for _, t := range g.All(&node, nil, nil) {
		if len(t.Predicate.Value()) > len(SHNEX) && t.Predicate.Value()[:len(SHNEX)] == SHNEX {
			return true
		}
	}
	return false
}

// parseAFNodeExpr reads a node expression written in SHACL-AF vocabulary.
//
// It returns an error rather than a silent nil for a construct that looks like
// an expression but cannot be resolved — most importantly a function call
// naming an IRI that no sh:SPARQLFunction declares, which is otherwise
// indistinguishable from a rule that legitimately produces nothing.
func parseAFNodeExpr(ctx *afContext, node Term, depth int) (NodeExpr, error) {
	if depth > afExprMaxDepth {
		return nil, fmt.Errorf("%w: nested more than %d levels deep at %s", ErrMalformedExpression, afExprMaxDepth, node)
	}
	g := ctx.shapesGraph

	if node.IsIRI() && node.Value() == SHThis {
		return &afThisExpr{}, nil
	}
	if node.IsLiteral() || node.IsIRI() {
		return &ConstantExpr{Value: node}, nil
	}

	if list := g.Objects(node, IRI(SHUnion)); len(list) > 0 {
		parts, err := parseAFExprList(ctx, list[0], depth)
		if err != nil {
			return nil, err
		}
		return &afUnionExpr{Parts: parts}, nil
	}
	if list := g.Objects(node, IRI(SHIntersection)); len(list) > 0 {
		parts, err := parseAFExprList(ctx, list[0], depth)
		if err != nil {
			return nil, err
		}
		return &afIntersectionExpr{Parts: parts}, nil
	}

	// sh:filterShape is checked before sh:path: a filter's shape may itself be
	// a property shape carrying sh:path, and reading that path as the filter's
	// own would silently evaluate something else entirely.
	if fs := g.Objects(node, IRI(SHFilterShape)); len(fs) > 0 {
		nodesArg := g.Objects(node, IRI(SHNodes))
		if len(nodesArg) == 0 {
			return nil, fmt.Errorf("%w: the sh:filterShape expression %s has no sh:nodes to filter", ErrMalformedExpression, node)
		}
		inner, err := parseAFNodeExpr(ctx, nodesArg[0], depth+1)
		if err != nil {
			return nil, err
		}
		return &afFilterShapeExpr{Nodes: inner, Shape: fs[0]}, nil
	}

	if p := g.Objects(node, IRI(SH+"path")); len(p) > 0 {
		expr := &afPathExpr{Path: parsePath(g, p[0])}
		if nodesArg := g.Objects(node, IRI(SHNodes)); len(nodesArg) > 0 {
			inner, err := parseAFNodeExpr(ctx, nodesArg[0], depth+1)
			if err != nil {
				return nil, err
			}
			expr.Nodes = inner
		}
		return expr, nil
	}

	return parseAFFunctionExpr(ctx, node, depth)
}

// parseAFFunctionExpr reads [ ex:fn ( args ) ].
//
// A function call is the only expression identified by its predicate rather
// than by a fixed keyword, so it is recognised last: any remaining blank node
// whose single predicate points at an RDF list is a call.
func parseAFFunctionExpr(ctx *afContext, node Term, depth int) (NodeExpr, error) {
	g := ctx.shapesGraph
	firstPred := IRI(RDFFirst)

	var fnIRI Term
	var argList Term
	for _, t := range g.All(&node, nil, nil) {
		// sh:message may annotate any expression; it is not the call.
		if t.Predicate.Value() == SH+"message" {
			continue
		}
		if t.Object.IsLiteral() {
			continue
		}
		obj := t.Object
		isList := g.Has(&obj, &firstPred, nil) || (obj.IsIRI() && obj.Value() == RDFNil)
		if !isList {
			continue
		}
		fnIRI, argList = t.Predicate, t.Object
		break
	}
	if fnIRI.IsNone() {
		return nil, fmt.Errorf("%w: %s is not a node expression — no sh:this, sh:path, sh:union, sh:intersection, sh:filterShape, and no function call", ErrMalformedExpression, node)
	}

	fn := ctx.funcs[fnIRI.Value()]
	if fn == nil {
		return nil, fmt.Errorf("%w: the node expression %s calls %s, which no sh:SPARQLFunction in the shapes graph declares", ErrMalformedExpression, node, fnIRI)
	}

	args, err := parseAFExprList(ctx, argList, depth)
	if err != nil {
		return nil, err
	}
	if len(args) != len(fn.params) {
		return nil, fmt.Errorf("%w: %s takes %d argument(s) but is called with %d", ErrMalformedExpression, fnIRI, len(fn.params), len(args))
	}
	return &afFunctionExpr{Fn: fn, Args: args}, nil
}

func parseAFExprList(ctx *afContext, listHead Term, depth int) ([]NodeExpr, error) {
	items := ctx.shapesGraph.RDFList(listHead)
	exprs := make([]NodeExpr, 0, len(items))
	for _, item := range items {
		e, err := parseAFNodeExpr(ctx, item, depth+1)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, e)
	}
	return exprs, nil
}
