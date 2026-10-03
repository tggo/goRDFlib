package shacl

import (
	"context"
	"errors"
)

// ErrNilShapesGraph is returned by CompileShapes when given no shapes graph.
var ErrNilShapesGraph = errors.New("shacl: nil shapes graph")

// CompiledShapes is a shapes graph parsed once, for validating many data
// graphs against the same shapes (issue #39). Validate and ValidateContext
// parse the shapes graph on every call, which dominates the cost of
// validating many small documents.
//
// CompiledShapes is safe for concurrent use: the parsed shapes are only read
// during validation, and each run clones the shape map before anything
// (an anonymous sh:condition or sh:filterShape) is added to it. The shapes
// graph must not be modified after CompileShapes; data graphs may differ on
// every call.
type CompiledShapes struct {
	shapesGraph *Graph
	opts        []Option
	advanced    bool
	shapes      map[string]*Shape
	ordered     []*Shape // shapesInOrder(shapes), sorted once instead of on every run
	illFormed   []error  // illFormedShapes(shapes), reported on every run
}

// CompileShapes parses shapesGraph with opts. The options also apply to every
// validation made with the result; see CompiledShapes.ValidateContext for
// options given per call.
func CompileShapes(shapesGraph *Graph, opts ...Option) (*CompiledShapes, error) {
	if shapesGraph == nil {
		return nil, ErrNilShapesGraph
	}
	if err := buildIndexes(context.Background(), shapesGraph); err != nil {
		return nil, err
	}
	opts = append([]Option(nil), opts...)
	cfg := newConfig(opts)
	shapes := parseShapes(shapesGraph)
	illFormed := illFormedShapes(shapesGraph, shapes)
	if cfg.strictShapes && len(illFormed) > 0 {
		return nil, errors.Join(illFormed...)
	}
	if cfg.advanced {
		// sh:target definitions come from the shapes graph alone, so they are
		// attached here once. Problems loading them are reported per
		// validation, exactly as Validate reports them.
		if af, err := newAFContext(NewGraph(), shapesGraph, cfg); err == nil {
			addAFTargets(af, shapes)
		}
	}
	return &CompiledShapes{
		shapesGraph: shapesGraph,
		opts:        opts,
		advanced:    cfg.advanced,
		shapes:      shapes,
		ordered:     shapesInOrder(shapes),
		illFormed:   illFormed,
	}, nil
}

// Validate is the package-level Validate against the compiled shapes.
func (c *CompiledShapes) Validate(dataGraph *Graph, opts ...Option) ValidationReport {
	opts = append(opts[:len(opts):len(opts)], lenientShapes)
	report, _ := c.ValidateContext(context.Background(), dataGraph, opts...)
	return report
}

// ValidateContext is the package-level ValidateContext against the compiled
// shapes. opts are applied after the options given to CompileShapes, which is
// how per-document options such as WithSourceLines or WithErrorHandler are
// passed. An option that switches advanced features relative to
// CompileShapes makes this call parse the shapes again, since the compiled
// ones were read for the other mode.
func (c *CompiledShapes) ValidateContext(ctx context.Context, dataGraph *Graph, opts ...Option) (ValidationReport, error) {
	all := c.opts
	if len(opts) > 0 {
		all = append(append(make([]Option, 0, len(c.opts)+len(opts)), c.opts...), opts...)
	}
	cfg := newConfig(all)
	compiled := c
	if cfg.advanced != c.advanced {
		compiled = nil
	}
	p, err := prepare(ctx, dataGraph, c.shapesGraph, cfg, compiled)
	if err != nil {
		return ValidationReport{}, err
	}
	return p.ValidateContext(ctx)
}
