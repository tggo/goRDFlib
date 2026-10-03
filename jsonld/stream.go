package jsonld

import (
	"context"
	"errors"
	"fmt"
	"io"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/stream"
)

// TripleHandler receives each triple from ParseStream. Returning a non-nil
// error stops the delivery; ParseStream returns that error.
type TripleHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error

// ErrNilHandler is returned by ParseStream for a nil handler.
var ErrNilHandler = errors.New("jsonld: ParseStream handler must not be nil")

// ParseStream parses JSON-LD from r and passes each triple to h instead of
// adding it to a graph. It takes the same options as Parse and delivers
// exactly the triples Parse would add, in the same order, with the same
// provenance calls. As with Parse, named graphs are flattened.
//
// JSON-LD cannot be streamed on the input side: the document is expanded
// whole, and h is called only after the conversion succeeded — a document
// that fails delivers nothing, matching Parse's "the graph is changed only
// when Parse succeeds". What the stream saves is building a graph, which is
// what a caller that indexes the triples itself (shacl.LoadJsonLD) or writes
// them straight out does not need. A triple repeated in the document may be
// delivered more than once; a graph would hold it once.
func ParseStream(r io.Reader, h TripleHandler, opts ...Option) error {
	return ParseStreamContext(context.Background(), r, h, opts...)
}

// ParseStreamContext is ParseStream that also stops, with an error matching
// ctx.Err(), once ctx is done. json-gold's expansion cannot be interrupted, so
// the context is checked before it and while delivering.
func ParseStreamContext(ctx context.Context, r io.Reader, h TripleHandler, opts ...Option) error {
	if h == nil {
		return ErrNilHandler
	}
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	stop := stream.New(ctx)
	if err := stop.Poll(); err != nil {
		return err
	}
	return parse(stop.Reader(r), &cfg, func(st statement) error {
		if err := stop.Tick(); err != nil {
			return err
		}
		if err := h(st.s, st.p, st.o); err != nil {
			return fmt.Errorf("jsonld: %w", err)
		}
		return nil
	})
}
