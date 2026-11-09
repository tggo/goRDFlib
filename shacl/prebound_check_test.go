package shacl

import (
	"context"
	"errors"
	"testing"
)

func runTurtleContext(t *testing.T, ttl string) (ValidationReport, error) {
	t.Helper()
	g, err := LoadTurtleString(`@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://e/> .
`+ttl, "http://e/")
	if err != nil {
		t.Fatal(err)
	}
	return ValidateContext(context.Background(), g, g)
}

// A query that breaks the pre-binding restrictions is not run: the run fails.
func TestPreBoundRestrictionFailsTheRun(t *testing.T) {
	for name, q := range map[string]string{
		"MINUS":          `SELECT $this WHERE { $this ?p ?o MINUS { $this ?p 1 } }`,
		"VALUES":         `SELECT $this WHERE { VALUES $this { ex:a } }`,
		"AS pre-bound":   `SELECT $this WHERE { BIND (true AS $this) }`,
		"in a subquery":  `SELECT $this WHERE { { SELECT $this WHERE { $this ?p ?o MINUS { $this ?p 1 } } } }`,
		"inside EXISTS":  `SELECT $this WHERE { FILTER EXISTS { $this ?p ?o MINUS { $this ?p 1 } } }`,
		"in a component": "",
	} {
		ttl := `ex:S a sh:NodeShape ; sh:targetNode ex:a ; sh:sparql [ sh:select """` + q + `""" ] .`
		if q == "" {
			ttl = `ex:C a sh:ConstraintComponent ; sh:parameter [ sh:path ex:p ] ;
  sh:validator [ a sh:SPARQLSelectValidator ; sh:select """SELECT $this WHERE { $this ?x ?y MINUS { $this ?x 1 } }""" ] .
ex:S a sh:NodeShape ; sh:targetNode ex:a ; ex:p 1 .`
		}
		_, err := runTurtleContext(t, ttl)
		if !errors.Is(err, ErrPreBoundRestriction) {
			t.Errorf("%s: err = %v, want ErrPreBoundRestriction", name, err)
		}
	}
}

// The same constructs are fine for a variable that is not pre-bound.
func TestPreBoundRestrictionIgnoresOtherVariables(t *testing.T) {
	r, err := runTurtleContext(t, `ex:S a sh:NodeShape ; sh:targetNode ex:a ; sh:sparql [ sh:select """
SELECT $this WHERE { VALUES ?x { 1 } $this ?p ?o FILTER NOT EXISTS { ?o ?p ?x } BIND (1 AS ?y) }""" ] .
ex:a ex:q ex:b .`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Results) != 1 {
		t.Fatalf("want the query to run and yield a result, got %+v", r.Results)
	}
}

func TestSPARQLFailureBinding(t *testing.T) {
	_, err := runTurtleContext(t, `ex:S a sh:NodeShape ; sh:targetNode ex:a ;
  sh:sparql [ sh:select "SELECT $this (true AS ?failure) WHERE { }" ] .`)
	if !errors.Is(err, ErrQueryReportedFailure) {
		t.Fatalf("err = %v, want ErrQueryReportedFailure", err)
	}
	// ?failure false is an ordinary solution.
	r, err := runTurtleContext(t, `ex:S a sh:NodeShape ; sh:targetNode ex:a ;
  sh:sparql [ sh:select "SELECT $this (false AS ?failure) WHERE { }" ] .`)
	if err != nil || len(r.Results) != 1 {
		t.Fatalf("results %+v, err %v", r.Results, err)
	}
}
