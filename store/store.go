package store

import (
	"iter"

	"github.com/tggo/goRDFlib/term"
)

// Store is the abstract interface for RDF triple storage backends.
//
// Context conventions, which every method below follows and which the shipped
// backends all implement:
//
//   - A nil context means the default graph. It does NOT mean "all graphs":
//     Len(nil) counts the default graph, Triples(pat, nil) reads it, and
//     Remove(pat, nil) deletes from it.
//   - DefaultGraph means the default graph as well. It is the identifier of an
//     unnamed Graph, which passes it straight through; test a context with
//     IsDefaultGraph rather than against nil.
//   - Every other term names a graph, a blank node included: TriG and N-Quads
//     name graphs with blank nodes, and treating them as the default graph lost
//     those graphs. A backend whose protocol cannot express a blank-node graph
//     name (SPARQL) documents that as a limitation.
//   - Contexts reports named graphs only; the default graph is not a context.
//
// Iteration order is unspecified and need not be stable between calls. Terms
// are serialized with term.TermKey and restored with term.TermFromKey.
//
// Implementations outside this module are expected to pass the shared
// conformance suite in store/storetest, which pins the behaviour described
// here.
type Store interface {
	// Add inserts a triple into the store, associated with the given context.
	Add(triple term.Triple, context term.Term)

	// AddN batch-adds quads (triple + context).
	AddN(quads []term.Quad)

	// Remove deletes triples matching the pattern from the given context.
	// A nil context or DefaultGraph means the default graph.
	Remove(pattern term.TriplePattern, context term.Term)

	// Set atomically removes all triples matching (s, p, *) and adds (s, p, o)
	// under a single lock, preventing concurrent callers from observing an
	// intermediate state where the old value is removed but the new one is not
	// yet added.
	Set(triple term.Triple, context term.Term)

	// Triples returns an iterator over triples matching the pattern in the given context.
	Triples(pattern term.TriplePattern, context term.Term) TripleIterator

	// Len returns the number of triples in the given context.
	// A nil context or DefaultGraph means the default graph.
	Len(context term.Term) int

	// Contexts returns an iterator over all contexts, optionally filtered by a triple.
	Contexts(triple *term.Triple) TermIterator

	// Bind associates a prefix with a namespace URI.
	Bind(prefix string, namespace term.URIRef)

	// Namespace returns the namespace URI for a prefix.
	Namespace(prefix string) (term.URIRef, bool)

	// Prefix returns the prefix for a namespace URI.
	Prefix(namespace term.URIRef) (string, bool)

	// Namespaces returns an iterator over all (prefix, namespace) bindings.
	Namespaces() NamespaceIterator

	// ContextAware reports whether this store supports named graphs.
	ContextAware() bool

	// TransactionAware reports whether this store supports transactions.
	TransactionAware() bool
}

// TripleIterator is an iterator over triples, compatible with range-over-func (Go 1.23+).
type TripleIterator = iter.Seq[term.Triple]

// TermIterator is an iterator over terms, compatible with range-over-func (Go 1.23+).
type TermIterator = iter.Seq[term.Term]

// TermPairIterator is an iterator over (Term, Term) pairs, compatible with range-over-func.
type TermPairIterator = iter.Seq2[term.Term, term.Term]

// NamespaceIterator yields (prefix, namespace) pairs.
type NamespaceIterator = iter.Seq2[string, term.URIRef]

// QueryableStore is an optional interface that stores can implement to support
// query pushdown optimizations (LIMIT/OFFSET, COUNT, EXISTS).
// Not safe for concurrent use unless the underlying Store is also safe.
type QueryableStore interface {
	// TriplesWithLimit returns triples matching the pattern with store-level
	// LIMIT and OFFSET applied, avoiding full materialization.
	//
	// A limit of zero or less means "no limit"; a negative offset is treated
	// as zero. Because Store promises no iteration order, paging through
	// separate calls is only consistent on a backend whose order happens to be
	// stable — MemoryStore's is not, since Go randomizes map iteration.
	TriplesWithLimit(pattern term.TriplePattern, ctx term.Term, limit, offset int) TripleIterator

	// Count returns the number of triples matching the pattern without
	// materializing them.
	Count(pattern term.TriplePattern, ctx term.Term) int

	// Exists checks whether at least one triple matches the pattern.
	Exists(pattern term.TriplePattern, ctx term.Term) bool
}

// CardinalityStore is an optional interface for stores that can report how
// many triples match a pattern without visiting the matches, typically from
// index sizes. The SPARQL engine uses it to plan join order; a store without
// it is probed through Triples instead, which costs about as much per match as
// evaluating the pattern.
//
// Cardinality must equal the number of triples Triples yields for the same
// pattern and context. Implement it only if its cost does not grow with the
// number of matches; otherwise QueryableStore.Count already covers counting.
type CardinalityStore interface {
	Cardinality(pattern term.TriplePattern, ctx term.Term) int
}
