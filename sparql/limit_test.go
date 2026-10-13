package sparql

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// crossGraph is the graph of issue #51: n items with four triples each.
func crossGraph(n int) *graph.Graph {
	g := graph.NewGraph()
	ex := func(s string) rdflibgo.URIRef { return rdflibgo.NewURIRefUnsafe("http://example.org/" + s) }
	for i := 0; i < n; i++ {
		s := ex("item" + strconv.Itoa(i))
		g.Add(s, ex("type"), ex("Item"))
		g.Add(s, ex("title"), rdflibgo.NewLiteral("title "+strconv.Itoa(i)))
		g.Add(s, ex("n"), rdflibgo.NewLiteral(i))
		g.Add(s, ex("link"), ex("item"+strconv.Itoa((i+1)%n)))
	}
	return g
}

// Issue #51: LIMIT built the whole cross product before cutting it. 400
// triples cubed is 6.4e7 solutions; with the limit pushed down a few rows are
// built. The allocation bound is what fails without the pushdown (it took
// gigabytes), not the timing.
func TestLimitStopsCrossProduct(t *testing.T) {
	g := crossGraph(100)
	const cross = `{ ?a ?b ?c . ?d ?e ?f . ?g ?h ?i }`
	queries := map[string]int{
		`SELECT * WHERE ` + cross + ` LIMIT 10`:                                 10,
		`SELECT * WHERE ` + cross + ` LIMIT 10 OFFSET 5`:                        10,
		`SELECT * WHERE { { ?a ?b ?c } { ?d ?e ?f } { ?g ?h ?i } } LIMIT 3`:     3,
		`SELECT * WHERE { ?a ?b ?c OPTIONAL { ?d ?e ?f . ?g ?h ?i } } LIMIT 7`:  7,
		`SELECT * WHERE { { ?a ?b ?c . ?d ?e ?f } UNION { ?g ?h ?i } } LIMIT 4`: 4,
		`SELECT (STR(?c) AS ?s) WHERE ` + cross + ` LIMIT 2`:                    2,
		`ASK ` + cross: 1,
		`CONSTRUCT { ?a ?b ?c } WHERE ` + cross + ` LIMIT 1`:                       1,
		`SELECT * WHERE { ?a <http://example.org/type> ?t . ?a ?p ?o } LIMIT 1000`: 400,
	}
	for q, want := range queries {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		r, err := Query(g, q)
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		got := len(r.Bindings)
		switch r.Type {
		case "ASK":
			got = 0
			if r.AskResult {
				got = 1
			}
		case "CONSTRUCT":
			got = r.Graph.Len()
		}
		if got != want {
			t.Errorf("%s: %d results, want %d", q, got, want)
		}
		if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 64<<20 {
			t.Errorf("%s: allocated %d MiB", q, alloc>>20)
		}
	}
}

// What the pushdown returns must still be solutions of the pattern.
func TestLimitPushdownRowsAreSolutions(t *testing.T) {
	g := crossGraph(5)
	full, err := Query(g, `SELECT ?a ?d WHERE { ?a <http://example.org/link> ?d . ?d ?p ?o OPTIONAL { ?o ?q ?a } }`)
	if err != nil {
		t.Fatal(err)
	}
	all := map[string]bool{}
	for _, b := range full.Bindings {
		all[b["a"].N3()+" "+b["d"].N3()] = true
	}
	part, err := Query(g, `SELECT ?a ?d WHERE { ?a <http://example.org/link> ?d . ?d ?p ?o OPTIONAL { ?o ?q ?a } } LIMIT 3`)
	if err != nil {
		t.Fatal(err)
	}
	if len(part.Bindings) != 3 {
		t.Fatalf("%d rows", len(part.Bindings))
	}
	for _, b := range part.Bindings {
		if !all[b["a"].N3()+" "+b["d"].N3()] {
			t.Errorf("row %v is not a solution", b)
		}
	}
}

func TestMaxSolutions(t *testing.T) {
	g := crossGraph(40)
	ctx := WithMaxSolutions(context.Background(), 10_000)

	_, err := QueryContext(ctx, g, `SELECT * WHERE { ?a ?b ?c . ?d ?e ?f . ?g ?h ?i }`)
	if !errors.Is(err, ErrTooManySolutions) {
		t.Fatalf("cross product: err = %v, want ErrTooManySolutions", err)
	}
	if errors.Is(err, ErrQueryCancelled) {
		t.Errorf("budget error reported as a cancellation: %v", err)
	}
	// Built in joins, not in one BGP.
	if _, err := QueryContext(ctx, g, `SELECT * WHERE { { ?a ?b ?c } { ?d ?e ?f } }`); !errors.Is(err, ErrTooManySolutions) {
		t.Errorf("join: err = %v", err)
	}
	// Within the budget nothing changes.
	r, err := QueryContext(ctx, g, `SELECT * WHERE { ?a ?b ?c . ?d ?e ?f } LIMIT 5000`)
	if err != nil || len(r.Bindings) != 5000 {
		t.Errorf("within budget: %v, %v", err, r)
	}
	// Updates honour it too.
	ds := &Dataset{Default: crossGraph(40)}
	if err := UpdateContext(ctx, ds, `INSERT { ?a <http://x> ?f } WHERE { ?a ?b ?c . ?d ?e ?f }`); !errors.Is(err, ErrTooManySolutions) {
		t.Errorf("update: err = %v", err)
	}
	if n := ds.Default.Len(); n != 160 {
		t.Errorf("update over budget wrote: %d triples", n)
	}
	// n <= 0 removes the bound.
	if _, err := QueryContext(WithMaxSolutions(ctx, 0), g, `SELECT * WHERE { ?a ?b ?c . ?d ?e ?f }`); err != nil {
		t.Errorf("no budget: %v", err)
	}
}
