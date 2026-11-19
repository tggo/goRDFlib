package sparql

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// WithMaxPathProbes sets how many path probes one evaluation may run: the
// OPTIONAL below is planned once per row of 1,500 rows, with two probeable
// paths per row, so every bound below 3,000 is reached exactly, and without
// a bound the probes go past the default. The answer never changes.
func TestWithMaxPathProbes(t *testing.T) {
	g := pathProbeGraph(t)
	q := `SELECT ?y ?c WHERE { <urn:root> <urn:part> ?y OPTIONAL { ?y <urn:type>/<urn:sub>* ?c . ?c <urn:sub>* <urn:Class1> } }`
	probes := 0
	pathProbeHook = func() { probes++ }
	defer func() { pathProbeHook = nil }()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want func(int) bool
		desc string
	}{
		{"default", t.Context(), func(n int) bool { return n == bgpMaxPathProbes }, "the default bound"},
		{"raised", WithMaxPathProbes(t.Context(), 200), func(n int) bool { return n == 200 }, "200"},
		{"lowered", WithMaxPathProbes(t.Context(), 1), func(n int) bool { return n == 1 }, "1"},
		{"unbounded", WithMaxPathProbes(t.Context(), 0), func(n int) bool { return n > 200 }, "more than 200"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probes = 0
			res, err := QueryContext(tc.ctx, g, q)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.want(probes) {
				t.Errorf("%d path probes, want %s", probes, tc.desc)
			}
			if len(res.Bindings) != 1500 {
				t.Errorf("%d rows, want 1500", len(res.Bindings))
			}
		})
	}
}

func TestMaxPathProbesFrom(t *testing.T) {
	for _, tc := range []struct {
		ctx  context.Context
		want int
	}{
		{context.Background(), bgpMaxPathProbes},
		{WithMaxPathProbes(context.Background(), 5000), 5000},
		{WithMaxPathProbes(context.Background(), 0), math.MaxInt},
		{WithMaxPathProbes(context.Background(), -1), math.MaxInt},
	} {
		if got := newEvalCtx(tc.ctx).maxPathProbes; got != tc.want {
			t.Errorf("maxPathProbes = %d, want %d", got, tc.want)
		}
	}
}

// An evalCtx not built by newEvalCtx has maxPathProbes 0. That must mean the
// default bound, not "no probe at all", and a nil evalCtx stays unbounded.
func TestTakePathProbeZeroValueKeepsDefault(t *testing.T) {
	ec := &evalCtx{}
	n := 0
	for range 3 * bgpMaxPathProbes {
		if ec.takePathProbe() {
			n++
		}
	}
	if n != bgpMaxPathProbes {
		t.Errorf("a zero evalCtx allowed %d probes, want %d", n, bgpMaxPathProbes)
	}
	var none *evalCtx
	for range 3 * bgpMaxPathProbes {
		if !none.takePathProbe() {
			t.Fatal("a nil evalCtx refused a probe")
		}
	}
}

// budgetScanStore is a MemoryStore (exact cardinalities, reachability) that
// counts the reads of ex:holdsBudget with no subject: the read a plan makes
// when it starts from that triple pattern instead of from the path.
type budgetScanStore struct {
	*store.MemoryStore
	scans int
}

func (s *budgetScanStore) isScan(p term.TriplePattern) bool {
	return p.Subject == nil && p.Predicate != nil && p.Predicate.Value() == "http://example.org/holdsBudget"
}

func (s *budgetScanStore) Triples(ctx context.Context, p term.TriplePattern, g term.Term) store.TripleIterator {
	if s.isScan(p) {
		s.scans++
	}
	return s.MemoryStore.Triples(ctx, p, g)
}

func (s *budgetScanStore) TriplesWithLimit(ctx context.Context, p term.TriplePattern, g term.Term, limit, offset int) store.TripleIterator {
	if s.isScan(p) {
		s.scans++
	}
	return s.MemoryStore.TriplesWithLimit(ctx, p, g, limit, offset)
}

// orgChartGraph is a binary tree of 255 employees of ex:org linked by
// ex:reportsTo; each of the 127 who have reports holds a budget.
func orgChartGraph(t *testing.T, st store.Store) (*graph.Graph, int) {
	t.Helper()
	ctx := t.Context()
	g := graph.NewGraph(graph.WithStore(st))
	u := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	const employees = 255
	for i := range employees {
		e := u("e%d", i)
		g.Add(ctx, e, u("worksFor"), u("org"))
		if i > 0 {
			g.Add(ctx, e, u("reportsTo"), u("e%d", (i-1)/2))
		}
		if 2*i+1 < employees {
			g.Add(ctx, e, u("holdsBudget"), u("budget%d", i))
		}
	}
	return g, employees
}

// A bound that covers every row changes the plan of the rows past the
// default: the NOT EXISTS below plans { ?e ex:reportsTo+ ?m . ?m
// ex:holdsBudget ?b } once per employee, with one probe each. A probed row
// starts from the few managers above ?e; once the default bound is spent,
// every later row starts from ex:holdsBudget and reads all 127 budgets. The
// answer (only the root has no one above) is the same every time.
func TestWithMaxPathProbesChangesThePlan(t *testing.T) {
	st := &budgetScanStore{MemoryStore: store.NewMemoryStore()}
	g, employees := orgChartGraph(t, st)
	q := exPrefix + `SELECT ?e WHERE { ?e ex:worksFor ex:org FILTER NOT EXISTS { ?e ex:reportsTo+ ?m . ?m ex:holdsBudget ?b } }`
	probes := 0
	pathProbeHook = func() { probes++ }
	defer func() { pathProbeHook = nil }()
	for _, tc := range []struct {
		name          string
		ctx           context.Context
		probes, scans int
	}{
		{"default", t.Context(), bgpMaxPathProbes, employees - bgpMaxPathProbes},
		{"covers every row", WithMaxPathProbes(t.Context(), employees), employees, 0},
		{"unbounded", WithMaxPathProbes(t.Context(), 0), employees, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probes, st.scans = 0, 0
			res, err := QueryContext(tc.ctx, g, q)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Bindings) != 1 || res.Bindings[0]["e"].String() != "http://example.org/e0" {
				t.Errorf("rows %v, want only ex:e0", res.Bindings)
			}
			if probes != tc.probes {
				t.Errorf("%d path probes, want %d", probes, tc.probes)
			}
			if st.scans != tc.scans {
				t.Errorf("%d rows read every ex:holdsBudget triple, want %d", st.scans, tc.scans)
			}
		})
	}
}

// The bound decides only which plans see probed paths, never an answer: the
// random queries of TestOptimisationsDoNotChangeAnswers give the same rows
// with the default bound, with one probe per evaluation and with no bound.
func TestPathProbeBoundDoesNotChangeAnswers(t *testing.T) {
	g := optGraph(t)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	probes := 0
	pathProbeHook = func() { probes++ }
	defer func() { pathProbeHook = nil }()
	n := 300
	if testing.Short() {
		n = 100
	}
	bounds := []int{1, 0}
	probesPerBound := make([]int, len(bounds))
	compared := 0
	for i := range n {
		gen := &optGen{r: rand.New(rand.NewPCG(20261119, uint64(i)))}
		q := gen.query()
		pq, err := Parse(q)
		if err != nil {
			continue
		}
		pq.PreBindSubqueries = true
		want, skipped, wantErr := optRowsContext(t.Context(), g, pq, root)
		if skipped {
			continue
		}
		rows := make([][]string, len(bounds))
		errs := make([]error, len(bounds))
		skip := false
		for j, bound := range bounds {
			probes = 0
			var sk bool
			rows[j], sk, errs[j] = optRowsContext(WithMaxPathProbes(t.Context(), bound), g, pq, root)
			probesPerBound[j] += probes
			skip = skip || sk
		}
		if skip {
			continue
		}
		compared++
		for j, bound := range bounds {
			if (errs[j] == nil) != (wantErr == nil) {
				t.Errorf("query %d, bound %d: error %v, default bound: %v\n%s", i, bound, errs[j], wantErr, q)
				continue
			}
			if wantErr == nil && !slices.Equal(rows[j], want) {
				t.Errorf("query %d: rows differ with bound %d\n%s\nbound %d (%d): %v\ndefault (%d): %v", i, bound, q, bound, len(rows[j]), head(rows[j]), len(want), head(want))
			}
		}
	}
	t.Logf("compared %d random queries; %d path probes with bound 1, %d without a bound", compared, probesPerBound[0], probesPerBound[1])
	if compared < n/3 {
		t.Errorf("only %d of %d queries compared: the generator is too weak", compared, n)
	}
	if probesPerBound[0] >= probesPerBound[1] {
		t.Errorf("bound 1 ran %d probes and no bound %d: no evaluation reached the bound, so nothing was compared", probesPerBound[0], probesPerBound[1])
	}
}
