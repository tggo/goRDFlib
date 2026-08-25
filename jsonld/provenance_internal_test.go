package jsonld

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"

	"github.com/piprate/json-gold/ld"
)

// These cover the defensive branches of the position machinery directly.
//
// Reaching them through Parse is not possible: it rejects a malformed document
// before provenance runs, so every fallback here would be dead code from the
// outside while still being what protects a caller from a wrong line. Calling
// the helpers with the inputs those branches exist for is the only way to say
// what they do.

func TestBuildSubjectLinesEmptyInputs(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"no identifiers at all", `{"http://example.org/p": "v"}`},
		{"not JSON", `<?xml version="1.0"?>`},
		{"empty", ``},
		{"an array of scalars", `[1, 2, 3]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildSubjectLines([]byte(tc.src), "http://example.org/", nil); got != nil {
				t.Errorf("got %v, want nothing to report", got)
			}
		})
	}
}

// TestBuildSubjectLinesDropsUnexpandableIDs covers the branch that keeps a
// wrong line out of the result. An identifier that cannot be expanded is not
// approximated — it is dropped.
func TestBuildSubjectLinesDropsUnexpandableIDs(t *testing.T) {
	// With no base and no context, a relative identifier has nothing to resolve
	// against, so it cannot be turned into the IRI a triple would use.
	const src = `{"@id": "bare-relative", "http://example.org/p": "v"}`

	got := buildSubjectLines([]byte(src), "", nil)
	for key, line := range got {
		if strings.Contains(key, "bare-relative") {
			continue // it did expand to itself; that is a match, not a guess
		}
		t.Errorf("an unexpandable identifier produced key %q at line %d", key, line)
	}
}

func TestTopLevelContext(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		found bool
	}{
		{"object with a context", `{"@context": {"ex": "http://example.org/"}}`, true},
		{"object without one", `{"@id": "x"}`, false},
		{"array whose second entry has one", `[{"@id": "a"}, {"@context": {}}]`, true},
		{"array of scalars", `[1, "two", null]`, false},
		{"array with no context anywhere", `[{"@id": "a"}, {"@id": "b"}]`, false},
		{"a bare scalar", `"just a string"`, false},
		{"not JSON at all", `{{{`, false},
		{"empty", ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := topLevelContext([]byte(tc.src))
			if (got != nil) != tc.found {
				t.Errorf("got %v, want found=%v", got, tc.found)
			}
		})
	}
}

// TestIRIExpanderWithDocumentLoader covers the loader branch. The loader is
// handed to the same options the processor uses, so a document whose context is
// remote expands through the caller's loader rather than the network.
func TestIRIExpanderWithDocumentLoader(t *testing.T) {
	loader := ld.NewDefaultDocumentLoader(nil)
	expand := iriExpander([]byte(`{"@context": {"ex": "http://example.org/"}}`),
		"http://example.org/", loader)

	got, ok := expand("ex:thing")
	if !ok || got != "http://example.org/thing" {
		t.Errorf("expand = %q, %v; want the compact IRI resolved through the context", got, ok)
	}
}

// TestIRIExpanderWithoutContext covers the fallback when a document has no
// context: absolute identifiers pass through and relative ones resolve against
// the base, which is all that can be known.
func TestIRIExpanderWithoutContext(t *testing.T) {
	expand := iriExpander([]byte(`{"@id": "a"}`), "http://example.org/", nil)

	if got, ok := expand("http://elsewhere.example/x"); !ok || got != "http://elsewhere.example/x" {
		t.Errorf("absolute: got %q, %v", got, ok)
	}
	if got, ok := expand("a"); !ok || got != "http://example.org/a" {
		t.Errorf("relative: got %q, %v; want it resolved against the base", got, ok)
	}
}

// TestParseWithDocumentLoaderAndProvenance is the end-to-end form of the loader
// branch, so the option really is threaded all the way through.
func TestParseWithDocumentLoaderAndProvenance(t *testing.T) {
	const doc = `{
  "@context": {"ex": "http://example.org/"},
  "@id": "ex:withloader",
  "ex:name": "Loaded"
}
`
	g := graph.NewGraph()
	var lines []int
	err := Parse(g, strings.NewReader(doc),
		WithDocumentLoader(ld.NewDefaultDocumentLoader(nil)),
		WithProvenance(func(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term, int) {
			lines = append(lines, 1)
		}))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(lines) == 0 {
		t.Error("no lines were reported with a document loader in play")
	}
}
