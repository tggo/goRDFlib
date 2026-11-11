package shacl

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// plainStore hides every optional interface of the store it wraps, so the
// SPARQL engine sees neither cardinalities, counts nor reachability.
type plainStore struct{ store.Store }

// diffQueries are the query shapes the planner, the COUNT shortcut and path
// pushdown treat differently, including the ones implementations get wrong:
// a variable repeated within a pattern, paths with one endpoint bound and
// cycles, a variable bound on one side of OPTIONAL/EXISTS.
var diffQueries = []string{
	`SELECT * WHERE { ?s ?p ?o }`,
	`SELECT * WHERE { ?x ex:p0 ?x }`,
	`SELECT * WHERE { ?s ?s ?o }`,
	`SELECT * WHERE { ?a ex:p0 ?b . ?b ex:p1 ?c . ?c ex:p2 ?a }`,
	`SELECT * WHERE { ?a ex:p1 ex:n1 . ?a ex:p0 ?b . ?b ex:p3 ?c }`,
	`SELECT ?o WHERE { ex:n0 ex:p0+ ?o }`,
	`SELECT ?o WHERE { ex:n0 ex:p0* ?o }`,
	`SELECT ?s WHERE { ?s ex:p1+ ex:n2 }`,
	`SELECT ?o WHERE { ex:n1 ^ex:p0+ ?o }`,
	`SELECT ?o WHERE { ex:n0 (ex:p0|ex:p1)+ ?o }`,
	`SELECT ?o WHERE { ex:n0 !(ex:p0)+ ?o }`,
	`SELECT ?x WHERE { ?x ex:p2+ ?x }`,
	`SELECT ?s ?o WHERE { ?s ex:p0/ex:p1* ?o }`,
	`SELECT (COUNT(*) AS ?n) WHERE { ?s ex:p0 ?o }`,
	`SELECT (COUNT(*) AS ?n) WHERE { ?s ?p ex:n1 }`,
	`SELECT (COUNT(DISTINCT ?s) AS ?n) WHERE { ?s ?p ?o }`,
	`ASK { ex:n0 ex:p0 ?o }`,
	`ASK { ex:n4 ex:p3+ ex:n0 }`,
	`SELECT * WHERE { ?s ex:p0 ?o OPTIONAL { ?o ex:p1 ?z } }`,
	`SELECT * WHERE { ?s ex:p0 ?o FILTER NOT EXISTS { ?o ex:p2 ?s } }`,
	`SELECT * WHERE { ?s ex:p1 ?o MINUS { ?s ex:p0 ?o } }`,
	`SELECT ?s WHERE { ?s ex:p0 ?o } LIMIT 3`,
	`SELECT ?s ?o WHERE { ?s ex:p3 ?o } LIMIT 2 OFFSET 1`,
	`SELECT ?s (COUNT(?o) AS ?n) WHERE { ?s ex:p1 ?o } GROUP BY ?s`,
}

// TestOverlayStoreDifferential runs the same queries over an overlay graph
// built three ways — with disjointUnion (the change), with the plain
// store.UnionStore (offDisjointUnion), and with every optional interface
// hidden — and over a graph that simply holds base plus added. All four must
// return equal multisets. Seeded, random data; the added triples overlap the
// base and are sometimes all already in it (the empty over store).
func TestOverlayStoreDifferential(t *testing.T) {
	ctx := context.Background()
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	var nodes []term.Term
	for i := range 6 {
		nodes = append(nodes, ex(fmt.Sprintf("n%d", i)))
	}
	nodes = append(nodes, term.NewBNode("shared"), term.NewLiteral("v"))
	var preds []term.URIRef
	for i := range 4 {
		preds = append(preds, ex(fmt.Sprintf("p%d", i)))
	}
	random := func(r *rand.Rand) term.Triple {
		s := nodes[r.IntN(len(nodes)-1)].(term.Subject)
		return term.Triple{Subject: s, Predicate: preds[r.IntN(len(preds))], Object: nodes[r.IntN(len(nodes))]}
	}

	for seed := range uint64(60) {
		r := rand.New(rand.NewPCG(seed, 0x0e))
		rg := graph.NewGraph()
		var baseTriples []term.Triple
		for range 5 + r.IntN(30) {
			tr := random(r)
			baseTriples = append(baseTriples, tr)
			if err := rg.Add(ctx, tr.Subject, tr.Predicate, tr.Object); err != nil {
				t.Fatal(err)
			}
		}
		var added []term.Triple
		allHeld := r.IntN(4) == 0
		for range r.IntN(12) {
			if allHeld || r.IntN(3) == 0 {
				added = append(added, baseTriples[r.IntN(len(baseTriples))])
			} else {
				added = append(added, random(r))
			}
		}
		merged := graph.NewGraph()
		for _, tr := range append(slices.Clone(baseTriples), added...) {
			if err := merged.Add(ctx, tr.Subject, tr.Predicate, tr.Object); err != nil {
				t.Fatal(err)
			}
		}
		base, err := NewGraphFromRDF(ctx, rg, "")
		if err != nil {
			t.Fatal(err)
		}

		views := map[string]*graph.Graph{"merged": merged}
		for _, mode := range []string{"disjoint", "union", "plain"} {
			offDisjointUnion = mode != "disjoint"
			ov, err := base.Overlay(ctx, added)
			if err != nil {
				t.Fatal(err)
			}
			g, err := ov.rdf(ctx)
			offDisjointUnion = false
			if err != nil {
				t.Fatal(err)
			}
			if mode == "plain" {
				g = graph.NewGraph(graph.WithStore(plainStore{g.Store()}), graph.WithIdentifier(g.Identifier()))
			}
			views[mode] = g
		}
		if _, ok := views["disjoint"].Store().(*disjointUnion); !ok {
			t.Fatalf("seed %d: the overlay store is %T, not disjointUnion: the test would not test the change", seed, views["disjoint"].Store())
		}
		if _, ok := views["union"].Store().(*store.UnionStore); !ok {
			t.Fatalf("seed %d: the off switch did not take: %T", seed, views["union"].Store())
		}

		for _, q := range diffQueries {
			q = "PREFIX ex: <http://example.org/>\n" + q
			want := solutions(t, views["merged"], q)
			for _, mode := range []string{"disjoint", "union", "plain"} {
				if got := solutions(t, views[mode], q); !slices.Equal(got, want) {
					t.Fatalf("seed %d, %s, query %s:\n got  %v\n want %v", seed, mode, q, got, want)
				}
			}
		}
	}
}

// solutions renders a result as a sorted multiset of rows. LIMIT queries
// without ORDER BY may pick any rows, so for them only the count is kept.
func solutions(t *testing.T, g *graph.Graph, q string) []string {
	t.Helper()
	res, err := sparql.Query(g, q)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	if res.Type == "ASK" {
		return []string{fmt.Sprint(res.AskResult)}
	}
	if strings.Contains(q, "LIMIT") {
		return []string{fmt.Sprint(len(res.Bindings))}
	}
	rows := make([]string, 0, len(res.Bindings))
	for _, b := range res.Bindings {
		var sb strings.Builder
		for _, v := range res.Vars {
			sb.WriteString(v)
			sb.WriteByte('=')
			if x, ok := b[v]; ok && x != nil {
				sb.WriteString(term.TermKey(x))
			}
			sb.WriteByte(' ')
		}
		rows = append(rows, sb.String())
	}
	slices.Sort(rows)
	return rows
}
