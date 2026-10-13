package store

import (
	"context"
	"iter"

	"github.com/tggo/goRDFlib/term"
)

// Store is the abstract interface for RDF triple storage backends.
//
// Context conventions, which every method below follows and which the shipped
// backends all implement:
//
//   - A nil graph means the default graph. It does NOT mean "all graphs":
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
// Every method that reads or writes takes a context.Context and reports
// failure as an error; a backend that cannot reach its data says so rather
// than answering as if the graph were empty. A call made with a done context
// returns its error and changes nothing. An iterator that fails yields one
// (zero, err) pair and stops.
//
// Iteration order is unspecified and need not be stable between calls. Terms
// are serialized with term.TermKey and restored with term.TermFromKey.
//
// Implementations outside this module are expected to pass the shared
// conformance suite in store/storetest, which pins the behaviour described
// here.
type Store interface {
	// Add inserts a triple into the store, associated with the given context.
	Add(ctx context.Context, triple term.Triple, graph term.Term) error

	// AddN batch-adds quads (triple + context).
	AddN(ctx context.Context, quads []term.Quad) error

	// Remove deletes triples matching the pattern from the given context.
	// A nil context or DefaultGraph means the default graph.
	Remove(ctx context.Context, pattern term.TriplePattern, graph term.Term) error

	// Set atomically removes all triples matching (s, p, *) and adds (s, p, o)
	// under a single lock, preventing concurrent callers from observing an
	// intermediate state where the old value is removed but the new one is not
	// yet added.
	Set(ctx context.Context, triple term.Triple, graph term.Term) error

	// Triples returns an iterator over triples matching the pattern in the given context.
	Triples(ctx context.Context, pattern term.TriplePattern, graph term.Term) TripleIterator

	// Len returns the number of triples in the given context.
	// A nil context or DefaultGraph means the default graph.
	Len(ctx context.Context, graph term.Term) (int, error)

	// Contexts returns an iterator over all contexts, optionally filtered by a triple.
	Contexts(ctx context.Context, triple *term.Triple) TermIterator

	// Bind associates a prefix with a namespace URI.
	Bind(ctx context.Context, prefix string, namespace term.URIRef) error

	// Namespace returns the namespace URI for a prefix.
	Namespace(ctx context.Context, prefix string) (term.URIRef, bool, error)

	// Prefix returns the prefix for a namespace URI.
	Prefix(ctx context.Context, namespace term.URIRef) (string, bool, error)

	// Namespaces returns an iterator over all (prefix, namespace) bindings.
	Namespaces(ctx context.Context) NamespaceIterator

	// ContextAware reports whether this store supports named graphs.
	ContextAware() bool

	// TransactionAware reports whether this store supports transactions.
	TransactionAware() bool
}

// TripleIterator yields triples. An iteration that fails yields one
// (zero, err) pair and stops; callers stop at the first non-nil error.
type TripleIterator = iter.Seq2[term.Triple, error]

// TermIterator yields terms, with errors as TripleIterator does.
type TermIterator = iter.Seq2[term.Term, error]

// TermPair is two terms yielded together, such as a subject and an object.
type TermPair struct {
	First, Second term.Term
}

// TermPairIterator yields term pairs, with errors as TripleIterator does.
type TermPairIterator = iter.Seq2[TermPair, error]

// NamespaceBinding is one prefix and the namespace it names.
type NamespaceBinding struct {
	Prefix    string
	Namespace term.URIRef
}

// NamespaceIterator yields namespace bindings, with errors as TripleIterator
// does.
type NamespaceIterator = iter.Seq2[NamespaceBinding, error]

// ErrorIterator returns an iterator that yields err once. A store returns it
// when an iteration fails before it has begun.
func ErrorIterator[T any](err error) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		yield(zero, err)
	}
}

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
	TriplesWithLimit(ctx context.Context, pattern term.TriplePattern, graph term.Term, limit, offset int) TripleIterator

	// Count returns the number of triples matching the pattern without
	// materializing them.
	Count(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error)

	// Exists checks whether at least one triple matches the pattern.
	Exists(ctx context.Context, pattern term.TriplePattern, graph term.Term) (bool, error)
}

// SnapshotStore is an optional interface for stores that can serve a series of
// reads from one consistent snapshot. A query makes many small reads — a
// nested loop looks up every match of one pattern in the next — and a store
// that opens a transaction per read pays for that on every lookup; under
// concurrent queries Badger spent most of its time in transaction bookkeeping
// rather than reading.
//
// ReadSnapshot returns a read-only Store over the data as of the call, and a
// release function that must be called when the reads are done. The snapshot
// is not safe for concurrent use, and writing to it is a programming error.
type SnapshotStore interface {
	ReadSnapshot(ctx context.Context) (snapshot Store, release func(), err error)
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
	Cardinality(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error)
}
