package trig

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// limitWriter writes up to limit bytes then returns an error.
type limitWriter struct {
	limit   int
	written int
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.written+len(p) > w.limit {
		n := w.limit - w.written
		if n > 0 {
			w.written += n
			return n, io.ErrClosedPipe
		}
		return 0, io.ErrClosedPipe
	}
	w.written += len(p)
	return len(p), nil
}

// errReader always fails on Read.
type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errors.New("read fail") }

func mustFail(t *testing.T, input string) {
	t.Helper()
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Fatal("expected parse error")
	}
}

func covSerialize(t *testing.T, g *rdflibgo.Graph) string {
	t.Helper()
	var buf strings.Builder
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func covSerializeDS(t *testing.T, ds *graph.Dataset) string {
	t.Helper()
	var buf strings.Builder
	if err := SerializeDataset(ds, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// --- Parser coverage ---

// TestCovParseDataset covers ParseDataset.
func TestCovParseDataset(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .
{ ex:s ex:p "default" . }
ex:g1 { ex:s ex:p "named" . }
`
	ds := graph.NewDataset()
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovParseWithBase covers WithBase option for Parse.
func TestCovParseWithBase(t *testing.T) {
	g := rdflibgo.NewGraph()
	input := `{ <s> <p> "v" . }`
	if err := Parse(g, strings.NewReader(input), WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestCovParseDatasetWithBase covers WithBase for ParseDataset.
func TestCovParseDatasetWithBase(t *testing.T) {
	ds := graph.NewDataset()
	input := `{ <s> <p> "v" . }`
	if err := ParseDataset(ds, strings.NewReader(input), WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
}

// TestCovVersionDirective covers VERSION directive.
func TestCovVersionDirective(t *testing.T) {
	mustParse(t, `VERSION "1.2"
@prefix ex: <http://example.org/> .
{ ex:s ex:p "v" . }`)
}

// TestCovVersionSingleQuote covers VERSION with single quotes.
func TestCovVersionSingleQuote(t *testing.T) {
	mustParse(t, `VERSION '1.2'
@prefix ex: <http://example.org/> .
{ ex:s ex:p "v" . }`)
}

// TestCovVersionTripleQuoteError covers triple-quoted version string error.
func TestCovVersionTripleQuoteError(t *testing.T) {
	mustFail(t, `VERSION """1.2"""`)
}

// TestCovVersionNewlineError covers newline in version string.
func TestCovVersionNewlineError(t *testing.T) {
	mustFail(t, "VERSION \"1\n2\"")
}

// TestCovVersionMissingQuote covers missing quote in VERSION.
func TestCovVersionMissingQuote(t *testing.T) {
	mustFail(t, `VERSION 1.2`)
}

// TestCovVersionEOF covers VERSION at EOF.
func TestCovVersionEOF(t *testing.T) {
	mustFail(t, `VERSION `)
}

// TestCovVersionUnterminated covers unterminated version string.
func TestCovVersionUnterminated(t *testing.T) {
	mustFail(t, `VERSION "1.2`)
}

// TestCovSPARQLBase covers SPARQL-style BASE.
func TestCovSPARQLBase(t *testing.T) {
	mustParse(t, `BASE <http://example.org/>
{ <s> <p> "v" . }`)
}

// TestCovSPARQLBaseError covers error in SPARQL BASE.
func TestCovSPARQLBaseError(t *testing.T) {
	mustFail(t, `BASE
{ <s> <p> "v" . }`)
}

// TestCovSPARQLPrefix covers SPARQL-style PREFIX.
func TestCovSPARQLPrefix(t *testing.T) {
	mustParse(t, `PREFIX ex: <http://example.org/>
{ ex:s ex:p "v" . }`)
}

// TestCovAnnotation covers {| |} annotation.
func TestCovAnnotation(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o {| ex:source ex:doc |} . }`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovEmptyAnnotationError covers empty annotation block.
func TestCovEmptyAnnotationError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o {| |} . }`)
}

// TestCovReifier covers ~ reifier syntax.
func TestCovReifier(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o ~ ex:r1 . }`)
}

// TestCovReifierAnonymous covers anonymous ~ reifier.
func TestCovReifierAnonymous(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o ~ . }`)
}

// TestCovReifierBNode covers ~ _:bnode reifier.
func TestCovReifierBNode(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o ~ _:r1 . }`)
}

// TestCovReifierIRI covers ~ <IRI> reifier.
func TestCovReifierIRI(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o ~ <http://example.org/reifier> . }`)
}

// TestCovReifierWithAnnotation covers ~ id {| ... |}.
func TestCovReifierWithAnnotation(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o ~ ex:r {| ex:source ex:doc |} . }`)
}

// TestCovReifiedTriple covers << s p o >> as subject.
func TestCovReifiedTriple(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p ex:o >> ex:source ex:doc . }`)
}

// TestCovReifiedTripleNamedReifier covers << s p o ~ id >>.
func TestCovReifiedTripleNamedReifier(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p ex:o ~ ex:r >> ex:source ex:doc . }`)
}

// TestCovTripleTermObject covers <<( s p o )>> as object.
func TestCovTripleTermObject(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p <<( ex:a ex:b ex:c )>> . }`)
}

// TestCovReifiedInnerSubjectBNode covers bnode in reified triple subject.
func TestCovReifiedInnerSubjectBNode(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << _:b1 ex:p ex:o >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerSubjectEmptyBNode covers [] in reified triple subject.
func TestCovReifiedInnerSubjectEmptyBNode(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << [] ex:p ex:o >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerSubjectNested covers nested reified triple as subject.
func TestCovReifiedInnerSubjectNested(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << << ex:a ex:b ex:c >> ex:p ex:o >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectBNode covers bnode as reified inner object.
func TestCovReifiedInnerObjectBNode(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p _:b1 >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectLiteral covers literal as reified inner object.
func TestCovReifiedInnerObjectLiteral(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p "hello" >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectBoolean covers boolean as reified inner object.
func TestCovReifiedInnerObjectBoolean(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p true >> ex:source ex:doc . }`)
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p false >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectNumeric covers numeric as reified inner object.
func TestCovReifiedInnerObjectNumeric(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p 42 >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectEmptyBNode covers [] as reified inner object.
func TestCovReifiedInnerObjectEmptyBNode(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p [] >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectPrefixed covers prefixed name as reified inner object.
func TestCovReifiedInnerObjectPrefixed(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p ex:o >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectTripleTerm covers triple term as reified inner object.
func TestCovReifiedInnerObjectTripleTerm(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p <<( ex:a ex:b ex:c )>> >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectNestedReified covers nested reified as inner object.
func TestCovReifiedInnerObjectNestedReified(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p << ex:a ex:b ex:c >> >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerCollectionError covers collection rejection in reified triple.
func TestCovReifiedInnerCollectionError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p (1 2 3) >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerBNodePropertyListError covers [p o] rejection.
func TestCovReifiedInnerBNodePropertyListError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ << [ex:a ex:b] ex:p ex:o >> ex:source ex:doc . }`)
}

// TestCovTripleTermSubjectError covers triple term as subject error.
func TestCovTripleTermSubjectError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ <<( ex:s ex:p ex:o )>> ex:q ex:r . }`)
}

// TestCovTripleTermSubjectBNode covers bnode as triple term subject.
func TestCovTripleTermSubjectBNode(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p <<( _:b1 ex:q ex:o )>> . }`)
}

// TestCovTripleTermSubjectPrefixed covers prefixed name as triple term subject.
func TestCovTripleTermSubjectPrefixed(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p <<( ex:a ex:b ex:c )>> . }`)
}

// TestCovMatchKeywordCI covers keyword edge cases.
func TestCovMatchKeywordCI(t *testing.T) {
	mustFail(t, `PREFIXfoo: <http://example.org/> .`)
	mustFail(t, `PREFIX`)
}

// TestCovCollectionAsSubject covers collection ( ... ) as subject.
func TestCovCollectionAsSubject(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
( "a" "b" ) ex:p "v" .`)
}

// TestCovBracketSubjectOrGraph covers blank node property list handling.
func TestCovBracketSubjectOrGraph(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
[ ex:a ex:b ] ex:p "v" .`)
}

// TestCovDirectiveError covers @prefix error.
func TestCovDirectiveError(t *testing.T) {
	mustFail(t, `@prefix ex .`)
}

// TestCovUnicodeEscapeTruncated covers truncated unicode escape.
func TestCovUnicodeEscapeTruncated(t *testing.T) {
	mustFail(t, `<http://example.org/\u00> <http://example.org/p> "v" .`)
}

// TestCovUnicodeEscapeSurrogate covers surrogate in unicode escape.
func TestCovUnicodeEscapeSurrogate(t *testing.T) {
	mustFail(t, `<http://example.org/\uD800> <http://example.org/p> "v" .`)
}

// TestCovReadGraphLabel covers graph label parsing.
func TestCovReadGraphLabel(t *testing.T) {
	ds := graph.NewDataset()
	input := `@prefix ex: <http://example.org/> .
GRAPH ex:g1 { ex:s ex:p "v" . }
GRAPH <http://example.org/g2> { ex:s ex:p "v2" . }
`
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovReadGraphLabelBNode covers bnode as graph label.
func TestCovReadGraphLabelBNode(t *testing.T) {
	ds := graph.NewDataset()
	input := `@prefix ex: <http://example.org/> .
_:g1 { ex:s ex:p "v" . }
`
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovReifiedTripleStatement covers reified triple as top-level statement.
func TestCovReifiedTripleStatement(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:source ex:doc .`)
}

// TestCovCollectionTripleStatement covers collection as top-level subject.
func TestCovCollectionTripleStatement(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
( "a" "b" ) ex:p "v" .`)
}

// TestCovResolveIRI covers resolveIRI with base.
func TestCovResolveIRI(t *testing.T) {
	mustParse(t, `@base <http://example.org/dir/> .
<../other> <http://example.org/p> "v" .`)
}

// TestCovIsAbsoluteIRI covers isAbsoluteIRI edge cases.
func TestCovIsAbsoluteIRI(t *testing.T) {
	mustParse(t, `@base <http://example.org/> .
<relative> <http://example.org/p> "v" .`)
}

// --- Serializer coverage ---

// TestCovSerializeTripleTerm covers TripleTerm serialization.
func TestCovSerializeTripleTerm(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ex:b ex:c )>> .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term, got:\n%s", out)
	}
}

// TestCovSerializeList covers list serialization.
func TestCovSerializeList(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ( "a" "b" "c" ) .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "(") {
		t.Errorf("expected list syntax, got:\n%s", out)
	}
}

// TestCovSerializeInlineBNode covers inline blank node serialization.
func TestCovSerializeInlineBNode(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p [ ex:a "val" ] .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "[") {
		t.Errorf("expected inline bnode, got:\n%s", out)
	}
}

// TestCovSerializeWithBase covers @base in TriG output.
func TestCovSerializeWithBase(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("val"))

	var buf strings.Builder
	if err := Serialize(g, &buf, WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "@base") {
		t.Errorf("expected @base, got:\n%s", buf.String())
	}
}

// TestCovSerializePrefixed covers prefixed name serialization.
func TestCovSerializePrefixed(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("val"))
	out := covSerialize(t, g)
	if !strings.Contains(out, "ex:") {
		t.Errorf("expected prefixed names, got:\n%s", out)
	}
}

// TestCovSerializeNamedGraph covers named graph serialization.
func TestCovSerializeNamedGraph(t *testing.T) {
	ds := graph.NewDataset()
	ds.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	ng := ds.Graph(rdflibgo.NewURIRefUnsafe("http://example.org/g1"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	ng.Add(s, p, rdflibgo.NewLiteral("named"))

	out := covSerializeDS(t, ds)
	if !strings.Contains(out, "ex:g1") || !strings.Contains(out, "{") {
		t.Errorf("expected named graph block, got:\n%s", out)
	}
}

// TestCovSerializeRDFSClass covers rdfs:Class ordering in serializer.
func TestCovSerializeRDFSClass(t *testing.T) {
	g := mustParse(t, `@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix ex: <http://example.org/> .
ex:MyClass rdf:type rdfs:Class .
ex:other ex:p "v" .`)
	covSerialize(t, g)
}

// TestCovSerializeUnreferencedBNode covers [] for unreferenced bnodes.
func TestCovSerializeUnreferencedBNode(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
_:b1 ex:p "v" .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "[]") {
		t.Errorf("expected [], got:\n%s", out)
	}
}

// TestCovSerializeMultiplePredicates covers semicolon-separated predicates.
func TestCovSerializeMultiplePredicates(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p1 "a" ; ex:p2 "b" .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, ";") {
		t.Errorf("expected ;, got:\n%s", out)
	}
}

// TestCovSerializeMultipleObjects covers comma-separated objects.
func TestCovSerializeMultipleObjects(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "a", "b" .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, ",") {
		t.Errorf("expected comma, got:\n%s", out)
	}
}

// TestCovIsValidPrefixName covers prefix name validation.
func TestCovIsValidPrefixName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"", true},
		{"rdf", true},
		{"1bad", false},
		{"ok.", false},
		{"ok.ok", true},
	}
	for _, tc := range tests {
		got := isValidPrefixName(tc.name)
		if got != tc.want {
			t.Errorf("isValidPrefixName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCovIsValidLocalName covers local name validation.
func TestCovIsValidLocalName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"", false},
		{"name", true},
		{":name", true},
		{"0name", true},
		{"a.", false},
		{"a b", false},
	}
	for _, tc := range tests {
		got := isValidLocalName(tc.name)
		if got != tc.want {
			t.Errorf("isValidLocalName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCovSerializeInvalidLocalName covers fallback to full IRI.
func TestCovSerializeInvalidLocalName(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/has space")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("val"))
	out := covSerialize(t, g)
	if !strings.Contains(out, "<http://example.org/has space>") {
		t.Errorf("expected full IRI, got:\n%s", out)
	}
}

// TestCovSerializeListCyclic covers cyclic list detection.
func TestCovSerializeListCyclic(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn1 := rdflibgo.NewBNode("cyc1")
	bn2 := rdflibgo.NewBNode("cyc2")
	g.Add(bn1, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	g.Add(bn1, rdflibgo.RDF.Rest, bn2)
	g.Add(bn2, rdflibgo.RDF.First, rdflibgo.NewLiteral("b"))
	g.Add(bn2, rdflibgo.RDF.Rest, bn1)
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/list"), bn1)
	covSerialize(t, g)
}

// TestCovSerializeListInvalid covers invalid list detection (extra pred).
func TestCovSerializeListInvalid(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn := rdflibgo.NewBNode("l1")
	g.Add(bn, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	g.Add(bn, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
	g.Add(bn, rdflibgo.NewURIRefUnsafe("http://example.org/extra"), rdflibgo.NewLiteral("x"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/list"), bn)
	covSerialize(t, g)
}

// TestCovSerializeTripleTermWithPrefix covers TripleTerm with prefixed names.
func TestCovSerializeTripleTermWithPrefix(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(s, p, o)
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/asserts"), tt)
	out := covSerialize(t, g)
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term, got:\n%s", out)
	}
}

// TestCovSerializeEmptyGraph covers empty graph serialization.
func TestCovSerializeEmptyGraph(t *testing.T) {
	g := rdflibgo.NewGraph()
	out := covSerialize(t, g)
	_ = out
}

// --- More parser/serializer coverage ---

// TestCovReifiedInnerObjectEmptyBNodeTrig covers [] as object in reified triple.
func TestCovReifiedInnerObjectEmptyBNodeTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p [] >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectLiteralTrig covers string literal in reified triple.
func TestCovReifiedInnerObjectLiteralTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p "hello" >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectPrefixedTrig covers prefixed name in reified object.
func TestCovReifiedInnerObjectPrefixedTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:source ex:doc .`)
}

// TestCovReifiedTripleAnonymousReifierTrig covers << s p o ~ >> syntax.
func TestCovReifiedTripleAnonymousReifierTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o ~ >> ex:source ex:doc .`)
}

// TestCovReifiedTripleIRIReifierTrig covers ~ <IRI> reifier.
func TestCovReifiedTripleIRIReifierTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o ~ <http://example.org/r1> >> ex:source ex:doc .`)
}

// TestCovReifiedTripleBNodeReifierTrig covers ~ _:bnode reifier.
func TestCovReifiedTripleBNodeReifierTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o ~ _:r1 >> ex:source ex:doc .`)
}

// TestCovReifiedTriplePrefixedReifierTrig covers ~ prefix:name reifier.
func TestCovReifiedTriplePrefixedReifierTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o ~ ex:r1 >> ex:source ex:doc .`)
}

// TestCovReifiedInnerSubjectIRITrig covers IRI as reified inner subject.
func TestCovReifiedInnerSubjectIRITrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< <http://example.org/s> ex:p ex:o >> ex:source ex:doc .`)
}

// TestCovTripleTermSubjectIRITrig covers IRI as triple term subject.
func TestCovTripleTermSubjectIRITrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( <http://example.org/a> ex:b ex:c )>> .`)
}

// TestCovTripleTermSubjectPrefixedTrig covers prefixed name as triple term subject.
func TestCovTripleTermSubjectPrefixedTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ex:b ex:c )>> .`)
}

// TestCovTripleTermSubjectBNodeTrig covers bnode as triple term subject.
func TestCovTripleTermSubjectBNodeTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( _:b1 ex:b ex:c )>> .`)
}

// TestCovReifiedInnerObjectTripleTermTrig covers triple term as reified inner object.
func TestCovReifiedInnerObjectTripleTermTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p <<( ex:a ex:b ex:c )>> >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectNestedReifiedTrig covers nested reified as inner object.
func TestCovReifiedInnerObjectNestedReifiedTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p << ex:a ex:b ex:c >> >> ex:source ex:doc .`)
}

// TestCovAnnotationMultiPredTrig covers annotation with multiple predicates.
func TestCovAnnotationMultiPredTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o {| ex:source ex:doc ; ex:date "2024" |} .`)
}

// TestCovReifiedWithAnnotationTrig covers reified + annotation.
func TestCovReifiedWithAnnotationTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ ex:r {| ex:source ex:doc |} .`)
}

// TestCovReifierErrorTrig covers error in reifier.
func TestCovReifierErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ `)
}

// TestCovBooleanInReifiedTrig covers boolean in reified object.
func TestCovBooleanInReifiedTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p true >> ex:source ex:doc .`)
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p false >> ex:source ex:doc .`)
}

// TestCovNumericInReifiedTrig covers numeric in reified object.
func TestCovNumericInReifiedTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p 42 >> ex:source ex:doc .`)
}

// TestCovDirectiveBase covers @base directive.
func TestCovDirectiveBase(t *testing.T) {
	mustParse(t, `@base <http://example.org/> .
<s> <p> "v" .`)
}

// TestCovDirectivePrefixError covers error in @prefix.
func TestCovDirectivePrefixError2(t *testing.T) {
	mustFail(t, `@prefix .`)
}

// TestCovDirectiveBaseError covers error in @base.
func TestCovDirectiveBaseError(t *testing.T) {
	mustFail(t, `@base .`)
}

// TestCovReadSubjectReifiedTripleTrig covers reified triple as subject.
func TestCovReadSubjectReifiedTripleTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:a ex:b ex:c >> ex:source ex:doc .`)
}

// TestCovReadSubjectCollectionTrig covers collection as subject.
func TestCovReadSubjectCollectionTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
("a" "b") ex:p "v" .`)
}

// TestCovReadDatatypeIRITrig covers datatype IRI reading.
func TestCovReadDatatypeIRITrig(t *testing.T) {
	mustParse(t, `@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
<http://example.org/s> <http://example.org/p> "42"^^xsd:integer .`)
}

// TestCovReadDatatypeIRIErrorTrig covers datatype IRI error.
func TestCovReadDatatypeIRIErrorTrig(t *testing.T) {
	mustFail(t, `<http://example.org/s> <http://example.org/p> "v"^^.`)
}

// TestCovPredicateObjectListTrailingSemicolonTrig covers trailing semicolon.
func TestCovPredicateObjectListTrailingSemicolonTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "v" ; .`)
}

// TestCovObjectListMultipleTrig covers multiple objects.
func TestCovObjectListMultipleTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "a", "b", "c" .`)
}

// TestCovReadBlankNodeLabelEdgeTrig covers bnode label with dot in middle.
func TestCovReadBlankNodeLabelEdgeTrig(t *testing.T) {
	mustParse(t, `_:b1.2 <http://example.org/p> "v" .`)
}

// TestCovSerializeWithRDFSLabel covers rdfs:label predicate ordering.
func TestCovSerializeWithRDFSLabel(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("rdfs", rdflibgo.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#"))
	g.Bind("rdf", rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.RDFS.Class)
	g.Add(s, rdflibgo.RDFS.Label, rdflibgo.NewLiteral("My Class"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/extra"), rdflibgo.NewLiteral("x"))
	covSerialize(t, g)
}

// TestCovSerializeMultiSubjectTrig covers multiple subjects.
func TestCovSerializeMultiSubjectTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	for i := 0; i < 3; i++ {
		s := rdflibgo.NewURIRefUnsafe("http://example.org/s" + string(rune('a'+i)))
		g.Add(s, p, rdflibgo.NewLiteral("v"))
	}
	covSerialize(t, g)
}

// TestCovSerializeBNodeNoPredicatesTrig covers writing a bnode with no predicates.
func TestCovSerializeBNodeNoPredicatesTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	bn := rdflibgo.NewBNode("nopreds")
	g.Add(s, p, bn)
	covSerialize(t, g)
}

// TestCovResolveIRIRelativeTrig covers relative IRI resolution.
func TestCovResolveIRIRelativeTrig(t *testing.T) {
	mustParse(t, `@base <http://example.org/dir/> .
<../other> <http://example.org/p> "v" .`)
}

// TestCovReadEscapeTrig covers escape sequences in strings.
func TestCovReadEscapeTrig(t *testing.T) {
	mustParse(t, `<http://example.org/s> <http://example.org/p> "a\nb\rc\td" .`)
}

// TestCovReadUnicodeEscapeBadHexTrig covers bad hex.
func TestCovReadUnicodeEscapeBadHexTrig(t *testing.T) {
	mustFail(t, `<http://example.org/s> <http://example.org/p> "a\uXXXXb" .`)
}

// TestCovUnescapeIRIBadTrig covers bad escape in IRI.
func TestCovUnescapeIRIBadTrig(t *testing.T) {
	mustFail(t, `<http://example.org/\uXXXX> <http://example.org/p> "v" .`)
}

// TestCovReadCollectionNestedTrig covers nested collections.
func TestCovReadCollectionNestedTrig(t *testing.T) {
	mustParse(t, `<http://example.org/s> <http://example.org/p> ((1 2) 3) .`)
}

// TestCovReadCollectionEmptyTrig covers empty collection.
func TestCovReadCollectionEmptyTrig(t *testing.T) {
	mustParse(t, `<http://example.org/s> <http://example.org/p> () .`)
}

// TestCovSPARQLPrefixErrorTrig covers error in SPARQL PREFIX.
func TestCovSPARQLPrefixErrorTrig(t *testing.T) {
	mustFail(t, `PREFIX
<http://example.org/s> <http://example.org/p> "v" .`)
}

// TestCovParseReadError covers io.ReadAll error in Parse.
func TestCovParseReadError(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, errReader{})
	if err == nil {
		t.Error("expected read error")
	}
}

// TestCovParseDatasetReadError covers io.ReadAll error in ParseDataset.
func TestCovParseDatasetReadError(t *testing.T) {
	ds := graph.NewDataset()
	err := ParseDataset(ds, errReader{})
	if err == nil {
		t.Error("expected read error")
	}
}

// TestCovGRAPHKeyword covers GRAPH keyword in various positions.
func TestCovGRAPHKeyword(t *testing.T) {
	ds := graph.NewDataset()
	input := `@prefix ex: <http://example.org/> .
GRAPH ex:g1 { ex:s ex:p "v" . }
`
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovDirectiveInGraph covers directive inside graph block.
func TestCovDirectiveInGraph(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:g1 {
  ex:s ex:p "v" .
}`)
}

// TestCovParseWithBaseOption covers WithBase.
func TestCovParseWithBaseOption(t *testing.T) {
	g := rdflibgo.NewGraph()
	input := `<s> <p> "v" .`
	if err := Parse(g, strings.NewReader(input), WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
}

// --- Additional trig parser coverage ---

// TestCovCollectionAsGraphLabelError covers collection not allowed as graph label.
func TestCovCollectionAsGraphLabelError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
( "a" "b" ) { ex:s ex:p "v" . }`)
}

// TestCovCollectionStandaloneError covers standalone collection without predicates.
func TestCovCollectionStandaloneError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
( "a" "b" ) .`)
}

// TestCovBracketGraphBlock covers [] { } anonymous graph block.
func TestCovBracketGraphBlock(t *testing.T) {
	ds := graph.NewDataset()
	input := `@prefix ex: <http://example.org/> .
[] { ex:s ex:p "v" . }
`
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovBracketSubjectDot covers [] . (standalone empty bnode subject).
func TestCovBracketSubjectDot(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
[] .`)
}

// TestCovBracketSubjectPredObj covers [] ex:p "v" .
func TestCovBracketSubjectPredObj(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
[] ex:p "v" .`)
}

// TestCovBNodePropListAsGraphLabelError covers [...] { } error.
func TestCovBNodePropListAsGraphLabelError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
[ ex:a ex:b ] { ex:s ex:p "v" . }`)
}

// TestCovBNodePropListDot covers [...] . (standalone).
func TestCovBNodePropListDot(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
[ ex:a ex:b ] .`)
}

// TestCovBNodePropListPredObj covers [...] ex:p "v" .
func TestCovBNodePropListPredObj(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
[ ex:a ex:b ] ex:p "v" .`)
}

// TestCovReifiedTripleStandaloneWithDot covers << s p o >> . (no predicate list).
func TestCovReifiedTripleStandaloneWithDot(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> .`)
}

// TestCovSubjectOrGraphBNodeAsGraphLabel covers _:bnode { } as graph label.
func TestCovSubjectOrGraphBNodeAsGraphLabel(t *testing.T) {
	ds := graph.NewDataset()
	input := `@prefix ex: <http://example.org/> .
_:g1 { ex:s ex:p "v" . }
`
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovSubjectOrGraphIRIAsTriple covers IRI used as subject (not graph).
func TestCovSubjectOrGraphIRIAsTriple(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "v" .`)
}

// TestCovDirectiveVersion covers @version directive in TriG.
func TestCovDirectiveVersion(t *testing.T) {
	mustParse(t, `@version "1.2" .
@prefix ex: <http://example.org/> .
{ ex:s ex:p "v" . }`)
}

// TestCovDirectiveUnknown covers unknown directive error.
func TestCovDirectiveUnknown(t *testing.T) {
	mustFail(t, `@unknown .`)
}

// TestCovReadGraphLabelEmpty covers empty graph label (EOF).
func TestCovReadGraphLabelError(t *testing.T) {
	mustFail(t, `GRAPH`)
}

// TestCovObjectListSecondError covers error in second object.
func TestCovObjectListSecondError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "a", .`)
}

// TestCovPredicateObjectListEmpty covers missing predicate after subject.
func TestCovPredicateObjectListEmpty(t *testing.T) {
	mustFail(t, `<http://example.org/s> .`)
}

// TestCovObjectListEmpty covers missing object after predicate.
func TestCovObjectListEmpty(t *testing.T) {
	mustFail(t, `<http://example.org/s> <http://example.org/p> .`)
}

// TestCovSPARQLBaseError2 covers BASE with invalid argument.
func TestCovSPARQLBaseError2(t *testing.T) {
	mustFail(t, `BASE bad`)
}

// TestCovSPARQLPrefixMissingColon covers PREFIX without colon.
func TestCovSPARQLPrefixMissingColon(t *testing.T) {
	mustFail(t, `PREFIX ex <http://example.org/>`)
}

// TestCovReadSubjectError covers error in readSubject.
func TestCovReadSubjectError(t *testing.T) {
	mustFail(t, `{ 42 <http://example.org/p> "v" . }`)
}

// TestCovReadIRIInvalid covers unescaped space in IRI.
func TestCovReadIRIInvalid(t *testing.T) {
	mustFail(t, `<http://example.org/a b> <http://example.org/p> "v" .`)
}

// TestCovReadDatatypeIRIPrefixed covers prefixed datatype IRI.
func TestCovReadDatatypeIRIPrefixed(t *testing.T) {
	mustParse(t, `@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
<http://example.org/s> <http://example.org/p> "42"^^xsd:integer .`)
}

// TestCovReadDatatypeIRIFull covers full IRI datatype.
func TestCovReadDatatypeIRIFull(t *testing.T) {
	mustParse(t, `<http://example.org/s> <http://example.org/p> "42"^^<http://www.w3.org/2001/XMLSchema#integer> .`)
}

// TestCovResolveIRIInvalidBase covers invalid base IRI.
func TestCovResolveIRIInvalidBase(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`<s> <p> <o> .`), WithBase("://invalid"))
	_ = err
}

// TestCovReadBlankNodePropertyListNested covers nested bnode property lists.
func TestCovReadBlankNodePropertyListNested(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p [ ex:a [ ex:b "deep" ] ] .`)
}

// TestCovReadBlankNodeLabelComplex covers complex bnode label.
func TestCovReadBlankNodeLabelComplex(t *testing.T) {
	mustParse(t, `_:b.node-1 <http://example.org/p> "v" .`)
}

// TestCovResolveIRIEmptyFragment covers empty fragment with base.
func TestCovResolveIRIEmptyFragment(t *testing.T) {
	mustParse(t, `@base <http://example.org/> .
<#> <http://example.org/p> "v" .`)
}

// TestCovMatchKeywordCIEdge covers keyword not followed by whitespace.
func TestCovMatchKeywordCIEdge(t *testing.T) {
	mustFail(t, `PREFIXfoo: <http://example.org/>
{ <s> <p> "v" . }`)
}

// TestCovTripleTermInnerError covers error in triple term inner.
func TestCovTripleTermInnerError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p <<( ex:a ex:b ex:c )> . }`)
}

// TestCovReadPrefixNameError covers undeclared prefix error.
func TestCovReadPrefixNameError(t *testing.T) {
	mustFail(t, `nope:s <http://example.org/p> "v" .`)
}

// TestCovReifiedInnerSubjectPrefixed covers prefixed name in reified subject.
func TestCovReifiedInnerSubjectPrefixedTrig2(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p "val" >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectIRIFullTrig covers full IRI in reified object.
func TestCovReifiedInnerObjectIRIFullTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p <http://example.org/o> >> ex:source ex:doc .`)
}

// --- Additional trig serializer coverage ---

// TestCovSerializeRDFTypeShorthandTrig covers "a" shorthand for rdf:type.
func TestCovSerializeRDFTypeShorthandTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("rdf", rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Thing"))
	out := covSerialize(t, g)
	if !strings.Contains(out, " a ") {
		t.Errorf("expected 'a' shorthand, got:\n%s", out)
	}
}

// TestCovSerializeMultiRefBNodeTrig covers bnode referenced multiple times (not inlined).
func TestCovSerializeMultiRefBNodeTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	bn := rdflibgo.NewBNode("ref1")
	s1 := rdflibgo.NewURIRefUnsafe("http://example.org/s1")
	s2 := rdflibgo.NewURIRefUnsafe("http://example.org/s2")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	q := rdflibgo.NewURIRefUnsafe("http://example.org/q")
	g.Add(bn, p, rdflibgo.NewLiteral("val"))
	g.Add(s1, q, bn)
	g.Add(s2, q, bn)
	out := covSerialize(t, g)
	if !strings.Contains(out, "_:ref1") {
		t.Errorf("expected _:ref1, got:\n%s", out)
	}
}

// TestCovSerializeTrackNSTripleTermTrig covers trackNSForTerm with TripleTerm.
func TestCovSerializeTrackNSTripleTermTrig(t *testing.T) {
	ds := graph.NewDataset()
	ds.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	defG := ds.DefaultContext()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	innerS := rdflibgo.NewURIRefUnsafe("http://example.org/a")
	innerP := rdflibgo.NewURIRefUnsafe("http://example.org/b")
	innerO := rdflibgo.NewURIRefUnsafe("http://example.org/c")
	tt := rdflibgo.NewTripleTerm(innerS, innerP, innerO)
	defG.Add(s, p, tt)
	out := covSerializeDS(t, ds)
	if !strings.Contains(out, "ex:") {
		t.Errorf("expected ex: prefix, got:\n%s", out)
	}
}

// TestCovSerializePrefixedDatatypeTrig covers literal with prefixed datatype.
func TestCovSerializePrefixedDatatypeTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("xsd", rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#"))
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	xsdType := rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#dateTime")
	g.Add(s, rdflibgo.RDF.Type, xsdType)
	g.Add(s, p, rdflibgo.NewLiteral("2024-01-01", rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#date"))))
	out := covSerialize(t, g)
	if !strings.Contains(out, "xsd:") {
		t.Errorf("expected xsd: prefix, got:\n%s", out)
	}
}

// TestCovSerializeInlineBNodeMultiPredTrig covers inline bnode with multiple predicates.
func TestCovSerializeInlineBNodeMultiPredTrig(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p [ ex:a "1" ; ex:b "2" ] .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "[") {
		t.Errorf("expected inline bnode, got:\n%s", out)
	}
}

// TestCovSerializeIsValidPrefixNameTrig covers additional prefix name validation.
func TestCovSerializeIsValidPrefixNameTrig(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"a\u0300b", true},
		{"a\u203Fb", true},
		{"a$b", false},
		{"a\u00B7b", true},
	}
	for _, tc := range tests {
		got := isValidPrefixName(tc.name)
		if got != tc.want {
			t.Errorf("isValidPrefixName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCovSerializeIsValidLocalNameTrig covers additional local name validation.
func TestCovSerializeIsValidLocalNameTrig(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"a\u0300b", true},
		{"a\u203Fb", true},
		{"a\u00B7b", true},
		{"a-b", true},
		{"a$b", false},
	}
	for _, tc := range tests {
		got := isValidLocalName(tc.name)
		if got != tc.want {
			t.Errorf("isValidLocalName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCovSerializeIsValidListNilPredsTrig covers isValidList when rest points to non-existent node.
func TestCovSerializeIsValidListNilPredsTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn := rdflibgo.NewBNode("l1")
	g.Add(bn, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	g.Add(bn, rdflibgo.RDF.Rest, rdflibgo.NewBNode("nonexistent"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/list"), bn)
	covSerialize(t, g)
}

// TestCovSerializeIsValidListMultiFirstTrig covers multiple rdf:first values.
func TestCovSerializeIsValidListMultiFirstTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn := rdflibgo.NewBNode("l1")
	g.Add(bn, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	g.Add(bn, rdflibgo.RDF.First, rdflibgo.NewLiteral("b"))
	g.Add(bn, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/list"), bn)
	covSerialize(t, g)
}

// TestCovSerializeDatasetMultipleNamedGraphs covers multiple named graphs.
func TestCovSerializeDatasetMultipleNamedGraphs(t *testing.T) {
	ds := graph.NewDataset()
	ds.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	defG := ds.DefaultContext()
	defG.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("default"))
	ng1 := ds.Graph(rdflibgo.NewURIRefUnsafe("http://example.org/g1"))
	ng1.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s1"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("g1"))
	ng2 := ds.Graph(rdflibgo.NewURIRefUnsafe("http://example.org/g2"))
	ng2.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s2"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("g2"))
	out := covSerializeDS(t, ds)
	if strings.Count(out, "{") < 3 {
		t.Errorf("expected at least 3 graph blocks, got:\n%s", out)
	}
}

// TestCovSerializeDatasetWithBase covers @base in dataset serialization.
func TestCovSerializeDatasetWithBase(t *testing.T) {
	ds := graph.NewDataset()
	defG := ds.DefaultContext()
	defG.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	var buf strings.Builder
	if err := SerializeDataset(ds, &buf, WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "@base") {
		t.Errorf("expected @base, got:\n%s", buf.String())
	}
}

// TestCovSerializeTripleTermBNodeSubjectTrig covers tripleTermStr with BNode subject.
func TestCovSerializeTripleTermBNodeSubjectTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	bn := rdflibgo.NewBNode("ts")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(bn, p, o)
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/q"), tt)
	out := covSerialize(t, g)
	if !strings.Contains(out, "_:ts") {
		t.Errorf("expected _:ts in triple term, got:\n%s", out)
	}
}

// TestCovSerializeInlineBNodeWithRDFTypeTrig covers inline bnode with rdf:type.
func TestCovSerializeInlineBNodeWithRDFTypeTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("rdf", rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	bn := rdflibgo.NewBNode("typed")
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(bn, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Type"))
	g.Add(s, p, bn)
	out := covSerialize(t, g)
	if !strings.Contains(out, "[ a ") {
		t.Errorf("expected inline bnode with 'a', got:\n%s", out)
	}
}

// TestCovTrigLabelBNode covers trigLabel with BNode argument.
func TestCovTrigLabelBNode(t *testing.T) {
	ds := graph.NewDataset()
	// A BNode graph label
	bnID := rdflibgo.NewBNode("glabel")
	ng := ds.Graph(bnID)
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	out := covSerializeDS(t, ds)
	if !strings.Contains(out, "_:") {
		t.Errorf("expected bnode graph label, got:\n%s", out)
	}
}

// TestCovSerializeDatasetSortsGraphs covers graph sorting (default first).
func TestCovSerializeDatasetSortsGraphs(t *testing.T) {
	ds := graph.NewDataset()
	ds.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	ng := ds.Graph(rdflibgo.NewURIRefUnsafe("http://example.org/named"))
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/ns"), rdflibgo.NewURIRefUnsafe("http://example.org/np"), rdflibgo.NewLiteral("named"))
	defG := ds.DefaultContext()
	defG.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("default"))
	out := covSerializeDS(t, ds)
	// Should have both default and named graph blocks
	if !strings.Contains(out, "ex:named {") {
		t.Errorf("expected named graph block, got:\n%s", out)
	}
}

// TestCovSerializeWriteIndentedSkip covers writeIndented skipping serialized subjects.
func TestCovSerializeWriteIndentedSkip(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s1 ex:p "a" .
ex:s2 ex:q "b" .`)
	out := covSerialize(t, g)
	if strings.Count(out, ".") < 2 {
		t.Errorf("expected at least 2 subjects, got:\n%s", out)
	}
}

// --- More parser error coverage ---

// TestCovDirectiveVersionMissingDot covers @version without dot.
func TestCovDirectiveVersionMissingDotTrig(t *testing.T) {
	mustFail(t, `@version "1.2"
<http://example.org/s> <http://example.org/p> "v" .`)
}

// TestCovDirectivePrefixMissingDot covers @prefix without dot.
func TestCovDirectivePrefixMissingDotTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/>
ex:s ex:p "v" .`)
}

// TestCovDirectiveBaseMissingDot covers @base without dot.
func TestCovDirectiveBaseMissingDotTrig(t *testing.T) {
	mustFail(t, `@base <http://example.org/>
<s> <p> "v" .`)
}

// TestCovIsAbsoluteIRINonAlphaScheme covers isAbsoluteIRI edge cases.
func TestCovIsAbsoluteIRINonAlphaSchemeTrig(t *testing.T) {
	mustParse(t, `@base <http://example.org/> .
<noscheme> <http://example.org/p> "v" .`)
	mustParse(t, `@base <http://example.org/> .
<123:foo> <http://example.org/p> "v" .`)
}

// TestCovResolveIRIRefParseError covers resolveIRI when ref IRI parse fails.
func TestCovResolveIRIRefParseErrorTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`@base <http://example.org/> .
<http://[::> <http://example.org/p> "v" .`))
	_ = err
}

// TestCovReadCollectionErrorItem covers error in collection item.
func TestCovReadCollectionErrorItemTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ( "a" ^ ) .`)
}

// TestCovStatementEOFAfterChar covers statement starting with char then EOF.
func TestCovStatementEOFAfterCharTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s`)
}

// TestCovReadBlankNodeLabelUnderscore covers bnode label starting with underscore.
func TestCovReadBlankNodeLabelUnderscoreTrig(t *testing.T) {
	mustParse(t, `_:_test <http://example.org/p> "v" .`)
}

// TestCovReadBlankNodeLabelSpecialChars covers bnode label with special chars.
func TestCovReadBlankNodeLabelSpecialCharsTrig(t *testing.T) {
	mustParse(t, "_:b\u00B7test <http://example.org/p> \"v\" .")
}

// TestCovReadPrefixNameEnd covers prefix name at end of input.
func TestCovReadPrefixNameEndTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:`)
}

// TestCovReadUnicodeEscapeBigU covers \U (8-digit) unicode escape.
func TestCovReadUnicodeEscapeBigUTrig(t *testing.T) {
	mustParse(t, `<http://example.org/\U00000073> <http://example.org/p> "v" .`)
}

// TestCovReadUnicodeEscapeBigUTruncated covers truncated \U escape.
func TestCovReadUnicodeEscapeBigUTruncatedTrig(t *testing.T) {
	mustFail(t, `<http://example.org/\U0000007> <http://example.org/p> "v" .`)
}

// TestCovReadSubjectTripleTermAsSubjectError covers <<( as subject.
func TestCovReadSubjectTripleTermAsSubjectErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ <<( ex:s ex:p ex:o )>> ex:q ex:r . }`)
}

// TestCovCollectionTripleStatementMissingDot covers collection subject missing dot.
func TestCovCollectionTripleStatementMissingDot(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
( "a" "b" ) ex:p "v"`)
}

// TestCovReifiedTripleStatementMissingDot covers reified triple missing dot.
func TestCovReifiedTripleStatementMissingDot(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:source ex:doc`)
}

// TestCovBracketSubjectDotMissingDot covers [] pred-obj missing dot.
func TestCovBracketSubjectMissingDot(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
[] ex:p "v"`)
}

// TestCovSubjectOrGraphMissingDot covers subject followed by pred-obj missing dot.
func TestCovSubjectOrGraphMissingDot(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "v"`)
}

// TestCovSubjectOrGraphInvalidGraphLabel covers invalid graph label type.
func TestCovSubjectOrGraphInvalidGraphLabel(t *testing.T) {
	// This would require a non-URIRef, non-BNode subject followed by {
	// In practice the parser can't produce this since readSubject only returns URIRef or BNode
	// But we can exercise the error path indirectly
	mustFail(t, `@prefix ex: <http://example.org/> .
42 { ex:s ex:p "v" . }`)
}

// TestCovReadGraphLabelPrefixed covers prefixed name graph label.
func TestCovReadGraphLabelPrefixedWithGRAPH(t *testing.T) {
	ds := graph.NewDataset()
	input := `@prefix ex: <http://example.org/> .
GRAPH ex:g1 { ex:s ex:p "v" . }
`
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovReadGraphLabelEmptyBNode covers [] graph label.
func TestCovReadGraphLabelEmptyBNode(t *testing.T) {
	ds := graph.NewDataset()
	input := `@prefix ex: <http://example.org/> .
GRAPH [] { ex:s ex:p "v" . }
`
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovReadGraphLabelError covers invalid graph label after GRAPH.
func TestCovReadGraphLabelInvalid(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
GRAPH "invalid" { ex:s ex:p "v" . }`)
}

// TestCovReadEscapeUnknown covers unknown escape sequence.
func TestCovReadEscapeUnknownTrig(t *testing.T) {
	mustFail(t, `<http://example.org/s> <http://example.org/p> "hello\x" .`)
}

// TestCovReadDatatypeIRIErrorInvalid covers invalid datatype IRI.
func TestCovReadDatatypeIRIErrorInvalid(t *testing.T) {
	mustFail(t, `<http://example.org/s> <http://example.org/p> "v"^^<broken iri> .`)
}

// TestCovPredicateObjectListTrailingSemicolonTrig2 covers trailing ; before .
func TestCovPredicateObjectListTrailingSemicolonTrig2(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p "v" ; . }`)
}

// TestCovBlockTripleStatementMissingDot covers triple inside {} missing dot.
func TestCovBlockTripleStatementMissingDot(t *testing.T) {
	// Missing dot inside graph block — some parsers tolerate this
	mustFail(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p "v" ; ex:q }`)
}

// TestCovSPARQLPrefixUnterminated covers unterminated SPARQL PREFIX IRI.
func TestCovSPARQLPrefixUnterminatedTrig(t *testing.T) {
	mustFail(t, `PREFIX ex: <http://example.org/`)
}

// TestCovSPARQLBaseUnterminated covers unterminated SPARQL BASE IRI.
func TestCovSPARQLBaseUnterminatedTrig(t *testing.T) {
	mustFail(t, `BASE <http://example.org/`)
}

// TestCovTripleTermInnerMissingParen covers missing ) in triple term.
func TestCovTripleTermInnerMissingParenTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p <<( ex:a ex:b ex:c >> . }`)
}

// TestCovTripleTermInnerMissingGT covers missing >> in triple term.
func TestCovTripleTermInnerMissingGTTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p <<( ex:a ex:b ex:c ) . }`)
}

// TestCovTripleTermSubjectEOF covers EOF in triple term subject.
func TestCovTripleTermSubjectEOFTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p <<(`)
}

// TestCovTripleTermSubjectIRI covers IRI as triple term subject (in graph block).
func TestCovTripleTermSubjectIRIInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p <<( <http://example.org/a> ex:b ex:c )>> . }`)
}

// TestCovCollectionTripleStatementCollectionError covers error in collection.
func TestCovCollectionTripleStatementCollectionError(t *testing.T) {
	mustFail(t, `( ^ ) <http://example.org/p> "v" .`)
}

// TestCovReifiedInnerSubjectEOF covers EOF in reified inner subject.
func TestCovReifiedInnerSubjectEOFTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<<`)
}

// TestCovReifiedInnerObjectEOF covers EOF in reified inner object.
func TestCovReifiedInnerObjectEOFTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p`)
}

// TestCovReifiedInnerObjectIRIFull covers full IRI as reified inner object.
func TestCovReifiedInnerObjectIRIFullTrig2(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p <http://example.org/o> >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectNumericDot covers numeric with dot as reified inner object.
func TestCovReifiedInnerObjectNumericDotTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p 3.14 >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectNumericExp covers numeric with exponent.
func TestCovReifiedInnerObjectNumericExpTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p 1e2 >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerObjectBNodePropListError covers bnode prop list in reified object.
func TestCovReifiedInnerObjectBNodePropListErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p [ex:a ex:b] >> ex:source ex:doc . }`)
}

// TestCovReifiedInnerSubjectPrefixedInBlock covers prefixed subject in reified triple in block.
func TestCovReifiedInnerSubjectPrefixedInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p "val" >> ex:source ex:doc . }`)
}

// TestCovAnnotationInBlock covers annotation inside graph block.
func TestCovAnnotationInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o {| ex:source ex:doc ; ex:date "2024" |} . }`)
}

// TestCovReifierErrorInBlock covers reifier error inside graph block.
func TestCovReifierErrorInBlock(t *testing.T) {
	// ~ at end of input (inside block, missing reifier ID or dot)
	mustFail(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o ~`)
}

// TestCovReadGraphLabelIRI covers IRI graph label after GRAPH.
func TestCovReadGraphLabelIRITrig(t *testing.T) {
	ds := graph.NewDataset()
	input := `GRAPH <http://example.org/g1> { <http://example.org/s> <http://example.org/p> "v" . }
`
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovReadGraphLabelBNodeLabel covers _:bnode graph label after GRAPH.
func TestCovReadGraphLabelBNodeLabelTrig(t *testing.T) {
	ds := graph.NewDataset()
	input := `@prefix ex: <http://example.org/> .
GRAPH _:g1 { ex:s ex:p "v" . }
`
	if err := ParseDataset(ds, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestCovReadSubjectBNodePropList covers bnode property list as subject in block.
func TestCovReadSubjectBNodePropListInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ [ex:a "v1"] ex:p "v2" . }`)
}

// TestCovReadObjectBNodePropList covers bnode property list as object in block.
func TestCovReadObjectBNodePropListInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p [ex:a "v1"] . }`)
}

// TestCovReadCollectionInBlock covers collection in graph block.
func TestCovReadCollectionInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ( "a" "b" "c" ) . }`)
}

// TestCovReadLiteralSingleQuoteInBlock covers single-quoted string in block.
func TestCovReadLiteralSingleQuoteInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p 'hello' . }`)
}

// TestCovReadLiteralTripleQuoteInBlock covers triple-quoted string in block.
func TestCovReadLiteralTripleQuoteInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p """multi
line""" . }`)
}

// TestCovReadObjectBoolean covers boolean objects.
func TestCovReadObjectBooleanInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p true . ex:s2 ex:q false . }`)
}

// TestCovReadObjectNumeric covers various numeric literals.
func TestCovReadObjectNumericInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p 42 . ex:s2 ex:q 3.14 . ex:s3 ex:r 1e10 . }`)
}

// TestCovBracketSubjectPropListInBlock covers [...] as subject inside graph block.
func TestCovBracketSubjectPropListInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ [ ex:a "v" ] ex:p "v2" . }`)
}

// TestCovReadBlankNodePropertyListInBlock covers nested bnode prop list.
func TestCovReadBlankNodePropertyListInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p [ ex:a [ ex:b "deep" ] ] . }`)
}

// TestCovReadSubjectBNodeInBlock covers bnode as subject inside block.
func TestCovReadSubjectBNodeInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ _:b1 ex:p "v" . }`)
}

// TestCovReifiedTripleInsideBlock covers reified triple inside graph block.
func TestCovReifiedTripleInsideBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p ex:o >> ex:source ex:doc . }`)
}

// TestCovCollectionInsideBlock covers collection as subject inside graph block.
func TestCovCollectionInsideBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ( "a" "b" ) ex:p "v" . }`)
}

// TestCovReadBlankNodeLabelInBlock covers bnode label in graph block.
func TestCovReadBlankNodeLabelInBlock(t *testing.T) {
	mustParse(t, `{ _:b1.2 <http://example.org/p> "v" . }`)
}

// TestCovReadLocalNameEdge covers local name with dots and dashes.
func TestCovReadLocalNameEdgeTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:a.b-c ex:p "v" . }`)
}

// TestCovMatchKeywordCIAtEOF covers keyword at EOF.
func TestCovMatchKeywordCIAtEOFTrig(t *testing.T) {
	mustFail(t, `PREFIX`)
}

// TestCovMatchKeywordCIFollowedByAngle covers BASE followed by <.
func TestCovMatchKeywordCIFollowedByAngle(t *testing.T) {
	mustParse(t, `BASE<http://example.org/>
<s> <p> "v" .`)
}

// TestCovMatchKeywordCIFollowedByColon covers PREFIX followed by :.
func TestCovMatchKeywordCIFollowedByColon(t *testing.T) {
	mustParse(t, `PREFIX:<http://example.org/>
:s :p "v" .`)
}

// TestCovReadBlankNodeLabelComplex2 covers bnode label with middle dot.
func TestCovReadBlankNodeLabelComplex2Trig(t *testing.T) {
	mustParse(t, "_:b\u00B7test <http://example.org/p> \"v\" .")
}

// TestCovBracketSubjectOrGraphError covers error in bnode prop list subject.
func TestCovBracketSubjectOrGraphError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
[ ^ ] ex:p "v" .`)
}

// TestCovSerializePrefixInvalidTrig covers invalid prefix name.
func TestCovSerializePrefixInvalidTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("1bad", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))
	out := covSerialize(t, g)
	if strings.Contains(out, "1bad:") {
		t.Errorf("should not use invalid prefix, got:\n%s", out)
	}
}

// TestCovTripleTermSubjectPrefixedName covers readTripleTermSubject with prefixed name.
func TestCovTripleTermSubjectPrefixedName(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ex:b ex:c )>> .`)
}

// TestCovReifierIDPrefixedName covers readReifierID with prefixed name.
func TestCovReifierIDPrefixedName(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ ex:r1 .`)
}

// TestCovReifierIDBNode covers readReifierID with blank node.
func TestCovReifierIDBNode(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ _:r1 .`)
}

// TestCovReifierAnonymousWithAnnotation covers anonymous reifier with annotation block.
func TestCovReifierAnonymousWithAnnotation(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ {| ex:source ex:doc |} .`)
}

// TestCovAnnotationBlockWithoutReifier covers {| |} without ~ (bare annotation).
func TestCovAnnotationBlockWithoutReifier(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o {| ex:source ex:doc |} .`)
}

// TestCovReifierWithAnnotationNamed covers reifier with IRI ID and annotation.
func TestCovReifierWithAnnotationNamed(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ <http://example.org/r1> {| ex:source ex:doc |} .`)
}

// TestCovReadEscapeUnknown2 covers unknown escape character error.
func TestCovReadEscapeUnknown2Trig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "\z" .`)
}

// TestCovReadUnicodeEscapeInvalidHex covers invalid hex in unicode escape.
func TestCovReadUnicodeEscapeInvalidHexTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "\uZZZZ" .`)
}

// TestCovReadUnicodeEscapeTruncated covers truncated unicode escape.
func TestCovReadUnicodeEscapeTruncatedTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "\u00" .`)
}

// TestCovReadUnicodeEscapeSurrogate covers surrogate in unicode escape.
func TestCovReadUnicodeEscapeSurrogateTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "\uD800" .`)
}

// TestCovReadUnicodeEscape8DigitInvalid covers invalid 8-digit unicode escape.
func TestCovReadUnicodeEscape8DigitInvalidTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "\UZZZZZZZZ" .`)
}

// TestCovCollectionStandaloneError covers standalone collection with dot.
func TestCovCollectionStandaloneErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
( ex:a ) .`)
}

// TestCovCollectionAsGraphLabel covers collection followed by { error.
func TestCovCollectionAsGraphLabelTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
( ex:a ) { ex:s ex:p ex:o . }`)
}

// TestCovReadSubjectCollectionAsSubject covers collection used as subject with predicates.
func TestCovReadSubjectCollectionAsSubjectTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
( ex:a ex:b ) ex:p ex:o .`)
}

// TestCovObjectListAnnotationError covers annotation error in object list continuation.
func TestCovObjectListAnnotationErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o1, ex:o2 ~ `)
}

// TestCovReadBlankNodeLabelTrailingDots covers bnode label with trailing dots stripped.
func TestCovReadBlankNodeLabelTrailingDotsTrig(t *testing.T) {
	// _:b1.. should parse as _:b1 followed by two dots
	mustFail(t, `_:b1.. <http://example.org/p> "v" .`)
}

// TestCovReadBlankNodeLabelEmptyAfterPrefix covers empty label after _:.
func TestCovReadBlankNodeLabelEmptyTrig(t *testing.T) {
	mustFail(t, `_: <http://example.org/p> "v" .`)
}

// TestCovUnescapeIRITruncatedU covers truncated \u escape in IRI.
func TestCovUnescapeIRITruncatedUTrig(t *testing.T) {
	mustFail(t, `<http://example.org/\u00> <http://example.org/p> "v" .`)
}

// TestCovUnescapeIRIInvalidU covers invalid \u escape in IRI.
func TestCovUnescapeIRIInvalidUTrig(t *testing.T) {
	mustFail(t, `<http://example.org/\uZZZZ> <http://example.org/p> "v" .`)
}

// TestCovUnescapeIRISurrogateU covers surrogate \u escape in IRI.
func TestCovUnescapeIRISurrogateUTrig(t *testing.T) {
	mustFail(t, `<http://example.org/\uD800> <http://example.org/p> "v" .`)
}

// TestCovUnescapeIRITruncatedBigU covers truncated \U escape in IRI.
func TestCovUnescapeIRITruncatedBigUTrig(t *testing.T) {
	mustFail(t, `<http://example.org/\U000000> <http://example.org/p> "v" .`)
}

// TestCovUnescapeIRIInvalidBigU covers invalid \U escape in IRI.
func TestCovUnescapeIRIInvalidBigUTrig(t *testing.T) {
	mustFail(t, `<http://example.org/\UZZZZZZZZ> <http://example.org/p> "v" .`)
}

// TestCovResolveIRIWithBase covers resolveIRI with base and relative IRI.
func TestCovResolveIRIWithBaseTrig(t *testing.T) {
	mustParse(t, `@base <http://example.org/> .
<s> <http://example.org/p> "v" .`)
}

// TestCovReifiedTriplePredicateError covers predicate error in reified triple.
func TestCovReifiedTriplePredicateErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< ex:s ^invalid ex:o >> ex:p "v" .`)
}

// TestCovReadAnnotationBlockEmpty covers empty annotation block error.
func TestCovReadAnnotationBlockEmptyTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o {| |} .`)
}

// TestCovBlockTripleCollectionSubjectTrig covers collection as subject inside block.
func TestCovBlockTripleCollectionSubjectTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ( ex:a ex:b ) ex:p ex:o . }`)
}

// TestCovBlockTripleReifiedSubjectDotTrig covers reified triple as subject with dot in block.
func TestCovBlockTripleReifiedSubjectDotTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p ex:o >> . }`)
}

// TestCovSubjectOrGraphDefaultTypeSwitch covers the default branch in subjectOrGraph.
func TestCovSubjectOrGraphBNodeGraphBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
_:g1 { ex:s ex:p ex:o . }`)
}

// TestCovBracketSubjectDotOnly covers [] . (empty bnode subject with just dot).
func TestCovBracketSubjectDotOnlyTrig2(t *testing.T) {
	mustParse(t, `[] .`)
}

// TestCovPredicateObjectListErrorInContinuation covers error after ; in pred-obj list.
func TestCovPredicateObjectListErrorContinuation(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p1 "v1" ; ^bad "v2" .`)
}

// TestCovObjectListReadObjectError covers error reading object in comma-separated list.
func TestCovObjectListReadObjectErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "v1", <bad iri .`)
}

// TestCovReadSubjectPrefixedName covers readSubject with prefixed name subject.
func TestCovReadSubjectPrefixedNameTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p "v" . }`)
}

// TestCovReadSubjectCollectionInBlock covers collection as subject in block.
func TestCovReadSubjectCollectionInBlockTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ( ex:a ) ex:p "v" . }`)
}

// TestCovReifiedTripleWithAnnotationInsideBlock covers reified triple + annotation in block.
func TestCovReifiedTripleWithAnnotationInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o ~ ex:r1 {| ex:source ex:doc |} . }`)
}

// TestCovReadPrefixNameTrailingDots covers prefix name with trailing dots stripped.
func TestCovReadPrefixNameTrailingDotsTrig(t *testing.T) {
	// prefix name like "ex.." should have trailing dots stripped
	mustFail(t, `@prefix ex..: <http://example.org/> .`)
}

// TestCovReadLocalNameEscapedChar covers escaped character in local name.
func TestCovReadLocalNameEscapedCharTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:hello\.world .`)
}

// TestCovSerializeListStrError covers listStr error path (list with error in objectStr).
func TestCovSerializeListStrErrorTrig(t *testing.T) {
	// Build an RDF list with a triple term that triggers objectStr recursion
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	// Create list: (<<(s p o)>>)
	innerS := rdflibgo.NewURIRefUnsafe("http://example.org/a")
	innerP := rdflibgo.NewURIRefUnsafe("http://example.org/b")
	innerO := rdflibgo.NewURIRefUnsafe("http://example.org/c")
	tt := rdflibgo.NewTripleTerm(innerS, innerP, innerO)

	listHead := rdflibgo.NewBNode("list1")
	g.Add(listHead, rdflibgo.RDF.First, tt)
	g.Add(listHead, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)

	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/hasItems")
	g.Add(s, p, listHead)

	out := covSerialize(t, g)
	if !strings.Contains(out, "ex:a") {
		t.Errorf("expected list serialization, got:\n%s", out)
	}
}

// TestCovSerializeWritePredicatesEmpty covers writePredicates with no predicates.
func TestCovSerializeWritePredicatesEmptyTrig(t *testing.T) {
	// BNode subject with no predicates triggers writePredicates empty path
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	bn := rdflibgo.NewBNode("empty")
	g.Add(s, p, bn)
	// bn is referenced as object but has no predicates itself
	out := covSerialize(t, g)
	if !strings.Contains(out, "_:empty") {
		t.Errorf("expected bnode reference, got:\n%s", out)
	}
}

// TestCovSerializeMultipleObjectsSamePred covers multiple objects on same predicate.
func TestCovSerializeMultipleObjectsSamePredTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("a"))
	g.Add(s, p, rdflibgo.NewLiteral("b"))
	g.Add(s, p, rdflibgo.NewLiteral("c"))
	out := covSerialize(t, g)
	if !strings.Contains(out, ",") {
		t.Errorf("expected comma-separated objects, got:\n%s", out)
	}
}

// TestCovSerializeTripleTermObjError covers tripleTermStr error path via objectStr.
func TestCovSerializeTripleTermObjErrorTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	innerTT := rdflibgo.NewTripleTerm(s, p, rdflibgo.NewLiteral("inner"))
	outerTT := rdflibgo.NewTripleTerm(s, p, innerTT)
	outerP := rdflibgo.NewURIRefUnsafe("http://example.org/q")
	g.Add(s, outerP, outerTT)
	out := covSerialize(t, g)
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected nested triple term, got:\n%s", out)
	}
}

// TestCovMarkListNodesMissedRest covers markListNodes when rests is empty.
func TestCovMarkListNodesMissedRestTrig(t *testing.T) {
	// Build a malformed list: first but no rest -> markListNodes breaks on empty rests
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	listHead := rdflibgo.NewBNode("badlist")
	g.Add(listHead, rdflibgo.RDF.First, rdflibgo.NewLiteral("only"))
	// No rdf:rest triple — markListNodes should break
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, listHead)
	out := covSerialize(t, g)
	_ = out // just ensure it doesn't panic
}

// TestCovSPARQLVersionTrig covers VERSION keyword (SPARQL-style) in trig.
func TestCovSPARQLVersionTrig(t *testing.T) {
	mustParse(t, `VERSION "1.2"
@prefix ex: <http://example.org/> .
ex:s ex:p "v" .`)
}

// TestCovSPARQLBaseTrig covers BASE keyword (SPARQL-style) in trig.
func TestCovSPARQLBaseTrig(t *testing.T) {
	mustParse(t, `BASE <http://example.org/>
PREFIX ex: <http://example.org/>
ex:s ex:p "v" .`)
}

// TestCovReadReifiedInnerSubjectBNode covers bnode in reified inner subject.
func TestCovReadReifiedInnerSubjectBNodeTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< _:b1 ex:p ex:o >> ex:q "v" .`)
}

// TestCovReadReifiedInnerSubjectEmptyBNode covers [] in reified inner subject.
func TestCovReadReifiedInnerSubjectEmptyBNodeTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< [] ex:p ex:o >> ex:q "v" .`)
}

// TestCovReadReifiedInnerSubjectPrefixed covers prefixed name in reified inner subject.
func TestCovReadReifiedInnerSubjectPrefixedTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:q "v" .`)
}

// TestCovReadReifiedInnerObjectTripleTerm covers triple term as reified inner object.
func TestCovReadReifiedInnerObjectTripleTermTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p <<( ex:a ex:b ex:c )>> >> ex:q "v" .`)
}

// TestCovReifiedTripleStatementPredicateList covers reified triple followed by predicate-object list.
func TestCovReifiedTripleStatementPredicateListTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:q "v" ; ex:r "w" .`)
}

// TestCovReadAnnotationBlockPredicateError covers error in annotation block predicate.
func TestCovReadAnnotationBlockPredicateErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o {| ^bad ex:doc |} .`)
}

// TestCovObjectBooleanTrue covers boolean true as object.
func TestCovObjectBooleanTrueTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p true .`)
}

// TestCovObjectBooleanFalse covers boolean false as object.
func TestCovObjectBooleanFalseTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p false .`)
}

// TestCovReadObjectPrefixedName covers prefixed name as object.
func TestCovReadObjectPrefixedNameTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o .`)
}

// TestCovReadObjectNumericDecimal covers decimal numeric literal as object.
func TestCovReadObjectNumericDecimalTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p 3.14 .`)
}

// TestCovSerializeDatasetLargeOutput covers SerializeDataset with many triples exercising
// the writeIndented/writePredicates code paths.
func TestCovSerializeDatasetLargeOutputTrig(t *testing.T) {
	ds := graph.NewDataset()
	g := ds.DefaultContext()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	for i := 0; i < 20; i++ {
		si := rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		g.Add(si, p, rdflibgo.NewLiteral(fmt.Sprintf("v%d", i)))
	}
	var buf strings.Builder
	if err := SerializeDataset(ds, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ex:s0") {
		t.Error("expected subject in output")
	}
}

// TestCovSerializeInlineBNodeWithListHead covers inline bnode that is also a list head.
func TestCovSerializeInlineBNodeWithListHeadTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	// Build a list: _:list -> first: "a", rest: nil
	listHead := rdflibgo.NewBNode("lst")
	g.Add(listHead, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	g.Add(listHead, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)

	// Reference the list head from another subject
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/items")
	g.Add(s, p, listHead)

	out := covSerialize(t, g)
	if !strings.Contains(out, "( ") {
		t.Errorf("expected list syntax, got:\n%s", out)
	}
}

// TestCovBlockTripleBNodePropListSubject covers [...] as subject inside block.
func TestCovBlockTripleBNodePropListSubjectTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ [ ex:name "Alice" ] ex:knows ex:Bob . }`)
}

// TestCovReadCollectionErrorInObject covers error reading object inside collection.
func TestCovReadCollectionErrorInObjectTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
( <bad iri ) ex:p "v" .`)
}

// TestCovOrderSubjectsBNodeVsURI covers ordering where BNode subjects come after URIs.
func TestCovOrderSubjectsBNodeVsURITrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Bind("rdfs", rdflibgo.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#"))

	// Add a class subject (should come first)
	cls := rdflibgo.NewURIRefUnsafe("http://example.org/MyClass")
	g.Add(cls, rdflibgo.RDF.Type, rdflibgo.RDFS.Class)
	g.Add(cls, rdflibgo.RDFS.Label, rdflibgo.NewLiteral("My Class"))

	// Add a bnode subject (should come after URIs)
	bn := rdflibgo.NewBNode("bn1")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(bn, p, rdflibgo.NewLiteral("bnode val"))

	// Add a regular URI subject
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, p, rdflibgo.NewLiteral("uri val"))

	out := covSerialize(t, g)
	// Class should appear, and output should have multiple subjects
	if !strings.Contains(out, "MyClass") {
		t.Errorf("expected class subject, got:\n%s", out)
	}
}

// TestCovValidateIRIInvalidChar covers validateIRI with invalid character.
func TestCovValidateIRIInvalidCharTrig(t *testing.T) {
	mustFail(t, "<http://example.org/a{b> <http://example.org/p> \"v\" .")
}

// TestCovReadIRIUnterminated covers unterminated IRI.
func TestCovReadIRIUnterminatedTrig(t *testing.T) {
	mustFail(t, `<http://example.org/s`)
}

// TestCovReadIRIInvalidInlineChar covers invalid inline char in IRI.
func TestCovReadIRIInvalidInlineCharTrig(t *testing.T) {
	mustFail(t, "<http://example.org/a\x01b> <http://example.org/p> \"v\" .")
}

// TestCovBlockTripleStandaloneCollectionError covers standalone collection error inside block.
func TestCovBlockTripleStandaloneCollectionErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ ( ex:a ) . }`)
}

// TestCovBlockTripleStandaloneCollectionCloseBrace covers standalone collection followed by }.
func TestCovBlockTripleStandaloneCollectionCloseBraceTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{ ( ex:a ) }`)
}

// TestCovBracketSubjectMissingDot covers [] pred obj without dot.
func TestCovBracketSubjectMissingDotTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
[] ex:p "v"`)
}

// TestCovCollectionMissingDotError covers collection pred obj without dot.
func TestCovCollectionMissingDotErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
( ex:a ) ex:p "v"`)
}

// TestCovBNodePropListDotInBlock covers [pred obj] . inside block.
func TestCovBNodePropListDotInBlockTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ [ ex:p "v" ] . }`)
}

// TestCovBNodePropListNoDotInBlock covers [pred obj] } (no dot before close brace).
func TestCovBNodePropListNoDotInBlockTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ [ ex:p "v" ] }`)
}

// TestCovEmptyBNodeStandaloneInBlock covers [] } (empty bnode, no pred, close brace).
func TestCovEmptyBNodeStandaloneInBlockTrig(t *testing.T) {
	mustParse(t, `{ [] }`)
}

// TestCovReifiedTripleMissingDot covers reified triple pred obj without dot.
func TestCovReifiedTripleMissingDotTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:q "v"`)
}

// TestCovTripleTermInnerPredicateError covers predicate error in triple term.
func TestCovTripleTermInnerPredicateErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ^bad ex:c )>> .`)
}

// TestCovTripleTermInnerObjectError covers object error in triple term.
func TestCovTripleTermInnerObjectErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ex:b <bad iri )>> .`)
}

// TestCovTripleTermSubjectBNodeLabel covers bnode label as triple term subject.
func TestCovTripleTermSubjectBNodeLabelTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( _:b1 ex:b ex:c )>> .`)
}

// TestCovReadGraphLabelPrefixedName covers prefixed name as graph label.
func TestCovReadGraphLabelPrefixedNameTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
GRAPH ex:g1 { ex:s ex:p "v" . }`)
}

// TestCovReadGraphLabelEmptyBNode covers [] as graph label.
func TestCovReadGraphLabelEmptyBNodeTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
GRAPH [] { ex:s ex:p "v" . }`)
}

// TestCovReadGraphLabelBNode covers _:x as graph label.
func TestCovReadGraphLabelBNodeLabel(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
GRAPH _:g1 { ex:s ex:p "v" . }`)
}

// TestCovReadAnnotationsReifierAnonymousFollowedBySemicolon covers ~ ; pattern.
func TestCovReadAnnotationsReifierAnonymousSemicolon(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ ; ex:q "v" .`)
}

// TestCovReadAnnotationsReifierAnonymousFollowedByClosingBrace covers ~ } pattern.
func TestCovReadAnnotationsReifierAnonymousCloseBrace(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ ex:s ex:p ex:o ~ }`)
}

// TestCovReadAnnotationsReifierAnonymousFollowedByComma covers ~ , pattern.
func TestCovReadAnnotationsReifierAnonymousComma(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o1 ~ , ex:o2 .`)
}

// TestCovReadAnnotationsReifierAnonymousFollowedByTilde covers ~ ~ pattern (chained reifiers).
func TestCovReadAnnotationsReifierAnonymousTilde(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ ~ .`)
}

// TestCovReadAnnotationsReifierAnonymousFollowedByPipe covers ~ |} pattern.
func TestCovReadAnnotationsReifierAnonymousPipe(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p1 ex:o1 {| ex:p2 ex:o2 ~ |} .`)
}

// TestCovReadAnnotationsReifierAnonymousFollowedByDot covers ~ . pattern.
func TestCovReadAnnotationsReifierAnonymousDot(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ .`)
}

// TestCovReadEscapeFormFeed covers \f escape in string.
func TestCovReadEscapeFormFeedTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "hello\fworld" .`)
}

// TestCovReadEscapeBackspace covers \b escape in string.
func TestCovReadEscapeBackspaceTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "hello\bworld" .`)
}

// TestCovReadEscapeSingleQuote covers \' escape in string.
func TestCovReadEscapeSingleQuoteTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "hello\'world" .`)
}

// TestCovReadSubjectReifiedTriple covers << s p o >> as subject in readSubject.
func TestCovReadSubjectReifiedTripleInBlock(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
{ << ex:s ex:p ex:o >> ex:q "v" . }`)
}

// TestCovSerializeObjectStrDefaultBranch covers objectStr with URIRef that has no prefix match.
func TestCovSerializeObjectStrDefaultBranchTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	// No prefix bindings - forces full IRI output
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://other.example/o")
	g.Add(s, p, o)
	out := covSerialize(t, g)
	if !strings.Contains(out, "<http://other.example/o>") {
		t.Errorf("expected full IRI, got:\n%s", out)
	}
}

// TestCovSerializeInlineBNodeMultiPredicate covers inlineBNode with multiple predicates and objects.
func TestCovSerializeInlineBNodeMultiPredicateTrig2(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	bn := rdflibgo.NewBNode("inner")
	g.Add(s, p, bn)

	p1 := rdflibgo.NewURIRefUnsafe("http://example.org/name")
	p2 := rdflibgo.NewURIRefUnsafe("http://example.org/value")
	g.Add(bn, p1, rdflibgo.NewLiteral("Alice"))
	g.Add(bn, p2, rdflibgo.NewLiteral("val1"))
	g.Add(bn, p2, rdflibgo.NewLiteral("val2"))

	out := covSerialize(t, g)
	if !strings.Contains(out, "[") {
		t.Errorf("expected inline bnode, got:\n%s", out)
	}
}

// TestCovReadBlankNodeLabelInvalidStart covers invalid start char in bnode label.
func TestCovReadBlankNodeLabelInvalidStartTrig(t *testing.T) {
	mustFail(t, `_:! <http://example.org/p> "v" .`)
}

// TestCovSerializeLocalNameSpecialChars covers local names that aren't valid.
func TestCovSerializeLocalNameSpecialCharsTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/has space")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))
	out := covSerialize(t, g)
	// Should fall back to full IRI since "has space" is not a valid local name
	if !strings.Contains(out, "<http://example.org/has space>") {
		t.Errorf("expected full IRI for invalid local name, got:\n%s", out)
	}
}

// TestCovReadReifiedInnerSubjectReified covers nested reified triple in reified inner subject.
func TestCovReadReifiedInnerSubjectReifiedTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< << ex:a ex:b ex:c >> ex:p ex:o >> ex:q "v" .`)
}

// TestCovReadReifiedInnerObjectBNode covers bnode as reified inner object.
func TestCovReadReifiedInnerObjectBNodeTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p _:b1 >> ex:q "v" .`)
}

// TestCovReadReifiedInnerObjectLiteral covers literal as reified inner object.
func TestCovReadReifiedInnerObjectLiteralTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p "hello" >> ex:q "v" .`)
}

// TestCovReadReifiedInnerObjectPrefixed covers prefixed name as reified inner object.
func TestCovReadReifiedInnerObjectPrefixedTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:q "v" .`)
}

// TestCovReadReifierIDEOF covers readReifierID at EOF.
func TestCovReadReifierIDEOFTrig(t *testing.T) {
	// Reifier tilde at end of input with no ID and no valid next char => error
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~`)
}

// TestCovReadTripleTermSubjectPrefixedError covers readTripleTermSubject error.
func TestCovReadTripleTermSubjectPrefixedErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( undefined:name ex:b ex:c )>> .`)
}

// TestCovMatchKeywordCINotKeyword covers matchKeywordCI returning false for non-matching text.
func TestCovMatchKeywordCINotKeywordTrig(t *testing.T) {
	// "PREFIXED" starts with "PREFIX" but is followed by 'E' not delimiter
	mustParse(t, `@prefix PREFIXED: <http://example.org/> .
PREFIXED:s PREFIXED:p "v" .`)
}

// TestCovCollectionMissingDotAtEOF covers collection pred obj at EOF (missing dot).
func TestCovCollectionMissingDotAtEOFTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
( ex:a ) ex:p "v"`)
}

// TestCovReadSubjectEOF covers readSubject at EOF.
func TestCovReadSubjectEOFTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
{`)
}

// TestCovReadCollectionBNodeObjects covers collection with bnode objects.
func TestCovReadCollectionBNodeObjectsTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ( _:a _:b ) .`)
}

// TestCovReadGraphLabelBNodePropListError covers [pred obj] as graph label error.
func TestCovReadGraphLabelBNodePropListErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
GRAPH [ ex:p "v" ] { ex:s ex:p "v" . }`)
}

// TestCovSubjectOrGraphInvalidGraphLabelType covers the default branch in subjectOrGraph type switch.
// In TriG, the subjectOrGraph function has a type switch for graph labels:
// case URIRef (covered), case BNode (covered), default: error.
// A TripleTerm subject followed by { would trigger it, but readSubject returns TripleTerm
// as a Subject (which is neither URIRef nor BNode) only for reified triples... actually
// reified triples return BNode. So this default branch may be unreachable in practice.

// TestCovStatementEOFAfterPrefix covers statement returning nil at end of input.
func TestCovStatementEOFAfterPrefixTrig(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .`)
}

// TestCovBracketSubjectPredicateErrorTrig covers error in predicate after [] subject.
func TestCovBracketSubjectPredicateErrorTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
[] ^bad "v" .`)
}

// TestCovReifiedTripleStatementMissingDotTrig covers missing dot error.
func TestCovReifiedTripleStatementMissingDotAtEOFTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:q "v"`)
}

// TestCovBlockTripleStatementEOFError covers EOF inside block.
func TestCovBlockTripleStatementEOFErrorTrig(t *testing.T) {
	mustFail(t, `{ <http://example.org/s> <http://example.org/p>`)
}

// TestCovObjectListAnnotationErrorInContinuationTrig covers annotation error after second object.
func TestCovObjectListAnnotationErrorInContinuationTrig(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o1, ex:o2 {| ^bad |} .`)
}

// TestCovSerializeWritePredicatesMultiWithLabel covers writePredicates with rdfs:label predicate.
func TestCovSerializeWritePredicatesMultiWithLabelTrig(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Bind("rdfs", rdflibgo.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.RDFS.Class)
	g.Add(s, rdflibgo.RDFS.Label, rdflibgo.NewLiteral("MyThing"))
	p := rdflibgo.NewURIRefUnsafe("http://example.org/extra")
	g.Add(s, p, rdflibgo.NewLiteral("val"))
	out := covSerialize(t, g)
	if !strings.Contains(out, "a ") || !strings.Contains(out, "rdfs:label") {
		t.Errorf("expected rdf:type 'a' and rdfs:label, got:\n%s", out)
	}
}
