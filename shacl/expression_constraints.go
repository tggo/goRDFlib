package shacl

// ExpressionConstraint implements sh:expression.
//
// SHACL 1.2 node expressions: the expression is evaluated once for each value
// node, with the shape's focus node as the focus node and the value node
// bound to the scope variable "value"; the value node violates the constraint
// unless the output nodes are exactly ( true ). An evaluation failure fails
// the validation (see recursionGuard.fail).
//
// A SHACL-AF expression keeps its AF reading: it is evaluated once per focus
// node and conforms when its result is truthy.
type ExpressionConstraint struct {
	ExprNode Term // RDF node defining the expression
}

func (c *ExpressionConstraint) ComponentIRI() string {
	return SH + "ExpressionConstraintComponent"
}

func (c *ExpressionConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	expr := parseExpressionFor(ctx, c.ExprNode)
	nCtx := &nodeExprContext{
		dataGraph:   ctx.dataGraph,
		shapesGraph: ctx.shapesGraph,
		shapesMap:   ctx.shapesMap,
		focusNode:   focusNode,
		af:          ctx.af,
		guard:       ctx.sharedGuard(),
	}
	if isAFExpr(expr) {
		if isTruthy(expr.Eval(nCtx)) {
			return nil
		}
		r := makeResult(shape, focusNode, focusNode, c.ComponentIRI())
		r.SourceConstraint = c.ExprNode
		return []ValidationResult{r}
	}

	var results []ValidationResult
	for _, v := range valueNodes {
		nCtx.vars = map[string]Term{"value": v}
		out, err := evalNodeExpr(nCtx, expr)
		if err != nil {
			ctx.expressionFailed(err)
			return results
		}
		if isTrueOnly(out) {
			continue
		}
		r := makeResult(shape, focusNode, v, c.ComponentIRI())
		r.SourceConstraint = c.ExprNode
		results = append(results, r)
		if ctx.sharedGuard().quick {
			return results
		}
	}
	return results
}

// expressionFailed records the failure of a node expression of a constraint:
// the run returns it instead of a report, and the error handler sees it.
func (ctx *evalContext) expressionFailed(err error) {
	ctx.sharedGuard().fail(err)
	ctx.report(err)
}

// isAFExpr reports whether e is written in SHACL-AF vocabulary.
func isAFExpr(e NodeExpr) bool {
	switch e.(type) {
	case *afThisExpr, *afUnionExpr, *afIntersectionExpr, *afPathExpr, *afFilterShapeExpr, *afFunctionExpr:
		return true
	}
	return false
}

// NodeByExpressionConstraint implements sh:nodeByExpression.
//
// Each value node is the focus node of the expression, evaluated with an
// empty scope; the output nodes are the shapes the value node must conform
// to. A violation names the shape it failed as its sh:sourceConstraint. This
// is sh:node with the shape computed per value node.
type NodeByExpressionConstraint struct {
	ShapeRef Term // the node expression (an IRI is the expression for itself, so a plain shape reference still works)
}

func (c *NodeByExpressionConstraint) ComponentIRI() string {
	return SH + "NodeByExpressionConstraintComponent"
}

func (c *NodeByExpressionConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	expr := parseNodeExpr(ctx.shapesGraph, c.ShapeRef)
	nCtx := &nodeExprContext{
		dataGraph:   ctx.dataGraph,
		shapesGraph: ctx.shapesGraph,
		shapesMap:   ctx.shapesMap,
		af:          ctx.af,
		guard:       ctx.sharedGuard(),
	}
	var results []ValidationResult
	for _, vn := range valueNodes {
		out, err := evalNodeExpr(nCtx.at(vn), expr)
		if err != nil {
			ctx.expressionFailed(err)
			return results
		}
		for _, ref := range out {
			s := nCtx.resolveShape(ref)
			if s == nil {
				continue // a node with no constraints is a shape every node conforms to
			}
			if len(validateNodeAgainstShape(ctx, s, vn)) == 0 {
				continue
			}
			if err := ctx.sharedGuard().err; err != nil {
				ctx.expressionFailed(err)
				return results
			}
			r := makeResult(shape, focusNode, vn, c.ComponentIRI())
			r.SourceConstraint = ref
			results = append(results, r)
		}
	}
	return results
}
