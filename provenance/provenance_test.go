package provenance

import (
	"strings"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

func uri(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }

func TestRecordAndLookup(t *testing.T) {
	idx := NewIndex()
	idx.Triple(uri("a"), uri("p"), uri("b"), 7)

	if line, ok := idx.Line(uri("a"), uri("p"), uri("b")); !ok || line != 7 {
		t.Errorf("Line = %d, %v; want 7, true", line, ok)
	}
	if _, ok := idx.Line(uri("a"), uri("p"), uri("zzz")); ok {
		t.Error("Line reported a triple that was never recorded")
	}
	if idx.Len() != 1 {
		t.Errorf("Len = %d, want 1", idx.Len())
	}
}

func TestSubjectLineIsTheEarliest(t *testing.T) {
	idx := NewIndex()
	idx.Triple(uri("a"), uri("p"), uri("b"), 12)
	idx.Triple(uri("a"), uri("q"), uri("c"), 4)
	idx.Triple(uri("a"), uri("r"), uri("d"), 9)

	// A resource is "defined" where it is first mentioned; that is the line to
	// send a reader to when the complaint is about the resource as a whole.
	if line, ok := idx.SubjectLine(uri("a")); !ok || line != 4 {
		t.Errorf("SubjectLine = %d, %v; want 4, true", line, ok)
	}
	if _, ok := idx.SubjectLine(uri("never-seen")); ok {
		t.Error("SubjectLine reported a node that was never recorded")
	}
}

func TestDuplicateTripleKeepsTheFirstLine(t *testing.T) {
	idx := NewIndex()
	idx.Triple(uri("a"), uri("p"), uri("b"), 3)
	idx.Triple(uri("a"), uri("p"), uri("b"), 40)

	// Restating a triple does not change the graph, so there is no second thing
	// to point at.
	if line, _ := idx.Line(uri("a"), uri("p"), uri("b")); line != 3 {
		t.Errorf("Line = %d after a restated triple, want the first line 3", line)
	}
	if idx.Len() != 1 {
		t.Errorf("Len = %d, want 1 — a restated triple is still one triple", idx.Len())
	}
}

func TestNonPositiveLineIsIgnored(t *testing.T) {
	idx := NewIndex()
	idx.Triple(uri("a"), uri("p"), uri("b"), 0)
	idx.Triple(uri("a"), uri("p"), uri("c"), -1)

	// Lines are 1-based, so a zero means "no line" rather than "the first
	// line". Recording it would make Line report a triple whose line is a lie.
	if idx.Len() != 0 {
		t.Errorf("Len = %d, want 0", idx.Len())
	}
}

func TestQuadIgnoresTheGraphOnLookup(t *testing.T) {
	idx := NewIndex()
	idx.Quad(uri("a"), uri("p"), uri("b"), uri("g1"), 5)

	// A validator works on one graph and asks about a triple. Keying by graph
	// would make the lookup miss whenever a caller validates a graph assembled
	// from several named ones.
	if line, ok := idx.Line(uri("a"), uri("p"), uri("b")); !ok || line != 5 {
		t.Errorf("Line = %d, %v; want 5, true", line, ok)
	}
}

func TestLiteralsWithSameLexicalFormAreDistinct(t *testing.T) {
	idx := NewIndex()
	idx.Triple(uri("a"), uri("p"), term.NewLiteral("x"), 1)
	idx.Triple(uri("a"), uri("p"), term.NewLiteral("x", term.WithLang("en")), 2)
	idx.Triple(uri("a"), uri("p"), term.NewLiteral(1), 3)

	if idx.Len() != 3 {
		t.Fatalf("Len = %d, want 3 — a language tag and a datatype make different terms", idx.Len())
	}
	if line, _ := idx.Line(uri("a"), uri("p"), term.NewLiteral("x", term.WithLang("en"))); line != 2 {
		t.Errorf("the tagged literal resolved to line %d, want 2", line)
	}
}

// TestKeySeparatorCannotCollide guards the choice of NUL as the separator. A
// literal's lexical form goes into the key verbatim, so a printable separator
// would let a crafted literal forge another triple's key.
func TestKeySeparatorCannotCollide(t *testing.T) {
	idx := NewIndex()
	// The lexical form contains what a naive separator would be.
	idx.Triple(uri("a"), uri("p"), term.NewLiteral("b|U:http://example.org/p|c"), 1)
	idx.Triple(uri("a"), uri("p"), term.NewLiteral("other"), 2)

	if idx.Len() != 2 {
		t.Errorf("Len = %d, want 2", idx.Len())
	}
	if _, ok := idx.Line(uri("a"), uri("p"), uri("c")); ok {
		t.Error("a crafted literal forged the key of a different triple")
	}
}

func TestSplitKeyRoundTrip(t *testing.T) {
	key := TripleKey(uri("a"), uri("p"), term.NewLiteral("x"))
	s, p, o, ok := splitKey(key)
	if !ok {
		t.Fatal("splitKey rejected a key TripleKey produced")
	}
	if s != term.TermKey(uri("a")) || p != term.TermKey(uri("p")) || o != term.TermKey(term.NewLiteral("x")) {
		t.Errorf("splitKey = %q, %q, %q", s, p, o)
	}
	if _, _, _, ok := splitKey("not a key"); ok {
		t.Error("splitKey accepted a string that is not a key")
	}
}

func TestMerge(t *testing.T) {
	a := NewIndex()
	a.Triple(uri("x"), uri("p"), uri("y"), 10)
	a.Triple(uri("shared"), uri("p"), uri("v"), 8)

	b := NewIndex()
	b.Triple(uri("z"), uri("p"), uri("w"), 3)
	b.Triple(uri("shared"), uri("p"), uri("v"), 2)

	a.Merge(b)

	if a.Len() != 3 {
		t.Errorf("Len = %d, want 3", a.Len())
	}
	if line, _ := a.Line(uri("z"), uri("p"), uri("w")); line != 3 {
		t.Errorf("merged triple resolved to line %d, want 3", line)
	}
	if line, _ := a.Line(uri("shared"), uri("p"), uri("v")); line != 2 {
		t.Errorf("a triple in both indexes resolved to line %d, want the earlier 2", line)
	}

	a.Merge(nil) // must not panic
	a.Merge(a)   // must not deadlock on its own lock
}

// TestParserIntegration is the check that matters: the method values really do
// satisfy the parsers' ProvenanceHandler, and the lines they report are the
// lines in the file.
func TestParserIntegration(t *testing.T) {
	const doc = `@prefix ex: <http://example.org/> .

ex:a ex:p ex:b .
ex:a ex:q "text" ;
     ex:r 42 .
`
	g := graph.NewGraph()
	idx := NewIndex()
	if err := turtle.Parse(g, strings.NewReader(doc), turtle.WithProvenance(idx.Triple)); err != nil {
		t.Fatalf("parse: %v", err)
	}

	cases := []struct {
		o    term.Term
		p    term.URIRef
		want int
	}{
		{uri("b"), uri("p"), 3},
		{term.NewLiteral("text"), uri("q"), 4},
		{term.NewLiteral(42), uri("r"), 5},
	}
	for _, tc := range cases {
		line, ok := idx.Line(uri("a"), tc.p, tc.o)
		if !ok {
			t.Errorf("no line for %s %s", tc.p.N3(), tc.o.N3())
			continue
		}
		if line != tc.want {
			t.Errorf("%s resolved to line %d, want %d", tc.o.N3(), line, tc.want)
		}
	}
	if line, _ := idx.SubjectLine(uri("a")); line != 3 {
		t.Errorf("SubjectLine = %d, want 3", line)
	}
}

func TestConcurrentUse(t *testing.T) {
	idx := NewIndex()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 1; i <= 100; i++ {
				idx.Triple(uri("s"), uri("p"), term.NewLiteral(w*1000+i), i)
				idx.Line(uri("s"), uri("p"), term.NewLiteral(i))
				idx.SubjectLine(uri("s"))
			}
		}(w)
	}
	wg.Wait()

	if idx.Len() != 800 {
		t.Errorf("Len = %d, want 800", idx.Len())
	}
}

// TestRecord covers the entry point for a caller feeding the index from
// something other than one of the bundled parsers — a custom reader, or a
// format this repository does not ship.
func TestRecord(t *testing.T) {
	idx := NewIndex()
	idx.Record(uri("s"), uri("p"), term.NewLiteral("v"), 11)

	if line, ok := idx.Line(uri("s"), uri("p"), term.NewLiteral("v")); !ok || line != 11 {
		t.Errorf("Line = %d, %v; want 11, true", line, ok)
	}
	if line, ok := idx.SubjectLine(uri("s")); !ok || line != 11 {
		t.Errorf("SubjectLine = %d, %v; want 11, true", line, ok)
	}

	// Record obeys the same rule as the handlers: a non-positive line means
	// "no line" and is dropped rather than stored as if it were line zero.
	idx.Record(uri("s2"), uri("p"), term.NewLiteral("v"), 0)
	if _, ok := idx.SubjectLine(uri("s2")); ok {
		t.Error("Record stored a non-positive line")
	}
}

// TestSubjectKey checks that the exported key helper agrees with the lookup it
// exists to feed; a caller holding a term uses it to avoid rebuilding one.
func TestSubjectKey(t *testing.T) {
	idx := NewIndex()
	idx.Triple(uri("s"), uri("p"), uri("o"), 6)

	line, ok := idx.SubjectLineOfKey(SubjectKey(uri("s")))
	if !ok || line != 6 {
		t.Errorf("SubjectLineOfKey(SubjectKey(...)) = %d, %v; want 6, true", line, ok)
	}
	if _, ok := idx.SubjectLineOfKey(SubjectKey(uri("nope"))); ok {
		t.Error("a key for an unrecorded node resolved")
	}
}
