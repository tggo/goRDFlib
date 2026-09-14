package jsonld

import (
	"bytes"
	"encoding/json"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/testutil"
)

func outputContext(t *testing.T, out []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, out)
	}
	ctx, _ := doc["@context"].(map[string]any)
	return ctx
}

// rdflib #2747/#3373: an empty-prefix binding put {"": "http://e/"} into the
// context and the output read back as zero triples, without an error.
func TestSerializeEmptyPrefixRoundTrips(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("", rdflibgo.NewURIRefUnsafe("http://e/"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://e/s"), rdflibgo.NewURIRefUnsafe("http://e/p"), rdflibgo.NewLiteral("v"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if _, ok := outputContext(t, buf.Bytes())[""]; ok {
		t.Errorf("empty term in context:\n%s", buf.String())
	}
	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("%v\n%s", err, buf.String())
	}
	testutil.AssertGraphEqual(t, g, g2)
	if t.Failed() {
		t.Logf("output:\n%s", buf.String())
	}
}

func TestSerializeContextHasOnlyUsedPrefixes(t *testing.T) {
	g := rdflibgo.NewGraph() // binds rdf, rdfs, xsd, owl
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Bind("unused", rdflibgo.NewURIRefUnsafe("http://unused.example/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/C"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/n"), rdflibgo.NewLiteral("1", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/l"), rdflibgo.NewLiteral("x", rdflibgo.WithLang("en")))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	ctx := outputContext(t, buf.Bytes())
	for _, want := range []string{"ex", "xsd"} {
		if _, ok := ctx[want]; !ok {
			t.Errorf("used prefix %q missing from context:\n%s", want, buf.String())
		}
	}
	for _, unwanted := range []string{"unused", "owl", "rdfs", "rdf"} {
		if _, ok := ctx[unwanted]; ok {
			t.Errorf("unused prefix %q in context:\n%s", unwanted, buf.String())
		}
	}
	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	testutil.AssertGraphEqual(t, g, g2)
}
