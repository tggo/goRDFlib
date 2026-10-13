package turtle_test

import (
	"os"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/testdata/w3c"
	"github.com/tggo/goRDFlib/testutil"
	"github.com/tggo/goRDFlib/turtle"
)

func TestW3CTurtle12Syntax(t *testing.T) {
	const manifest = "../testdata/w3c/rdf-tests/rdf/rdf12/rdf-turtle/syntax/manifest.ttl"
	m, err := w3c.ParseManifest(manifest)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	for _, e := range m.Entries {
		e := e
		t.Run(e.Name, func(t *testing.T) {
			switch e.Type {
			case w3c.RDFT + "TestTurtlePositiveSyntax":
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				base := m.BaseURI(e.Action)
				g := graph.NewGraph(graph.WithBase(base))
				if err := turtle.Parse(t.Context(), g, f, turtle.WithBase(base)); err != nil {
					t.Errorf("expected no error, got: %v", err)
				}

			case w3c.RDFT + "TestTurtleNegativeSyntax":
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				base := m.BaseURI(e.Action)
				g := graph.NewGraph(graph.WithBase(base))
				if err := turtle.Parse(t.Context(), g, f, turtle.WithBase(base)); err == nil {
					t.Error("expected error, got nil")
				}

			default:
				t.Skipf("unknown test type: %s", e.Type)
			}
		})
	}
}

func TestW3CTurtle12Eval(t *testing.T) {
	const manifest = "../testdata/w3c/rdf-tests/rdf/rdf12/rdf-turtle/eval/manifest.ttl"
	m, err := w3c.ParseManifest(manifest)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	for _, e := range m.Entries {
		e := e
		t.Run(e.Name, func(t *testing.T) {
			switch e.Type {
			case w3c.RDFT + "TestTurtleEval":
				base := m.BaseURI(e.Action)
				actual := graph.NewGraph(graph.WithBase(base))
				f, err := os.Open(e.Action)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := turtle.Parse(t.Context(), actual, f, turtle.WithBase(base)); err != nil {
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

			default:
				t.Skipf("unknown test type: %s", e.Type)
			}
		})
	}
}
