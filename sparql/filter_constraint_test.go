package sparql

import "testing"

// A FILTER constraint is a bracketed expression, a built-in call or a function
// call (rule [69]); the '.' after it is optional (rule [54]). So a triple
// written with an IRI subject may follow a FILTER whose constraint is not
// bracketed. The parser used to read past the call looking for an operator and
// took that IRI's '<' for less-than. SHACL-SPARQL hits this whenever $this is
// substituted with an IRI after a FILTER NOT EXISTS { ... }.
func TestFilter_UnbracketedConstraintThenIRI(t *testing.T) {
	const data = `<urn:a> <urn:p> <urn:b> .`
	for name, q := range map[string]string{
		"NOT EXISTS":         `SELECT ?o WHERE { FILTER NOT EXISTS { <urn:a> <urn:q> ?x } <urn:a> <urn:p> ?o }`,
		"EXISTS":             `SELECT ?o WHERE { FILTER EXISTS { <urn:a> <urn:p> ?x } <urn:a> <urn:p> ?o }`,
		"newline":            "SELECT ?o WHERE { FILTER NOT EXISTS { <urn:a> <urn:q> ?x }\n<urn:a> <urn:p> ?o }",
		"bound":              `SELECT ?o WHERE { FILTER bound(?o) <urn:a> <urn:p> ?o }`,
		"isIRI":              `SELECT ?o WHERE { FILTER isIRI(?o) <urn:a> <urn:p> ?o }`,
		"regex":              `SELECT ?o WHERE { FILTER regex(str(?o), "b") <urn:a> <urn:p> ?o }`,
		"sameTerm":           `SELECT ?o WHERE { FILTER sameTerm(?o, <urn:b>) <urn:a> <urn:p> ?o }`,
		"bracketed negation": `SELECT ?o WHERE { FILTER (!isLiteral(?o)) <urn:a> <urn:p> ?o }`,
		"bracketed":          `SELECT ?o WHERE { FILTER (NOT EXISTS { <urn:a> <urn:q> ?x }) <urn:a> <urn:p> ?o }`,
		"bracketed then":     `SELECT ?o WHERE { FILTER (?o = <urn:b>) <urn:a> <urn:p> ?o }`,
	} {
		t.Run(name, func(t *testing.T) {
			if n := countRows(t, data, q); n != 1 {
				t.Errorf("%d rows, want 1", n)
			}
		})
	}
}
