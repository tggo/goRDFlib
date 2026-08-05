package sparql

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/turtle"
)

// tripleFn is a query-scoped stand-in for doubleFn: same shape, different
// factor, so a test can tell which of the two answered a call.
func tripleFn(args []rdflibgo.Term) (rdflibgo.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("triple: want 1 argument, got %d", len(args))
	}
	lit, ok := args[0].(rdflibgo.Literal)
	if !ok {
		return nil, fmt.Errorf("triple: want a literal, got %T", args[0])
	}
	n, err := strconv.ParseFloat(lit.Lexical(), 64)
	if err != nil {
		return nil, fmt.Errorf("triple: %w", err)
	}
	return rdflibgo.NewLiteral(n * 3), nil
}

// bindQuery parses q and binds funcs to it, failing the test if either step
// does not behave. It returns the query and the number of call sites bound.
func bindQuery(t *testing.T, q string, funcs map[string]Function) (*ParsedQuery, int) {
	t.Helper()
	pq, err := Parse(q)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return pq, pq.BindFunctions(funcs)
}

// values runs pq and returns the ?v column as lexical strings, sorted by the
// query's own ORDER BY where one is present.
func values(t *testing.T, pq *ParsedQuery, v string) []string {
	t.Helper()
	res, err := EvalQuery(extFuncGraph(t), pq, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	var out []string
	for _, row := range res.Bindings {
		term, ok := row[v]
		if !ok {
			out = append(out, "<unbound>")
			continue
		}
		out = append(out, term.String())
	}
	return out
}

func TestBindFunctions_CallsBoundFunction(t *testing.T) {
	pq, n := bindQuery(t, `
		PREFIX ex: <http://example.org/>
		SELECT ?v WHERE { ex:a ex:value ?x . BIND(ex:scale(?x) AS ?v) }
	`, map[string]Function{"http://example.org/scale": tripleFn})

	if n != 1 {
		t.Fatalf("bound %d call sites, want 1", n)
	}
	got := values(t, pq, "v")
	if len(got) != 1 || !strings.HasPrefix(got[0], "63") {
		t.Errorf("got %v, want one value of 63 (21*3)", got)
	}
}

// The IRI is matched, not the surface syntax, so the <iri>(...) form and a
// prefixed name resolving to the same IRI must both bind.
func TestBindFunctions_MatchesIRINotSyntax(t *testing.T) {
	for name, call := range map[string]string{
		"prefixed name":  "ex:scale(?x)",
		"angle brackets": "<http://example.org/scale>(?x)",
	} {
		t.Run(name, func(t *testing.T) {
			pq, n := bindQuery(t, `
				PREFIX ex: <http://example.org/>
				SELECT ?v WHERE { ex:a ex:value ?x . BIND(`+call+` AS ?v) }
			`, map[string]Function{"http://example.org/scale": tripleFn})

			if n != 1 {
				t.Fatalf("bound %d call sites, want 1", n)
			}
			if got := values(t, pq, "v"); len(got) != 1 || !strings.HasPrefix(got[0], "63") {
				t.Errorf("got %v, want 63", got)
			}
		})
	}
}

// A query-scoped binding must win over a global registration of the same IRI,
// otherwise a caller's own vocabulary could be silently overridden by whatever
// the process happens to have registered.
func TestBindFunctions_OverridesGlobalRegistration(t *testing.T) {
	const iri = "http://example.org/queryscoped-override"
	if err := RegisterFunction(iri, doubleFn); err != nil {
		t.Fatalf("RegisterFunction: %v", err)
	}
	defer UnregisterFunction(iri)

	q := `
		PREFIX ex: <http://example.org/>
		SELECT ?v WHERE { ex:a ex:value ?x . BIND(<` + iri + `>(?x) AS ?v) }
	`

	global, err := Parse(q)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := values(t, global, "v"); len(got) != 1 || !strings.HasPrefix(got[0], "42") {
		t.Fatalf("unbound query got %v, want the global 42 (21*2)", got)
	}

	scoped, n := bindQuery(t, q, map[string]Function{iri: tripleFn})
	if n != 1 {
		t.Fatalf("bound %d call sites, want 1", n)
	}
	if got := values(t, scoped, "v"); len(got) != 1 || !strings.HasPrefix(got[0], "63") {
		t.Errorf("bound query got %v, want the scoped 63 (21*3)", got)
	}
}

// An IRI that is not in the table keeps falling through to the global registry.
func TestBindFunctions_UnboundIRIFallsThroughToRegistry(t *testing.T) {
	const iri = "http://example.org/queryscoped-fallthrough"
	if err := RegisterFunction(iri, doubleFn); err != nil {
		t.Fatalf("RegisterFunction: %v", err)
	}
	defer UnregisterFunction(iri)

	pq, n := bindQuery(t, `
		PREFIX ex: <http://example.org/>
		SELECT ?v WHERE { ex:a ex:value ?x . BIND(<`+iri+`>(?x) AS ?v) }
	`, map[string]Function{"http://example.org/never-called": tripleFn})

	if n != 0 {
		t.Fatalf("bound %d call sites, want 0", n)
	}
	if got := values(t, pq, "v"); len(got) != 1 || !strings.HasPrefix(got[0], "42") {
		t.Errorf("got %v, want the global 42", got)
	}
}

// The return count is what tells a caller a typo'd IRI from a function that is
// simply never called, so it must be exact rather than merely non-zero.
func TestBindFunctions_CountsEveryCallSite(t *testing.T) {
	pq, n := bindQuery(t, `
		PREFIX ex: <http://example.org/>
		SELECT ?a ?b WHERE {
			ex:a ex:value ?x .
			BIND(ex:scale(?x) AS ?a)
			BIND(ex:scale(ex:scale(?x)) AS ?b)
		}
	`, map[string]Function{"http://example.org/scale": tripleFn})

	if n != 3 {
		t.Errorf("bound %d call sites, want 3", n)
	}
	// Nesting must actually evaluate: 21*3*3.
	if got := values(t, pq, "b"); len(got) != 1 || !strings.HasPrefix(got[0], "189") {
		t.Errorf("nested call got %v, want 189", got)
	}
}

// Every expression position the walker claims to cover, exercised in one query.
func TestBindFunctions_ReachesEveryExpressionPosition(t *testing.T) {
	cases := map[string]string{
		"filter":     `SELECT ?x WHERE { ?s ex:value ?x . FILTER(ex:scale(?x) > 10) }`,
		"bind":       `SELECT ?x WHERE { ?s ex:value ?x . BIND(ex:scale(?x) AS ?v) }`,
		"projection": `SELECT (ex:scale(?x) AS ?v) WHERE { ?s ex:value ?x }`,
		"order by":   `SELECT ?x WHERE { ?s ex:value ?x } ORDER BY ex:scale(?x)`,
		"group by":   `SELECT (COUNT(?s) AS ?c) WHERE { ?s ex:value ?x } GROUP BY ex:scale(?x)`,
		"having":     `SELECT ?x (COUNT(?s) AS ?c) WHERE { ?s ex:value ?x } GROUP BY ?x HAVING (ex:scale(?x) > 10)`,
		"optional":   `SELECT ?x WHERE { ?s ex:value ?x OPTIONAL { ?s ex:name ?n FILTER(ex:scale(?x) > 10) } }`,
		"union":      `SELECT ?x WHERE { { ?s ex:value ?x FILTER(ex:scale(?x) > 10) } UNION { ?s ex:name ?x } }`,
		"minus":      `SELECT ?x WHERE { ?s ex:value ?x MINUS { ?s ex:value ?x FILTER(ex:scale(?x) > 10) } }`,
		"exists":     `SELECT ?x WHERE { ?s ex:value ?x FILTER EXISTS { ?s ex:value ?y FILTER(ex:scale(?y) > 10) } }`,
		"graph":      `SELECT ?x WHERE { GRAPH ?g { ?s ex:value ?x FILTER(ex:scale(?x) > 10) } }`,
		"subquery":   `SELECT ?x WHERE { { SELECT ?x WHERE { ?s ex:value ?x FILTER(ex:scale(?x) > 10) } } }`,
		"nested arg": `SELECT ?x WHERE { ?s ex:value ?x FILTER(ABS(ex:scale(?x)) > 10) }`,
	}
	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			_, n := bindQuery(t, "PREFIX ex: <http://example.org/>\n"+query,
				map[string]Function{"http://example.org/scale": tripleFn})
			if n != 1 {
				t.Errorf("bound %d call sites, want 1 — the walker misses this position", n)
			}
		})
	}
}

func TestBindFunctions_NoOpCases(t *testing.T) {
	pq, err := Parse(`PREFIX ex: <http://example.org/> SELECT ?x WHERE { ?s ex:value ?x }`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n := pq.BindFunctions(nil); n != 0 {
		t.Errorf("nil table bound %d, want 0", n)
	}
	if n := pq.BindFunctions(map[string]Function{}); n != 0 {
		t.Errorf("empty table bound %d, want 0", n)
	}
	var nilQuery *ParsedQuery
	if n := nilQuery.BindFunctions(map[string]Function{"http://example.org/scale": tripleFn}); n != 0 {
		t.Errorf("nil query bound %d, want 0", n)
	}
}

// A hand-assembled cyclic sub-query graph must not hang the walker. The parser
// never builds one, but ParsedQuery is an exported type a caller can construct.
func TestBindFunctions_CyclicSubqueryTerminates(t *testing.T) {
	inner := &ParsedQuery{Type: "SELECT"}
	inner.Where = &SubqueryPattern{Query: inner}
	done := make(chan int, 1)
	go func() { done <- inner.BindFunctions(map[string]Function{"http://example.org/scale": tripleFn}) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("BindFunctions did not terminate on a cyclic sub-query")
	}
}

// A bound query is fixed before evaluation, so evaluating it concurrently must
// be race-free. Run with -race for this to mean anything.
func TestBindFunctions_BoundQueryIsConcurrentlyEvaluable(t *testing.T) {
	pq, n := bindQuery(t, `
		PREFIX ex: <http://example.org/>
		SELECT ?v WHERE { ex:a ex:value ?x . BIND(ex:scale(?x) AS ?v) }
	`, map[string]Function{"http://example.org/scale": tripleFn})
	if n != 1 {
		t.Fatalf("bound %d call sites, want 1", n)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g := rdflibgo.NewGraph()
			if err := turtle.Parse(g, strings.NewReader(extFuncData)); err != nil {
				t.Error(err)
				return
			}
			if _, err := EvalQuery(g, pq, nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
