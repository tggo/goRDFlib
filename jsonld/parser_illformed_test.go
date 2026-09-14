package jsonld

import (
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nt"
)

// rdflib #2168: "@type": "@id" on a node object makes json-gold emit <@id> as
// a type. The parse aborted with "relative IRI not allowed in N-Triples" and
// left the triples read so far in the graph.
const typeAtIDDoc = `{"@context":{"@base":"https://examples.r.us/","@vocab":"https://examples.r.us/vocab#"},` +
	`"@id":"Subject","loc":{"@id":"Georgia","@type":"@id"}}`

func TestParseDropsIllFormedTypeIRI(t *testing.T) {
	g := rdflibgo.NewGraph()
	var skipped []string
	var skipErrs []error
	err := Parse(g, strings.NewReader(typeAtIDDoc), WithSkipHandler(func(stmt string, err error) {
		skipped = append(skipped, stmt)
		skipErrs = append(skipErrs, err)
	}))
	if err != nil {
		t.Fatalf("ill-formed IRI must be dropped, got %v", err)
	}
	loc := rdflibgo.NewURIRefUnsafe("https://examples.r.us/vocab#loc")
	if g.Len() != 1 || !g.Contains(rdflibgo.NewURIRefUnsafe("https://examples.r.us/Subject"), loc, rdflibgo.NewURIRefUnsafe("https://examples.r.us/Georgia")) {
		t.Errorf("want only the loc triple, got %d triples", g.Len())
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0], "<@id>") || !errors.Is(skipErrs[0], nt.ErrRelativeIRI) {
		t.Errorf("skip handler: got %q, %v", skipped, skipErrs)
	}
}

func TestParseStrictIRIsLeavesGraphUnchanged(t *testing.T) {
	g := rdflibgo.NewGraph()
	keep := rdflibgo.NewURIRefUnsafe("http://e/keep")
	g.Add(keep, keep, keep)
	err := Parse(g, strings.NewReader(typeAtIDDoc), WithStrictIRIs())
	if !errors.Is(err, nt.ErrRelativeIRI) {
		t.Fatalf("want ErrRelativeIRI, got %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("failed parse changed the graph: %d triples", g.Len())
	}
}

// Any failure after some statements were read must not leave them behind.
func TestParseNQuadsFailureIsAtomic(t *testing.T) {
	const partial = `<http://example.org/s> <http://example.org/p> <http://example.org/good> .
<http://example.org/s> <http://example.org/p> .
`
	g := rdflibgo.NewGraph()
	var provenance int
	cfg := config{provenance: func(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term, int) { provenance++ }}
	if err := parseNQuadsInto(g, partial, &cfg, []byte(`{}`)); err == nil {
		t.Fatal("expected a syntax error")
	}
	if g.Len() != 0 || provenance != 0 {
		t.Errorf("failed parse left %d triples and %d provenance calls", g.Len(), provenance)
	}
}
