package jsonld_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/provenance"
	"github.com/tggo/goRDFlib/term"

	"github.com/piprate/json-gold/ld"
)

// exampleContext is a context definition in the shape json-gold decodes JSON
// into, which is also what a caller building one by hand writes.
func exampleContext() map[string]any {
	return map[string]any{
		"ex":   "http://example.org/",
		"name": "http://example.org/name",
	}
}

// TestExpandContextSuppliesMissingTerms is the case the option exists for: a
// document that relies on terms it never declares.
func TestExpandContextSuppliesMissingTerms(t *testing.T) {
	const doc = `{"@id": "ex:thing", "name": "Thing"}`

	// Without the context nothing is recognised: "ex:thing" is a relative-ish
	// IRI and "name" is not a term, so json-gold drops the property.
	bare := graph.NewGraph()
	if err := jsonld.Parse(bare, strings.NewReader(doc)); err != nil {
		t.Fatal(err)
	}
	if bare.Len() != 0 {
		t.Fatalf("without an expand context got %d triples, want 0", bare.Len())
	}

	g := graph.NewGraph()
	if err := jsonld.Parse(g, strings.NewReader(doc), jsonld.WithExpandContext(exampleContext())); err != nil {
		t.Fatal(err)
	}
	want := term.Triple{
		Subject:   term.NewURIRefUnsafe("http://example.org/thing"),
		Predicate: term.NewURIRefUnsafe("http://example.org/name"),
		Object:    term.NewLiteral("Thing"),
	}
	if g.Len() != 1 || !g.Contains(want.Subject, want.Predicate, want.Object) {
		t.Fatalf("with an expand context got %d triples and missing %v", g.Len(), want)
	}
}

// TestExpandContextDocumentWins pins the precedence the JSON-LD API specifies:
// the expand context is processed first, so the document's own @context
// overrides it term by term.
func TestExpandContextDocumentWins(t *testing.T) {
	const doc = `{
  "@context": {"name": "http://example.org/label"},
  "@id": "ex:thing",
  "name": "Thing",
  "ex:other": "kept from the expand context"
}`
	g := graph.NewGraph()
	if err := jsonld.Parse(g, strings.NewReader(doc), jsonld.WithExpandContext(exampleContext())); err != nil {
		t.Fatal(err)
	}
	s := term.NewURIRefUnsafe("http://example.org/thing")
	if !g.Contains(s, term.NewURIRefUnsafe("http://example.org/label"), term.NewLiteral("Thing")) {
		t.Error("document's own definition of \"name\" did not take precedence")
	}
	if g.Contains(s, term.NewURIRefUnsafe("http://example.org/name"), nil) {
		t.Error("expand context's definition of \"name\" leaked past the document's")
	}
	if !g.Contains(s, term.NewURIRefUnsafe("http://example.org/other"), nil) {
		t.Error("prefix from the expand context was lost once the document declared its own context")
	}
}

// TestExpandContextForms covers the value shapes the option accepts: a bare
// context, a document wrapping one under "@context", an array, and an IRI that
// goes through the document loader.
func TestExpandContextForms(t *testing.T) {
	const doc = `{"@id": "ex:thing", "name": "Thing"}`
	loader := &staticLoader{docs: map[string]any{
		"http://example.org/context.jsonld": map[string]any{"@context": exampleContext()},
	}}
	cases := []struct {
		name string
		ctx  any
	}{
		{"bare context", exampleContext()},
		{"wrapped in a document", map[string]any{"@context": exampleContext()}},
		{"array of contexts", []any{map[string]any{"ex": "http://example.org/"}, map[string]any{"name": "ex:name"}}},
		{"IRI through the document loader", "http://example.org/context.jsonld"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := graph.NewGraph()
			err := jsonld.Parse(g, strings.NewReader(doc),
				jsonld.WithExpandContext(tc.ctx), jsonld.WithDocumentLoader(loader))
			if err != nil {
				t.Fatal(err)
			}
			if !g.Contains(term.NewURIRefUnsafe("http://example.org/thing"), term.NewURIRefUnsafe("http://example.org/name"), term.NewLiteral("Thing")) {
				t.Fatalf("triple not produced; graph has %d triples", g.Len())
			}
		})
	}
}

// TestExpandContextInvalidIsAnError: a broken expand context must fail the
// parse, not be silently ignored, and must not panic.
func TestExpandContextInvalidIsAnError(t *testing.T) {
	const doc = `{"@id": "http://example.org/thing", "http://example.org/name": "Thing"}`
	for _, ctx := range []any{
		map[string]any{"@context": map[string]any{"name": map[string]any{"@id": 42}}}, // @id must be a string
		"http://example.org/unresolvable.jsonld",                                      // the loader has nothing
		[]any{map[string]any{"@base": 7}},                                             // @base must be a string or null
	} {
		g := graph.NewGraph()
		err := jsonld.Parse(g, strings.NewReader(doc),
			jsonld.WithExpandContext(ctx), jsonld.WithDocumentLoader(&staticLoader{}))
		if err == nil {
			t.Errorf("expand context %v: no error, graph has %d triples", ctx, g.Len())
		}
		if errors.Is(err, jsonld.ErrProcessorPanic) {
			t.Errorf("expand context %v: processor panicked: %v", ctx, err)
		}
	}
}

// TestExpandContextProvenance: identifiers that only expand given the expand
// context still get their source line, and an @id alias declared there is seen.
func TestExpandContextProvenance(t *testing.T) {
	const doc = `[
  {"@id": "ex:a", "name": "A"},
  {
    "identifier": "ex:b",
    "name": "B"
  }
]`
	ctx := exampleContext()
	ctx["identifier"] = "@id"

	g := graph.NewGraph()
	idx := provenance.NewIndex()
	err := jsonld.Parse(g, strings.NewReader(doc),
		jsonld.WithExpandContext(ctx), jsonld.WithProvenance(idx.Triple))
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Fatalf("got %d triples, want 2", g.Len())
	}
	name := term.NewURIRefUnsafe("http://example.org/name")
	for _, tc := range []struct {
		local string
		line  int
	}{{"a", 2}, {"b", 3}} {
		s := term.NewURIRefUnsafe("http://example.org/" + tc.local)
		line, ok := idx.Line(s, name, term.NewLiteral(strings.ToUpper(tc.local)))
		if !ok || line != tc.line {
			t.Errorf("ex:%s: line %d, found %v; want %d", tc.local, line, ok, tc.line)
		}
	}
}

// TestExpandContextIgnoredBySerializer: the option is a parsing option and must
// not change or break serialization when passed along with a caller's other
// options.
func TestExpandContextIgnoredBySerializer(t *testing.T) {
	g := graph.NewGraph()
	g.Add(term.NewURIRefUnsafe("http://example.org/thing"), term.NewURIRefUnsafe("http://example.org/name"), term.NewLiteral("Thing"))
	var with, without strings.Builder
	if err := jsonld.Serialize(g, &without, jsonld.WithExpanded()); err != nil {
		t.Fatal(err)
	}
	if err := jsonld.Serialize(g, &with, jsonld.WithExpanded(), jsonld.WithExpandContext(exampleContext())); err != nil {
		t.Fatal(err)
	}
	if with.String() != without.String() {
		t.Errorf("serializer output changed with WithExpandContext:\n%s\nvs\n%s", with.String(), without.String())
	}
}

// staticLoader serves contexts from memory and errors on anything else, so no
// test reaches the network.
type staticLoader struct {
	docs map[string]any
}

func (l *staticLoader) LoadDocument(u string) (*ld.RemoteDocument, error) {
	doc, ok := l.docs[u]
	if !ok {
		return nil, ld.NewJsonLdError(ld.LoadingRemoteContextFailed, u)
	}
	return &ld.RemoteDocument{DocumentURL: u, Document: doc}, nil
}
