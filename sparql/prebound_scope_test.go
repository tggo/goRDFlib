package sparql

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// An initial binding is a constant substituted into the query (SHACL §5.3.1),
// so every expression sees it, wherever it sits. Rows reach EXISTS, GRAPH,
// the right side of OPTIONAL and subqueries without the pre-bound values; each
// case here failed before evalCtx.pre, once SHACL stopped splicing $this into
// the query text and passed it as a binding.
func TestPreBoundVisibleInEveryScope(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph()
	a := term.NewURIRefUnsafe("urn:a")
	p := term.NewURIRefUnsafe("urn:p")
	if err := g.Add(ctx, a, p, a); err != nil {
		t.Fatal(err)
	}
	ng := graph.NewGraph()
	if err := ng.Add(ctx, a, p, a); err != nil {
		t.Fatal(err)
	}
	for name, q := range map[string]string{
		"filter":     `SELECT * WHERE { ?s ?p ?o FILTER(?x = <urn:a>) }`,
		"exists":     `SELECT * WHERE { ?s ?p ?o FILTER EXISTS { FILTER(?x = <urn:a>) } }`,
		"not exists": `SELECT * WHERE { ?s ?p ?o FILTER NOT EXISTS { FILTER(?x != <urn:a>) } }`,
		"graph":      `SELECT * WHERE { GRAPH <urn:g> { ?s ?p ?o FILTER(?x = <urn:a>) } }`,
		"optional":   `SELECT * WHERE { ?s ?p ?o OPTIONAL { ?s ?p ?z FILTER(?x = <urn:a>) } FILTER(bound(?z)) }`,
		"union":      `SELECT * WHERE { { ?s ?p ?o FILTER(?x = <urn:a>) } UNION { FILTER(false) } }`,
		"group":      `SELECT * WHERE { ?s ?p ?o { FILTER(?x = <urn:a>) } }`,
		"bind":       `SELECT * WHERE { ?s ?p ?o BIND(?x AS ?w) FILTER(bound(?w)) }`,
		"bound":      `SELECT * WHERE { ?s ?p ?o FILTER EXISTS { FILTER(bound(?x)) } }`,
		"subselect":  `SELECT * WHERE { { SELECT ?x WHERE { FILTER(?x = <urn:a>) } } }`,
		"nested":     `SELECT * WHERE { { SELECT ?x WHERE { { SELECT ?x WHERE { FILTER(?x = <urn:a>) } } } } }`,
		"sub exists": `SELECT * WHERE { { SELECT ?x WHERE { FILTER NOT EXISTS { FILTER(?x != <urn:a>) } } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			pq, err := Parse(q)
			if err != nil {
				t.Fatal(err)
			}
			pq.NamedGraphs = map[string]*graph.Graph{"urn:g": ng}
			res, err := EvalQuery(g, pq, map[string]term.Term{"x": a})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Bindings) == 0 {
				t.Error("no rows: the pre-bound ?x did not reach the expression")
			}
		})
	}
}
