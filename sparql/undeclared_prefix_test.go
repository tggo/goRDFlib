package sparql

import (
	"errors"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/turtle"
)

// Issue #49: an undeclared prefix used to resolve to nothing, and a term that
// resolves to nothing matches like a variable — so a typo returned every node.
func TestUndeclaredPrefixIsAParseError(t *testing.T) {
	queries := []string{
		`SELECT ?e WHERE { ?e a nosuch:Tool }`,
		`SELECT ?e WHERE { ?e nosuch:p ?o }`,
		`SELECT ?e WHERE { ?e nosuch:p+ ?o }`,
		`SELECT ?e WHERE { ?e ?p "1"^^nosuch:int }`,
		`SELECT ?e WHERE { ?e ?p ?o FILTER(?o = nosuch:x) }`,
		`SELECT (nosuch:f(1) AS ?x) WHERE {}`,
		`SELECT * FROM nosuch:g WHERE { ?s ?p ?o }`,
		`PREFIX ex: <http://example.org/> SELECT ?e WHERE { ?e a ex:T . ?e a nosuch:T }`,
	}
	for _, q := range queries {
		_, err := Parse(q)
		if !errors.Is(err, ErrUndeclaredPrefix) {
			t.Errorf("Parse(%q) = %v, want ErrUndeclaredPrefix", q, err)
		} else if !strings.Contains(err.Error(), `"nosuch"`) || !strings.Contains(err.Error(), "pos ") {
			t.Errorf("error %q does not name the prefix and position", err)
		}
	}

	updates := []string{
		`INSERT DATA { <http://a> nosuch:p 1 }`,
		`PREFIX ex: <http://e/> INSERT DATA { ex:a ex:p 1 } ; INSERT DATA { ex:a nosuch:p 1 }`,
	}
	for _, u := range updates {
		if _, err := ParseUpdate(u); !errors.Is(err, ErrUndeclaredPrefix) {
			t.Errorf("ParseUpdate(%q) = %v, want ErrUndeclaredPrefix", u, err)
		}
	}

	// A prefix declared before a later update operation is in scope there.
	if _, err := ParseUpdate(`INSERT DATA { <http://a> <http://p> 1 } ; PREFIX ex: <http://e/> INSERT DATA { ex:a ex:p 1 }`); err != nil {
		t.Errorf("prefix declared between operations: %v", err)
	}
	// The empty prefix needs a declaration too.
	if _, err := Parse(`SELECT * WHERE { :a ?p ?o }`); !errors.Is(err, ErrUndeclaredPrefix) {
		t.Errorf("empty prefix: %v", err)
	}
	if _, err := Parse(`PREFIX : <http://e/> SELECT * WHERE { :a ?p ?o . _:b ?p "x:y" }`); err != nil {
		t.Errorf("declared prefixes, blank node, colon in string: %v", err)
	}

	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(`@prefix ex: <http://example.org/> . ex:a a ex:Tool . ex:b a ex:Person .`)); err != nil {
		t.Fatal(err)
	}
	if res, err := Query(g, `SELECT ?e WHERE { ?e a nosuch:Tool }`); err == nil {
		t.Fatalf("Query returned %d rows and no error", len(res.Bindings))
	}
}
