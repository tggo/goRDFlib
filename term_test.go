package rdflibgo

import (
	"strings"
	"testing"
)

// --- URIRef tests ---
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
	u, err := NewURIRefWithBase("foo", "http://example.org/bar/")
	if err != nil {
		t.Fatal(err)
	}
	if u.Value() != "http://example.org/bar/foo" {
		t.Errorf("got %q", u.Value())
	}
}

func TestNewURIRefWithBaseErrors(t *testing.T) {
	_, err := NewURIRefWithBase("foo", "://bad")
	if err == nil {
		t.Error("expected error for invalid base")
	}
}

func TestURIRefInvalidChars(t *testing.T) {
	tests := []string{
		"http://example.org/has space",
		"http://example.org/<bad>",
		"http://example.org/has{brace}",
		"http://example.org/pipe|here",
		"http://example.org/back\\slash",
		"http://example.org/caret^",
		"http://example.org/tick`",
		`http://example.org/quote"`,
	}
	for _, uri := range tests {
		_, err := NewURIRef(uri)
		if err == nil {
			t.Errorf("expected error for %q", uri)
		}
	}
}

func TestURIRefN3(t *testing.T) {
	u, _ := NewURIRef("http://example.org/")
	if got := u.N3(); got != "<http://example.org/>" {
		t.Errorf("got %q", got)
	}
}

func TestURIRefN3WithNamespaceManager(t *testing.T) {
	u, _ := NewURIRef("http://example.org/Thing")
	store := NewMemoryStore()
	store.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	mgr := NewNSManager(store)

	got := u.N3(mgr)
	if got != "ex:Thing" {
		t.Errorf("expected ex:Thing, got %q", got)
	}
}

func TestURIRefString(t *testing.T) {
	u, _ := NewURIRef("http://example.org/")
	if u.String() != "http://example.org/" {
		t.Errorf("got %q", u.String())
	}
}

func TestURIRefDefragFragment(t *testing.T) {
	u, _ := NewURIRef("http://example.org/page#section")
	if u.Defrag().Value() != "http://example.org/page" {
		t.Errorf("defrag: got %q", u.Defrag().Value())
	}
	if u.Fragment() != "section" {
		t.Errorf("fragment: got %q", u.Fragment())
	}
}

func TestURIRefDefragNoFragment(t *testing.T) {
	u, _ := NewURIRef("http://example.org/page")
	if u.Defrag().Value() != "http://example.org/page" {
		t.Errorf("defrag without #: got %q", u.Defrag().Value())
	}
	if u.Fragment() != "" {
		t.Errorf("fragment without #: got %q", u.Fragment())
	}
}

func TestURIRefEquality(t *testing.T) {
	a, _ := NewURIRef("http://example.org/")
	b, _ := NewURIRef("http://example.org/")
	if a != b {
		t.Error("equal URIRefs should be ==")
	}
}

func TestURIRefEqual(t *testing.T) {
	a, _ := NewURIRef("http://example.org/a")
	b, _ := NewURIRef("http://example.org/a")
	c, _ := NewURIRef("http://example.org/c")
	if !a.Equal(b) {
		t.Error("same URI should be Equal")
	}
	if a.Equal(c) {
		t.Error("different URI should not be Equal")
	}
	if a.Equal(NewBNode("x")) {
		t.Error("URIRef should not Equal BNode")
	}
	if a.Equal(NewLiteral("x")) {
		t.Error("URIRef should not Equal Literal")
	}
}

func TestURIRefTermType(t *testing.T) {
	// termType is unexported; verify via N3 output instead
	u, _ := NewURIRef("http://example.org/")
	if u.N3() != "<http://example.org/>" {
		t.Errorf("unexpected N3: %q", u.N3())
	}
}

// --- BNode tests ---
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
	b := NewBNode("abc")
	if got := b.N3(); got != "_:abc" {
		t.Errorf("got %q", got)
	}
}

func TestBNodeString(t *testing.T) {
	b := NewBNode("abc")
	if b.String() != "abc" {
		t.Errorf("got %q", b.String())
	}
}

func TestBNodeGeneratedID(t *testing.T) {
	b := NewBNode()
	if !strings.HasPrefix(b.Value(), "N") {
		t.Errorf("auto-generated BNode should start with N, got %q", b.Value())
	}
	if len(b.Value()) != 33 { // "N" + 32 hex chars
		t.Errorf("expected length 33, got %d: %q", len(b.Value()), b.Value())
	}
}

func TestBNodeSkolemize(t *testing.T) {
	b := NewBNode("abc")
	s := b.Skolemize("http://example.org")
	if s.Value() != "http://example.org/.well-known/genid/abc" {
		t.Errorf("got %q", s.Value())
	}
	// With trailing slash
	s2 := b.Skolemize("http://example.org/")
	if s2.Value() != "http://example.org/.well-known/genid/abc" {
		t.Errorf("got %q", s2.Value())
	}
}

func TestBNodeEqual(t *testing.T) {
	a := NewBNode("x")
	b := NewBNode("x")
	c := NewBNode("y")
	if !a.Equal(b) {
		t.Error("same id should be Equal")
	}
	if a.Equal(c) {
		t.Error("different id should not be Equal")
	}
	u, _ := NewURIRef("http://example.org/x")
	if a.Equal(u) {
		t.Error("BNode should not Equal URIRef")
	}
}

func TestBNodeTermType(t *testing.T) {
	b := NewBNode("x")
	if b.N3() != "_:x" {
		t.Errorf("unexpected N3: %q", b.N3())
	}
}

// --- Variable tests ---
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

func TestVariableEqual(t *testing.T) {
	a := NewVariable("x")
	b := NewVariable("x")
	c := NewVariable("y")
	if !a.Equal(b) {
		t.Error("same name should be Equal")
	}
	if a.Equal(c) {
		t.Error("different name should not be Equal")
	}
	if a.Equal(NewBNode("x")) {
		t.Error("Variable should not Equal BNode")
	}
}

func TestVariableTermType(t *testing.T) {
	v := NewVariable("x")
	if v.N3() != "?x" {
		t.Errorf("unexpected N3: %q", v.N3())
	}
}

// --- Subject interface ---

func TestSubjectInterface(t *testing.T) {
	var s Subject
	u, _ := NewURIRef("http://example.org/")
	s = u
	_ = s
	s = NewBNode("b")
	_ = s
}

// --- Benchmarks ---

func BenchmarkNewURIRef(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewURIRef("http://example.org/resource")
	}
}

func BenchmarkNewBNode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewBNode()
	}
}

func BenchmarkURIRefN3(b *testing.B) {
	u, _ := NewURIRef("http://example.org/resource")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.N3()
	}
}

func BenchmarkTermKey(b *testing.B) {
	u, _ := NewURIRef("http://example.org/resource")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		termKey(u)
	}
}
