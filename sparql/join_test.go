package sparql

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// A join's right side that isn't a BGP is a group whose solutions don't
// depend on the left row, so it must not be matched in full once per left
// row. 20 parts each select one of 3,000 fields; a field's server comes from
// its subgraph, or from its schema.
func TestJoinWork(t *testing.T) {
	st := &probingStore{Store: store.NewMemoryStore()}
	g := graph.NewGraph(graph.WithStore(st))
	ctx := t.Context()
	ex := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	for i := range 3000 {
		f := ex("f%d", i)
		if i < 20 {
			g.Add(ctx, ex("root"), ex("part"), f)
		}
		g.Add(ctx, f, ex("inSchema"), ex("schema"))
		if i%2 == 0 {
			g.Add(ctx, f, ex("fromSubgraph"), ex("subgraph%d", i%10))
		}
	}
	for k := range 10 {
		g.Add(ctx, ex("subgraph%d", k), ex("partOf"), ex("system%d", k))
	}
	g.Add(ctx, ex("schema"), ex("partOf"), ex("gateway"))

	for _, c := range []struct {
		name, query string
		rows        int
	}{
		// main matches the whole UNION (4,500 triples) per left row
		{"substituted", `SELECT ?f ?server WHERE {
			ex:root ex:part ?f .
			{ ?f ex:fromSubgraph ?s . ?s ex:partOf ?server } UNION { ?f ex:inSchema ?s . ?s ex:partOf ?server }
		}`, 30},
		// main evaluates the group with the FILTER per left row
		{"evaluated once", `SELECT ?f ?server WHERE {
			ex:root ex:part ?f .
			{ ?f ex:inSchema ?s . ?s ex:partOf ?server FILTER NOT EXISTS { ?f ex:fromSubgraph ?any } }
		}`, 10},
	} {
		t.Run(c.name, func(t *testing.T) {
			st.pulled = 0
			if rows := preboundRows(t, g, c.query, nil); len(rows) != c.rows {
				t.Fatalf("%d rows, want %d", len(rows), c.rows)
			}
			// 20 parts, the planner's probes, and at most one pass over the
			// right side's triples
			if st.pulled > 12_000 {
				t.Errorf("pulled %d triples: the right side was matched in full per left row", st.pulled)
			}
		})
	}
}

// A right side that calls a function giving each call its own value is still
// evaluated per left row, so every row gets its own value.
func TestJoinVolatileRightSide(t *testing.T) {
	g := preboundGraph(t)
	for _, q := range []string{
		`SELECT ?p ?u WHERE { ex:root ex:part ?p { BIND (STRUUID() AS ?u) } }`,
		`SELECT ?p ?u WHERE { ex:root ex:part ?p { SELECT (UUID() AS ?u) WHERE {} } }`,
		`SELECT ?p ?u WHERE { ex:root ex:part ?p { ?x ex:label ?l FILTER (?l = "kind 0") BIND (rand() AS ?u) } } LIMIT 100`,
	} {
		pq, err := Parse("PREFIX ex: <http://example.org/>\n" + q)
		if err != nil {
			t.Fatal(err)
		}
		res, err := EvalQuery(g, pq, nil)
		if err != nil {
			t.Fatal(err)
		}
		values := map[string]bool{}
		for _, b := range res.Bindings {
			values[b["u"].N3()] = true
		}
		if len(res.Bindings) != 12 || len(values) != 12 {
			t.Errorf("%d rows, %d distinct values, want 12 and 12\n%s", len(res.Bindings), len(values), q)
		}
	}
}
