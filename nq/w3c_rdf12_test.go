package nq_test

import (
	"os"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/testdata/w3c"
)

const nq12Manifest = "../testdata/w3c/rdf-tests/rdf/rdf12/rdf-n-quads/syntax/manifest.ttl"

func TestW3CNQuads12(t *testing.T) {
	m, err := w3c.ParseManifest(nq12Manifest)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	for _, e := range m.Entries {
		e := e
		t.Run(e.Name, func(t *testing.T) {
			switch e.Type {
			case w3c.RDFT + "TestNQuadsPositiveSyntax":
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				g := graph.NewGraph()
				if err := nq.Parse(t.Context(), g, f); err != nil {
					t.Errorf("expected no error, got: %v", err)
				}

			case w3c.RDFT + "TestNQuadsNegativeSyntax":
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				g := graph.NewGraph()
				if err := nq.Parse(t.Context(), g, f); err == nil {
					t.Error("expected error, got nil")
				}

			default:
				t.Skipf("unknown test type: %s", e.Type)
			}
		})
	}
}
