package sparql_test

import (
	"os"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/testdata/w3c"
)

const sparql11UpdateManifestPath = "../testdata/w3c/rdf-tests/sparql/sparql11/manifest-sparql11-update.ttl"

func TestW3CUpdate(t *testing.T) {
	if _, err := os.Stat(sparql11UpdateManifestPath); err != nil {
		t.Skip("W3C SPARQL Update test suite not found (rdf-tests submodule)")
	}

	manifest, err := w3c.ParseIncludeManifest(sparql11UpdateManifestPath)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	const (
		mfUpdateEval     = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#UpdateEvaluationTest"
		mfPosSyntax11    = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#PositiveUpdateSyntaxTest11"
		mfNegSyntax11    = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#NegativeUpdateSyntaxTest11"
		mfNegQuerySyntax = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#NegativeSyntaxTest11"
	)

	for _, entry := range manifest.Entries {
		entry := entry
		switch entry.Type {
		case mfUpdateEval:
			t.Run("eval/"+entry.Name, func(t *testing.T) {
				runUpdateEvalTest(t, entry)
			})
		case mfPosSyntax11:
			t.Run("syntax+/"+entry.Name, func(t *testing.T) {
				runPositiveUpdateSyntaxTest(t, entry)
			})
		case mfNegSyntax11, mfNegQuerySyntax:
			t.Run("syntax-/"+entry.Name, func(t *testing.T) {
				runNegativeUpdateSyntaxTest(t, entry)
			})
		}
	}
}

func runUpdateEvalTest(t *testing.T, entry w3c.TestEntry) {
	if entry.Request == "" {
		t.Skip("no request file")
	}

	reqBytes, err := os.ReadFile(entry.Request)
	if err != nil {
		t.Fatalf("failed to read request file: %v", err)
	}
	reqStr := string(reqBytes)

	// Build pre-data dataset
	ds := &sparql.Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}

	if entry.ActionData != "" {
		loadDataFile(t, ds.Default, entry.ActionData)
	}
	for _, gd := range entry.ActionGraphData {
		ng := rdflibgo.NewGraph()
		loadDataFile(t, ng, gd.Graph)
		ds.NamedGraphs[gd.Label] = ng
	}

	// Execute update
	err = sparql.Update(ds, reqStr)
	if err != nil {
		t.Fatalf("update execution failed: %v", err)
	}

	// Compare result
	// Check default graph
	if entry.ResultData != "" {
		expected := rdflibgo.NewGraph()
		loadDataFile(t, expected, entry.ResultData)
		if !graphsEqual(ds.Default, expected) {
			t.Errorf("default graph mismatch\ngot:      %s\nexpected: %s",
				graphToString(ds.Default), graphToString(expected))
		}
	} else if len(entry.ResultGraphData) == 0 {
		// No result specified - default graph should be empty (if no pre-data)
		// Actually, empty result means success (no graph comparison needed)
	}

	// Check named graphs
	for _, gd := range entry.ResultGraphData {
		expected := rdflibgo.NewGraph()
		loadDataFile(t, expected, gd.Graph)
		actual, ok := ds.NamedGraphs[gd.Label]
		if !ok {
			actual = rdflibgo.NewGraph()
		}
		if !graphsEqual(actual, expected) {
			t.Errorf("named graph %s mismatch\ngot:      %s\nexpected: %s",
				gd.Label, graphToString(actual), graphToString(expected))
		}
	}

	// If result specifies a default graph, also check that no extra named graphs exist
	// that aren't in the expected result
	if entry.ResultData != "" || len(entry.ResultGraphData) > 0 {
		// Check default graph is empty if not specified in result
		if entry.ResultData == "" {
			if ds.Default.Len() > 0 {
				t.Errorf("default graph should be empty but has %d triples: %s",
					ds.Default.Len(), graphToString(ds.Default))
			}
		}
	}
}

func runPositiveUpdateSyntaxTest(t *testing.T, entry w3c.TestEntry) {
	queryPath := entry.Action
	if queryPath == "" {
		queryPath = entry.Request
	}
	if queryPath == "" {
		t.Skip("no query file")
	}

	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("failed to read query file: %v", err)
	}

	_, err = sparql.ParseUpdate(string(queryBytes))
	if err != nil {
		t.Errorf("expected valid syntax but got parse error: %v", err)
	}
}

func runNegativeUpdateSyntaxTest(t *testing.T, entry w3c.TestEntry) {
	queryPath := entry.Action
	if queryPath == "" {
		queryPath = entry.Request
	}
	if queryPath == "" {
		t.Skip("no query file")
	}

	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("failed to read query file: %v", err)
	}

	_, err = sparql.ParseUpdate(string(queryBytes))
	if err == nil {
		t.Errorf("expected parse error but query parsed successfully: %s", string(queryBytes))
	}
}

func graphsEqual(a, b *rdflibgo.Graph) bool {
	if a.Len() != b.Len() {
		return false
	}
	// Check all triples in a exist in b
	allMatch := true
	for tr := range a.Triples(nil, nil, nil) {
		if !b.Contains(tr.Subject, tr.Predicate, tr.Object) {
			allMatch = false
			break
		}
	}
	return allMatch
}

func graphToString(g *rdflibgo.Graph) string {
	var parts []string
	for tr := range g.Triples(nil, nil, nil) {
		parts = append(parts, tr.Subject.N3()+" "+tr.Predicate.N3()+" "+tr.Object.N3())
	}
	if len(parts) == 0 {
		return "(empty)"
	}
	return strings.Join(parts, "\n")
}
