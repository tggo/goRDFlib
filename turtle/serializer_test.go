package turtle

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Ported from: test/test_serializers/test_serializer_turtle.py

func serializeTurtle(t *testing.T, g *rdflibgo.Graph) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestTurtleBasic(t *testing.T) {
	// Ported from: rdflib turtle serializer — basic subject-predicate-object
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, o)

	out := serializeTurtle(t, g)
	if !strings.Contains(out, "ex:s") {
		t.Errorf("expected prefixed subject, got:\n%s", out)
	}
	if !strings.Contains(out, `"hello"`) {
		t.Errorf("expected literal, got:\n%s", out)
	}
	if !strings.Contains(out, "@prefix ex:") {
		t.Errorf("expected prefix declaration, got:\n%s", out)
	}
}

func TestTurtleRDFTypeShorthand(t *testing.T) {
	// Ported from: rdflib turtle serializer — "a" shorthand for rdf:type
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Person"))

	out := serializeTurtle(t, g)
	if !strings.Contains(out, " a ") {
		t.Errorf("expected 'a' shorthand, got:\n%s", out)
	}
}

func TestTurtleMultiplePredicates(t *testing.T) {
	// Ported from: rdflib turtle serializer — semicolon separator
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p1, _ := rdflibgo.NewURIRef("http://example.org/p1")
	p2, _ := rdflibgo.NewURIRef("http://example.org/p2")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p1, rdflibgo.NewLiteral("a"))
	g.Add(s, p2, rdflibgo.NewLiteral("b"))

	out := serializeTurtle(t, g)
	if !strings.Contains(out, ";") {
		t.Errorf("expected semicolon for multiple predicates, got:\n%s", out)
	}
}

func TestTurtleMultipleObjects(t *testing.T) {
	// Ported from: rdflib turtle serializer — comma separator
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, rdflibgo.NewLiteral("a"))
	g.Add(s, p, rdflibgo.NewLiteral("b"))

	out := serializeTurtle(t, g)
	if !strings.Contains(out, ",") {
		t.Errorf("expected comma for multiple objects, got:\n%s", out)
	}
}

func TestTurtleLiteralTypes(t *testing.T) {
	// Ported from: rdflib turtle serializer — literal shorthands
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	g.Add(s, p, rdflibgo.NewLiteral(42))
	g.Add(s, p, rdflibgo.NewLiteral(true))
	g.Add(s, p, rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en")))

	out := serializeTurtle(t, g)
	if !strings.Contains(out, "42") {
		t.Errorf("expected integer shorthand, got:\n%s", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("expected boolean shorthand, got:\n%s", out)
	}
	if !strings.Contains(out, `"hello"@en`) {
		t.Errorf("expected language tag, got:\n%s", out)
	}
}

func TestTurtlePrefixOnlyUsed(t *testing.T) {
	// Ported from: rdflib turtle serializer — only emit used prefixes
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Bind("unused", rdflibgo.NewURIRefUnsafe("http://unused.org/"))
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	out := serializeTurtle(t, g)
	if strings.Contains(out, "unused") {
		t.Errorf("should not emit unused prefix, got:\n%s", out)
	}
}

func TestTurtleDeterministic(t *testing.T) {
	// Ported from: test/test_turtle_sort_issue613.py — deterministic output
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	for i := 0; i < 5; i++ {
		s, _ := rdflibgo.NewURIRef(fmt.Sprintf("http://example.org/s%d", i))
		p, _ := rdflibgo.NewURIRef("http://example.org/p")
		g.Add(s, p, rdflibgo.NewLiteral(fmt.Sprintf("v%d", i)))
	}

	out1 := serializeTurtle(t, g)
	out2 := serializeTurtle(t, g)
	if out1 != out2 {
		t.Errorf("output not deterministic:\n---\n%s\n---\n%s", out1, out2)
	}
}

func TestTurtleBase(t *testing.T) {
	// Ported from: rdflib turtle serializer — @base emission
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf bytes.Buffer
	err := Serialize(g, &buf, WithBase("http://example.org/"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "@base <http://example.org/>") {
		t.Errorf("expected @base, got:\n%s", buf.String())
	}
}

func TestTurtleInlineBNode(t *testing.T) {
	// Ported from: rdflib turtle serializer — blank node inlining as [ ... ]
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	name, _ := rdflibgo.NewURIRef("http://example.org/name")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	bnode := rdflibgo.NewBNode()
	g.Add(s, p, bnode)
	g.Add(bnode, name, rdflibgo.NewLiteral("Alice"))

	out := serializeTurtle(t, g)
	if !strings.Contains(out, "[") || !strings.Contains(out, "]") {
		t.Errorf("expected inline blank node, got:\n%s", out)
	}
}

func TestTurtleCollection(t *testing.T) {
	// Ported from: rdflib turtle serializer — rdf:List as ( ... )
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/list")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	// Build list: (1 2 3)
	n1 := rdflibgo.NewBNode()
	n2 := rdflibgo.NewBNode()
	n3 := rdflibgo.NewBNode()
	g.Add(s, p, n1)
	g.Add(n1, rdflibgo.RDF.First, rdflibgo.NewLiteral(1))
	g.Add(n1, rdflibgo.RDF.Rest, n2)
	g.Add(n2, rdflibgo.RDF.First, rdflibgo.NewLiteral(2))
	g.Add(n2, rdflibgo.RDF.Rest, n3)
	g.Add(n3, rdflibgo.RDF.First, rdflibgo.NewLiteral(3))
	g.Add(n3, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)

	out := serializeTurtle(t, g)
	if !strings.Contains(out, "( ") {
		t.Errorf("expected collection syntax, got:\n%s", out)
	}
}

func TestTurtleSortRDFTypeFirst(t *testing.T) {
	// Ported from: rdflib turtle serializer — rdf:type predicate comes first
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/name")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, rdflibgo.NewLiteral("Alice"))
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Person"))

	out := serializeTurtle(t, g)
	aIdx := strings.Index(out, " a ")
	nameIdx := strings.Index(out, "ex:name")
	if aIdx < 0 || nameIdx < 0 || aIdx > nameIdx {
		t.Errorf("rdf:type should come before other predicates, got:\n%s", out)
	}
}
