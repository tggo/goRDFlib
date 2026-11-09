package reasoning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	w3c "github.com/tggo/goRDFlib/testdata/w3c"
	"github.com/tggo/goRDFlib/turtle"
)

const (
	manifestPath   = "../testdata/w3c/rdf-tests/rdf/rdf11/rdf-mt/manifest.ttl"
	manifestPath12 = "../testdata/w3c/rdf-tests/rdf/rdf12/rdf-semantics/manifest.ttl"
	mfPositive     = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#PositiveEntailmentTest"
	mfNegative     = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#NegativeEntailmentTest"
	minRDFMT       = 39 // rdf11/rdf-mt entries at the oldest supported pin (48 at rdf-tests 202a74e)
	minRDF12Sem    = 29 // rdf12/rdf-semantics entries (its manifest before 202a74e is not valid Turtle)
)

// w3cKnown lists entailment tests the checker does not pass, each with the
// reason. A listed test that starts passing fails the run, so the list can
// only shrink. It is empty: every test of both suites runs under the regime
// and datatype set its manifest declares.
var w3cKnown = map[string]string{}

func TestW3C_Entailment_RDF11(t *testing.T) {
	runEntailmentManifest(t, manifestPath, minRDFMT)
}

func TestW3C_Entailment_RDF12(t *testing.T) {
	runEntailmentManifest(t, manifestPath12, minRDF12Sem)
}

// runEntailmentManifest runs every test of a manifest under the entailment
// regime and the recognized datatypes the test declares (README of
// rdf-tests/rdf/rdf11/rdf-mt: a positive test passes if the premise entails
// the conclusion, or is inconsistent when the conclusion is false; a negative
// test is the reverse). Nothing is skipped: an unknown test type or regime
// fails the test.
func runEntailmentManifest(t *testing.T, path string, atLeast int) {
	t.Helper()
	m, err := w3c.ParseManifest(path)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}
	// A manifest that shrinks means tests were lost, whether by an upstream
	// change or by the parser dropping entries: either way the run must not
	// pass quietly.
	if len(m.Entries) < atLeast {
		t.Fatalf("manifest %s has %d entries, fewer than the %d the runner has seen", path, len(m.Entries), atLeast)
	}
	for _, e := range m.Entries {
		t.Run(e.Name, func(t *testing.T) {
			got := runEntailmentTest(t, e)
			if reason, known := w3cKnown[e.Name]; known {
				if got {
					t.Fatalf("listed as failing (%s) but passes: remove it from w3cKnown", reason)
				}
				t.Logf("known failure: %s", reason)
				return
			}
			if !got {
				t.Fail()
			}
		})
	}
}

func runEntailmentTest(t *testing.T, e w3c.TestEntry) bool {
	t.Helper()
	var sem Semantics
	switch e.EntailmentRegime {
	case "simple":
		sem = SimpleEntailment
	case "RDF":
		sem = RDFEntailment
	case "RDFS":
		sem = RDFSEntailment
	default:
		t.Fatalf("unknown entailment regime %q", e.EntailmentRegime)
	}
	for _, d := range e.UnrecognizedDatatypes {
		for _, r := range e.RecognizedDatatypes {
			if d == r {
				t.Fatalf("datatype %s is both recognized and unrecognized", d)
			}
		}
	}
	opts := []EntailOption{WithSemantics(sem), WithRecognizedDatatypes(e.RecognizedDatatypes...)}

	var positive bool
	switch e.Type {
	case mfPositive:
		positive = true
	case mfNegative:
	default:
		t.Fatalf("unknown test type %s", e.Type)
	}
	premise := loadGraph(t, e.Action)

	if e.ResultFalse {
		ok, err := Satisfiable(t.Context(), premise, opts...)
		if err != nil {
			t.Fatalf("Satisfiable: %v", err)
		}
		if positive && ok {
			t.Error("premise is satisfiable, but the test expects it to be inconsistent")
		}
		if !positive && !ok {
			t.Error("premise is inconsistent, but the test expects it to be consistent")
		}
		return ok != positive
	}
	if e.Result == "" {
		t.Fatalf("test has neither a result graph nor mf:result false")
	}
	conclusion := loadGraph(t, e.Result)
	ok, err := Entails(t.Context(), premise, conclusion, opts...)
	if err != nil {
		t.Fatalf("Entails: %v", err)
	}
	if positive && !ok {
		t.Error("premise does not entail the conclusion")
	}
	if !positive && ok {
		t.Error("premise entails the conclusion, but it must not")
	}
	return ok == positive
}

// loadGraph parses a .nt or .ttl file into a graph.
func loadGraph(t *testing.T, path string) *graph.Graph {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("cannot open %s: %v", path, err)
	}
	defer f.Close()

	g := graph.NewGraph()
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".nt":
		if err := nt.Parse(t.Context(), g, f); err != nil {
			t.Fatalf("parse error %s: %v", path, err)
		}
	case ".ttl":
		if err := turtle.Parse(t.Context(), g, f, turtle.WithBase("file://"+path)); err != nil {
			t.Fatalf("parse error %s: %v", path, err)
		}
	default:
		t.Fatalf("unsupported format: %s", ext)
	}
	return g
}
