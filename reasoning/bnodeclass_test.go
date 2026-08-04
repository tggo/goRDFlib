package reasoning

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// A class in RDF may be a blank node: an anonymous class expression such as an
// owl:Restriction is a blank node by construction. The rules that propagate
// class membership must therefore accept a blank node wherever they accept a
// named class, both as the class the membership is read from and as the class
// the membership is written to.
//
// Reference behaviour for every case here is rdflib + owlrl.

// TestRDFSClosure_BNodeSubClass covers rdfs9 in isolation. No OWL rule is
// involved: the subclass axiom is stated directly on a blank node.
//
//	ex:x rdf:type _:c . _:c rdfs:subClassOf ex:B . ⊨ ex:x rdf:type ex:B .
func TestRDFSClosure_BNodeSubClass(t *testing.T) {
	g := graph.NewGraph()
	c := term.NewBNode("c")
	g.Add(exX, namespace.RDF.Type, c)
	g.Add(c, namespace.RDFS.SubClassOf, exB)

	RDFSClosure(g)

	if !g.Contains(exX, namespace.RDF.Type, exB) {
		t.Error("rdfs9 did not propagate membership out of a blank-node class")
	}
}

// TestRDFSClosure_BNodeSuperClass covers the opposite direction: the class the
// membership is written to is the blank node. This requires the subclass index
// to hold a blank node as a value, not only as a key.
//
//	ex:x rdf:type ex:A . ex:A rdfs:subClassOf _:c . ⊨ ex:x rdf:type _:c .
func TestRDFSClosure_BNodeSuperClass(t *testing.T) {
	g := graph.NewGraph()
	c := term.NewBNode("c")
	g.Add(exX, namespace.RDF.Type, exA)
	g.Add(exA, namespace.RDFS.SubClassOf, c)

	RDFSClosure(g)

	if !g.Contains(exX, namespace.RDF.Type, c) {
		t.Error("rdfs9 did not propagate membership into a blank-node class")
	}
}

// TestRDFSClosure_BNodeSubClassTransitive checks that the transitive closure of
// rdfs:subClassOf passes through a blank-node class rather than stopping at it.
//
//	ex:x rdf:type ex:A . ex:A rdfs:subClassOf _:c . _:c rdfs:subClassOf ex:C .
func TestRDFSClosure_BNodeSubClassTransitive(t *testing.T) {
	g := graph.NewGraph()
	c := term.NewBNode("c")
	g.Add(exX, namespace.RDF.Type, exA)
	g.Add(exA, namespace.RDFS.SubClassOf, c)
	g.Add(c, namespace.RDFS.SubClassOf, exC)

	RDFSClosure(g)

	if !g.Contains(exX, namespace.RDF.Type, exC) {
		t.Error("subclass transitive closure did not pass through a blank-node class")
	}
}

// TestRDFSClosure_LiteralNotAClass guards the widening. A literal is not a
// class, so rdfs:subClassOf with a literal object must stay inert.
func TestRDFSClosure_LiteralNotAClass(t *testing.T) {
	g := graph.NewGraph()
	lit := term.NewLiteral("not a class")
	g.Add(exX, namespace.RDF.Type, exA)
	g.Add(exA, namespace.RDFS.SubClassOf, lit)

	RDFSClosure(g)

	if g.Contains(exX, namespace.RDF.Type, lit) {
		t.Error("a literal was accepted as a superclass")
	}
}

// TestOWLRL_BNodeEquivalentClass covers cax-eqc1/2 with an anonymous class on
// one side of the axiom, in both directions.
func TestOWLRL_BNodeEquivalentClass(t *testing.T) {
	t.Run("out of the blank node", func(t *testing.T) {
		g := graph.NewGraph()
		c := term.NewBNode("c")
		g.Add(exX, namespace.RDF.Type, c)
		g.Add(c, namespace.OWL.EquivalentClass, exB)

		if _, err := Expand(g, RDFS|OWLRL); err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if !g.Contains(exX, namespace.RDF.Type, exB) {
			t.Error("cax-eqc did not propagate membership out of a blank-node class")
		}
	})

	t.Run("into the blank node", func(t *testing.T) {
		g := graph.NewGraph()
		c := term.NewBNode("c")
		g.Add(exX, namespace.RDF.Type, exA)
		g.Add(exA, namespace.OWL.EquivalentClass, c)

		if _, err := Expand(g, RDFS|OWLRL); err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if !g.Contains(exX, namespace.RDF.Type, c) {
			t.Error("cax-eqc did not propagate membership into a blank-node class")
		}
	})
}

// TestOWLRL_AnonymousRestrictionClassifies is the practical case. An
// owl:Restriction is anonymous, so unless a blank node can act as a class no
// restriction can ever classify an individual under a named class.
//
// The restriction selects products sold in NL. ex:p1 qualifies, ex:p2 does not.
func TestOWLRL_AnonymousRestrictionClassifies(t *testing.T) {
	var (
		exProduct   = ex.Term("Product")
		exSoldIn    = ex.Term("sold_in")
		exNL        = ex.Term("Netherlands")
		exUK        = ex.Term("UK")
		exSoldInNL  = ex.Term("ProductSoldInNL")
		exP1, exP2  = ex.Term("p1"), ex.Term("p2")
		linkingPred = map[string]term.URIRef{
			"rdfs:subClassOf":     namespace.RDFS.SubClassOf,
			"owl:equivalentClass": namespace.OWL.EquivalentClass,
		}
	)

	for name, pred := range linkingPred {
		t.Run(name, func(t *testing.T) {
			g := graph.NewGraph()
			r := term.NewBNode("r")
			g.Add(r, namespace.RDF.Type, namespace.OWL.Restriction)
			g.Add(r, namespace.OWL.OnProperty, exSoldIn)
			g.Add(r, namespace.OWL.HasValue, exNL)
			g.Add(r, pred, exSoldInNL)

			g.Add(exP1, namespace.RDF.Type, exProduct)
			g.Add(exP1, exSoldIn, exNL)
			g.Add(exP2, namespace.RDF.Type, exProduct)
			g.Add(exP2, exSoldIn, exUK)

			if _, err := Expand(g, RDFS|OWLRL); err != nil {
				t.Fatalf("Expand: %v", err)
			}

			if !g.Contains(exP1, namespace.RDF.Type, exSoldInNL) {
				t.Error("p1 satisfies the restriction but was not classified")
			}
			if g.Contains(exP2, namespace.RDF.Type, exSoldInNL) {
				t.Error("p2 does not satisfy the restriction but was classified")
			}
		})
	}
}

// TestExpand_OWLFeedsRDFS covers the regime layering with named classes only.
// ExpandCheck runs the RDFS closure before the OWL RL closure, so any rdf:type
// triple produced by an OWL rule must still be seen by rdfs9.
//
//	ex:A owl:equivalentClass ex:B . ex:B rdfs:subClassOf ex:C . ex:x a ex:A .
//
// cax-eqc gives ex:x a ex:B, and rdfs9 must then give ex:x a ex:C.
func TestExpand_OWLFeedsRDFS(t *testing.T) {
	g := graph.NewGraph()
	g.Add(exA, namespace.OWL.EquivalentClass, exB)
	g.Add(exB, namespace.RDFS.SubClassOf, exC)
	g.Add(exX, namespace.RDF.Type, exA)

	if _, err := Expand(g, RDFS|OWLRL); err != nil {
		t.Fatalf("Expand: %v", err)
	}

	if !g.Contains(exX, namespace.RDF.Type, exB) {
		t.Error("cax-eqc did not infer x rdf:type B")
	}
	if !g.Contains(exX, namespace.RDF.Type, exC) {
		t.Error("rdfs9 did not run on the type triple that cax-eqc produced")
	}
}

// TestExpand_ReachesFixpoint checks that one Expand call returns a complete
// closure. Chaining an anonymous restriction to a named class needs cls-hv2 to
// feed rdfs9; if the two run in a fixed order within a single pass, a second
// call would still add triples.
func TestExpand_ReachesFixpoint(t *testing.T) {
	exSoldIn := ex.Term("sold_in")
	exNL := ex.Term("Netherlands")
	exSoldInNL := ex.Term("ProductSoldInNL")

	g := graph.NewGraph()
	r := term.NewBNode("r")
	g.Add(r, namespace.RDF.Type, namespace.OWL.Restriction)
	g.Add(r, namespace.OWL.OnProperty, exSoldIn)
	g.Add(r, namespace.OWL.HasValue, exNL)
	g.Add(r, namespace.RDFS.SubClassOf, exSoldInNL)
	g.Add(exX, exSoldIn, exNL)

	if _, err := Expand(g, RDFS|OWLRL); err != nil {
		t.Fatalf("first Expand: %v", err)
	}
	added, err := Expand(g, RDFS|OWLRL)
	if err != nil {
		t.Fatalf("second Expand: %v", err)
	}
	if added != 0 {
		t.Errorf("first Expand did not reach a fixpoint: second call added %d triples", added)
	}
}
