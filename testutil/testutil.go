package testutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// AssertGraphEqual checks that two graphs contain the same triples.
// Note: blank node labels must match exactly; this does not perform BNode isomorphism.
// On failure, reports which triples differ.
// Ported from: test infrastructure for rdflib graph comparison
func AssertGraphEqual(t *testing.T, expected, actual *graph.Graph) {
	t.Helper()

	// Collect N3 forms of all triples (for non-BNode comparison)
	expTriples := graphTripleStrings(expected)
	actTriples := graphTripleStrings(actual)

	// Compare counts first
	if len(expTriples) != len(actTriples) {
		t.Errorf("graph triple count: expected %d, got %d", len(expTriples), len(actTriples))
	}

	// Find missing and extra
	expSet := make(map[string]bool)
	for _, s := range expTriples {
		expSet[s] = true
	}
	actSet := make(map[string]bool)
	for _, s := range actTriples {
		actSet[s] = true
	}

	var missing, extra []string
	for s := range expSet {
		if !actSet[s] {
			missing = append(missing, s)
		}
	}
	for s := range actSet {
		if !expSet[s] {
			extra = append(extra, s)
		}
	}

	if len(missing) > 0 || len(extra) > 0 {
		var sb strings.Builder
		if len(missing) > 0 {
			sb.WriteString("Missing triples:\n")
			for _, s := range missing {
				sb.WriteString("  - " + s + "\n")
			}
		}
		if len(extra) > 0 {
			sb.WriteString("Extra triples:\n")
			for _, s := range extra {
				sb.WriteString("  + " + s + "\n")
			}
		}
		t.Error(sb.String())
	}
}

func graphTripleStrings(g *graph.Graph) []string {
	var result []string
	g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		s := fmt.Sprintf("%s %s %s", t.Subject.N3(), t.Predicate.N3(), t.Object.N3())
		result = append(result, s)
		return true
	})
	return result
}

// AssertGraphContains checks that the graph contains a specific triple.
func AssertGraphContains(t *testing.T, g *graph.Graph, s term.Subject, p term.URIRef, o term.Term) {
	t.Helper()
	if !g.Contains(s, p, o) {
		t.Errorf("graph does not contain: %s %s %s", s.N3(), p.N3(), o.N3())
	}
}

// AssertGraphLen checks the number of triples in a graph.
func AssertGraphLen(t *testing.T, g *graph.Graph, expected int) {
	t.Helper()
	if g.Len() != expected {
		t.Errorf("expected %d triples, got %d", expected, g.Len())
	}
}
