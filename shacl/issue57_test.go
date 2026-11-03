package shacl

import "testing"

// #57, faults 2 and 3, as reported: $this inside an OPTIONAL sub-SELECT with
// GROUP BY $this, and a constraint that opens with FILTER NOT EXISTS { $this
// ... FILTER (...) }.
func TestIssue57SHACL(t *testing.T) {
	for name, c := range map[string]struct {
		shapes, data, want string
	}{
		"this in sub-select": {`
@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
ex:S a sh:NodeShape ; sh:targetClass ex:T ; sh:sparql [ sh:message "needs two e" ; sh:select """
PREFIX ex: <http://example.org/>
SELECT $this WHERE {
  OPTIONAL { SELECT $this (COUNT(DISTINCT ?e) AS ?n) WHERE { $this ex:e ?e } GROUP BY $this }
  FILTER (COALESCE(?n, 0) < 2) }""" ] .`, `@prefix ex: <http://example.org/> .
ex:a a ex:T ; ex:e ex:e1 , ex:e2 , ex:e3 .
ex:b a ex:T ; ex:e ex:e1 .`, "http://example.org/b"},
		"not exists first": {`
@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
@prefix skos: <http://www.w3.org/2004/02/skos/core#> .
ex:S a sh:NodeShape ; sh:targetClass ex:Consignment ; sh:sparql [ sh:message "m" ; sh:select """
PREFIX ex: <http://example.org/>
PREFIX skos: <http://www.w3.org/2004/02/skos/core#>
SELECT $this WHERE {
  FILTER NOT EXISTS { $this ex:outcome ?o . FILTER (?o != ex:handed-over) }
  $this ex:hasLine ?l . ?l ex:species ?sp .
  FILTER NOT EXISTS { ?sp skos:inScheme ex:Species } }""" ] .`, `
@prefix ex: <http://example.org/> . @prefix skos: <http://www.w3.org/2004/02/skos/core#> .
ex:c1 a ex:Consignment ; ex:hasLine ex:l1 . ex:l1 ex:species ex:sp1 . ex:sp1 skos:inScheme ex:Species .
ex:c2 a ex:Consignment ; ex:hasLine ex:l2 . ex:l2 ex:species ex:sp2 .`, "http://example.org/c2"},
	} {
		t.Run(name, func(t *testing.T) {
			shapes, err := LoadTurtleString(c.shapes, "")
			if err != nil {
				t.Fatal(err)
			}
			data, err := LoadTurtleString(c.data, "")
			if err != nil {
				t.Fatal(err)
			}
			r := Validate(data, shapes)
			if len(r.Results) != 1 || r.Results[0].FocusNode.Value() != c.want {
				t.Errorf("results %v, want one for %s", r.Results, c.want)
			}
		})
	}
}
