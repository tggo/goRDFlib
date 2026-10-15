package trig

import (
	"strings"
	"testing"
	"unsafe"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestParseSharesRepeatedIRIs: a predicate or datatype repeated across a
// document is one URIRef (issue #53), whether written as <…> or prefixed.
func TestParseSharesRepeatedIRIs(t *testing.T) {
	doc := `@prefix s: <https://schema.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
<urn:a> s:age "1"^^xsd:integer .
<urn:b> <https://schema.org/age> "2"^^<http://www.w3.org/2001/XMLSchema#integer> .`
	var preds, dts []rdflibgo.URIRef
	err := ParseStream(strings.NewReader(doc), func(_ rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, _ rdflibgo.Term) error {
		preds = append(preds, p)
		dts = append(dts, o.(rdflibgo.Literal).Datatype())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	same := func(a, b rdflibgo.URIRef) bool {
		return a == b && unsafe.StringData(a.Value()) == unsafe.StringData(b.Value())
	}
	if !same(preds[0], preds[1]) {
		t.Error("a repeated predicate was built twice")
	}
	if !same(dts[0], dts[1]) {
		t.Error("a repeated datatype was built twice")
	}
}
