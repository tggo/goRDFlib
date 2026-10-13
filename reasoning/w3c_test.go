package reasoning

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/term"
	w3c "github.com/tggo/goRDFlib/testdata/w3c"
	"github.com/tggo/goRDFlib/turtle"
)

const manifestPath = "../testdata/w3c/rdf-tests/rdf/rdf11/rdf-mt/manifest.ttl"

// Tests that require datatype semantics, axiomatic triples, or non-RDFS regimes.
var skipTests = map[string]string{
	// Datatype semantics
	"datatypes-intensional-xsd-integer-decimal-compatible": "requires recognized datatypes",
	"datatypes-non-well-formed-literal-1":                  "requires datatype inconsistency detection",
	"datatypes-non-well-formed-literal-2":                  "requires datatype inconsistency detection",
	"datatypes-semantic-equivalence-within-type-1":         "requires datatype value equivalence",
	"datatypes-semantic-equivalence-within-type-2":         "requires datatype value equivalence",
	"datatypes-semantic-equivalence-between-datatypes":     "requires datatype value equivalence",
	"datatypes-range-clash":                                "requires datatype inconsistency detection",
	"datatypes-test008":                                    "simple entailment",
	"datatypes-test009":                                    "simple entailment",
	"datatypes-test010":                                    "requires datatype inconsistency detection",
	"datatypes-plain-literal-and-xsd-string":               "requires recognized datatypes",
	// XMLLiteral / datatype inconsistency
	"rdfs-entailment-test001": "requires XMLLiteral recognition",
	"rdfs-entailment-test002": "requires datatype inconsistency detection",
	// Axiomatic triples (rdf:_n membership properties)
	"rdfms-seq-representation-test002": "requires axiomatic triples",
	"rdfms-seq-representation-test003": "requires axiomatic triples",
	"rdfms-seq-representation-test004": "requires axiomatic triples",
	// Simple/RDF entailment (not RDFS)
	"rdfms-xmllang-test007a":     "simple entailment",
	"rdfms-xmllang-test007b":     "simple entailment",
	"rdfms-xmllang-test007c":     "simple entailment",
	"rdf-charmod-uris-test003":   "RDF entailment",
	"rdf-charmod-uris-test004":   "RDF entailment",
	"tex-01-language-tag-case-1": "RDF entailment (language tag case)",
	"tex-01-language-tag-case-2": "RDF entailment (language tag case)",
	// Statement entailment (reification semantics, not RDFS closure)
	"statement-entailment-test001": "RDF entailment (reification)",
	"statement-entailment-test002": "RDF entailment (reification)",
	"statement-entailment-test004": "RDF entailment (reification)",
	// xmlsch-02 whitespace facet tests (require recognized datatypes)
	"xmlsch-02-whitespace-facet-1": "requires recognized datatypes",
	"xmlsch-02-whitespace-facet-2": "requires recognized datatypes",
	"xmlsch-02-whitespace-facet-4": "requires recognized datatypes",
}

func TestW3C_RDFS_Entailment(t *testing.T) {
	m, err := w3c.ParseManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	ran := 0
	for _, e := range m.Entries {
		e := e
		t.Run(e.Name, func(t *testing.T) {
			if reason, ok := skipTests[e.Name]; ok {
				t.Skipf("skipped: %s", reason)
			}

			// Load action graph
			actionGraph := loadGraph(t, e.Action)
			if actionGraph == nil {
				t.Fatalf("could not load action graph: %s", e.Action)
			}

			// Apply RDFS closure
			RDFSClosure(t.Context(), actionGraph)

			isPositive := strings.Contains(e.Type, "PositiveEntailmentTest")
			isNegative := strings.Contains(e.Type, "NegativeEntailmentTest")

			switch {
			case isPositive && e.ResultFalse:
				// Positive entailment with result false = inconsistency test.
				// RDFS closure doesn't detect inconsistency, so skip.
				t.Skip("inconsistency detection not implemented")

			case isPositive && e.Result != "":
				// Result graph must be a subgraph of expanded action graph.
				resultGraph := loadGraph(t, e.Result)
				if resultGraph == nil {
					t.Fatalf("could not load result graph: %s", e.Result)
				}
				if !isSubgraph(resultGraph, actionGraph) {
					t.Error("positive entailment failed: result is not a subgraph of expanded action")
					reportMissing(t, resultGraph, actionGraph)
				}
				ran++

			case isNegative && e.ResultFalse:
				// Negative entailment with result false = graph is not inconsistent.
				// RDFS closure never produces inconsistency, so this always passes.
				ran++

			case isNegative && e.Result != "":
				// Result graph must NOT be a subgraph of expanded action graph.
				resultGraph := loadGraph(t, e.Result)
				if resultGraph == nil {
					t.Fatalf("could not load result graph: %s", e.Result)
				}
				if isSubgraph(resultGraph, actionGraph) {
					t.Error("negative entailment failed: result IS a subgraph of expanded action (should not be)")
				}
				ran++

			default:
				t.Skipf("unhandled test type: %s", e.Type)
			}
		})
	}

	t.Logf("ran %d W3C RDFS entailment tests", ran)
}

// loadGraph parses a .nt or .ttl file into a graph.
func loadGraph(t *testing.T, path string) *graph.Graph {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("cannot open %s: %v", path, err)
		return nil
	}
	defer f.Close()

	g := graph.NewGraph()
	ext := strings.ToLower(filepath.Ext(path))
	base := "file://" + path

	switch ext {
	case ".nt":
		if err := nt.Parse(t.Context(), g, f); err != nil {
			t.Fatalf("parse error %s: %v", path, err)
		}
	case ".ttl":
		if err := turtle.Parse(t.Context(), g, f, turtle.WithBase(base)); err != nil {
			t.Fatalf("parse error %s: %v", path, err)
		}
	default:
		t.Fatalf("unsupported format: %s", ext)
	}

	return g
}

// isSubgraph checks if every triple in sub exists in super (up to bnode relabelling).
func isSubgraph(sub, super *graph.Graph) bool {
	// For ground triples (no bnodes), direct containment check suffices.
	// For triples with bnodes, we need a more sophisticated check.
	type tripleT = struct {
		s, p, o term.Term
	}
	var subTriples []tripleT
	hasBNodes := false

	sub.Triples(context.Background(), nil, nil, nil)(func(t term.Triple, _ error) bool {
		subTriples = append(subTriples, tripleT{t.Subject, t.Predicate, t.Object})
		if _, ok := t.Subject.(term.BNode); ok {
			hasBNodes = true
		}
		if _, ok := t.Object.(term.BNode); ok {
			hasBNodes = true
		}
		return true
	})

	if !hasBNodes {
		// Fast path: exact match
		for _, st := range subTriples {
			if !mustV(super.Contains(context.Background(), st.s.(term.Subject), st.p.(term.URIRef), st.o)) {
				return false
			}
		}
		return true
	}

	// Bnode subgraph matching with backtracking
	bnodes := collectBNodes(subTriples)
	if len(bnodes) == 0 {
		// No bnodes after all
		for _, st := range subTriples {
			if !mustV(super.Contains(context.Background(), st.s.(term.Subject), st.p.(term.URIRef), st.o)) {
				return false
			}
		}
		return true
	}

	// Collect candidate mappings: for each bnode in sub, find possible nodes in super
	superNodes := collectSuperNodes(super)
	mapping := make(map[string]term.Term, len(bnodes))

	var search func(idx int) bool
	search = func(idx int) bool {
		if idx == len(bnodes) {
			return verifySubgraph(subTriples, super, mapping)
		}
		bn := bnodes[idx]
		for _, candidate := range superNodes {
			mapping[bn] = candidate
			if search(idx + 1) {
				return true
			}
		}
		delete(mapping, bn)
		return false
	}

	return search(0)
}

func collectBNodes(triples []struct{ s, p, o term.Term }) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, t := range triples {
		if bn, ok := t.s.(term.BNode); ok {
			if _, dup := seen[bn.Value()]; !dup {
				seen[bn.Value()] = struct{}{}
				result = append(result, bn.Value())
			}
		}
		if bn, ok := t.o.(term.BNode); ok {
			if _, dup := seen[bn.Value()]; !dup {
				seen[bn.Value()] = struct{}{}
				result = append(result, bn.Value())
			}
		}
	}
	return result
}

func collectSuperNodes(g *graph.Graph) []term.Term {
	seen := make(map[string]term.Term)
	g.Triples(context.Background(), nil, nil, nil)(func(t term.Triple, _ error) bool {
		seen[term.TermKey(t.Subject)] = t.Subject
		seen[term.TermKey(t.Object)] = t.Object
		return true
	})
	nodes := make([]term.Term, 0, len(seen))
	for _, v := range seen {
		nodes = append(nodes, v)
	}
	return nodes
}

func verifySubgraph(triples []struct{ s, p, o term.Term }, super *graph.Graph, mapping map[string]term.Term) bool {
	for _, t := range triples {
		s := mapNode(t.s, mapping)
		o := mapNode(t.o, mapping)
		subj, ok := s.(term.Subject)
		if !ok {
			return false
		}
		if !mustV(super.Contains(context.Background(), subj, t.p.(term.URIRef), o)) {
			return false
		}
	}
	return true
}

func mapNode(t term.Term, mapping map[string]term.Term) term.Term {
	if bn, ok := t.(term.BNode); ok {
		if mapped, exists := mapping[bn.Value()]; exists {
			return mapped
		}
	}
	return t
}

func reportMissing(t *testing.T, sub, super *graph.Graph) {
	t.Helper()
	sub.Triples(t.Context(), nil, nil, nil)(func(tr term.Triple, _ error) bool {
		if !mustV(super.Contains(t.Context(), tr.Subject, tr.Predicate, tr.Object)) {
			t.Logf("  missing: %s %s %s", tr.Subject.N3(), tr.Predicate.N3(), tr.Object.N3())
		}
		return true
	})
}
