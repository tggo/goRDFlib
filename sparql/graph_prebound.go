package sparql

import (
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// evalGraphPatternPreBound is a GRAPH pattern on the pre-bound path, such as
// in an EXISTS body with the outer row pre-bound. The inner pattern is
// evaluated with the pre-bound values as constants, in the one graph a
// constant or pre-bound graph name selects, or else in each named graph. Its
// solutions are those of evalGraphPattern restricted to pre, without matching
// the inner pattern in full in every named graph first.
func evalGraphPatternPreBound(ec *evalCtx, gp *GraphPattern, pre map[string]rdflibgo.Term, prefixes map[string]string, namedGraphs map[string]*rdflibgo.Graph) []map[string]rdflibgo.Term {
	if graphPreBoundHook != nil {
		graphPreBoundHook()
	}
	if !strings.HasPrefix(gp.Name, "?") {
		resolved := resolvePatternTerm(gp.Name, nil, prefixes)
		if resolved == nil {
			return nil
		}
		iri := resolved.String()
		if u, ok := resolved.(rdflibgo.URIRef); ok {
			iri = u.Value()
		}
		namedG, ok := namedGraphs[iri]
		if !ok {
			return nil
		}
		return evalPatternPreBound(ec, namedG, gp.Pattern, pre, prefixes, namedGraphs)
	}

	varName := gp.Name[1:]
	graphs := namedGraphs
	if bound, ok := pre[varName]; ok {
		// A graph name is an IRI, so a row that binds the variable to
		// anything else, or to a graph the dataset doesn't have, matches
		// nothing.
		u, isIRI := bound.(rdflibgo.URIRef)
		if !isIRI {
			return nil
		}
		namedG, ok := namedGraphs[u.Value()]
		if !ok {
			return nil
		}
		graphs = map[string]*rdflibgo.Graph{u.Value(): namedG}
	}
	var results []map[string]rdflibgo.Term
	for name, namedG := range graphs {
		if ec.poll() {
			return nil
		}
		graphURI := rdflibgo.NewURIRefUnsafe(name)
		inner := evalPatternPreBound(ec, namedG, gp.Pattern, mergeBindings(pre, map[string]rdflibgo.Term{varName: graphURI}), prefixes, namedGraphs)
		for _, b := range inner {
			nb := copyBindings(b)
			nb[varName] = graphURI
			results = append(results, nb)
			if ec.produced(1) {
				return nil
			}
		}
	}
	return results
}
