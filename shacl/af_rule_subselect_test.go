package shacl

import "testing"

// $this inside a sub-SELECT is the focus node too (SHACL §5.3.1: pre-bound
// variables are bound in every subquery). Splicing the focus node into the
// query text put an IRI into the sub-SELECT's projection and GROUP BY, which
// is not SPARQL, so the rule was malformed for every IRI focus node (#55).
// Blank focus nodes always took the binding path; both kinds are checked.
func TestAF_SPARQLRule_ThisInSubSelect(t *testing.T) {
	g := afGraph(t, `
ex:car a ex:Assembly ; ex:hasPart ex:wheel, ex:engine .
_:bike a ex:Assembly ; ex:hasPart ex:frame .
ex:wheel ex:mass 10 . ex:engine ex:mass 90 . ex:frame ex:mass 7 .
ex:other a ex:Assembly ; ex:hasPart ex:tiny .
ex:tiny ex:mass 1 .

ex:AssemblyShape a sh:NodeShape ;
    sh:targetClass ex:Assembly ;
    sh:rule [
        a sh:SPARQLRule ;
        sh:construct """
            CONSTRUCT { $this <http://example.org/totalMass> ?total }
            WHERE {
                { SELECT $this (SUM(?m) AS ?total)
                  WHERE { $this <http://example.org/hasPart> ?p . ?p <http://example.org/mass> ?m }
                  GROUP BY $this }
            }
        """ ;
    ] .
`)
	applyRules(t, g, g)
	total := IRI(ex + "totalMass")
	if !hasTriple(g, exIRI("car"), total, Literal("100", XSD+"integer", "")) {
		t.Errorf("no totalMass 100 for ex:car")
	}
	if !hasTriple(g, exIRI("other"), total, Literal("1", XSD+"integer", "")) {
		t.Errorf("no totalMass 1 for ex:other")
	}
	n := 0
	for _, tr := range g.All(nil, &total, nil) {
		_ = tr
		n++
	}
	if n != 3 {
		t.Errorf("%d totalMass triples, want 3 (one per assembly; the sub-SELECT must not range over other nodes)", n)
	}
}
