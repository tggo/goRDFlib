package shacl

import (
	"errors"
	"fmt"
	"strconv"
)

// SHNEX is the namespace for SHACL Node Expressions.
const SHNEX = "http://www.w3.org/ns/shacl-node-expr#"

// SPARQLNS is the namespace of the SPARQL function expressions
// (sparql:abs, sparql:plus, ...) of SHACL 1.2 Node Expressions.
const SPARQLNS = "http://www.w3.org/ns/sparql#"

// ErrNodeExprFailure is the evaluation failure of SHACL 1.2 Node Expressions
// ("Handling of Failures"): the expression has no output nodes because it
// could not be evaluated, which is different from producing none. An
// instancesOf expression over a literal, a path values expression with two
// focus nodes, a SPARQL function applied to an argument it rejects, or a
// shape that cannot be checked all produce it.
//
// A failure of a nested expression is the failure of the surrounding one;
// only the SPARQL function expressions that handle errors (sparql:coalesce,
// sparql:if, sparql:logical-and, sparql:logical-or) absorb it. A constraint
// that meets a failure makes the whole validation fail instead of reporting
// the constraint as satisfied (see recursionGuard.fail). The message names the
// expression and the value that caused it; test with errors.Is.
var ErrNodeExprFailure = errors.New("shacl: node expression evaluation failure")

// failf builds an ErrNodeExprFailure with a message.
func failf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrNodeExprFailure}, args...)...)
}

// NodeExpr represents a SHACL node expression that can be evaluated
// to produce a list of terms.
//
// The SHACL 1.2 expressions also implement failingNodeExpr, whose Evaluate
// reports an evaluation failure; Eval is the same evaluation with the failure
// read as "no output nodes", which is all a caller that cannot fail (the
// SHACL-AF expressions, which have no failures) needs.
type NodeExpr interface {
	Eval(ctx *nodeExprContext) []Term
}

// failingNodeExpr is a node expression that can fail.
type failingNodeExpr interface {
	NodeExpr
	// Evaluate returns the output nodes, or an error wrapping
	// ErrNodeExprFailure.
	Evaluate(ctx *nodeExprContext) ([]Term, error)
}

// evalNodeExpr evaluates e, returning the failure of a SHACL 1.2 expression.
func evalNodeExpr(ctx *nodeExprContext, e NodeExpr) ([]Term, error) {
	if f, ok := e.(failingNodeExpr); ok {
		return f.Evaluate(ctx)
	}
	return e.Eval(ctx), nil
}

// dropFailure is Eval in terms of Evaluate.
func dropFailure(nodes []Term, _ error) []Term { return nodes }

// nodeExprContext holds the evaluation context for node expressions.
type nodeExprContext struct {
	dataGraph *Graph
	shapesMap map[string]*Shape
	focusNode Term
	vars      map[string]Term // the scope: variables read by shnex:var

	// args is the scope of a custom node expression function body: the
	// arguments read by shnex:arg, keyed by argKey. Nil outside such a body.
	args map[string]NodeExpr

	// depth counts the custom function bodies being evaluated, which is how a
	// function that calls itself fails instead of overflowing the stack.
	depth int

	// The remaining fields are only populated for SHACL-AF expressions, which
	// can call functions and validate against a shape. SHACL 1.2's shnex:
	// expressions leave them nil.
	shapesGraph *Graph
	af          *afContext

	// guard is the recursion guard of the validation this expression runs in;
	// see evalContext.guard.
	guard *recursionGuard
}

// maxCustomDepth bounds the nesting of custom node expression function
// bodies. A function whose body calls it again has no output; the limit turns
// that into an ErrNodeExprFailure.
const maxCustomDepth = 128

// shapeDefinitions returns the graph that shapes referenced from an expression
// are read from: the shapes graph when the expression runs inside a validation,
// or the data graph when it is evaluated on its own (the SHACL 1.2 node
// expression tests keep both in one graph). Reading the data graph inside a
// validation made a shape reached through shnex:filterShape lose every
// constraint defined in the shapes graph, sh:expression included.
func (ctx *nodeExprContext) shapeDefinitions() *Graph {
	if ctx.shapesGraph != nil {
		return ctx.shapesGraph
	}
	return ctx.dataGraph
}

// sharedGuard returns ctx's recursion guard, creating it on first use.
func (ctx *nodeExprContext) sharedGuard() *recursionGuard {
	if ctx.guard == nil {
		ctx.guard = &recursionGuard{}
	}
	return ctx.guard
}

// at returns a context for evaluating a nested expression with focus as the
// focus node and the same scope.
func (ctx *nodeExprContext) at(focus Term) *nodeExprContext {
	c := *ctx
	c.guard = ctx.sharedGuard()
	c.focusNode = focus
	return &c
}

// single is the focus node as a list, which is what an expression without
// shnex:nodes starts from; nil when there is no focus node.
func (ctx *nodeExprContext) single() []Term {
	if ctx.focusNode.IsNone() {
		return nil
	}
	return []Term{ctx.focusNode}
}

// ---------- ConstantExpr ----------

// ConstantExpr returns the constant term as a single-element list.
type ConstantExpr struct {
	Value Term
}

func (e *ConstantExpr) Eval(ctx *nodeExprContext) []Term {
	return []Term{e.Value}
}

// ---------- ListExpr ----------

// ListExpr returns the list members as-is.
// An empty RDF list ( ) returns [ rdf:nil ].
type ListExpr struct {
	Members []NodeExpr
	IsNil   bool // true when the expression was rdf:nil (empty list)
}

func (e *ListExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *ListExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	if e.IsNil {
		return []Term{IRI(RDFNil)}, nil
	}
	return concatAll(ctx, e.Members)
}

// concatAll concatenates the output nodes of members, left to right.
func concatAll(ctx *nodeExprContext, members []NodeExpr) ([]Term, error) {
	var result []Term
	for _, m := range members {
		nodes, err := evalNodeExpr(ctx, m)
		if err != nil {
			return nil, err
		}
		result = append(result, nodes...)
	}
	return result, nil
}

// ---------- EmptyExpr ----------

// EmptyExpr returns an empty list (blank node with no shnex properties).
type EmptyExpr struct{}

func (e *EmptyExpr) Eval(ctx *nodeExprContext) []Term {
	return nil
}

// ---------- IllFormedExpr ----------

// IllFormedExpr is a blank node that is described but is no node expression
// this processor knows: its properties are not the key parameter of any
// built-in or declared function. Evaluating it is a failure, not an empty
// result, so a misspelled or undeclared function is not mistaken for an
// expression that legitimately produces nothing.
type IllFormedExpr struct {
	Node Term
}

func (e *IllFormedExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *IllFormedExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	return nil, failf("%s is not a well-formed node expression: none of its properties is the key parameter of a node expression function declared in the shapes graph", e.Node)
}

// ---------- VarExpr ----------

// VarExpr looks up a variable by name.
type VarExpr struct {
	Name string
}

func (e *VarExpr) Eval(ctx *nodeExprContext) []Term {
	if e.Name == "focusNode" {
		return ctx.single()
	}
	if v, ok := ctx.vars[e.Name]; ok {
		return []Term{v}
	}
	return nil
}

// ---------- ArgExpr ----------

// ArgExpr is [ shnex:arg key ]: an argument of the custom node expression
// function whose body it appears in. The argument is itself a node expression
// and is evaluated with an empty scope, so it cannot see the variables of the
// caller (SHACL 1.2 Node Expressions, Arg Expressions).
type ArgExpr struct {
	Key string // see argKey
}

// argKey is the key of an argument in nodeExprContext.args: an IRI by its
// TermKey, an integer (the index of a list parameter) by its value.
func argKey(t Term) (string, bool) {
	switch {
	case t.IsIRI():
		return t.TermKey(), true
	case t.IsLiteral() && t.Datatype() == XSD+"integer":
		if n, err := strconv.Atoi(t.Value()); err == nil {
			return "#" + strconv.Itoa(n), true
		}
	}
	return "", false
}

func (e *ArgExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *ArgExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	arg, ok := ctx.args[e.Key]
	if !ok {
		return nil, nil
	}
	inner := *ctx
	inner.guard = ctx.sharedGuard()
	inner.vars = nil
	inner.args = nil
	return evalNodeExpr(&inner, arg)
}

// ---------- PathValuesExpr ----------

// PathValuesExpr evaluates a property path from a focus node.
type PathValuesExpr struct {
	Path      *PropertyPath
	FocusNode NodeExpr // optional shnex:focusNode override
}

func (e *PathValuesExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *PathValuesExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	focusNodes := ctx.single()
	if e.FocusNode != nil {
		var err error
		if focusNodes, err = evalNodeExpr(ctx, e.FocusNode); err != nil {
			return nil, err
		}
	}
	switch len(focusNodes) {
	case 0:
		return nil, nil
	case 1:
		return evalPath(ctx.dataGraph, e.Path, focusNodes[0]), nil
	}
	return nil, failf("shnex:focusNode of a path values expression produced %d nodes, one is required", len(focusNodes))
}

// isTrueOnly reports whether nodes is exactly the list ( true ).
func isTrueOnly(nodes []Term) bool {
	return len(nodes) == 1 && nodes[0].IsLiteral() && nodes[0].Datatype() == XSD+"boolean" && nodes[0].Value() == "true"
}

func boolTerm(b bool) Term {
	if b {
		return Literal("true", XSD+"boolean", "")
	}
	return Literal("false", XSD+"boolean", "")
}

func integerTerm(n int) Term {
	return Literal(strconv.Itoa(n), XSD+"integer", "")
}

// isTruthy checks if an expression result is truthy.
// A single xsd:boolean "true" literal is truthy; a non-empty list of non-false values is truthy.
//
// This is the SHACL-AF reading of a condition. A SHACL 1.2 condition is
// isTrueOnly.
func isTruthy(terms []Term) bool {
	if len(terms) == 0 {
		return false
	}
	for _, t := range terms {
		if t.IsLiteral() && t.Value() == "false" && (t.Datatype() == XSD+"boolean" || t.Datatype() == "") {
			return false
		}
	}
	return true
}
