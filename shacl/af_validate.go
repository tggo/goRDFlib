package shacl

// prepareAdvanced sets up the SHACL-AF layer for a validation run.
//
// It returns the AF context to attach to the evaluation, and the data graph to
// validate — a copy carrying the inferred triples when rules ran, otherwise the
// caller's graph untouched. Returning the graph rather than mutating in place
// is what keeps Validate free of side effects on its input.
func prepareAdvanced(dataGraph, shapesGraph *Graph, cfg *config) (*afContext, *Graph) {
	checkEntailment(shapesGraph, cfg)
	if !cfg.advanced {
		return nil, dataGraph
	}

	ctx, err := newAFContext(dataGraph, shapesGraph, cfg)
	if err != nil {
		cfg.report(err)
		return nil, dataGraph
	}
	cfg.report(ctx.loadErr)

	if !hasRules(shapesGraph) {
		return ctx, dataGraph
	}

	// Rules infer into a copy so that validating a graph never changes it.
	expanded := NewGraph()
	expanded.Merge(dataGraph)
	ctx.dataGraph = expanded
	if _, err := ctx.applyRules(); err != nil {
		cfg.report(err)
	}
	return ctx, expanded
}

// hasRules reports whether the shapes graph attaches any rule to any shape.
// Checked before copying the data graph, which is otherwise wasted work.
func hasRules(shapesGraph *Graph) bool {
	pred := IRI(SHRule)
	return shapesGraph.Has(nil, &pred, nil)
}

// addAFTargets attaches each shape's sh:target definitions to it.
//
// Core target properties are read during shape parsing; sh:target is read here
// instead because resolving it needs the AF context, and because a shapes graph
// that uses it must not behave differently when AF is off. Every caller that
// parses shapes for an AF run has to call this — a shape whose only target is a
// sh:target otherwise has no focus nodes at all.
func addAFTargets(ctx *afContext, shapes map[string]*Shape) {
	if ctx == nil {
		return
	}
	for _, s := range shapes {
		s.Targets = append(s.Targets, ctx.afTargets[s.ID.String()]...)
	}
}
