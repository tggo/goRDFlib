package sparql_test

import (
	"testing"

	"github.com/tggo/goRDFlib/sparql"
)

// TestParseDatasetClause checks that FROM and FROM NAMED are recorded on
// ParsedQuery in query order (issue #37); the parser used to consume and
// discard them, so a query declaring a dataset was indistinguishable from one
// that did not.
func TestParseDatasetClause(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  []sparql.DatasetClause
	}{
		{
			name:  "none",
			query: `SELECT * WHERE { ?s ?p ?o }`,
		},
		{
			name:  "from and from named in order",
			query: `SELECT * FROM <http://a> FROM NAMED <http://b> WHERE { ?s ?p ?o }`,
			want: []sparql.DatasetClause{
				{IRI: "http://a"},
				{IRI: "http://b", Named: true},
			},
		},
		{
			name:  "prefixed name is expanded",
			query: `PREFIX ex: <http://ex/> SELECT * FROM ex:g WHERE { ?s ?p ?o }`,
			want:  []sparql.DatasetClause{{IRI: "http://ex/g"}},
		},
		{
			name:  "empty prefix",
			query: `PREFIX : <http://ex/> SELECT * FROM :g WHERE { ?s ?p ?o }`,
			want:  []sparql.DatasetClause{{IRI: "http://ex/g"}},
		},
		{
			name:  "relative iri against BASE",
			query: `BASE <http://base/dir/> SELECT * FROM <g> FROM NAMED <../n> WHERE { ?s ?p ?o }`,
			want: []sparql.DatasetClause{
				{IRI: "http://base/dir/g"},
				{IRI: "http://base/n", Named: true},
			},
		},
		{
			name:  "relative iri without BASE is kept as written",
			query: `SELECT * FROM <g> WHERE { ?s ?p ?o }`,
			want:  []sparql.DatasetClause{{IRI: "g"}},
		},
		{
			name:  "undeclared prefix is kept as written",
			query: `SELECT * FROM undeclared:g WHERE { ?s ?p ?o }`,
			want:  []sparql.DatasetClause{{IRI: "undeclared:g"}},
		},
		{
			name:  "lowercase keywords",
			query: `ask from named <http://b> { ?s ?p ?o }`,
			want:  []sparql.DatasetClause{{IRI: "http://b", Named: true}},
		},
		{
			name:  "construct where shorthand",
			query: `CONSTRUCT FROM <http://a> WHERE { ?s ?p ?o }`,
			want:  []sparql.DatasetClause{{IRI: "http://a"}},
		},
		{
			name:  "construct template",
			query: `CONSTRUCT { ?s ?p ?o } FROM NAMED <http://b> WHERE { ?s ?p ?o }`,
			want:  []sparql.DatasetClause{{IRI: "http://b", Named: true}},
		},
		{
			name:  "FROM inside a subquery is not a dataset clause of the outer query",
			query: `SELECT * WHERE { { SELECT * WHERE { ?s ?p ?o } } }`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q, err := sparql.Parse(c.query)
			if err != nil {
				t.Fatalf("Parse(%q): %v", c.query, err)
			}
			if len(q.DatasetClause) != len(c.want) {
				t.Fatalf("DatasetClause = %+v, want %+v", q.DatasetClause, c.want)
			}
			for i, w := range c.want {
				if q.DatasetClause[i] != w {
					t.Errorf("DatasetClause[%d] = %+v, want %+v", i, q.DatasetClause[i], w)
				}
			}
		})
	}
}
