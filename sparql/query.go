package sparql

import (
	"fmt"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Query executes a SPARQL query against the graph.
// Ported from: rdflib.graph.Graph.query → SPARQLProcessor.query
func Query(g *rdflibgo.Graph, query string, initBindings ...map[string]rdflibgo.Term) (*Result, error) {
	q, err := Parse(query)
	if err != nil {
		return nil, fmt.Errorf("sparql parse error: %w", err)
	}

	var bindings map[string]rdflibgo.Term
	if len(initBindings) > 0 {
		bindings = initBindings[0]
	}

	return EvalQuery(g, q, bindings)
}
