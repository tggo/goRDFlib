package sparql

import "testing"

// These are standard SPARQL forms encountered in SHACL constraint queries.
func TestParseNumericTripleTerminatorAndHavingSubquery(t *testing.T) {
	for name, query := range map[string]string{
		"integer followed by dot":        `SELECT * WHERE { ?s <urn:p> 1. ?s <urn:q> 2. }`,
		"signed integer followed by dot": `SELECT * WHERE { ?s <urn:p> -1. ?s <urn:q> +2. }`,
		"decimal and double":             `SELECT * WHERE { ?s <urn:p> 1.2. ?s <urn:q> 1.e3. }`,
		"dot followed by prefixed name":  `PREFIX ex: <urn:> SELECT * WHERE { ?s ex:p 1.ex:s ex:q 2. }`,
		"subquery having":                `SELECT ?s WHERE { { SELECT ?s (COUNT(?o) AS ?n) WHERE { ?s <urn:p> ?o } GROUP BY ?s HAVING (COUNT(?o) > 1) } }`,
		"multiple having constraints":    `SELECT ?s WHERE { { SELECT ?s WHERE { ?s <urn:p> ?o } GROUP BY ?s HAVING (COUNT(?o) > 1) (COUNT(?o) < 5) } }`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(query); err != nil {
				t.Fatalf("Parse(%q): %v", query, err)
			}
		})
	}
}
