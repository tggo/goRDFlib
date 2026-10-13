package reasoning

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// --- Phase 3: Class restriction rules ---

// --- cls-hv1: hasValue + type → property value ---

func TestOWLRL_HasValue_TypeToProperty(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.HasValue, exA)

	g.Add(t.Context(), exX, namespace.RDF.Type, restr)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, exP, exA)) {
		t.Error("expected x p A (cls-hv1: hasValue + type)")
	}
}

// --- cls-hv2: hasValue + property → type ---

func TestOWLRL_HasValue_PropertyToType(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.HasValue, exA)

	g.Add(t.Context(), exX, exP, exA)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, restr)) {
		t.Error("expected x rdf:type restriction (cls-hv2: hasValue + property)")
	}
}

// --- cls-svf1: someValuesFrom + typed value → type ---

func TestOWLRL_SomeValuesFrom(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.SomeValuesFrom, exA)

	g.Add(t.Context(), exX, exP, exY)
	g.Add(t.Context(), exY, namespace.RDF.Type, exA)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, restr)) {
		t.Error("expected x rdf:type restriction (cls-svf1)")
	}
}

// --- cls-svf2: someValuesFrom owl:Thing ---

func TestOWLRL_SomeValuesFromThing(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.SomeValuesFrom, namespace.OWL.Thing)

	g.Add(t.Context(), exX, exP, exY)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, restr)) {
		t.Error("expected x rdf:type restriction (cls-svf2: someValuesFrom owl:Thing)")
	}
}

// --- cls-avf: allValuesFrom ---

func TestOWLRL_AllValuesFrom(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.AllValuesFrom, exA)

	g.Add(t.Context(), exX, namespace.RDF.Type, restr)
	g.Add(t.Context(), exX, exP, exY)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exY, namespace.RDF.Type, exA)) {
		t.Error("expected y rdf:type A (cls-avf: allValuesFrom)")
	}
}

// --- cls-maxc2: maxCardinality 1 → sameAs ---

func TestOWLRL_MaxCardinality1(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.MaxCardinality, term.NewLiteral("1",
		term.WithDatatype(term.XSDInteger)))

	g.Add(t.Context(), exX, namespace.RDF.Type, restr)
	g.Add(t.Context(), exX, exP, exA)
	g.Add(t.Context(), exX, exP, exB)

	OWLRLClosure(t.Context(), g)

	hasSameAs := mustV(g.Contains(t.Context(), exA, namespace.OWL.SameAs, exB)) ||
		mustV(g.Contains(t.Context(), exB, namespace.OWL.SameAs, exA))
	if !hasSameAs {
		t.Error("expected A owl:sameAs B (cls-maxc2: maxCardinality 1)")
	}
}

// --- cls-maxc1: maxCardinality 0 → inconsistency ---

func TestOWLRL_MaxCardinality0(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.MaxCardinality, term.NewLiteral("0",
		term.WithDatatype(term.XSDInteger)))

	g.Add(t.Context(), exX, namespace.RDF.Type, restr)
	g.Add(t.Context(), exX, exP, exA)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "cls-maxc1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cls-maxc1 inconsistency")
	}
}

// --- cls-maxqc3: maxQualifiedCardinality 1 with onClass ---

func TestOWLRL_MaxQualifiedCardinality1(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.MaxQualifiedCardinality, term.NewLiteral("1",
		term.WithDatatype(term.XSDInteger)))
	g.Add(t.Context(), restr, namespace.OWL.OnClass, exC)

	g.Add(t.Context(), exX, namespace.RDF.Type, restr)
	g.Add(t.Context(), exX, exP, exA)
	g.Add(t.Context(), exA, namespace.RDF.Type, exC)
	g.Add(t.Context(), exX, exP, exB)
	g.Add(t.Context(), exB, namespace.RDF.Type, exC)

	OWLRLClosure(t.Context(), g)

	hasSameAs := mustV(g.Contains(t.Context(), exA, namespace.OWL.SameAs, exB)) ||
		mustV(g.Contains(t.Context(), exB, namespace.OWL.SameAs, exA))
	if !hasSameAs {
		t.Error("expected A owl:sameAs B (cls-maxqc3)")
	}
}

// --- cls-maxqc4: maxQualifiedCardinality 1 with onClass owl:Thing ---

func TestOWLRL_MaxQualifiedCardinality1Thing(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.MaxQualifiedCardinality, term.NewLiteral("1",
		term.WithDatatype(term.XSDInteger)))
	g.Add(t.Context(), restr, namespace.OWL.OnClass, namespace.OWL.Thing)

	g.Add(t.Context(), exX, namespace.RDF.Type, restr)
	g.Add(t.Context(), exX, exP, exA)
	g.Add(t.Context(), exX, exP, exB)

	OWLRLClosure(t.Context(), g)

	hasSameAs := mustV(g.Contains(t.Context(), exA, namespace.OWL.SameAs, exB)) ||
		mustV(g.Contains(t.Context(), exB, namespace.OWL.SameAs, exA))
	if !hasSameAs {
		t.Error("expected A owl:sameAs B (cls-maxqc4: onClass owl:Thing)")
	}
}

// --- cls-maxqc1: maxQualifiedCardinality 0 → inconsistency ---

func TestOWLRL_MaxQualifiedCardinality0(t *testing.T) {
	g := graph.NewGraph()
	restr := term.NewBNode("")
	g.Add(t.Context(), restr, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(t.Context(), restr, namespace.OWL.OnProperty, exP)
	g.Add(t.Context(), restr, namespace.OWL.MaxQualifiedCardinality, term.NewLiteral("0",
		term.WithDatatype(term.XSDInteger)))
	g.Add(t.Context(), restr, namespace.OWL.OnClass, exC)

	g.Add(t.Context(), exX, namespace.RDF.Type, restr)
	g.Add(t.Context(), exX, exP, exA)
	g.Add(t.Context(), exA, namespace.RDF.Type, exC)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "cls-maxqc1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cls-maxqc1 inconsistency")
	}
}

// --- cls-int1: intersectionOf → type ---

func TestOWLRL_IntersectionOf_MemberToClass(t *testing.T) {
	g := graph.NewGraph()
	intClass := term.NewBNode("")
	members := buildRDFList(g, []term.Term{exA, exB})
	g.Add(t.Context(), intClass, namespace.OWL.IntersectionOf, members)

	g.Add(t.Context(), exX, namespace.RDF.Type, exA)
	g.Add(t.Context(), exX, namespace.RDF.Type, exB)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, intClass)) {
		t.Error("expected x rdf:type intersection (cls-int1)")
	}
}

func TestOWLRL_IntersectionOf_Partial(t *testing.T) {
	g := graph.NewGraph()
	intClass := term.NewBNode("")
	members := buildRDFList(g, []term.Term{exA, exB})
	g.Add(t.Context(), intClass, namespace.OWL.IntersectionOf, members)

	g.Add(t.Context(), exX, namespace.RDF.Type, exA) // only one member type

	OWLRLClosure(t.Context(), g)

	if mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, intClass)) {
		t.Error("should NOT be typed as intersection (missing type B)")
	}
}

// --- cls-int2: type intersection → member types ---

func TestOWLRL_IntersectionOf_ClassToMembers(t *testing.T) {
	g := graph.NewGraph()
	intClass := term.NewBNode("")
	members := buildRDFList(g, []term.Term{exA, exB})
	g.Add(t.Context(), intClass, namespace.OWL.IntersectionOf, members)

	g.Add(t.Context(), exX, namespace.RDF.Type, intClass)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exA)) {
		t.Error("expected x rdf:type A (cls-int2)")
	}
	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, exB)) {
		t.Error("expected x rdf:type B (cls-int2)")
	}
}

// --- cls-uni: unionOf ---

func TestOWLRL_UnionOf(t *testing.T) {
	g := graph.NewGraph()
	unionClass := term.NewBNode("")
	members := buildRDFList(g, []term.Term{exA, exB})
	g.Add(t.Context(), unionClass, namespace.OWL.UnionOf, members)

	g.Add(t.Context(), exX, namespace.RDF.Type, exA)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, unionClass)) {
		t.Error("expected x rdf:type union (cls-uni)")
	}
}

func TestOWLRL_UnionOfSecondMember(t *testing.T) {
	g := graph.NewGraph()
	unionClass := term.NewBNode("")
	members := buildRDFList(g, []term.Term{exA, exB})
	g.Add(t.Context(), unionClass, namespace.OWL.UnionOf, members)

	g.Add(t.Context(), exX, namespace.RDF.Type, exB)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exX, namespace.RDF.Type, unionClass)) {
		t.Error("expected x rdf:type union via second member (cls-uni)")
	}
}

// --- cls-oo: oneOf ---

func TestOWLRL_OneOf(t *testing.T) {
	g := graph.NewGraph()
	enumClass := term.NewBNode("")
	members := buildRDFList(g, []term.Term{exA, exB, exC})
	g.Add(t.Context(), enumClass, namespace.OWL.OneOf, members)

	OWLRLClosure(t.Context(), g)

	if !mustV(g.Contains(t.Context(), exA, namespace.RDF.Type, enumClass)) {
		t.Error("expected A rdf:type enumClass (cls-oo)")
	}
	if !mustV(g.Contains(t.Context(), exB, namespace.RDF.Type, enumClass)) {
		t.Error("expected B rdf:type enumClass (cls-oo)")
	}
	if !mustV(g.Contains(t.Context(), exC, namespace.RDF.Type, enumClass)) {
		t.Error("expected C rdf:type enumClass (cls-oo)")
	}
}

// --- cls-com: complementOf → inconsistency ---

func TestOWLRL_ComplementOf(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.OWL.ComplementOf, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA)
	g.Add(t.Context(), exX, namespace.RDF.Type, exB)

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	found := false
	for _, i := range incon {
		if i.Rule == "cls-com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cls-com inconsistency")
	}
}

func TestOWLRL_ComplementOfNoViolation(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), exA, namespace.OWL.ComplementOf, exB)
	g.Add(t.Context(), exX, namespace.RDF.Type, exA) // only one class

	_, incon, _ := OWLRLClosureCheck(t.Context(), g)

	for _, i := range incon {
		if i.Rule == "cls-com" {
			t.Error("should not have cls-com inconsistency")
		}
	}
}
