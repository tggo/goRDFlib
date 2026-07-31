package sparql

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/turtle"
)

// Regression test for issue #21: a variable that is already bound when a basic
// graph pattern is evaluated must stay bound even when its value cannot occupy
// the position it appears in. evalBGP resolved the binding, failed the
// Subject/URIRef type assertion for a literal, and left the position nil — a
// wildcard — so the pattern matched unrelated triples. Inside FILTER (NOT)
// EXISTS that made `?value a ex:Widget` behave as if `?value` were a fresh
// variable: any Widget in the graph satisfied it, and NOT EXISTS wrongly
// dropped every literal-bound row.

const issue21Data = `
@prefix ex: <http://example.com/> .

ex:a a ex:Thing ; ex:prop "hello" .
ex:b a ex:Thing ; ex:prop ex:dangling .
ex:w a ex:Widget .
`

func issue21Graph(t *testing.T) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(issue21Data)); err != nil {
		t.Fatal(err)
	}
	return g
}

func issue21Values(t *testing.T, g *rdflibgo.Graph, q string) []string {
	t.Helper()
	res, err := Query(g, q)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range res.Bindings {
		got = append(got, b["value"].String())
	}
	return got
}

func TestIssue21NotExistsKeepsLiteralBinding(t *testing.T) {
	g := issue21Graph(t)

	got := issue21Values(t, g, `PREFIX ex: <http://example.com/>
		SELECT ?this ?value WHERE {
			?this ex:prop ?value .
			FILTER NOT EXISTS { ?value a ex:Widget . }
		}`)
	if len(got) != 2 {
		t.Fatalf("NOT EXISTS: want 2 rows (literal and dangling IRI), got %d: %v", len(got), got)
	}

	// The mirror case: EXISTS must not match either, since neither value is a
	// Widget. Before the fix the literal row matched via the wildcard subject.
	got = issue21Values(t, g, `PREFIX ex: <http://example.com/>
		SELECT ?this ?value WHERE {
			?this ex:prop ?value .
			FILTER EXISTS { ?value a ex:Widget . }
		}`)
	if len(got) != 0 {
		t.Fatalf("EXISTS: want 0 rows, got %d: %v", len(got), got)
	}
}

// A literal bound in one BGP triple must not act as a wildcard subject in the
// next one, independently of EXISTS.
func TestIssue21LiteralBoundSubjectInBGP(t *testing.T) {
	g := issue21Graph(t)

	got := issue21Values(t, g, `PREFIX ex: <http://example.com/>
		SELECT ?value WHERE {
			?this ex:prop ?value .
			?value a ex:Widget .
		}`)
	if len(got) != 0 {
		t.Fatalf("want 0 rows, got %d: %v", len(got), got)
	}
}

// Same for the predicate position: a variable bound to a literal can never be
// a predicate, so the join must yield nothing rather than matching everything.
func TestIssue21LiteralBoundPredicate(t *testing.T) {
	g := issue21Graph(t)

	res, err := Query(g, `PREFIX ex: <http://example.com/>
		SELECT ?s ?value WHERE {
			?this ex:prop ?value .
			?s ?value ?o .
		}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range res.Bindings {
		if _, isLit := b["value"].(rdflibgo.Literal); isLit {
			t.Fatalf("literal used as predicate matched: %v", b)
		}
	}
}

// Property paths take a separate evaluation route with the same resolution
// step, so they need the same guard.
func TestIssue21LiteralBoundSubjectInPath(t *testing.T) {
	g := issue21Graph(t)

	got := issue21Values(t, g, `PREFIX ex: <http://example.com/>
		PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
		SELECT ?value WHERE {
			?this ex:prop ?value .
			?value rdf:type/^rdf:type ?other .
		}`)
	if len(got) != 0 {
		t.Fatalf("want 0 rows, got %d: %v", len(got), got)
	}
}
