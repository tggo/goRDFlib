package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// The places TestRelativeIRIsResolveAgainstBase does not reach resolve a
// relative IRI against BASE too: a property path, an IRI in an expression
// list (IN), a literal's datatype, and an IRI inside an aggregate.
func TestRelativeIRIsResolveAgainstBaseInPathsDatatypesAndAggregates(t *testing.T) {
	const base = "https://example.org/id/"
	ctx := t.Context()
	id := func(s string) rdflibgo.URIRef { return rdflibgo.NewURIRefUnsafe(base + s) }
	g := rdflibgo.NewGraph()
	for _, tr := range [][3]rdflibgo.Term{
		{id("a"), id("rel/:next"), id("b")},
		{id("b"), id("rel/:next"), id("c")},
		{id("a"), id("v"), rdflibgo.NewLiteral("5", rdflibgo.WithDatatype(id("types/:km")))},
	} {
		if err := g.Add(ctx, tr[0].(rdflibgo.URIRef), tr[1].(rdflibgo.URIRef), tr[2]); err != nil {
			t.Fatal(err)
		}
	}
	pro := "BASE <" + base + ">\n"
	for desc, tc := range map[string]struct {
		query string
		want  []string
	}{
		"path, plain":      {`SELECT ?o WHERE { <a> <rel/:next> ?o }`, []string{base + "b"}},
		"path, closure":    {`SELECT ?o WHERE { <a> <rel/:next>+ ?o }`, []string{base + "b", base + "c"}},
		"path, sequence":   {`SELECT ?o WHERE { <a> <rel/:next>/<./rel/:next> ?o }`, []string{base + "c"}},
		"path, inverse":    {`SELECT ?o WHERE { <c> ^<rel/:next> ?o }`, []string{base + "b"}},
		"IN list":          {`SELECT ?o WHERE { <a> <rel/:next> ?o FILTER(?o IN (<b>, <x/:y>)) }`, []string{base + "b"}},
		"datatype":         {`SELECT ?s WHERE { ?s <v> "5"^^<types/:km> }`, []string{base + "a"}},
		"datatype, filter": {`SELECT ?s WHERE { ?s <v> ?v FILTER(DATATYPE(?v) = <types/:km>) }`, []string{base + "a"}},
		"aggregate":        {`SELECT (SAMPLE(<x/:y>) AS ?o) WHERE { <a> <v> ?v }`, []string{base + "x/:y"}},
	} {
		t.Run(desc, func(t *testing.T) {
			res, err := Query(g, pro+tc.query)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, row := range res.Bindings {
				for _, v := range row {
					got = append(got, v.(rdflibgo.URIRef).Value())
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for _, w := range tc.want {
				found := false
				for _, x := range got {
					found = found || x == w
				}
				if !found {
					t.Errorf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}
