package shacl

import (
	"iter"
	"slices"
)

// Shapes enumerates detached descriptions in the engine's shape order.
// It includes untargeted and deactivated shapes; Targets selects active focuses.
func (p *Prepared) Shapes() iter.Seq[ShapeInfo] {
	return func(yield func(ShapeInfo) bool) {
		for _, s := range shapesInOrder(p.ctx.shapesMap) {
			if !yield(shapeInfo(s)) {
				return
			}
		}
	}
}

// Shape looks up a parsed shape without exposing its mutable representation.
// Named and anonymous shapes use the same lookup and retain their RDF identity.
func (p *Prepared) Shape(id Term) (ShapeInfo, bool) {
	s, ok := p.ctx.shapesMap[id.String()]
	if !ok {
		return ShapeInfo{}, false
	}
	return shapeInfo(s), true
}

// Targets returns an independent slice of the active shape's focus nodes.
// Selection is cached on first use and shared with Validate, including empty sets.
func (p *Prepared) Targets(shape Term) []Term {
	s := p.ctx.shapesMap[shape.String()]
	if s == nil || s.Deactivated {
		return nil
	}
	return slices.Clone(p.targetNodes(p.inspector(), s))
}

func (p *Prepared) targetNodes(ctx *evalContext, s *Shape) []Term {
	if targets, ok := p.targets[s]; ok {
		return targets
	}
	if p.targets == nil {
		p.targets = make(map[*Shape][]Term, len(p.ctx.shapesMap))
	}
	targets := resolveTargets(ctx, s)
	p.targets[s] = targets
	return targets
}

// ValueNodes selects values with the same path or sh:values operation as validation.
// Node shapes select the focus itself. Returned slices do not alias engine state.
func (p *Prepared) ValueNodes(shape, focus Term) []Term {
	s := p.ctx.shapesMap[shape.String()]
	if s == nil {
		return nil
	}
	if !s.IsProperty {
		return []Term{focus}
	}
	return slices.Clone(propertyValueNodes(p.inspector(), s, focus))
}

// PathValues evaluates a SHACL path against the prepared data using the engine.
// Complex path definitions are read from the shapes graph; the result is detached.
func (p *Prepared) PathValues(path, focus Term) []Term {
	return slices.Clone(evalPath(p.ctx.dataGraph, parsePath(p.ctx.shapesGraph, path), focus))
}

// ShapeObjects reads parameter and path-definition values from the shapes graph.
// The returned slice can be retained or modified without changing the source graph.
func (p *Prepared) ShapeObjects(subject, predicate Term) []Term {
	return slices.Clone(p.ctx.shapesGraph.Objects(subject, predicate))
}

// ShapeLinks enumerates parsed sh:property, sh:node, sh:and, sh:or, sh:xone,
// and sh:not references without evaluating constraints. Other shape-valued
// parameters, rule conditions, functions, and target definitions are not followed.
func (p *Prepared) ShapeLinks(shape Term) iter.Seq2[Term, Term] {
	return func(yield func(Term, Term) bool) {
		s := p.ctx.shapesMap[shape.String()]
		if s == nil {
			return
		}
		for _, property := range s.Properties {
			if !yield(IRI(SH+"property"), property.ID) {
				return
			}
		}
		for _, c := range s.Constraints {
			if !constraintLinks(c, yield) {
				return
			}
		}
	}
}

func constraintLinks(c Constraint, yield func(Term, Term) bool) bool {
	emit := func(via string, refs ...Term) bool {
		for _, ref := range refs {
			if !yield(IRI(SH+via), ref) {
				return false
			}
		}
		return true
	}
	switch c := c.(type) {
	case *NodeConstraint:
		return emit("node", c.ShapeRef)
	case *PropertyConstraint:
		return emit("property", c.ShapeRef)
	case *AndConstraint:
		return emit("and", c.Shapes...)
	case *OrConstraint:
		return emit("or", c.Shapes...)
	case *XoneConstraint:
		return emit("xone", c.Shapes...)
	case *NotConstraint:
		return emit("not", c.ShapeRef)
	case *severityOverrideConstraint:
		return constraintLinks(c.inner, yield)
	default:
		return true
	}
}
