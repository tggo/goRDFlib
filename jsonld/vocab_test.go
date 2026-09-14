package jsonld

import (
	"reflect"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func mustParse(t *testing.T, doc string, opts ...Option) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(doc), opts...); err != nil {
		t.Fatalf("%v\n%s", err, doc)
	}
	return g
}

func hasTriple(g *rdflibgo.Graph, s, p string, o rdflibgo.Term) bool {
	return g.Contains(rdflibgo.NewURIRefUnsafe(s), rdflibgo.NewURIRefUnsafe(p), o)
}

// rdflib #2167: "@vocab": "#" with base http://example/document expanded name
// to http://example/documentname instead of http://example/document#name.
func TestParseRelativeVocabKeepsEmptyFragment(t *testing.T) {
	g := mustParse(t, `{"@context":{"@version":1.1,"@base":"http://example/document","@vocab":"#"},`+
		`"@id":"BrewEats","@type":"Restaurant","name":"Brew Eats"}`)
	if !hasTriple(g, "http://example/BrewEats", "http://example/document#name", rdflibgo.NewLiteral("Brew Eats")) {
		t.Errorf("name not under http://example/document#: %d triples", g.Len())
	}
	if !hasTriple(g, "http://example/BrewEats", rdfNSType, rdflibgo.NewURIRefUnsafe("http://example/document#Restaurant")) {
		t.Error("type not under http://example/document#")
	}
}

const rdfNSType = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"

func TestParseRelativeVocabAgainstOptionBase(t *testing.T) {
	g := mustParse(t, `{"@context":{"@vocab":"#"},"@id":"x","name":"n"}`, WithBase("http://example/document"))
	if !hasTriple(g, "http://example/x", "http://example/document#name", rdflibgo.NewLiteral("n")) {
		t.Errorf("got %d triples, want name under the option base", g.Len())
	}
}

func TestParseRelativeVocabInNestedContext(t *testing.T) {
	g := mustParse(t, `{"@context":{"@base":"http://a.example/doc"},"@id":"http://a.example/s",`+
		`"http://a.example/child":{"@context":{"@base":"other","@vocab":"v#"},"@id":"c","name":"n"}}`)
	if !hasTriple(g, "http://a.example/c", "http://a.example/v#name", rdflibgo.NewLiteral("n")) {
		t.Errorf("nested @vocab not resolved against the nested base (%d triples)", g.Len())
	}
}

func TestParseRelativeVocabInExpandContext(t *testing.T) {
	ec := map[string]any{"@base": "http://example/document", "@vocab": "#"}
	orig := map[string]any{"@base": "http://example/document", "@vocab": "#"}
	g := mustParse(t, `{"@id":"x","name":"n"}`, WithExpandContext(ec))
	if !hasTriple(g, "http://example/x", "http://example/document#name", rdflibgo.NewLiteral("n")) {
		t.Errorf("expand context @vocab not resolved (%d triples)", g.Len())
	}
	if !reflect.DeepEqual(ec, orig) {
		t.Errorf("caller's expand context was modified: %v", ec)
	}
}

// The workaround touches only relative @vocab values ending in "#".
func TestResolveEmptyFragmentVocabLeavesOthersAlone(t *testing.T) {
	for _, vocab := range []string{"http://v.example/#", "ex:#", "_:b#", "term#", "path/", ""} {
		ctx := map[string]any{"@vocab": vocab, "term#": "http://t.example/"}
		resolveEmptyFragmentVocab(map[string]any{"@context": ctx}, "http://example/document")
		if ctx["@vocab"] != vocab {
			t.Errorf("@vocab %q rewritten to %q", vocab, ctx["@vocab"])
		}
	}
	noBase := map[string]any{"@vocab": "#"}
	resolveEmptyFragmentVocab(map[string]any{"@context": noBase}, "")
	if noBase["@vocab"] != "#" {
		t.Errorf("@vocab rewritten without a base: %q", noBase["@vocab"])
	}
}
