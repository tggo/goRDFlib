package shacl

import "context"

// afContext holds everything the SHACL-AF layer needs while it runs: the two
// graphs, the caller's options, and the functions declared by the shapes graph.
//
// It is not safe for concurrent use — applying rules mutates the data graph.
type afContext struct {
	dataGraph   *Graph
	shapesGraph *Graph
	cfg         *config
	funcs       map[string]*shaclFunction

	// ctx is the context rules run with; nil means context.Background. SHACL
	// functions do not use it: they run with the context of the query that
	// calls them.
	ctx context.Context

	// afTargets holds the sh:target definitions of the shapes graph, resolved
	// once and keyed by shape. Shapes are re-parsed on every rule round, so
	// resolving there would repeat the work and re-report the same error.
	afTargets map[string][]Target

	// loadErr is the first problem found while reading the shapes graph.
	// Loading continues past it so that one malformed declaration does not
	// disable everything else, but the error is still surfaced.
	loadErr error
}

// newAFContext reads the AF declarations out of the shapes graph.
//
// It returns an error only for a problem that leaves nothing usable; a single
// malformed function or rule is recorded in loadErr and reported to the caller
// once the usable part has run.
func newAFContext(dataGraph, shapesGraph *Graph, c *config) (*afContext, error) {
	if dataGraph == nil || shapesGraph == nil {
		return nil, ErrAdvancedFeatures
	}
	ctx := &afContext{dataGraph: dataGraph, shapesGraph: shapesGraph, cfg: c}
	funcs, err := loadFunctions(shapesGraph)
	ctx.funcs = funcs
	ctx.loadErr = err

	targets, terr := ctx.loadAFTargets()
	ctx.afTargets = targets
	if ctx.loadErr == nil {
		ctx.loadErr = terr
	}
	return ctx, nil
}

// evalCtx builds a validation context over the current state of the data graph.
//
// It is rebuilt rather than cached because rules add triples: a rule that
// infers an rdf:type changes which nodes a later rule's sh:targetClass selects,
// and a stale class index would hide that.
//
// The AF targets are attached here as well, so that a rule and a constraint on
// the same shape resolve the same focus nodes.
func (ctx *afContext) evalCtx() *evalContext {
	shapes := parseShapes(ctx.shapesGraph)
	addAFTargets(ctx, shapes)
	return &evalContext{
		dataGraph:      ctx.dataGraph,
		shapesGraph:    ctx.shapesGraph,
		shapesMap:      shapes,
		classInstances: buildClassIndex(ctx.dataGraph),
		cfg:            ctx.cfg,
		af:             ctx,
		guard:          &recursionGuard{ctx: ctx.ctx},
	}
}
