package sparql

import (
	"context"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// A relative IRI is resolved against BASE by RFC 3986 §5.2 wherever a query
// writes it, whatever characters its path holds: a colon after the first
// segment does not make it absolute, and a non-ASCII character is kept as it
// is. An absolute IRI is used as written.
func TestRelativeIRIsResolveAgainstBase(t *testing.T) {
	const base = "https://example.org/id/"
	ctx := context.Background()
	id := func(s string) rdflibgo.URIRef { return rdflibgo.NewURIRefUnsafe(base + s) }
	name := rdflibgo.NewURIRefUnsafe("https://example.org/ns#name")
	dependsOn := rdflibgo.NewURIRefUnsafe("https://example.org/ns#dependsOn")

	def := rdflibgo.NewGraph()
	named := rdflibgo.NewGraph()
	for _, tr := range []struct {
		s, p rdflibgo.URIRef
		o    rdflibgo.Term
	}{
		{id("module/android/:core:ui"), name, rdflibgo.NewLiteral(":core:ui")},
		{id("module/android/:app"), name, rdflibgo.NewLiteral(":app")},
		{id("module/android/:app"), dependsOn, id("module/android/:core:ui")},
		{id(":core:ui"), name, rdflibgo.NewLiteral("first segment")},
		{id("café"), name, rdflibgo.NewLiteral("café")},
		{rdflibgo.NewURIRefUnsafe(base + "dots/../kept"), name, rdflibgo.NewLiteral("kept")},
	} {
		if err := def.Add(ctx, tr.s, tr.p, tr.o); err != nil {
			t.Fatal(err)
		}
	}
	if err := named.Add(ctx, id("module/android/:core:ui"), name, rdflibgo.NewLiteral(":core:ui")); err != nil {
		t.Fatal(err)
	}

	pro := "BASE <" + base + ">\nPREFIX ex: <https://example.org/ns#>\n"
	for desc, tc := range map[string]struct{ query, want string }{
		"colon after a slash, subject": {`SELECT ?x WHERE { <module/android/:core:ui> ex:name ?x }`, ":core:ui"},
		"colon after a slash, object":  {`SELECT ?x WHERE { ?s ex:dependsOn <module/android/:core:ui> ; ex:name ?x }`, ":app"},
		"colon after a slash, VALUES":  {`SELECT ?x WHERE { VALUES ?s { <module/android/:core:ui> } ?s ex:name ?x }`, ":core:ui"},
		"colon after a slash, FILTER":  {`SELECT ?x WHERE { ?s ex:name ?x FILTER(?s = <module/android/:core:ui>) }`, ":core:ui"},
		"colon after a slash, BIND":    {`SELECT ?x WHERE { BIND(<module/android/:core:ui> AS ?s) ?s ex:name ?x }`, ":core:ui"},
		"colon after a slash, IRI()":   {`SELECT ?x WHERE { BIND(IRI("module/android/:core:ui") AS ?s) ?s ex:name ?x }`, ":core:ui"},
		"colon in the graph name":      {`SELECT ?x WHERE { GRAPH <graph/android:modules> { ?s ex:name ?x } }`, ":core:ui"},
		"colon in the first segment":   {`SELECT ?x WHERE { <./:core:ui> ex:name ?x }`, "first segment"},
		"dot segments, pattern":        {`SELECT ?x WHERE { <module/android/:app/../:core:ui> ex:name ?x }`, ":core:ui"},
		"dot segments, IRI()":          {`SELECT ?x WHERE { BIND(IRI("./module/android/:app/../:core:ui") AS ?s) ?s ex:name ?x }`, ":core:ui"},
		"non-ASCII, pattern":           {`SELECT ?x WHERE { <café> ex:name ?x }`, "café"},
		"non-ASCII, VALUES":            {`SELECT ?x WHERE { VALUES ?s { <café> } ?s ex:name ?x }`, "café"},
		"absolute, as written":         {`SELECT ?x WHERE { <` + base + `dots/../kept> ex:name ?x }`, "kept"},
	} {
		t.Run(desc, func(t *testing.T) {
			pq, err := Parse(pro + tc.query)
			if err != nil {
				t.Fatal(err)
			}
			pq.NamedGraphs = map[string]*rdflibgo.Graph{base + "graph/android:modules": named}
			res, err := EvalQueryContext(ctx, def, pq, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Bindings) != 1 || res.Bindings[0]["x"] != rdflibgo.Term(rdflibgo.NewLiteral(tc.want)) {
				t.Fatalf("got %v, want one row ?x=%q", res.Bindings, tc.want)
			}
		})
	}
}

// The IRIs in a template, here an update's, are resolved the same way: a
// non-ASCII character is kept as it is rather than percent-encoded.
func TestTemplateResolvesRelativeIRIsAgainstBase(t *testing.T) {
	ctx := context.Background()
	ds := &Dataset{Default: rdflibgo.NewGraph(), NamedGraphs: map[string]*rdflibgo.Graph{}}
	u := `BASE <https://example.org/id/>
INSERT { <café> <https://example.org/ns#name> "café" } WHERE {}`
	if err := Update(ds, u); err != nil {
		t.Fatal(err)
	}
	want := rdflibgo.NewURIRefUnsafe("https://example.org/id/café")
	n := 0
	for _, err := range ds.Default.Triples(ctx, want, nil, nil) {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != 1 {
		t.Fatalf("got %d triples for %s, want 1", n, want)
	}
}
