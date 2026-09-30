package jsonld

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/testutil"

	"github.com/piprate/json-gold/ld"
)

// outcome is everything a caller can observe about a Parse.
type outcome struct {
	g     *graph.Graph
	skips []string // "statement | error", sorted
	err   string
	prov  []string // "predicate object line", sorted
}

// lineNum masks the N-Quads line number in messages: it is the statement's
// position in json-gold's output, whose order differs between two expansions
// of one document, so it is not something the two paths could agree on.
var lineNum = regexp.MustCompile(`line \d+:`)

// provKey names a statement without its subject or blank-node labels, which
// differ between two parses of one document.
func provKey(p rdflibgo.URIRef, o rdflibgo.Term, line int) string {
	ok := term.TermKey(o)
	if _, isB := o.(rdflibgo.BNode); isB {
		ok = "_"
	}
	return fmt.Sprintf("%s %s %d", p.N3(), ok, line)
}

func runParse(doc string, forceText bool, opts ...Option) outcome {
	var out outcome
	out.g = rdflibgo.NewGraph()
	opts = append(opts,
		WithSkipHandler(func(stmt string, err error) {
			out.skips = append(out.skips, strings.TrimSpace(stmt)+" | "+err.Error())
		}),
		WithProvenance(func(_ rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, line int) {
			out.prov = append(out.prov, provKey(p, o, line))
		}),
		func(c *config) { c.forceTextPath = forceText },
	)
	if err := Parse(out.g, strings.NewReader(doc), opts...); err != nil {
		out.err = lineNum.ReplaceAllString(err.Error(), "line N:")
	}
	for i, s := range out.skips {
		out.skips[i] = lineNum.ReplaceAllString(s, "line N:")
	}
	sort.Strings(out.skips)
	sort.Strings(out.prov)
	return out
}

func equalOutcomes(t *testing.T, fast, text outcome) {
	t.Helper()
	if fast.err != text.err {
		t.Errorf("error differs:\n fast: %q\n text: %q", fast.err, text.err)
	}
	if strings.Join(fast.skips, "\n") != strings.Join(text.skips, "\n") {
		t.Errorf("skip handler calls differ:\n fast: %q\n text: %q", fast.skips, text.skips)
	}
	if strings.Join(fast.prov, "\n") != strings.Join(text.prov, "\n") {
		t.Errorf("provenance differs:\n fast: %q\n text: %q", fast.prov, text.prov)
	}
	testutil.AssertGraphEqual(t, text.g, fast.g)
}

// expand runs json-gold the way Parse does and returns its dataset.
func expand(t *testing.T, doc string) *ld.RDFDataset {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(doc), &v); err != nil {
		t.Fatal(err)
	}
	r, err := ld.NewJsonLdProcessor().ToRDF(v, ld.NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	return r.(*ld.RDFDataset)
}

func bigLiteral(n int) string { return strings.Repeat("x", n) }

// Documents that json-gold itself turns into RDF the fast path must either
// take or hand to the text path; either way the observable result is the same.
func TestDatasetPathsAgreeOnParse(t *testing.T) {
	cases := []struct {
		name     string
		doc      string
		opts     []Option
		wantFast bool
	}{
		{"clean", `{"@id":"http://a/s","http://a/p":[{"@value":"x"},{"@value":"y","@language":"en"},{"@value":"1","@type":"http://www.w3.org/2001/XMLSchema#integer"},{"@id":"http://a/o"}]}`, nil, true},
		{"blank nodes", `{"@id":"http://a/s","http://a/p":{"http://a/q":{"@id":"_:x"}},"http://a/r":{"@id":"_:x"}}`, nil, true},
		{"ill-formed type IRI dropped (<@id>)", typeAtIDDoc, nil, false},
		{"ill-formed type IRI strict", typeAtIDDoc, []Option{WithStrictIRIs()}, false},
		{"backslash in object IRI", `{"@id":"http://a/s","http://a/p":{"@id":"http://a/b\\c"}}`, nil, false},
		{"backslash in object IRI strict", `{"@id":"http://a/s","http://a/p":{"@id":"http://a/b\\c"}}`, []Option{WithStrictIRIs()}, false},
		{"backslash in datatype", `{"@id":"http://a/s","http://a/p":{"@value":"x","@type":"http://a/d\\e"}}`, nil, false},
		{"invalid language tag", `{"@id":"http://a/s","http://a/p":{"@value":"x","@language":"toolongtagxx"}}`, nil, false},
		{"invalid language tag with handler", `{"@id":"http://a/s","http://a/p":{"@value":"x","@language":"en.x"}}`, nil, true},
		{"literal at the line cap", `{"@id":"http://a/s","http://a/p":{"@value":"` + bigLiteral(70000) + `"}}`, nil, false},
		{"literal near the line cap", `{"@id":"http://a/s","http://a/p":{"@value":"` + bigLiteral(30000) + `"}}`, nil, false},
		{"literal at the line cap, unbounded", `{"@id":"http://a/s","http://a/p":{"@value":"` + bigLiteral(70000) + `"}}`, []Option{WithUnboundedLines()}, true},
		{"escapes in literal", `{"@id":"http://a/s","http://a/p":{"@value":"q\"b\\s\nn\rr\tt"}}`, nil, true},
		{"named graph", `{"@id":"http://a/g","@graph":[{"@id":"http://a/s","http://a/p":"v"}]}`, nil, true},
		{"preserve blank nodes", `{"@id":"_:a","http://a/p":{"@id":"_:b"}}`, []Option{WithPreserveBlankNodeIDs()}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, fastOK := datasetStatements(expand(t, tc.doc), false, hasUnbounded(tc.opts))
			if fastOK != tc.wantFast {
				t.Errorf("fast-path eligibility = %v, want %v", fastOK, tc.wantFast)
			}
			equalOutcomes(t, runParse(tc.doc, false, tc.opts...), runParse(tc.doc, true, tc.opts...))
		})
	}
}

func hasUnbounded(opts []Option) bool {
	var c config
	for _, o := range opts {
		o(&c)
	}
	return c.unbounded
}

// The default text path refuses a line over 64 KiB; the fast path must not
// quietly accept what it would refuse.
func TestDatasetPathLineCapIsReported(t *testing.T) {
	doc := `{"@id":"http://a/s","http://a/p":{"@value":"` + bigLiteral(70000) + `"}}`
	out := runParse(doc, false)
	if !strings.Contains(out.err, "line N: line exceeds maximum length") || !strings.Contains(out.err, "WithUnboundedLines") {
		t.Fatalf("want the actionable line-length error, got %q", out.err)
	}
	if err := Parse(rdflibgo.NewGraph(), strings.NewReader(doc)); !errors.Is(err, nq.ErrLineTooLong) {
		t.Errorf("error does not wrap ErrLineTooLong: %v", err)
	}
	if out.g.Len() != 0 {
		t.Errorf("failed parse left %d triples", out.g.Len())
	}
	if ok := runParse(doc, false, WithUnboundedLines()); ok.err != "" || ok.g.Len() != 1 {
		t.Errorf("unbounded: err=%q len=%d", ok.err, ok.g.Len())
	}
}

// Hand-built datasets reach the nodes json-gold filters out itself (relative
// IRIs, generalized RDF, odd labels). Every one of them must leave the fast
// path and then agree with the text path. Each case targets one eligibility
// check, so removing that check fails exactly its case.
func TestDatasetFastPathEligibility(t *testing.T) {
	iri := func(s string) ld.Node { return ld.NewIRI(s) }
	bn := func(s string) ld.Node { return ld.NewBlankNode(s) }
	lit := func(v, dt, lang string) ld.Node { return ld.NewLiteral(v, dt, lang) }
	s, p := iri("http://a/s"), iri("http://a/p")
	cases := []struct {
		name     string
		q        *ld.Quad
		wantFast bool
	}{
		{"baseline IRI object", ld.NewQuad(s, p, iri("http://a/o"), ""), true},
		{"baseline bnode", ld.NewQuad(bn("_:b0"), p, bn("_:b1"), ""), true},
		{"baseline plain literal", ld.NewQuad(s, p, lit("v", "", ""), ""), true},
		{"baseline typed literal", ld.NewQuad(s, p, lit("1", "http://www.w3.org/2001/XMLSchema#integer", ""), ""), true},
		{"baseline lang literal", ld.NewQuad(s, p, lit("v", ld.RDFLangString, "en-GB"), ""), true},
		{"relative subject", ld.NewQuad(iri("rel"), p, iri("http://a/o"), ""), false},
		{"relative predicate", ld.NewQuad(s, iri("rel"), iri("http://a/o"), ""), false},
		{"relative object", ld.NewQuad(s, p, iri("a/b"), ""), false},
		{"ill-formed IRI (space)", ld.NewQuad(s, p, iri("http://a/b c"), ""), false},
		{"ill-formed IRI (angle bracket)", ld.NewQuad(s, p, iri("http://a/<b>"), ""), false},
		{"backslash in subject", ld.NewQuad(iri(`http://a/b\c`), p, iri("http://a/o"), ""), false},
		{"backslash in object", ld.NewQuad(s, p, iri(`http://a/b\c`), ""), false},
		{"scheme starts with digit", ld.NewQuad(s, p, iri("1http://a"), ""), false},
		{"scheme with bad character", ld.NewQuad(s, p, iri("ht_tp://a"), ""), false},
		{"blank node label with dot", ld.NewQuad(bn("_:a.b"), p, iri("http://a/o"), ""), false},
		{"blank node label without prefix", ld.NewQuad(bn("b0"), p, iri("http://a/o"), ""), false},
		{"blank node object label with space", ld.NewQuad(s, p, bn("_:a b"), ""), false},
		{"blank node predicate", ld.NewQuad(s, bn("_:p"), iri("http://a/o"), ""), false},
		{"literal subject", ld.NewQuad(lit("v", "", ""), p, iri("http://a/o"), ""), false},
		{"invalid language tag", ld.NewQuad(s, p, lit("v", ld.RDFLangString, "toolongtagxx"), ""), false},
		{"directional language tag", ld.NewQuad(s, p, lit("v", ld.RDFLangString, "en--ltr"), ""), false},
		{"language on a typed literal", ld.NewQuad(s, p, lit("v", "http://a/d", "en"), ""), false},
		{"empty datatype", ld.NewQuad(s, p, ld.Literal{Value: "v"}, ""), false},
		{"relative datatype", ld.NewQuad(s, p, lit("v", "rel", ""), ""), false},
		{"backslash in datatype", ld.NewQuad(s, p, lit("v", `http://a/d\e`, ""), ""), false},
		{"pipe in datatype", ld.NewQuad(s, p, lit("v", "urn:x|y", ""), ""), false},
		{"brace in datatype", ld.NewQuad(s, p, lit("v", "urn:x{y}", ""), ""), false},
		{"quote in datatype", ld.NewQuad(s, p, lit("v", `urn:x"y`, ""), ""), false},
		{"literal at the line cap", ld.NewQuad(s, p, lit(bigLiteral(70000), "", ""), ""), false},
		{"IRI at the line cap", ld.NewQuad(s, p, iri("http://a/"+bigLiteral(70000)), ""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds := ld.NewRDFDataset()
			ds.Graphs["@default"] = []*ld.Quad{ld.NewQuad(s, p, iri("http://a/first"), ""), tc.q}
			_, ok := datasetStatements(ds, false, false)
			if ok != tc.wantFast {
				t.Fatalf("fast-path eligibility = %v, want %v", ok, tc.wantFast)
			}
			for _, o := range [][]Option{nil, {WithStrictIRIs()}, {WithUnboundedLines()}} {
				var cFast, cText config
				for _, f := range o {
					f(&cFast)
					f(&cText)
				}
				cText.forceTextPath = true
				equalOutcomes(t, runDataset(ds, &cFast), runDataset(ds, &cText))
			}
		})
	}
}

func runDataset(ds *ld.RDFDataset, cfg *config) outcome {
	var out outcome
	out.g = rdflibgo.NewGraph()
	c := *cfg
	c.skipHandler = func(stmt string, err error) {
		out.skips = append(out.skips, strings.TrimSpace(stmt)+" | "+err.Error())
	}
	if err := addDataset(out.g, ds, &c, []byte(`{}`)); err != nil {
		out.err = lineNum.ReplaceAllString(err.Error(), "line N:")
	}
	for i, s := range out.skips {
		out.skips[i] = lineNum.ReplaceAllString(s, "line N:")
	}
	sort.Strings(out.skips)
	return out
}

// corpus is every JSON-LD document the package tests use, plus a generated
// schema.org-style document with the constructs nabu sends.
func corpus() []string {
	docs := []string{
		`{"@context":{"ex":"http://example.org/"},"@id":"ex:a","ex:p":"v"}`,
		`{"@context":{"id":"@id","ex":"http://example.org/"},"id":"http://example.org/a","ex:p":[1,2.5,true,null]}`,
		`[{"@id":"http://e/a","http://e/p":{"@list":[{"@value":"a"},{"@value":"b"}]}},{"@id":"http://e/b","@reverse":{"http://e/q":{"@id":"http://e/a"}}}]`,
		`{"@context":{"@vocab":"http://e/","l":{"@container":"@language"}},"@id":"http://e/s","l":{"en":"hello","de":["hallo","moin"]}}`,
		`{"@context":{"@base":"http://e/dir/","@vocab":"#"},"@id":"s","p":{"@id":"../o"},"q":{"@id":"#f"}}`,
		`{"@id":"http://e/g","@graph":[{"@id":"http://e/s","http://e/p":{"@id":"_:a"}},{"@id":"_:a","http://e/q":"v"}]}`,
		`{"@id":"http://e/s","@type":["http://e/T1","http://e/T2"],"http://e/p":{"@value":"2020-01-01","@type":"http://www.w3.org/2001/XMLSchema#date"}}`,
		`{"@id":"http://e/s","http://e/p":{"@value":"x","@language":"en"},"http://e/q":{"@value":"é中😀"}}`,
		`{"@id":"http://e/s","http://e/p":{"@value":{"a":1},"@type":"@json"}}`,
		`{"@id":"http://e/s","http://e/p":{"@id":"http://e/a b"}}`,
		`{"@id":"rel","http://e/p":"x"}`,
		`{}`, `[]`, `{"@context":{}}`,
	}
	for _, seed := range []string{typeAtIDDoc} {
		docs = append(docs, seed)
	}
	docs = append(docs, generatedPlace(40))
	return docs
}

// generatedPlace builds a Place-like document with n nested nodes.
func generatedPlace(n int) string {
	var b strings.Builder
	b.WriteString(`{"@context":{"@vocab":"https://schema.org/","xsd":"http://www.w3.org/2001/XMLSchema#"},"@id":"https://e.org/place/1","@type":"Place","name":"P","geo":{"@type":"GeoCoordinates","latitude":{"@value":"1.5","@type":"xsd:decimal"},"longitude":-2.25},"hasPart":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"@type":"Dataset","name":{"@value":"d%d","@language":"en"},"url":{"@id":"https://e.org/d/%d"},"variableMeasured":[{"@type":"PropertyValue","value":%d}]}`, i, i, i)
	}
	b.WriteString(`]}`)
	return b.String()
}

// Every corpus document goes through both paths under every option that can
// change the outcome, and the two must agree.
func TestDatasetPathsAgreeOnCorpus(t *testing.T) {
	variants := map[string][]Option{
		"default":    nil,
		"strict":     {WithStrictIRIs()},
		"unbounded":  {WithUnboundedLines()},
		"preserve":   {WithPreserveBlankNodeIDs()},
		"with base":  {WithBase("http://base.example/")},
		"strict+bas": {WithStrictIRIs(), WithBase("http://base.example/")},
	}
	for i, doc := range corpus() {
		for name, opts := range variants {
			t.Run(fmt.Sprintf("doc%d/%s", i, name), func(t *testing.T) {
				equalOutcomes(t, runParse(doc, false, opts...), runParse(doc, true, opts...))
			})
		}
	}
}

// Blank nodes share one scope per Parse on the fast path too.
func TestDatasetPathBlankNodeScopePerParse(t *testing.T) {
	const doc = `{"@id":"http://e/s","http://e/p":{"http://e/q":"v"}}`
	g := rdflibgo.NewGraph()
	for range 2 {
		if err := Parse(g, strings.NewReader(doc)); err != nil {
			t.Fatal(err)
		}
	}
	if g.Len() != 4 {
		t.Errorf("two parses of one document must give two blank nodes: %d triples", g.Len())
	}
}
