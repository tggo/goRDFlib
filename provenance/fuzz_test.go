package provenance

import (
	"testing"

	"github.com/tggo/goRDFlib/term"
)

// FuzzIndexKeyCollisions checks the one property the whole index rests on: two
// different triples must never share a key.
//
// The key is built by concatenating term keys, and a literal's lexical form goes
// into its term key verbatim — so a lexical form containing whatever separates
// the components could otherwise forge another triple's key and make a lookup
// report a line belonging to something else.
func FuzzIndexKeyCollisions(f *testing.F) {
	f.Add("plain", "other")
	f.Add("a\x00b", "ab")
	f.Add("U:http://example.org/p", "x")
	f.Add("b\x00U:http://example.org/p\x00c", "b")
	f.Add("", "\x00")
	f.Add("\x00\x00\x00", "")

	subj := term.NewURIRefUnsafe("http://example.org/s")
	pred := term.NewURIRefUnsafe("http://example.org/p")

	f.Fuzz(func(t *testing.T, first, second string) {
		if first == second {
			t.Skip()
		}
		idx := NewIndex()
		idx.Triple(subj, pred, term.NewLiteral(first), 1)
		idx.Triple(subj, pred, term.NewLiteral(second), 2)

		if idx.Len() != 2 {
			t.Fatalf("two distinct literals collapsed to %d entries: %q vs %q",
				idx.Len(), first, second)
		}
		if line, ok := idx.Line(subj, pred, term.NewLiteral(first)); !ok || line != 1 {
			t.Errorf("%q resolved to line %d (%v), want 1", first, line, ok)
		}
		if line, ok := idx.Line(subj, pred, term.NewLiteral(second)); !ok || line != 2 {
			t.Errorf("%q resolved to line %d (%v), want 2", second, line, ok)
		}
	})
}

// FuzzTripleKeySplit checks that a key can always be taken apart again, which is
// what makes the layout debuggable and the collision argument checkable.
func FuzzTripleKeySplit(f *testing.F) {
	f.Add("http://example.org/s", "http://example.org/p", "value")
	f.Add("", "", "")
	f.Add("s", "p", "o\x00with\x00nuls")

	f.Fuzz(func(t *testing.T, s, p, o string) {
		key := TripleKey(
			term.NewURIRefUnsafe(s),
			term.NewURIRefUnsafe(p),
			term.NewLiteral(o),
		)
		gotS, gotP, _, ok := splitKey(key)
		// An object containing NUL splits into more than three parts; that is
		// expected and is exactly why the subject and predicate — which cannot
		// contain one, being IRIs — come first.
		if !ok {
			return
		}
		if gotS != term.TermKey(term.NewURIRefUnsafe(s)) {
			t.Errorf("subject key round-tripped as %q", gotS)
		}
		if gotP != term.TermKey(term.NewURIRefUnsafe(p)) {
			t.Errorf("predicate key round-tripped as %q", gotP)
		}
	})
}

// FuzzSubjectLineIsTheMinimum checks that SubjectLine is the smallest line
// recorded for a node, whatever order the lines arrive in — a parser is free to
// revisit a subject, and the earliest mention is the one a reader calls its
// definition.
func FuzzSubjectLineIsTheMinimum(f *testing.F) {
	f.Add(3, 1, 2)
	f.Add(1, 1, 1)
	f.Add(0, 5, -3)
	f.Add(1<<30, 2, 3)

	subj := term.NewURIRefUnsafe("http://example.org/s")
	pred := term.NewURIRefUnsafe("http://example.org/p")

	f.Fuzz(func(t *testing.T, a, b, c int) {
		idx := NewIndex()
		want := 0
		for i, line := range []int{a, b, c} {
			idx.Triple(subj, pred, term.NewLiteral(i), line)
			if line > 0 && (want == 0 || line < want) {
				want = line
			}
		}

		got, ok := idx.SubjectLine(subj)
		if want == 0 {
			if ok {
				t.Errorf("SubjectLine = %d for lines %d/%d/%d, none of which is positive", got, a, b, c)
			}
			return
		}
		if !ok || got != want {
			t.Errorf("SubjectLine = %d (%v) for lines %d/%d/%d, want %d", got, ok, a, b, c, want)
		}
	})
}
