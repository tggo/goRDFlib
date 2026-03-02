package shacl

// HasValueConstraint implements sh:hasValue.
type HasValueConstraint struct {
	Value Term
}

func (c *HasValueConstraint) ComponentIRI() string {
	return SH + "HasValueConstraintComponent"
}

func (c *HasValueConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	for _, vn := range valueNodes {
		if vn.Equal(c.Value) {
			return nil
		}
	}
	return []ValidationResult{makeResult(shape, focusNode, Term{}, c.ComponentIRI())}
}

// InConstraint implements sh:in.
type InConstraint struct {
	Values []Term
}

func (c *InConstraint) ComponentIRI() string {
	return SH + "InConstraintComponent"
}

func (c *InConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		found := false
		for _, allowed := range c.Values {
			if vn.Equal(allowed) {
				found = true
				break
			}
		}
		if !found {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// ClosedConstraint implements sh:closed.
type ClosedConstraint struct {
	AllowedProperties []Term
	IgnoredProperties []Term
}

func (c *ClosedConstraint) ComponentIRI() string {
	return SH + "ClosedConstraintComponent"
}

func (c *ClosedConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	triples := ctx.dataGraph.All(&focusNode, nil, nil)
	for _, t := range triples {
		if c.isAllowed(t.Predicate) {
			continue
		}
		r := makeResult(shape, focusNode, t.Object, c.ComponentIRI())
		r.ResultPath = t.Predicate
		results = append(results, r)
	}
	return results
}

func (c *ClosedConstraint) isAllowed(pred Term) bool {
	for _, a := range c.AllowedProperties {
		if pred.Equal(a) {
			return true
		}
	}
	for _, ig := range c.IgnoredProperties {
		if pred.Equal(ig) {
			return true
		}
	}
	return false
}
