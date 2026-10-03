package rdfxml

import (
	"context"
	"errors"
	"io"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/stream"
)

// TripleHandler receives each triple from ParseStream. Returning a non-nil
// error stops the parse; ParseStream returns that error.
type TripleHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error

// ErrNilHandler is returned by ParseStream for a nil handler.
var ErrNilHandler = errors.New("rdfxml: ParseStream handler must not be nil")

// ParseStream parses RDF/XML from r and passes each triple to h instead of
// adding it to a graph. All parser options apply (base, provenance, blank-node
// scoping). A handler error stops the parse before the next triple and is
// returned with the line and column reached; the input is not read further.
//
// The XML is decoded as it is read, so memory is bounded by the parser's
// state (open elements, rdf:ID values seen for the uniqueness check, blank
// node labels), not by the input or the number of triples.
func ParseStream(r io.Reader, h TripleHandler, opts ...Option) error {
	return ParseStreamContext(context.Background(), r, h, opts...)
}

// ParseStreamContext is ParseStream that also stops, with an error matching
// ctx.Err(), once ctx is done.
func ParseStreamContext(ctx context.Context, r io.Reader, h TripleHandler, opts ...Option) error {
	if h == nil {
		return ErrNilHandler
	}
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	p := newParser(&cfg, h)
	p.stop = stream.New(ctx)
	return p.parse(p.stop.Reader(r))
}
