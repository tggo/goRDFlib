package shacl

import (
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
}

func (c *SPARQLConstraint) ComponentIRI() string {
	return SH + "SPARQLConstraintComponent"
}

func (c *SPARQLConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if c.Deactivated {
		return nil
	}

	// Build the full query with prefixes, normalizing $var to ?var
	fullQuery := normalizeDollarVars(c.Prefixes + c.Select)

	// Pre-bind variables. IRI and literal values are spliced in via textual
	// substitution, but blank-node values must use real pre-binding (initial
	// bindings): a blank node label written into SPARQL query text denotes a
	// fresh, query-scoped node, not a reference to the data-graph node
	// (SPARQL 1.1 §4.1.4), so textual substitution of "_:b0" cannot scope the
	// pattern to that focus node. SHACL pre-binding is defined as a solution
	// binding, not text substitution (SHACL §5.2.1.3).
	bindings := map[string]string{}
	initBindings := map[string]term.Term{}
	preBindTerm(bindings, initBindings, "this", focusNode)
	if shape.Path != nil && shape.Path.Kind == PathPredicate {
		bindings["PATH"] = termToSPARQL(shape.Path.Pred)
	}
	bindings["currentShape"] = termToSPARQL(shape.ID)
	if ctx.shapesGraph != nil && ctx.shapesGraph.baseURI != "" {
		bindings["shapesGraph"] = "<" + ctx.shapesGraph.baseURI + ">"
	}

	query := preBindQuery(fullQuery, bindings)

	// Provide shapes graph as a named graph for GRAPH ?shapesGraph { ... }
	var namedGraphs map[string]*graph.Graph
	if ctx.shapesGraph != nil && ctx.shapesGraph.g != nil && ctx.shapesGraph.baseURI != "" {
		namedGraphs = map[string]*graph.Graph{
			ctx.shapesGraph.baseURI: ctx.shapesGraph.g,
		}
	}

	rows, err := executeSPARQL(ctx.dataGraph, query, initBindings, namedGraphs, ctx.sparqlFuncs())
	if err != nil {
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
