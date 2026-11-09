package integration

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// rdfManifestRunners names, for every manifest.ttl under rdf-tests/rdf, the
// test that runs it, "not run: <reason>", or "aggregate" for a manifest that only mf:includes
// others. A manifest the upstream suite adds and nobody wired in fails here
// instead of going unnoticed at a pin bump; one that is listed but gone fails
// too, so the list cannot rot.
var rdfManifestRunners = map[string]string{
	"rdf11/manifest.ttl":                      "aggregate",
	"rdf11/rdf-n-triples/manifest.ttl":        "nt: TestW3CNTriples",
	"rdf11/rdf-n-quads/manifest.ttl":          "nq: TestW3CNQuads",
	"rdf11/rdf-turtle/manifest.ttl":           "turtle: TestW3CTurtle",
	"rdf11/rdf-trig/manifest.ttl":             "trig: TestW3CTrig",
	"rdf11/rdf-xml/manifest.ttl":              "rdfxml: TestW3CRDFXML",
	"rdf11/rdf-mt/manifest.ttl":               "reasoning: TestW3C_Entailment_RDF11",
	"rdf11/rdf-mt/az-tests/manifest.ttl":      "not run: an Unapproved manifest no parent includes (needs rdf:HTML and other datatypes)",
	"rdf12/manifest.ttl":                      "aggregate",
	"rdf12/rdf-n-triples/manifest.ttl":        "aggregate",
	"rdf12/rdf-n-triples/syntax/manifest.ttl": "nt: TestW3CNTriples12",
	"rdf12/rdf-n-triples/c14n/manifest.ttl":   "nt: TestW3CNTriples12C14N",
	"rdf12/rdf-n-quads/manifest.ttl":          "aggregate",
	"rdf12/rdf-n-quads/syntax/manifest.ttl":   "nq: TestW3CNQuads12",
	"rdf12/rdf-n-quads/c14n/manifest.ttl":     "nq: TestW3CNQuads12C14N",
	"rdf12/rdf-semantics/manifest.ttl":        "reasoning: TestW3C_Entailment_RDF12",
	"rdf12/rdf-turtle/manifest.ttl":           "aggregate",
	"rdf12/rdf-turtle/syntax/manifest.ttl":    "turtle: TestW3CTurtle12Syntax",
	"rdf12/rdf-turtle/eval/manifest.ttl":      "turtle: TestW3CTurtle12Eval",
	"rdf12/rdf-trig/manifest.ttl":             "aggregate",
	"rdf12/rdf-trig/syntax/manifest.ttl":      "trig: TestW3CTrigRDF12",
	"rdf12/rdf-trig/eval/manifest.ttl":        "trig: TestW3CTrigRDF12",
	"rdf12/rdf-xml/manifest.ttl":              "aggregate",
	"rdf12/rdf-xml/eval/manifest.ttl":         "rdfxml: TestW3CRDFXML12",
}

func TestEveryRDFManifestIsRun(t *testing.T) {
	root := "../../testdata/w3c/rdf-tests/rdf"
	if _, err := os.Stat(filepath.Join(root, "manifest.ttl")); err == nil {
		t.Log("rdf/manifest.ttl exists but is not expected; list it")
	}
	var found []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "manifest.ttl" {
			rel, _ := filepath.Rel(root, path)
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if len(found) == 0 {
		t.Skip("no W3C files (submodule not checked out?)")
	}
	for _, rel := range found {
		if _, ok := rdfManifestRunners[rel]; !ok {
			t.Errorf("%s is not run by any test: wire it in and list it in rdfManifestRunners", rel)
		}
	}
	for rel := range rdfManifestRunners {
		if !slices.Contains(found, rel) {
			t.Errorf("%s is listed but absent from the suite: remove it from rdfManifestRunners", rel)
		}
	}
}
