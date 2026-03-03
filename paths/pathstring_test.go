// Internal test file (package paths, not paths_test) to cover unexported pathString methods.
package paths

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

func TestPathStringInvPath(t *testing.T) {
	p, _ := term.NewURIRef("http://example.org/p")
	inv := Inv(AsPath(p))
	s := inv.pathString()
	if !strings.Contains(s, "^(") {
		t.Errorf("InvPath.pathString() = %q, want to contain '^('", s)
	}
}

func TestPathStringSequencePath(t *testing.T) {
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")
	seq := Sequence(AsPath(p), AsPath(q))
	s := seq.pathString()
	if !strings.Contains(s, "/") {
		t.Errorf("SequencePath.pathString() = %q, want to contain '/'", s)
	}
}

func TestPathStringAlternativePath(t *testing.T) {
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")
	alt := Alternative(AsPath(p), AsPath(q))
	s := alt.pathString()
	if !strings.Contains(s, "|") {
		t.Errorf("AlternativePath.pathString() = %q, want to contain '|'", s)
	}
}

func TestPathStringMulPath(t *testing.T) {
	p, _ := term.NewURIRef("http://example.org/p")

	star := ZeroOrMore(AsPath(p))
	if s := star.pathString(); !strings.Contains(s, "*") {
		t.Errorf("ZeroOrMore.pathString() = %q, want '*'", s)
	}

	plus := OneOrMore(AsPath(p))
	if s := plus.pathString(); !strings.Contains(s, "+") {
		t.Errorf("OneOrMore.pathString() = %q, want '+'", s)
	}

	qmark := ZeroOrOne(AsPath(p))
	if s := qmark.pathString(); !strings.Contains(s, "?") {
		t.Errorf("ZeroOrOne.pathString() = %q, want '?'", s)
	}
}

func TestPathStringNegatedPath(t *testing.T) {
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")
	neg := Negated(p, q)
	s := neg.pathString()
	if !strings.Contains(s, "!(") {
		t.Errorf("NegatedPath.pathString() = %q, want to contain '!('", s)
	}
}

func TestPathStringURIRefPath(t *testing.T) {
	p, _ := term.NewURIRef("http://example.org/p")
	up := AsPath(p)
	s := up.pathString()
	if !strings.Contains(s, "example.org/p") {
		t.Errorf("URIRefPath.pathString() = %q, want to contain 'example.org/p'", s)
	}
}
