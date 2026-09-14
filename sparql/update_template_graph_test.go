package sparql

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/turtle"
)

func updateTestDataset(t *testing.T, defaultData string) *Dataset {
	t.Helper()
	g := rdflibgo.NewGraph()
	if defaultData != "" {
		if err := turtle.Parse(g, strings.NewReader(defaultData)); err != nil {
			t.Fatal(err)
		}
	}
	return &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
}

// SPARQL 1.1 Update §3.1.3: a template triple containing an unbound variable
// is not included. A GRAPH ?g whose ?g is unbound used to fall back to the
// default graph.
func TestUpdateTemplate_UnboundGraphVarDeletesNothing(t *testing.T) {
	ds := updateTestDataset(t, `<urn:s> <urn:p> 1 .`)
	err := Update(ds, `DELETE { GRAPH ?g { <urn:s> <urn:p> ?o } }
WHERE { VALUES ?o { 1 } OPTIONAL { GRAPH ?g { <urn:nothing> <urn:p> ?x } } }`)
	if err != nil {
		t.Fatal(err)
	}
	if n := ds.Default.Len(); n != 1 {
		t.Errorf("default graph has %d triples, want 1", n)
	}
}

func TestUpdateTemplate_UnboundGraphVarInsertsNothing(t *testing.T) {
	ds := updateTestDataset(t, `<urn:s> <urn:p> 1 .`)
	err := Update(ds, `INSERT { GRAPH ?g { <urn:new> <urn:p> ?o } }
WHERE { VALUES ?o { 2 } OPTIONAL { GRAPH ?g { <urn:nothing> <urn:p> ?x } } }`)
	if err != nil {
		t.Fatal(err)
	}
	if n := ds.Default.Len(); n != 1 {
		t.Errorf("default graph has %d triples, want 1", n)
	}
	if n := len(ds.NamedGraphs); n != 0 {
		t.Errorf("%d named graphs created, want 0", n)
	}
}

// A GRAPH variable bound to a literal is an illegal RDF construct (§3.1.3).
func TestUpdateTemplate_LiteralGraphVarInsertsNothing(t *testing.T) {
	ds := updateTestDataset(t, ``)
	err := Update(ds, `INSERT { GRAPH ?g { <urn:new> <urn:p> 1 } } WHERE { BIND("g" AS ?g) }`)
	if err != nil {
		t.Fatal(err)
	}
	if n := ds.Default.Len(); n != 0 {
		t.Errorf("default graph has %d triples, want 0", n)
	}
}

// The other quads of the same template are still applied.
func TestUpdateTemplate_BoundGraphVarStillApplies(t *testing.T) {
	ds := updateTestDataset(t, ``)
	err := Update(ds, `INSERT { GRAPH ?g { <urn:a> <urn:p> 1 } <urn:b> <urn:p> 2 } WHERE { BIND(<urn:g> AS ?g) }`)
	if err != nil {
		t.Fatal(err)
	}
	if g := ds.NamedGraphs["urn:g"]; g == nil || g.Len() != 1 {
		t.Errorf("named graph <urn:g> not written")
	}
	if n := ds.Default.Len(); n != 1 {
		t.Errorf("default graph has %d triples, want 1", n)
	}
}
