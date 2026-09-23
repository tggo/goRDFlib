package sparql_test

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
)

// datasetFixture builds two named graphs with one triple each, plus a default
// graph with a third, so that every dataset choice gives a different answer.
func datasetFixture() (*rdflibgo.Graph, map[string]*rdflibgo.Graph) {
	p := rdflibgo.NewURIRefUnsafe("http://ex/p")
	mk := func(s string) *rdflibgo.Graph {
		g := rdflibgo.NewGraph()
		g.Add(rdflibgo.NewURIRefUnsafe("http://ex/"+s), p, rdflibgo.NewLiteral(s))
		return g
	}
	return mk("d"), map[string]*rdflibgo.Graph{
		"http://ex/g1": mk("s1"),
		"http://ex/g2": mk("s2"),
	}
}

func evalWith(t *testing.T, query string, prepare func(*sparql.ParsedQuery)) (*sparql.Result, error) {
	t.Helper()
	def, named := datasetFixture()
	q, err := sparql.Parse(query)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	q.NamedGraphs = named
	if prepare != nil {
		prepare(q)
	}
	return sparql.EvalQuery(def, q, nil)
}

func subjects(t *testing.T, res *sparql.Result, v string) []string {
	t.Helper()
	var out []string
	for _, b := range res.Bindings {
		if term, ok := b[v]; ok && term != nil {
			out = append(out, term.N3())
		}
	}
	return out
}

func TestEvalDatasetClause(t *testing.T) {
	cases := []struct {
		name  string
		query string
		v     string
		want  []string
	}{
		{
			name:  "FROM replaces the graph the caller passed",
			query: `SELECT ?s FROM <http://ex/g1> WHERE { ?s ?p ?o }`,
			v:     "s",
			want:  []string{"<http://ex/s1>"},
		},
		{
			name:  "two FROM clauses merge into one default graph",
			query: `SELECT ?s FROM <http://ex/g1> FROM <http://ex/g2> WHERE { ?s ?p ?o }`,
			v:     "s",
			want:  []string{"<http://ex/s1>", "<http://ex/s2>"},
		},
		{
			name:  "the same FROM twice is the same merge",
			query: `SELECT ?s FROM <http://ex/g1> FROM <http://ex/g1> WHERE { ?s ?p ?o }`,
			v:     "s",
			want:  []string{"<http://ex/s1>"},
		},
		{
			name:  "FROM NAMED alone leaves the default graph empty",
			query: `SELECT ?s FROM NAMED <http://ex/g1> WHERE { ?s ?p ?o }`,
			v:     "s",
			want:  nil,
		},
		{
			name:  "GRAPH ranges over the FROM NAMED graphs only",
			query: `SELECT ?g FROM NAMED <http://ex/g2> WHERE { GRAPH ?g { ?s ?p ?o } }`,
			v:     "g",
			want:  []string{"<http://ex/g2>"},
		},
		{
			name:  "FROM and FROM NAMED name separate halves of the dataset",
			query: `SELECT ?s FROM <http://ex/g1> FROM NAMED <http://ex/g2> WHERE { ?s ?p ?o }`,
			v:     "s",
			want:  []string{"<http://ex/s1>"},
		},
		{
			name:  "without a dataset clause the caller's graph is used unchanged",
			query: `SELECT ?s WHERE { ?s ?p ?o }`,
			v:     "s",
			want:  []string{"<http://ex/d>"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := evalWith(t, c.query, nil)
			if err != nil {
				t.Fatalf("EvalQuery: %v", err)
			}
			got := subjects(t, res, c.v)
			if len(got) != len(c.want) {
				t.Fatalf("?%s = %v, want %v", c.v, got, c.want)
			}
			for _, w := range c.want {
				if !contains(got, w) {
					t.Errorf("?%s = %v, want it to contain %s", c.v, got, w)
				}
			}
		})
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// TestEvalDatasetClauseUnknownGraph pins the quiet half of the contract: the
// engine never fetches a graph, so an IRI the caller did not supply
// contributes an empty graph — a restriction of the dataset, as in Jena,
// RDF4J and rdflib — and never the graph the clause excluded.
func TestEvalDatasetClauseUnknownGraph(t *testing.T) {
	for _, q := range []string{
		`SELECT ?s FROM <http://ex/missing> WHERE { ?s ?p ?o }`,
		`SELECT ?s FROM NAMED <http://ex/missing> WHERE { ?s ?p ?o }`,
		`SELECT ?s FROM <http://ex/missing> WHERE { GRAPH <http://ex/missing> { ?s ?p ?o } }`,
		`SELECT ?g FROM NAMED <http://ex/missing> WHERE { GRAPH ?g { } }`,
	} {
		res, err := evalWith(t, q, nil)
		if err != nil {
			t.Errorf("%s: %v", q, err)
			continue
		}
		if len(res.Bindings) != 0 {
			t.Errorf("%s: bindings = %v, want none", q, res.Bindings)
		}
	}
}

// A graph the caller did supply is still merged when another clause names one
// it did not: a missing graph subtracts nothing.
func TestEvalDatasetClauseMixesKnownAndUnknown(t *testing.T) {
	res, err := evalWith(t, `SELECT ?s FROM <http://ex/g1> FROM <http://ex/missing> WHERE { ?s ?p ?o }`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, res, "s"); len(got) != 1 || got[0] != "<http://ex/s1>" {
		t.Errorf("?s = %v, want <http://ex/s1>", got)
	}
}

// TestEvalDatasetClauseOptOut is the documented way back to the behaviour of
// every release before this one, for a caller that handles FROM itself (the
// SPARQL Protocol endpoint does).
func TestEvalDatasetClauseOptOut(t *testing.T) {
	res, err := evalWith(t, `SELECT ?s FROM <http://ex/g1> WHERE { ?s ?p ?o }`,
		func(q *sparql.ParsedQuery) { q.DatasetClause = nil })
	if err != nil {
		t.Fatalf("EvalQuery: %v", err)
	}
	if got := subjects(t, res, "s"); len(got) != 1 || got[0] != "<http://ex/d>" {
		t.Errorf("?s = %v, want the caller's own graph", got)
	}
}

// TestEvalDatasetClauseRelativeIRI covers a base the caller supplies after
// parsing, which is what a test runner and the endpoint both do: the clause is
// resolved when the dataset is built, not only when the query declares BASE.
func TestEvalDatasetClauseRelativeIRI(t *testing.T) {
	res, err := evalWith(t, `SELECT ?s FROM <g1> WHERE { ?s ?p ?o }`,
		func(q *sparql.ParsedQuery) { q.BaseURI = "http://ex/" })
	if err != nil {
		t.Fatalf("EvalQuery: %v", err)
	}
	if got := subjects(t, res, "s"); len(got) != 1 || got[0] != "<http://ex/s1>" {
		t.Errorf("?s = %v, want <http://ex/s1>", got)
	}
}

// TestEvalDatasetClauseDoesNotMutateQuery guards the documented promise that
// EvalQuery never mutates the caller's ParsedQuery — building a dataset
// rewrites both NamedGraphs and DatasetClause, so it is the operation most
// likely to break it. A reused ParsedQuery must give the same answer twice.
func TestEvalDatasetClauseDoesNotMutateQuery(t *testing.T) {
	def, named := datasetFixture()
	q, err := sparql.Parse(`SELECT ?s FROM <http://ex/g1> WHERE { ?s ?p ?o }`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	q.NamedGraphs = named

	first, err := sparql.EvalQuery(def, q, nil)
	if err != nil {
		t.Fatalf("EvalQuery: %v", err)
	}
	if len(q.DatasetClause) != 1 || q.DatasetClause[0].IRI != "http://ex/g1" {
		t.Errorf("DatasetClause = %+v, want it untouched", q.DatasetClause)
	}
	if len(q.NamedGraphs) != 2 {
		t.Errorf("NamedGraphs has %d entries, want the caller's 2", len(q.NamedGraphs))
	}
	second, err := sparql.EvalQuery(def, q, nil)
	if err != nil {
		t.Fatalf("EvalQuery (second run): %v", err)
	}
	if len(first.Bindings) != len(second.Bindings) {
		t.Errorf("second run gave %d bindings, first gave %d", len(second.Bindings), len(first.Bindings))
	}
}
