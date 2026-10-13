package reasoning

import (
	"context"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// --- Phase 2: Extended property rules ---

// --- prp-spo1: property chain axiom ---

func TestOWLRL_PropertyChainAxiom(t *testing.T) {
	g := graph.NewGraph()
	uncle := ex.Term("uncle")
	parent := ex.Term("parent")
	brother := ex.Term("brother")

	// uncle = parent . brother
	chain := buildRDFList(g, []term.Term{parent, brother})
	g.Add(t.Context(), uncle, namespace.OWL.PropertyChainAxiom, chain)

	g.Add(t.Context(), exA, parent, exB)
	g.Add(t.Context(), exB, brother, exC)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exA, uncle, exC)) {
		t.Error("expected A uncle C via property chain (parent . brother)")
	}
}

func TestOWLRL_PropertyChainAxiomThreeSteps(t *testing.T) {
	g := graph.NewGraph()
	result := ex.Term("result")
	p1 := ex.Term("p1")
	p2 := ex.Term("p2")
	p3 := ex.Term("p3")

	chain := buildRDFList(g, []term.Term{p1, p2, p3})
	g.Add(t.Context(), result, namespace.OWL.PropertyChainAxiom, chain)

	g.Add(t.Context(), exA, p1, exB)
	g.Add(t.Context(), exB, p2, exC)
	g.Add(t.Context(), exC, p3, exD)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exA, result, exD)) {
		t.Error("expected A result D via 3-step chain")
	}
}

// --- prp-key: hasKey ---

func TestOWLRL_HasKey(t *testing.T) {
	g := graph.NewGraph()
	keyProp := ex.Term("ssn")
	cls := ex.Term("Person")

	keys := buildRDFList(g, []term.Term{keyProp})
	g.Add(t.Context(), cls, namespace.OWL.HasKey, keys)

	g.Add(t.Context(), exA, namespace.RDF.Type, cls)
	g.Add(t.Context(), exB, namespace.RDF.Type, cls)
	ssn := term.NewLiteral("123-45-6789")
	g.Add(t.Context(), exA, keyProp, ssn)
	g.Add(t.Context(), exB, keyProp, ssn)

	OWLRLClosure(t.Context(), g)

	hasSameAs := mustV(g.Contains(t.Context(), exA, namespace.OWL.SameAs, exB)) ||
		mustV(g.Contains(t.Context(), exB, namespace.OWL.SameAs, exA))
	if !hasSameAs {
		t.Error("expected A owl:sameAs B (same key values)")
	}
}

func TestOWLRL_HasKeyNoMatch(t *testing.T) {
	g := graph.NewGraph()
	keyProp := ex.Term("ssn")
	cls := ex.Term("Person")

	keys := buildRDFList(g, []term.Term{keyProp})
	g.Add(t.Context(), cls, namespace.OWL.HasKey, keys)

	g.Add(t.Context(), exA, namespace.RDF.Type, cls)
	g.Add(t.Context(), exB, namespace.RDF.Type, cls)
	g.Add(t.Context(), exA, keyProp, term.NewLiteral("111"))
	g.Add(t.Context(), exB, keyProp, term.NewLiteral("222"))

	OWLRLClosure(t.Context(), g)

	hasSameAs := mustV(g.Contains(t.Context(), exA, namespace.OWL.SameAs, exB)) ||
		mustV(g.Contains(t.Context(), exB, namespace.OWL.SameAs, exA))
	if hasSameAs {
		t.Error("should NOT have sameAs (different key values)")
	}
}

// --- prp-irp: irreflexive property (consistency) ---

func TestOWLRL_IrreflexiveProperty(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.IrreflexiveProperty)
	g.Add(t.Context(), exA, exP, exA)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-irp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-irp inconsistency for reflexive use of irreflexive property")
	}
}

func TestOWLRL_IrreflexivePropertyNoViolation(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.IrreflexiveProperty)
	g.Add(t.Context(), exA, exP, exB)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	for _, i := range incon {
		if i.Rule == "prp-irp" {
			t.Error("should not have prp-irp inconsistency")
		}
	}
}

// --- prp-asyp: asymmetric property (consistency) ---

func TestOWLRL_AsymmetricProperty(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.RDF.Type, namespace.OWL.AsymmetricProperty)
	g.Add(t.Context(), exA, exP, exB)
	g.Add(t.Context(), exB, exP, exA)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-asyp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-asyp inconsistency")
	}
}

// --- prp-pdw: property disjoint with (consistency) ---

func TestOWLRL_PropertyDisjointWith(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exP, namespace.OWL.PropertyDisjointWith, exQ)
	g.Add(t.Context(), exA, exP, exB)
	g.Add(t.Context(), exA, exQ, exB)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-pdw" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-pdw inconsistency")
	}
}

// --- prp-adp: all disjoint properties (consistency) ---

func TestOWLRL_AllDisjointProperties(t *testing.T) {
	g := graph.NewGraph()
	adpNode := term.NewBNode("")
	g.Add(t.Context(), adpNode, namespace.RDF.Type, namespace.OWL.AllDisjointProperties)
	memberList := buildRDFList(g, []term.Term{exP, exQ, exR})
	g.Add(t.Context(), adpNode, namespace.OWL.Members, memberList)

	g.Add(t.Context(), exA, exP, exB)
	g.Add(t.Context(), exA, exQ, exB) // violates disjointness with P

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-adp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-adp inconsistency")
	}
}

// --- Helper: build an RDF list in the graph ---

func buildRDFList(g *graph.Graph, items []term.Term) term.Subject {
	if len(items) == 0 {
		return namespace.RDF.Nil
	}
	var head term.Subject
	var prev term.Subject
	for i, item := range items {
		node := term.NewBNode("")
		if i == 0 {
			head = node
		}
		g.Add(context.Background(), node, namespace.RDF.First, item)
		if prev != nil {
			g.Add(context.Background(), prev, namespace.RDF.Rest, node)
		}
		prev = node
	}
	g.Add(context.Background(), prev, namespace.RDF.Rest, namespace.RDF.Nil)
	return head
}
