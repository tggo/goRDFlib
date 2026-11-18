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
	if graphPreBoundHook != nil && !offGraphPreBound {
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
	if bound, ok := pre[varName]; ok && !offGraphPreBound {
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
	// When the row does not bind the graph variable, the inner pattern is
	// evaluated before it is joined with the graph name (SPARQL 1.1 §18.6):
	// a FILTER or BIND inside must not see it. Substituting the name as a
	// constant is only safe for a pattern no expression can observe.
	//
	// With offGraphPreBound set this is the plain evaluation the differential
	// test compares against: every named graph, the pattern evaluated with
	// pre alone, the graph name joined in afterwards.
	_, rowBinds := pre[varName]
	// It also only pays when the pattern mentions the variable: otherwise
	// merging it into pre per graph costs more than it restricts.
	substitute := !offGraphPreBound && (rowBinds || (expressionFree(gp.Pattern) && mentionsVar(gp.Pattern, varName)))
	var results []map[string]rdflibgo.Term
	for name, namedG := range graphs {
		if ec.poll() {
			return nil
		}
		graphURI := rdflibgo.NewURIRefUnsafe(name)
		nameRow := map[string]rdflibgo.Term{varName: graphURI}
		if !isCompatible(pre, nameRow) {
			continue // the row bound the graph variable to another graph
		}
		var inner []map[string]rdflibgo.Term
		if substitute {
			inner = evalPatternPreBound(ec, namedG, gp.Pattern, mergeBindings(pre, nameRow), prefixes, namedGraphs)
		} else {
			inner = evalPatternPreBound(ec, namedG, gp.Pattern, pre, prefixes, namedGraphs)
		}
		for _, b := range inner {
			if !isCompatible(b, nameRow) {
				continue // the pattern bound the graph variable to another term
			}
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

// mentionsVar reports whether the expression-free pattern p mentions ?name.
func mentionsVar(p Pattern, name string) bool {
	found := false
	collectRightVars(p, func(v string) {
		if v == name {
			found = true
		}
	})
	return found
}
