package sparql

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/turtle"
)

// A FILTER at the top level of an OPTIONAL is the LeftJoin's condition and
// sees the left side's variables (SPARQL 1.1 §18.2.2, #58). Every evaluation
// path of OPTIONAL is covered: plain, streamed (LIMIT pushdown), pre-bound,
// and inside EXISTS.
func TestOptionalFilterSeesLeftSide(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph()
	if err := turtle.Parse(ctx, g, strings.NewReader(`@prefix ex: <http://example.org/> .
ex:b ex:q ex:a . ex:a ex:p ex:b . ex:a ex:p ex:c .`)); err != nil {
		t.Fatal(err)
	}
	const pre = `PREFIX ex: <http://example.org/> `
	for name, c := range map[string]struct {
		q    string
		want string // ?y of the single row; "" means unbound
	}{
		"plain":           {`SELECT ?y WHERE { ?o ex:q ?z OPTIONAL { ?z ex:p ?y FILTER (?y = ?o) } }`, "http://example.org/b"},
		"limit":           {`SELECT ?y WHERE { ?o ex:q ?z OPTIONAL { ?z ex:p ?y FILTER (?y = ?o) } } LIMIT 5`, "http://example.org/b"},
		"no match":        {`SELECT ?y WHERE { ?o ex:q ?z OPTIONAL { ?z ex:p ?y FILTER (?y != ?o && ?y != ex:c) } }`, ""},
		"two filters":     {`SELECT ?y WHERE { ?o ex:q ?z OPTIONAL { ?z ex:p ?y FILTER (?y = ?o) FILTER (isIRI(?y)) } }`, "http://example.org/b"},
		"nested group":    {`SELECT ?y WHERE { ?o ex:q ?z OPTIONAL { { ?z ex:p ?y FILTER (?y = ?o) } } }`, ""},
		"nested limit":    {`SELECT ?y WHERE { ?o ex:q ?z OPTIONAL { { ?z ex:p ?y FILTER (?y = ?o) } } } LIMIT 5`, ""},
		"nested is local": {`SELECT ?y WHERE { ?o ex:q ?z OPTIONAL { ?z ex:p ?y { FILTER (bound(?o)) } } } LIMIT 1`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := QueryContext(ctx, g, pre+c.q)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Bindings) != 1 {
				t.Fatalf("%d rows, want 1: %v", len(r.Bindings), r.Bindings)
			}
			got := ""
			if y := r.Bindings[0]["y"]; y != nil {
				got = y.String()
			}
			if got != c.want {
				t.Errorf("?y = %q, want %q", got, c.want)
			}
		})
	}
	r, err := QueryContext(ctx, g, pre+`ASK { ?o ex:q ?z OPTIONAL { ?z ex:p ?y FILTER (?y = ?o) } FILTER (!bound(?y)) }`)
	if err != nil || r.AskResult {
		t.Errorf("ASK = %v, %v; want false (?y is bound for the only row)", r.AskResult, err)
	}
	r, err = QueryContext(ctx, g, pre+`ASK { ?s ?p ?x FILTER EXISTS { ?o ex:q ?z OPTIONAL { ?z ex:p ?y FILTER (?y = ?o) } FILTER (!bound(?y)) } }`)
	if err != nil || r.AskResult {
		t.Errorf("EXISTS ASK = %v, %v; want false", r.AskResult, err)
	}
}
