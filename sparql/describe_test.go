package sparql_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

const dPrefix = "PREFIX ex: <http://example.org/>\n"

func dIRI(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }

func dGraph(t *testing.T, fn func(add func(s term.Subject, p string, o term.Term))) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	fn(func(s term.Subject, p string, o term.Term) {
		if err := g.Add(context.Background(), s, dIRI(p), o); err != nil {
			t.Fatal(err)
		}
	})
	return g
}

func describe(t *testing.T, g *rdflibgo.Graph, q string) *rdflibgo.Graph {
	t.Helper()
	res, err := sparql.Query(g, dPrefix+q)
	if err != nil {
		t.Fatal(err)
	}
	if res.Type != "DESCRIBE" || res.Graph == nil {
		t.Fatalf("result = %+v", res)
	}
	return res.Graph
}

func size(t *testing.T, g *rdflibgo.Graph) int {
	t.Helper()
	n, err := g.Len(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func has(t *testing.T, g *rdflibgo.Graph, s term.Subject, p string, o term.Term) bool {
	t.Helper()
	ok, err := g.Contains(context.Background(), s, dIRI(p), o)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestDescribeBlankNodeChain(t *testing.T) {
	b1, b2 := term.NewBNode(), term.NewBNode()
	g := dGraph(t, func(add func(term.Subject, string, term.Term)) {
		add(dIRI("a"), "name", term.NewLiteral("A"))
		add(dIRI("a"), "addr", b1)
		add(b1, "city", term.NewLiteral("X"))
		add(b1, "geo", b2)
		add(b2, "lat", term.NewLiteral("1"))
		add(dIRI("z"), "knows", dIRI("a")) // incoming: not described
		add(dIRI("a"), "friend", dIRI("b"))
		add(dIRI("b"), "name", term.NewLiteral("B")) // IRI object: not followed
	})
	d := describe(t, g, `DESCRIBE ex:a`)
	if size(t, d) != 6 {
		t.Fatalf("got %d triples, want 6", size(t, d))
	}
	if !has(t, d, b2, "lat", term.NewLiteral("1")) {
		t.Error("blank node chain not followed")
	}
	if has(t, d, dIRI("z"), "knows", dIRI("a")) || has(t, d, dIRI("b"), "name", term.NewLiteral("B")) {
		t.Error("description includes triples outside the CBD")
	}
}

func TestDescribeBlankNodeCycle(t *testing.T) {
	b1, b2 := term.NewBNode(), term.NewBNode()
	g := dGraph(t, func(add func(term.Subject, string, term.Term)) {
		add(dIRI("a"), "p", b1)
		add(b1, "p", b2)
		add(b2, "p", b1)
		add(b2, "q", term.NewLiteral("end"))
	})
	d := describe(t, g, `DESCRIBE ex:a`)
	if size(t, d) != 4 {
		t.Fatalf("got %d triples, want 4", size(t, d))
	}
}

func TestDescribeMultipleAndVariables(t *testing.T) {
	g := dGraph(t, func(add func(term.Subject, string, term.Term)) {
		add(dIRI("a"), "t", dIRI("T"))
		add(dIRI("b"), "t", dIRI("T"))
		add(dIRI("c"), "t", dIRI("U"))
		add(dIRI("a"), "n", term.NewLiteral("A"))
		add(dIRI("b"), "n", term.NewLiteral("B"))
		add(dIRI("c"), "n", term.NewLiteral("C"))
		add(dIRI("d"), "n", term.NewLiteral("D"))
	})
	cases := []struct {
		q    string
		want int
	}{
		{`DESCRIBE ex:a ex:d`, 3},                      // two IRIs
		{`DESCRIBE ?x WHERE { ?x ex:t ex:T }`, 4},      // a and b
		{`DESCRIBE ?x ex:c WHERE { ?x ex:t ex:T }`, 6}, // a, b and c
		{`DESCRIBE ?x WHERE { ?x ex:t ex:T } LIMIT 1`, 2},
		{`DESCRIBE ?n WHERE { ?x ex:n ?n }`, 0},      // literals describe nothing
		{`DESCRIBE ?x WHERE { ?x ex:t ex:none }`, 0}, // no solutions
		{`DESCRIBE ex:unknown`, 0},
		{`DESCRIBE * WHERE { ?x ex:t ex:U }`, 2}, // c; ex:U is subject of nothing
		{`DESCRIBE * WHERE { ?x ex:t ?y }`, 6},   // a, b, c
	}
	for _, c := range cases {
		t.Run(c.q, func(t *testing.T) {
			if n := size(t, describe(t, g, c.q)); n != c.want {
				t.Fatalf("got %d triples, want %d", n, c.want)
			}
		})
	}
}

func TestDescribeReifiers(t *testing.T) {
	r := term.NewBNode()
	g := dGraph(t, func(add func(term.Subject, string, term.Term)) {
		add(dIRI("a"), "p", dIRI("o"))
		add(r, "source", term.NewLiteral("census"))
	})
	tt := term.NewTripleTerm(dIRI("a"), dIRI("p"), dIRI("o"))
	if err := g.Add(context.Background(), r, term.RDFReifies, tt); err != nil {
		t.Fatal(err)
	}
	d := describe(t, g, `DESCRIBE ex:a`)
	if size(t, d) != 3 {
		t.Fatalf("got %d triples, want the statement, its reifier link and the reifier's own triple", size(t, d))
	}
}

func TestDescribeBudget(t *testing.T) {
	g := dGraph(t, func(add func(term.Subject, string, term.Term)) {
		for i := range 10 {
			add(dIRI("a"), "p", term.NewLiteral(fmt.Sprint(i)))
		}
	})
	ctx := sparql.WithMaxSolutions(context.Background(), 5)
	res, err := sparql.QueryContext(ctx, g, dPrefix+`DESCRIBE ex:a`)
	if !errors.Is(err, sparql.ErrTooManySolutions) || res != nil {
		t.Fatalf("res=%v err=%v, want ErrTooManySolutions and no result", res, err)
	}
}

func TestDescribeWithDescriber(t *testing.T) {
	g := dGraph(t, func(add func(term.Subject, string, term.Term)) {
		add(dIRI("a"), "p", term.NewLiteral("x"))
	})
	var got []rdflibgo.Subject
	ctx := sparql.WithDescriber(context.Background(), func(_ context.Context, _ *rdflibgo.Graph, rs []rdflibgo.Subject) (*rdflibgo.Graph, error) {
		got = rs
		return nil, nil
	})
	res, err := sparql.QueryContext(ctx, g, dPrefix+`DESCRIBE ex:a ex:a ?x WHERE { ?x ex:p "x" }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || size(t, res.Graph) != 0 {
		t.Fatalf("resources = %v", got)
	}
}

func TestDescribeCancel(t *testing.T) {
	// One resource with a huge blank-node tree: the CBD loop is the only work.
	g := rdflibgo.NewGraph()
	root := dIRI("root")
	for i := range 300_000 {
		b := term.NewBNode()
		g.Add(context.Background(), root, dIRI("p"), b)
		g.Add(context.Background(), b, dIRI("q"), term.NewLiteral(fmt.Sprint(i)))
	}
	runCancelled(t, func(ctx context.Context) {
		res, err := sparql.QueryContext(ctx, g, dPrefix+`DESCRIBE ex:root`)
		assertCancelled(t, res, err, context.Canceled)
	})
}
