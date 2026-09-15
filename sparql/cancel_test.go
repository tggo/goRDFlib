package sparql_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// Every slow query here would run for seconds without cancellation (checked
// by hand when the tests were written). They are cancelled after
// cancelAfter; stopLatencyLimit is deliberately loose next to the ~1 ms
// the checks aim for, so the tests stay stable under -race and on a loaded
// machine while still failing outright when a check is missing.
const (
	cancelAfter      = 20 * time.Millisecond
	stopLatencyLimit = 250 * time.Millisecond
)

const cancelPrefix = "PREFIX ex: <http://example.org/>\n"

// cancelGraph has n subjects with one ex:p literal and one ex:q literal each.
// It has no ex:r or ex:s triples, so patterns on those match nothing.
func cancelGraph(n int) *rdflibgo.Graph {
	g := rdflibgo.NewGraph()
	p := term.NewURIRefUnsafe("http://example.org/p")
	q := term.NewURIRefUnsafe("http://example.org/q")
	for i := range n {
		s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		g.Add(s, p, term.NewLiteral(fmt.Sprintf("v%d", i)))
		g.Add(s, q, term.NewLiteral(fmt.Sprintf("w%d", i)))
	}
	return g
}

// chainGraph links n nodes with ex:next.
func chainGraph(n int) *rdflibgo.Graph {
	g := rdflibgo.NewGraph()
	next := term.NewURIRefUnsafe("http://example.org/next")
	for i := range n - 1 {
		g.Add(term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", i)), next,
			term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", i+1)))
	}
	return g
}

// assertCancelled checks the error and the result of a stopped evaluation.
func assertCancelled(t *testing.T, res *sparql.Result, err error, want error) {
	t.Helper()
	if err == nil {
		t.Fatalf("got a result (%d rows) and no error; the query was not stopped", len(resRows(res)))
	}
	if !errors.Is(err, want) {
		t.Fatalf("errors.Is(err, %v) = false; err = %v", want, err)
	}
	if !errors.Is(err, sparql.ErrQueryCancelled) {
		t.Fatalf("errors.Is(err, ErrQueryCancelled) = false; err = %v", err)
	}
	if res != nil {
		t.Fatalf("a stopped query returned a partial result: %+v", res)
	}
}

func resRows(res *sparql.Result) []map[string]rdflibgo.Term {
	if res == nil {
		return nil
	}
	return res.Bindings
}

// runCancelled starts fn with a context cancelled after cancelAfter and
// returns how long fn kept running after the cancellation.
func runCancelled(t *testing.T, fn func(ctx context.Context)) time.Duration {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var cancelledAt time.Time
	timer := time.AfterFunc(cancelAfter, func() {
		cancelledAt = time.Now()
		cancel()
	})
	defer timer.Stop()
	fn(ctx)
	<-ctx.Done() // fn must not have returned before the cancellation
	latency := time.Since(cancelledAt)
	t.Logf("stopped %v after cancel", latency)
	if latency > stopLatencyLimit {
		t.Fatalf("query kept running %v after cancel, want < %v", latency, stopLatencyLimit)
	}
	return latency
}

func TestQueryContext_CancelStopsEvaluation(t *testing.T) {
	g := cancelGraph(10_000)
	tests := []struct{ name, query string }{
		// The triple term with variables keeps the planner from reordering,
		// so the first two patterns run as a 10^8 nested loop whose inner
		// lookups all come back empty: the loop produces no rows, only work.
		{"BGP nested loop", `SELECT ?a ?b WHERE {
			?a ex:p ?x . ?b ex:p ?y . ?z ex:s <<( ?a ex:r ?b )>> }`},
		// No shared variable: every left row is compared with every right row.
		{"MINUS", `SELECT ?a WHERE { ?a ex:p ?x MINUS { ?b ex:q ?y } }`},
		// The inner group is not a plain BGP, so it is evaluated once per row.
		{"FILTER NOT EXISTS", `SELECT ?a WHERE {
			?a ex:p ?x FILTER NOT EXISTS { ?b ex:q ?y FILTER(?y = "none") } }`},
		{"OPTIONAL", `SELECT ?a WHERE {
			?a ex:p ?x OPTIONAL { ?b ex:q ?y FILTER(?y = "none") } }`},
		{"sub-SELECT", `SELECT * WHERE { { SELECT ?a WHERE { ?a ex:p ?x MINUS { ?b ex:q ?y } } } }`},
		{"GROUP BY over MINUS", `SELECT ?x (COUNT(*) AS ?n) WHERE { ?a ex:p ?x MINUS { ?b ex:q ?y } } GROUP BY ?x`},
		{"UNION", `SELECT ?a WHERE { { ?a ex:p ?x MINUS { ?b ex:q ?y } } UNION { ?a ex:q ?x } }`},
		{"ASK", `ASK { ?a ex:p ?x MINUS { ?b ex:q ?y } }`},
		{"CONSTRUCT", `CONSTRUCT { ?a ex:t ?x } WHERE { ?a ex:p ?x MINUS { ?b ex:q ?y } }`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCancelled(t, func(ctx context.Context) {
				res, err := sparql.QueryContext(ctx, g, cancelPrefix+tt.query)
				assertCancelled(t, res, err, context.Canceled)
			})
		})
	}
}

func TestQueryContext_DeadlineExceeded(t *testing.T) {
	g := cancelGraph(10_000)
	ctx, cancel := context.WithTimeout(context.Background(), cancelAfter)
	defer cancel()
	start := time.Now()
	res, err := sparql.QueryContext(ctx, g, cancelPrefix+`SELECT ?a WHERE { ?a ex:p ?x MINUS { ?b ex:q ?y } }`)
	assertCancelled(t, res, err, context.DeadlineExceeded)
	if errors.Is(err, context.Canceled) {
		t.Fatalf("a deadline must not report context.Canceled: %v", err)
	}
	if elapsed := time.Since(start); elapsed > cancelAfter+stopLatencyLimit {
		t.Fatalf("query ran %v with a %v deadline", elapsed, cancelAfter)
	}
}

func TestQueryContext_PropertyPathCancel(t *testing.T) {
	// ?s ex:next* ?o over a chain has n²/2 solutions: 12.5 million here.
	g := chainGraph(5_000)
	runCancelled(t, func(ctx context.Context) {
		res, err := sparql.QueryContext(ctx, g, cancelPrefix+`SELECT ?s ?o WHERE { ?s ex:next* ?o }`)
		assertCancelled(t, res, err, context.Canceled)
	})
}

func TestQueryContext_AlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := sparql.QueryContext(ctx, cancelGraph(10), cancelPrefix+`SELECT * WHERE { ?s ?p ?o }`)
	assertCancelled(t, res, err, context.Canceled)
}

func TestQueryContext_SameResultAsQuery(t *testing.T) {
	g := cancelGraph(3_000)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, q := range []string{
		`SELECT ?a ?x WHERE { ?a ex:p ?x OPTIONAL { ?a ex:q ?y } FILTER(STRSTARTS(?x, "v1")) } ORDER BY DESC(?x)`,
		`SELECT (COUNT(DISTINCT ?a) AS ?n) WHERE { ?a ex:p ?x MINUS { ?a ex:q "w7" } }`,
		`SELECT ?a WHERE { ?a ex:p ?x FILTER EXISTS { ?a ex:q ?y } } ORDER BY ?a LIMIT 5`,
	} {
		old, err := sparql.Query(g, cancelPrefix+q)
		if err != nil {
			t.Fatal(err)
		}
		got, err := sparql.QueryContext(ctx, g, cancelPrefix+q)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(old.Bindings) != fmt.Sprint(got.Bindings) {
			t.Errorf("%s: QueryContext and Query disagree:\n%v\n%v", q, got.Bindings, old.Bindings)
		}
		if len(got.Bindings) == 0 {
			t.Errorf("%s: no rows; the comparison proves nothing", q)
		}
	}
}

// snapshot returns the dataset's default graph as a set of N-Triples lines.
func snapshot(g *rdflibgo.Graph) map[string]bool {
	out := make(map[string]bool, g.Len())
	for tr := range g.Triples(nil, nil, nil) {
		out[tr.Subject.N3()+" "+tr.Predicate.N3()+" "+tr.Object.N3()] = true
	}
	return out
}

func TestUpdateContext_CancelledModifyLeavesDatasetUnchanged(t *testing.T) {
	tests := []struct{ name, update string }{
		// A cross product: rows arrive quickly, so the WHERE clause is
		// stopped with part of its solutions already computed. Applying
		// those would delete some ex:p triples.
		{"DELETE/INSERT WHERE", `DELETE { ?a ex:p ?x } INSERT { ?a ex:t ?y } WHERE { ?a ex:p ?x . ?b ex:q ?y }`},
		{"DELETE WHERE", `DELETE WHERE { ?a ex:p ?x . ?b ex:q ?y }`},
		{"MINUS in WHERE", `DELETE { ?a ex:p ?x } WHERE { ?a ex:p ?x MINUS { ?b ex:q ?y } }`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := cancelGraph(10_000)
			before := snapshot(g)
			ds := &sparql.Dataset{Default: g}
			runCancelled(t, func(ctx context.Context) {
				err := sparql.UpdateContext(ctx, ds, cancelPrefix+tt.update)
				if !errors.Is(err, context.Canceled) || !errors.Is(err, sparql.ErrQueryCancelled) {
					t.Fatalf("err = %v, want context.Canceled and ErrQueryCancelled", err)
				}
			})
			after := snapshot(g)
			if len(after) != len(before) {
				t.Fatalf("cancelled update changed the dataset: %d triples before, %d after", len(before), len(after))
			}
			for k := range before {
				if !after[k] {
					t.Fatalf("cancelled update removed %s", k)
				}
			}
		})
	}
}

func TestUpdateContext_EarlierOperationsStayApplied(t *testing.T) {
	g := cancelGraph(10_000)
	ds := &sparql.Dataset{Default: g}
	marker := `<http://example.org/marker> <http://example.org/is> "set"`
	runCancelled(t, func(ctx context.Context) {
		err := sparql.UpdateContext(ctx, ds, cancelPrefix+`INSERT DATA { `+marker+` } ;
			DELETE { ?a ex:p ?x } WHERE { ?a ex:p ?x MINUS { ?b ex:q ?y } } ;
			INSERT DATA { <http://example.org/marker> <http://example.org/is> "late" }`)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
	snap := snapshot(g)
	if !snap[marker] {
		t.Error("the operation before the cancelled one was not applied")
	}
	if snap[`<http://example.org/marker> <http://example.org/is> "late"`] {
		t.Error("an operation after the cancelled one was applied")
	}
	if len(snap) != 20_001 {
		t.Errorf("dataset has %d triples, want 20001", len(snap))
	}
}

func TestUpdate_OldAPIStillWorks(t *testing.T) {
	g := cancelGraph(3)
	ds := &sparql.Dataset{Default: g}
	if err := sparql.Update(ds, cancelPrefix+`DELETE { ?a ex:p ?x } INSERT { ?a ex:t ?x } WHERE { ?a ex:p ?x }`); err != nil {
		t.Fatal(err)
	}
	res, err := sparql.Query(g, cancelPrefix+`SELECT ?a WHERE { ?a ex:t ?x }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 3 {
		t.Fatalf("got %d rows, want 3", len(res.Bindings))
	}
	pq, err := sparql.Parse(cancelPrefix + `ASK { ?a ex:p ?x }`)
	if err != nil {
		t.Fatal(err)
	}
	if res, err := sparql.EvalQuery(g, pq, nil); err != nil || res.AskResult {
		t.Fatalf("EvalQuery: ask = %v, err = %v; want false, nil", res != nil && res.AskResult, err)
	}
	pu, err := sparql.ParseUpdate(cancelPrefix + `INSERT DATA { ex:x ex:p "1" }`)
	if err != nil {
		t.Fatal(err)
	}
	if err := sparql.EvalUpdate(ds, pu); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 7 {
		t.Fatalf("graph has %d triples after EvalUpdate, want 7", g.Len())
	}
}
