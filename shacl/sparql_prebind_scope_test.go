package shacl

import "testing"

// $this used only inside EXISTS or the right side of OPTIONAL. Spliced into
// the text these worked; passed as a binding they did not, until the engine
// let every expression see pre-bound values (evalCtx.pre).
func TestSPARQLConstraint_ThisInNestedScopes(t *testing.T) {
	for name, sel := range map[string]string{
		"not-exists-only": `SELECT $this WHERE { FILTER NOT EXISTS { $this <http://example.org/label> ?l } }`,
		"exists-filter":   `SELECT $this WHERE { FILTER NOT EXISTS { FILTER ($this = <http://example.org/good>) } }`,
		"optional-filter": `SELECT $this WHERE { OPTIONAL { ?x <http://example.org/label> ?l FILTER (?x = $this) } FILTER (!bound(?l)) }`,
	} {
		g := afGraph(t, `
ex:good a ex:T ; ex:label "x" .
ex:bad a ex:T .
ex:S a sh:NodeShape ; sh:targetClass ex:T ;
    sh:sparql [ sh:select """`+sel+`""" ] .
`)
		r := Validate(g, g)
		if len(r.Results) != 1 || r.Results[0].FocusNode != exIRI("bad") {
			t.Errorf("%s: %d results, want only ex:bad", name, len(r.Results))
		}
	}
}

// $currentShape of a blank-node shape. Spliced into the text, the label was
// a query-scoped blank node, which matches any subject: the shape seemed to
// carry a label that another node has, and every focus node was reported.
func TestSPARQLConstraint_BlankCurrentShape(t *testing.T) {
	g := afGraph(t, `
ex:a a ex:T .
ex:other rdfs:label "other" .
[] a sh:NodeShape ; sh:targetClass ex:T ; rdfs:label "anon" ;
    sh:sparql [ sh:select """
        SELECT $this WHERE { $currentShape <http://www.w3.org/2000/01/rdf-schema#label> "other" }
    """ ] .
`)
	if r := Validate(g, g); len(r.Results) != 0 {
		t.Errorf("%d results, want 0: $currentShape matched a node other than the shape", len(r.Results))
	}
}

// A sub-SELECT that uses $this without projecting it. SHACL defines
// pre-binding as substitution (§5.3.1), and while $this was spliced into the
// text such shapes worked, so they must keep working as a binding.
func TestSPARQLConstraint_ThisInUnprojectedSubSelect(t *testing.T) {
	for name, c := range map[string]struct {
		sel  string
		want []string
	}{
		"filter": {`SELECT $this ?value WHERE { { SELECT ?value WHERE { $this <http://example.org/p> ?value } } FILTER (?value > 3) }`, []string{"b"}},
		"count":  {`SELECT $this WHERE { { SELECT (COUNT(*) AS ?n) WHERE { $this <http://example.org/p> ?v } } FILTER (?n = 1) }`, []string{"a", "b"}},
	} {
		g := afGraph(t, `
ex:a a ex:T ; ex:p 1 .
ex:b a ex:T ; ex:p 5 .
ex:S a sh:NodeShape ; sh:targetClass ex:T ;
    sh:sparql [ sh:select """`+c.sel+`""" ] .
`)
		r := Validate(g, g)
		got := map[string]bool{}
		for _, res := range r.Results {
			got[res.FocusNode.Value()] = true
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: focus nodes %v, want %v", name, got, c.want)
			continue
		}
		for _, w := range c.want {
			if !got[ex+w] {
				t.Errorf("%s: focus nodes %v, want %v", name, got, c.want)
			}
		}
	}
}
