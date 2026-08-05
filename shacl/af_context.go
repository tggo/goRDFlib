package shacl

// afContext holds everything the SHACL-AF layer needs while it runs: the two
// graphs, the caller's options, and the functions declared by the shapes graph.
//
// It is not safe for concurrent use — applying rules mutates the data graph.
type afContext struct {
	dataGraph   *Graph
	shapesGraph *Graph
	cfg         *config
	funcs       map[string]*shaclFunction

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
	return ctx, nil
}

// evalCtx builds a validation context over the current state of the data graph.
//
// It is rebuilt rather than cached because rules add triples: a rule that
// infers an rdf:type changes which nodes a later rule's sh:targetClass selects,
// and a stale class index would hide that.
func (ctx *afContext) evalCtx() *evalContext {
	return &evalContext{
		dataGraph:      ctx.dataGraph,
		shapesGraph:    ctx.shapesGraph,
		shapesMap:      parseShapes(ctx.shapesGraph),
		classInstances: buildClassIndex(ctx.dataGraph),
		af:             ctx,
	}
}
