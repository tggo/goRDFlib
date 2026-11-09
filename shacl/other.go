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
	// SHACL 1.2 Core: closed applies to each value node (for a node shape that
	// is the focus node), and the result keeps the focus node of the run.
	var results []ValidationResult
	for _, vn := range valueNodes {
		if vn.IsLiteral() {
			continue
		}
		for _, t := range ctx.dataGraph.All(&vn, nil, nil) {
			if c.isAllowed(t.Predicate) {
				continue
			}
			r := makeResult(shape, focusNode, t.Object, c.ComponentIRI())
			r.ResultPath = t.Predicate
			results = append(results, r)
		}
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

// ClosedByTypesConstraint implements sh:closed sh:ByTypes (SHACL 1.2).
// Allowed properties are determined dynamically based on the focus node's types
// and the shape hierarchy. Properties from the declaring shape and all shapes
// that are superclasses of the focus node's types are allowed.
type ClosedByTypesConstraint struct {
	Shape *Shape
}

func (c *ClosedByTypesConstraint) ComponentIRI() string {
	return SH + "ClosedConstraintComponent"
}

func (c *ClosedByTypesConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !vn.IsLiteral() {
			results = append(results, c.evaluateNode(ctx, shape, focusNode, vn)...)
		}
	}
	return results
}

// evaluateNode checks one value node (SHACL 1.2 Core: sh:closed applies to
// each value node).
func (c *ClosedByTypesConstraint) evaluateNode(ctx *evalContext, shape *Shape, focusNode, vn Term) []ValidationResult {
	// Collect allowed properties from this shape and all type-compatible shapes.
	allowed := make(map[string]bool)

	// collectProps adds all sh:property paths from a shape (and sh:node refs transitively).
	visited := make(map[string]bool)
	var collectProps func(s *Shape)
	collectProps = func(s *Shape) {
		key := s.ID.String()
		if visited[key] {
			return
		}
		visited[key] = true
		for _, ps := range s.Properties {
			if ps.Path != nil && ps.Path.Kind == PathPredicate {
				allowed[ps.Path.Pred.TermKey()] = true
			}
		}
		// Follow sh:node references
		for _, c := range s.Constraints {
			if nc, ok := c.(*NodeConstraint); ok {
				if ref, ok := ctx.shapesMap[nc.ShapeRef.String()]; ok {
					collectProps(ref)
				}
			}
		}
	}

	// Properties from the declaring shape
	collectProps(shape)

	// Get the focus node's types and their superclasses
	typePred := IRI(RDFType)
	types := ctx.dataGraph.Objects(vn, typePred)
	allTypes := make(map[string]bool)
	for _, t := range types {
		allTypes[t.TermKey()] = true
		for _, super := range superClasses(ctx.shapesGraph, t) {
			allTypes[super.TermKey()] = true
		}
	}

	// Collect properties from shapes whose ID matches a type or that target a type
	for _, s := range ctx.shapesMap {
		if allTypes[s.ID.TermKey()] {
			collectProps(s)
		}
		for _, tgt := range s.Targets {
			if (tgt.Kind == TargetClass || tgt.Kind == TargetImplicitClass) && allTypes[tgt.Value.TermKey()] {
				collectProps(s)
				break
			}
		}
	}

	// Always allow rdf:type
	allowed[IRI(RDFType).TermKey()] = true

	for _, ig := range shape.IgnoredProperties {
		allowed[ig.TermKey()] = true
	}

	var results []ValidationResult
	triples := ctx.dataGraph.All(&vn, nil, nil)
	for _, t := range triples {
		if !allowed[t.Predicate.TermKey()] {
			r := makeResult(shape, focusNode, t.Object, c.ComponentIRI())
			r.ResultPath = t.Predicate
			results = append(results, r)
		}
	}
	return results
}

// superClasses returns all superclasses of a class (transitive via rdfs:subClassOf).
func superClasses(g *Graph, class Term) []Term {
	subClassPred := IRI(RDFSSubClassOf)
	visited := map[string]bool{class.TermKey(): true}
	queue := []Term{class}
	var result []Term
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, parent := range g.Objects(cur, subClassPred) {
			k := parent.TermKey()
			if !visited[k] {
				visited[k] = true
				result = append(result, parent)
				queue = append(queue, parent)
			}
		}
	}
	return result
}
