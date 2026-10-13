package integration_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/store/storetest"
	"github.com/tggo/goRDFlib/turtle"
)

// Issue #52: a store failure reaches the caller of every format. Parse returns
// it, and Serialize returns it without leaving a complete-looking document.
func TestStoreFailureReachesFormats(t *testing.T) {
	const doc = `<http://example.org/s> <http://example.org/p> "o" .` + "\n"

	formats := []struct {
		name      string
		parse     func(context.Context, *Graph, io.Reader) error
		serialize func(context.Context, *Graph, io.Writer) error
		input     string
	}{
		{"nt", func(c context.Context, g *Graph, r io.Reader) error { return nt.Parse(c, g, r) },
			func(c context.Context, g *Graph, w io.Writer) error { return nt.Serialize(c, g, w) }, doc},
		{"nq", func(c context.Context, g *Graph, r io.Reader) error { return nq.Parse(c, g, r) },
			func(c context.Context, g *Graph, w io.Writer) error { return nq.Serialize(c, g, w) }, doc},
		{"turtle", func(c context.Context, g *Graph, r io.Reader) error { return turtle.Parse(c, g, r) },
			func(c context.Context, g *Graph, w io.Writer) error { return turtle.Serialize(c, g, w) }, doc},
		{"rdfxml", func(c context.Context, g *Graph, r io.Reader) error { return rdfxml.Parse(c, g, r) },
			func(c context.Context, g *Graph, w io.Writer) error { return rdfxml.Serialize(c, g, w) },
			`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/">` +
				`<rdf:Description rdf:about="http://example.org/s"><ex:p>o</ex:p></rdf:Description></rdf:RDF>`},
		{"jsonld", func(c context.Context, g *Graph, r io.Reader) error { return jsonld.Parse(c, g, r) },
			func(c context.Context, g *Graph, w io.Writer) error { return jsonld.Serialize(c, g, w) },
			`{"@id":"http://example.org/s","http://example.org/p":"o"}`},
	}

	for _, f := range formats {
		t.Run(f.name+"/parse", func(t *testing.T) {
			fs := storetest.NewFailingStore(NewMemoryStore())
			g := NewGraph(WithStore(fs))
			fs.Fail(nil)
			err := f.parse(t.Context(), g, strings.NewReader(f.input))
			if !errors.Is(err, storetest.ErrInjected) {
				t.Fatalf("Parse err = %v, want the store's error", err)
			}
		})
		t.Run(f.name+"/serialize", func(t *testing.T) {
			fs := storetest.NewFailingStore(NewMemoryStore())
			g := NewGraph(WithStore(fs))
			if err := f.parse(t.Context(), g, strings.NewReader(f.input)); err != nil {
				t.Fatal(err)
			}
			fs.Fail(nil)
			var buf bytes.Buffer
			err := f.serialize(t.Context(), g, &buf)
			if !errors.Is(err, storetest.ErrInjected) {
				t.Fatalf("Serialize err = %v, want the store's error", err)
			}
			if strings.Contains(buf.String(), "example.org/p") {
				t.Errorf("Serialize wrote a document despite the failure:\n%s", buf.String())
			}
		})
	}
}
