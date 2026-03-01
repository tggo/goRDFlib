package rdflibgo

import "fmt"

// SPARQLResult holds the result of a SPARQL query.
// Ported from: rdflib.plugins.sparql.sparql.Query result types
type SPARQLResult struct {
	Type     string            // "SELECT", "ASK", "CONSTRUCT"
	Vars     []string          // variable names for SELECT
	Bindings []map[string]Term // solution mappings for SELECT
	AskResult bool            // result for ASK
	Graph    *Graph            // result graph for CONSTRUCT
}

// Query executes a SPARQL query against the graph.
// Ported from: rdflib.graph.Graph.query → SPARQLProcessor.query
func (g *Graph) Query(sparql string, initBindings ...map[string]Term) (*SPARQLResult, error) {
	q, err := ParseSPARQL(sparql)
	if err != nil {
		return nil, fmt.Errorf("sparql parse error: %w", err)
	}

	var bindings map[string]Term
	if len(initBindings) > 0 {
		bindings = initBindings[0]
	}

	return EvalQuery(g, q, bindings)
}
