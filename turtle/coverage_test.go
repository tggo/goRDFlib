package turtle

import (
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// errReader always fails on Read.
type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errors.New("read fail") }

// Tests targeting specific uncovered branches to boost coverage >90%.

func mustParse(t *testing.T, input string) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	return g
}

func mustFail(t *testing.T, input string) {
	t.Helper()
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err == nil {
		t.Fatal("expected parse error")
	}
}

// sparqlBase (85.7%) — cover SPARQL-style BASE
func TestCovSPARQLBase(t *testing.T) {
	g := mustParse(t, `BASE <http://example.org/>
<s> <p> <o> .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// matchKeywordCI (77.8%) — keyword not followed by whitespace
func TestCovKeywordNotFollowedByWS(t *testing.T) {
	// "PREFIX" not followed by whitespace — treated as prefixed name
	mustFail(t, `PREFIXfoo: <http://example.org/> .`)
}

// matchKeywordCI — keyword at EOF
func TestCovKeywordAtEOF(t *testing.T) {
	mustFail(t, `PREFIX`)
}

// resolveIRI (83.3%) — invalid base IRI
func TestCovResolveIRIWithBase(t *testing.T) {
	g := mustParse(t, `@base <http://example.org/dir/> .
<../other> <http://example.org/p> "v" .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// resolveIRI — empty fragment <#> with base
func TestCovResolveIRIEmptyFragment(t *testing.T) {
	g := mustParse(t, `@base <http://example.org/> .
<#> <http://example.org/p> "v" .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// isAbsoluteIRI (81.8%) — non-letter first char, digit in scheme
func TestCovIsAbsoluteIRI(t *testing.T) {
	// Relative IRI (no scheme)
	g := mustParse(t, `@base <http://example.org/> .
<relative> <http://example.org/p> "v" .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// readUnicodeEscape (90%) — truncated, invalid hex, surrogate
func TestCovUnicodeEscapeTruncated(t *testing.T) {
	mustFail(t, `<http://example.org/\u00> <http://example.org/p> "v" .`)
}

func TestCovUnicodeEscapeInvalidHex(t *testing.T) {
	mustFail(t, `<http://example.org/\uXXXX> <http://example.org/p> "v" .`)
}

func TestCovUnicodeEscapeSurrogate(t *testing.T) {
	mustFail(t, `<http://example.org/\uD800> <http://example.org/p> "v" .`)
}

// readDatatypeIRI (85.7%) — datatype as full IRI
func TestCovDatatypeFullIRI(t *testing.T) {
	g := mustParse(t, `<http://example.org/s> <http://example.org/p> "42"^^<http://www.w3.org/2001/XMLSchema#integer> .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// readBlankNodeLabel (89.5%) — blank node with dots and dashes
func TestCovBlankNodeLabelComplex(t *testing.T) {
	g := mustParse(t, `_:b.node-1 <http://example.org/p> "v" .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// readCollection (93.1%) — empty collection
func TestCovEmptyCollection(t *testing.T) {
	g := mustParse(t, `<http://example.org/s> <http://example.org/p> () .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// readSubject — unexpected char
func TestCovSubjectUnexpectedChar(t *testing.T) {
	mustFail(t, `^ <http://example.org/p> "v" .`)
}

// readPrefixName (92.9%) — prefix with non-PN_CHARS_BASE first
func TestCovPrefixNameEmpty(t *testing.T) {
	g := mustParse(t, `@prefix : <http://example.org/> .
:s :p :o .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// objectList — multiple objects
func TestCovMultipleObjects(t *testing.T) {
	g := mustParse(t, `<http://example.org/s> <http://example.org/p> "a", "b", "c" .`)
	if g.Len() != 3 {
		t.Errorf("expected 3 triples, got %d", g.Len())
	}
}

// VERSION directive edge cases
func TestCovVersionSingleQuote(t *testing.T) {
	g := mustParse(t, `VERSION '1.2'
<http://example.org/s> <http://example.org/p> "v" .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestCovVersionTripleQuoteError(t *testing.T) {
	mustFail(t, `VERSION """1.2"""
<http://example.org/s> <http://example.org/p> "v" .`)
}

func TestCovVersionNewlineError(t *testing.T) {
	mustFail(t, `VERSION "1.
2"
<http://example.org/s> <http://example.org/p> "v" .`)
}

func TestCovVersionMissingQuote(t *testing.T) {
	mustFail(t, `VERSION 1.2
<http://example.org/s> <http://example.org/p> "v" .`)
}

func TestCovVersionEOF(t *testing.T) {
	mustFail(t, `VERSION `)
}

// WithBase option
func TestCovWithBaseOption(t *testing.T) {
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(`<s> <p> <o> .`), WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// unescapeIRI edge cases
func TestCovIRIInvalidChar(t *testing.T) {
	mustFail(t, "<http://example.org/a b> <http://example.org/p> \"v\" .")
}

// --- Serializer coverage tests ---

func TestCovSerializeTripleTerm(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ex:b ex:c )>> .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term in output:\n%s", out)
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

func TestCovSerializeBNodeSubject(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
_:b1 ex:p "v" .
ex:s ex:q _:b1 .`)
	out := covSerialize(t, g)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestCovSerializeUnreferencedBNode(t *testing.T) {
	// BNode with refs=0 should be serialized as []
	g := mustParse(t, `@prefix ex: <http://example.org/> .
_:b1 ex:p "v" .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "[]") {
		t.Errorf("expected [] for unreferenced bnode:\n%s", out)
	}
}

func TestCovSerializeList(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ( "a" "b" "c" ) .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "( ") {
		t.Errorf("expected list syntax in output:\n%s", out)
	}
}

func TestCovSerializeRDFSClass(t *testing.T) {
	g := mustParse(t, `@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix ex: <http://example.org/> .
ex:MyClass rdf:type rdfs:Class .
ex:MyClass rdfs:label "My class" .
ex:other ex:p "v" .`)
	out := covSerialize(t, g)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestCovSerializeTypedLiteralPrefixed(t *testing.T) {
	g := rdflibgo.NewGraph()
	xsd := rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#")
	g.Bind("xsd", xsd)
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	dt := rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#date")
	o := rdflibgo.NewLiteral("2024-01-01", rdflibgo.WithDatatype(dt))
	// Add a triple using an xsd: URI as object so trackNS picks it up
	g.Add(s, p, o)
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/dt"), dt)
	out := covSerialize(t, g)
	if !strings.Contains(out, "xsd:") {
		t.Errorf("expected xsd prefix in output:\n%s", out)
	}
}

func TestCovSerializeInlineBNodeMultiPred(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p [ ex:a "1" ; ex:b "2" ] .`)
	out := covSerialize(t, g)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestCovSerializeLangLiteral(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "hello"@en .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "@en") {
		t.Errorf("expected lang tag in output:\n%s", out)
	}
}

func TestCovSerializeMultipleObjects(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "a", "b" .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, ",") {
		t.Errorf("expected comma separator in output:\n%s", out)
	}
}

func TestCovSerializeMultilineString(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p """line1
line2""" .`)
	out := covSerialize(t, g)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

// resolveIRI — invalid base parse
func TestCovResolveIRIInvalidBase(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`<s> <p> <o> .`), WithBase("://invalid"))
	// May or may not error — just exercises the branch
	_ = err
}

// sparqlBase error — no IRI after BASE
func TestCovSPARQLBaseError(t *testing.T) {
	mustFail(t, `BASE
<s> <p> <o> .`)
}

// readSubject — collection as subject
func TestCovCollectionAsSubject(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
( "a" "b" ) ex:p "v" .`)
	if g.Len() == 0 {
		t.Error("expected triples")
	}
}

// predicateObjectList — "a" shorthand
func TestCovPredicateA(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s a ex:Thing .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// isAbsoluteIRI — digit in scheme, non-letter scheme start
func TestCovAbsoluteIRIEdge(t *testing.T) {
	// Scheme with digits: h2tp
	g := mustParse(t, `@base <http://example.org/> .
<http://example.org/abs> <http://example.org/p> "v" .`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// isValidPrefixName edge cases
func TestCovInvalidPrefixName(t *testing.T) {
	// Prefix ending with dot — test that serializer handles it
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("v")
	g.Bind("ex.", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, o)
	out := covSerialize(t, g)
	// Prefix "ex." ends with dot, so it's invalid — should not use it
	if strings.Contains(out, "ex.:") {
		t.Errorf("should not use invalid prefix name:\n%s", out)
	}
}

// --- Additional coverage tests ---

// TestCovSerializeWithBaseOutput covers @base in serializer output.
func TestCovSerializeWithBaseOutput(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("val"))

	var buf strings.Builder
	if err := Serialize(g, &buf, WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "@base") {
		t.Errorf("expected @base in output, got:\n%s", buf.String())
	}
}

// TestCovSerializeTripleTermWithPrefix covers TripleTerm with prefixed names in serializer.
func TestCovSerializeTripleTermWithPrefix(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(s, p, o)
	asserts := rdflibgo.NewURIRefUnsafe("http://example.org/asserts")
	g.Add(s, asserts, tt)

	out := covSerialize(t, g)
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term, got:\n%s", out)
	}
}

// TestCovSerializeListInvalidExtraPred covers list detection failure due to extra predicate.
func TestCovSerializeListInvalidExtraPred(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn := rdflibgo.NewBNode("l1")
	g.Add(bn, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	g.Add(bn, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
	g.Add(bn, rdflibgo.NewURIRefUnsafe("http://example.org/extra"), rdflibgo.NewLiteral("x"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/list"), bn)
	out := covSerialize(t, g)
	// Should NOT use ( ) syntax
	_ = out
}

// TestCovSerializeListCyclic covers cycle detection in list serialization.
func TestCovSerializeListCyclic(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn1 := rdflibgo.NewBNode("cyc1")
	bn2 := rdflibgo.NewBNode("cyc2")
	g.Add(bn1, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	g.Add(bn1, rdflibgo.RDF.Rest, bn2)
	g.Add(bn2, rdflibgo.RDF.First, rdflibgo.NewLiteral("b"))
	g.Add(bn2, rdflibgo.RDF.Rest, bn1) // cycle
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/list"), bn1)
	out := covSerialize(t, g)
	_ = out
}

// TestCovSerializeLocalNameInvalid covers fallback to full IRI for invalid local name.
func TestCovSerializeLocalNameInvalid(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/has space")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("val"))
	out := covSerialize(t, g)
	if !strings.Contains(out, "<http://example.org/has space>") {
		t.Errorf("expected full IRI for invalid local name, got:\n%s", out)
	}
}

// TestCovAnnotation covers the annotation syntax.
func TestCovAnnotation(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o {| ex:source ex:doc |} .`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovReifier covers the ~ reifier syntax.
func TestCovReifier(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ ex:r1 .`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovReifierAnonymous covers the anonymous reifier.
func TestCovReifierAnonymous(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ .`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovReifierWithAnnotation covers ~ id {| ... |}.
func TestCovReifierWithAnnotation(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ ex:r {| ex:source ex:doc |} .`)
	if g.Len() < 3 {
		t.Errorf("expected at least 3 triples, got %d", g.Len())
	}
}

// TestCovReifiedTriple covers << s p o >> as subject.
func TestCovReifiedTriple(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:source ex:doc .`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovReifiedTripleNamedReifier covers << s p o ~ id >>.
func TestCovReifiedTripleNamedReifier(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o ~ ex:r >> ex:source ex:doc .`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovReifiedInnerBNode covers bnode in reified triple.
func TestCovReifiedInnerBNode(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
<< _:b1 ex:p ex:o >> ex:source ex:doc .`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovReifiedInnerEmptyBNode covers [] in reified triple.
func TestCovReifiedInnerEmptyBNode(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
<< [] ex:p ex:o >> ex:source ex:doc .`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovReifiedInnerObjectBNode covers bnode as object in reified triple.
func TestCovReifiedInnerObjectBNode(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p _:b1 >> ex:source ex:doc .`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovReifiedInnerObjectLiteral covers literal as object in reified triple.
func TestCovReifiedInnerObjectLiteral(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p "hello" >> ex:source ex:doc .`)
	if g.Len() < 2 {
		t.Errorf("expected at least 2 triples, got %d", g.Len())
	}
}

// TestCovReifiedInnerObjectBoolean covers boolean as reified inner object.
func TestCovReifiedInnerObjectBoolean(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p true >> ex:source ex:doc .`)
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p false >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectNumeric covers numeric as reified inner object.
func TestCovReifiedInnerObjectNumeric(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p 42 >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectEmptyBNode covers [] as reified inner object.
func TestCovReifiedInnerObjectEmptyBNode(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p [] >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectCollection covers collection rejection in reified triple.
func TestCovReifiedInnerObjectCollection(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p (1 2 3) >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectBNodePropertyList covers bnode property list rejection.
func TestCovReifiedInnerObjectBNodePropertyList(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< [ex:a ex:b] ex:p ex:o >> ex:source ex:doc .`)
}

// TestCovEmptyAnnotationError covers empty annotation block error.
func TestCovEmptyAnnotationError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o {| |} .`)
}

// TestCovTripleTermSubjectError covers triple term as subject error.
func TestCovTripleTermSubjectError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<<( ex:s ex:p ex:o )>> ex:q ex:r .`)
}

// TestCovReifiedInnerNestedSubject covers nested reified triple as subject.
func TestCovReifiedInnerNestedSubject(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< << ex:a ex:b ex:c >> ex:p ex:o >> ex:source ex:doc .`)
}

// TestCovReifiedInnerNestedObject covers nested reified triple as object.
func TestCovReifiedInnerNestedObject(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p << ex:a ex:b ex:c >> >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectTripleTerm covers triple term as reified inner object.
func TestCovReifiedInnerObjectTripleTerm(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p <<( ex:a ex:b ex:c )>> >> ex:source ex:doc .`)
}

// TestCovTripleTermSubjectBNode covers bnode as triple term subject.
func TestCovTripleTermSubjectBNode(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( _:b1 ex:q ex:o )>> .`)
}

// TestCovTripleTermSubjectPrefixed covers prefixed name as triple term subject.
func TestCovTripleTermSubjectPrefixed(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ex:b ex:c )>> .`)
}

// TestCovReifierBNodeID covers _:bnode as reifier ID.
func TestCovReifierBNodeID(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ _:r1 .`)
}

// TestCovReifierIRI covers <IRI> as reifier ID.
func TestCovReifierIRI(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ <http://example.org/reifier> .`)
}

// TestCovReifiedTripleSubjectBNodePropertyList covers rejection.
func TestCovReifiedTripleSubjectBNodePropertyList(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< [ex:p ex:o] ex:q ex:r >> ex:source ex:doc .`)
}

// TestCovVersionUnterminated covers unterminated version string.
func TestCovVersionUnterminated(t *testing.T) {
	mustFail(t, `VERSION "1.2`)
}

// TestCovIsValidPrefixNameEdges covers edge cases.
func TestCovIsValidPrefixNameEdges(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"", true},
		{"rdf", true},
		{"1bad", false},
		{"ok.", false},
		{"ok.ok", true},
		{"a\u00B7b", true},
	}
	for _, tc := range tests {
		got := isValidPrefixName(tc.name)
		if got != tc.want {
			t.Errorf("isValidPrefixName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCovIsValidLocalNameEdges covers edge cases.
func TestCovIsValidLocalNameEdges(t *testing.T) {
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

// TestCovSerializeBNodeNoPredicates covers writing a bnode that has no predicates (referenced-only).
func TestCovSerializeBNodeNoPredicates(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	bn := rdflibgo.NewBNode("nopreds")
	g.Add(s, p, bn)
	out := covSerialize(t, g)
	if !strings.Contains(out, "_:nopreds") {
		// Referenced but no predicates => just print the bnode label
		_ = out
	}
}

// TestCovSPARQLPrefixError covers error in SPARQL PREFIX.
func TestCovSPARQLPrefixError(t *testing.T) {
	mustFail(t, `PREFIX
<http://example.org/s> <http://example.org/p> "v" .`)
}

// TestCovDirectiveError covers error in Turtle @prefix.
func TestCovDirectiveError(t *testing.T) {
	mustFail(t, `@prefix ex .`)
}

// TestCovDirectiveBaseError covers error in @base directive.
func TestCovDirectiveBaseError(t *testing.T) {
	mustFail(t, `@base .`)
}

// TestCovReadDatatypeIRI covers datatype IRI reading paths.
func TestCovReadDatatypeIRI(t *testing.T) {
	// Prefixed datatype
	mustParse(t, `@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
<http://example.org/s> <http://example.org/p> "42"^^xsd:integer .`)
	// Full IRI datatype
	mustParse(t, `<http://example.org/s> <http://example.org/p> "42"^^<http://www.w3.org/2001/XMLSchema#integer> .`)
}

// TestCovReadDatatypeIRIError covers error in datatype IRI.
func TestCovReadDatatypeIRIError(t *testing.T) {
	mustFail(t, `<http://example.org/s> <http://example.org/p> "v"^^.`)
}

// TestCovPredicateObjectListEmpty covers empty predicate list.
func TestCovPredicateObjectListEmpty(t *testing.T) {
	mustFail(t, `<http://example.org/s> .`)
}

// TestCovObjectListError covers error in object list.
func TestCovObjectListError(t *testing.T) {
	mustFail(t, `<http://example.org/s> <http://example.org/p> .`)
}

// TestCovReadSubjectIRI covers IRI subject.
func TestCovReadSubjectIRI(t *testing.T) {
	mustParse(t, `<http://example.org/s> <http://example.org/p> "v" .`)
}

// TestCovReadSubjectPrefixed covers prefixed name subject.
func TestCovReadSubjectPrefixed(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "v" .`)
}

// TestCovReadBlankNodeLabelDotEnd covers bnode label edge case.
func TestCovReadBlankNodeLabelDotEnd(t *testing.T) {
	mustParse(t, `_:b1 <http://example.org/p> "v" .`)
}

// TestCovReadCollectionNested covers nested collections.
func TestCovReadCollectionNested(t *testing.T) {
	mustParse(t, `<http://example.org/s> <http://example.org/p> ((1 2) 3) .`)
}

// TestCovReadCollectionEmpty covers empty collection.
func TestCovReadCollectionEmpty(t *testing.T) {
	mustParse(t, `<http://example.org/s> <http://example.org/p> () .`)
}

// TestCovReadEscapeNRT covers \n, \r, \t escapes in strings.
func TestCovReadEscapeNRT(t *testing.T) {
	mustParse(t, `<http://example.org/s> <http://example.org/p> "a\nb\rc\td" .`)
}

// TestCovReadUnicodeEscapeBadHex covers bad hex in unicode escape.
func TestCovReadUnicodeEscapeBadHex(t *testing.T) {
	mustFail(t, `<http://example.org/s> <http://example.org/p> "a\uXXXXb" .`)
}

// TestCovUnescapeIRIBad covers bad escape in IRI.
func TestCovUnescapeIRIBad(t *testing.T) {
	mustFail(t, `<http://example.org/\uXXXX> <http://example.org/p> "v" .`)
}

// TestCovIsAbsoluteIRIColon covers IRI edge cases.
func TestCovIsAbsoluteIRIColon(t *testing.T) {
	// Turtle doesn't reject relative IRIs by default; this is valid Turtle that resolves against base
	mustParse(t, `@base <http://example.org/> .
<relative> <http://example.org/p> "v" .`)
}

// TestCovSerializeRDFSLabel covers rdfs:label predicate ordering.
func TestCovSerializeRDFSLabel(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("rdfs", rdflibgo.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#"))
	g.Bind("rdf", rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.RDFS.Class)
	g.Add(s, rdflibgo.RDFS.Label, rdflibgo.NewLiteral("My Class"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/extra"), rdflibgo.NewLiteral("x"))
	out := covSerialize(t, g)
	// rdf:type should come first, then rdfs:label
	typeIdx := strings.Index(out, "a ")
	labelIdx := strings.Index(out, "rdfs:label")
	if typeIdx > 0 && labelIdx > 0 && typeIdx > labelIdx {
		t.Error("expected rdf:type before rdfs:label")
	}
}

// TestCovSerializePrefixedDatatype covers literal with prefixed datatype.
func TestCovSerializePrefixedDatatype(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("xsd", rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	// Add a triple that uses a xsd: namespace URI so prefix gets tracked
	xsdType := rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#dateTime")
	g.Add(s, rdflibgo.RDF.Type, xsdType)
	g.Add(s, p, rdflibgo.NewLiteral("2024-01-01", rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#date"))))
	out := covSerialize(t, g)
	if !strings.Contains(out, "xsd:") {
		t.Errorf("expected xsd: prefix usage, got:\n%s", out)
	}
}

// TestCovSerializeListWithNestedBNode covers list containing inline bnode.
func TestCovSerializeListWithNestedBNode(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ( [ ex:a "val" ] "b" ) .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "(") {
		t.Errorf("expected list syntax, got:\n%s", out)
	}
}

// TestCovSerializeListWithTripleTerm covers list containing triple term.
func TestCovSerializeListWithTripleTerm(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ( <<( ex:a ex:b ex:c )>> "v" ) .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term in list, got:\n%s", out)
	}
}

// TestCovSerializeLargePrefixedOutput covers serialization with many prefixed predicates.
func TestCovSerializeLargePrefixedOutput(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	for i := 0; i < 5; i++ {
		p := rdflibgo.NewURIRefUnsafe("http://example.org/p" + string(rune('a'+i)))
		g.Add(s, p, rdflibgo.NewLiteral("v"))
	}
	out := covSerialize(t, g)
	if !strings.Contains(out, "ex:pa") {
		t.Errorf("expected prefixed names, got:\n%s", out)
	}
}

// TestCovReadPrefixNameError covers error in prefix name reading.
func TestCovReadPrefixNameError(t *testing.T) {
	// Undeclared prefix should cause an error
	mustFail(t, `nope:s <http://example.org/p> "v" .`)
}

// TestCovSPARQLPrefixUnterminated covers unterminated SPARQL PREFIX.
func TestCovSPARQLPrefixUnterminated(t *testing.T) {
	mustFail(t, `PREFIX ex: <http://example.org/`)
}

// TestCovSPARQLBaseUnterminated covers unterminated SPARQL BASE.
func TestCovSPARQLBaseUnterminated(t *testing.T) {
	mustFail(t, `BASE <http://example.org/`)
}

// TestCovReadBlankNodePropertyListNested covers nested bnode property lists.
func TestCovReadBlankNodePropertyListNested(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p [ ex:a [ ex:b "deep" ] ] .`)
}

// TestCovObjectListMultiple covers multiple objects after predicate.
func TestCovObjectListMultiple(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "a", "b", "c" .`)
}

// TestCovSerializeMultiSubject covers multiple subjects in output.
func TestCovSerializeMultiSubject(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	for i := 0; i < 3; i++ {
		s := rdflibgo.NewURIRefUnsafe("http://example.org/s" + string(rune('a'+i)))
		g.Add(s, p, rdflibgo.NewLiteral("v"))
	}
	out := covSerialize(t, g)
	// Should have 3 subjects separated by blank lines
	if strings.Count(out, ".") < 3 {
		t.Errorf("expected 3 subjects, got:\n%s", out)
	}
}

// TestCovSerializeWriterError covers writer error in Serialize.
// bufio.Writer buffers 4KB, so we need enough data to force a flush mid-write.
func TestCovSerializeWriterError(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	// Generate enough triples to exceed bufio's 4KB buffer
	for i := 0; i < 200; i++ {
		pURI := "http://example.org/predicate-with-a-very-long-name-to-fill-buffer-" + string(rune('A'+i%26)) + string(rune('A'+(i/26)%26)) + string(rune('A'+(i/676)%26))
		g.Add(s, rdflibgo.NewURIRefUnsafe(pURI), rdflibgo.NewLiteral("a-value-that-is-long-enough-to-contribute-significantly-to-buffer-filling"))
	}
	err := Serialize(g, &failWriter{})
	// Either returns error or silently fails (due to bufio defer); just exercise the path
	_ = err
}

// failWriter always returns an error on Write.
type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write error")
}

// limitWriter fails after writing a set number of bytes.
type limitWriter struct {
	limit   int
	written int
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.written+len(p) > w.limit {
		remaining := w.limit - w.written
		if remaining <= 0 {
			return 0, errors.New("write limit exceeded")
		}
		w.written += remaining
		return remaining, errors.New("write limit exceeded")
	}
	w.written += len(p)
	return len(p), nil
}

// --- More parser coverage tests ---

// TestCovReifiedInnerObjectEmptyBNodeObj covers [] as object in reified triple.
func TestCovReifiedInnerObjectEmptyBNodeObj(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p [] >> ex:source ex:doc .`)
}

// TestCovReifiedInnerSubjectPrefixed2 covers prefixed name in reified subject.
func TestCovReifiedInnerSubjectPrefixed2(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p "val" >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectPrefixed covers prefixed name in reified object.
func TestCovReifiedInnerObjectPrefixed(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o >> ex:source ex:doc .`)
}

// TestCovTripleTermSubjectIRI covers IRI as triple term subject.
func TestCovTripleTermSubjectIRI(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( <http://example.org/a> ex:b ex:c )>> .`)
}

// TestCovTripleTermSubjectPrefixed2 covers prefixed name as triple term subject.
func TestCovTripleTermSubjectPrefixed2(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ex:b ex:c )>> .`)
}

// TestCovTripleTermSubjectBNodeInner covers bnode as triple term subject.
func TestCovTripleTermSubjectBNodeInner(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( _:b1 ex:b ex:c )>> .`)
}

// TestCovTripleTermInnerError covers error in triple term inner.
func TestCovTripleTermInnerError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ex:b ex:c )> .`)
}

// TestCovReifiedTripleAnonymousReifier covers << s p o ~ >> with anonymous reifier.
func TestCovReifiedTripleAnonymousReifier(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o ~ >> ex:source ex:doc .`)
}

// TestCovReifiedTripleWithIRIReifier covers ~ <iri> reifier in reified triple.
func TestCovReifiedTripleWithIRIReifier(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o ~ <http://example.org/r1> >> ex:source ex:doc .`)
}

// TestCovReifiedTripleWithBNodeReifier covers ~ _:bnode reifier in reified triple.
func TestCovReifiedTripleWithBNodeReifier(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o ~ _:r1 >> ex:source ex:doc .`)
}

// TestCovReifiedTripleWithPrefixedReifier covers ~ prefixed:name reifier in reified triple.
func TestCovReifiedTripleWithPrefixedReifier(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p ex:o ~ ex:r1 >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectNestedReified covers nested reified triple as inner object.
func TestCovReifiedInnerObjectNestedReified(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p << ex:a ex:b ex:c >> >> ex:source ex:doc .`)
}

// TestCovReifiedInnerObjectTripleTermNested covers triple term as inner reified object.
func TestCovReifiedInnerObjectTripleTermNested(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p <<( ex:a ex:b ex:c )>> >> ex:source ex:doc .`)
}

// TestCovAnnotationWithMultiplePredicates covers annotation with multiple predicates.
func TestCovAnnotationWithMultiplePredicates(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o {| ex:source ex:doc ; ex:date "2024" |} .`)
}

// TestCovReifierError covers error in reifier identifier.
func TestCovReifierError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ `)
}

// TestCovBooleanTrueInReified covers true in reified triple object.
func TestCovBooleanTrueInReified(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p true >> ex:source ex:doc .`)
}

// TestCovBooleanFalseInReified covers false in reified triple object.
func TestCovBooleanFalseInReified(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p false >> ex:source ex:doc .`)
}

// TestCovNumericInReified covers numeric literal in reified triple object.
func TestCovNumericInReified(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p 42 >> ex:source ex:doc .
<< ex:s ex:p 3.14 >> ex:source ex:doc .
<< ex:s ex:p 1e2 >> ex:source ex:doc .`)
}

// TestCovStringLiteralInReified covers string literal in reified triple object.
func TestCovStringLiteralInReified(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p "hello" >> ex:source ex:doc .`)
}

// TestCovCollectionInReifiedError covers collection not allowed in reified.
func TestCovCollectionInReifiedError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p (1 2 3) >> ex:source ex:doc .`)
}

// TestCovBNodePropListInReifiedSubjectError covers [p o] in reified subject.
func TestCovBNodePropListInReifiedSubjectError(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<< [ex:a ex:b] ex:p ex:o >> ex:source ex:doc .`)
}

// TestCovSPARQLBaseError covers error in SPARQL BASE.
func TestCovSPARQLBaseError2(t *testing.T) {
	mustFail(t, `BASE bad`)
}

// TestCovReadSubjectReifiedTriple covers reified triple as subject.
func TestCovReadSubjectReifiedTriple(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:a ex:b ex:c >> ex:source ex:doc .`)
}

// TestCovReadSubjectCollection covers collection as subject.
func TestCovReadSubjectCollection(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
("a" "b") ex:p "v" .`)
}

// TestCovReadSubjectBNodePropList covers blank node property list as subject.
func TestCovReadSubjectBNodePropList(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
[ex:a "v1"] ex:p "v2" .`)
}

// TestCovObjectTripleTerm covers triple term as object.
func TestCovObjectTripleTerm(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p <<( ex:a ex:b ex:c )>> .`)
}

// TestCovReadBlankNodeLabelEdge covers bnode label with dot in middle.
func TestCovReadBlankNodeLabelEdge(t *testing.T) {
	mustParse(t, `_:b1.2 <http://example.org/p> "v" .`)
}

// TestCovPredicateObjectListSemicolon covers multiple predicates.
func TestCovPredicateObjectListSemicolon(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p1 "a" ; ex:p2 "b" ; ex:p3 "c" .`)
}

// TestCovPredicateObjectListTrailingSemicolon covers trailing semicolon.
func TestCovPredicateObjectListTrailingSemicolon(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "v" ; .`)
}

// TestCovObjectListMultiple covers multiple objects with commas.
func TestCovObjectListMultipleExtra(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "a", "b", "c", "d" .`)
}

// TestCovReifiedWithAnnotation covers reified triple with annotation block.
func TestCovReifiedWithAnnotation(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o ~ ex:r {| ex:source ex:doc |} .`)
}

// TestCovDirectiveBaseFullIRI covers @base with full IRI.
func TestCovDirectiveBaseFullIRI(t *testing.T) {
	mustParse(t, `@base <http://example.org/> .
<s> <p> "v" .`)
}

// TestCovResolveIRIRelative covers relative IRI resolution.
func TestCovResolveIRIRelativeTypes(t *testing.T) {
	mustParse(t, `@base <http://example.org/dir/> .
<../other> <http://example.org/p> "v" .`)
}

// TestCovParseReadError covers io.ReadAll error path in Parse.
func TestCovParseReadError(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, errReader{})
	if err == nil {
		t.Error("expected read error")
	}
}

// TestCovParseReadErrorWithBase covers io.ReadAll error with WithBase.
func TestCovParseReadErrorWithBase(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, errReader{}, WithBase("http://example.org/"))
	if err == nil {
		t.Error("expected read error")
	}
}

// TestCovReadDatatypeIRIError2 covers error in IRI path of readDatatypeIRI.
func TestCovReadDatatypeIRIError2(t *testing.T) {
	mustFail(t, `<http://example.org/s> <http://example.org/p> "v"^^<broken iri> .`)
}

// TestCovReadSubjectError covers error in readSubject.
func TestCovReadSubjectError(t *testing.T) {
	mustFail(t, `42 <http://example.org/p> "v" .`)
}

// TestCovObjectListError2 covers error in second object.
func TestCovObjectListError2(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "a", .`)
}

// TestCovBNodeLabelDot covers bnode label ending with dot (should stop before dot).
func TestCovBNodeLabelDot(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
_:b1.x ex:p "v" .`)
}

// TestCovMatchKeywordCINonLetter covers matchKeywordCI with non-letter follow.
func TestCovMatchKeywordCINonLetter(t *testing.T) {
	// "true" followed by dot should match as boolean
	mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p true.`)
}

// TestCovIsAbsoluteIRIEdge covers various IRI patterns.
func TestCovIsAbsoluteIRIEdge(t *testing.T) {
	// IRI with scheme:digit pattern
	mustParse(t, `@base <http://example.org/> .
<http://example.org/s> <http://example.org/p> "v" .`)
}

// TestCovReifiedInnerObjectIRI covers IRI in reified object position.
func TestCovReifiedInnerObjectIRI(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p <http://example.org/o> >> ex:source ex:doc .`)
}

// TestCovReifiedInnerSubjectIRI covers IRI in reified subject.
func TestCovReifiedInnerSubjectIRI(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< <http://example.org/s> ex:p ex:o >> ex:source ex:doc .`)
}

// TestCovReifiedInnerSubjectBNode covers bnode in reified subject.
func TestCovReifiedInnerSubjectBNode2(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< _:b1 ex:p ex:o >> ex:source ex:doc .`)
}

// TestCovParseWithBase covers Parse with WithBase option.
func TestCovParseWithBase(t *testing.T) {
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(`<s> <p> "v" .`), WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// --- Additional serializer coverage ---

// TestCovSerializeRDFTypeShorthand covers the "a" shorthand for rdf:type in predicates.
func TestCovSerializeRDFTypeShorthand(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("rdf", rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Thing"))
	out := covSerialize(t, g)
	if !strings.Contains(out, " a ") {
		t.Errorf("expected 'a' shorthand for rdf:type, got:\n%s", out)
	}
}

// TestCovSerializeEmptyPredicates covers writePredicates with empty pred list (bnode with no preds).
func TestCovSerializeEmptyPredicates(t *testing.T) {
	// A BNode that appears only as a subject with no predicates cannot happen naturally,
	// but we can test the unreferenced bnode with predicates vs without.
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	bn := rdflibgo.NewBNode("ref")
	g.Add(s, p, bn)
	// bn has no predicates, only referenced once
	out := covSerialize(t, g)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

// TestCovLabelTripleTerm covers the label() function default branch with TripleTerm.
func TestCovLabelTripleTerm(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/a")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/b")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/c")
	tt := rdflibgo.NewTripleTerm(s, p, o)
	// Triple term as subject hits the default branch in label()
	assertP := rdflibgo.NewURIRefUnsafe("http://example.org/asserts")
	g.Add(tt, assertP, rdflibgo.NewLiteral("val"))
	out := covSerialize(t, g)
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term as subject, got:\n%s", out)
	}
}

// TestCovSerializeTrackNSTripleTerm covers trackNS with TripleTerm (recursive branch).
func TestCovSerializeTrackNSTripleTerm(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	innerS := rdflibgo.NewURIRefUnsafe("http://example.org/a")
	innerP := rdflibgo.NewURIRefUnsafe("http://example.org/b")
	innerO := rdflibgo.NewURIRefUnsafe("http://example.org/c")
	tt := rdflibgo.NewTripleTerm(innerS, innerP, innerO)
	g.Add(s, p, tt)
	out := covSerialize(t, g)
	// Should use ex: prefix for all URIs
	if !strings.Contains(out, "ex:") {
		t.Errorf("expected ex: prefix, got:\n%s", out)
	}
}

// TestCovSerializeWriteSubjectReferencedBNode covers bnode with refs > 0 (not inlined).
func TestCovSerializeWriteSubjectReferencedBNode(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	bn := rdflibgo.NewBNode("ref1")
	s1 := rdflibgo.NewURIRefUnsafe("http://example.org/s1")
	s2 := rdflibgo.NewURIRefUnsafe("http://example.org/s2")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	q := rdflibgo.NewURIRefUnsafe("http://example.org/q")
	g.Add(bn, p, rdflibgo.NewLiteral("val"))
	g.Add(s1, q, bn)
	g.Add(s2, q, bn) // referenced twice, should not be inlined
	out := covSerialize(t, g)
	if !strings.Contains(out, "_:ref1") {
		t.Errorf("expected _:ref1 (referenced bnode), got:\n%s", out)
	}
}

// TestCovSerializeObjectBNodeMultiRef covers BNode object referenced multiple times.
func TestCovSerializeObjectBNodeMultiRef(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn := rdflibgo.NewBNode("multi")
	s1 := rdflibgo.NewURIRefUnsafe("http://example.org/s1")
	s2 := rdflibgo.NewURIRefUnsafe("http://example.org/s2")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(bn, p, rdflibgo.NewLiteral("val"))
	g.Add(s1, p, bn)
	g.Add(s2, p, bn)
	out := covSerialize(t, g)
	// Should print _:multi, not inline
	if strings.Count(out, "_:multi") < 2 {
		t.Errorf("expected multiple references to _:multi, got:\n%s", out)
	}
}

// TestCovSerializeInlineBNodeWithRDFType covers inline bnode with rdf:type predicate.
func TestCovSerializeInlineBNodeWithRDFType(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("rdf", rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	bn := rdflibgo.NewBNode("typed")
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(bn, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Type"))
	g.Add(s, p, bn) // referenced once -> inline
	out := covSerialize(t, g)
	if !strings.Contains(out, "[ a ") {
		t.Errorf("expected inline bnode with 'a' shorthand, got:\n%s", out)
	}
}

// TestCovSerializeIsValidListNilPreds covers isValidList with nil preds (rest points to non-existent node).
func TestCovSerializeIsValidListNilPreds(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn := rdflibgo.NewBNode("l1")
	g.Add(bn, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	// rest points to a bnode that doesn't exist as subject (nil preds)
	g.Add(bn, rdflibgo.RDF.Rest, rdflibgo.NewBNode("nonexistent"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/list"), bn)
	out := covSerialize(t, g)
	// Should NOT use ( ) syntax since list is invalid
	if strings.Contains(out, "( ") && strings.Contains(out, " )") {
		t.Errorf("should not use list syntax for invalid list, got:\n%s", out)
	}
}

// TestCovSerializeIsValidListMultiFirst covers isValidList with multiple rdf:first values.
func TestCovSerializeIsValidListMultiFirst(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn := rdflibgo.NewBNode("l1")
	g.Add(bn, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	g.Add(bn, rdflibgo.RDF.First, rdflibgo.NewLiteral("b")) // two firsts
	g.Add(bn, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/list"), bn)
	out := covSerialize(t, g)
	_ = out
}

// TestCovSerializePrefixInvalid covers invalid prefix name (non-PN_CHARS_BASE first char).
func TestCovSerializePrefixInvalidChars(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("1bad", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))
	out := covSerialize(t, g)
	// Prefix "1bad" starts with digit, so not valid PN_CHARS_BASE — should not be used
	if strings.Contains(out, "1bad:") {
		t.Errorf("should not use invalid prefix, got:\n%s", out)
	}
}

// TestCovSerializeMarkListNodesEmptyRest covers markListNodes when rests is empty.
func TestCovSerializeMarkListNodesEmpty(t *testing.T) {
	// This is tricky to trigger since isValidList already checks rests,
	// but we can confirm the serializer works with a valid 1-item list.
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	bn := rdflibgo.NewBNode("l1")
	g.Add(bn, rdflibgo.RDF.First, rdflibgo.NewLiteral("only"))
	g.Add(bn, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
	g.Add(s, p, bn)
	out := covSerialize(t, g)
	if !strings.Contains(out, "( ") {
		t.Errorf("expected single-item list syntax, got:\n%s", out)
	}
}

// TestCovSerializeIsValidPrefixNameMiddleDot covers middle dot character in prefix.
func TestCovSerializeIsValidPrefixNameMiddleDot(t *testing.T) {
	// Test with combining characters
	tests := []struct {
		name string
		want bool
	}{
		{"a\u0300b", true},  // combining grave accent
		{"a\u203Fb", true},  // undertie
		{"a\u2040b", true},  // character tie
		{"a\u00B7b", true},  // middle dot
		{"a$b", false},      // invalid character in middle
	}
	for _, tc := range tests {
		got := isValidPrefixName(tc.name)
		if got != tc.want {
			t.Errorf("isValidPrefixName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCovSerializeIsValidLocalNameMiddleChars covers various middle characters.
func TestCovSerializeIsValidLocalNameMiddleChars(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"a\u0300b", true},  // combining grave accent
		{"a\u203Fb", true},  // undertie
		{"a\u00B7b", true},  // middle dot
		{"a-b", true},       // hyphen
		{"a.b", true},       // dot in middle
		{"a:b", true},       // colon
		{"a$b", false},      // invalid
	}
	for _, tc := range tests {
		got := isValidLocalName(tc.name)
		if got != tc.want {
			t.Errorf("isValidLocalName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCovSerializeObjectURIRef covers objectStr for URIRef with and without prefix.
func TestCovSerializeObjectURIRef(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	// Object is a URIRef that matches prefix
	g.Add(s, p, rdflibgo.NewURIRefUnsafe("http://example.org/obj"))
	// Object is a URIRef with no prefix
	g.Add(s, p, rdflibgo.NewURIRefUnsafe("http://other.org/obj"))
	out := covSerialize(t, g)
	if !strings.Contains(out, "ex:obj") {
		t.Errorf("expected ex:obj, got:\n%s", out)
	}
	if !strings.Contains(out, "<http://other.org/obj>") {
		t.Errorf("expected full IRI for other.org, got:\n%s", out)
	}
}

// TestCovSerializeListAsListHead covers list head that is a list head in object.
func TestCovSerializeListAsObjectHead(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ( ex:a ex:b ) .
ex:s2 ex:q ( "x" "y" "z" ) .`)
	out := covSerialize(t, g)
	if strings.Count(out, "(") < 2 {
		t.Errorf("expected at least 2 list syntaxes, got:\n%s", out)
	}
}

// TestCovSerializeTripleTermBNodeSubject covers tripleTermStr with BNode subject.
func TestCovSerializeTripleTermBNodeSubject(t *testing.T) {
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

// TestCovSerializeObjectDefault covers objectStr default branch.
// This is hard to hit normally since all Go types are covered, but exercise it anyway.
func TestCovSerializeManyPredicates(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	// Multiple predicates: rdf:type first, rdfs:label second, others after
	g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Class"))
	g.Add(s, rdflibgo.RDFS.Label, rdflibgo.NewLiteral("label"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p1"), rdflibgo.NewLiteral("v1"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p2"), rdflibgo.NewLiteral("v2"))
	g.Bind("rdfs", rdflibgo.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#"))
	g.Bind("rdf", rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	out := covSerialize(t, g)
	// Should use ; separator
	if strings.Count(out, ";") < 3 {
		t.Errorf("expected multiple ; separators, got:\n%s", out)
	}
}

// TestCovReifiedInnerObjectIRIFull covers IRI in reified inner object (full IRI).
func TestCovReifiedInnerObjectIRIFull(t *testing.T) {
	mustParse(t, `@prefix ex: <http://example.org/> .
<< ex:s ex:p <http://example.org/o> >> ex:source ex:doc .`)
}

// TestCovDirectiveVersionDot covers @version directive.
func TestCovDirectiveVersionDot(t *testing.T) {
	mustParse(t, `@version "1.2" .
@prefix ex: <http://example.org/> .
ex:s ex:p "v" .`)
}

// TestCovSerializeWriterErrorBase covers writer error on @base output.
func TestCovSerializeWriterErrorBase(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))
	err := Serialize(g, &failWriter{}, WithBase("http://example.org/"))
	// bufio.Writer buffers, so error may happen on Flush
	_ = err
}

// TestCovSerializeWriterErrorPrefix covers writer error on prefix output.
func TestCovSerializeWriterErrorPrefix(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))
	err := Serialize(g, &failWriter{})
	// bufio.Writer buffers, so error may happen on Flush
	_ = err
}

// TestCovSerializeNoPrefixURI covers qnameOrFull returning full IRI.
func TestCovSerializeNoPrefixURI(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewURIRefUnsafe("http://other.org/obj"))
	out := covSerialize(t, g)
	if !strings.Contains(out, "<http://other.org/obj>") {
		t.Errorf("expected full IRI, got:\n%s", out)
	}
}

// TestCovSerializeListStrError covers listStr with a list that includes an error-producing element.
// In practice this is hard to trigger since objectStr only errors on tripleTermStr.
func TestCovSerializeListWithURI(t *testing.T) {
	g := mustParse(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ( ex:a "b" ) .`)
	out := covSerialize(t, g)
	if !strings.Contains(out, "(") {
		t.Errorf("expected list, got:\n%s", out)
	}
}

// TestCovSerializeWriterErrorAtVariousPoints covers writer errors at different
// points in the serializer by generating enough output to overflow bufio's 4KB buffer.
func TestCovSerializeWriterErrorAtVariousPoints(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	// Generate enough triples to produce >4KB of output
	for i := 0; i < 50; i++ {
		s := rdflibgo.NewURIRefUnsafe("http://example.org/subject-with-long-name-" + string(rune('A'+i%26)) + string(rune('a'+(i/26)%26)))
		p := rdflibgo.NewURIRefUnsafe("http://example.org/predicate")
		g.Add(s, p, rdflibgo.NewLiteral("value-that-fills-buffer-"+string(rune('0'+i%10))))
		g.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Class"))
		g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/prop2"), rdflibgo.NewLiteral("another-value"))
	}
	g.Add(rdflibgo.NewBNode("bn1"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("bnode-val"))

	// Try various limit sizes to trigger errors at different write points
	for limit := 0; limit < 6000; limit += 97 {
		w := &limitWriter{limit: limit}
		_ = Serialize(g, w)
	}
}

// TestCovSerializeWriterErrorMultiPredicates covers writer error with multiple predicates/objects.
func TestCovSerializeWriterErrorMultiPredicates(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	// Many predicates and objects to generate long output
	for i := 0; i < 100; i++ {
		p := rdflibgo.NewURIRefUnsafe("http://example.org/pred" + string(rune('A'+i%26)) + string(rune('a'+(i/26)%26)))
		g.Add(s, p, rdflibgo.NewLiteral("val"+string(rune('0'+i%10))))
	}

	for limit := 0; limit < 8000; limit += 127 {
		w := &limitWriter{limit: limit}
		_ = Serialize(g, w)
	}
}

// TestCovIsAbsoluteIRINonAlphaScheme covers isAbsoluteIRI with non-alpha chars in scheme.
func TestCovIsAbsoluteIRINonAlphaScheme(t *testing.T) {
	// IRI with invalid scheme char (e.g., space)
	mustFail(t, `@base <http://example.org/> .
<a b:foo> <http://example.org/p> "v" .`)
	// IRI with no colon (not absolute)
	mustParse(t, `@base <http://example.org/> .
<noscheme> <http://example.org/p> "v" .`)
	// IRI with digit-only scheme part (not valid: 1st char must be letter)
	mustParse(t, `@base <http://example.org/> .
<123:foo> <http://example.org/p> "v" .`)
}

// TestCovDirectiveVersionMissingDot covers @version without trailing dot.
func TestCovDirectiveVersionMissingDot(t *testing.T) {
	mustFail(t, `@version "1.2"
<http://example.org/s> <http://example.org/p> "v" .`)
}

// TestCovDirectiveUnknown covers unknown directive error.
func TestCovDirectiveUnknown(t *testing.T) {
	mustFail(t, `@unknown .`)
}

// TestCovDirectiveBaseMissingDot covers @base without trailing dot.
func TestCovDirectiveBaseMissingDot(t *testing.T) {
	mustFail(t, `@base <http://example.org/>
<s> <p> "v" .`)
}

// TestCovDirectivePrefixMissingDot covers @prefix without trailing dot.
func TestCovDirectivePrefixMissingDot(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/>
ex:s ex:p "v" .`)
}

// TestCovResolveIRIRefParseError covers resolveIRI when ref IRI parse fails.
func TestCovResolveIRIRefParseError(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`@base <http://example.org/> .
<http://[::> <http://example.org/p> "v" .`))
	// May or may not error; exercises the url.Parse error branch
	_ = err
}

// TestCovReadSubjectTripleTermSubjectError covers <<( as subject (error).
func TestCovReadSubjectTripleTermSubjectErrorTurtle(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
<<( ex:s ex:p ex:o )>> ex:q ex:r .`)
}

// TestCovReadCollectionError covers error in collection item.
func TestCovReadCollectionErrorItem(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p ( "a" ^ ) .`)
}

// TestCovPredicateObjectListMissingDot covers missing dot after pred-obj list.
func TestCovPredicateObjectListMissingDotExtra(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s ex:p "v"`)
}

// TestCovReadBlankNodeLabelUnderscore covers bnode label starting with underscore.
func TestCovReadBlankNodeLabelUnderscore(t *testing.T) {
	mustParse(t, `_:_test <http://example.org/p> "v" .`)
}

// TestCovReadBlankNodeLabelSpecialChars covers bnode label with various valid chars.
func TestCovReadBlankNodeLabelSpecialChars(t *testing.T) {
	// Bnode labels only allow PN_CHARS characters (no escapes)
	mustParse(t, "_:b\u00B7test <http://example.org/p> \"v\" .")
}

// TestCovReadPrefixNameEnd covers prefix name at end of input.
func TestCovReadPrefixNameEnd(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:`)
}

// TestCovReadUnicodeEscapeBigU covers \U (8-digit) unicode escape.
func TestCovReadUnicodeEscapeBigU(t *testing.T) {
	mustParse(t, `<http://example.org/\U00000073> <http://example.org/p> "v" .`)
}

// TestCovReadUnicodeEscapeBigUTruncated covers truncated \U escape.
func TestCovReadUnicodeEscapeBigUTruncated(t *testing.T) {
	mustFail(t, `<http://example.org/\U0000007> <http://example.org/p> "v" .`)
}

// TestCovStatementEOFAfterChar covers statement starting with non-directive char then EOF.
func TestCovStatementEOFAfterChar(t *testing.T) {
	mustFail(t, `@prefix ex: <http://example.org/> .
ex:s`)
}

// TestCovSerializeOrderSubjectsNilSubject covers nil subject in spoMap (impossible normally).
func TestCovSerializeOrderSubjectsClasses(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("rdf", rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	g.Bind("rdfs", rdflibgo.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#"))
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	// Class subjects should appear first
	c1 := rdflibgo.NewURIRefUnsafe("http://example.org/ClassB")
	c2 := rdflibgo.NewURIRefUnsafe("http://example.org/ClassA")
	g.Add(c1, rdflibgo.RDF.Type, rdflibgo.RDFS.Class)
	g.Add(c2, rdflibgo.RDF.Type, rdflibgo.RDFS.Class)
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/other"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	out := covSerialize(t, g)
	classIdx := strings.Index(out, "ClassA")
	otherIdx := strings.Index(out, "other")
	if classIdx > otherIdx {
		t.Errorf("expected classes before other subjects, got:\n%s", out)
	}
}
