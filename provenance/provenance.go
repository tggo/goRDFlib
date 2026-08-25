// Package provenance records where each triple came from in the source text,
// so that a later stage — SHACL validation, a schema check, a linter — can
// point at a line instead of at an abstract node.
//
// The parsers already hand out this information: nt, nq, turtle and trig each
// take a WithProvenance option that calls back with the source line for every
// triple they add. What was missing is somewhere to put it and a way for a
// consumer to ask. Index is that, and its Triple and Quad methods have exactly
// the signatures those options expect:
//
//	idx := provenance.NewIndex()
//	turtle.Parse(g, r, "", turtle.WithProvenance(idx.Triple))
//	nq.Parse(ds, r, "", nq.WithProvenance(idx.Quad))
//
//	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
//	for _, res := range report.Results {
//		fmt.Printf("%s:%d: %s\n", file, res.SourceLine, res.ResultMessages)
//	}
//
// # What a line means
//
// A line is recorded for the triple as a whole. Turtle can spread one triple
// over several lines, and predicate-object lists put many triples on one; the
// parsers report the line the triple was completed on, which is the line a
// reader looks at to find the value that is wrong.
//
// A triple stated twice keeps the first line. Repeating a triple does not
// change the graph, so there is no second thing to point at.
//
// # Graphs
//
// Quad records the graph a quad landed in, but lookups ignore it. A validator
// works on one graph at a time and asks about a triple; carrying the graph into
// the key would only make the lookup miss whenever the caller validates a graph
// assembled from several sources.
//
// # Thread safety
//
// Safe for concurrent use. Parsing usually records from one goroutine and
// validation reads from another, and several validations may read at once.
package provenance

import (
	"strings"
	"sync"

	"github.com/tggo/goRDFlib/term"
)

// Index maps triples to the source lines they were parsed from.
// Safe for concurrent use.
type Index struct {
	mu       sync.RWMutex
	triples  map[string]int
	subjects map[string]int
}

// NewIndex returns an empty Index.
func NewIndex() *Index {
	return &Index{
		triples:  make(map[string]int),
		subjects: make(map[string]int),
	}
}

// keySeparator joins the three term keys. It is a NUL because term.TermKey may
// contain any other byte — a literal's lexical form is embedded verbatim — and
// a separator that can occur inside a component would let two different triples
// collide on one key.
const keySeparator = "\x00"

// TripleKey builds the lookup key for a triple from the terms themselves.
func TripleKey(s term.Term, p term.Term, o term.Term) string {
	return term.TermKey(s) + keySeparator + term.TermKey(p) + keySeparator + term.TermKey(o)
}

// Triple records a triple's source line. Its signature matches the
// ProvenanceHandler of the nt and turtle parsers, so it can be passed directly:
//
//	turtle.Parse(g, r, "", turtle.WithProvenance(idx.Triple))
func (i *Index) Triple(s term.Subject, p term.URIRef, o term.Term, line int) {
	i.record(TripleKey(s, p, o), term.TermKey(s), line)
}

// Quad records a quad's source line. Its signature matches the
// ProvenanceHandler of the nq and trig parsers:
//
//	trig.Parse(ds, r, "", trig.WithProvenance(idx.Quad))
//
// The graph is accepted because the handler supplies it, and ignored because
// lookups are per triple. See the package comment.
func (i *Index) Quad(s term.Subject, p term.URIRef, o term.Term, _ term.Term, line int) {
	i.record(TripleKey(s, p, o), term.TermKey(s), line)
}

// Record stores a line for an arbitrary triple, for a caller feeding the index
// from something other than one of the bundled parsers.
func (i *Index) Record(s term.Term, p term.Term, o term.Term, line int) {
	i.record(TripleKey(s, p, o), term.TermKey(s), line)
}

func (i *Index) record(tripleKey, subjectKey string, line int) {
	if line <= 0 {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, seen := i.triples[tripleKey]; !seen {
		i.triples[tripleKey] = line
	}
	// The subject's line is the earliest line mentioning it, which is where a
	// reader would look for "the definition of this resource". It is the answer
	// for a result that blames a node rather than a single triple — a failed
	// sh:minCount has no offending triple, because the problem is one that is
	// absent.
	if prev, seen := i.subjects[subjectKey]; !seen || line < prev {
		i.subjects[subjectKey] = line
	}
}

// Line returns the source line of a triple, and whether it was recorded.
func (i *Index) Line(s term.Term, p term.Term, o term.Term) (int, bool) {
	return i.LineOfKey(TripleKey(s, p, o))
}

// LineOfKey is Line for a key already built with TripleKey. It exists so a
// caller holding its own term representation can compute the key once instead
// of rebuilding terms per lookup.
func (i *Index) LineOfKey(key string) (int, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	line, ok := i.triples[key]
	return line, ok
}

// SubjectLine returns the earliest line on which a node appeared as a subject.
//
// It is the fallback for anything that blames a node rather than a triple: a
// missing required property, a class that is not among the node's types, a node
// shape that does not hold.
func (i *Index) SubjectLine(s term.Term) (int, bool) {
	return i.SubjectLineOfKey(term.TermKey(s))
}

// SubjectLineOfKey is SubjectLine for a key already built with term.TermKey.
func (i *Index) SubjectLineOfKey(key string) (int, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	line, ok := i.subjects[key]
	return line, ok
}

// Len returns how many distinct triples have a recorded line.
//
// A count of zero after parsing means the provenance option never reached the
// parser — the most likely mistake, since nothing else reports it.
func (i *Index) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.triples)
}

// Merge copies other's entries into i, keeping the earlier line where both hold
// the same triple. Use it when a graph is assembled from several sources and
// each was parsed with its own index.
//
// Note that lines from different files are indistinguishable afterwards. Keep
// one index per file and merge only when the caller knows they share an origin.
func (i *Index) Merge(other *Index) {
	if other == nil || other == i {
		return
	}
	other.mu.RLock()
	triples := make(map[string]int, len(other.triples))
	for k, v := range other.triples {
		triples[k] = v
	}
	subjects := make(map[string]int, len(other.subjects))
	for k, v := range other.subjects {
		subjects[k] = v
	}
	other.mu.RUnlock()

	i.mu.Lock()
	defer i.mu.Unlock()
	for k, v := range triples {
		if prev, seen := i.triples[k]; !seen || v < prev {
			i.triples[k] = v
		}
	}
	for k, v := range subjects {
		if prev, seen := i.subjects[k]; !seen || v < prev {
			i.subjects[k] = v
		}
	}
}

// SubjectKey exposes the key SubjectLineOfKey expects, for a caller that has a
// term.Term already.
func SubjectKey(s term.Term) string { return term.TermKey(s) }

// splitKey is used only by tests and diagnostics; it is here so the key layout
// has exactly one definition.
func splitKey(key string) (s, p, o string, ok bool) {
	parts := strings.Split(key, keySeparator)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
