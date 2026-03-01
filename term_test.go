package rdflibgo

import (
	"strings"
	"testing"
)

// Ported from: rdflib.term.URIRef

func TestNewURIRef(t *testing.T) {
	u, err := NewURIRef("http://example.org/resource")
	if err != nil {
		t.Fatal(err)
	}
	if u.Value() != "http://example.org/resource" {
		t.Errorf("got %q", u.Value())
	}
}

func TestURIRefRelativeResolution(t *testing.T) {
	// Ported from: rdflib.term.URIRef with base resolution
	u, err := NewURIRef("foo", "http://example.org/bar/")
	if err != nil {
		t.Fatal(err)
	}
	if u.Value() != "http://example.org/bar/foo" {
		t.Errorf("got %q", u.Value())
	}
}

func TestURIRefInvalidChars(t *testing.T) {
	// Ported from: rdflib.term.URIRef validation
	_, err := NewURIRef("http://example.org/has space")
	if err == nil {
		t.Error("expected error for space in IRI")
	}
}

func TestURIRefN3(t *testing.T) {
	// Ported from: rdflib.term.URIRef.n3()
	u, _ := NewURIRef("http://example.org/")
	if got := u.N3(); got != "<http://example.org/>" {
		t.Errorf("got %q", got)
	}
}

func TestURIRefDefragFragment(t *testing.T) {
	// Ported from: rdflib.term.URIRef.defrag(), rdflib.term.URIRef.fragment()
	u, _ := NewURIRef("http://example.org/page#section")
	if u.Defrag().Value() != "http://example.org/page" {
		t.Errorf("defrag: got %q", u.Defrag().Value())
	}
	if u.Fragment() != "section" {
		t.Errorf("fragment: got %q", u.Fragment())
	}
}

func TestURIRefEquality(t *testing.T) {
	// Ported from: rdflib.term.URIRef.__eq__
	a, _ := NewURIRef("http://example.org/")
	b, _ := NewURIRef("http://example.org/")
	if a != b {
		t.Error("equal URIRefs should be ==")
	}
}

// Ported from: rdflib.term.BNode

func TestBNodeUnique(t *testing.T) {
	a := NewBNode()
	b := NewBNode()
	if a == b {
		t.Error("two BNodes should differ")
	}
}

func TestBNodeCustomID(t *testing.T) {
	b := NewBNode("myid")
	if b.Value() != "myid" {
		t.Errorf("got %q", b.Value())
	}
}

func TestBNodeN3(t *testing.T) {
	// Ported from: rdflib.term.BNode.n3()
	b := NewBNode("abc")
	if got := b.N3(); got != "_:abc" {
		t.Errorf("got %q", got)
	}
}

func TestBNodeGeneratedID(t *testing.T) {
	// Ported from: rdflib.term.BNode default ID generation (N + uuid hex)
	b := NewBNode()
	if !strings.HasPrefix(b.Value(), "N") {
		t.Errorf("auto-generated BNode should start with N, got %q", b.Value())
	}
	if len(b.Value()) != 33 { // "N" + 32 hex chars
		t.Errorf("expected length 33, got %d: %q", len(b.Value()), b.Value())
	}
}

func TestBNodeSkolemize(t *testing.T) {
	// Ported from: rdflib.term.BNode.skolemize()
	b := NewBNode("abc")
	s := b.Skolemize("http://example.org")
	if s.Value() != "http://example.org/.well-known/genid/abc" {
		t.Errorf("got %q", s.Value())
	}
}

// Ported from: rdflib.term.Variable

func TestVariable(t *testing.T) {
	v := NewVariable("x")
	if v.N3() != "?x" {
		t.Errorf("got %q", v.N3())
	}
	if v.String() != "?x" {
		t.Errorf("got %q", v.String())
	}
}

// Ported from: rdflib.term — Subject interface

func TestSubjectInterface(t *testing.T) {
	var s Subject
	u, _ := NewURIRef("http://example.org/")
	s = u
	_ = s
	s = NewBNode("b")
	_ = s
}
