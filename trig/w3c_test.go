package trig_test

import (
	"os"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/testdata/w3c"
	"github.com/tggo/goRDFlib/trig"
)

const trigManifest = "../testdata/w3c/rdf-tests/rdf/rdf11/rdf-trig/manifest.ttl"

func TestW3CTrig(t *testing.T) {
	m, err := w3c.ParseManifest(trigManifest)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

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

// parseNQuadsIntoDataset parses N-Quads into a dataset, routing quads to named graphs.
func parseNQuadsIntoDataset(t *testing.T, ds *graph.Dataset, f *os.File) {
	t.Helper()
	defaultGraph := ds.DefaultContext()
	nq.Parse(defaultGraph, f, nq.WithQuadHandler(func(s term.Subject, p term.URIRef, o term.Term, graphCtx term.Term) {
		target := ds.Graph(graphCtx) // nil → default graph
		target.Add(s, p, o)
	}))
}

// assertDatasetEqual checks that two datasets have isomorphic graphs.
func assertDatasetEqual(t *testing.T, expected, actual *graph.Dataset) {
	t.Helper()

	// Collect quads from both datasets
	expQuads := collectQuads(expected)
	actQuads := collectQuads(actual)

	if len(expQuads) != len(actQuads) {
		t.Errorf("quad count: expected %d, got %d", len(expQuads), len(actQuads))
		t.Logf("expected quads:")
		for _, q := range expQuads {
			t.Logf("  %s", q)
		}
		t.Logf("actual quads:")
		for _, q := range actQuads {
			t.Logf("  %s", q)
		}
		return
	}

	// Check isomorphism considering blank nodes in both triple positions and graph labels
	if !quadsIsomorphic(expQuads, actQuads) {
		t.Errorf("datasets not isomorphic")
		t.Logf("expected quads:")
		for _, q := range expQuads {
			t.Logf("  %s", q)
		}
		t.Logf("actual quads:")
		for _, q := range actQuads {
			t.Logf("  %s", q)
		}
	}
}

type quad struct {
	s, p, o term.Term
	g       term.Term // nil for default graph
}

func (q quad) String() string {
	gs := "<default>"
	if q.g != nil {
		gs = q.g.N3()
	}
	return q.s.N3() + " " + q.p.N3() + " " + q.o.N3() + " " + gs
}

func collectQuads(ds *graph.Dataset) []quad {
	var quads []quad
	defaultID := ds.DefaultContext().Identifier()
	for g := range ds.Graphs() {
		var graphLabel term.Term
		if g.Identifier() != defaultID {
			graphLabel = g.Identifier()
		}
		g.Triples(nil, nil, nil)(func(t term.Triple) bool {
			quads = append(quads, quad{t.Subject, t.Predicate, t.Object, graphLabel})
			return true
		})
	}
	return quads
}

// quadsIsomorphic checks if two sets of quads are isomorphic up to blank node renaming.
func quadsIsomorphic(expected, actual []quad) bool {
	// Try to find a blank node mapping from expected to actual
	mapping := make(map[string]string) // expected bnode label -> actual bnode label
	return matchQuads(expected, actual, mapping, 0)
}

func matchQuads(expected, actual []quad, mapping map[string]string, idx int) bool {
	if idx == len(expected) {
		return true
	}

	eq := expected[idx]
	for i, aq := range actual {
		if aq.p == nil {
			continue // already matched
		}
		newMapping := copyMapping(mapping)
		if quadMatch(eq, aq, newMapping) {
			// Mark as used
			saved := actual[i]
			actual[i] = quad{} // sentinel
			if matchQuads(expected, actual, newMapping, idx+1) {
				return true
			}
			actual[i] = saved
		}
	}
	return false
}

func quadMatch(e, a quad, mapping map[string]string) bool {
	return termMatch(e.s, a.s, mapping) &&
		termMatch(e.p, a.p, mapping) &&
		termMatch(e.o, a.o, mapping) &&
		termMatch(e.g, a.g, mapping)
}

func termMatch(e, a term.Term, mapping map[string]string) bool {
	if e == nil && a == nil {
		return true
	}
	if e == nil || a == nil {
		return false
	}

	eb, eBNode := e.(term.BNode)
	ab, aBNode := a.(term.BNode)
	if eBNode && aBNode {
		ek := eb.N3()
		ak := ab.N3()
		if mapped, ok := mapping[ek]; ok {
			return mapped == ak
		}
		// Check reverse: no other expected bnode maps to this actual bnode
		for _, v := range mapping {
			if v == ak {
				return false
			}
		}
		mapping[ek] = ak
		return true
	}
	if eBNode || aBNode {
		return false
	}

	// For triple terms, recursively match
	ett, eIsTT := e.(term.TripleTerm)
	att, aIsTT := a.(term.TripleTerm)
	if eIsTT && aIsTT {
		return termMatch(ett.Subject(), att.Subject(), mapping) &&
			termMatch(ett.Predicate(), att.Predicate(), mapping) &&
			termMatch(ett.Object(), att.Object(), mapping)
	}

	return e.N3() == a.N3()
}

func copyMapping(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
