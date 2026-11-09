package sparql

import "testing"

// SEPARATOR takes a string literal. Anything else made the parser panic on a
// type assertion (found by FuzzParse with "SELECT(GROUP_CONCAT(0;SEPARATOR=a ";
// the input stays in the fuzz corpus).
func TestGroupConcatSeparatorMustBeALiteral(t *testing.T) {
	for _, q := range []string{
		`SELECT (GROUP_CONCAT(?o; SEPARATOR=a) AS ?x) WHERE { ?s ?p ?o }`,
		`SELECT (GROUP_CONCAT(?o; SEPARATOR=<urn:x>) AS ?x) WHERE { ?s ?p ?o }`,
		`SELECT(GROUP_CONCAT(0;SEPARATOR=a `,
	} {
		if _, err := Parse(q); err == nil {
			t.Errorf("accepted %q", q)
		}
	}
	if _, err := Parse(`SELECT (GROUP_CONCAT(?o; SEPARATOR=", ") AS ?x) WHERE { ?s ?p ?o }`); err != nil {
		t.Errorf("rejected a literal separator: %v", err)
	}
}
