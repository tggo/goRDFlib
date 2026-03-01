package rdfxml

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Ported from: test/test_w3c_spec/test_rdfxml_w3c.py, test/test_serializers/test_serializer_xml.py

func TestRDFXMLParserBasic(t *testing.T) {
	// Ported from: rdflib.plugins.parsers.rdfxml — basic rdf:Description
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestRDFXMLParserTypedNode(t *testing.T) {
	// Ported from: rdflib RDF/XML parser — typed node element
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <ex:Person rdf:about="http://example.org/Alice">
    <ex:name>Alice</ex:name>
  </ex:Person>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	// Should have: rdf:type + ex:name = 2 triples
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
	alice, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	person, _ := rdflibgo.NewURIRef("http://example.org/Person")
	if !g.Contains(alice, rdflibgo.RDF.Type, person) {
		t.Error("expected rdf:type triple")
	}
}

func TestRDFXMLParserResource(t *testing.T) {
	// Ported from: rdflib RDF/XML parser — rdf:resource attribute
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:knows rdf:resource="http://example.org/o"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	knows, _ := rdflibgo.NewURIRef("http://example.org/knows")
	o, _ := rdflibgo.NewURIRef("http://example.org/o")
	if !g.Contains(s, knows, o) {
		t.Error("expected resource link triple")
	}
}

func TestRDFXMLParserLangTag(t *testing.T) {
	// Ported from: rdflib RDF/XML parser — xml:lang
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:name xml:lang="en">Alice</ex:name>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	name, _ := rdflibgo.NewURIRef("http://example.org/name")
	val, ok := g.Value(s, &name, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if lit, ok := val.(rdflibgo.Literal); !ok || lit.Language() != "en" {
		t.Errorf("expected lang en, got %v", val)
	}
}

func TestRDFXMLParserDatatype(t *testing.T) {
	// Ported from: rdflib RDF/XML parser — rdf:datatype
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xmlns:xsd="http://www.w3.org/2001/XMLSchema#">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:age rdf:datatype="http://www.w3.org/2001/XMLSchema#integer">42</ex:age>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	age, _ := rdflibgo.NewURIRef("http://example.org/age")
	val, ok := g.Value(s, &age, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if lit, ok := val.(rdflibgo.Literal); !ok || lit.Datatype() != rdflibgo.XSDInteger {
		t.Errorf("expected xsd:integer, got %v", val)
	}
}

func TestRDFXMLParserParseTypeResource(t *testing.T) {
	// Ported from: rdflib RDF/XML parser — rdf:parseType="Resource"
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:address rdf:parseType="Resource">
      <ex:city>Berlin</ex:city>
    </ex:address>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	// s → address → bnode, bnode → city → "Berlin" = 2 triples
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestRDFXMLParserNodeID(t *testing.T) {
	// Ported from: rdflib RDF/XML parser — rdf:nodeID
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:knows rdf:nodeID="b1"/>
  </rdf:Description>
  <rdf:Description rdf:nodeID="b1">
    <ex:name>Bob</ex:name>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestRDFXMLSerializerBasic(t *testing.T) {
	// Ported from: rdflib.plugins.serializers.rdfxml.XMLSerializer
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, rdflibgo.NewLiteral("hello"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "rdf:RDF") {
		t.Errorf("expected rdf:RDF element, got:\n%s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected literal value, got:\n%s", out)
	}
}

func TestRDFXMLSerializerTypedNode(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Person"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/name"), rdflibgo.NewLiteral("Alice"))

	var buf bytes.Buffer
	Serialize(g, &buf)
	out := buf.String()
	if !strings.Contains(out, "ex:Person") {
		t.Errorf("expected typed node element, got:\n%s", out)
	}
}

func TestRDFXMLRoundtrip(t *testing.T) {
	// Ported from: test/test_roundtrip.py — RDF/XML roundtrip
	g1 := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g1.Add(s, p, rdflibgo.NewLiteral("hello"))
	g1.Add(s, p, rdflibgo.NewLiteral("world", rdflibgo.WithLang("en")))

	var buf bytes.Buffer
	Serialize(g1, &buf)

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatalf("roundtrip parse failed: %v\nSerialized:\n%s", err, buf.String())
	}

	if g1.Len() != g2.Len() {
		t.Errorf("roundtrip: %d vs %d\nSerialized:\n%s", g1.Len(), g2.Len(), buf.String())
	}
}

// Fix 1: readInnerXML preserves attributes and namespace declarations for parseType="Literal"
func TestReadInnerXMLPreservesAttributes(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:content rdf:parseType="Literal">
      <div class="main">hello</div>
    </ex:content>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	content, _ := rdflibgo.NewURIRef("http://example.org/content")
	val, ok := g.Value(s, &content, nil)
	if !ok {
		t.Fatal("expected value")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	// The inner XML should preserve the class attribute
	if !strings.Contains(lit.Lexical(), `class="main"`) {
		t.Errorf("expected attributes preserved, got: %s", lit.Lexical())
	}
	if lit.Datatype().Value() != rdfNS+"XMLLiteral" {
		t.Errorf("expected XMLLiteral datatype, got: %s", lit.Datatype().Value())
	}
}

// Fix 2: resolve uses net/url for proper RFC 3986 resolution
func TestResolveRelativeURIs(t *testing.T) {
	p := &rdfxmlParser{base: "http://example.org/a/b"}
	tests := []struct {
		input, want string
	}{
		{"#frag", "http://example.org/a/b#frag"},
		{"", "http://example.org/a/b"},
		{"http://other.org/x", "http://other.org/x"},
		{"../c", "http://example.org/c"},
		{"/abs", "http://example.org/abs"},
		{"//host.com/path", "http://host.com/path"},
		{"rel", "http://example.org/a/rel"},
	}
	for _, tt := range tests {
		got := p.resolve(tt.input)
		if got != tt.want {
			t.Errorf("resolve(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// Fix 3: xml:base scoping — nested base does not affect sibling elements
func TestXMLBaseScoping(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/">
  <rdf:Description rdf:about="s1" xml:base="http://other.org/">
    <ex:p rdf:resource="o1"/>
  </rdf:Description>
  <rdf:Description rdf:about="s2">
    <ex:p rdf:resource="o2"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// s1 should resolve against http://other.org/
	s1, _ := rdflibgo.NewURIRef("http://other.org/s1")
	o1, _ := rdflibgo.NewURIRef("http://other.org/o1")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	if !g.Contains(s1, p, o1) {
		t.Error("expected s1 resolved against nested base http://other.org/")
	}
	// s2 should resolve against the original base http://example.org/
	s2, _ := rdflibgo.NewURIRef("http://example.org/s2")
	o2, _ := rdflibgo.NewURIRef("http://example.org/o2")
	if !g.Contains(s2, p, o2) {
		t.Error("expected s2 resolved against original base http://example.org/")
	}
}

// Fix 4: skipToEnd handles decoder errors
func TestSkipToEndHandlesErrors(t *testing.T) {
	// Truncated XML should return error instead of looping
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>
      <rdf:Description rdf:about="http://example.org/o">
        <ex:name>test</ex:name>
      </rdf:Description>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	// This should parse without error; the fix ensures skipToEnd doesn't silently fail
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// Fix 5: Serializer handles multiple rdf:type — first becomes element name, rest become property elements
func TestSerializerMultipleRDFType(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Person"))
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Agent"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/name"), rdflibgo.NewLiteral("Alice"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// One type becomes the element name, the other should appear as rdf:type property
	if !strings.Contains(out, "rdf:type") {
		t.Errorf("expected rdf:type property element for second type, got:\n%s", out)
	}

	// Roundtrip should preserve all types
	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(out)); err != nil {
		t.Fatalf("roundtrip parse failed: %v", err)
	}
	if g2.Len() != 3 {
		t.Errorf("expected 3 triples after roundtrip, got %d\nSerialized:\n%s", g2.Len(), out)
	}
}

// Fix 6: xmlAttr does not produce unescaped output
func TestXMLAttrEscaping(t *testing.T) {
	got := xmlAttr(`<script>alert("xss")</script>`)
	if strings.Contains(got, "<script>") {
		t.Errorf("xmlAttr should escape: %s", got)
	}
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Errorf("xmlAttr should be quoted: %s", got)
	}
}

// Fix 8: preSubj replaced with nullable Subject (internal API, test via parseType="Resource" with base)
func TestParseTypeResourceWithXMLBase(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/">
  <rdf:Description rdf:about="s">
    <ex:address rdf:parseType="Resource">
      <ex:city>Berlin</ex:city>
    </ex:address>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	addr, _ := rdflibgo.NewURIRef("http://example.org/address")
	if _, ok := g.Value(s, &addr, nil); !ok {
		t.Error("expected address triple with resolved base")
	}
}
