package reasoning

import (
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// collectList walks an RDF collection (rdf:first/rdf:rest/rdf:nil) starting from head
// and returns all elements. Returns nil if head is not a valid list node.
func collectList(stop *stopper, g *graph.Graph, head term.Term) []term.Term {
	var result []term.Term
	visited := make(map[string]struct{}) // cycle protection

	cur := head
	nilKey := term.TermKey(namespace.RDF.Nil)
	firstPred := namespace.RDF.First
	restPred := namespace.RDF.Rest

	for {
		if cur == nil {
			break
		}
		ck := term.TermKey(cur)
		if ck == nilKey {
			break
		}
		if _, seen := visited[ck]; seen {
			break // cycle
		}
		visited[ck] = struct{}{}

		node, ok := cur.(term.Subject)
		if !ok {
			break
		}

		// Get rdf:first value
		var firstVal term.Term
		stop.triples(g, node, &firstPred, nil, func(t term.Triple) bool {
			firstVal = t.Object
			return false // take first
		})
		if firstVal == nil {
			break
		}
		result = append(result, firstVal)

		// Get rdf:rest value
		var restVal term.Term
		stop.triples(g, node, &restPred, nil, func(t term.Triple) bool {
			restVal = t.Object
			return false
		})
		cur = restVal
	}

	return result
}
