package sparql_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

func svcGraph(t *testing.T) *rdflibgo.Graph {
	t.Helper()
	return dGraph(t, func(add func(term.Subject, string, term.Term)) {
		add(dIRI("a"), "p", term.NewLiteral("local-a"))
		add(dIRI("b"), "p", term.NewLiteral("local-b"))
		add(dIRI("a"), "ep", dIRI("e1"))
		add(dIRI("b"), "ep", dIRI("e2"))
	})
}

func TestServiceParses(t *testing.T) {
	for _, q := range []string{
		`SELECT * { SERVICE <http://e/> { ?s ?p ?o } }`,
		`SELECT * { SERVICE SILENT <http://e/> { ?s ?p ?o } }`,
		`SELECT * { ?x ?y ?z SERVICE ?e { ?s ?p ?o } . ?a ?b ?c }`,
		`PREFIX e: <http://e/> SELECT * { SERVICE e:x { { ?s ?p ?o } UNION { ?s ?q ?o } } }`,
	} {
		if _, err := sparql.Parse(q); err != nil {
			t.Errorf("%s: %v", q, err)
		}
	}
	for _, q := range []string{
		`SELECT * { SERVICE { ?s ?p ?o } }`,
		`SELECT * { SERVICE "x" { ?s ?p ?o } }`,
		`SELECT * { SERVICE <http://e/> ?s ?p ?o }`,
	} {
		if _, err := sparql.Parse(q); err == nil {
			t.Errorf("%s: accepted", q)
		}
	}
}

// Without a handler the engine makes no call, and says how to enable one.
func TestServiceIsOffByDefault(t *testing.T) {
	g := svcGraph(t)
	res, err := sparql.Query(g, dPrefix+`SELECT * { ?s ex:p ?o SERVICE <http://127.0.0.1:1/> { ?s ?p2 ?o2 } }`)
	if !errors.Is(err, sparql.ErrServiceUnsupported) || res != nil {
		t.Fatalf("res=%v err=%v, want ErrServiceUnsupported", res, err)
	}
	if !strings.Contains(err.Error(), "WithServiceHandler") {
		t.Errorf("error does not name the remedy: %v", err)
	}
	// SILENT: one solution with no bindings, joined with the rest.
	res, err = sparql.Query(g, dPrefix+`SELECT ?s ?o ?o2 { ?s ex:p ?o SERVICE SILENT <http://127.0.0.1:1/> { ?s ?p2 ?o2 } }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 2 {
		t.Fatalf("got %d rows, want the 2 local ones", len(res.Bindings))
	}
	for _, r := range res.Bindings {
		if _, ok := r["o2"]; ok {
			t.Errorf("o2 bound in %v", r)
		}
	}
	// Alone, SILENT is the single empty solution.
	res, err = sparql.Query(g, `SELECT * { SERVICE SILENT <http://127.0.0.1:1/> { ?s ?p ?o } }`)
	if err != nil || len(res.Bindings) != 1 || len(res.Bindings[0]) != 0 {
		t.Fatalf("res=%v err=%v", res, err)
	}
}

func TestServiceHandler(t *testing.T) {
	g := svcGraph(t)
	var calls []*sparql.ServiceCall
	h := func(_ context.Context, c *sparql.ServiceCall) ([]map[string]rdflibgo.Term, error) {
		calls = append(calls, c)
		switch c.Endpoint.Value() {
		case "http://example.org/e1":
			return []map[string]rdflibgo.Term{{"s": dIRI("a"), "o2": term.NewLiteral("remote-a")}, {"s": dIRI("b"), "o2": term.NewLiteral("remote-b")}}, nil
		}
		return nil, errors.New("boom")
	}
	ctx := sparql.WithServiceHandler(context.Background(), h)

	res, err := sparql.QueryContext(ctx, g, dPrefix+`SELECT ?s ?o ?o2 { ?s ex:p ?o SERVICE ex:e1 { ?s ex:q ?o2 } } ORDER BY ?s`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 2 || res.Bindings[0]["o2"].String() != "remote-a" || res.Bindings[1]["o2"].String() != "remote-b" {
		t.Fatalf("bindings = %v", res.Bindings)
	}
	c := calls[0]
	for _, want := range []string{"PREFIX ex: <http://example.org/>", "SELECT * WHERE ", "?s ex:q ?o2"} {
		if !strings.Contains(c.Query, want) {
			t.Errorf("Query %q lacks %q", c.Query, want)
		}
	}
	if _, err := sparql.Parse(c.Query); err != nil {
		t.Errorf("the query handed to the service does not parse: %v\n%s", err, c.Query)
	}

	// A failing service fails the query; SILENT ignores the failure.
	_, err = sparql.QueryContext(ctx, g, dPrefix+`SELECT * { SERVICE ex:down { ?s ?p ?o } }`)
	if err == nil || !strings.Contains(err.Error(), "boom") || !strings.Contains(err.Error(), "http://example.org/down") {
		t.Fatalf("err = %v", err)
	}
	res, err = sparql.QueryContext(ctx, g, dPrefix+`SELECT * { SERVICE SILENT ex:down { ?s ?p ?o } }`)
	if err != nil || len(res.Bindings) != 1 {
		t.Fatalf("res=%v err=%v", res, err)
	}
}

// SERVICE ?e is asked once per binding of ?e, and ?e is bound in the answer.
func TestServiceVariableEndpoint(t *testing.T) {
	g := svcGraph(t)
	h := func(_ context.Context, c *sparql.ServiceCall) ([]map[string]rdflibgo.Term, error) {
		return []map[string]rdflibgo.Term{{"v": term.NewLiteral("from " + c.Endpoint.Value())}}, nil
	}
	ctx := sparql.WithServiceHandler(context.Background(), h)
	res, err := sparql.QueryContext(ctx, g, dPrefix+`SELECT ?s ?e ?v { ?s ex:ep ?e SERVICE ?e { ?x ?y ?v } } ORDER BY ?s`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 2 ||
		res.Bindings[0]["v"].String() != "from http://example.org/e1" ||
		res.Bindings[1]["v"].String() != "from http://example.org/e2" {
		t.Fatalf("bindings = %v", res.Bindings)
	}

	// Pre-bound path (initial bindings) takes the same route.
	pq, err := sparql.Parse(dPrefix + `SELECT ?s ?v { ?s ex:ep ?e SERVICE ?e { ?x ?y ?v } } ORDER BY ?s`)
	if err != nil {
		t.Fatal(err)
	}
	res, err = sparql.EvalQueryContext(ctx, g, pq, map[string]rdflibgo.Term{"s": dIRI("b")})
	if err != nil || len(res.Bindings) != 1 || res.Bindings[0]["v"].String() != "from http://example.org/e2" {
		t.Fatalf("res=%v err=%v", res, err)
	}

	// Unbound: an error, or the empty solution under SILENT.
	_, err = sparql.QueryContext(ctx, g, `SELECT * { SERVICE ?e { ?x ?y ?v } }`)
	if !errors.Is(err, sparql.ErrServiceEndpoint) {
		t.Fatalf("err = %v, want ErrServiceEndpoint", err)
	}
	res, err = sparql.QueryContext(ctx, g, `SELECT * { SERVICE SILENT ?e { ?x ?y ?v } }`)
	if err != nil || len(res.Bindings) != 1 {
		t.Fatalf("res=%v err=%v", res, err)
	}
	// Bound to a literal: not an endpoint.
	_, err = sparql.QueryContext(ctx, g, dPrefix+`SELECT * { ?s ex:p ?e SERVICE ?e { ?x ?y ?v } }`)
	if !errors.Is(err, sparql.ErrServiceEndpoint) {
		t.Fatalf("err = %v, want ErrServiceEndpoint", err)
	}
}

// SILENT ignores a failing service, never a query that was told to stop.
func TestServiceSilentDoesNotHideCancellation(t *testing.T) {
	g := svcGraph(t)
	ctx, cancel := context.WithCancel(context.Background())
	h := func(ctx context.Context, c *sparql.ServiceCall) ([]map[string]rdflibgo.Term, error) {
		cancel()
		return nil, ctx.Err()
	}
	ctx = sparql.WithServiceHandler(ctx, h)
	res, err := sparql.QueryContext(ctx, g, `SELECT * { SERVICE SILENT <http://e/> { ?s ?p ?o } }`)
	if !errors.Is(err, sparql.ErrQueryCancelled) || res != nil {
		t.Fatalf("res=%v err=%v", res, err)
	}
}

func TestServiceRespectsSolutionBudget(t *testing.T) {
	g := svcGraph(t)
	h := func(_ context.Context, c *sparql.ServiceCall) ([]map[string]rdflibgo.Term, error) {
		rows := make([]map[string]rdflibgo.Term, 100)
		for i := range rows {
			rows[i] = map[string]rdflibgo.Term{"v": term.NewLiteral(i)}
		}
		return rows, nil
	}
	ctx := sparql.WithMaxSolutions(sparql.WithServiceHandler(context.Background(), h), 10)
	_, err := sparql.QueryContext(ctx, g, `SELECT * { SERVICE SILENT <http://e/> { ?s ?p ?v } }`)
	if !errors.Is(err, sparql.ErrTooManySolutions) {
		t.Fatalf("err = %v, want ErrTooManySolutions even under SILENT", err)
	}
}

func TestServiceSelectStar(t *testing.T) {
	q, err := sparql.Parse(`SELECT * { ?a ?b ?c SERVICE ?e { ?x ?y ?z } }`)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.PreBindRestrictions("a"); err == nil || !errors.Is(err, sparql.ErrPreBindRestriction) {
		t.Errorf("SERVICE passed the SHACL pre-binding restrictions: %v", err)
	}
	g := svcGraph(t)
	h := func(_ context.Context, c *sparql.ServiceCall) ([]map[string]rdflibgo.Term, error) {
		return []map[string]rdflibgo.Term{{"x": dIRI("x")}}, nil
	}
	ctx := sparql.WithServiceHandler(context.Background(), h)
	res, err := sparql.EvalQueryContext(ctx, g, mustParse(t, `SELECT * { SERVICE SILENT <http://e/> { ?x ?y ?z } }`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Vars, ",") != "x,y,z" {
		t.Errorf("SELECT * vars = %v, want the clause's variables", res.Vars)
	}
}

func mustParse(t *testing.T, q string) *sparql.ParsedQuery {
	t.Helper()
	pq, err := sparql.Parse(q)
	if err != nil {
		t.Fatal(err)
	}
	return pq
}
