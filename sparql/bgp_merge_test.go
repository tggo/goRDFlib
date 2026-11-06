package sparql

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// mergeGraph builds ex:root with n ex:ClassC parts of size ex:s3, two
// ex:ClassA parts of size ex:s1 (one of them flagged) and one ex:ClassB part,
// ex:t, of size ex:s2 that reaches ex:v through two intermediate nodes, so the
// sequence path ex:a/ex:b gives two identical rows.
func mergeGraph(t testing.TB, n int) *graph.Graph {
	ctx := context.Background()
	g := graph.NewGraph()
	ex := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	add := func(s term.Subject, p term.URIRef, o term.Term) {
		if err := g.Add(ctx, s, p, o); err != nil {
			t.Fatal(err)
		}
	}
	part := func(name, class, size string) term.URIRef {
		p := ex("%s", name)
		add(ex("root"), ex("part"), p)
		add(p, ex("type"), ex("%s", class))
		add(p, ex("size"), ex("%s", size))
		return p
	}
	for i := range n {
		part(fmt.Sprintf("p%d", i), "ClassC", "s3")
	}
	part("h0", "ClassA", "s1")
	add(part("h1", "ClassA", "s1"), ex("flag"), term.NewLiteral(true))
	tail := part("t", "ClassB", "s2")
	add(tail, ex("a"), ex("m1"))
	add(tail, ex("a"), ex("m2"))
	add(ex("m1"), ex("b"), ex("v"))
	add(ex("m2"), ex("b"), ex("v"))
	return g
}

// mergeRows evaluates q (prefixed with ex:) within a budget of intermediate
// solutions (0: none) and returns its rows as sorted strings, each with its
// variables in name order (?this left out): two results compare equal exactly
// when they hold the same rows the same number of times.
func mergeRows(g *graph.Graph, q string, pre map[string]term.Term, budget int) ([]string, error) {
	pq, err := Parse("PREFIX ex: <http://example.org/>\n" + q)
	if err != nil {
		return nil, err
	}
	res, err := EvalQueryContext(WithMaxSolutions(context.Background(), budget), g, pq, pre)
	if err != nil {
		if errors.Is(err, ErrTooManySolutions) {
			return nil, fmt.Errorf("budget of %d solutions exceeded", budget)
		}
		return nil, err
	}
	rows := make([]string, 0, len(res.Bindings))
	for _, b := range res.Bindings {
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
	return rows, nil
}

// A group split by a FILTER or a nested group is a join of BGPs. Evaluated as
// one BGP it must give exactly the rows of the same query with the second BGP
// written as a nested group ending in FILTER (true), which is never merged, in
// every construct around or inside the group, with ?this pre-bound and with
// the constant written in the text.
func TestJoinedBGPsMatchUnmergedJoin(t *testing.T) {
	g := mergeGraph(t, 30)
	const (
		a = ` ?this ex:part ?c , ?p . ?c ex:type ?class . `
		f = ` FILTER (?class IN (ex:ClassA)) `
		b = ` ?p ex:type ex:ClassB `
	)
	unmerged := func(bgp string) string { return "{ " + bgp + " FILTER (true) }" }
	cases := []struct {
		name, query, reference string
		rows                   int // expected row count, 0: not checked
	}{
		{"filter split", `SELECT ?c ?p WHERE { ` + a + f + b + ` }`,
			`SELECT ?c ?p WHERE { ` + a + f + unmerged(b) + ` }`, 2},
		{"filter split, written as one BGP", `SELECT ?c ?p WHERE { ` + a + f + b + ` }`,
			`SELECT ?c ?p WHERE { ` + a + b + f + ` }`, 2},
		{"paths and NOT EXISTS over both halves",
			`SELECT ?c ?p ?v WHERE { ?this ex:part+ ?c , ?p . ?c ex:type/ex:sub* ?class ; ex:size ?any .
			   FILTER (?class IN (ex:ClassA)) ?p ex:type/ex:sub* ex:ClassB ; ex:size ?v .
			   FILTER NOT EXISTS { ?c ex:size ?v } }`,
			`SELECT ?c ?p ?v WHERE { ?this ex:part+ ?c , ?p . ?c ex:type/ex:sub* ?class ; ex:size ?any .
			   FILTER (?class IN (ex:ClassA)) { ?p ex:type/ex:sub* ex:ClassB ; ex:size ?v FILTER (true) }
			   FILTER NOT EXISTS { ?c ex:size ?v } }`, 2},
		{"nested group, repeated solutions", `SELECT ?p ?x WHERE { ?this ex:part ?p . FILTER (?p != ex:p0) { ?p ex:a/ex:b ?x } }`,
			`SELECT ?p ?x WHERE { ?this ex:part ?p . FILTER (?p != ex:p0) ` + unmerged(`?p ex:a/ex:b ?x`) + ` }`, 2},
		{"filter over both leaves", `SELECT ?c ?p WHERE { ?this ex:part ?c . ?c ex:type ex:ClassA FILTER (?c != ?p) ?this ex:part ?p . ?p ex:size ex:s1 }`,
			`SELECT ?c ?p WHERE { ?this ex:part ?c . ?c ex:type ex:ClassA FILTER (?c != ?p) ` + unmerged(`?this ex:part ?p . ?p ex:size ex:s1`) + ` }`, 2},
		{"optional after the group", `SELECT * WHERE { ` + a + f + b + ` OPTIONAL { ?p ex:a ?m } }`,
			`SELECT * WHERE { ` + a + f + unmerged(b) + ` OPTIONAL { ?p ex:a ?m } }`, 4},
		{"split group inside an optional, its filter the condition",
			`SELECT * WHERE { ?this ex:part ?c . ?c ex:type ex:ClassA OPTIONAL { ?c ex:size ?s FILTER (?s != ex:s3) ?x ex:size ?s } }`,
			`SELECT * WHERE { ?this ex:part ?c . ?c ex:type ex:ClassA OPTIONAL { ?c ex:size ?s FILTER (?s != ex:s3) ` + unmerged(`?x ex:size ?s`) + ` } }`, 4},
		{"minus", `SELECT * WHERE { ` + a + f + b + ` MINUS { ?c ex:flag ?any } }`,
			`SELECT * WHERE { ` + a + f + unmerged(b) + ` MINUS { ?c ex:flag ?any } }`, 1},
		{"split group inside minus", `SELECT ?c WHERE { ?this ex:part ?c . ?c ex:type ex:ClassA MINUS { ?c ex:flag ?any FILTER (BOUND(?any)) ?c ex:size ex:s1 } }`,
			`SELECT ?c WHERE { ?this ex:part ?c . ?c ex:type ex:ClassA MINUS { ?c ex:flag ?any FILTER (BOUND(?any)) ` + unmerged(`?c ex:size ex:s1`) + ` } }`, 1},
		{"union", `SELECT * WHERE { { ` + a + f + b + ` } UNION { ?this ex:part ?p . FILTER (?p = ex:t) ?p ex:a ?m } }`,
			`SELECT * WHERE { { ` + a + f + unmerged(b) + ` } UNION { ?this ex:part ?p . FILTER (?p = ex:t) ` + unmerged(`?p ex:a ?m`) + ` } }`, 4},
		{"subquery with order, limit and offset", `SELECT * WHERE { { SELECT ?c ?p WHERE { ` + a + f + b + ` } ORDER BY ?c ?p LIMIT 1 OFFSET 1 } ?c ex:size ?s }`,
			`SELECT * WHERE { { SELECT ?c ?p WHERE { ` + a + f + unmerged(b) + ` } ORDER BY ?c ?p LIMIT 1 OFFSET 1 } ?c ex:size ?s }`, 1},
		{"order, limit and offset", `SELECT ?p ?x WHERE { ?this ex:part ?p . FILTER (?p != ex:t) ?p ex:size ?x } ORDER BY ?p LIMIT 5 OFFSET 10`,
			`SELECT ?p ?x WHERE { ?this ex:part ?p . FILTER (?p != ex:t) ` + unmerged(`?p ex:size ?x`) + ` } ORDER BY ?p LIMIT 5 OFFSET 10`, 5},
		{"split group in an EXISTS body", `SELECT ?p WHERE { ?this ex:part ?p FILTER EXISTS { ?p ex:size ?s FILTER (?s != ex:s3) ?p ex:type ?k } }`,
			`SELECT ?p WHERE { ?this ex:part ?p FILTER EXISTS { ?p ex:size ?s FILTER (?s != ex:s3) ` + unmerged(`?p ex:type ?k`) + ` } }`, 3},
		{"blank node label in two leaves (not merged)", `SELECT ?p ?v ?w WHERE { ?this ex:part ?p . _:n ex:b ?v FILTER (?p = ex:t) _:n ex:b ?w }`,
			`SELECT ?p ?v ?w WHERE { ?this ex:part ?p . _:n ex:b ?v FILTER (?p = ex:t) ` + unmerged(`_:n ex:b ?w`) + ` }`, 4},
		{"count", `SELECT (COUNT(*) AS ?n) WHERE { ` + a + f + b + ` }`,
			`SELECT (COUNT(*) AS ?n) WHERE { ` + a + f + unmerged(b) + ` }`, 1},
		{"distinct", `SELECT DISTINCT ?p WHERE { ?this ex:part ?p . FILTER (?p != ex:p0) { ?p ex:a/ex:b ?x } }`,
			`SELECT DISTINCT ?p WHERE { ?this ex:part ?p . FILTER (?p != ex:p0) ` + unmerged(`?p ex:a/ex:b ?x`) + ` }`, 1},
	}
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	for _, c := range cases {
		for _, prebound := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/prebound=%v", c.name, prebound), func(t *testing.T) {
				query, reference, pre := c.query, c.reference, root
				if !prebound {
					query = strings.ReplaceAll(query, "?this", "ex:root")
					reference = strings.ReplaceAll(reference, "?this", "ex:root")
					pre = nil
				}
				want, err := mergeRows(g, reference, pre, 0)
				if err != nil {
					t.Fatal(err)
				}
				if len(want) != c.rows {
					t.Fatalf("the reference gives %d rows, want %d: %v", len(want), c.rows, want)
				}
				got, err := mergeRows(g, query, pre, 0)
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, want) {
					t.Errorf("got\n%v\nwant\n%v", got, want)
				}
			})
		}
	}

	// LIMIT without ORDER BY picks unspecified rows: as many as asked for, each
	// a row of the full result. Without ?this pre-bound, LIMIT is pushed down
	// into the streamed evaluation, which merges the join too.
	for _, prebound := range []bool{true, false} {
		q, pre := `SELECT ?p ?x WHERE { ?this ex:part ?p . FILTER (?p != ex:t) ?p ex:size ?x }`, root
		if !prebound {
			q, pre = strings.ReplaceAll(q, "?this", "ex:root"), nil
		}
		all, err := mergeRows(g, q, pre, 0)
		if err != nil {
			t.Fatal(err)
		}
		got, err := mergeRows(g, q+" LIMIT 3 OFFSET 2", pre, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Errorf("LIMIT 3 OFFSET 2 (pre-bound %v): %d rows", prebound, len(got))
		}
		for _, r := range got {
			if !slices.Contains(all, r) {
				t.Errorf("LIMIT 3 OFFSET 2 (pre-bound %v): row %q is not a solution", prebound, r)
			}
		}
	}
}

// The first BGP of a FILTER-split group alone pairs every part with every
// part (303 x 303 rows) before the second restricts ?p to the one ex:ClassB.
// Evaluated as one BGP, the planner starts from the restriction and the query
// stays far below a budget of 20,000 intermediate solutions.
func TestJoinedBGPsAreEvaluatedAsOneBGP(t *testing.T) {
	g := mergeGraph(t, 300)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	for _, q := range []string{
		`SELECT ?c ?p WHERE { ?this ex:part ?c , ?p . ?c ex:type ?class . FILTER (?class IN (ex:ClassA)) ?p ex:type ex:ClassB }`,
		`SELECT ?c ?p ?v WHERE { ?this ex:part+ ?c , ?p . ?c ex:type/ex:sub* ?class ; ex:size ?any .
		   FILTER (?class IN (ex:ClassA)) ?p ex:type/ex:sub* ex:ClassB ; ex:size ?v .
		   FILTER NOT EXISTS { ?c ex:size ?v } }`,
	} {
		if _, err := mergeRows(g, q, root, 20_000); err != nil {
			t.Errorf("pre-bound: %v (the split group was evaluated as nested BGPs)\n%s", err, q)
		}
		if _, err := mergeRows(g, strings.ReplaceAll(q, "?this", "ex:root"), nil, 20_000); err != nil {
			t.Errorf("constant: %v (the split group was evaluated as nested BGPs)\n%s", err, q)
		}
	}
}

// Leaves that are not all basic graph patterns, or that share a blank node
// label, are left as a join; the query's own patterns are not modified.
func TestMergeJoinedBGPsKeepsOtherJoins(t *testing.T) {
	bgp := func(s, o string) *BGP { return &BGP{Triples: []Triple{{Subject: s, Predicate: "<urn:p>", Object: o}}} }
	l, r1, r2 := bgp("?a", "?b"), bgp("?b", "?c"), bgp("?c", "?d")
	if m, ok := mergeJoinedBGPs(&JoinPattern{Left: l, Right: &JoinPattern{Left: r1, Right: r2}}).(*BGP); !ok || len(m.Triples) != 3 {
		t.Errorf("join of three BGPs: got %#v", m)
	} else if m.Triples[0] != l.Triples[0] || m.Triples[2] != r2.Triples[0] {
		t.Errorf("join of three BGPs: patterns out of order: %v", m.Triples)
	}
	if len(l.Triples) != 1 || len(r1.Triples) != 1 || len(r2.Triples) != 1 {
		t.Error("a leaf was modified")
	}
	if _, ok := mergeJoinedBGPs(&JoinPattern{Left: bgp("?a", "?b"), Right: &FilterPattern{Pattern: bgp("?b", "?c")}}).(*JoinPattern); !ok {
		t.Error("a join with a FILTER side was merged")
	}
	if _, ok := mergeJoinedBGPs(&JoinPattern{Left: bgp("?a", "?b"), Right: &OptionalPattern{Main: bgp("?b", "?c"), Optional: bgp("?c", "?d")}}).(*JoinPattern); !ok {
		t.Error("a join with an OPTIONAL side was merged")
	}
	if _, ok := mergeJoinedBGPs(&JoinPattern{Left: bgp("_:x", "?b"), Right: bgp("_:x", "?c")}).(*JoinPattern); !ok {
		t.Error("BGPs sharing a blank node label were merged")
	}
	if _, ok := mergeJoinedBGPs(&JoinPattern{Left: bgp("?a", "?b"), Right: bgp("<<( _:x <urn:p> ?c )>>", "?d")}).(*JoinPattern); !ok {
		t.Error("a BGP with a blank node in a triple term was merged")
	}
	if m, ok := mergeJoinedBGPs(&JoinPattern{Left: bgp("_:x", "?b"), Right: bgp("_:y", "?b")}).(*BGP); !ok || len(m.Triples) != 2 {
		t.Error("BGPs with distinct blank node labels were not merged")
	}
	if m, ok := mergeJoinedBGPs(&JoinPattern{Left: &BGP{Triples: []Triple{{Subject: "_:x", Predicate: "<urn:p>", Object: "_:x"}}}, Right: bgp("?a", "?b")}).(*BGP); !ok || len(m.Triples) != 2 {
		t.Error("a label repeated within one BGP stopped the merge")
	}
	if p := bgp("?a", "?b"); mergeJoinedBGPs(p) != Pattern(p) {
		t.Error("a BGP was not returned unchanged")
	}
}
