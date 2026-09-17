package shacl

import "maps"

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
	cfg := newConfig(opts)
	af, dataGraph := prepareAdvanced(dataGraph, shapesGraph, cfg)
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
	}}
}

func (p *Prepared) evaluation() *evalContext {
	return &evalContext{
		dataGraph:      p.ctx.dataGraph,
		shapesGraph:    p.ctx.shapesGraph,
		shapesMap:      maps.Clone(p.ctx.shapesMap),
		classInstances: p.ctx.classInstances,
		cfg:            p.ctx.cfg,
		af:             p.ctx.af,
	}
}

func (p *Prepared) inspector() *evalContext {
	if p.inspection == nil {
		p.inspection = p.evaluation()
	}
	return p.inspection
}
