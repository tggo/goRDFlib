package shacl

import (
	"strconv"

	"github.com/tggo/goRDFlib/sparql"
)

// Aggregation and miscellaneous expressions of SHACL 1.2 Node Expressions.
// The aggregates follow SPARQL: MIN and MAX use the ORDER BY order, SUM the
// numeric addition of SPARQL (so integers, decimals and doubles promote).

// integerValue reads an xsd:integer literal as an int.
func integerValue(t Term) (int, bool) {
	if !t.IsLiteral() || t.Datatype() != XSD+"integer" {
		return 0, false
	}
	n, err := strconv.Atoi(t.Value())
	return n, err == nil
}

// ---------- CountExpr ----------

// CountExpr returns the count of input nodes as an xsd:integer.
type CountExpr struct {
	Nodes NodeExpr
}

func (e *CountExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *CountExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	nodes, err := evalNodeExpr(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	return []Term{integerTerm(len(nodes))}, nil
}

// ---------- SumExpr ----------

// SumExpr returns the sum of the input nodes: 0 for no nodes, and a failure
// when a node is not a number (the SPARQL SUM error).
type SumExpr struct {
	Nodes NodeExpr
}

func (e *SumExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *SumExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	nodes, err := evalNodeExpr(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	sum := integerTerm(0)
	for _, n := range nodes {
		res, err := callSPARQLFunction(ctx.goContext(), "plus", []Term{sum, n})
		if err != nil {
			return nil, failf("shnex:sum cannot add %s: %w", n, err)
		}
		sum = res
	}
	return []Term{sum}, nil
}

// ---------- MinExpr / MaxExpr ----------

// MinExpr returns the minimum of the input nodes in the order of SPARQL
// ORDER BY, which is what SPARQL MIN uses.
type MinExpr struct {
	Nodes NodeExpr
}

func (e *MinExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *MinExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	return extremum(ctx, e.Nodes, func(c int) bool { return c < 0 })
}

// MaxExpr returns the maximum of the input nodes in the order of SPARQL
// ORDER BY, which is what SPARQL MAX uses.
type MaxExpr struct {
	Nodes NodeExpr
}

func (e *MaxExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *MaxExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	return extremum(ctx, e.Nodes, func(c int) bool { return c > 0 })
}

// extremum returns the node n for which better(compare(n, best)) holds
// against every other node, the first one on ties; nothing for no nodes.
func extremum(ctx *nodeExprContext, in NodeExpr, better func(int) bool) ([]Term, error) {
	nodes, err := evalNodeExpr(ctx, in)
	if err != nil || len(nodes) == 0 {
		return nil, err
	}
	best := nodes[0]
	for _, n := range nodes[1:] {
		if better(sparql.CompareTerms(toTerm(n), toTerm(best))) {
			best = n
		}
	}
	return []Term{best}, nil
}

// ---------- InstancesOfExpr ----------

// InstancesOfExpr returns all instances of the classes its argument produces
// (including instances of their subclasses), each once.
type InstancesOfExpr struct {
	Classes NodeExpr
}

func (e *InstancesOfExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *InstancesOfExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	classes, err := evalNodeExpr(ctx, e.Classes)
	if err != nil {
		return nil, err
	}
	for _, c := range classes {
		if !c.IsIRI() {
			return nil, failf("shnex:instancesOf needs classes that are IRIs, not %s", c)
		}
	}
	seen := make(map[string]bool)
	var result []Term
	addInstances := func(cls Term) {
		for _, inst := range ctx.dataGraph.Subjects(IRI(RDFType), cls) {
			key := inst.TermKey()
			if !seen[key] {
				seen[key] = true
				result = append(result, inst)
			}
		}
	}
	for _, c := range classes {
		addInstances(c)
		for _, sub := range subClasses(ctx.dataGraph, c) {
			addInstances(sub)
		}
	}
	return result, nil
}

// ---------- NodesMatchingExpr ----------

// NodesMatchingExpr returns all nodes from the data graph that conform to a shape.
type NodesMatchingExpr struct {
	ShapeRef Term
}

func (e *NodesMatchingExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *NodesMatchingExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	shape := ctx.resolveShape(e.ShapeRef)
	if shape == nil {
		return allNodes(ctx.dataGraph), nil
	}
	var result []Term
	for _, n := range allNodes(ctx.dataGraph) {
		ok, err := ctx.conforms(shape, n)
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, n)
		}
	}
	return result, nil
}
