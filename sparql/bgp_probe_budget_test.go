package sparql

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// orgTree builds n employees whose manager is a random earlier employee (a
// random tree), all working for urn:org; employees chosen at random hold
// budgets. Employee 0 has no manager.
func orgTree(t testing.TB, r *rand.Rand, n int, budgetShare float64) *graph.Graph {
	t.Helper()
	ctx := t.Context()
	g := graph.NewGraph()
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	emp := func(i int) term.URIRef { return u(fmt.Sprintf("e%d", i)) }
	for i := range n {
		g.Add(ctx, emp(i), u("worksFor"), u("org"))
		if i > 0 {
			g.Add(ctx, emp(i), u("reportsTo"), emp(r.IntN(i)))
		}
		if r.Float64() < budgetShare {
			g.Add(ctx, emp(i), u("holdsBudget"), term.NewLiteral(fmt.Sprintf("b%d", i)))
		}
	}
	return g
}

var probeBudgetQueries = []string{
	`SELECT ?e WHERE { ?e <urn:worksFor> <urn:org> FILTER NOT EXISTS { ?e <urn:reportsTo>+ ?m . ?m <urn:holdsBudget> ?b } }`,
	`SELECT ?e ?b WHERE { ?e <urn:worksFor> <urn:org> OPTIONAL { ?e <urn:reportsTo>+ ?m . ?m <urn:holdsBudget> ?b } }`,
	`SELECT ?e WHERE { ?e <urn:worksFor> <urn:org> MINUS { ?e <urn:reportsTo>* ?m . ?m <urn:holdsBudget> ?b } }`,
	`SELECT ?e ?m WHERE { ?e <urn:worksFor> <urn:org> FILTER EXISTS { ?m <urn:holdsBudget> ?b . ?e <urn:reportsTo>/<urn:reportsTo> ?m } }`,
}

// The probe allowance (probeAllowance) only decides which plan runs: with no
// floor, the default and an unlimited one, and with probing off, random
// organisations answer every per-row query with the same multiset of rows.
// The hook proves the queries plan with probes at all.
func TestProbeBudgetDoesNotChangeAnswers(t *testing.T) {
	defer func() { probeFloor = bgpProbeFloor; setOpts(false); pathProbeHook = nil }()
	probes := 0
	pathProbeHook = func() { probes++ }
	runs := 12
	if testing.Short() {
		runs = 4
	}
	for i := range runs {
		r := rand.New(rand.NewPCG(20261119, uint64(i)))
		g := orgTree(t, r, 150+r.IntN(250), []float64{0.1, 0.5, 0.9}[i%3])
		for _, q := range probeBudgetQueries {
			var want []string
			for j, tc := range []struct {
				floor int
				off   bool
			}{{bgpProbeFloor, true}, {0, false}, {bgpProbeFloor, false}, {1 << 40, false}} {
				probeFloor = tc.floor
				setOpts(tc.off)
				res, err := Query(g, q)
				if err != nil {
					t.Fatal(err)
				}
				got := renderRows(res.Bindings)
				slices.Sort(got)
				if j == 0 {
					want = got
				} else if !slices.Equal(got, want) {
					t.Fatalf("run %d floor %d off=%v: %d rows, want %d\n%s", i, tc.floor, tc.off, len(got), len(want), q)
				}
			}
		}
	}
	if probes == 0 {
		t.Error("no query planned a path probe")
	}
}

// The cliff: an EXISTS planned once per outer row, whose path from the bound
// employee reaches a few managers while the pattern it joins matches half the
// employees. Probing must keep the plan on the path for every row, not only
// the first few: it is far cheaper than
// planning without probes, which starts from the large pattern.
func TestProbeBudgetKeepsPlanOnPathAcrossRows(t *testing.T) {
	g := graph.NewGraph()
	ctx := t.Context()
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	emp := func(i int) term.URIRef { return u(fmt.Sprintf("e%d", i)) }
	const n = 600
	for i := range n {
		g.Add(ctx, emp(i), u("worksFor"), u("org"))
		if i > 0 {
			g.Add(ctx, emp(i), u("reportsTo"), emp((i-1)/4))
		}
		if i*4+1 < n {
			g.Add(ctx, emp(i), u("holdsBudget"), term.NewLiteral(fmt.Sprintf("b%d", i)))
		}
	}
	q := `SELECT ?e WHERE { ?e <urn:worksFor> <urn:org> FILTER NOT EXISTS { ?e <urn:reportsTo>+ ?m . ?m <urn:holdsBudget> ?b } }`
	defer setOpts(false)
	allocs := [2]float64{}
	for i, off := range []bool{false, true} {
		setOpts(off)
		allocs[i] = testing.AllocsPerRun(2, func() {
			res, err := Query(g, q)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Bindings) != 1 {
				t.Fatalf("%d rows, want only the root", len(res.Bindings))
			}
		})
	}
	if allocs[0]*5 > allocs[1] {
		t.Errorf("with probes %.0f allocations, without %.0f: want at least 5x fewer", allocs[0], allocs[1])
	}
}
