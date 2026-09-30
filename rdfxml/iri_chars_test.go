package rdfxml

import (
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/term"
)

// An attribute value is legal XML long before it is a legal IRI. The parser
// used to take rdf:about="urn:a|b" as it was, and the serializer then refused
// the graph. Every place an IRI comes from is held to term.ValidIRI now.
func TestParseRejectsIllegalIRIs(t *testing.T) {
	const head = `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/">`
	cases := map[string]string{
		"rdf:about":       `<rdf:Description rdf:about="urn:a|b"><ex:p>v</ex:p></rdf:Description>`,
		"rdf:resource":    `<rdf:Description rdf:about="urn:s"><ex:p rdf:resource="urn:o{x}"/></rdf:Description>`,
		"rdf:datatype":    `<rdf:Description rdf:about="urn:s"><ex:p rdf:datatype="urn:d^t">v</ex:p></rdf:Description>`,
		"rdf:type attr":   `<rdf:Description rdf:about="urn:s" rdf:type="urn:c` + "`" + `x"/>`,
		"typed node":      `<t:C xmlns:t="urn:t|" rdf:about="urn:s"/>`,
		"property":        `<rdf:Description rdf:about="urn:s"><q:p xmlns:q="urn:q{">v</q:p></rdf:Description>`,
		"property attr":   `<rdf:Description rdf:about="urn:s" xmlns:q="urn:q|" q:p="v"/>`,
		"rdf:ID via base": `<rdf:Description xml:base="urn:b|" rdf:ID="x"><ex:p>v</ex:p></rdf:Description>`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			err := Parse(rdflibgo.NewGraph(), strings.NewReader(head+body+`</rdf:RDF>`))
			if !errors.Is(err, term.ErrInvalidIRI) {
				t.Fatalf("err = %v, want ErrInvalidIRI", err)
			}
			if !strings.Contains(err.Error(), "line ") {
				t.Fatalf("error has no position: %v", err)
			}
		})
	}

	// Percent-encoded, the same IRIs are fine.
	ok := head + `<rdf:Description rdf:about="urn:a%7Cb"><ex:p rdf:datatype="urn:d%5Et">v</ex:p></rdf:Description></rdf:RDF>`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(ok)); err != nil || g.Len() != 1 {
		t.Fatalf("percent-encoded IRIs: err=%v len=%d", err, g.Len())
	}
}
