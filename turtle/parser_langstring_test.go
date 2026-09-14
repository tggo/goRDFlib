package turtle

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestLangStringDatatypeNeedsALanguageTag: "a"^^rdf:langString was accepted
// and stored as a literal of that datatype with no tag, which RDF 1.1 Concepts
// §3.3 rules out and the N-Triples parser rejects.
func TestLangStringDatatypeNeedsALanguageTag(t *testing.T) {
	const rdf = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	for _, obj := range []string{
		`"a"^^<` + rdf + `langString>`,
		`"a"^^rdf:langString`,
		`"a"^^<` + rdf + `dirLangString>`,
		`<<( <http://e/s> <http://e/p> "a"^^rdf:langString )>>`,
	} {
		src := "@prefix rdf: <" + rdf + "> .\n<http://e/s> <http://e/p> " + obj + " ."
		err := Parse(rdflibgo.NewGraph(), strings.NewReader(src))
		if err == nil {
			t.Errorf("%s: accepted", obj)
			continue
		}
		if !strings.Contains(err.Error(), "line 2") {
			t.Errorf("%s: error has no position: %v", obj, err)
		}
	}
	src := "<http://e/s> <http://e/p> \"a\"@en, \"b\"@ar--rtl ."
	if err := Parse(rdflibgo.NewGraph(), strings.NewReader(src)); err != nil {
		t.Errorf("language-tagged literals rejected: %v", err)
	}
}
