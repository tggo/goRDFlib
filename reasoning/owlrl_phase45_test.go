package reasoning

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// --- Phase 4: Class axiom rules ---

// --- cax-dw: disjointWith ---

func TestOWLRL_DisjointWith(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.OWL.DisjointWith, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)
	g.Add(t.Context(), exX, namespace.RDF.Type, exB)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "cax-dw" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cax-dw inconsistency")
	}
}

func TestOWLRL_DisjointWithNoViolation(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.OWL.DisjointWith, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)
	g.Add(t.Context(), exY, namespace.RDF.Type, exB)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	for _, i := range incon {
		if i.Rule == "cax-dw" {
			t.Error("should not have cax-dw inconsistency")
		}
	}
}

// --- cax-adc: AllDisjointClasses ---

func TestOWLRL_AllDisjointClasses(t *testing.T) {
	g := graph.NewGraph()
	adcNode := term.NewBNode("")
	g.Add(t.Context(), adcNode, namespace.RDF.Type, namespace.OWL.AllDisjointClasses)
	members := buildRDFList(g, []term.Term{exA, exB, exC})
	g.Add(t.Context(), adcNode, namespace.OWL.Members, members)

	g.Add(t.Context(), exX, namespace.RDF.Type, exA)
	g.Add(t.Context(), exX, namespace.RDF.Type, exB)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "cax-adc" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cax-adc inconsistency")
	}
}

// --- Phase 5: Consistency rules ---

// --- cls-nothing2 ---

func TestOWLRL_Nothing(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exX, namespace.RDF.Type, namespace.OWL.Nothing)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "cls-nothing2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cls-nothing2 inconsistency")
	}
}

// --- eq-diff1: sameAs + differentFrom ---

func TestOWLRL_SameAsDifferentFrom(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.OWL.SameAs, exB)
	g.Add(t.Context(), exA, namespace.OWL.DifferentFrom, exB)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "eq-diff1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected eq-diff1 inconsistency")
	}
}

func TestOWLRL_SameAsDifferentFromTransitive(t *testing.T) {
	g := graph.NewGraph()
	// A sameAs B sameAs C, A differentFrom C
	g.Add(t.Context(), exA, namespace.OWL.SameAs, exB)
	g.Add(t.Context(), exB, namespace.OWL.SameAs, exC)
	g.Add(t.Context(), exA, namespace.OWL.DifferentFrom, exC)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "eq-diff1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected eq-diff1 inconsistency (via transitive sameAs)")
	}
}

// --- prp-npa1: NegativePropertyAssertion with targetIndividual ---

func TestOWLRL_NegativePropertyAssertion(t *testing.T) {
	g := graph.NewGraph()
	npa := term.NewBNode("")
	g.Add(t.Context(), npa, namespace.RDF.Type, namespace.OWL.NegativePropertyAssertion)
	g.Add(t.Context(), npa, namespace.OWL.SourceIndividual, exA)
	g.Add(t.Context(), npa, namespace.OWL.AssertionProperty, exP)
	g.Add(t.Context(), npa, namespace.OWL.TargetIndividual, exB)

	// The asserted triple exists → violation
	g.Add(t.Context(), exA, exP, exB)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-npa1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-npa1 inconsistency")
	}
}

func TestOWLRL_NegativePropertyAssertionNoViolation(t *testing.T) {
	g := graph.NewGraph()
	npa := term.NewBNode("")
	g.Add(t.Context(), npa, namespace.RDF.Type, namespace.OWL.NegativePropertyAssertion)
	g.Add(t.Context(), npa, namespace.OWL.SourceIndividual, exA)
	g.Add(t.Context(), npa, namespace.OWL.AssertionProperty, exP)
	g.Add(t.Context(), npa, namespace.OWL.TargetIndividual, exB)

	// The asserted triple does NOT exist
	g.Add(t.Context(), exA, exP, exC) // different object

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	for _, i := range incon {
		if i.Rule == "prp-npa1" {
			t.Error("should not have prp-npa1 inconsistency")
		}
	}
}

// --- prp-npa2: NegativePropertyAssertion with targetValue ---

func TestOWLRL_NegativePropertyAssertionValue(t *testing.T) {
	g := graph.NewGraph()
	npa := term.NewBNode("")
	g.Add(t.Context(), npa, namespace.RDF.Type, namespace.OWL.NegativePropertyAssertion)
	g.Add(t.Context(), npa, namespace.OWL.SourceIndividual, exA)
	g.Add(t.Context(), npa, namespace.OWL.AssertionProperty, exP)
	lit := term.NewLiteral("forbidden")
	g.Add(t.Context(), npa, namespace.OWL.TargetValue, lit)

	// The asserted triple with the literal exists
	g.Add(t.Context(), exA, exP, lit)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-npa2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-npa2 inconsistency")
	}
}

// --- RDF list helper test ---

func TestCollectList(t *testing.T) {
	g := graph.NewGraph()
	items := []term.Term{exA, exB, exC}
	head := buildRDFList(g, items)

	result := collectList(newStopper(t.Context()), g, head)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	for i, item := range items {
		if term.TermKey(result[i]) != term.TermKey(item) {
			t.Errorf("item %d: expected %s, got %s", i, item.N3(), result[i].N3())
		}
	}
}

func TestCollectList_Empty(t *testing.T) {
	g := graph.NewGraph()
	result := collectList(newStopper(t.Context()), g, namespace.RDF.Nil)
	if len(result) != 0 {
		t.Errorf("expected 0 items for nil list, got %d", len(result))
	}
}

// --- Combined integration test ---

func TestOWLRL_FullIntegration(t *testing.T) {
	g := graph.NewGraph()
	code := namespace.NewNamespace("http://example.org/code/")

	// Class hierarchy with equivalentClass
	g.Add(t.Context(), code.Term("Constructor"), namespace.RDFS.SubClassOf, code.Term("Method"))
	g.Add(t.Context(), code.Term("Method"), namespace.RDFS.SubClassOf, code.Term("Function"))
	g.Add(t.Context(), code.Term("Callable"), namespace.OWL.EquivalentClass, code.Term("Function"))

	// Symmetric property
	g.Add(t.Context(), code.Term("relatedTo"), namespace.RDF.Type, namespace.OWL.SymmetricProperty)

	// Transitive property
	g.Add(t.Context(), code.Term("dependsOn"), namespace.RDF.Type, namespace.OWL.TransitiveProperty)

	// InverseOf
	g.Add(t.Context(), code.Term("contains"), namespace.OWL.InverseOf, code.Term("belongsTo"))

	// Data
	g.Add(t.Context(), code.Term("init"), namespace.RDF.Type, code.Term("Constructor"))
	g.Add(t.Context(), code.Term("moduleA"), code.Term("relatedTo"), code.Term("moduleB"))
	g.Add(t.Context(), code.Term("X"), code.Term("dependsOn"), code.Term("Y"))
	g.Add(t.Context(), code.Term("Y"), code.Term("dependsOn"), code.Term("Z"))
	g.Add(t.Context(), code.Term("file1"), code.Term("belongsTo"), code.Term("dir1"))

	n, incon, err := ExpandCheck(g, RDFS|OWLRL)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("expected triples to be added")
	}
	if len(incon) != 0 {
		t.Errorf("expected no inconsistencies, got %d", len(incon))
	}

	// RDFS: init should be a Method and Function
	if !mustV(g.Contains(t.Context(), code.Term("init"), namespace.RDF.Type, code.Term("Method"))) {
		t.Error("expected init rdf:type Method")
	}
	if !mustV(g.Contains(t.Context(), code.Term("init"), namespace.RDF.Type, code.Term("Function"))) {
		t.Error("expected init rdf:type Function")
	}

	// OWL equivalentClass: init should also be Callable
	if !mustV(g.Contains(t.Context(), code.Term("init"), namespace.RDF.Type, code.Term("Callable"))) {
		t.Error("expected init rdf:type Callable")
	}

	// Symmetric
	if !mustV(g.Contains(t.Context(), code.Term("moduleB"), code.Term("relatedTo"), code.Term("moduleA"))) {
		t.Error("expected moduleB relatedTo moduleA")
	}

	// Transitive
	if !mustV(g.Contains(t.Context(), code.Term("X"), code.Term("dependsOn"), code.Term("Z"))) {
		t.Error("expected X dependsOn Z")
	}

	// Inverse
	if !mustV(g.Contains(t.Context(), code.Term("dir1"), code.Term("contains"), code.Term("file1"))) {
		t.Error("expected dir1 contains file1")
	}
}
