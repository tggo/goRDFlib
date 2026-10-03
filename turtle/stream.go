package turtle

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
var ErrNilHandler = errors.New("turtle: ParseStream handler must not be nil")

// ParseStream parses Turtle from r and passes each triple to h instead of
// adding it to a graph. All parser options apply (base, provenance, blank-node
// scoping, depth limit). A handler error stops the parse before the next
// triple and is returned, wrapped with the line reached.
//
// Memory: no graph is built, so triples cost nothing once h returns. The
// parser still reads the whole input into memory first (see the package doc),
// so memory is bounded by the input size, not by the number of triples —
// unlike nt and nq, which hold one line.
func ParseStream(r io.Reader, h TripleHandler, opts ...Option) error {
	return ParseStreamContext(context.Background(), r, h, opts...)
}

// ParseStreamContext is ParseStream that also stops, with an error matching
// ctx.Err(), once ctx is done.
func ParseStreamContext(ctx context.Context, r io.Reader, h TripleHandler, opts ...Option) error {
	if h == nil {
		return ErrNilHandler
	}
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	stop := stream.New(ctx)
	data, err := io.ReadAll(stop.Reader(r))
	if err != nil {
		return err
	}
	p := newParser(cfg, string(data), h)
	p.stop = stop
	return p.parse()
}
