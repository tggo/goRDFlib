package shacl

// EqualsConstraint implements sh:equals.
type EqualsConstraint struct{ Path Term }

// DisjointConstraint implements sh:disjoint.
type DisjointConstraint struct{ Path Term }

// LessThanConstraint implements sh:lessThan.
type LessThanConstraint struct{ Path Term }

// LessThanOrEqualsConstraint implements sh:lessThanOrEquals.
type LessThanOrEqualsConstraint struct{ Path Term }

func (c *EqualsConstraint) ComponentIRI() string {
	return SH + "EqualsConstraintComponent"
}
func (c *DisjointConstraint) ComponentIRI() string {
	return SH + "DisjointConstraintComponent"
}
func (c *LessThanConstraint) ComponentIRI() string {
	return SH + "LessThanConstraintComponent"
}
func (c *LessThanOrEqualsConstraint) ComponentIRI() string {
	return SH + "LessThanOrEqualsConstraintComponent"
}

func (c *EqualsConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	otherValues := ctx.dataGraph.Objects(focusNode, c.Path)
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !containsTerm(otherValues, vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	for _, ov := range otherValues {
		if !containsTerm(valueNodes, ov) {
			results = append(results, makeResult(shape, focusNode, ov, c.ComponentIRI()))
		}
	}
	return results
}

func (c *DisjointConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	otherValues := ctx.dataGraph.Objects(focusNode, c.Path)
	var results []ValidationResult
	for _, vn := range valueNodes {
		if containsTerm(otherValues, vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

func (c *LessThanConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	return evalPairComparison(ctx, shape, focusNode, valueNodes, c.Path, c.ComponentIRI(), false)
}

func (c *LessThanOrEqualsConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	return evalPairComparison(ctx, shape, focusNode, valueNodes, c.Path, c.ComponentIRI(), true)
}

// evalPairComparison is shared logic for sh:lessThan and sh:lessThanOrEquals.
// When allowEqual is false, value must be strictly less than other (lessThan).
// When allowEqual is true, value may be equal (lessThanOrEquals).
func evalPairComparison(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term, path Term, component string, allowEqual bool) []ValidationResult {
	otherValues := ctx.dataGraph.Objects(focusNode, path)
	var results []ValidationResult
	for _, vn := range valueNodes {
		for _, ov := range otherValues {
			cmp, ok := compareLiterals(vn, ov)
			if !ok {
				if vn.IsIRI() && ov.IsIRI() {
					if (!allowEqual && vn.Value() >= ov.Value()) || (allowEqual && vn.Value() > ov.Value()) {
						results = append(results, makeResult(shape, focusNode, vn, component))
					}
					continue
				}
				results = append(results, makeResult(shape, focusNode, vn, component))
			} else if (!allowEqual && cmp >= 0) || (allowEqual && cmp > 0) {
				results = append(results, makeResult(shape, focusNode, vn, component))
			}
		}
	}
	return results
}

func containsTerm(terms []Term, t Term) bool {
	for _, term := range terms {
		if term.Equal(t) {
			return true
		}
	}
	return false
}
