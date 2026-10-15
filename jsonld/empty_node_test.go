package jsonld

import (
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/term"
)

// An ill-formed IRI as a list element left json-gold with a nil node, and
// its N-Quads serializer panicked on it: a 70-byte document took the
// process down. Found by FuzzParseNodes.
const emptyNodeDoc = `{"@context":{"@base":"http://base/"},"@id":"http://e/s","http://e/p":{"@list":[{"@id":"%"}]}}`

func TestIllFormedListElementIsDropped(t *testing.T) {
	g := rdflibgo.NewGraph()
	var skipped []string
	err := Parse(t.Context(), g, strings.NewReader(emptyNodeDoc), WithSkipHandler(func(st string, err error) {
		if !errors.Is(err, term.ErrInvalidIRI) {
			t.Errorf("skip reason %v, want ErrInvalidIRI", err)
		}
		skipped = append(skipped, st)
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0], "rdf-syntax-ns#first> ?") {
		t.Fatalf("skipped %q, want the rdf:first statement with no object", skipped)
	}
	// The list cell and its link to the subject stay, as for any other
	// dropped statement.
	if n, _ := g.Len(t.Context()); n != 2 {
		t.Fatalf("graph has %d triples, want 2 (p -> cell, cell rdf:rest nil)", n)
	}
}

func TestIllFormedListElementStrict(t *testing.T) {
	err := Parse(t.Context(), rdflibgo.NewGraph(), strings.NewReader(emptyNodeDoc), WithStrictIRIs())
	if !errors.Is(err, term.ErrInvalidIRI) || errors.Is(err, ErrProcessorPanic) {
		t.Fatalf("error = %v, want ErrInvalidIRI", err)
	}
}

// TestTrailingDataIsAnError: the decoder stopped at the end of the first
// JSON value, so whatever followed was dropped without a word.
func TestTrailingDataIsAnError(t *testing.T) {
	for _, doc := range []string{
		`{"@id": "http://e/a", "http://e/p": "1"} {"@id": "http://e/b", "http://e/p": "2"}`,
		`{"@id": "http://e/a", "http://e/p": "1"}]`,
		`0,{"0":""}`,
	} {
		err := Parse(t.Context(), rdflibgo.NewGraph(), strings.NewReader(doc))
		if !errors.Is(err, ErrInvalidJSON) || !strings.Contains(err.Error(), "after the JSON value") {
			t.Errorf("%s: error = %v, want ErrInvalidJSON for trailing data", doc, err)
		}
	}
	if err := Parse(t.Context(), rdflibgo.NewGraph(), strings.NewReader("{\"@id\": \"http://e/a\", \"http://e/p\": \"1\"}\n\t \n")); err != nil {
		t.Errorf("trailing whitespace rejected: %v", err)
	}
}

// TestUnsupportedJSONLiteralIsAnError: json-gold stored its own error text
// as the value of a JSON literal it could not canonicalise.
func TestUnsupportedJSONLiteralIsAnError(t *testing.T) {
	for _, v := range []string{`[1, 2]`, `"text"`, `null`} {
		doc := `{"@id": "http://e/s", "http://e/p": {"@value": ` + v + `, "@type": "@json"}}`
		g := rdflibgo.NewGraph()
		err := Parse(t.Context(), g, strings.NewReader(doc))
		if !errors.Is(err, ErrUnsupportedJSONLiteral) {
			t.Errorf("%s: error = %v, want ErrUnsupportedJSONLiteral", v, err)
		}
		if n, _ := g.Len(t.Context()); n != 0 {
			t.Errorf("%s: %d triples added", v, n)
		}
	}
	doc := `{"@id": "http://e/s", "http://e/p": {"@value": {"b": 1, "a": [2]}, "@type": "@json"}}`
	g := rdflibgo.NewGraph()
	if err := Parse(t.Context(), g, strings.NewReader(doc)); err != nil {
		t.Fatalf("an object JSON literal: %v", err)
	}
}
