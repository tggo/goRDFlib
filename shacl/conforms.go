package shacl

import (
	"context"
	"errors"
	"fmt"
)

// ErrUnknownShape is returned by Conforms for a shape term that is not a shape
// in the shapes graph.
var ErrUnknownShape = errors.New("shacl: unknown shape")

// Conforms reports whether node conforms to shape over the prepared data graph
// (issue #44): the question sh:node, sh:qualifiedValueShape and the logical
// constraints ask internally. The shape's declared targets are ignored; node
// need not be one of them. Nested shapes, sh:and/sh:or/sh:xone/sh:not and
// recursive shapes are evaluated exactly as validation evaluates them, and a
// recursive (shape, node) pair is assumed to conform (SHACL §3.4.3).
//
// Any validation result makes node non-conforming, whatever its severity: this
// is conformance as sh:node checks it, not the report-level sh:conforms that
// ignores sh:Debug and sh:Trace. A deactivated shape conforms. shape may be
// any node of the shapes graph that carries constraints, including a blank
// node; one that is not returns ErrUnknownShape.
//
// Conforms is safe for concurrent use with other Conforms calls on the same
// Prepared: each call evaluates in its own state. A stopped ctx returns an
// error matching ErrCancelled, never a verdict.
func (p *Prepared) Conforms(goctx context.Context, node, shape Term) (bool, error) {
	if err := stopped(goctx); err != nil {
		return false, err
	}
	ctx := p.evaluation(goctx)
	s := resolveShape(ctx, shape)
	if s == nil {
		return false, fmt.Errorf("%w: %s", ErrUnknownShape, shape)
	}
	ok := nodeConforms(ctx, s, node)
	// A bound store that gave up on a read looks like "no match", which could
	// turn a violation into conformance; poll before trusting the verdict.
	if err := stopped(goctx); err != nil {
		return false, err
	}
	return ok, nil
}

// Conforms is Prepared.Conforms against the compiled shapes and dataGraph.
// opts apply as they do to ValidateContext. To ask about many nodes or shapes
// over one data graph, call CompiledShapes.Prepare once and use
// Prepared.Conforms: each call here prepares dataGraph again (and, with advanced features, runs
// its rules again).
func (c *CompiledShapes) Conforms(ctx context.Context, dataGraph *Graph, node, shape Term, opts ...Option) (bool, error) {
	p, err := c.Prepare(ctx, dataGraph, opts...)
	if err != nil {
		return false, err
	}
	return p.Conforms(ctx, node, shape)
}

// Prepare prepares dataGraph against the compiled shapes, for repeated
// Conforms calls or inspection over one data graph without parsing the shapes
// again. opts apply as they do to ValidateContext.
func (c *CompiledShapes) Prepare(ctx context.Context, dataGraph *Graph, opts ...Option) (*Prepared, error) {
	cfg := newConfig(c.options(opts))
	compiled := c
	if cfg.advanced != c.advanced {
		compiled = nil
	}
	return prepare(ctx, dataGraph, c.shapesGraph, cfg, compiled)
}
