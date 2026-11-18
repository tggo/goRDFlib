package sparql

import (
	"fmt"
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// The right side of an OPTIONAL that isn't a BGP must not be matched in full
// once per left row either (the same shape as TestJoinWork).
func TestOptionalWork(t *testing.T) {
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

	root := map[string]term.Term{"r": ex("root")}
	for _, c := range []struct {
		name, query string
		pre         map[string]term.Term
		rows        int
	}{
		{"union", `SELECT ?f ?server WHERE {
			ex:root ex:part ?f .
			OPTIONAL { { ?f ex:fromSubgraph ?s . ?s ex:partOf ?server } UNION { ?f ex:inSchema ?s . ?s ex:partOf ?server } }
		}`, nil, 30},
		// the same over initial bindings (SHACL, the protocol's initBindings)
		{"union, pre-bound", `SELECT ?f ?server WHERE {
			?r ex:part ?f .
			OPTIONAL { { ?f ex:fromSubgraph ?s . ?s ex:partOf ?server } UNION { ?f ex:inSchema ?s . ?s ex:partOf ?server } }
		}`, root, 30},
		{"group with a filter", `SELECT ?f ?server WHERE {
			ex:root ex:part ?f .
			OPTIONAL { { ?f ex:inSchema ?s . ?s ex:partOf ?server FILTER NOT EXISTS { ?f ex:fromSubgraph ?any } } }
		}`, nil, 20},
	} {
		t.Run(c.name, func(t *testing.T) {
			st.pulled = 0
			if rows := preboundRows(t, g, c.query, c.pre); len(rows) != c.rows {
				t.Fatalf("%d rows, want %d", len(rows), c.rows)
			}
			if st.pulled > 12_000 {
				t.Errorf("pulled %d triples: the OPTIONAL's right side was matched in full per left row", st.pulled)
			}
		})
	}
}

// A right side evaluated once is shared by every left row, and the
// LeftJoin's FILTER must not filter those shared rows in place: each left
// row has to see all of them again. With != every row keeps two of three,
// which is what an in-place filter garbles.
func TestOptionalSharedRowsWithCondition(t *testing.T) {
	g := graph.NewGraph()
	ctx := t.Context()
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	for _, l := range []string{"a", "b", "c", "d"} {
		g.Add(ctx, ex("root"), ex("left"), ex(l))
		g.Add(ctx, ex("x"+l), ex("q"), ex(l))
	}
	q := `SELECT ?l ?r WHERE {
	ex:root ex:left ?l
	OPTIONAL { { ?r ex:q ?v } UNION { ?r ex:none ?v } FILTER (?v != ?l) }
}`
	offJoinRightSide = true
	want := preboundRows(t, g, q, nil)
	offJoinRightSide = false
	got := preboundRows(t, g, q, nil)
	if len(want) != 12 {
		t.Fatalf("plain evaluation gave %d rows, want 12", len(want))
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}
