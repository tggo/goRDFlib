package rdflibgo_test

import (
	"strings"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/turtle"
)

func TestPathBackwardEval(t *testing.T) {
	g := makePathGraphExt(t)
	c, _ := NewURIRef("http://example.org/c")
	p, _ := NewURIRef("http://example.org/p")

	path := OneOrMore(AsPath(p))
	pairs := collectPairsExt(g, path, nil, c)
	if len(pairs) < 2 {
		t.Errorf("expected >=2 backward pairs, got %d: %v", len(pairs), pairs)
	}
}

func TestZeroOrMoreNoConstraints(t *testing.T) {
	g := makePathGraphExt(t)
	p, _ := NewURIRef("http://example.org/p")

	path := ZeroOrMore(AsPath(p))
	pairs := collectPairsExt(g, path, nil, nil)
	if len(pairs) < 4 {
		t.Errorf("expected >=4, got %d", len(pairs))
	}
}

func TestNegatedPathNoMatch(t *testing.T) {
	g := makePathGraphExt(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")
	q, _ := NewURIRef("http://example.org/q")

	path := Negated(p, q)
	pairs := collectPairsExt(g, path, a, nil)
	if len(pairs) != 0 {
		t.Errorf("expected 0 when all predicates excluded, got %d", len(pairs))
	}
}

func TestResourceGraphAndIdentifier(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	r := NewResource(g, s)
	if r.Graph() != g {
		t.Error("wrong graph")
	}
	if r.Identifier() != s {
		t.Error("wrong identifier")
	}
}

func TestNSManagerNamespacesPathsCov(t *testing.T) {
	store := NewMemoryStore()
	store.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	mgr := NewNSManager(store)
	count := 0
	mgr.Namespaces()(func(string, URIRef) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestNSManagerBindPathsCov(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewNSManager(store)
	ns, _ := NewURIRef("http://example.org/")
	mgr.Bind("ex", ns, true)
	ns2, _ := NewURIRef("http://other.org/")
	mgr.Bind("ex", ns2, false)
	got, ok := store.Namespace("ex")
	if !ok || got != ns {
		t.Errorf("override=false should keep original, got %v", got)
	}
}

func TestNamespaceBase(t *testing.T) {
	ns := NewNamespace("http://example.org/")
	if ns.Base() != "http://example.org/" {
		t.Error("wrong base")
	}
}

func TestMemoryStoreContextAware(t *testing.T) {
	s := NewMemoryStore()
	if s.ContextAware() {
		t.Error("should not be context aware")
	}
	if s.TransactionAware() {
		t.Error("should not be transaction aware")
	}
}

func TestTurtleParserSPARQLBase(t *testing.T) {
	g := NewGraph()
	turtle.Parse(g, strings.NewReader(`
		BASE <http://example.org/>
		<s> <p> "hello" .
	`))
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestRDFXMLParserParseLiteral(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:content rdf:parseType="Literal"><b>bold</b></ex:content>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	rdfxml.Parse(g, strings.NewReader(input))
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}
