package reasoning

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

var (
	ex  = namespace.NewNamespace("http://example.org/")
	exA = ex.Term("A")
	exB = ex.Term("B")
	exC = ex.Term("C")
	exD = ex.Term("D")
	exP = ex.Term("p")
	exQ = ex.Term("q")
	exR = ex.Term("r")
	exX = ex.Term("x")
	exY = ex.Term("y")
)

func TestRDFSClosure_SubClassOf(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.RDFS.SubClassOf, exB)
	g.Add(t.Context(), exB, namespace.RDFS.SubClassOf, exC)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)

	n, _ := RDFSClosure(t.Context(), g)
	if n < 2 {
		t.Fatalf("expected at least 2 inferred triples, got %d", n)
	}

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exB)) {
		t.Error("expected x rdf:type B")
	}
	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exC)) {
		t.Error("expected x rdf:type C")
	}
}

func TestRDFSClosure_SubPropertyOf(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDFS.SubPropertyOf, exQ)
	g.Add(t.Context(), exX, exP, exY)

	n, _ := RDFSClosure(t.Context(), g)
	if n < 1 {
		t.Fatalf("expected at least 1 inferred triple, got %d", n)
	}

	if !mustV(g.Contains(t.Context(), exX, exQ, exY)) {
		t.Error("expected x q y")
	}
}

func TestRDFSClosure_Domain(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDFS.Domain, exC)
	g.Add(t.Context(), exX, exP, exY)

	RDFSClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exC)) {
		t.Error("expected x rdf:type C")
	}
}

func TestRDFSClosure_Range(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDFS.Range, exC)
	g.Add(t.Context(), exX, exP, exY)

	RDFSClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exY, namespace.RDF.Type, exC)) {
		t.Error("expected y rdf:type C")
	}
}

func TestRDFSClosure_SubClassOfTransitive(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.RDFS.SubClassOf, exB)
	g.Add(t.Context(), exB, namespace.RDFS.SubClassOf, exC)
	g.Add(t.Context(), exC, namespace.RDFS.SubClassOf, exD)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)

	RDFSClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exD)) {
		t.Error("expected x rdf:type D through transitive subClassOf")
	}
}

func TestRDFSClosure_SubPropertyOfTransitive(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDFS.SubPropertyOf, exQ)
	g.Add(t.Context(), exQ, namespace.RDFS.SubPropertyOf, exR)
	g.Add(t.Context(), exX, exP, exY)

	RDFSClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, exQ, exY)) {
		t.Error("expected x q y")
	}
	if !mustV(g.Contains(t.Context(), exX, exR, exY)) {
		t.Error("expected x r y")
	}
}

func TestRDFSClosure_DomainWithSubClass(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDFS.Domain, exA)
	g.Add(t.Context(), exA, namespace.RDFS.SubClassOf, exB)
	g.Add(t.Context(), exX, exP, exY)

	RDFSClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exA)) {
		t.Error("expected x rdf:type A from domain")
	}
	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exB)) {
		t.Error("expected x rdf:type B from domain + subClassOf")
	}
}

func TestRDFSClosure_CyclicSubClass(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.RDFS.SubClassOf, exB)
	g.Add(t.Context(), exB, namespace.RDFS.SubClassOf, exA)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)

	// Must terminate
	RDFSClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exB)) {
		t.Error("expected x rdf:type B")
	}
	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exA)) {
		t.Error("expected x rdf:type A (still present)")
	}
}

func TestRDFSClosure_LiteralObject(t *testing.T) {
	g := graph.NewGraph()
	lit := term.NewLiteral("hello")
	g.Add(t.Context(), exP, namespace.RDFS.Range, exC)
	g.Add(t.Context(), exX, exP, lit)

	before, _ := g.Len(t.Context())
	RDFSClosure(t.Context(), g)

	// Range rule should NOT add "hello" rdf:type C since literals can't be subjects
	if mustV(g.Len(t.Context())) != before {
		t.Error("expected no new triples when object is a literal (range rule)")
	}
}

func TestRDFSClosure_EmptyGraph(t *testing.T) {
	g := graph.NewGraph()
	n, _ := RDFSClosure(t.Context(), g)
	if n != 0 {
		t.Errorf("expected 0 triples added for empty graph, got %d", n)
	}
}

func TestRDFSClosure_Idempotent(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.RDFS.SubClassOf, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)

	n1, _ := RDFSClosure(t.Context(), g)
	count1, _ := g.Len(t.Context())

	n2, _ := RDFSClosure(t.Context(), g)
	count2, _ := g.Len(t.Context())

	if n1 == 0 {
		t.Error("first closure should add triples")
	}
	if n2 != 0 {
		t.Errorf("second closure should add 0 triples, got %d", n2)
	}
	if count1 != count2 {
		t.Errorf("graph size changed: %d → %d", count1, count2)
	}
}

func TestRDFSClosure_SPARQLIntegration(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.RDFS.SubClassOf, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)

	// Before closure: query for ?s a ex:B should return nothing
	results1, err := sparql.Query(g, `SELECT ?s WHERE { ?s a <http://example.org/B> }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(results1.Bindings) != 0 {
		t.Error("expected 0 results before closure")
	}

	RDFSClosure(t.Context(), g)

	// After closure: query should find x
	results2, err := sparql.Query(g, `SELECT ?s WHERE { ?s a <http://example.org/B> }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(results2.Bindings) != 1 {
		t.Errorf("expected 1 result after closure, got %d", len(results2.Bindings))
	}
}

func TestExpand_RDFS(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.RDFS.SubClassOf, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)

	n, err := Expand(g, RDFS)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("expected triples to be added")
	}
	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exB)) {
		t.Error("expected x rdf:type B")
	}
}

func TestExpand_UnknownRegime(t *testing.T) {
	g := graph.NewGraph()
	_, err := Expand(g, Regime(99))
	if err != ErrUnknownRegime {
		t.Errorf("expected ErrUnknownRegime, got %v", err)
	}
}
