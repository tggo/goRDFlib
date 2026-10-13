package reasoning

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// --- prp-symp ---

func TestOWLRL_SymmetricProperty(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.SymmetricProperty)
	g.Add(t.Context(), exX, exP, exY)

	n, _ := OWLRLClosure(t.Context(), g)
	if n == 0 {
		t.Fatal("expected inferred triples")
	}
	if !mustV(g.Contains(t.Context(), exY, exP, exX)) {
		t.Error("expected y p x (symmetric)")
	}
}

// --- prp-trp ---

func TestOWLRL_TransitiveProperty(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.TransitiveProperty)
	g.Add(t.Context(), exX, exP, exY)
	g.Add(t.Context(), exY, exP, ex.Term("z"))

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, exP, ex.Term("z"))) {
		t.Error("expected x p z (transitive)")
	}
}

func TestOWLRL_TransitivePropertyChain(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.TransitiveProperty)
	g.Add(t.Context(), exA, exP, exB)
	g.Add(t.Context(), exB, exP, exC)
	g.Add(t.Context(), exC, exP, exD)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exA, exP, exC)) {
		t.Error("expected A p C")
	}
	if !mustV(g.Contains(t.Context(), exA, exP, exD)) {
		t.Error("expected A p D")
	}
	if !mustV(g.Contains(t.Context(), exB, exP, exD)) {
		t.Error("expected B p D")
	}
}

func TestOWLRL_TransitivePropertyCycle(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.TransitiveProperty)
	g.Add(t.Context(), exA, exP, exB)
	g.Add(t.Context(), exB, exP, exA) // cycle

	// Must terminate
	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exA, exP, exA)) {
		t.Error("expected A p A (reflexive from cycle)")
	}
	if !mustV(g.Contains(t.Context(), exB, exP, exB)) {
		t.Error("expected B p B (reflexive from cycle)")
	}
}

// --- prp-inv1/2 ---

func TestOWLRL_InverseOf(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.OWL.InverseOf, exQ)
	g.Add(t.Context(), exX, exP, exY)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exY, exQ, exX)) {
		t.Error("expected y q x (inverse)")
	}
}

func TestOWLRL_InverseOfBidirectional(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.OWL.InverseOf, exQ)
	g.Add(t.Context(), exY, exQ, exX)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, exP, exY)) {
		t.Error("expected x p y (inverse of q)")
	}
}

// --- prp-eqp1/2 ---

func TestOWLRL_EquivalentProperty(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.OWL.EquivalentProperty, exQ)
	g.Add(t.Context(), exX, exP, exY)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, exQ, exY)) {
		t.Error("expected x q y (equivalent property)")
	}
}

func TestOWLRL_EquivalentPropertyBidirectional(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.OWL.EquivalentProperty, exQ)
	g.Add(t.Context(), exX, exQ, exY)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, exP, exY)) {
		t.Error("expected x p y (equivalent property reverse)")
	}
}

// --- cax-eqc1/2 ---

func TestOWLRL_EquivalentClass(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.OWL.EquivalentClass, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exB)) {
		t.Error("expected x rdf:type B (equivalent class)")
	}
}

func TestOWLRL_EquivalentClassBidirectional(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.OWL.EquivalentClass, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exB)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exA)) {
		t.Error("expected x rdf:type A (equivalent class reverse)")
	}
}

// --- prp-fp ---

func TestOWLRL_FunctionalProperty(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.FunctionalProperty)
	g.Add(t.Context(), exX, exP, exA)
	g.Add(t.Context(), exX, exP, exB)

	OWLRLClosure(t.Context(), g)

	hasSameAs := mustV(g.Contains(t.Context(), exA, namespace.OWL.SameAs, exB)) ||
		mustV(g.Contains(t.Context(), exB, namespace.OWL.SameAs, exA))
	if !hasSameAs {
		t.Error("expected A owl:sameAs B (functional property)")
	}
}

// --- prp-ifp ---

func TestOWLRL_InverseFunctionalProperty(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.InverseFunctionalProperty)
	g.Add(t.Context(), exA, exP, exX)
	g.Add(t.Context(), exB, exP, exX)

	OWLRLClosure(t.Context(), g)

	hasSameAs := mustV(g.Contains(t.Context(), exA, namespace.OWL.SameAs, exB)) ||
		mustV(g.Contains(t.Context(), exB, namespace.OWL.SameAs, exA))
	if !hasSameAs {
		t.Error("expected A owl:sameAs B (inverse functional property)")
	}
}

// --- eq-sym ---

func TestOWLRL_SameAsSymmetric(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exX, namespace.OWL.SameAs, exY)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exY, namespace.OWL.SameAs, exX)) {
		t.Error("expected y owl:sameAs x (symmetric)")
	}
}

// --- eq-trans ---

func TestOWLRL_SameAsTransitive(t *testing.T) {
	g := graph.NewGraph()
	z := ex.Term("z")
	g.Add(t.Context(), exX, namespace.OWL.SameAs, exY)
	g.Add(t.Context(), exY, namespace.OWL.SameAs, z)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.OWL.SameAs, z)) {
		t.Error("expected x owl:sameAs z (transitive)")
	}
}

// --- eq-rep-s/p/o ---

func TestOWLRL_SameAsReplacement(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exX, namespace.OWL.SameAs, exY)
	g.Add(t.Context(), exX, exP, exA)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exY, exP, exA)) {
		t.Error("expected y p A (sameAs replacement in subject)")
	}
}

func TestOWLRL_SameAsReplacementObject(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.OWL.SameAs, exB)
	g.Add(t.Context(), exX, exP, exA)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, exP, exB)) {
		t.Error("expected x p B (sameAs replacement in object)")
	}
}

// --- Combined rules ---

func TestOWLRL_SymmetricAndTransitive(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.SymmetricProperty)
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.TransitiveProperty)
	g.Add(t.Context(), exA, exP, exB)
	g.Add(t.Context(), exB, exP, exC)

	OWLRLClosure(t.Context(), g)

	// Transitive: A→C
	if !mustV(g.Contains(t.Context(), exA, exP, exC)) {
		t.Error("expected A p C (transitive)")
	}
	// Symmetric: B→A, C→B, C→A
	if !mustV(g.Contains(t.Context(), exB, exP, exA)) {
		t.Error("expected B p A (symmetric)")
	}
	if !mustV(g.Contains(t.Context(), exC, exP, exB)) {
		t.Error("expected C p B (symmetric)")
	}
	if !mustV(g.Contains(t.Context(), exC, exP, exA)) {
		t.Error("expected C p A (symmetric + transitive)")
	}
}

// --- Idempotent ---

func TestOWLRL_Idempotent(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.SymmetricProperty)
	g.Add(t.Context(), exX, exP, exY)

	n1, _ := OWLRLClosure(t.Context(), g)
	count1, _ := g.Len(t.Context())

	n2, _ := OWLRLClosure(t.Context(), g)
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

// --- Empty graph ---

func TestOWLRL_EmptyGraph(t *testing.T) {
	g := graph.NewGraph()
	n, _ := OWLRLClosure(t.Context(), g)
	if n != 0 {
		t.Errorf("expected 0 triples added for empty graph, got %d", n)
	}
}

// --- Expand with OWLRL ---

func TestExpand_OWLRL(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.SymmetricProperty)
	g.Add(t.Context(), exX, exP, exY)

	n, err := Expand(g, OWLRL)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("expected triples to be added")
	}
	if !mustV(g.Contains(t.Context(), exY, exP, exX)) {
		t.Error("expected y p x")
	}
}

func TestExpand_CombinedRegimes(t *testing.T) {
	g := graph.NewGraph()
	// RDFS: subClassOf
	g.Add(t.Context(), exA, namespace.RDFS.SubClassOf, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)
	// OWL: symmetric property
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.SymmetricProperty)
	g.Add(t.Context(), exX, exP, exY)

	n, err := Expand(g, RDFS|OWLRL)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("expected triples to be added")
	}
	// RDFS inference
	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exB)) {
		t.Error("expected x rdf:type B (RDFS)")
	}
	// OWL inference
	if !mustV(g.Contains(t.Context(), exY, exP, exX)) {
		t.Error("expected y p x (OWL symmetric)")
	}
}

// --- SPARQL integration ---

func TestOWLRL_SPARQLIntegration(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.TransitiveProperty)
	g.Add(t.Context(), exA, exP, exB)
	g.Add(t.Context(), exB, exP, exC)

	// Before closure
	results1, err := sparql.Query(g, `SELECT ?o WHERE { <http://example.org/A> <http://example.org/p> ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(results1.Bindings) != 1 {
		t.Errorf("expected 1 result before closure, got %d", len(results1.Bindings))
	}

	OWLRLClosure(t.Context(), g)

	// After closure
	results2, err := sparql.Query(g, `SELECT ?o WHERE { <http://example.org/A> <http://example.org/p> ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(results2.Bindings) != 2 {
		t.Errorf("expected 2 results after closure, got %d", len(results2.Bindings))
	}
}

// --- Issue #2 use case ---

func TestOWLRL_IssueUseCase(t *testing.T) {
	code := namespace.NewNamespace("http://example.org/code/")

	g := graph.NewGraph()
	// Symmetric property
	g.Add(t.Context(), code.Term("sharedTypeWith"), namespace.RDF.Type, namespace.OWL.SymmetricProperty)
	g.Add(t.Context(), code.Term("A"), code.Term("sharedTypeWith"), code.Term("B"))

	// Transitive property
	g.Add(t.Context(), code.Term("inherits"), namespace.RDF.Type, namespace.OWL.TransitiveProperty)
	g.Add(t.Context(), code.Term("C"), code.Term("inherits"), code.Term("D"))
	g.Add(t.Context(), code.Term("D"), code.Term("inherits"), code.Term("E"))

	// InverseOf
	g.Add(t.Context(), code.Term("containsRepo"), namespace.OWL.InverseOf, code.Term("belongsToGroup"))
	g.Add(t.Context(), code.Term("repo1"), code.Term("belongsToGroup"), code.Term("group1"))

	OWLRLClosure(t.Context(), g)

	// Symmetric: B sharedTypeWith A
	if !mustV(g.Contains(t.Context(), code.Term("B"), code.Term("sharedTypeWith"), code.Term("A"))) {
		t.Error("expected B sharedTypeWith A")
	}
	// Transitive: C inherits E
	if !mustV(g.Contains(t.Context(), code.Term("C"), code.Term("inherits"), code.Term("E"))) {
		t.Error("expected C inherits E")
	}
	// Inverse: group1 containsRepo repo1
	if !mustV(g.Contains(t.Context(), code.Term("group1"), code.Term("containsRepo"), code.Term("repo1"))) {
		t.Error("expected group1 containsRepo repo1")
	}
}

// --- Union-find ---

func TestUnionFind_Basic(t *testing.T) {
	uf := newUnionFind()
	a := term.NewURIRefUnsafe("http://example.org/a")
	b := term.NewURIRefUnsafe("http://example.org/b")
	c := term.NewURIRefUnsafe("http://example.org/c")

	uf.union(a, b)
	uf.union(b, c)

	fa := uf.find(a)
	fb := uf.find(b)
	fc := uf.find(c)

	if term.TermKey(fa) != term.TermKey(fb) || term.TermKey(fb) != term.TermKey(fc) {
		t.Errorf("expected all same canonical: a=%s b=%s c=%s", fa.N3(), fb.N3(), fc.N3())
	}

	classes := uf.classes()
	if len(classes) != 1 {
		t.Errorf("expected 1 equivalence class, got %d", len(classes))
	}
	if len(classes) > 0 && len(classes[0]) != 3 {
		t.Errorf("expected 3 members, got %d", len(classes[0]))
	}
}

func TestUnionFind_Disjoint(t *testing.T) {
	uf := newUnionFind()
	a := term.NewURIRefUnsafe("http://example.org/a")
	b := term.NewURIRefUnsafe("http://example.org/b")
	c := term.NewURIRefUnsafe("http://example.org/c")
	d := term.NewURIRefUnsafe("http://example.org/d")

	uf.union(a, b)
	uf.union(c, d)

	if term.TermKey(uf.find(a)) == term.TermKey(uf.find(c)) {
		t.Error("a and c should be in different classes")
	}

	classes := uf.classes()
	if len(classes) != 2 {
		t.Errorf("expected 2 equivalence classes, got %d", len(classes))
	}
}
