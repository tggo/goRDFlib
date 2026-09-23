package shacl

import (
	"strings"
	"testing"
)

// A SPARQL-based constraint is evaluated against the data graph (SHACL
// §5.2.1). A FROM clause in sh:select names a graph the shapes graph has no
// way to supply, so the query must keep running against the data graph rather
// than being rejected for a dataset that cannot be built. Before the engine
// honored dataset clauses this happened by itself; now the bridge has to say
// so, and this test is what notices if it stops.
func TestSPARQLConstraintIgnoresDatasetClause(t *testing.T) {
	data := loadTurtle(t, `
@prefix ex: <http://example.org/> .
ex:ok a ex:Thing ; ex:value "good" .
ex:bad a ex:Thing .
`)
	shapes := loadTurtle(t, `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:ThingShape a sh:NodeShape ;
    sh:targetClass ex:Thing ;
    sh:sparql [
        sh:message "no ex:value" ;
        sh:select """
            SELECT $this
            FROM <http://example.org/nowhere>
            WHERE { $this a <http://example.org/Thing> . FILTER NOT EXISTS { $this <http://example.org/value> ?v } }
        """ ;
    ] .
`)

	var reported []error
	report := Validate(data, shapes, WithErrorHandler(func(err error) {
		reported = append(reported, err)
	}))

	if len(reported) != 0 {
		t.Errorf("errors reported: %v", reported)
	}
	if report.Conforms {
		t.Fatal("conforms = true, want the ex:bad violation")
	}
	if len(report.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(report.Results))
	}
	if got := report.Results[0].FocusNode.Value(); got != "http://example.org/bad" {
		t.Errorf("focus node = %s, want http://example.org/bad", got)
	}
}

// The same for a SHACL function, whose body is a query from the shapes graph
// too, and which reaches the engine by a different path.
func TestSHACLFunctionIgnoresDatasetClause(t *testing.T) {
	got := inferObjects(t, `
ex:focus ex:width 6 .

ex:double a sh:SPARQLFunction ;
    sh:parameter [ sh:path ex:op1 ] ;
    sh:select "SELECT ($op1 * 2 AS ?result) FROM <http://example.org/nowhere> WHERE { }" .
`, `[ ex:double ( [ sh:path ex:width ] ) ]`)

	if len(got) != 1 || !strings.Contains(got[0], "12") {
		t.Errorf("got %v, want 12", got)
	}
}
