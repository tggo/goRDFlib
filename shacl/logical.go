package shacl

// AndConstraint implements sh:and.
type AndConstraint struct {
	Shapes []Term
}

func (c *AndConstraint) ComponentIRI() string {
	return SH + "AndConstraintComponent"
}

func (c *AndConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		for _, sRef := range c.Shapes {
			s, ok := ctx.shapesMap[sRef.String()]
			if !ok {
				continue
			}
			if len(validateNodeAgainstShape(ctx, s, vn)) > 0 {
				results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
				break
			}
		}
	}
	return results
}

// OrConstraint implements sh:or.
type OrConstraint struct {
	Shapes []Term
}

func (c *OrConstraint) ComponentIRI() string {
	return SH + "OrConstraintComponent"
}

func (c *OrConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		anyConforms := false
		for _, sRef := range c.Shapes {
			s, ok := ctx.shapesMap[sRef.String()]
			if !ok {
				continue
			}
			if len(validateNodeAgainstShape(ctx, s, vn)) == 0 {
				anyConforms = true
				break
			}
		}
		if !anyConforms {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// NotConstraint implements sh:not.
type NotConstraint struct {
	ShapeRef Term
}

func (c *NotConstraint) ComponentIRI() string {
	return SH + "NotConstraintComponent"
}

func (c *NotConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	s, ok := ctx.shapesMap[c.ShapeRef.String()]
	if !ok {
		return nil
	}
	var results []ValidationResult
	for _, vn := range valueNodes {
		if len(validateNodeAgainstShape(ctx, s, vn)) == 0 {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// XoneConstraint implements sh:xone.
type XoneConstraint struct {
	Shapes []Term
}

func (c *XoneConstraint) ComponentIRI() string {
	return SH + "XoneConstraintComponent"
}

func (c *XoneConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		count := 0
		for _, sRef := range c.Shapes {
			s, ok := ctx.shapesMap[sRef.String()]
			if !ok {
				continue
			}
			if len(validateNodeAgainstShape(ctx, s, vn)) == 0 {
				count++
			}
		}
		if count != 1 {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}
