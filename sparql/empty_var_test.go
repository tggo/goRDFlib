package sparql

import "testing"

// A ? or $ with no name is not a variable (VARNAME is one or more
// characters). It parsed as a variable called "", which the fuzz target of the
// optimisations found acting differently with the optimisations on and off.
func TestEmptyVariableNameIsRejected(t *testing.T) {
	for _, q := range []string{
		`SELECT { BIND (1 AS $}`,
		`SELECT * WHERE { ? <urn:p> <urn:o> }`,
		`SELECT * WHERE { ?s <urn:p> $ }`,
		`SELECT * WHERE { ?s <urn:p> ?o FILTER (? = 1) }`,
	} {
		if _, err := Parse(q); err == nil {
			t.Errorf("accepted %q", q)
		}
	}
	if _, err := ParseUpdate(`INSERT { ? <urn:p> <urn:o> } WHERE { ?s ?p ?o }`); err == nil {
		t.Error("update accepted a bare ?")
	}
	if _, err := Parse(`SELECT * WHERE { ?s <urn:p>? ?o }`); err != nil {
		t.Errorf("path modifier ? rejected: %v", err)
	}
}
