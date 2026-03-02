package nt_test

import (
	"os"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/testdata/w3c"
)

const ntManifest = "../testdata/w3c/rdf-tests/rdf/rdf11/rdf-n-triples/manifest.ttl"

func TestW3CNTriples(t *testing.T) {
	m, err := w3c.ParseManifest(ntManifest)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	for _, e := range m.Entries {
		e := e
		t.Run(e.Name, func(t *testing.T) {
			switch e.Type {
			case w3c.RDFT + "TestNTriplesPositiveSyntax":
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				g := graph.NewGraph()
				if err := nt.Parse(g, f); err != nil {
					t.Errorf("expected no error, got: %v", err)
				}

			case w3c.RDFT + "TestNTriplesNegativeSyntax":
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				g := graph.NewGraph()
				if err := nt.Parse(g, f); err == nil {
					t.Error("expected error, got nil")
				}

			default:
				t.Skipf("unknown test type: %s", e.Type)
			}
		})
	}
}
