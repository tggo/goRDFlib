package sparql

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// preboundGraph gives ex:root twelve parts ex:p0 … ex:p11. Part i has type
// ex:Kind(i mod 3), every fourth one the second type ex:Extra, every even one
// an ex:opt value. Each kind has an ex:label. ex:p1 reaches ex:v through two
// intermediate nodes (ex:a/ex:b gives two identical rows) and ex:p2 has a
// blank-node note.
func preboundGraph(t testing.TB, opts ...graph.GraphOption) *graph.Graph {
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
	for i := range 12 {
		p := ex("p%d", i)
		add(ex("root"), ex("part"), p)
		add(p, ex("type"), ex("Kind%d", i%3))
		if i%4 == 0 {
			add(p, ex("type"), ex("Extra"))
		}
		if i%2 == 0 {
			add(p, ex("opt"), term.NewLiteral(i))
		}
	}
	for k := range 3 {
		add(ex("Kind%d", k), ex("label"), term.NewLiteral(fmt.Sprintf("kind %d", k)))
	}
	add(ex("p1"), ex("a"), ex("m1"))
	add(ex("p1"), ex("a"), ex("m2"))
	add(ex("m1"), ex("b"), ex("v"))
	add(ex("m2"), ex("b"), ex("v"))
	note := term.NewBNode("note")
	add(ex("p2"), ex("note"), note)
	add(note, ex("text"), term.NewLiteral("a note"))
	return g
}

// preboundRows returns the rows of q (prefixed with ex:) as sorted strings,
// each with its variables in name order (?this left out): two results compare
// equal exactly when they hold the same rows the same number of times.
func preboundRows(t *testing.T, g *graph.Graph, q string, pre map[string]term.Term) []string {
	t.Helper()
	pq, err := Parse("PREFIX ex: <http://example.org/>\n" + q)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	res, err := EvalQuery(g, pq, pre)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
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
	return rows
}

// A group split by a FILTER or an OPTIONAL is a Join. Evaluated with ?this
// pre-bound, it must give the rows the same query gives with the value written
// into its text (the path without initial bindings), multiplicities included:
// when the left row is substituted into the right side, when the right side is
// evaluated once, and whatever the right side holds.
func TestPreBoundJoinMatchesTextualSubstitution(t *testing.T) {
	g := preboundGraph(t)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	for _, c := range []struct{ name, query string }{
		// The right side is a BGP, or a UNION or join of BGPs, that a left row
		// binds: the row is substituted into it.
		{"filter split", `SELECT ?p ?k WHERE { ?this ex:part ?p FILTER (?p != ex:p0) ?p ex:type ?k }`},
		{"optional then bgp", `SELECT ?p ?o ?k WHERE { ?this ex:part ?p OPTIONAL { ?p ex:opt ?o } ?p ex:type ?k }`},
		{"three blocks", `SELECT ?p ?k ?o WHERE { ?this ex:part ?p FILTER (?p != ex:p1) ?p ex:type ?k FILTER (?k != ex:Extra) ?p ex:opt ?o }`},
		{"union sharing a variable", `SELECT ?p ?z WHERE { ?this ex:part ?p { ?p ex:opt ?z } UNION { ?p ex:type ?z } }`},
		{"union of joined BGPs", `SELECT ?p ?v WHERE { ?this ex:part ?p { ?p ex:a ?m { ?m ex:b ?v } } UNION { ?p ex:opt ?v } }`},
		{"repeated solutions", `SELECT ?p ?v WHERE { ?this ex:part ?p FILTER (?p != ex:p0) ?p ex:a/ex:b ?v }`},
		{"blank node from the data substituted", `SELECT ?p ?s WHERE { ?this ex:part ?p . ?p ex:note ?n FILTER (true) ?n ex:text ?s }`},
		{"blank node label on both sides", `SELECT ?p ?v ?w WHERE { ?this ex:part ?p . _:n ex:b ?v FILTER (?p = ex:p3) _:n ex:b ?w }`},
		{"order, limit and offset", `SELECT ?p ?k WHERE { ?this ex:part ?p FILTER (?p != ex:p0) ?p ex:type ?k } ORDER BY ?p ?k LIMIT 4 OFFSET 2`},
		{"count", `SELECT (COUNT(*) AS ?n) WHERE { ?this ex:part ?p FILTER (?p != ex:p0) ?p ex:type ?k }`},
		// Any other right side does not see the left row and is evaluated once.
		{"union not sharing a variable", `SELECT ?p ?z WHERE { ?this ex:part ?p { ex:p0 ex:opt ?z } UNION { ex:p4 ex:opt ?z } }`},
		{"subquery", `SELECT ?p ?n WHERE { ?this ex:part ?p { SELECT (COUNT(*) AS ?n) WHERE { ?x ex:type ex:Kind0 } } }`},
		{"subquery with order, limit and offset", `SELECT ?p ?x WHERE { ?this ex:part ?p { SELECT ?x WHERE { ?x ex:type ex:Kind0 } ORDER BY ?x LIMIT 2 OFFSET 1 } }`},
		{"filter in the right group does not see the left row", `SELECT ?p ?q WHERE { ?this ex:part ?p { ?q ex:type ex:Kind1 FILTER (!BOUND(?p)) } }`},
		{"filter in a right group joined on a variable does not see the left row", `SELECT ?p ?o ?k WHERE { ?this ex:part ?p . ?p ex:opt ?o { ?p ex:type ?k FILTER (!BOUND(?o)) } }`},
		{"filter in the right group sees ?this", `SELECT ?p ?k WHERE { ?this ex:part ?p { ?p ex:type ?k FILTER (?this = ex:root) } }`},
		{"optional in the right group", `SELECT ?p ?k ?o WHERE { ?this ex:part ?p { ?p ex:type ?k OPTIONAL { ?p ex:opt ?o } } }`},
		{"minus in the right group", `SELECT ?p ?k WHERE { ?this ex:part ?p { ?p ex:type ?k MINUS { ?p ex:type ex:Extra } } }`},
		{"bind in the right group", `SELECT ?p ?one WHERE { ?this ex:part ?p { BIND (1 AS ?one) } }`},
		{"values on the right", `SELECT ?p WHERE { ?this ex:part ?p VALUES ?p { ex:p1 ex:p2 ex:nothing } }`},
	} {
		t.Run(c.name, func(t *testing.T) {
			want := preboundRows(t, g, strings.ReplaceAll(c.query, "?this", "ex:root"), nil)
			if len(want) == 0 {
				t.Fatal("the textual form has no row: the test checks nothing")
			}
			if got := preboundRows(t, g, c.query, root); !slices.Equal(got, want) {
				t.Errorf("pre-bound rows differ from textual rows:\n%v\n%v", got, want)
			}
		})
	}
}

// A right side that calls RAND, UUID, STRUUID or BNODE (or an extension
// function) may give each left row its own value, so it is still evaluated
// once per left row, as on the path without initial bindings.
func TestPreBoundJoinVolatileRightSide(t *testing.T) {
	g := preboundGraph(t)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	for _, q := range []string{
		`SELECT ?p ?u WHERE { ?this ex:part ?p { BIND (STRUUID() AS ?u) } }`,
		`SELECT ?p ?u WHERE { ?this ex:part ?p { SELECT (UUID() AS ?u) WHERE {} } }`,
		`SELECT ?p ?u WHERE { ?this ex:part ?p { ?x ex:label ?l FILTER (?l = "kind 0") BIND (rand() AS ?u) } }`,
	} {
		for _, pre := range []map[string]term.Term{root, nil} {
			text := q
			if pre == nil {
				text = strings.ReplaceAll(q, "?this", "ex:root")
			}
			pq, err := Parse("PREFIX ex: <http://example.org/>\n" + text)
			if err != nil {
				t.Fatal(err)
			}
			res, err := EvalQuery(g, pq, pre)
			if err != nil {
				t.Fatal(err)
			}
			values := map[string]bool{}
			for _, b := range res.Bindings {
				values[b["u"].N3()] = true
			}
			if len(res.Bindings) != 12 || len(values) != 12 {
				t.Errorf("pre-bound %v: %d rows, %d distinct values, want 12 and 12\n%s", pre != nil, len(res.Bindings), len(values), q)
			}
		}
	}
}

func TestCallsVolatileFunction(t *testing.T) {
	for q, want := range map[string]bool{
		`SELECT * WHERE { ?s ?p ?o }`:                                                             false,
		`SELECT * WHERE { ?s ?p ?o FILTER (?o > 1) BIND (STR(?o) AS ?x) }`:                        false,
		`SELECT * WHERE { BIND (NOW() AS ?t) }`:                                                   false,
		`SELECT * WHERE { ?s ?p ?o BIND (<http://www.w3.org/2001/XMLSchema#integer>(?o) AS ?i) }`: false,
		`SELECT * WHERE { { SELECT (COUNT(*) AS ?n) WHERE { ?s ?p ?o } } }`:                       false,
		`SELECT * WHERE { BIND (rand() AS ?r) }`:                                                  true,
		`SELECT * WHERE { BIND (STRUUID() AS ?r) }`:                                               true,
		`SELECT * WHERE { BIND (BNODE() AS ?r) }`:                                                 true,
		`SELECT * WHERE { ?s ?p ?o FILTER (?o != UUID()) }`:                                       true,
		`SELECT * WHERE { ?s ?p ?o BIND (<urn:fn>(?o) AS ?x) }`:                                   true,
		`SELECT * WHERE { { SELECT (STRUUID() AS ?u) WHERE {} } }`:                                true,
		`SELECT * WHERE { { SELECT ?s WHERE { ?s ?p ?o } ORDER BY RAND() LIMIT 1 } }`:             true,
		`SELECT * WHERE { ?s ?p ?o OPTIONAL { ?o ?q ?x FILTER (?x < RAND()) } }`:                  true,
		`SELECT * WHERE { ?s ?p ?o MINUS { ?s ?q ?x FILTER EXISTS { BIND (UUID() AS ?u) } } }`:    true,
	} {
		pq, err := Parse(q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if got := callsVolatileFunction(pq.Where); got != want {
			t.Errorf("%s: %v, want %v", q, got, want)
		}
	}
}

// EXISTS evaluates its pattern with the outer row as initial bindings, so a
// group split by a FILTER in an EXISTS body is a pre-bound join. FILTER EXISTS
// keeps the rows that the same group, written in the outer query, extends.
func TestPreBoundJoinInExistsBody(t *testing.T) {
	g := preboundGraph(t)
	for _, c := range []struct{ name, query, reference string }{
		{"exists", `SELECT ?p WHERE { ex:root ex:part ?p FILTER EXISTS { ?p ex:type ?k FILTER (?k != ex:Extra) ?k ex:label ?l } }`,
			`SELECT DISTINCT ?p WHERE { ex:root ex:part ?p . ?p ex:type ?k FILTER (?k != ex:Extra) ?k ex:label ?l }`},
		{"not exists", `SELECT ?p WHERE { ex:root ex:part ?p FILTER NOT EXISTS { ?p ex:opt ?o FILTER (?o > 4) ?p ex:type ex:Extra } }`,
			`SELECT ?p WHERE { ex:root ex:part ?p MINUS { ?p ex:opt ?o FILTER (?o > 4) ?p ex:type ex:Extra } }`},
		{"exists, union body", `SELECT ?p WHERE { ex:root ex:part ?p FILTER EXISTS { ?p ex:type ?k { ?k ex:label "kind 1" } UNION { ?p ex:a ?m } } }`,
			`SELECT DISTINCT ?p WHERE { ex:root ex:part ?p . ?p ex:type ?k { ?k ex:label "kind 1" } UNION { ?p ex:a ?m } }`},
	} {
		t.Run(c.name, func(t *testing.T) {
			want := preboundRows(t, g, c.reference, nil)
			if len(want) == 0 {
				t.Fatal("the reference has no row: the test checks nothing")
			}
			if got := preboundRows(t, g, c.query, nil); !slices.Equal(got, want) {
				t.Errorf("got\n%v\nwant\n%v", got, want)
			}
		})
	}
}

// The left row is substituted into a BGP on the right, which then reads only
// the triples it can match, rather than all of them once or once per left row;
// and a right side that does not depend on the row is evaluated once, not
// once per left row.
func TestPreBoundJoinWork(t *testing.T) {
	st := &probingStore{Store: store.NewMemoryStore()}
	g := graph.NewGraph(graph.WithStore(st))
	ctx := t.Context()
	ex := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	for i := range 3000 {
		p := ex("p%d", i)
		if i < 20 {
			g.Add(ctx, ex("root"), ex("part"), p)
		}
		g.Add(ctx, p, ex("type"), ex("Kind%d", i%3))
	}
	root := map[string]term.Term{"this": ex("root")}
	for _, c := range []struct {
		name, query string
		rows        int
	}{
		// main matches the right BGP in full (3,000 triples) per left row
		{"substituted", `SELECT ?p ?k WHERE { ?this ex:part ?p FILTER (?p != ex:p0) ?p ex:type ?k }`, 19},
		// main evaluates the subquery per left row
		{"evaluated once", `SELECT ?p ?n WHERE { ?this ex:part ?p { SELECT (COUNT(?x) AS ?n) WHERE { ?x ex:type ex:Kind0 } } }`, 20},
	} {
		t.Run(c.name, func(t *testing.T) {
			st.pulled = 0
			if rows := preboundRows(t, g, c.query, root); len(rows) != c.rows {
				t.Fatalf("%d rows, want %d", len(rows), c.rows)
			}
			// 20 parts, the planner's probes, and at most one pass over the
			// 1,000 triples the subquery matches
			if st.pulled > 2000 {
				t.Errorf("pulled %d triples: the right side was matched in full", st.pulled)
			}
		})
	}
}
