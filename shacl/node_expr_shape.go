package shacl

// Node expressions that check nodes against a shape: shnex:filterShape,
// shnex:findFirst, shnex:matchAll, shnex:nodesMatching and
// shnex:conformsToShape.

// resolveShape returns the shape ref names, parsing an anonymous or
// constraint-only shape from the shapes graph on first use. A node with no
// constraints at all is not a shape worth checking and yields nil, which every
// caller reads as "everything conforms".
func (ctx *nodeExprContext) resolveShape(ref Term) *Shape {
	if s := ctx.shapesMap[ref.String()]; s != nil {
		return s
	}
	if ctx.shapesMap == nil {
		ctx.shapesMap = make(map[string]*Shape)
	}
	return parseAdHocShape(ctx.shapeDefinitions(), ref, ctx.shapesMap)
}

// evalContext is the SHACL validation context in which shape conformance is
// checked from inside an expression.
func (ctx *nodeExprContext) validation() *evalContext {
	return &evalContext{
		dataGraph:   ctx.dataGraph,
		shapesGraph: ctx.shapeDefinitions(),
		shapesMap:   ctx.shapesMap,
		af:          ctx.af,
		guard:       ctx.sharedGuard(),
	}
}

// conforms reports whether node conforms to shape. A failure of the
// validation it runs (a constraint that is itself a failing expression, a
// query the engine cannot evaluate) is a failure of the expression.
func (ctx *nodeExprContext) conforms(shape *Shape, node Term) (bool, error) {
	eCtx := ctx.validation()
	ok := nodeConforms(eCtx, shape, node)
	if g := ctx.sharedGuard(); g.err != nil {
		return false, failf("checking %s against %s: %w", node, shape.ID, g.err)
	}
	return ok, nil
}

// ---------- FilterShapeExpr ----------

// FilterShapeExpr filters input nodes by conformance to a shape.
type FilterShapeExpr struct {
	ShapeRef Term
	Nodes    NodeExpr
}

func (e *FilterShapeExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *FilterShapeExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	nodes, err := evalNodeExpr(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	shape := ctx.resolveShape(e.ShapeRef)
	if shape == nil {
		return nodes, nil // no constraints = all pass
	}
	var result []Term
	for _, n := range nodes {
		ok, err := ctx.conforms(shape, n)
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, n)
		}
	}
	return result, nil
}

// ---------- FindFirstExpr ----------

// FindFirstExpr returns the first input node that conforms to a shape.
type FindFirstExpr struct {
	ShapeRef Term
	Nodes    NodeExpr // nil: the focus node
}

func (e *FindFirstExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *FindFirstExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	nodes, err := nodesOrFocus(ctx, e.Nodes)
	if err != nil || len(nodes) == 0 {
		return nil, err
	}
	shape := ctx.resolveShape(e.ShapeRef)
	if shape == nil {
		return nodes[:1], nil // no constraints = the first node passes
	}
	for _, n := range nodes {
		ok, err := ctx.conforms(shape, n)
		if err != nil {
			return nil, err
		}
		if ok {
			return []Term{n}, nil
		}
	}
	return nil, nil
}

// ---------- MatchAllExpr ----------

// MatchAllExpr returns true if all input nodes conform to a shape, false otherwise.
type MatchAllExpr struct {
	ShapeRef Term
	Nodes    NodeExpr // nil: the focus node
}

func (e *MatchAllExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *MatchAllExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	nodes, err := nodesOrFocus(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	shape := ctx.resolveShape(e.ShapeRef)
	if shape == nil {
		return []Term{boolTerm(true)}, nil
	}
	for _, n := range nodes {
		ok, err := ctx.conforms(shape, n)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Term{boolTerm(false)}, nil
		}
	}
	return []Term{boolTerm(true)}, nil
}

// ---------- ConformsToShapeExpr ----------

// ConformsToShapeExpr is [ shnex:conformsToShape ( node shape ) ]: a list
// parameter function whose output is whether the node conforms to the shape.
type ConformsToShapeExpr struct {
	Args []NodeExpr
}

func (e *ConformsToShapeExpr) Eval(ctx *nodeExprContext) []Term {
	return dropFailure(e.Evaluate(ctx))
}

func (e *ConformsToShapeExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	if len(e.Args) != 2 {
		return nil, failf("shnex:conformsToShape takes a node and a shape, not %d arguments", len(e.Args))
	}
	args := evalListArguments(ctx, e.Args)
	if err := firstArgumentFailure(args); err != nil {
		return nil, err
	}
	if !allBound(args) {
		return nil, nil
	}
	node, shapeRef := args[0].node, args[1].node
	shape := ctx.resolveShape(shapeRef)
	if shape == nil {
		return []Term{boolTerm(true)}, nil
	}
	ok, err := ctx.conforms(shape, node)
	if err != nil {
		return nil, err
	}
	return []Term{boolTerm(ok)}, nil
}

// parseAdHocShape parses constraints from an RDF node that may define
// inline shape constraints (like sh:minInclusive, sh:datatype, etc.)
// without being declared as a shape. Used by filterShape, findFirst, matchAll.
func parseAdHocShape(g *Graph, node Term, shapesMap map[string]*Shape) *Shape {
	if s, ok := shapesMap[node.String()]; ok {
		return s
	}
	s := &Shape{ID: node}
	s.Constraints = parseConstraints(g, s, shapesMap)
	s.Properties = parsePropertyShapes(g, s.ID, shapesMap)
	if len(s.Constraints) == 0 && len(s.Properties) == 0 {
		return nil
	}
	shapesMap[node.String()] = s
	return s
}

// parsePropertyShapes parses sh:property shapes for ad-hoc shapes.
func parsePropertyShapes(g *Graph, shapeID Term, shapesMap map[string]*Shape) []*Shape {
	propPred := IRI(SH + "property")
	props := g.Objects(shapeID, propPred)
	var result []*Shape
	for _, p := range props {
		ps := &Shape{
			ID:         p,
			IsProperty: true,
		}
		pathVals := g.Objects(p, IRI(SH+"path"))
		if len(pathVals) > 0 {
			ps.Path = parsePath(g, pathVals[0])
		}
		ps.Constraints = parseConstraints(g, ps, shapesMap)
		if len(ps.Constraints) > 0 || ps.Path != nil {
			result = append(result, ps)
		}
	}
	return result
}
