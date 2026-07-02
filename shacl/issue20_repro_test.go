package shacl

import "testing"

// Regression test for issue #20: a sh:sparql constraint that uses $this as a
// triple subject must stay scoped to the focus node when that focus node is a
// blank node. Previously $this was pre-bound by textual substitution, which
// produced "_:b0" in the query body; a blank node label in SPARQL text is a
// fresh query-scoped node (SPARQL 1.1 §4.1.4), not a reference to the data
// node, so the pattern matched every subject and the constraint fired on
// non-focus nodes. The fix pre-binds blank-node focus nodes via real initial
// bindings (SHACL §5.2.1.3 defines pre-binding as a solution binding).

const issue20Shape = `
	@prefix sh: <http://www.w3.org/ns/shacl#> .
	@prefix ex: <http://example.org/> .

	ex:PersonShape a sh:NodeShape ;
		sh:targetClass ex:Person ;
		sh:sparql [
			sh:message "bad flag" ;
			sh:select """
				PREFIX ex: <http://example.org/>
				SELECT $this WHERE {
					$this ex:flag ?f .
					FILTER(?f = "bad")
				}
			""" ;
		] .
`

func TestIssue20BlankNodeFocusScoped(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		_:alice a ex:Person ; ex:flag "bad"^^xsd:string .
		_:bob   a ex:Person .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	shapesG, err := LoadTurtleString(issue20Shape, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if len(report.Results) != 1 {
		t.Errorf("expected exactly 1 violation (only _:alice has ex:flag \"bad\"); got %d", len(report.Results))
		for _, r := range report.Results {
			t.Logf("  focus=%s value=%s msg=%v", r.FocusNode, r.Value, r.ResultMessages)
		}
	}
}

func TestIssue20IRIFocusScoped(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ; ex:flag "bad"^^xsd:string .
		ex:Bob   a ex:Person .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	shapesG, err := LoadTurtleString(issue20Shape, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if len(report.Results) != 1 {
		t.Errorf("expected exactly 1 violation (only ex:Alice has ex:flag \"bad\"); got %d", len(report.Results))
	}
}
