package sparql

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/turtle"
)

// #57, faults 1 and 4. EXISTS substitutes the outer row into its pattern
// (§18.6), so a FILTER inside NOT EXISTS sees ?f bound outside; and an IRI
// may follow a FILTER NOT EXISTS group without a '.'.
func TestIssue57SPARQL(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph()
	if err := turtle.Parse(ctx, g, strings.NewReader(`@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
ex:asg ex:derivedFrom ex:rec ; ex:from "2026-10-10T12:00:00+02:00"^^xsd:dateTime .
ex:rec ex:observedAt "2026-10-10T12:00:00+02:00"^^xsd:dateTime .
ex:a ex:q ex:b .`)); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		q    string
		want bool
	}{
		"not exists sees outer var": {`PREFIX ex: <http://example.org/> ASK { ex:asg ex:derivedFrom ?rec ; ex:from ?f .
			FILTER (NOT EXISTS { ?rec ex:observedAt ?t . FILTER (?t = ?f) }) }`, false},
		"exists sees outer var": {`PREFIX ex: <http://example.org/> ASK { ex:asg ex:derivedFrom ?rec ; ex:from ?f .
			FILTER (EXISTS { ?rec ex:observedAt ?t . FILTER (?t = ?f) }) }`, true},
		"control": {`PREFIX ex: <http://example.org/> ASK { ex:asg ex:derivedFrom ?rec ; ex:from ?f .
			FILTER (NOT EXISTS { ?rec ex:observedAt ?f }) }`, false},
		"iri after group": {`PREFIX ex: <http://example.org/> ASK { FILTER NOT EXISTS { ?x ex:p ?y } <http://example.org/a> ex:q ?z }`, true},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := QueryContext(ctx, g, c.q)
			if err != nil {
				t.Fatal(err)
			}
			if r.AskResult != c.want {
				t.Errorf("ASK %v, want %v", r.AskResult, c.want)
			}
		})
	}
}
