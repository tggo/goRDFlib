package trig

import (
	"context"
	"errors"
	"io"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/bnodes"
	"github.com/tggo/goRDFlib/internal/stream"
	"github.com/tggo/goRDFlib/store"
)

// QuadHandler receives each quad from ParseStream. graph is nil for a triple
// of the default graph, as in nq.StreamHandler. Returning a non-nil error stops
// the parse; ParseStream returns that error.
type QuadHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error

// ErrNilHandler is returned by ParseStream for a nil handler.
var ErrNilHandler = errors.New("trig: ParseStream handler must not be nil")

// ParseStream parses TriG from r and passes each quad to h instead of building
// a dataset. All parser options apply (base, provenance, blank-node scoping,
// depth limit). A handler error stops the parse before the next quad and is
// returned, wrapped with the line reached. An empty GRAPH block produces no
// call.
//
// Memory: no dataset is built, but the parser reads the whole input into
// memory first (see the package doc), so memory is bounded by the input size,
// not by the number of quads.
func ParseStream(r io.Reader, h QuadHandler, opts ...Option) error {
	return ParseStreamContext(context.Background(), r, h, opts...)
}

// ParseStreamContext is ParseStream that also stops, with an error matching
// ctx.Err(), once ctx is done.
func ParseStreamContext(ctx context.Context, r io.Reader, h QuadHandler, opts ...Option) error {
	if h == nil {
		return ErrNilHandler
	}
	cfg := newConfig(opts)
	stop := stream.New(ctx)
	data, err := io.ReadAll(stop.Reader(r))
	if err != nil {
		return err
	}
	p := newTrigParser(cfg, string(data), store.DefaultGraph, h)
	p.stop = stop
	return p.parse()
}

func newConfig(opts []Option) *config {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func newTrigParser(cfg *config, input string, defaultID rdflibgo.Term, sink QuadHandler) *trigParser {
	return &trigParser{
		sink:       sink,
		defaultID:  defaultID,
		input:      input,
		base:       cfg.base,
		prefixes:   make(map[string]string),
		provenance: cfg.provenance,
		bnodes:     bnodes.New(cfg.preserveBlankNodeIDs),
		maxDepth:   cfg.maxDepth(),
	}
}
