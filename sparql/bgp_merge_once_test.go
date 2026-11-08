package sparql

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// The merge of a join is built once per evaluation: asking again returns the
// same pattern without allocating, however many rows ask.
func TestMergeJoinedIsBuiltOncePerEvaluation(t *testing.T) {
	bgp := func(s, o string) *BGP { return &BGP{Triples: []Triple{{Subject: s, Predicate: "<urn:p>", Object: o}}} }
	j := &JoinPattern{Left: bgp("?a", "?b"), Right: bgp("?b", "?c")}
	ec := newEvalCtx(t.Context())
	first := ec.mergeJoined(j)
	if m, ok := first.(*BGP); !ok || len(m.Triples) != 2 {
		t.Fatalf("not merged: %#v", first)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		if ec.mergeJoined(j) != first {
			t.Error("a second call built a new pattern")
		}
	}); allocs != 0 {
		t.Errorf("%v allocations per repeated call, want 0", allocs)
	}
	// A join that cannot be merged is remembered too: it is not recounted.
	stay := &JoinPattern{Left: bgp("_:x", "?b"), Right: bgp("_:x", "?c")}
	if ec.mergeJoined(stay) != Pattern(stay) {
		t.Error("BGPs sharing a label were merged")
	}
	if allocs := testing.AllocsPerRun(100, func() { ec.mergeJoined(stay) }); allocs != 0 {
		t.Errorf("%v allocations for a join that stays a join, want 0", allocs)
	}
	// Not a join: returned as is, no entry kept.
	if b := bgp("?a", "?b"); ec.mergeJoined(b) != Pattern(b) {
		t.Error("a BGP was not returned unchanged")
	}
	if len(ec.merged) != 2 {
		t.Errorf("%d remembered joins, want 2", len(ec.merged))
	}
	// A nil context merges every time but still works.
	var none *evalCtx
	if m, ok := none.mergeJoined(j).(*BGP); !ok || len(m.Triples) != 2 {
		t.Error("nil evalCtx did not merge")
	}
}

// A join of BGPs under an OPTIONAL is evaluated for every row of the left
// side. The merge must not be rebuilt for each of them: the allocations of
// the query grow with the rows it produces, not with a merge per row.
func TestMergedJoinUnderOptionalIsNotRebuiltPerRow(t *testing.T) {
	build := func(n int) *graph.Graph {
		g := graph.NewGraph()
		ex := func(f string, a ...any) term.URIRef {
			return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
		}
		for i := range n {
			s := ex("s%d", i)
			g.Add(t.Context(), s, ex("type"), ex("T"))
			g.Add(t.Context(), s, ex("p"), ex("o%d", i))
			g.Add(t.Context(), ex("o%d", i), ex("q"), term.NewLiteral(i))
		}
		return g
	}
	const q = exPrefix + `SELECT * WHERE { ?s ex:type ex:T OPTIONAL { ?s ex:p ?o . FILTER (true) ?o ex:q ?v } }`
	pq, err := Parse(q)
	if err != nil {
		t.Fatal(err)
	}
	allocsFor := func(n int) float64 {
		g := build(n)
		return testing.AllocsPerRun(5, func() {
			if _, err := EvalQuery(g, pq, nil); err != nil {
				t.Fatal(err)
			}
		}) / float64(n)
	}
	small, large := allocsFor(20), allocsFor(400)
	// Per-row cost is a constant; a merge rebuilt per row would add its
	// (BGP + slice + map) allocations to every row, and the large run could
	// not be cheaper per row than the small one's fixed costs spread out.
	if large > small {
		t.Errorf("allocations per row rise with the rows: %.1f at 20 rows, %.1f at 400", small, large)
	}
}

// Evaluating never modifies the parsed query, which is shared (parse cache):
// the same ParsedQuery evaluated concurrently (run with -race) keeps its
// shape, and its join is still a join of two BGPs.
func TestMergeJoinedLeavesTheParsedQueryAlone(t *testing.T) {
	g := mergeGraph(t, 20)
	pq, err := Parse(exPrefix + `SELECT * WHERE { ?c ex:type ?class . FILTER (?class IN (ex:ClassA)) ?c ex:size ?s }`)
	if err != nil {
		t.Fatal(err)
	}
	shape := func() string { return fmt.Sprintf("%T %s", pq.Where, describePattern(pq.Where)) }
	before := shape()
	var wg sync.WaitGroup
	rows := make([][]string, 8)
	for i := range rows {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := EvalQuery(g, pq, nil)
			if err != nil {
				t.Error(err)
				return
			}
			rows[i] = renderRows(res.Bindings)
		}()
	}
	wg.Wait()
	if after := shape(); after != before {
		t.Errorf("the parsed query changed:\n%s\n%s", before, after)
	}
	for i := 1; i < len(rows); i++ {
		if !slices.Equal(rows[0], rows[i]) {
			t.Fatal("concurrent evaluations of one parsed query disagree")
		}
	}
	if len(rows[0]) == 0 {
		t.Fatal("no rows: the test checks nothing")
	}
}

func describePattern(p Pattern) string {
	switch p := p.(type) {
	case *BGP:
		return fmt.Sprintf("BGP%v", p.Triples)
	case *JoinPattern:
		return "Join(" + describePattern(p.Left) + "," + describePattern(p.Right) + ")"
	case *FilterPattern:
		return "Filter(" + describePattern(p.Pattern) + ")"
	}
	return fmt.Sprintf("%T", p)
}
