package sparql

import (
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

// SPARQL 1.1 §18.5: `?s ex:part* ?d` with both ends variables has the
// zero-length solutions of the graph's nodes only, and a VALUES/BIND/earlier
// pattern joins with it; a term the graph does not hold gives no row, in any
// order the patterns are written in and whichever way the engine evaluates the
// join (nested loops, pre-bound substitution, store pushdown). Oxigraph
// agrees; rdflib, which substitutes the value into the path, does not
// (see zero_length.go). A term in the pattern text, or an initial binding, is
// a constant and does match as a zero-length step (W3C property paths, "?
// with start being a constant on the empty dataset").
func TestZeroLengthPathOnAForeignTerm(t *testing.T) {
	g := optGraph(t)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	defer setOpts(false)
	for _, q := range []string{
		`SELECT * WHERE { VALUES ?d { "x" } ?s ex:part* ?d }`,
		`SELECT * WHERE { ?s ex:part* ?d VALUES ?d { "x" } }`,
		`SELECT * WHERE { VALUES ?d { ex:nothing } ?s ex:part* ?d }`,
		`SELECT * WHERE { VALUES ?d { "x" } ?s ex:part? ?d }`,
		`SELECT * WHERE { VALUES ?d { "x" } ?s (ex:part+)? ?d }`,
		`SELECT * WHERE { BIND ("x" AS ?d) ?s ex:part* ?d }`,
		`SELECT * WHERE { VALUES ?s { "x" } ?s ex:part* ?d }`,
		`SELECT * WHERE { VALUES ?s { "x" } VALUES ?d { "x" } ?s ex:part* ?d }`,
		`SELECT * WHERE { { SELECT ?d WHERE { VALUES ?d { "x" } } } ?s ex:part* ?d }`,
		`SELECT * WHERE { VALUES ?d { "x" } OPTIONAL { ?s ex:part* ?d } FILTER (BOUND(?s)) }`,
		`SELECT * WHERE { ?this ex:part ?m . VALUES ?d { "x" } ?s ex:part* ?d }`,
		// the foreign term is a triple term built from the row
		`SELECT * WHERE { VALUES ?b { ex:nothing } ?s ex:part* <<( ?b ex:type ex:K )>> }`,
		`SELECT * WHERE { ex:nothing ex:part* ?b . ?s ex:part* <<( ?b ex:type ex:K )>> }`,
		`SELECT * WHERE { ?b ex:part* ex:nothing . ?s ex:part* <<( ?b ex:type ex:K )>> }`,
	} {
		for _, off := range []bool{false, true} {
			setOpts(off)
			for name, pre := range map[string]map[string]term.Term{"pre-bound": root, "plain": nil} {
				if got := queryRows(t, g, q, pre); len(got) != 0 {
					t.Errorf("%s (%s, optimisations off=%v): %v, want no row", q, name, off, got)
				}
			}
		}
	}
	setOpts(false)

	// What stays a match: a node of the graph, a term written in the pattern,
	// an initial binding.
	for _, c := range []struct {
		query string
		pre   map[string]term.Term
		rows  int
	}{
		{`SELECT * WHERE { VALUES ?d { ex:p1 } ?s ex:part* ?d }`, nil, 2},                         // p1 is a node: itself and root
		{`SELECT * WHERE { ?s ex:part* "x" }`, nil, 1},                                            // a constant
		{`SELECT * WHERE { ?s ex:part* ex:nothing }`, nil, 1},                                     // a constant
		{`SELECT * WHERE { ?s ex:part* ?d }`, map[string]term.Term{"d": term.NewLiteral("x")}, 1}, // an initial binding
		{`SELECT * WHERE { VALUES ?d { "x" } ?s ex:part* "x" }`, nil, 1},                          // the path side is a constant
		{`SELECT * WHERE { VALUES ?d { ex:nothing } ?d ex:part* ex:nothing }`, nil, 1},            // a constant for the very term (found by FuzzOptimisationsAgree)
	} {
		if got := queryRows(t, g, c.query, c.pre); len(got) != c.rows {
			t.Errorf("%s: %d rows %v, want %d", c.query, len(got), got, c.rows)
		}
	}
	// pre-bound and plain agree on every one of them
	for _, q := range []string{
		`SELECT * WHERE { VALUES ?d { ex:p1 } ?s ex:part* ?d }`,
		`SELECT * WHERE { ?s ex:part* "x" }`,
	} {
		if a, b := queryRows(t, g, q, root), queryRows(t, g, q, nil); !slices.Equal(a, b) {
			t.Errorf("%s: pre-bound %v, plain %v", q, a, b)
		}
	}
}
