package sparql

import (
	"sort"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/turtle"
)

// Guards for the rdf-tests changes of #71: each case fails without its fix.

// Codepoint escapes (SPARQL 1.2 §19.2) are recognised inside IRIREF and string
// literals only; the character one produces is data and never an escape's
// backslash or a literal's closing quote.
func TestCodepointEscapesOnlyInIRIAndString(t *testing.T) {
	// § stands for a backslash, so that this file never holds a \u escape
	// the Go compiler or an editor could decode.
	bs := func(s string) string { return strings.ReplaceAll(s, "§", "\\") }
	for name, q := range map[string]string{
		"whole query":      bs(`§u0041§u0053§u004B§u0020§u007B§u007D`),
		"in a local name":  bs(`PREFIX ns: <http://e/> SELECT * { ?x ns:a§u0062c ?o }`),
		"in a prefix":      bs(`SELECT * { ?x n§u0073:abc ?o }`),
		"in a variable":    bs(`SELECT * { ?a§u0062c <http://e/p> ?o }`),
		"surrogate in IRI": bs(`SELECT * { ?s ?p <http://e/§uD83C§uDCA1> }`),
		"truncated in IRI": bs(`SELECT * { ?s ?p <http://e/§u00> }`),
		"space in IRI":     bs(`SELECT * { ?s ?p <http://e/a§u0020b> }`),
		"surrogate string": bs(`SELECT * { ?s ?p "§uD83C" }`),
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			if _, err := Parse(q); err == nil {
				t.Errorf("parsed %q", q)
			}
		})
	}

	for name, c := range map[string]struct{ q, want string }{
		"quote is data":   {`SELECT ?v { VALUES ?v { "a§u0022b" } }`, `a"b`},
		"backslash data":  {`SELECT ?v { VALUES ?v { "§u005Cn" } }`, `§n`},
		"backslash twice": {`SELECT ?v { VALUES ?v { "§§§u005C" } }`, `§§`},
		"escaped escape":  {`SELECT ?v { VALUES ?v { "§§u005C" } }`, `§u005C`},
		"astral":          {`SELECT ?v { VALUES ?v { "§U0001F47E" } }`, "\U0001F47E"},
	} {
		t.Run("accepts "+name, func(t *testing.T) {
			pq, err := Parse(bs(c.q))
			if err != nil {
				t.Fatal(err)
			}
			v := pq.Where.(*ValuesPattern).Values[0][0].(interface{ Lexical() string }).Lexical()
			if want := bs(c.want); v != want {
				t.Errorf("value %q, want %q", v, want)
			}
		})
	}
	pq, err := Parse(bs(`SELECT * { ?s ?p <http://e/§u0061§U00000062> }`))
	if err != nil {
		t.Fatal(err)
	}
	if got := pq.Where.(*BGP).Triples[0].Object; got != "<http://e/ab>" {
		t.Errorf("IRI %q, want <http://e/ab>", got)
	}
}

func TestGroupByScope(t *testing.T) {
	for name, c := range map[string]struct {
		q  string
		ok bool
	}{
		"alias of a non-key pattern variable": {`SELECT (123 AS ?z) WHERE { ?s <http://e/p> ?z } GROUP BY ?s`, true},
		"alias of a key":                      {`SELECT (123 AS ?z) WHERE { <http://e/s> <http://e/p> ?z } GROUP BY ?z`, false},
		"alias of an unbound key":             {`SELECT (123 AS ?z) WHERE { } GROUP BY ?z`, false},
		"alias of a key alias":                {`SELECT (123 AS ?z) WHERE { } GROUP BY (1 AS ?z)`, false},
		"no group by keeps the pattern scope": {`SELECT (COUNT(*) AS ?z) WHERE { ?s <http://e/p> ?z }`, false},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(c.q)
			if (err == nil) != c.ok {
				t.Errorf("err = %v, want ok=%v", err, c.ok)
			}
		})
	}
}

func TestTrailingValuesAfterGroupBy(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph()
	if err := turtle.Parse(ctx, g, strings.NewReader(`@prefix : <http://e/> .
:a :p 1, 2 . :b :p 3 . :c :p 4 .`)); err != nil {
		t.Fatal(err)
	}
	rows := func(q string) []string {
		t.Helper()
		r, err := QueryContext(ctx, g, q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		var out []string
		for _, b := range r.Bindings {
			var parts []string
			for _, v := range r.Vars {
				if x, ok := b[v]; ok {
					parts = append(parts, "?"+v+"="+x.N3())
				}
			}
			out = append(out, strings.Join(parts, " "))
		}
		sort.Strings(out)
		return out
	}
	const pre = `PREFIX : <http://e/> `
	for name, c := range map[string]struct {
		q    string
		want string
	}{
		// ?v exists only after the grouped solutions are joined with VALUES.
		"projected":         {pre + `SELECT ?s ?v { ?s :p ?o } GROUP BY ?s VALUES ?v { 1 }`, `?s=<http://e/a> ?v=1|?s=<http://e/b> ?v=1|?s=<http://e/c> ?v=1`},
		"select expression": {pre + `SELECT ?s (STR(?v) AS ?w) { ?s :p ?o } GROUP BY ?s VALUES ?v { 1 }`, `?s=<http://e/a> ?w="1"|?s=<http://e/b> ?w="1"|?s=<http://e/c> ?w="1"`},
		// The join is on the grouping variable.
		"joined on key": {pre + `SELECT ?s (COUNT(*) AS ?n) { ?s :p ?o } GROUP BY ?s VALUES ?s { :a }`, `?s=<http://e/a> ?n=2`},
		// Subquery: OFFSET and LIMIT apply after its own VALUES.
		"subquery slice": {pre + `SELECT ?s { { SELECT ?s { ?s :p ?o } ORDER BY ?s OFFSET 1 LIMIT 1 VALUES ?s { :b :c } } }`, `?s=<http://e/c>`},
	} {
		t.Run(name, func(t *testing.T) {
			got := strings.Join(rows(c.q), "|")
			if got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
	if _, err := Parse(pre + `SELECT ?s (COUNT(*) AS ?c) { ?s :p ?o } GROUP BY ?s VALUES ?c { 0 1 }`); err == nil {
		t.Error("an alias the trailing VALUES already binds was accepted")
	}
}

// TRIPLE(s, p, o): a triple term is not a subject (RDF 1.2).
func TestTripleRejectsTripleTermSubject(t *testing.T) {
	g := graph.NewGraph()
	r, err := QueryContext(t.Context(), g, `PREFIX : <http://e/>
SELECT ?t { VALUES (?s ?p ?o) { (<<(:x :y :z)>> :b :c) (:a :b <<(:x :y :z)>>) } BIND(TRIPLE(?s, ?p, ?o) AS ?t) }`)
	if err != nil {
		t.Fatal(err)
	}
	bound := 0
	for _, b := range r.Bindings {
		if _, ok := b["t"]; ok {
			bound++
		}
	}
	if len(r.Bindings) != 2 || bound != 1 {
		t.Errorf("%d rows, %d with a triple, want 2 rows and 1", len(r.Bindings), bound)
	}
}
