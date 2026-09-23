package sparql_test

import (
	"context"
	"sync"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

type traceKey struct{}

// tracingStore stands in for an instrumented backend (issue #35): it records
// the trace value of the context every read and write was made with. The
// unbound store records "" for each call.
type tracingStore struct {
	store.Store
	ctx context.Context
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

func (s *tracingStore) BindContext(ctx context.Context) store.Store {
	c := *s
	c.ctx = ctx
	return &c
}

func (s *tracingStore) record() {
	v := ""
	if s.ctx != nil {
		v, _ = s.ctx.Value(traceKey{}).(string)
	}
	s.log.add(v)
}

func (s *tracingStore) Add(t term.Triple, g term.Term) { s.record(); s.Store.Add(t, g) }
func (s *tracingStore) Remove(p term.TriplePattern, g term.Term) {
	s.record()
	s.Store.Remove(p, g)
}
func (s *tracingStore) Triples(p term.TriplePattern, g term.Term) store.TripleIterator {
	s.record()
	return s.Store.Triples(p, g)
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
	g.Add(ex("a"), ex("p"), ex("b"))
	g.Add(ex("b"), ex("p"), ex("c"))
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
	for range paths.EvalContext(tracedCtx(), paths.OneOrMore(paths.URIRefPath{URI: ex("p")}), g, ex("a"), nil) {
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
		t.Fatal("the update left a context-bound view in the caller's dataset")
	}
	if got := def.Len(); got != 1 {
		t.Fatalf("default graph has %d triples, want 1", got)
	}
}

func TestContextFunctionReceivesQueryContext(t *testing.T) {
	g := graph.NewGraph()
	g.Add(term.NewURIRefUnsafe("http://example.org/a"), term.NewURIRefUnsafe("http://example.org/p"), term.NewLiteral("x"))

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
// that merge is a read like any other. Done before the query binds its graphs,
// it would reach the store with a context the caller never supplied.
func TestDatasetClauseContextReachesStore(t *testing.T) {
	st := newTracingStore()
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	named := graph.NewGraph(graph.WithStore(st), graph.WithIdentifier(ex("g")))
	named.Add(ex("a"), ex("p"), ex("b"))
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
