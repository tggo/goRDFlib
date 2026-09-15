package sparql

import "testing"

// A graph reference must be DEFAULT, NAMED, ALL or an IRI (grammar [46]-[47]).
// Anything else used to parse and act on a graph nobody has, so a typo such as
// CLEAR XYZ reported success and changed nothing.
func TestParseUpdate_GraphRefMustBeIRI(t *testing.T) {
	for _, bad := range []string{
		"CLEAR XYZ",
		"DROP GRAPH ?g",
		"CLEAR GRAPH \"lit\"",
		"CREATE GRAPH _:b",
		"ADD XYZ TO DEFAULT",
		"COPY DEFAULT TO undeclared:g",
		"MOVE GRAPH",
	} {
		if _, err := ParseUpdate(bad); err == nil {
			t.Errorf("%q parsed; want a syntax error", bad)
		}
	}
	for _, good := range []string{
		"CLEAR DEFAULT",
		"CLEAR NAMED",
		"DROP ALL",
		"CLEAR GRAPH <http://e/g>",
		"DROP SILENT <http://e/g>",
		"PREFIX e: <http://e/> CREATE GRAPH e:g",
		"ADD <http://e/a> TO GRAPH <http://e/b>",
		"COPY DEFAULT TO <http://e/b>",
	} {
		if _, err := ParseUpdate(good); err != nil {
			t.Errorf("%q: %v", good, err)
		}
	}
}
