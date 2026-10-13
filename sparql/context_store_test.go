package sparql_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
	"github.com/tggo/goRDFlib/term"
)

type traceKey struct{}

// tracingStore stands in for an instrumented backend (issue #35): it records
// the trace value of the context every read and write was made with.
type tracingStore struct {
	store.Store
	log *callLog
}

type callLog struct {
	mu    sync.Mutex
	calls []string
}

func (l *callLog) add(v string) {
	l.mu.Lock()
	l.calls = append(l.calls, v)
	l.mu.Unlock()
}

func (l *callLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.calls...)
}

func newTracingStore() *tracingStore {
	return &tracingStore{Store: store.NewMemoryStore(), log: &callLog{}}
}

func (s *tracingStore) record(ctx context.Context) {
	v, _ := ctx.Value(traceKey{}).(string)
	s.log.add(v)
}

func (s *tracingStore) Add(ctx context.Context, t term.Triple, g term.Term) error {
	s.record(ctx)
	return s.Store.Add(ctx, t, g)
}
func (s *tracingStore) Remove(ctx context.Context, p term.TriplePattern, g term.Term) error {
	s.record(ctx)
	return s.Store.Remove(ctx, p, g)
}
func (s *tracingStore) Triples(ctx context.Context, p term.TriplePattern, g term.Term) store.TripleIterator {
	s.record(ctx)
	return s.Store.Triples(ctx, p, g)
}

func tracedCtx() context.Context {
	return context.WithValue(context.Background(), traceKey{}, "span-1")
}

func assertAllTraced(t *testing.T, st *tracingStore) {
	t.Helper()
	calls := st.log.all()
	if len(calls) == 0 {
		t.Fatal("the store was never called")
	}
	for i, v := range calls {
		if v != "span-1" {
			t.Fatalf("store call %d of %d ran without the caller's context", i+1, len(calls))
		}
	}
}

func TestQueryContextReachesStore(t *testing.T) {
	st := newTracingStore()
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	g := graph.NewGraph(graph.WithStore(st))
	g.Add(t.Context(), ex("a"), ex("p"), ex("b"))
	g.Add(t.Context(), ex("b"), ex("p"), ex("c"))
	st.log = &callLog{}

	res, err := sparql.QueryContext(tracedCtx(), g,
		`PREFIX ex: <http://example.org/> SELECT ?x ?y WHERE { ?x ex:p ?y . ?y ex:p ?z FILTER NOT EXISTS { ?z ex:p ?x } } `)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("bindings = %v", res.Bindings)
	}
	assertAllTraced(t, st)

	st.log = &callLog{}
	for range paths.OneOrMore(paths.URIRefPath{URI: ex("p")}).Eval(tracedCtx(), g, ex("a"), nil) {
	}
	assertAllTraced(t, st)
}

func TestUpdateContextReachesStore(t *testing.T) {
	st := newTracingStore()
	def := graph.NewGraph(graph.WithStore(st))
	named := graph.NewGraph(graph.WithStore(st), graph.WithIdentifier(term.NewURIRefUnsafe("http://example.org/g")))
	ds := &sparql.Dataset{Default: def, NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g": named}}

	err := sparql.UpdateContext(tracedCtx(), ds, `
PREFIX ex: <http://example.org/>
INSERT DATA { ex:a ex:p ex:b . GRAPH ex:g { ex:a ex:p ex:c } } ;
DELETE { GRAPH ex:g { ?s ?p ?o } } INSERT { ?s ?p ?o } WHERE { GRAPH ex:g { ?s ?p ?o } } ;
DELETE WHERE { ?s ex:p ex:b } ;
CLEAR GRAPH ex:g`)
	if err != nil {
		t.Fatal(err)
	}
	assertAllTraced(t, st)
	if ds.NamedGraphs["http://example.org/g"] != named || ds.Default != def {
		t.Fatal("the update replaced a graph of the caller's dataset")
	}
	if got, _ := def.Len(t.Context()); got != 1 {
		t.Fatalf("default graph has %d triples, want 1", got)
	}
}

func TestContextFunctionReceivesQueryContext(t *testing.T) {
	g := graph.NewGraph()
	g.Add(t.Context(), term.NewURIRefUnsafe("http://example.org/a"), term.NewURIRefUnsafe("http://example.org/p"), term.NewLiteral("x"))

	var seen []string
	fn := func(ctx context.Context, args []rdflibgo.Term) (rdflibgo.Term, error) {
		v, _ := ctx.Value(traceKey{}).(string)
		seen = append(seen, v)
		return args[0], nil
	}
	const q = `SELECT ?v WHERE { ?s ?p ?o BIND(<http://example.org/fn/traced>(?o) AS ?v) }`

	if err := sparql.RegisterContextFunction("http://example.org/fn/traced", fn); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sparql.UnregisterFunction("http://example.org/fn/traced") })
	res, err := sparql.QueryContext(tracedCtx(), g, q)
	if err != nil || len(res.Bindings) != 1 || res.Bindings[0]["v"] == nil {
		t.Fatalf("registered: %v, %v", res, err)
	}

	pq, err := sparql.Parse(q)
	if err != nil {
		t.Fatal(err)
	}
	if n := pq.BindContextFunctions(map[string]sparql.ContextFunction{"http://example.org/fn/traced": fn}); n != 1 {
		t.Fatalf("bound %d call sites", n)
	}
	sparql.UnregisterFunction("http://example.org/fn/traced")
	if _, err := sparql.EvalQueryContext(tracedCtx(), g, pq, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := sparql.EvalQuery(g, pq, nil); err != nil {
		t.Fatal(err)
	}
	if want := []string{"span-1", "span-1", ""}; len(seen) != 3 || seen[0] != want[0] || seen[1] != want[1] || seen[2] != want[2] {
		t.Fatalf("contexts seen = %q, want %q", seen, want)
	}

	// A plain Function looked up from a context registration still works.
	sparql.MustRegisterContextFunction("http://example.org/fn/traced", fn)
	plain, ok := sparql.LookupFunction("http://example.org/fn/traced")
	if !ok {
		t.Fatal("LookupFunction did not find a context function")
	}
	if _, err := plain([]rdflibgo.Term{term.NewLiteral("y")}); err != nil || seen[3] != "" {
		t.Fatalf("plain call: %v, %q", err, seen)
	}
}

// TestDatasetClauseContextReachesStore covers the one read that is easy to
// forget: a FROM clause merges the graphs it names into the default graph, and
// that merge is a read like any other.
func TestDatasetClauseContextReachesStore(t *testing.T) {
	st := newTracingStore()
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	named := graph.NewGraph(graph.WithStore(st), graph.WithIdentifier(ex("g")))
	named.Add(t.Context(), ex("a"), ex("p"), ex("b"))
	st.log = &callLog{}

	q, err := sparql.Parse(`SELECT ?x WHERE { ?x <http://example.org/p> ?y }`)
	if err != nil {
		t.Fatal(err)
	}
	q.DatasetClause = []sparql.DatasetClause{{IRI: "http://example.org/g"}}
	q.NamedGraphs = map[string]*rdflibgo.Graph{"http://example.org/g": named}

	res, err := sparql.EvalQueryContext(tracedCtx(), graph.NewGraph(), q, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("bindings = %v, want the one triple of the FROM graph", res.Bindings)
	}
	assertAllTraced(t, st)
}

// Issue #52: a store that fails used to look like a store with no matches.
// Every way a query or update reaches a store must return its error, and no
// result.
func TestStoreErrorIsTheQueryError(t *testing.T) {
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	g := graph.NewGraph(graph.WithStore(fs))
	for _, n := range []string{"a", "b", "c"} {
		if err := g.Add(t.Context(), ex(n), ex("p"), ex("o")); err != nil {
			t.Fatal(err)
		}
	}
	queries := []string{
		`SELECT * WHERE { ?s ?p ?o }`,
		`SELECT * WHERE { ?s <http://example.org/p> ?o . ?o ?q ?r }`,
		`SELECT * WHERE { ?s <http://example.org/p>+ ?o }`,
		`SELECT * WHERE { ?s ?p ?o } LIMIT 1`,
		`SELECT (COUNT(*) AS ?n) WHERE { ?s ?p ?o }`,
		`ASK { ?s ?p ?o }`,
		`CONSTRUCT WHERE { ?s ?p ?o }`,
		`SELECT * WHERE { ?s ?p ?o FILTER NOT EXISTS { ?o ?q ?s } }`,
	}
	for _, mode := range []string{"fail", "midway"} {
		for _, q := range queries {
			if mode == "fail" {
				fs.Fail(nil)
			} else if strings.Contains(q, "LIMIT 1") || strings.HasPrefix(q, "ASK") {
				// Satisfied by the first triple: the read never reaches the
				// failure, and the answer is complete.
				continue
			} else {
				fs.FailReadsAfter(1, nil)
			}
			res, err := sparql.QueryContext(t.Context(), g, q)
			if !errors.Is(err, storetest.ErrInjected) || res != nil {
				t.Errorf("%s %s: result %v, err %v; want the store error and no result", mode, q, res, err)
			}
			if errors.Is(err, sparql.ErrQueryCancelled) {
				t.Errorf("%s %s: a store error reported as a cancellation", mode, q)
			}
			fs.Fail(nil)
			fs = resetFailing(fs)
			g = graph.NewGraph(graph.WithStore(fs))
		}
	}

	// An update whose WHERE clause cannot be read changes nothing.
	mem := store.NewMemoryStore()
	fs = storetest.NewFailingStore(mem)
	def := graph.NewGraph(graph.WithStore(fs))
	if err := def.Add(t.Context(), ex("a"), ex("p"), ex("o")); err != nil {
		t.Fatal(err)
	}
	fs.FailReadsAfter(0, nil)
	err := sparql.UpdateContext(t.Context(), &sparql.Dataset{Default: def},
		`INSERT { ?s <http://example.org/q> ?o } WHERE { ?s ?p ?o }`)
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("update: err = %v, want the store error", err)
	}
	if n, _ := mem.Len(t.Context(), nil); n != 1 {
		t.Fatalf("a failed WHERE clause still wrote: %d triples", n)
	}
	// An update whose write fails reports it.
	fs.Fail(nil)
	if err := sparql.UpdateContext(t.Context(), &sparql.Dataset{Default: def}, `INSERT DATA { <http://x> <http://y> <http://z> }`); !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("INSERT DATA: err = %v, want the store error", err)
	}
}

// resetFailing returns a fresh FailingStore over a copy of fs's three triples.
func resetFailing(_ *storetest.FailingStore) *storetest.FailingStore {
	mem := store.NewMemoryStore()
	for _, n := range []string{"a", "b", "c"} {
		_ = mem.Add(context.Background(), term.Triple{
			Subject:   term.NewURIRefUnsafe("http://example.org/" + n),
			Predicate: term.NewURIRefUnsafe("http://example.org/p"),
			Object:    term.NewURIRefUnsafe("http://example.org/o"),
		}, nil)
	}
	return storetest.NewFailingStore(mem)
}
