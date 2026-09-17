package shacl

import (
	"context"
	"maps"
)

// Prepared retains derived data and parsed shapes for validation and inspection.
// It is not safe for concurrent use. Caller-owned data and shapes must remain
// unchanged while it is in use; no mutable execution graphs are exposed.
type Prepared struct {
	ctx        evalContext
	inspection *evalContext
	targets    map[*Shape][]Term
}

// Prepare performs the same preparation as Validate, including the configured
// rule pass. It does not modify caller data. Existing WithErrorHandler reporting
// applies; a prepared run does not introduce a new execution error contract.
func Prepare(dataGraph, shapesGraph *Graph, opts ...Option) *Prepared {
	p, _ := PrepareContext(context.Background(), dataGraph, shapesGraph, opts...)
	return p
}

// PrepareContext is Prepare with a context: the rule pass and the reads of a
// store-backed graph run with ctx (see ErrCancelled). A stopped preparation
// returns a nil *Prepared, because rules cut short would leave the data
// without some of its inferred triples.
func PrepareContext(ctx context.Context, dataGraph, shapesGraph *Graph, opts ...Option) (*Prepared, error) {
	if err := stopped(ctx); err != nil {
		return nil, err
	}
	if err := buildIndexes(ctx, dataGraph, shapesGraph); err != nil {
		return nil, err
	}
	cfg := newConfig(opts)
	af, dataGraph := prepareAdvanced(ctx, dataGraph, shapesGraph, cfg)
	if err := stopped(ctx); err != nil {
		return nil, err
	}
	shapes := parseShapes(shapesGraph)
	if af != nil {
		addAFTargets(af, shapes)
	}
	return &Prepared{ctx: evalContext{
		dataGraph:      dataGraph,
		shapesGraph:    shapesGraph,
		shapesMap:      shapes,
		classInstances: buildClassIndex(dataGraph),
		cfg:            cfg,
		af:             af,
	}}, nil
}

// evaluation returns a fresh context for one run over the prepared state,
// running with goctx.
func (p *Prepared) evaluation(goctx context.Context) *evalContext {
	return &evalContext{
		dataGraph:      p.ctx.dataGraph,
		shapesGraph:    p.ctx.shapesGraph,
		shapesMap:      maps.Clone(p.ctx.shapesMap),
		classInstances: p.ctx.classInstances,
		cfg:            p.ctx.cfg,
		af:             p.ctx.af,
		guard:          &recursionGuard{ctx: goctx},
	}
}

func (p *Prepared) inspector() *evalContext {
	if p.inspection == nil {
		p.inspection = p.evaluation(nil)
	}
	return p.inspection
}
