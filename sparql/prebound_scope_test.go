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

// A subquery that does not project a pre-bound variable has its own variable
// of that name (SPARQL 1.1 §18.2.1): the initial binding must not reach it.
// The first version of evalCtx.pre leaked into every subquery.
func TestPreBoundDoesNotReachUnprojectedSubquery(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph()
	a := term.NewURIRefUnsafe("urn:a")
	b := term.NewURIRefUnsafe("urn:b")
	p := term.NewURIRefUnsafe("urn:p")
	for _, tr := range [][2]term.URIRef{{a, b}, {b, a}} {
		if err := g.Add(ctx, tr[0], p, tr[1]); err != nil {
			t.Fatal(err)
		}
	}
	for name, c := range map[string]struct {
		q    string
		rows int
		sub  bool // PreBindSubqueries
	}{
		"bound":           {`SELECT ?s2 WHERE { { SELECT ?s2 WHERE { ?s2 <urn:p> ?o2 FILTER(!bound(?x)) } } }`, 2, false},
		"compare":         {`SELECT ?s2 WHERE { { SELECT ?s2 WHERE { ?s2 <urn:p> ?o2 FILTER(?o2 != ?x) } } }`, 0, false}, // ?x unbound: an error
		"bind":            {`SELECT ?s2 WHERE { { SELECT ?s2 ?w WHERE { ?s2 <urn:p> ?o2 BIND(?x AS ?w) } } FILTER(!bound(?w)) }`, 2, false},
		"count":           {`SELECT ?c WHERE { { SELECT (COUNT(?x) AS ?c) WHERE { ?s2 <urn:p> ?o2 } } FILTER(?c = 0) }`, 1, false},
		"count star":      {`SELECT ?n WHERE { { SELECT (COUNT(*) AS ?n) WHERE { ?x <urn:p> ?o } } FILTER(?n = 1) }`, 1, true},
		"star":            {`SELECT ?s2 WHERE { { SELECT * WHERE { ?s2 <urn:p> ?o2 FILTER(!bound(?x)) } } }`, 2, false},
		"star in scope":   {`SELECT ?o WHERE { { SELECT * WHERE { ?x <urn:p> ?o } } }`, 1, false},
		"projected":       {`SELECT ?x WHERE { { SELECT ?x WHERE { ?x <urn:p> ?o } } }`, 1, false},
		"substitute":      {`SELECT ?s2 WHERE { { SELECT ?s2 WHERE { ?s2 <urn:p> ?o2 FILTER(?o2 != ?x) } } }`, 1, true},
		"substitute bgp":  {`SELECT ?o WHERE { { SELECT ?o WHERE { ?x <urn:p> ?o } } }`, 1, true},
		"no substitution": {`SELECT ?o WHERE { { SELECT ?o WHERE { ?x <urn:p> ?o } } }`, 2, false},
	} {
		t.Run(name, func(t *testing.T) {
			pq, err := Parse(c.q)
			if err != nil {
				t.Fatal(err)
			}
			pq.PreBindSubqueries = c.sub
			res, err := EvalQuery(g, pq, map[string]term.Term{"x": a})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Bindings) != c.rows {
				t.Errorf("%d rows, want %d: %v", len(res.Bindings), c.rows, res.Bindings)
			}
		})
	}
}
