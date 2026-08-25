package sparql_test

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// TestLiteralEscapesRoundTrip guards the closing-delimiter scan in
// parseLiteralString. It used to look for the first bare quote, so any literal
// containing an escaped quote was silently truncated at that quote — an
// INSERT DATA of `he said "hi"` stored `he said \` and nothing reported an
// error. The W3C suites never covered it, which is why it survived.
func TestLiteralEscapesRoundTrip(t *testing.T) {
	cases := []struct {
		n3   string
		want string
	}{
		{`"he said \"hi\""`, `he said "hi"`},
		{`"\"leading"`, `"leading`},
		{`"trailing\""`, `trailing"`},
		{`"\"\""`, `""`},
		{`"back\\slash"`, `back\slash`},
		{`"trailing\\"`, `trailing\`},
		{`"tab\there"`, "tab\there"},
		{`"newline\nhere"`, "newline\nhere"},
		{`"plain"`, `plain`},
		{`""`, ``},
		{`"""long \"quoted\" text"""`, `long "quoted" text`},
		{`"""long "single" quotes"""`, `long "single" quotes`},
	}

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	for _, tc := range cases {
		t.Run(tc.n3, func(t *testing.T) {
			g := graph.NewGraph()
			ds := &sparql.Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
			update := "INSERT DATA { " + s.N3() + " " + p.N3() + " " + tc.n3 + " . }"
			if err := sparql.Update(ds, update); err != nil {
				t.Fatalf("Update(%s): %v", update, err)
			}

			var got []term.Term
			g.Triples(nil, nil, nil)(func(tr term.Triple) bool {
				got = append(got, tr.Object)
				return true
			})
			if len(got) != 1 {
				t.Fatalf("stored %d triples, want 1", len(got))
			}
			lit, ok := got[0].(term.Literal)
			if !ok {
				t.Fatalf("object is %T, want a Literal", got[0])
			}
			if lit.Lexical() != tc.want {
				t.Errorf("lexical form = %q, want %q", lit.Lexical(), tc.want)
			}

			// The value must also survive being read back through a query,
			// which is how a remote store sees it.
			res, err := sparql.Query(g, "SELECT ?o WHERE { "+s.N3()+" "+p.N3()+" ?o }")
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if len(res.Bindings) != 1 {
				t.Fatalf("query returned %d rows, want 1", len(res.Bindings))
			}
			if l, ok := res.Bindings[0]["o"].(term.Literal); !ok || l.Lexical() != tc.want {
				t.Errorf("query returned %v, want %q", res.Bindings[0]["o"], tc.want)
			}
		})
	}
}

// TestLiteralEscapesInFilter checks the same scan on the expression side: a
// truncated literal in a FILTER silently changes which rows match.
func TestLiteralEscapesInFilter(t *testing.T) {
	g := graph.NewGraph()
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, term.NewLiteral(`he said "hi"`))
	g.Add(s, p, term.NewLiteral(`he said`))

	res, err := sparql.Query(g, `SELECT ?o WHERE { ?s ?p ?o FILTER(?o = "he said \"hi\"") }`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("FILTER matched %d rows, want 1", len(res.Bindings))
	}
	if l, ok := res.Bindings[0]["o"].(term.Literal); !ok || l.Lexical() != `he said "hi"` {
		t.Errorf("FILTER matched %v, want the quoted literal", res.Bindings[0]["o"])
	}
}
