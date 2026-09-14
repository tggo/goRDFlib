package rdfxml

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/testutil"
)

// serializeWellFormed serializes g, fails the test on a serializer error, and
// checks the output is namespace-well-formed XML: every token decodes, no
// start tag repeats an attribute, and every prefix used is declared.
func serializeWellFormed(t *testing.T, g *rdflibgo.Graph) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	out := buf.String()
	if err := checkWellFormed(out); err != "" {
		t.Fatalf("output is not well-formed XML: %s\n%s", err, out)
	}
	return out
}

func checkWellFormed(doc string) string {
	d := xml.NewDecoder(strings.NewReader(doc))
	d.Strict = true
	declared := map[string]bool{"": true, "xmlns": true, xmlNS: true}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return ""
		}
		if err != nil {
			return err.Error()
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		seen := make(map[xml.Name]bool, len(se.Attr))
		for _, a := range se.Attr {
			if seen[a.Name] {
				return "duplicate attribute " + a.Name.Space + ":" + a.Name.Local
			}
			seen[a.Name] = true
			if a.Name.Space == "xmlns" {
				declared[a.Value] = true
			}
		}
		// encoding/xml leaves an undeclared prefix in Name.Space as-is.
		if !declared[se.Name.Space] {
			return "undeclared prefix " + se.Name.Space + " on <" + se.Name.Local + ">"
		}
		if strings.Contains(se.Name.Local, ":") || se.Name.Local == "" {
			return "element name " + se.Name.Local + " is not a QName"
		}
		for _, a := range se.Attr {
			if !declared[a.Name.Space] {
				return "undeclared prefix " + a.Name.Space + " on attribute " + a.Name.Local
			}
		}
	}
}

// assertRoundTrip serializes g, checks well-formedness, parses the output
// back and requires an isomorphic graph. It returns the RDF/XML.
func assertRoundTrip(t *testing.T, g *rdflibgo.Graph) string {
	t.Helper()
	out := serializeWellFormed(t, g)
	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(out)); err != nil {
		t.Fatalf("output does not reparse: %v\n%s", err, out)
	}
	testutil.AssertGraphEqual(t, g, g2)
	if t.Failed() {
		t.Logf("output:\n%s", out)
	}
	return out
}

func iri(s string) rdflibgo.URIRef { return rdflibgo.NewURIRefUnsafe(s) }
