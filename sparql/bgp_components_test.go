package sparql

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

const exPrefix = "PREFIX ex: <http://example.org/>\n"

// componentsGraph builds a small tree: ex:root has six assemblies, each with
// five items. Item j of an assembly is an ex:ClassA (j = 0), an ex:ClassB
// (j = 1) or an ex:ClassC; every fourth item has a second type. Even items have
// an ex:val. Two items reach one value through two intermediate nodes (a
// sequence path gives two identical rows), and two have a blank-node note.
func componentsGraph(t testing.TB, opts ...graph.GraphOption) *graph.Graph {
	ctx := context.Background()
	g := graph.NewGraph(opts...)
	ex := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	add := func(s term.Subject, p term.URIRef, o term.Term) {
		if err := g.Add(ctx, s, p, o); err != nil {
			t.Fatal(err)
		}
	}
	classes := []string{"ClassA", "ClassB", "ClassC", "ClassC", "ClassC"}
	n := 0
	for i := range 6 {
		asm := ex("asm%d", i)
		add(ex("root"), ex("part"), asm)
		for j := range 5 {
			item := ex("item%d_%d", i, j)
			add(asm, ex("part"), item)
			add(item, ex("type"), ex("%s", classes[j]))
			if n%4 == 0 {
				add(item, ex("type"), ex("Extra"))
			}
			if n%2 == 0 {
				add(item, ex("val"), term.NewLiteral(n))
			}
			n++
		}
	}
	for _, s := range []string{"item0_0", "item1_1"} {
		add(ex("%s", s), ex("a"), ex("m1"))
		add(ex("%s", s), ex("a"), ex("m2"))
	}
	add(ex("m1"), ex("b"), ex("v"))
	add(ex("m2"), ex("b"), ex("v"))
	for _, s := range []string{"item2_0", "item3_1"} {
		note := term.NewBNode("note-" + s)
		add(ex("%s", s), ex("note"), note)
		add(note, ex("text"), term.NewLiteral("about "+s))
	}
	return g
}

// queryRows returns the rows of a SELECT query as sorted strings, each with
// its variables in name order (?this left out), so that two results compare
// equal exactly when they hold the same rows the same number of times.
func queryRows(t *testing.T, g *graph.Graph, q string, pre map[string]term.Term) []string {
	t.Helper()
	pq, err := Parse(exPrefix + q)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	res, err := EvalQuery(g, pq, pre)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return renderRows(res.Bindings)
}

func renderRows(bindings []map[string]rdflibgo.Term) []string {
	rows := make([]string, 0, len(bindings))
	for _, b := range bindings {
		keys := make([]string, 0, len(b))
		for k := range b {
			if k != "this" {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		var sb strings.Builder
		for _, k := range keys {
			sb.WriteString(k + "=" + b[k].N3() + " ")
		}
		rows = append(rows, sb.String())
	}
	slices.Sort(rows)
	return rows
}

func TestBGPComponents(t *testing.T) {
	tp := func(s, p, o string) Triple { return Triple{Subject: s, Predicate: p, Object: o} }
	this := map[string]rdflibgo.Term{"this": rdflibgo.NewURIRefUnsafe("http://example.org/root")}
	for _, c := range []struct {
		name     string
		triples  []Triple
		bindings map[string]rdflibgo.Term
		want     []int // group sizes, nil: not split
	}{
		{"connected chain", []Triple{tp("?a", "<p>", "?b"), tp("?b", "<p>", "?c")}, nil, nil},
		{"two groups", []Triple{tp("?a", "<p>", "?b"), tp("?c", "<p>", "?d"), tp("?b", "<q>", "?e")}, nil, []int{2, 1}},
		{"a bound variable joins nothing", []Triple{tp("?this", "<p>", "?a"), tp("?this", "<p>", "?b")}, this, []int{1, 1}},
		{"an unbound one does", []Triple{tp("?this", "<p>", "?a"), tp("?this", "<p>", "?b")}, nil, nil},
		{"a predicate variable joins", []Triple{tp("?a", "?p", "<o>"), tp("?b", "?p", "<o>")}, nil, nil},
		{"a blank node label joins", []Triple{tp("_:x", "<p>", "?a"), tp("_:x", "<q>", "?b")}, nil, nil},
		{"distinct labels do not", []Triple{tp("_:x", "<p>", "?a"), tp("_:y", "<q>", "?b")}, nil, []int{1, 1}},
		{"a ground pattern is its own group", []Triple{tp("<s>", "<p>", "<o>"), tp("?a", "<p>", "?b")}, nil, []int{1, 1}},
		{"a triple term with variables stops the split", []Triple{tp("<<( ?s <p> ?o )>>", "<r>", "?a"), tp("?b", "<p>", "?c")}, nil, nil},
		{"single pattern", []Triple{tp("?a", "<p>", "?b")}, nil, nil},
		{"two interleaved chains of 10 (map lookup)", interleavedChains(10, "?a", "?b"), nil, []int{10, 10}},
		{"one chain of 20 (map lookup)", interleavedChains(20, "?a"), nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			comps := bgpComponents(c.triples, c.bindings)
			var got []int
			for _, g := range comps {
				got = append(got, len(g))
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("group sizes %v, want %v", got, c.want)
			}
		})
	}
}

// interleavedChains returns ?a0 <p> ?a1 . ?b0 <p> ?b1 . ?a1 <p> ?a2 . ...: one
// chain of n patterns per variable prefix, written alternately.
func interleavedChains(n int, prefixes ...string) []Triple {
	var ts []Triple
	for i := range n {
		for _, v := range prefixes {
			ts = append(ts, Triple{Subject: fmt.Sprint(v, i), Predicate: "<p>", Object: fmt.Sprint(v, i+1)})
		}
	}
	return ts
}

// Splitting a BGP into groups gives exactly the solutions of the nested loops
// over the same planned patterns, repeated solutions included.
func TestDisjointBGPGroupsMatchNestedLoops(t *testing.T) {
	g := componentsGraph(t)
	ex := func(s string) string { return "<http://example.org/" + s + ">" }
	tp := func(s, p, o string) Triple { return Triple{Subject: s, Predicate: ex(p), Object: o} }
	seq := Triple{Subject: "?c", Object: "?o", PredicatePath: mustPath(t, "ex:a/ex:b")}
	root := map[string]rdflibgo.Term{"this": rdflibgo.NewURIRefUnsafe("http://example.org/root")}
	for _, c := range []struct {
		name     string
		triples  []Triple
		bindings map[string]rdflibgo.Term
	}{
		{"two groups", []Triple{tp("?a", "type", ex("ClassA")), tp("?a", "val", "?x"), tp("?b", "type", ex("ClassB"))}, nil},
		{"three groups, repeated solutions", []Triple{tp("?a", "type", "?t"), seq, tp("?b", "type", ex("ClassB"))}, nil},
		{"joined only through a bound variable", []Triple{tp("?this", "part", "?m"), tp("?m", "part", "?a"), tp("?this", "part", "?n"), tp("?n", "part", "?b")}, root},
		{"a group with no solution", []Triple{tp("?a", "type", ex("ClassA")), tp("?b", "type", ex("Missing"))}, nil},
		{"a ground pattern that holds", []Triple{tp(ex("asm0"), "part", ex("item0_0")), tp("?b", "type", ex("ClassB"))}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := c.bindings
			if b == nil {
				b = map[string]rdflibgo.Term{}
			}
			if bgpComponents(c.triples, b) == nil {
				t.Fatal("the patterns are not split: the test checks nothing")
			}
			planned := orderBGP(nil, g, c.triples, b, nil)
			got := renderRows(evalBGPInOrder(nil, g, planned, b, nil, 0))
			want := renderRows(collectBGPSolutions(nil, g, planned, b, nil, 0))
			if !slices.Equal(got, want) {
				t.Errorf("groups give\n%v\nnested loops give\n%v", got, want)
			}
			if lim := len(want) / 2; lim > 0 {
				if n := len(evalBGPInOrder(nil, g, planned, b, nil, lim)); n != lim {
					t.Errorf("limit %d: %d solutions", lim, n)
				}
			}
		})
	}
}

func mustPath(t *testing.T, s string) paths.Path {
	t.Helper()
	pq, err := Parse(exPrefix + "SELECT * WHERE { ?s " + s + " ?o }")
	if err != nil {
		t.Fatal(err)
	}
	return pq.Where.(*BGP).Triples[0].PredicatePath
}

// Each query gives the same rows as the same query with its groups written as
// separate nested groups ending in FILTER (true), a form no BGP optimisation
// looks into, in every construct around or inside the BGP.
func TestDisjointBGPGroupsInQueries(t *testing.T) {
	g := componentsGraph(t)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	const (
		ga = `?a ex:type ex:ClassA . ?a ex:val ?x`
		gb = `?b ex:type ex:ClassB`
		gc = `?c ex:a/ex:b ?o`
	)
	sep := func(groups ...string) string {
		var sb strings.Builder
		for _, gr := range groups {
			sb.WriteString("{ " + gr + " FILTER (true) } ")
		}
		return sb.String()
	}
	for _, c := range []struct {
		name, query, reference string
		pre                    map[string]term.Term
	}{
		{"two groups", `SELECT * WHERE { ` + ga + ` . ` + gb + ` }`, `SELECT * WHERE { ` + sep(ga, gb) + ` }`, nil},
		{"three groups, repeated solutions", `SELECT * WHERE { ` + ga + ` . ` + gb + ` . ` + gc + ` }`, `SELECT * WHERE { ` + sep(ga, gb, gc) + ` }`, nil},
		{"pre-bound variable", `SELECT ?a ?b WHERE { ?this ex:part ?m . ?m ex:part ?a . ?a ex:type ex:ClassA . ?this ex:part ?n . ?n ex:part ?b . ?b ex:type ex:ClassB }`,
			`SELECT ?a ?b WHERE { ` + sep(`?this ex:part ?m . ?m ex:part ?a . ?a ex:type ex:ClassA`, `?this ex:part ?n . ?n ex:part ?b . ?b ex:type ex:ClassB`) + ` }`, root},
		{"constant in the text", `SELECT ?a ?b WHERE { ex:root ex:part ?m . ?m ex:part ?a . ?a ex:type ex:ClassA . ex:root ex:part ?n . ?n ex:part ?b . ?b ex:type ex:ClassB }`,
			`SELECT ?a ?b WHERE { ` + sep(`ex:root ex:part ?m . ?m ex:part ?a . ?a ex:type ex:ClassA`, `ex:root ex:part ?n . ?n ex:part ?b . ?b ex:type ex:ClassB`) + ` }`, nil},
		{"filter over both groups", `SELECT * WHERE { ` + ga + ` . ` + gb + ` FILTER (STR(?a) < STR(?b)) }`,
			`SELECT * WHERE { ` + sep(ga, gb) + ` FILTER (STR(?a) < STR(?b)) }`, nil},
		{"filter in a nested group does not see the outer group", `SELECT * WHERE { ` + gb + ` { ` + ga + ` . ` + gc + ` FILTER (!BOUND(?b)) } }`,
			`SELECT * WHERE { ` + gb + ` { ` + sep(ga, gc) + ` FILTER (!BOUND(?b)) } }`, nil},
		{"optional after the groups", `SELECT * WHERE { ` + ga + ` . ` + gb + ` OPTIONAL { ?b ex:val ?y } }`,
			`SELECT * WHERE { ` + sep(ga, gb) + ` OPTIONAL { ?b ex:val ?y } }`, nil},
		{"groups inside an optional", `SELECT * WHERE { ` + gb + ` OPTIONAL { ?b ex:val ?y . ?z ex:type ex:ClassA } }`,
			`SELECT * WHERE { ` + gb + ` OPTIONAL { ` + sep(`?b ex:val ?y`, `?z ex:type ex:ClassA`) + ` } }`, nil},
		{"minus", `SELECT * WHERE { ` + ga + ` . ` + gb + ` MINUS { ?b ex:type ex:Extra } }`,
			`SELECT * WHERE { ` + sep(ga, gb) + ` MINUS { ?b ex:type ex:Extra } }`, nil},
		{"groups inside minus", `SELECT * WHERE { ` + gb + ` MINUS { ?b ex:val ?v . ?z ex:type ex:ClassA } }`,
			`SELECT * WHERE { ` + gb + ` MINUS { ` + sep(`?b ex:val ?v`, `?z ex:type ex:ClassA`) + ` } }`, nil},
		{"union", `SELECT * WHERE { { ` + ga + ` . ` + gb + ` } UNION { ` + gb + ` . ` + gc + ` } }`,
			`SELECT * WHERE { { ` + sep(ga, gb) + ` } UNION { ` + sep(gb, gc) + ` } }`, nil},
		{"subquery with order, limit and offset", `SELECT * WHERE { { SELECT ?a ?b WHERE { ` + ga + ` . ` + gb + ` } ORDER BY ?a ?b LIMIT 7 OFFSET 2 } ?a ex:type ?t }`,
			`SELECT * WHERE { { SELECT ?a ?b WHERE { ` + sep(ga, gb) + ` } ORDER BY ?a ?b LIMIT 7 OFFSET 2 } ?a ex:type ?t }`, nil},
		{"order, limit and offset", `SELECT * WHERE { ` + ga + ` . ` + gb + ` } ORDER BY ?a ?b LIMIT 5 OFFSET 3`,
			`SELECT * WHERE { ` + sep(ga, gb) + ` } ORDER BY ?a ?b LIMIT 5 OFFSET 3`, nil},
		{"shared predicate variable", `SELECT * WHERE { ?a ?p ex:ClassA . ?b ?p ?o }`, `SELECT * WHERE { { ?a ?p ex:ClassA FILTER (true) } { ?b ?p ?o FILTER (true) } }`, nil},
		{"count", `SELECT (COUNT(*) AS ?n) WHERE { ` + ga + ` . ` + gb + ` . ` + gc + ` }`, `SELECT (COUNT(*) AS ?n) WHERE { ` + sep(ga, gb, gc) + ` }`, nil},
		{"distinct", `SELECT DISTINCT ?b ?o WHERE { ` + gb + ` . ` + gc + ` }`, `SELECT DISTINCT ?b ?o WHERE { ` + sep(gb, gc) + ` }`, nil},
		{"blank nodes in the data", `SELECT * WHERE { ?a ex:note ?n . ?n ex:text ?s . ` + gb + ` }`,
			`SELECT * WHERE { ` + sep(`?a ex:note ?n . ?n ex:text ?s`, gb) + ` }`, nil},
		{"blank node label shared by two groups", `SELECT * WHERE { _:n ex:text ?s . ` + gb + ` . _:n ex:note ?q }`,
			`SELECT * WHERE { ` + sep(`_:n ex:text ?s . _:n ex:note ?q`, gb) + ` }`, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			want := queryRows(t, g, c.reference, c.pre)
			if len(want) == 0 {
				t.Fatal("the reference has no row: the test checks nothing")
			}
			if got := queryRows(t, g, c.query, c.pre); !slices.Equal(got, want) {
				t.Errorf("got\n%v\nwant\n%v", got, want)
			}
		})
	}

	// LIMIT without ORDER BY picks unspecified rows: as many as asked for, each
	// a row of the full result. With ?this pre-bound the BGP is evaluated in
	// full first; without, LIMIT is pushed down into the nested loops.
	for _, pre := range []map[string]term.Term{root, nil} {
		q := `SELECT ?a ?b WHERE { ?this ex:part ?m . ?m ex:part ?a . ?a ex:type ex:ClassA . ?b ex:type ex:ClassB }`
		if pre == nil {
			q = strings.ReplaceAll(q, "?this", "ex:root")
		}
		all := queryRows(t, g, q, pre)
		got := queryRows(t, g, q+" LIMIT 4", pre)
		if len(got) != 4 {
			t.Errorf("LIMIT 4 (pre-bound %v): %d rows", pre != nil, len(got))
		}
		for _, r := range got {
			if !slices.Contains(all, r) {
				t.Errorf("LIMIT 4 (pre-bound %v): row %q is not a solution", pre != nil, r)
			}
		}
	}
}

// Each group is evaluated once: the second group is not matched again for
// every solution of the first.
func TestDisjointBGPGroupsAreEvaluatedOnce(t *testing.T) {
	st := &probingStore{Store: store.NewMemoryStore()}
	g := graph.NewGraph(graph.WithStore(st))
	ctx := t.Context()
	ex := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	for i := range 20 {
		g.Add(ctx, ex("a%d", i), ex("type"), ex("ClassA"))
	}
	for i := range 200 {
		g.Add(ctx, ex("b%d", i), ex("val"), term.NewLiteral(i))
	}
	q := `SELECT * WHERE { ?a ex:type ex:ClassA . ?b ex:val ?v }`
	st.pulled = 0
	rows := queryRows(t, g, q, nil)
	if len(rows) != 20*200 {
		t.Fatalf("%d rows, want 4000", len(rows))
	}
	// Nested loops pull 20 + 20 x 200 triples; one pass per group pulls 220,
	// plus the planner's probes.
	if st.pulled > 1000 {
		t.Errorf("pulled %d triples: the second group was evaluated per row of the first", st.pulled)
	}
}

// The cross product of groups is built row by row: a solution budget stops it
// without first building the product of two groups out of three.
func TestDisjointBGPGroupsRespectTheBudget(t *testing.T) {
	g := crossGraph(100)
	pq, err := Parse(`SELECT * WHERE { ?a ?b ?c . ?d ?e ?f . ?g ?h ?i }`)
	if err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err = EvalQueryContext(WithMaxSolutions(t.Context(), 10_000), g, pq, nil)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrTooManySolutions) {
		t.Fatalf("err = %v, want ErrTooManySolutions", err)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 32<<20 {
		t.Errorf("allocated %d MiB for a budget of 10,000 solutions", alloc>>20)
	}
}
