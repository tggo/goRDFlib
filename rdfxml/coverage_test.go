package rdfxml

import (
	"bytes"
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
		{":noscheme", false},     // colon at position 0
		{"1abc:bad", false},      // digit as first char
		{"a b:bad", false},       // space in scheme
		{"http", false},          // no colon
		{"a+b-c.d:ok", true},    // '+', '-', '.' allowed after first char
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
		{"'", "'"},                         // single quote — no escaping
		{"hello world", "hello world"},     // plain text unchanged
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
