package shacl

import (
	"context"
	"maps"
)

// Prepared retains derived data and parsed shapes for validation and inspection.
// It is not safe for concurrent use, except that Conforms may be called from
// several goroutines at once. Caller-owned data and shapes must remain
// unchanged while it is in use; no mutable execution graphs are exposed.
type Prepared struct {
	ctx        evalContext
	inspection *evalContext
	targets    map[*Shape][]Term
	ordered    []*Shape // shapesInOrder of the parsed shapes when known up front; nil means compute it
}

// Prepare performs the same preparation as Validate, including the configured
// rule pass. It does not modify caller data. Existing WithErrorHandler reporting
// applies; a prepared run does not introduce a new execution error contract.
func Prepare(dataGraph, shapesGraph *Graph, opts ...Option) *Prepared {
	cfg := newConfig(opts)
	cfg.strictShapes = false // no error result to carry it; reported instead
	p, _ := prepare(context.Background(), dataGraph, shapesGraph, cfg, nil)
	return p
}

// PrepareContext is Prepare with a context: the rule pass and the reads of a
// store-backed graph run with ctx (see ErrCancelled). A stopped preparation
// returns a nil *Prepared, because rules cut short would leave the data
// without some of its inferred triples.
func PrepareContext(ctx context.Context, dataGraph, shapesGraph *Graph, opts ...Option) (*Prepared, error) {
	return prepare(ctx, dataGraph, shapesGraph, newConfig(opts), nil)
}

// prepare is PrepareContext over an already built config. With compiled set,
// its parsed shapes are used instead of parsing shapesGraph again; they must
// have been compiled with the same advanced-features setting as cfg, because
// addAFTargets has already been applied to them (or deliberately not).
// Ill-formed shapes are reported, or returned under WithStrictShapes, on every
// call: the error handler is often a per-call option.
func prepare(ctx context.Context, dataGraph, shapesGraph *Graph, cfg *config, compiled *CompiledShapes) (*Prepared, error) {
	if err := stopped(ctx); err != nil {
		return nil, err
	}
	if err := buildIndexes(ctx, dataGraph, shapesGraph); err != nil {
		return nil, err
	}
	var shapes map[string]*Shape
	var ordered []*Shape
	if compiled != nil {
		shapes, ordered = compiled.shapes, compiled.ordered
		if err := cfg.checkShapes(compiled.illFormed); err != nil {
			return nil, err
		}
	} else {
		shapes = parseShapes(shapesGraph)
		if err := cfg.checkShapes(illFormedShapes(shapesGraph, shapes)); err != nil {
			return nil, err
		}
	}
	af, dataGraph := prepareAdvanced(ctx, dataGraph, shapesGraph, cfg)
	if err := stopped(ctx); err != nil {
		return nil, err
	}
	if compiled == nil && af != nil {
		addAFTargets(af, shapes)
	}
	return &Prepared{ordered: ordered, ctx: evalContext{
		dataGraph:   dataGraph,
		shapesGraph: shapesGraph,
		shapesMap:   shapes,
		cfg:         cfg,
		af:          af,
	}}, nil
}

// evaluation returns a fresh context for one run over the prepared state,
// running with goctx.
func (p *Prepared) evaluation(goctx context.Context) *evalContext {
	return &evalContext{
		dataGraph:   p.ctx.dataGraph,
		shapesGraph: p.ctx.shapesGraph,
		shapesMap:   maps.Clone(p.ctx.shapesMap),
		cfg:         p.ctx.cfg,
		af:          p.ctx.af,
		guard:       &recursionGuard{ctx: goctx},
	}
}

func (p *Prepared) inspector() *evalContext {
	if p.inspection == nil {
		p.inspection = p.evaluation(nil)
	}
	return p.inspection
}
