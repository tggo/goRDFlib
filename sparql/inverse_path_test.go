package sparql_test

import (
	"sort"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// TestInversePathWithModifier guards PathEltOrInverse. The '^' branch used to
// return before looking for a modifier, so `^ex:p+` failed to parse at all and
// only the parenthesized `(^ex:p)+` worked. No W3C test covers the unbracketed
// form.
func TestInversePathWithModifier(t *testing.T) {
	const prefix = `PREFIX ex: <http://example.org/> `

	for _, q := range []string{
		`SELECT ?x WHERE { ex:d ^ex:knows+ ?x }`,
		`SELECT ?x WHERE { ex:d ^ex:knows* ?x }`,
		`SELECT ?x WHERE { ex:d ^ex:knows? ?x }`,
		`SELECT ?x WHERE { ex:d ^<http://example.org/knows>+ ?x }`,
		`SELECT ?x ?y WHERE { ?x ^ex:knows* ?y }`,
		`SELECT ?x WHERE { ex:a ex:knows/^ex:knows+ ?x }`,
		`SELECT ?x WHERE { ex:a (ex:knows|^ex:knows)+ ?x }`,
	} {
		if _, err := sparql.Parse(prefix + q); err != nil {
			t.Errorf("%s: %v", q, err)
		}
	}
}

// TestInversePathWithModifierResults checks that the unbracketed form means the
// same thing as the bracketed one, since the parser normalizes `^p+` to
// `(^p)+` rather than `^(p+)`.
func TestInversePathWithModifierResults(t *testing.T) {
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }

	g := graph.NewGraph()
	g.Add(ex("a"), ex("knows"), ex("b"))
	g.Add(ex("b"), ex("knows"), ex("c"))
	g.Add(ex("c"), ex("knows"), ex("d"))

	run := func(t *testing.T, query string) []string {
		t.Helper()
		res, err := sparql.Query(g, `PREFIX ex: <http://example.org/> `+query)
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		out := make([]string, 0, len(res.Bindings))
		for _, b := range res.Bindings {
			out = append(out, b["x"].N3())
		}
		sort.Strings(out)
		return out
	}

	cases := []struct{ unbracketed, bracketed string }{
		{`SELECT ?x WHERE { ex:d ^ex:knows+ ?x }`, `SELECT ?x WHERE { ex:d (^ex:knows)+ ?x }`},
		{`SELECT ?x WHERE { ex:d ^ex:knows* ?x }`, `SELECT ?x WHERE { ex:d (^ex:knows)* ?x }`},
		{`SELECT ?x WHERE { ex:d ^ex:knows? ?x }`, `SELECT ?x WHERE { ex:d (^ex:knows)? ?x }`},
	}
	for _, tc := range cases {
		got, want := run(t, tc.unbracketed), run(t, tc.bracketed)
		if len(got) != len(want) {
			t.Errorf("%s\n got  %v\n want %v (from %s)", tc.unbracketed, got, want, tc.bracketed)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s\n got  %v\n want %v (from %s)", tc.unbracketed, got, want, tc.bracketed)
				break
			}
		}
	}

	// Pin the actual answer too, so a change that makes both forms equally
	// wrong still fails.
	if got := run(t, `SELECT ?x WHERE { ex:d ^ex:knows+ ?x }`); len(got) != 3 {
		t.Errorf("ex:d ^ex:knows+ ?x = %v, want a, b and c", got)
	}
}
