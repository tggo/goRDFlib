package shacl

import "testing"

// Regression test for issue #21, reported against sh:sparql. The root cause was
// in the SPARQL engine (see sparql/issue21_repro_test.go): a variable bound to
// a literal was silently treated as an unbound wildcard inside FILTER NOT
// EXISTS, so any ex:Widget in the graph satisfied the inner pattern and the
// literal-bound row was dropped instead of being reported as a violation.

const issue21Data = `
	@prefix ex: <http://example.com/> .

	ex:a a ex:Thing ; ex:prop "hello" .
	ex:b a ex:Thing ; ex:prop ex:dangling .
	ex:w a ex:Widget .
`

const issue21Shapes = `
	@prefix sh: <http://www.w3.org/ns/shacl#> .
	@prefix ex: <http://example.com/> .

	ex:ThingShape a sh:NodeShape ;
		sh:targetClass ex:Thing ;
		sh:sparql [
			sh:message "prop value is not a declared ex:Widget" ;
			sh:select """
				PREFIX ex: <http://example.com/>
				SELECT ?this ?value
				WHERE {
					$this ex:prop ?value .
					FILTER NOT EXISTS { ?value a ex:Widget . }
				}
			""" ;
		] .
`

func TestIssue21NotExistsLiteralValue(t *testing.T) {
	dataG, err := LoadTurtleString(issue21Data, "http://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	shapesG, err := LoadTurtleString(issue21Shapes, "http://example.com/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 violations (literal \"hello\" and dangling IRI); got %d", len(report.Results))
	}

	var sawLiteral, sawIRI bool
	for _, r := range report.Results {
		switch r.Value.String() {
		case `"hello"`:
			sawLiteral = true
		case "<http://example.com/dangling>":
			sawIRI = true
		}
	}
	if !sawLiteral {
		t.Error("literal value \"hello\" was not reported")
	}
	if !sawIRI {
		t.Error("dangling IRI value was not reported")
	}
}
