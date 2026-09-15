package endpoint

import (
	"errors"
	"io"

	"github.com/piprate/json-gold/ld"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/trig"
	"github.com/tggo/goRDFlib/turtle"
)

// rdfFormat is an RDF serialization the handler writes (CONSTRUCT results,
// graph reads, the service description) and possibly reads (Graph Store PUT
// and POST).
type rdfFormat struct {
	offer
	contentType string
	// formatIRI is the W3C format IRI for the service description.
	formatIRI string
	serialize func(g *graph.Graph, w io.Writer) error
	// parse is nil for formats that describe a dataset rather than one graph.
	parse func(g *graph.Graph, r io.Reader, base string) error
}

// ErrRemoteContext is returned when a JSON-LD body references a remote
// @context. The handler never fetches one: doing so on behalf of a client is a
// server-side request forgery risk. Inline the context instead.
var ErrRemoteContext = errors.New("endpoint: remote JSON-LD contexts are not fetched; inline the @context")

type noRemoteLoader struct{}

func (noRemoteLoader) LoadDocument(u string) (*ld.RemoteDocument, error) {
	return nil, ld.NewJsonLdError(ld.LoadingRemoteContextFailed, ErrRemoteContext.Error()+": "+u)
}

// rdfFormats is the server preference order for RDF responses. Turtle comes
// first: GSP §5.2 requires RDF/XML, Turtle or N-Triples when there is no
// Accept header.
var rdfFormats = []rdfFormat{
	{
		offer:       offer{mediaType: "text/turtle", aliases: []string{"application/x-turtle", "text/n3", "text/rdf+n3"}},
		contentType: "text/turtle; charset=utf-8",
		formatIRI:   "http://www.w3.org/ns/formats/Turtle",
		serialize:   func(g *graph.Graph, w io.Writer) error { return turtle.Serialize(g, w) },
		parse: func(g *graph.Graph, r io.Reader, base string) error {
			return turtle.Parse(g, r, turtle.WithBase(base))
		},
	},
	{
		offer:       offer{mediaType: "application/n-triples", aliases: []string{"text/plain"}},
		contentType: "application/n-triples",
		formatIRI:   "http://www.w3.org/ns/formats/N-Triples",
		serialize:   func(g *graph.Graph, w io.Writer) error { return nt.Serialize(g, w) },
		parse: func(g *graph.Graph, r io.Reader, base string) error {
			return nt.Parse(g, r, nt.WithBase(base))
		},
	},
	{
		offer:       offer{mediaType: "application/ld+json", aliases: []string{"application/json"}},
		contentType: "application/ld+json",
		formatIRI:   "http://www.w3.org/ns/formats/JSON-LD",
		serialize:   func(g *graph.Graph, w io.Writer) error { return jsonld.Serialize(g, w) },
		parse: func(g *graph.Graph, r io.Reader, base string) error {
			return jsonld.Parse(g, r, jsonld.WithBase(base), jsonld.WithDocumentLoader(noRemoteLoader{}))
		},
	},
	{
		offer:       offer{mediaType: "application/rdf+xml", aliases: []string{"application/xml", "text/xml"}},
		contentType: "application/rdf+xml",
		formatIRI:   "http://www.w3.org/ns/formats/RDF_XML",
		serialize:   func(g *graph.Graph, w io.Writer) error { return rdfxml.Serialize(g, w) },
		parse: func(g *graph.Graph, r io.Reader, base string) error {
			return rdfxml.Parse(g, r, rdfxml.WithBase(base))
		},
	},
	{
		offer:       offer{mediaType: "application/trig"},
		contentType: "application/trig",
		formatIRI:   "http://www.w3.org/ns/formats/TriG",
		serialize:   func(g *graph.Graph, w io.Writer) error { return trig.Serialize(g, w) },
	},
	{
		offer:       offer{mediaType: "application/n-quads"},
		contentType: "application/n-quads",
		formatIRI:   "http://www.w3.org/ns/formats/N-Quads",
		serialize:   func(g *graph.Graph, w io.Writer) error { return nq.Serialize(g, w) },
	},
}

var rdfOffers = func() []offer {
	out := make([]offer, len(rdfFormats))
	for i, f := range rdfFormats {
		out[i] = f.offer
	}
	return out
}()

// negotiateRDF returns the RDF format for an Accept header, or nil.
func negotiateRDF(accept string) *rdfFormat {
	i := negotiate(accept, rdfOffers)
	if i < 0 {
		return nil
	}
	return &rdfFormats[i]
}

// rdfFormatFor returns the parseable RDF format for a request media type, or
// nil. An empty media type is RDF/XML, as GSP §5.3 recommends.
func rdfFormatFor(mt string) *rdfFormat {
	if mt == "" {
		mt = "application/rdf+xml"
	}
	for i := range rdfFormats {
		f := &rdfFormats[i]
		if f.parse == nil {
			continue
		}
		if f.mediaType == mt {
			return f
		}
		for _, a := range f.aliases {
			if a == mt {
				return f
			}
		}
	}
	return nil
}
