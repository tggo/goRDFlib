package shacl

import (
	"errors"
	"testing"
)

// $this in a sub-SELECT's projection and GROUP BY of an sh:select is the
// focus node, as in sh:construct (#55). Written into the text as an IRI it
// is not SPARQL; the constraint then failed to parse for every IRI focus node.
func TestSPARQLConstraint_ThisInSubSelect(t *testing.T) {
	g := afGraph(t, `
ex:heavy a ex:Assembly ; ex:hasPart ex:engine .
ex:light a ex:Assembly ; ex:hasPart ex:frame .
_:bnode a ex:Assembly ; ex:hasPart ex:engine .
ex:engine ex:mass 90 . ex:frame ex:mass 7 .

ex:AssemblyShape a sh:NodeShape ;
    sh:targetClass ex:Assembly ;
    sh:sparql [
        sh:select """
            SELECT $this ?total
            WHERE {
                { SELECT $this (SUM(?m) AS ?total)
                  WHERE { $this <http://example.org/hasPart> ?p . ?p <http://example.org/mass> ?m }
                  GROUP BY $this }
                FILTER (?total > 50)
            }
        """ ;
    ] .
`)
	var reported []error
	r := Validate(g, g, WithErrorHandler(func(err error) { reported = append(reported, err) }))
	if len(reported) != 0 {
		t.Fatalf("errors reported: %v", reported)
	}
	if len(r.Results) != 2 {
		t.Fatalf("%d results, want 2 (ex:heavy and the blank node): %+v", len(r.Results), r.Results)
	}
	for _, res := range r.Results {
		if res.FocusNode == exIRI("light") {
			t.Errorf("ex:light is under the limit but was reported")
		}
	}
}

// A sh:select that does not parse still produces a violation (a constraint
// that cannot be evaluated must not look satisfied), and the parse error goes
// to WithErrorHandler as ErrMalformedConstraint, once per constraint per run,
// so a broken shape is distinguishable from data that violates it (#54).
func TestSPARQLConstraint_ParseErrorIsReported(t *testing.T) {
	g := afGraph(t, `
ex:a a ex:Thing . ex:b a ex:Thing .

ex:ThingShape a sh:NodeShape ;
    sh:targetClass ex:Thing ;
    sh:sparql [ sh:select "SELECT $this WHERE { $this undeclared:p ?x }" ] .
`)
	var reported []error
	r := Validate(g, g, WithErrorHandler(func(err error) { reported = append(reported, err) }))
	if r.Conforms || len(r.Results) != 2 {
		t.Errorf("conforms=%v with %d results, want a violation per focus node", r.Conforms, len(r.Results))
	}
	if len(reported) != 1 || !errors.Is(reported[0], ErrMalformedConstraint) {
		t.Fatalf("reported %v, want one ErrMalformedConstraint", reported)
	}
}
