package rdfxml

import (
	"strings"
	"testing"
	"unsafe"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestParseSharesRepeatedIRIs: property and typed-node element names and
// datatypes repeated across a document are one URIRef each (issue #53).
func TestParseSharesRepeatedIRIs(t *testing.T) {
	doc := `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:s="https://schema.org/">
  <s:Person rdf:about="urn:a"><s:age rdf:datatype="http://www.w3.org/2001/XMLSchema#integer">1</s:age></s:Person>
  <s:Person rdf:about="urn:b"><s:age rdf:datatype="http://www.w3.org/2001/XMLSchema#integer">2</s:age></s:Person>
</rdf:RDF>`
	var types, preds, dts []rdflibgo.URIRef
	err := ParseStream(strings.NewReader(doc), func(_ rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
		if p == rdflibgo.RDF.Type {
			types = append(types, o.(rdflibgo.URIRef))
			return nil
		}
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
	if len(types) != 2 || !same(types[0], types[1]) {
		t.Error("a repeated typed-node element was built twice")
	}
	if len(preds) != 2 || !same(preds[0], preds[1]) {
		t.Error("a repeated property element was built twice")
	}
	if !same(dts[0], dts[1]) {
		t.Error("a repeated datatype was built twice")
	}
}
