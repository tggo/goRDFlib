package sparql

import rdflibgo "github.com/tggo/goRDFlib"

// Result holds the result of a SPARQL query.
// Ported from: rdflib.plugins.sparql.sparql.Query result types
type Result struct {
	Type      string                     // "SELECT", "ASK", "CONSTRUCT"
	Vars      []string                   // variable names for SELECT
	Bindings  []map[string]rdflibgo.Term // solution mappings for SELECT
	AskResult bool                       // result for ASK
	Graph     *rdflibgo.Graph            // result graph for CONSTRUCT
}
