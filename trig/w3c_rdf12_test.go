package trig_test

import (
	"os"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/testdata/w3c"
	"github.com/tggo/goRDFlib/trig"
)

var trigRDF12Manifests = []string{
	"../testdata/w3c/rdf-tests/rdf/rdf12/rdf-trig/eval/manifest.ttl",
	"../testdata/w3c/rdf-tests/rdf/rdf12/rdf-trig/syntax/manifest.ttl",
}

func TestW3CTrigRDF12(t *testing.T) {
	for _, mpath := range trigRDF12Manifests {
		m, err := w3c.ParseManifest(mpath)
		if err != nil {
			t.Fatalf("ParseManifest(%s): %v", mpath, err)
		}
		runTrigTests(t, m)
	}
}

func runTrigTests(t *testing.T, m *w3c.Manifest) {
	t.Helper()
	for _, e := range m.Entries {
		e := e
		t.Run(e.Name, func(t *testing.T) {
			switch e.Type {
			case w3c.RDFT + "TestTrigEval":
				base := m.BaseURI(e.Action)
				actual := graph.NewDataset()
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := trig.ParseDataset(actual, f, trig.WithBase(base)); err != nil {
					t.Fatalf("parse action: %v", err)
				}

				expected := graph.NewDataset()
				ef, err := os.Open(e.Result)
				if err != nil {
					t.Fatal(err)
				}
				defer ef.Close()
				parseNQuadsIntoDataset(t, expected, ef)

				assertDatasetEqual(t, expected, actual)

			case w3c.RDFT + "TestTrigPositiveSyntax":
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				base := m.BaseURI(e.Action)
				ds := graph.NewDataset()
				if err := trig.ParseDataset(ds, f, trig.WithBase(base)); err != nil {
					t.Errorf("expected no error, got: %v", err)
				}

			case w3c.RDFT + "TestTrigNegativeSyntax", w3c.RDFT + "TestTrigNegativeEval":
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				base := m.BaseURI(e.Action)
				ds := graph.NewDataset()
				if err := trig.ParseDataset(ds, f, trig.WithBase(base)); err == nil {
					t.Error("expected error, got nil")
				}

			default:
				t.Skipf("unknown test type: %s", e.Type)
			}
		})
	}
}
