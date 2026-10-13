package rdfxml_test

import (
	"os"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/testdata/w3c"
	"github.com/tggo/goRDFlib/testutil"
)

func TestW3CRDFXML12(t *testing.T) {
	const manifest = "../testdata/w3c/rdf-tests/rdf/rdf12/rdf-xml/eval/manifest.ttl"
	m, err := w3c.ParseManifest(manifest)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	for _, e := range m.Entries {
		e := e
		t.Run(e.Name, func(t *testing.T) {
			switch e.Type {
			case w3c.RDFT + "TestXMLEval":
				base := m.BaseURI(e.Action)
				actual := graph.NewGraph(graph.WithBase(base))
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := rdfxml.Parse(t.Context(), actual, f, rdfxml.WithBase(base)); err != nil {
					t.Fatalf("parse action: %v", err)
				}

				expected := graph.NewGraph()
				ef, err := os.Open(e.Result)
				if err != nil {
					t.Fatal(err)
				}
				defer ef.Close()
				if err := nt.Parse(t.Context(), expected, ef); err != nil {
					t.Fatalf("parse result: %v", err)
				}

				testutil.AssertGraphEqual(t, expected, actual)

			case w3c.RDFT + "TestXMLNegativeSyntax":
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				base := m.BaseURI(e.Action)
				g := graph.NewGraph(graph.WithBase(base))
				if err := rdfxml.Parse(t.Context(), g, f, rdfxml.WithBase(base)); err == nil {
					t.Error("expected error, got nil")
				}

			default:
				t.Skipf("unknown test type: %s", e.Type)
			}
		})
	}
}
