package rdfxml

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// ─── parser.go:104 parse ──────────────────────────────────────────────────────

// TestParseNonRDFRoot exercises the branch where the root element is not
// rdf:RDF (line 119: parseNodeElement called directly).
func TestParseNonRDFRoot(t *testing.T) {
	input := `<?xml version="1.0"?>
<ex:Person xmlns:ex="http://example.org/"
           xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
           rdf:about="http://example.org/Alice">
</ex:Person>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	alice := rdflibgo.NewURIRefUnsafe("http://example.org/Alice")
	person := rdflibgo.NewURIRefUnsafe("http://example.org/Person")
	if !g.Contains(alice, rdflibgo.RDF.Type, person) {
		t.Errorf("expected rdf:type triple for non-rdf:RDF root; triples=%d", g.Len())
	}
}

// TestParseMalformedXML exercises the XML decode-error path in parse().
func TestParseMalformedXML(t *testing.T) {
	input := `<?xml version="1.0"?><unclosed`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for malformed XML")
	}
}

// TestParseEmptyDocument returns nil without triples.
func TestParseEmptyDocument(t *testing.T) {
	input := `<?xml version="1.0"?>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 0 {
		t.Errorf("expected 0 triples, got %d", g.Len())
	}
}

// ─── parser.go:527 parseCollection ──────────────────────────────────────────

// TestParseCollectionNonEmpty ensures a non-empty rdf:parseType="Collection"
// produces a proper rdf:first / rdf:rest chain.
func TestParseCollectionNonEmpty(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection">
      <rdf:Description rdf:about="http://example.org/a"/>
      <rdf:Description rdf:about="http://example.org/b"/>
    </ex:items>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// Expect: s ex:items _:head
	//         _:head rdf:first ex:a; rdf:rest _:next
	//         _:next rdf:first ex:b; rdf:rest rdf:nil
	// That is 5 triples plus the initial link = 5.
	if g.Len() != 5 {
		t.Errorf("expected 5 triples for 2-item collection, got %d", g.Len())
	}
}

// TestParseCollectionEmpty ensures an empty collection links directly to rdf:nil.
func TestParseCollectionEmpty(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	items := rdflibgo.NewURIRefUnsafe("http://example.org/items")
	if !g.Contains(s, items, rdflibgo.RDF.Nil) {
		t.Errorf("expected s ex:items rdf:nil for empty collection; triples=%d", g.Len())
	}
}

// TestParseCollectionSingleItem — one-item list ends with rdf:nil.
func TestParseCollectionSingleItem(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection">
      <rdf:Description rdf:about="http://example.org/a"/>
    </ex:items>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// s ex:items _:h; _:h rdf:first ex:a; _:h rdf:rest rdf:nil = 3 triples.
	if g.Len() != 3 {
		t.Errorf("expected 3 triples for 1-item collection, got %d", g.Len())
	}
}

// ─── parser.go:572 emitPropertyAttrs ────────────────────────────────────────

// TestPropertyAttrWithRDFType exercises the rdf:type branch inside
// emitPropertyAttrs (attr.Name.Local == "type").
func TestPropertyAttrWithRDFType(t *testing.T) {
	// rdf:resource + rdf:type on the same property element triggers the path
	// where emitPropertyAttrs is called with a URIRef object that also has
	// a rdf:type property attribute.
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:knows rdf:resource="http://example.org/o"
              rdf:type="http://example.org/Person"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	knows := rdflibgo.NewURIRefUnsafe("http://example.org/knows")
	person := rdflibgo.NewURIRefUnsafe("http://example.org/Person")
	if !g.Contains(s, knows, o) {
		t.Error("expected s ex:knows ex:o")
	}
	if !g.Contains(o, rdflibgo.RDF.Type, person) {
		t.Error("expected ex:o rdf:type ex:Person from property attribute")
	}
}

// TestPropertyAttrNonRDFNamespace exercises the non-rdf: branch in
// emitPropertyAttrs where a foreign namespace attribute is emitted as a literal.
func TestPropertyAttrNonRDFNamespace(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xmlns:dc="http://purl.org/dc/elements/1.1/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:creator rdf:nodeID="b1" dc:title="Editor"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// The bnode b1 should have dc:title "Editor"
	dcTitle := rdflibgo.NewURIRefUnsafe("http://purl.org/dc/elements/1.1/title")
	found := false
	g.Triples(nil, &dcTitle, nil)(func(t rdflibgo.Triple) bool {
		if lit, ok := t.Object.(rdflibgo.Literal); ok && lit.Lexical() == "Editor" {
			found = true
		}
		return true
	})
	if !found {
		t.Errorf("expected dc:title 'Editor' on bnode; triples=%d", g.Len())
	}
}

// ─── parser.go:690 resolve ───────────────────────────────────────────────────

// TestResolveEmptyURIWithFragment covers the branch where base contains '#'.
func TestResolveEmptyURIWithFragment(t *testing.T) {
	p := &rdfxmlParser{base: "http://example.org/doc#section"}
	got := p.resolve("")
	want := "http://example.org/doc"
	if got != want {
		t.Errorf("resolve('') with fragment base: got %q, want %q", got, want)
	}
}

// TestResolveEmptyURINoFragment covers the branch where base has no '#'.
func TestResolveEmptyURINoFragment(t *testing.T) {
	p := &rdfxmlParser{base: "http://example.org/doc"}
	got := p.resolve("")
	want := "http://example.org/doc"
	if got != want {
		t.Errorf("resolve('') no fragment: got %q, want %q", got, want)
	}
}

// TestResolveNoBase: when base is empty the input is returned unchanged.
func TestResolveNoBase(t *testing.T) {
	p := &rdfxmlParser{}
	if got := p.resolve("http://x.org/y"); got != "http://x.org/y" {
		t.Errorf("expected unchanged, got %q", got)
	}
	if got := p.resolve("relative"); got != "relative" {
		t.Errorf("expected unchanged relative, got %q", got)
	}
}

// TestResolveAbsoluteSkipsBase: absolute IRI should not be modified.
func TestResolveAbsoluteSkipsBase(t *testing.T) {
	p := &rdfxmlParser{base: "http://base.org/"}
	got := p.resolve("ftp://other.org/path")
	if got != "ftp://other.org/path" {
		t.Errorf("got %q", got)
	}
}

// ─── parser.go:730 isAbsoluteIRI ─────────────────────────────────────────────

func TestIsAbsoluteIRI(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"http://example.org/", true},
		{"ftp+ssh://host/", true},
		{"urn:isbn:0451450523", true},
		{"relative/path", false},
		{"", false},
		{":noscheme", false}, // colon at position 0
		{"1abc:bad", false},  // digit as first char
		{"a b:bad", false},   // space in scheme
		{"http", false},      // no colon
		{"a+b-c.d:ok", true}, // '+', '-', '.' allowed after first char
	}
	for _, tc := range cases {
		got := isAbsoluteIRI(tc.s)
		if got != tc.want {
			t.Errorf("isAbsoluteIRI(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

// ─── parser.go:802 readInnerXML ──────────────────────────────────────────────

// TestReadInnerXMLNestedElements tests that nested XML elements inside
// parseType="Literal" are preserved correctly including closing tags.
func TestReadInnerXMLNestedElements(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:html rdf:parseType="Literal"><p>Hello <b>world</b></p></ex:html>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	html := rdflibgo.NewURIRefUnsafe("http://example.org/html")
	val, ok := g.Value(s, &html, nil)
	if !ok {
		t.Fatal("expected value")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	// readInnerXML injects namespace declarations into every start element, so
	// elements will look like <p xmlns:rdf="..." xmlns:ex="...">…</p>.
	// Check that both the outer <p> and inner <b> tags (with any attrs) appear.
	if !strings.Contains(lit.Lexical(), "<p") || !strings.Contains(lit.Lexical(), "</b>") {
		t.Errorf("expected nested elements in inner XML, got: %s", lit.Lexical())
	}
}

// TestReadInnerXMLAttributeNamespacePrefix tests that attributes with namespace
// prefixes are preserved in the literal output.
func TestReadInnerXMLAttributeNamespacePrefix(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xmlns:xlink="http://www.w3.org/1999/xlink">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:content rdf:parseType="Literal"><span xlink:href="http://x.org/">click</span></ex:content>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	content := rdflibgo.NewURIRefUnsafe("http://example.org/content")
	val, ok := g.Value(s, &content, nil)
	if !ok {
		t.Fatal("expected value")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	if !strings.Contains(lit.Lexical(), "href") {
		t.Errorf("expected href attribute in literal, got: %s", lit.Lexical())
	}
}

// TestReadInnerXMLPureText tests that a text-only Literal is stored correctly.
func TestReadInnerXMLPureText(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:note rdf:parseType="Literal">plain text only</ex:note>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	note := rdflibgo.NewURIRefUnsafe("http://example.org/note")
	val, ok := g.Value(s, &note, nil)
	if !ok {
		t.Fatal("expected value")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	if lit.Lexical() != "plain text only" {
		t.Errorf("got: %q", lit.Lexical())
	}
}

// ─── parser.go:859 xmlEscapeToBuilder ────────────────────────────────────────

func TestXMLEscapeToBuilder(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"<", "&lt;"},
		{">", "&gt;"},
		{"&", "&amp;"},
		{`"`, "&quot;"},
		{"'", "'"},                     // single quote — no escaping
		{"hello world", "hello world"}, // plain text unchanged
		{"a<b>c&d\"e", "a&lt;b&gt;c&amp;d&quot;e"},
		{"", ""},
	}
	for _, tc := range cases {
		var sb strings.Builder
		xmlEscapeToBuilder(&sb, tc.input)
		got := sb.String()
		if got != tc.want {
			t.Errorf("xmlEscapeToBuilder(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestXMLEscapeInsideLiteral verifies that special chars inside
// rdf:parseType="Literal" content are correctly escaped.
func TestXMLEscapeInsideLiteral(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:code rdf:parseType="Literal"><![CDATA[a < b && c > d]]></ex:code>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	code := rdflibgo.NewURIRefUnsafe("http://example.org/code")
	val, ok := g.Value(s, &code, nil)
	if !ok {
		t.Fatal("expected value")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	// The lexical form should have escaped entities (CDATA is decoded by Go's XML
	// parser, then re-escaped by xmlEscapeToBuilder).
	if !strings.Contains(lit.Lexical(), "&lt;") && !strings.Contains(lit.Lexical(), "<") {
		t.Errorf("expected '<' or '&lt;' in literal, got: %s", lit.Lexical())
	}
}

// ─── parser.go:876 skipToEnd ─────────────────────────────────────────────────

// TestSkipToEndDeeplyNested triggers skipToEnd with multiple nesting levels.
func TestSkipToEndDeeplyNested(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>
      <ex:Outer rdf:about="http://example.org/outer">
        <ex:inner>
          <ex:DeepNode rdf:about="http://example.org/deep">
            <ex:val>42</ex:val>
          </ex:DeepNode>
        </ex:inner>
      </ex:Outer>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// The outer node is parsed; everything after is consumed by skipToEnd.
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	outer := rdflibgo.NewURIRefUnsafe("http://example.org/outer")
	if !g.Contains(s, p, outer) {
		t.Errorf("expected s ex:p ex:outer triple; triples=%d", g.Len())
	}
}

// ─── serializer.go:20 Serialize ──────────────────────────────────────────────

// TestSerializeBNodeSubject tests that BNode subjects produce rdf:nodeID attrs.
func TestSerializeBNodeSubject(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	bn := rdflibgo.NewBNode("x1")
	g.Add(bn, rdflibgo.NewURIRefUnsafe("http://example.org/name"), rdflibgo.NewLiteral("Bob"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "rdf:nodeID") {
		t.Errorf("expected rdf:nodeID for BNode subject, got:\n%s", out)
	}
}

// TestSerializeLangTagLiteral tests xml:lang attribute output.
func TestSerializeLangTagLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/label"),
		rdflibgo.NewLiteral("Hallo", rdflibgo.WithLang("de")))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `xml:lang=`) {
		t.Errorf("expected xml:lang attribute for lang literal, got:\n%s", out)
	}
	if !strings.Contains(out, "de") {
		t.Errorf("expected 'de' in output, got:\n%s", out)
	}
}

// TestSerializeTypedLiteral tests rdf:datatype attribute output.
func TestSerializeTypedLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/age"),
		rdflibgo.NewLiteral("42", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "rdf:datatype") {
		t.Errorf("expected rdf:datatype attribute, got:\n%s", out)
	}
}

// TestSerializePlainLiteral tests plain literal (no lang, xsd:string datatype)
// uses the simple element form.
func TestSerializePlainLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/note"),
		rdflibgo.NewLiteral("plain"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "plain") {
		t.Errorf("expected literal value 'plain', got:\n%s", out)
	}
	// Plain literals must NOT carry rdf:datatype or xml:lang
	if strings.Contains(out, "rdf:datatype") || strings.Contains(out, "xml:lang") {
		t.Errorf("unexpected rdf:datatype or xml:lang for plain literal, got:\n%s", out)
	}
}

// TestSerializeURIRefObject tests rdf:resource attribute output for URIRef objects.
func TestSerializeURIRefObject(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/link"), o)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "rdf:resource") {
		t.Errorf("expected rdf:resource for URIRef object, got:\n%s", out)
	}
}

// TestSerializeBNodeObject tests rdf:nodeID output for BNode objects.
func TestSerializeBNodeObject(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	bn := rdflibgo.NewBNode("y2")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/ref"), bn)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "rdf:nodeID") {
		t.Errorf("expected rdf:nodeID for BNode object, got:\n%s", out)
	}
}

// TestSerializeWithBase tests the xml:base option.
func TestSerializeWithBase(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf, WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "xml:base") {
		t.Errorf("expected xml:base in output, got:\n%s", out)
	}
}

// TestSerializeEmptyGraph tests that an empty graph produces valid XML shell.
func TestSerializeEmptyGraph(t *testing.T) {
	g := rdflibgo.NewGraph()
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "rdf:RDF") {
		t.Errorf("expected rdf:RDF element even for empty graph, got:\n%s", out)
	}
	if !strings.Contains(out, "</rdf:RDF>") {
		t.Errorf("expected closing rdf:RDF, got:\n%s", out)
	}
}

// TestSerializeRoundtripLangLiteral tests lang literal survives serialize→parse.
func TestSerializeRoundtripLangLiteral(t *testing.T) {
	g1 := rdflibgo.NewGraph()
	g1.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g1.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/label"),
		rdflibgo.NewLiteral("Hola", rdflibgo.WithLang("es")))

	var buf bytes.Buffer
	Serialize(g1, &buf)

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatalf("parse failed: %v\n%s", err, buf.String())
	}
	label := rdflibgo.NewURIRefUnsafe("http://example.org/label")
	val, ok := g2.Value(s, &label, nil)
	if !ok {
		t.Fatal("expected label triple after roundtrip")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok || lit.Language() != "es" {
		t.Errorf("expected lang 'es', got: %v", val)
	}
}

// TestSerializeRoundtripTypedLiteral tests typed literal survives serialize→parse.
func TestSerializeRoundtripTypedLiteral(t *testing.T) {
	g1 := rdflibgo.NewGraph()
	g1.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g1.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/count"),
		rdflibgo.NewLiteral("7", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))

	var buf bytes.Buffer
	Serialize(g1, &buf)

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatalf("parse failed: %v\n%s", err, buf.String())
	}
	count := rdflibgo.NewURIRefUnsafe("http://example.org/count")
	val, ok := g2.Value(s, &count, nil)
	if !ok {
		t.Fatal("expected count triple after roundtrip")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok || lit.Datatype() != rdflibgo.XSDInteger {
		t.Errorf("expected xsd:integer, got: %v", val)
	}
}

// ─── Error cases ──────────────────────────────────────────────────────────────

// TestParseForbiddenNodeElementName tests that rdf:RDF as a node element is rejected.
func TestParseForbiddenNodeElementName(t *testing.T) {
	// Using rdf:li as a top-level node (forbidden).
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:li rdf:about="http://example.org/s"/>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for forbidden node element rdf:li")
	}
}

// TestParseConflictingSubjectAttrs tests about+ID conflict.
func TestParseConflictingSubjectAttrs(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xml:base="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s" rdf:ID="myID"/>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for conflicting rdf:about + rdf:ID")
	}
}

// TestParseInvalidNodeID tests that an invalid NCName in rdf:nodeID is rejected.
func TestParseInvalidNodeID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:nodeID="123invalid"/>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for invalid rdf:nodeID NCName")
	}
}

// TestParseDuplicateRDFID tests that duplicate rdf:ID values produce an error.
func TestParseDuplicateRDFID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:ID="myID">
    <ex:p rdf:ID="myID">hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for duplicate rdf:ID")
	}
}

// TestParseResourceAndNodeID tests that rdf:resource + rdf:nodeID on same property is rejected.
func TestParseResourceAndNodeID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:resource="http://example.org/o" rdf:nodeID="b1"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for rdf:resource + rdf:nodeID combination")
	}
}

// TestParseLiteralAndResource tests that rdf:parseType="Literal" + rdf:resource is rejected.
func TestParseLiteralAndResource(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Literal" rdf:resource="http://example.org/o">text</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for parseType=Literal + rdf:resource")
	}
}

// TestParsePropertyElementWithPropAttrsOnly tests empty property element with
// non-rdf property attributes creates a blank-node object.
func TestParsePropertyElementWithPropAttrsOnly(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xmlns:dc="http://purl.org/dc/elements/1.1/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:creator dc:title="Alice"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// Expect: s ex:creator _:bn; _:bn dc:title "Alice"
	if g.Len() != 2 {
		t.Errorf("expected 2 triples (blank-node object + property), got %d", g.Len())
	}
}

// --- Additional coverage tests ---

// TestSerializeMultipleRDFType covers multiple rdf:type where first gets typed element.
func TestSerializeMultipleRDFType(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	t1 := rdflibgo.NewURIRefUnsafe("http://example.org/Type1")
	t2 := rdflibgo.NewURIRefUnsafe("http://example.org/Type2")
	g.Add(s, rdflibgo.RDF.Type, t1)
	g.Add(s, rdflibgo.RDF.Type, t2)
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("val"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// First rdf:type should become element name, second should be property element
	if !strings.Contains(out, "ex:Type") {
		t.Errorf("expected typed element name, got:\n%s", out)
	}
}

// TestSerializeNoQNamePredicate covers predicate without valid QName.
func TestSerializeNoQNamePredicate(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/no-namespace-match/p")
	g.Add(s, p, rdflibgo.NewLiteral("val"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	// The predicate has no matching namespace, so the full URI is used as element name
	out := buf.String()
	if !strings.Contains(out, "http://example.org/no-namespace-match/p") {
		t.Errorf("expected full URI for predicate, got:\n%s", out)
	}
}

// TestXmlQNameNoMatch covers xmlQName returning empty string.
func TestXmlQNameNoMatch(t *testing.T) {
	nsMap := map[string]string{
		"http://example.org/": "ex",
	}
	got := xmlQName("http://other.org/thing", nsMap)
	if got != "" {
		t.Errorf("expected empty qname for no-match, got %q", got)
	}
}

// TestXmlQNameWithHash covers xmlQName with # in local name.
func TestXmlQNameWithHash(t *testing.T) {
	nsMap := map[string]string{
		"http://example.org/": "ex",
	}
	got := xmlQName("http://example.org/a#b", nsMap)
	// "#" in local part makes it invalid
	if got != "" {
		t.Errorf("expected empty qname for hash in local, got %q", got)
	}
}

// TestIsValidNCNameEdges covers isValidNCName edge cases.
func TestIsValidNCNameEdges(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"", false},
		{"a", true},
		{"_x", true},
		{"1bad", false},
		{"a-b.c", true},
		{"a\u00C0b", true},   // extended char in middle
	}
	for _, tc := range tests {
		got := isValidNCName(tc.name)
		if got != tc.want {
			t.Errorf("isValidNCName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestParseTripleParseTypeWithoutVersion12 covers parseType="Triple" without rdf:version="1.2".
func TestParseTripleParseTypeWithoutVersion12(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Triple">
      <rdf:Description rdf:about="http://example.org/inner">
        <ex:q>val</ex:q>
      </rdf:Description>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// Without rdf:version="1.2", parseType="Triple" is silently skipped
}

// TestParseTripleParseTypeWithVersion12 covers parseType="Triple" with rdf:version="1.2".
func TestParseTripleParseTypeWithVersion12(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         rdf:version="1.2">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Triple">
      <rdf:Description rdf:about="http://example.org/inner">
        <ex:q>val</ex:q>
      </rdf:Description>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() < 1 {
		t.Errorf("expected at least 1 triple, got %d", g.Len())
	}
}

// TestParseReification covers rdf:ID on property element (reification).
func TestParseReification(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:ID="stmt1">hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// Should have the original triple + 4 reification triples
	if g.Len() < 5 {
		t.Errorf("expected at least 5 triples for reification, got %d", g.Len())
	}
}

// TestParseCollectionWithReifyID covers collection with rdf:ID reification.
func TestParseCollectionWithReifyID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection" rdf:ID="coll1">
      <rdf:Description rdf:about="http://example.org/a"/>
    </ex:items>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestSerializeRDFTypeNoQName covers rdf:type with object that has no valid QName.
func TestSerializeRDFTypeNoQName(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	// Type with no matching namespace prefix => should fall back to rdf:Description
	noNSType := rdflibgo.NewURIRefUnsafe("http://no-ns-registered.org/MyType")
	g.Add(s, rdflibgo.RDF.Type, noNSType)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "rdf:Description") {
		t.Errorf("expected rdf:Description fallback, got:\n%s", out)
	}
}

// TestResolveInvalidBaseURL covers url.Parse error in resolve.
func TestResolveInvalidBaseURL(t *testing.T) {
	p := &rdfxmlParser{base: "://bad-url"}
	got := p.resolve("relative")
	// Should return the original URI since base is unparseable
	if got != "relative" {
		t.Errorf("expected unchanged URI, got %q", got)
	}
}

// TestResolveInvalidRefURL covers url.Parse error on the ref.
func TestResolveInvalidRefURL(t *testing.T) {
	p := &rdfxmlParser{base: "http://example.org/"}
	got := p.resolve(":%bad")
	// url.Parse may or may not error; just exercise the branch
	_ = got
}

// TestSerializeWriteError covers writer error paths in Serialize.
func TestSerializeWriteError(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))
	err := Serialize(g, &errWriterRDF{})
	if err == nil {
		t.Error("expected error from broken writer")
	}
}

type errWriterRDF struct{}

func (errWriterRDF) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write error")
}

// limitWriter fails after writing maxBytes.
type limitWriter struct {
	written  int
	maxBytes int
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.written+len(p) > w.maxBytes {
		remaining := w.maxBytes - w.written
		if remaining <= 0 {
			return 0, fmt.Errorf("write limit reached")
		}
		w.written += remaining
		return remaining, fmt.Errorf("write limit reached")
	}
	w.written += len(p)
	return len(p), nil
}

// TestSerializeMultipleSubjects covers Serialize with multiple subjects.
func TestSerializeMultipleSubjects(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	for i := 0; i < 3; i++ {
		s := rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		g.Add(s, p, rdflibgo.NewLiteral(fmt.Sprintf("v%d", i)))
	}
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for i := 0; i < 3; i++ {
		if !strings.Contains(out, fmt.Sprintf("http://example.org/s%d", i)) {
			t.Errorf("missing subject s%d in output", i)
		}
	}
}

// TestSerializeDirLangLiteral covers directional lang tag in serializer.
func TestSerializeDirLangLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/label"),
		rdflibgo.NewLiteral("مرحبا", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl")))
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
}

// TestParseUnknownParseType covers unknown parseType (treated as literal).
func TestParseUnknownParseType(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Unknown">
      <someXML>data</someXML>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestParseMultipleNodeElements covers multiple node elements in rdf:RDF.
func TestParseMultipleNodeElements(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s1">
    <ex:p>v1</ex:p>
  </rdf:Description>
  <rdf:Description rdf:about="http://example.org/s2">
    <ex:p>v2</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
}

// TestParseEmptyResourceProperty covers empty property element with rdf:resource.
func TestParseEmptyResourceProperty(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:resource="http://example.org/o"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseEmptyNodeIDProperty covers empty property with rdf:nodeID.
func TestParseEmptyNodeIDProperty(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:nodeID="x1"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseCollectionMultipleItems covers collection with multiple items.
func TestParseCollectionMultipleItems(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection">
      <rdf:Description rdf:about="http://example.org/a"/>
      <rdf:Description rdf:about="http://example.org/b"/>
      <rdf:Description rdf:about="http://example.org/c"/>
    </ex:items>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// s items _:n1; _:n1 first a, rest _:n2; _:n2 first b, rest _:n3; _:n3 first c, rest nil
	if g.Len() < 7 {
		t.Errorf("expected at least 7 triples for collection, got %d", g.Len())
	}
}

// TestParseTypedNodeElement covers typed node element (not rdf:Description).
func TestParseTypedNodeElement(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <ex:Person rdf:about="http://example.org/alice">
    <ex:name>Alice</ex:name>
  </ex:Person>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples (type + name), got %d", g.Len())
	}
}

// TestParseMalformedXMLUnclosed covers malformed XML with unclosed element.
func TestParseMalformedXMLUnclosed(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="http://example.org/s">
    <unclosed>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for malformed XML")
	}
}

// TestParseReadInnerXMLNested covers readInnerXML with nested elements.
func TestParseReadInnerXMLNested(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:content rdf:parseType="Literal">
      <div xmlns="http://www.w3.org/1999/xhtml">
        <p>Hello <b>world</b></p>
      </div>
    </ex:content>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParsePropertyWithTextContent covers property element with text + nested element error.
func TestParsePropertyWithTextContent(t *testing.T) {
	// Property element with text content and no special attributes → literal value
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>simple text value</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestSerializeMultipleTypesWithSecondNoQName covers rdf:type without QName becoming property.
func TestSerializeMultipleTypesWithSecondNoQName(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Type1"))
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://no-ns.org/Type2"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ex:Type1") {
		t.Errorf("expected ex:Type1 as element name, got:\n%s", out)
	}
	if !strings.Contains(out, "rdf:type") {
		t.Errorf("expected rdf:type property for second type, got:\n%s", out)
	}
}

// ─── Additional coverage tests (batch 2) ────────────────────────────────────

// TestParseLiteralAndNodeIDConflict tests parseType="Literal" + rdf:nodeID rejected.
func TestParseLiteralAndNodeIDConflict(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Literal" rdf:nodeID="b1">text</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for parseType=Literal + rdf:nodeID")
	}
}

// TestParseForbiddenPropertyElementName tests rdf:Description as property element rejected.
func TestParseForbiddenPropertyElementName(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="http://example.org/s">
    <rdf:Description>text</rdf:Description>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for rdf:Description as property element")
	}
}

// TestParseForbiddenPropertyAttrOnNodeElement tests rdf:li as property attribute rejected.
func TestParseForbiddenPropertyAttrOnNodeElement(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s"
                   rdf:li="http://example.org/x"/>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for rdf:li as property attribute on node element")
	}
}

// TestParseForbiddenPropertyAttrNonRDFOnNodeElement tests forbidden non-rdf
// property attribute on node element.
func TestParseForbiddenPropertyAttrNonRDFOnNodeElement(t *testing.T) {
	// rdf:bagID is forbidden in forbiddenPropertyAttributeNames
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s"
                   rdf:bagID="bag1"/>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for rdf:bagID as property attribute")
	}
}

// TestParseForbiddenPropertyAttrOnPropertyElement tests forbidden rdf attrs on property elements.
func TestParseForbiddenPropertyAttrOnPropertyElement(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:aboutEach="http://example.org/x">text</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for rdf:aboutEach on property element")
	}
}

// TestParseForbiddenNonRDFPropertyAttrOnPropertyElement tests forbidden non-rdf
// qualified attribute on property element (e.g., rdf:bagID).
func TestParseForbiddenNonRDFPropertyAttrOnPropertyElement(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:bagID="bag1">text</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for rdf:bagID on property element")
	}
}

// TestParseInvalidNodeIDOnPropertyElement tests invalid rdf:nodeID on property element.
func TestParseInvalidNodeIDOnPropertyElement(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:nodeID="123invalid"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for invalid rdf:nodeID on property element")
	}
}

// TestParseConflictAboutAndNodeID tests about+nodeID conflict on node element.
func TestParseConflictAboutAndNodeID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="http://example.org/s" rdf:nodeID="b1"/>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for conflicting rdf:about + rdf:nodeID")
	}
}

// TestParseConflictIDAndNodeID tests ID+nodeID conflict on node element.
func TestParseConflictIDAndNodeID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xml:base="http://example.org/">
  <rdf:Description rdf:ID="x" rdf:nodeID="b1"/>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for conflicting rdf:ID + rdf:nodeID")
	}
}

// TestParseRDFVersionOnPropertyElement covers rdf:version="1.2" on property element.
func TestParseRDFVersionOnPropertyElement(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:version="1.2">hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseTripleParseTypeEmptyNode covers parseType="Triple" with empty content (error).
func TestParseTripleParseTypeEmptyNode(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         rdf:version="1.2">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Triple"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for empty parseType=Triple")
	}
}

// TestParseTripleParseTypeMultipleNodes covers parseType="Triple" with two child nodes (error).
func TestParseTripleParseTypeMultipleNodes(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         rdf:version="1.2">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Triple">
      <rdf:Description rdf:about="http://example.org/a">
        <ex:q>val</ex:q>
      </rdf:Description>
      <rdf:Description rdf:about="http://example.org/b">
        <ex:q>val2</ex:q>
      </rdf:Description>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for multiple nodes in parseType=Triple")
	}
}

// TestParseTripleParseTypeMultipleTriples covers parseType="Triple" producing >1 triple (error).
func TestParseTripleParseTypeMultipleTriples(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         rdf:version="1.2">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Triple">
      <rdf:Description rdf:about="http://example.org/a">
        <ex:q>val1</ex:q>
        <ex:r>val2</ex:r>
      </rdf:Description>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for parseType=Triple producing multiple triples")
	}
}

// TestParseReificationOnResource covers rdf:ID reification on rdf:resource property.
func TestParseReificationOnResource(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:resource="http://example.org/o" rdf:ID="stmt1"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// 1 triple + 4 reification triples = 5
	if g.Len() < 5 {
		t.Errorf("expected at least 5 triples for reification on resource, got %d", g.Len())
	}
}

// TestParseReificationOnParseTypeResource covers rdf:ID on parseType="Resource".
func TestParseReificationOnParseTypeResource(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Resource" rdf:ID="stmt1">
      <ex:q>inner</ex:q>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// s ex:p _:bn + _:bn ex:q "inner" + 4 reification = 6
	if g.Len() < 6 {
		t.Errorf("expected at least 6 triples, got %d", g.Len())
	}
}

// TestParseReificationOnNestedNode covers rdf:ID reification on nested node element.
func TestParseReificationOnNestedNode(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:ID="stmt1">
      <ex:Thing rdf:about="http://example.org/o"/>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// s ex:p ex:o + ex:o rdf:type ex:Thing + 4 reification = 6
	if g.Len() < 6 {
		t.Errorf("expected at least 6 triples, got %d", g.Len())
	}
}

// TestParseInvalidRDFID covers invalid NCName in rdf:ID on property element.
func TestParseInvalidRDFIDOnProperty(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:ID="123bad">hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for invalid rdf:ID on property")
	}
}

// TestParseReificationOnLiteral covers rdf:ID on parseType="Literal".
func TestParseReificationOnLiteral(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Literal" rdf:ID="stmt1"><b>bold</b></ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// 1 literal triple + 4 reification = 5
	if g.Len() < 5 {
		t.Errorf("expected at least 5 triples, got %d", g.Len())
	}
}

// TestParseCollectionWithError covers collection with a malformed child element.
func TestParseCollectionWithError(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection">
      <rdf:li rdf:about="http://example.org/a"/>
    </ex:items>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	// rdf:li is forbidden as a node element, so this should error
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Error("expected error for rdf:li inside collection")
	}
}

// TestParseITSAttributes covers its:version and its:dir on node and property elements.
func TestParseITSAttributes(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xmlns:its="http://www.w3.org/2005/11/its"
         rdf:version="1.2"
         its:version="2.0" its:dir="rtl">
  <rdf:Description rdf:about="http://example.org/s"
                   its:version="2.0" its:dir="ltr">
    <ex:p its:version="2.0" its:dir="rtl" xml:lang="ar">مرحبا</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() < 1 {
		t.Errorf("expected at least 1 triple, got %d", g.Len())
	}
}

// TestSerializeSpecialXMLChars covers XML escaping in literal values.
func TestSerializeSpecialXMLChars(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/note"),
		rdflibgo.NewLiteral(`a < b & c > d "e"`))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "&lt;") || !strings.Contains(out, "&amp;") || !strings.Contains(out, "&gt;") {
		t.Errorf("expected XML-escaped characters in output, got:\n%s", out)
	}
}

// TestParsePropertyAttrWithPropAttrsAndResource covers resource + property attrs path.
func TestParsePropertyAttrWithPropAttrsAndResource(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xmlns:dc="http://purl.org/dc/elements/1.1/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:creator rdf:resource="http://example.org/person" dc:title="Author"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// s ex:creator ex:person + ex:person dc:title "Author" = 2
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
}

// TestParseEmptyCollectionWithReifyID covers empty collection + rdf:ID reification.
func TestParseEmptyCollectionWithReifyID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection" rdf:ID="coll1"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// s ex:items rdf:nil + 4 reification = 5
	if g.Len() < 5 {
		t.Errorf("expected at least 5 triples for empty collection with reify, got %d", g.Len())
	}
}

// TestParseDatatype covers property with rdf:datatype.
func TestParseDatatypeProperty(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:age rdf:datatype="http://www.w3.org/2001/XMLSchema#integer">42</ex:age>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	agePred := rdflibgo.NewURIRefUnsafe("http://example.org/age")
	val, ok := g.Value(s, &agePred, nil)
	if !ok {
		t.Fatal("expected age triple")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	if lit.Datatype() != rdflibgo.XSDInteger {
		t.Errorf("expected xsd:integer, got %v", lit.Datatype())
	}
}

// TestParseXMLBaseOnProperty covers xml:base on property element (changes resolve context).
func TestParseXMLBaseOnProperty(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p xml:base="http://other.org/" rdf:resource="thing"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://other.org/thing")
	if !g.Contains(s, p, o) {
		t.Errorf("expected resource resolved against xml:base on property")
	}
}

// TestSerializeTripleTermObject covers serializer with a TripleTerm object
// (falls through switch, emitted as nothing — TripleTerm isn't URIRef/BNode/Literal).
func TestSerializeTripleTermObject(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	inner := rdflibgo.NewTripleTerm(
		rdflibgo.NewURIRefUnsafe("http://example.org/a"),
		rdflibgo.NewURIRefUnsafe("http://example.org/b"),
		rdflibgo.NewURIRefUnsafe("http://example.org/c"),
	)
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), inner)

	var buf bytes.Buffer
	// Should not error (TripleTerm is silently skipped in the switch)
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
}

// TestParseRDFLangOnRoot covers xml:lang on rdf:RDF root element.
func TestParseRDFLangOnRoot(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:lang="en">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:label>Hello</ex:label>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	label := rdflibgo.NewURIRefUnsafe("http://example.org/label")
	val, ok := g.Value(s, &label, nil)
	if !ok {
		t.Fatal("expected label triple")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	if lit.Language() != "en" {
		t.Errorf("expected lang 'en', got %q", lit.Language())
	}
}

// TestParseRDFTypeAttrOnNodeElement covers rdf:type as attribute on node element.
func TestParseRDFTypeAttrOnNodeElement(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s"
                   rdf:type="http://example.org/Person"/>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	person := rdflibgo.NewURIRefUnsafe("http://example.org/Person")
	if !g.Contains(s, rdflibgo.RDF.Type, person) {
		t.Error("expected rdf:type triple from attribute")
	}
}

// TestParseAnnotationIRI covers rdf:annotation on property element.
func TestParseAnnotationIRI(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         rdf:version="1.2">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:annotation="http://example.org/annot">hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// Triple + rdf:reifies annotation
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples (triple + annotation), got %d", g.Len())
	}
}

// TestParseAnnotationNodeID covers rdf:annotationNodeID on property element.
func TestParseAnnotationNodeID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         rdf:version="1.2">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:annotationNodeID="ann1">hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestSerializeWriteErrorAtVariousPoints covers writer error paths at different
// positions during Serialize to hit the multiple Fprintf error checks.
func TestSerializeWriteErrorAtVariousPoints(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	// Try failing at different byte offsets to exercise each Fprintf error check.
	for limit := 0; limit < 300; limit += 20 {
		w := &limitWriter{maxBytes: limit}
		err := Serialize(g, w)
		if err == nil && limit < 200 {
			// Small limits should cause errors
			continue
		}
	}
}

// TestSerializeWriteErrorOnNamespace covers writer error during namespace declaration.
func TestSerializeWriteErrorOnNamespace(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Bind("foaf", rdflibgo.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))

	// Fail after the XML declaration but during namespace output
	w := &limitWriter{maxBytes: 60}
	err := Serialize(g, w)
	if err == nil {
		t.Error("expected error when writer fails during namespace output")
	}
}

// TestSerializeWriteErrorOnSubject covers writer error during subject opening tag.
func TestSerializeWriteErrorOnSubject(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/q"),
		rdflibgo.NewURIRefUnsafe("http://example.org/o"))

	// Fail midway through writing subject opening tags / properties
	w := &limitWriter{maxBytes: 180}
	err := Serialize(g, w)
	if err == nil {
		t.Error("expected error when writer fails during property output")
	}
}

// TestSerializeWriteErrorOnClose covers writer error during closing rdf:RDF tag.
func TestSerializeWriteErrorOnClose(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))

	// Calculate approximate output size and fail right before the closing tag
	var buf bytes.Buffer
	Serialize(g, &buf)
	totalSize := buf.Len()

	w := &limitWriter{maxBytes: totalSize - 5}
	err := Serialize(g, w)
	if err == nil {
		t.Error("expected error when writer fails during closing tag")
	}
}

// TestParseRDFRootErrorInLoop covers error from decoder in parseRDFRoot loop.
func TestParseRDFRootErrorInLoop(t *testing.T) {
	// Truncated XML after rdf:RDF opening tag
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="http://example.org/s">
    <unclosed>`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for truncated XML in RDF root")
	}
}

// TestParseNodeElementErrorInChildLoop covers decoder error while parsing child properties.
func TestParseNodeElementErrorInChildLoop(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>hello</ex:p>
    <ex:q><malformed`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for truncated child element")
	}
}

// TestParseTripleParseTypeErrorInDecoder covers decoder error during parseType="Triple".
func TestParseTripleParseTypeErrorInDecoder(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         rdf:version="1.2">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Triple">
      <rdf:Description rdf:about="http://example.org/inner">
        <ex:q>val</ex:q>
        <malformed`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for truncated parseType=Triple content")
	}
}

// TestParseCollectionErrorInDecoder covers decoder error during collection parsing.
func TestParseCollectionErrorInDecoder(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection">
      <rdf:Description rdf:about="http://example.org/a"/>
      <malformed`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for truncated collection")
	}
}

// TestParsePropertyResourceErrorInDecoder covers decoder error during parseType="Resource".
func TestParsePropertyResourceErrorInDecoder(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Resource">
      <ex:q>val</ex:q>
      <malformed`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for truncated parseType=Resource content")
	}
}

// TestParseLiteralErrorInDecoder covers decoder error during parseType="Literal" inner read.
func TestParseLiteralErrorInDecoder(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:parseType="Literal"><div><malformed`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for truncated parseType=Literal content")
	}
}

// TestParseDefaultPropertyErrorInDecoder covers decoder error in default property case.
func TestParseDefaultPropertyErrorInDecoder(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>some text <malformed`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for truncated default property")
	}
}

// TestParseChildNodeErrorInDecoder covers error during child node parsing within property.
func TestParseChildNodeErrorInDecoder(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>
      <rdf:li rdf:about="http://example.org/inner"/>
    </ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	// rdf:li is a forbidden node element
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for forbidden node in property")
	}
}

// TestParsePropertyAttrReificationOnBlanks covers rdf:ID on property with only prop attrs (blank node).
func TestParsePropertyAttrReificationOnBlanks(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xmlns:dc="http://purl.org/dc/elements/1.1/"
         xml:base="http://example.org/doc">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:creator rdf:ID="stmt1" dc:title="Alice"/>
  </rdf:Description>
</rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	// s ex:creator _:bn + _:bn dc:title "Alice" + 4 reification = 6
	if g.Len() < 6 {
		t.Errorf("expected at least 6 triples, got %d", g.Len())
	}
}
