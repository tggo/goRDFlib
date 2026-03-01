package integration_test

import (
	"strings"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/turtle"
)

// --- SPARQL: triple-quoted strings ---

func TestSPARQLTripleQuotedString(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/desc")
	g.Add(s, p, NewLiteral("line1\nline2"))

	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?d WHERE { ex:s ex:desc ?d }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 || !strings.Contains(r.Bindings[0]["d"].String(), "\n") {
		t.Errorf("expected multiline, got %v", r.Bindings)
	}
}

// --- SPARQL: typed literal in WHERE ---

func TestSPARQLTypedLiteralInWhere(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Bind("xsd", NewURIRefUnsafe(XSDNamespace))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral("42", WithDatatype(XSDInteger)))

	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:val 42 }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// --- SPARQL: prefixed name in patterns ---

func TestSPARQLPrefixedNameInPattern(t *testing.T) {
	g := makeSPARQLGraphExt(t)
	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:knows ex:Bob }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// --- InvPath eval with nil obj ---

func TestInvPathEvalNilObj(t *testing.T) {
	g := makePathGraphExt(t)
	p, _ := NewURIRef("http://example.org/p")
	a, _ := NewURIRef("http://example.org/a")

	inv := Inv(AsPath(p))
	pairs := collectPairsExt(g, inv, a, nil)
	if len(pairs) != 0 {
		t.Errorf("expected 0, got %d: %v", len(pairs), pairs)
	}
}

func TestInvPathEvalWithObj(t *testing.T) {
	g := makePathGraphExt(t)
	p, _ := NewURIRef("http://example.org/p")
	a, _ := NewURIRef("http://example.org/a")
	b, _ := NewURIRef("http://example.org/b")

	inv := Inv(AsPath(p))
	pairs := collectPairsExt(g, inv, b, a)
	if len(pairs) != 1 {
		t.Errorf("expected 1, got %d: %v", len(pairs), pairs)
	}
}

// --- Turtle serializer: empty predicate list ---

func TestTurtleSerializerMultipleSubjects(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	for _, name := range []string{"Alice", "Bob"} {
		s := NewURIRefUnsafe("http://example.org/" + name)
		p, _ := NewURIRef("http://example.org/name")
		g.Add(s, p, NewLiteral(name))
	}
	var buf strings.Builder
	turtle.Serialize(g, &buf)
	if strings.Count(buf.String(), " .") < 2 {
		t.Errorf("expected 2 statements, got:\n%s", buf.String())
	}
}

// --- SPARQL unary minus ---

func TestSPARQLUnaryMinus(t *testing.T) {
	g := makeSPARQLGraphExt(t)
	r, _ := sparql.Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?neg WHERE {
			?s ex:age ?age .
			BIND(-?age AS ?neg)
			FILTER(?s = ex:Alice)
		}
	`)
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1, got %d", len(r.Bindings))
	}
	if r.Bindings[0]["neg"].String() != "-30" {
		t.Errorf("expected -30, got %s", r.Bindings[0]["neg"])
	}
}

// --- Term order: Variable ---

func TestCompareTermVariable(t *testing.T) {
	v1 := NewVariable("a")
	v2 := NewVariable("b")
	if CompareTerm(v1, v2) >= 0 {
		t.Error("a < b")
	}
	if CompareTerm(v1, v1) != 0 {
		t.Error("same variable should be 0")
	}
}

// --- testutil: failed assertion paths ---

func TestAssertGraphContainsFail(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	mt := &testing.T{}
	AssertGraphContains(mt, g, s, p, NewLiteral("nope"))
}

func TestAssertGraphLenFail(t *testing.T) {
	g := NewGraph()
	mt := &testing.T{}
	AssertGraphLen(mt, g, 99)
}
