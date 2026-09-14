package shacl

// ExpressionConstraint implements sh:expression.
// The expression is a SHACL node expression. A node conforms if the expression
// evaluates to a truthy value (non-empty, not false).
type ExpressionConstraint struct {
	ExprNode Term // RDF node defining the expression
}

func (c *ExpressionConstraint) ComponentIRI() string {
	return SH + "ExpressionConstraintComponent"
}

func (c *ExpressionConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	expr := parseExpressionFor(ctx, c.ExprNode)
	nCtx := &nodeExprContext{
		dataGraph:      ctx.dataGraph,
		shapesGraph:    ctx.shapesGraph,
		shapesMap:      ctx.shapesMap,
		classInstances: ctx.classInstances,
		focusNode:      focusNode,
		af:             ctx.af,
		guard:          ctx.sharedGuard(),
	}
	result := expr.Eval(nCtx)
	if isTruthy(result) {
		return nil
	}
	r := makeResult(shape, focusNode, focusNode, c.ComponentIRI())
	r.SourceConstraint = c.ExprNode
	return []ValidationResult{r}
}

// NodeByExpressionConstraint implements sh:nodeByExpression.
// Each value node must conform to the referenced shape.
// This is like sh:node but is specifically for the node expression constraint component.
type NodeByExpressionConstraint struct {
	ShapeRef Term
}

func (c *NodeByExpressionConstraint) ComponentIRI() string {
	return SH + "NodeByExpressionConstraintComponent"
}

func (c *NodeByExpressionConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	s, ok := ctx.shapesMap[c.ShapeRef.String()]
	if !ok {
		return nil
	}
	var results []ValidationResult
	for _, vn := range valueNodes {
		vr := validateNodeAgainstShape(ctx, s, vn)
		if len(vr) > 0 {
			r := makeResult(shape, focusNode, vn, c.ComponentIRI())
			r.SourceConstraint = c.ShapeRef
			results = append(results, r)
		}
	}
	return results
}
