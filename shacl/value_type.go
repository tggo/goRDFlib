package shacl

// ClassConstraint implements sh:class.
type ClassConstraint struct {
	Class Term
}

func (c *ClassConstraint) ComponentIRI() string {
	return SH + "ClassConstraintComponent"
}

func (c *ClassConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !ctx.dataGraph.HasType(vn, c.Class) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// DatatypeConstraint implements sh:datatype.
type DatatypeConstraint struct {
	Datatype Term
}

func (c *DatatypeConstraint) ComponentIRI() string {
	return SH + "DatatypeConstraintComponent"
}

func (c *DatatypeConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	dt := c.Datatype.Value()
	for _, vn := range valueNodes {
		if !vn.IsLiteral() || vn.Datatype() != dt {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		} else if !isWellFormedLiteral(vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// NodeKindConstraint implements sh:nodeKind.
type NodeKindConstraint struct {
	NodeKind Term
}

func (c *NodeKindConstraint) ComponentIRI() string {
	return SH + "NodeKindConstraintComponent"
}

func (c *NodeKindConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	nk := c.NodeKind.Value()
	for _, vn := range valueNodes {
		if !matchesNodeKind(vn, nk) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

func matchesNodeKind(t Term, nk string) bool {
	switch nk {
	case SH + "IRI":
		return t.IsIRI()
	case SH + "BlankNode":
		return t.IsBlank()
	case SH + "Literal":
		return t.IsLiteral()
	case SH + "BlankNodeOrIRI":
		return t.IsBlank() || t.IsIRI()
	case SH + "BlankNodeOrLiteral":
		return t.IsBlank() || t.IsLiteral()
	case SH + "IRIOrLiteral":
		return t.IsIRI() || t.IsLiteral()
	}
	return false
}
