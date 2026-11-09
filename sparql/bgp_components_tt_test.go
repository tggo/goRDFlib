package sparql

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// Variables and blank node labels inside triple terms join the patterns that
// hold them (or, when unshared, leave them in their own groups), and the
// answer equals the same groups written as separate nested groups, which no
// BGP optimisation looks into.
func TestDisjointBGPGroupsWithTripleTerms(t *testing.T) {
	g := graph.NewGraph()
	if err := turtle.Parse(t.Context(), g, strings.NewReader(`@prefix ex: <http://example.org/> .
ex:a1 ex:pt <<( ex:s ex:q 1 )>> . ex:a2 ex:pt <<( ex:s2 ex:q 1 )>> . ex:a3 ex:pt <<( ex:s ex:q 2 )>> .
ex:c1 ex:rt <<( ex:s ex:q 1 )>> . ex:c2 ex:rt <<( ex:s3 ex:q 1 )>> . ex:c3 ex:rt <<( ex:s2 ex:q 2 )>> .
ex:n1 ex:nest <<( ex:s ex:q <<( ex:z ex:q 1 )>> )>> . ex:n2 ex:nest <<( ex:s ex:q <<( ex:y ex:q 1 )>> )>> .
ex:z ex:r ex:w1 . ex:y ex:r ex:w2 . ex:s ex:r ex:w3 . ex:s2 ex:r ex:w4 .
`)); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, query, reference string }{
		{"variable shared by two triple terms",
			`SELECT * WHERE { ?a ex:pt <<( ?x ex:q 1 )>> . ?c ex:rt <<( ?x ex:q 1 )>> }`,
			`SELECT * WHERE { { ?a ex:pt <<( ?x ex:q 1 )>> FILTER (true) } { ?c ex:rt <<( ?x ex:q 1 )>> FILTER (true) } }`},
		{"variable shared with a plain position",
			`SELECT * WHERE { ?a ex:pt <<( ?x ex:q 1 )>> . ?x ex:r ?w }`,
			`SELECT * WHERE { { ?a ex:pt <<( ?x ex:q 1 )>> FILTER (true) } { ?x ex:r ?w FILTER (true) } }`},
		{"variable in a nested triple term",
			`SELECT * WHERE { ?n ex:nest <<( ex:s ex:q <<( ?z ex:q 1 )>> )>> . ?z ex:r ?w }`,
			`SELECT * WHERE { { ?n ex:nest <<( ex:s ex:q <<( ?z ex:q 1 )>> )>> FILTER (true) } { ?z ex:r ?w FILTER (true) } }`},
		{"unshared variables in triple terms",
			`SELECT * WHERE { ?a ex:pt <<( ?x ex:q 1 )>> . ?c ex:rt <<( ?y ex:q 1 )>> }`,
			`SELECT * WHERE { { ?a ex:pt <<( ?x ex:q 1 )>> FILTER (true) } { ?c ex:rt <<( ?y ex:q 1 )>> FILTER (true) } }`},
		{"label shared by two triple terms",
			// a label cannot span two groups (§4.1.4), so the reference
			// joins them on a variable, which is what the label is
			`SELECT ?a ?c WHERE { ?a ex:pt <<( _:b ex:q 1 )>> . ?c ex:rt <<( _:b ex:q 1 )>> }`,
			`SELECT ?a ?c WHERE { { ?a ex:pt <<( ?b ex:q 1 )>> FILTER (true) } { ?c ex:rt <<( ?b ex:q 1 )>> FILTER (true) } }`},
	} {
		t.Run(c.name, func(t *testing.T) {
			want := queryRows(t, g, c.reference, nil)
			if len(want) == 0 {
				t.Fatal("the reference has no row: the test checks nothing")
			}
			if got := queryRows(t, g, c.query, nil); !slices.Equal(got, want) {
				t.Errorf("got\n%v\nwant\n%v", got, want)
			}
		})
	}
}

// With a limit the groups are collected only as far as the first limit rows
// of the product can reach, not in full.
func TestDisjointBGPGroupsWithLimitAreNotMaterialized(t *testing.T) {
	st := &probingStore{Store: store.NewMemoryStore()}
	g := graph.NewGraph(graph.WithStore(st))
	ctx := t.Context()
	ex := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	for i := range 500 {
		g.Add(ctx, ex("a%d", i), ex("type"), ex("ClassA"))
		g.Add(ctx, ex("b%d", i), ex("val"), term.NewLiteral(i))
	}
	pq, err := Parse(exPrefix + `SELECT * WHERE { ?a ex:type ex:ClassA . ?b ex:val ?v }`)
	if err != nil {
		t.Fatal(err)
	}
	triples := pq.Where.(*BGP).Triples
	prefixes := pq.Prefixes
	if bgpComponents(triples, nil) == nil {
		t.Fatal("not split: the test checks nothing")
	}
	full := evalBGPInOrder(nil, g, triples, map[string]term.Term{}, prefixes, 0)
	if len(full) != 500*500 {
		t.Fatalf("%d rows, want 250000", len(full))
	}
	st.pulled = 0
	rows := evalBGPInOrder(nil, g, triples, map[string]term.Term{}, prefixes, 5)
	if len(rows) != 5 {
		t.Fatalf("%d rows, want 5", len(rows))
	}
	if st.pulled > 50 {
		t.Errorf("pulled %d triples for 5 rows: the groups were collected in full", st.pulled)
	}
	// every row is a row of the full product
	all := renderRows(full)
	for _, r := range renderRows(rows) {
		if !slices.Contains(all, r) {
			t.Errorf("row %q is not a solution", r)
		}
	}
	// an empty group still empties the answer under a limit
	pq, err = Parse(exPrefix + `SELECT * WHERE { ?a ex:type ex:ClassA . ?b ex:val ex:Missing }`)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(evalBGPInOrder(nil, g, pq.Where.(*BGP).Triples, map[string]term.Term{}, pq.Prefixes, 5)); n != 0 {
		t.Errorf("%d rows from a BGP with an empty group", n)
	}
}
