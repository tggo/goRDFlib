package sparql

import (
	"context"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Issue #74: a sub-SELECT resolves relative IRIs against the query's BASE.
func TestSubqueryResolvesRelativeIRIsAgainstBase(t *testing.T) {
	const base = "https://example.org/id/"
	s, _ := rdflibgo.NewURIRef(base + "topic/orders")
	p, _ := rdflibgo.NewURIRef("https://example.org/ns#ownedBy")
	o, _ := rdflibgo.NewURIRef(base + "team/payments")
	ctx := context.Background()
	newG := func() *rdflibgo.Graph {
		g := rdflibgo.NewGraph()
		if err := g.Add(ctx, s, p, o); err != nil {
			t.Fatal(err)
		}
		return g
	}
	def, named := newG(), newG()
	pro := "BASE <" + base + ">\nPREFIX ex: <https://example.org/ns#>\n"
	queries := map[string]string{
		"graph in main":        `SELECT ?topic WHERE { GRAPH <graph/topics> { ?topic ex:ownedBy ?t } }`,
		"graph in subquery":    `SELECT ?topic WHERE { { SELECT ?topic WHERE { GRAPH <graph/topics> { ?topic ex:ownedBy ?t } } } }`,
		"object in subquery":   `SELECT ?topic WHERE { { SELECT ?topic WHERE { ?topic ex:ownedBy <team/payments> } } }`,
		"nested subquery":      `SELECT ?topic WHERE { { SELECT ?topic WHERE { { SELECT ?topic WHERE { ?topic ex:ownedBy <team/payments> } } } } }`,
		"subquery in EXISTS":   `SELECT ?topic WHERE { ?topic ex:ownedBy ?t FILTER EXISTS { { SELECT ?topic WHERE { ?topic ex:ownedBy <team/payments> } } } }`,
		"absolute in subquery": `SELECT ?topic WHERE { { SELECT ?topic WHERE { GRAPH <` + base + `graph/topics> { ?topic ex:ownedBy ?t } } } }`,
	}
	for name, q := range queries {
		t.Run(name, func(t *testing.T) {
			pq, err := Parse(pro + q)
			if err != nil {
				t.Fatal(err)
			}
			pq.NamedGraphs = map[string]*rdflibgo.Graph{base + "graph/topics": named}
			res, err := EvalQueryContext(ctx, def, pq, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Bindings) != 1 || res.Bindings[0]["topic"] != rdflibgo.Term(s) {
				t.Fatalf("got %v, want one row ?topic=%s", res.Bindings, s)
			}
		})
	}
}

// Issue #74, update side: a sub-SELECT in an update WHERE inherits BASE.
func TestUpdateSubqueryResolvesRelativeIRIsAgainstBase(t *testing.T) {
	ctx := context.Background()
	ds := &Dataset{Default: rdflibgo.NewGraph(), NamedGraphs: map[string]*rdflibgo.Graph{}}
	u := `BASE <https://example.org/id/>
INSERT DATA { <a> <p> <b> } ;
INSERT { <c> <q> ?x } WHERE { { SELECT ?x WHERE { <a> <p> ?x } } }`
	if err := Update(ds, u); err != nil {
		t.Fatal(err)
	}
	c, _ := rdflibgo.NewURIRef("https://example.org/id/c")
	n := 0
	for _, err := range ds.Default.Triples(ctx, c, nil, nil) {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != 1 {
		t.Fatalf("got %d triples for <c>, want 1", n)
	}
}
