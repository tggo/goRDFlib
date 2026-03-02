package shacl

// NodeConstraint implements sh:node.
type NodeConstraint struct {
	ShapeRef Term
}

func (c *NodeConstraint) ComponentIRI() string {
	return SH + "NodeConstraintComponent"
}

func (c *NodeConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	s, ok := ctx.shapesMap[c.ShapeRef.String()]
	if !ok {
		return nil
	}
	var results []ValidationResult
	for _, vn := range valueNodes {
		if len(validateNodeAgainstShape(ctx, s, vn)) > 0 {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// PropertyConstraint implements sh:property.
type PropertyConstraint struct {
	ShapeRef Term
}

func (c *PropertyConstraint) ComponentIRI() string {
	return SH + "PropertyConstraintComponent"
}

func (c *PropertyConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	s, ok := ctx.shapesMap[c.ShapeRef.String()]
	if !ok {
		return nil
	}
	return validatePropertyShape(ctx, s, focusNode)
}

// QualifiedValueShapeConstraint implements sh:qualifiedValueShape with
// sh:qualifiedMinCount, sh:qualifiedMaxCount, and sh:qualifiedValueShapesDisjoint.
type QualifiedValueShapeConstraint struct {
	ShapeRef                     Term
	QualifiedMinCount            int
	QualifiedMaxCount            int
	QualifiedValueShapesDisjoint bool
	SiblingShapes                []Term
}

func (c *QualifiedValueShapeConstraint) ComponentIRI() string {
	return SH + "QualifiedMinCountConstraintComponent"
}

func (c *QualifiedValueShapeConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	s, ok := ctx.shapesMap[c.ShapeRef.String()]
	if !ok {
		return nil
	}
	conformCount := 0
	for _, vn := range valueNodes {
		if len(validateNodeAgainstShape(ctx, s, vn)) > 0 {
			continue
		}
		if c.QualifiedValueShapesDisjoint {
			disjoint := true
			for _, sibRef := range c.SiblingShapes {
				sib, ok := ctx.shapesMap[sibRef.String()]
				if !ok {
					continue
				}
				if len(validateNodeAgainstShape(ctx, sib, vn)) == 0 {
					disjoint = false
					break
				}
			}
			if disjoint {
				conformCount++
			}
		} else {
			conformCount++
		}
	}

	var results []ValidationResult
	if c.QualifiedMinCount > 0 && conformCount < c.QualifiedMinCount {
		results = append(results, makeResult(shape, focusNode, Term{}, SH+"QualifiedMinCountConstraintComponent"))
	}
	if c.QualifiedMaxCount >= 0 && conformCount > c.QualifiedMaxCount {
		results = append(results, makeResult(shape, focusNode, Term{}, SH+"QualifiedMaxCountConstraintComponent"))
	}
	return results
}
