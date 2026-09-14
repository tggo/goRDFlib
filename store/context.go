package store

import "github.com/tggo/goRDFlib/term"

// DefaultGraphIRI is the lexical form of DefaultGraph. It is the identifier
// rdflib uses for the same purpose (DATASET_DEFAULT_GRAPH_ID), so data exchanged
// with rdflib agrees on it.
const DefaultGraphIRI = "urn:x-rdflib:default"

// DefaultGraph is the identifier of the default graph. A graph created without
// an identifier (graph.NewGraph, the default context of a Dataset) carries it,
// and passes it to its store as the context of every call.
//
// Stores treat it exactly like a nil context: both address the default graph,
// and neither is ever reported by Contexts. It exists so that an unnamed graph
// still has a non-nil identifier to hand out, without borrowing a blank node
// for the purpose — a blank node is a legal graph name (TriG `_:g { … }`), and
// folding every blank node into the default graph lost those graphs
// (rdflib #2445).
var DefaultGraph = term.NewURIRefUnsafe(DefaultGraphIRI)

// IsDefaultGraph reports whether ctx addresses the default graph: nil or
// DefaultGraph. Every other term, blank nodes included, names a graph.
//
// A Store implementation should route every context through this function
// rather than test for nil itself.
func IsDefaultGraph(ctx term.Term) bool {
	if ctx == nil {
		return true
	}
	u, ok := ctx.(term.URIRef)
	return ok && u.Value() == DefaultGraphIRI
}
