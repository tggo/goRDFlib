package shacl

import (
	"sync"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// SPARQLConstraint implements sh:sparql on shapes. Each focus node is validated
// by running a SELECT query with $this pre-bound. Each result row is a violation.
type SPARQLConstraint struct {
	Node        Term   // the blank/IRI node of the sh:sparql object
	Select      string // the sh:select query body
	Prefixes    string // resolved PREFIX preamble
	Messages    []Term // sh:message on the constraint
	Deactivated bool   // sh:deactivated

	// prepared holds the query text Evaluate runs, keyed by the textual
	// $PATH binding ("" for a shape without a predicate path): the text
	// depends only on the constraint and that path, since $this and
	// $currentShape are initial bindings. A constraint is parsed per shape, so
	// in practice there is one entry; keying by path keeps the text right if
	// a constraint is ever evaluated for another shape.
	prepared sync.Map // string -> string
}

// preparedQuery returns the text Evaluate runs for a shape whose $PATH is
// path ("" for a shape without a predicate path): the prefixes and the
// SELECT with $var normalized and $PATH substituted. It is built on first use
// and reused for every later focus node.
func (c *SPARQLConstraint) preparedQuery(path string) string {
	if q, ok := c.prepared.Load(path); ok {
		return q.(string)
	}
	bindings := map[string]string{}
	if path != "" {
		bindings["PATH"] = path
	}
	q := preBindQuery(normalizeDollarVars(c.Prefixes+c.Select), bindings)
	c.prepared.Store(path, q)
	return q
}

func (c *SPARQLConstraint) ComponentIRI() string {
	return SH + "SPARQLConstraintComponent"
}

func (c *SPARQLConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if c.Deactivated {
		return nil
	}

	// Pre-bind variables. Every focus-dependent variable ($this,
	// $currentShape) is an initial binding (preBindTerm): a blank node label
	// written into SPARQL query text denotes a fresh, query-scoped node, not a
	// reference to the data-graph node (SPARQL 1.1 §4.1.4), and SHACL
	// pre-binding is defined as a solution binding, not text substitution
	// (SHACL §5.2.1.3). Only $PATH is substituted into the text, and it
	// depends on the shape alone, so the text is prepared once per constraint
	// and path (preparedQuery).
	initBindings := map[string]term.Term{}
	preBindTerm(nil, initBindings, "this", focusNode)
	preBindTerm(nil, initBindings, "currentShape", shape.ID)
	if ctx.shapesGraph != nil && ctx.shapesGraph.baseURI != "" {
		initBindings["shapesGraph"] = term.NewURIRefUnsafe(ctx.shapesGraph.baseURI)
	}
	path := ""
	if shape.Path != nil && shape.Path.Kind == PathPredicate {
		path = termToSPARQL(shape.Path.Pred)
	}
	query := c.preparedQuery(path)

	// Provide shapes graph as a named graph for GRAPH ?shapesGraph { ... }
	var namedGraphs map[string]*graph.Graph
	if ctx.shapesGraph != nil && ctx.shapesGraph.baseURI != "" {
		sg, err := rdfOf(ctx.goContext(), ctx.shapesGraph)
		if err != nil {
			ctx.guard.fail(err)
			return nil
		}
		namedGraphs = map[string]*graph.Graph{ctx.shapesGraph.baseURI: sg}
	}

	rows, err := executeSPARQL(ctx.goContext(), ctx.dataGraph, query, initBindings, namedGraphs, ctx.sparqlFuncs())
	if err != nil {
		ctx.guard.fail(err)
		ctx.reportMalformed(c, c.Node, err)
		r := makeResult(shape, focusNode, focusNode, c.ComponentIRI())
		r.SourceConstraint = c.Node
		if msgs := resultMessages(nil, func() map[string]Term {
			return messageVars(focusNode, focusNode, r.ResultPath, shape.ID, nil, nil)
		}, c.Messages); msgs != nil {
			r.ResultMessages = msgs
		}
		return []ValidationResult{r}
	}

	var results []ValidationResult
	for _, row := range rows {
		value := focusNode
		if v, ok := row["value"]; ok && !v.IsNone() {
			value = v
		}

		r := makeResult(shape, focusNode, value, c.ComponentIRI())
		r.SourceConstraint = c.Node

		if p, ok := row["path"]; ok && !p.IsNone() {
			r.ResultPath = p
		}

		if msgs := resultMessages(row, func() map[string]Term {
			return messageVars(focusNode, value, r.ResultPath, shape.ID, nil, row)
		}, c.Messages); msgs != nil {
			r.ResultMessages = msgs
		}

		results = append(results, r)
	}
	return results
}
