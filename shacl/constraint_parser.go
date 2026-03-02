package shacl

func parseConstraints(g *Graph, s *Shape, shapes map[string]*Shape) []Constraint {
	var result []Constraint
	id := s.ID

	for _, v := range g.Objects(id, IRI(SH+"class")) {
		result = append(result, &ClassConstraint{Class: v})
	}

	for _, v := range g.Objects(id, IRI(SH+"datatype")) {
		result = append(result, &DatatypeConstraint{Datatype: v})
	}

	for _, v := range g.Objects(id, IRI(SH+"nodeKind")) {
		result = append(result, &NodeKindConstraint{NodeKind: v})
	}

	for _, v := range g.Objects(id, IRI(SH+"minCount")) {
		result = append(result, &MinCountConstraint{MinCount: parseInt(v)})
	}

	for _, v := range g.Objects(id, IRI(SH+"maxCount")) {
		result = append(result, &MaxCountConstraint{MaxCount: parseInt(v)})
	}

	for _, v := range g.Objects(id, IRI(SH+"minExclusive")) {
		result = append(result, &MinExclusiveConstraint{Value: v})
	}
	for _, v := range g.Objects(id, IRI(SH+"minInclusive")) {
		result = append(result, &MinInclusiveConstraint{Value: v})
	}
	for _, v := range g.Objects(id, IRI(SH+"maxExclusive")) {
		result = append(result, &MaxExclusiveConstraint{Value: v})
	}
	for _, v := range g.Objects(id, IRI(SH+"maxInclusive")) {
		result = append(result, &MaxInclusiveConstraint{Value: v})
	}

	for _, v := range g.Objects(id, IRI(SH+"minLength")) {
		result = append(result, &MinLengthConstraint{MinLength: parseInt(v)})
	}
	for _, v := range g.Objects(id, IRI(SH+"maxLength")) {
		result = append(result, &MaxLengthConstraint{MaxLength: parseInt(v)})
	}

	for _, v := range g.Objects(id, IRI(SH+"pattern")) {
		flags := ""
		if fVals := g.Objects(id, IRI(SH+"flags")); len(fVals) > 0 {
			flags = fVals[0].Value()
		}
		if c := NewPatternConstraint(v.Value(), flags); c != nil {
			result = append(result, c)
		}
	}

	for _, v := range g.Objects(id, IRI(SH+"languageIn")) {
		items := g.RDFList(v)
		langs := make([]string, len(items))
		for i, item := range items {
			langs[i] = item.Value()
		}
		result = append(result, &LanguageInConstraint{Languages: langs})
	}

	for _, v := range g.Objects(id, IRI(SH+"uniqueLang")) {
		if v.Value() == "true" {
			result = append(result, &UniqueLangConstraint{UniqueLang: true})
		}
	}

	for _, v := range g.Objects(id, IRI(SH+"equals")) {
		result = append(result, &EqualsConstraint{Path: v})
	}
	for _, v := range g.Objects(id, IRI(SH+"disjoint")) {
		result = append(result, &DisjointConstraint{Path: v})
	}
	for _, v := range g.Objects(id, IRI(SH+"lessThan")) {
		result = append(result, &LessThanConstraint{Path: v})
	}
	for _, v := range g.Objects(id, IRI(SH+"lessThanOrEquals")) {
		result = append(result, &LessThanOrEqualsConstraint{Path: v})
	}

	for _, v := range g.Objects(id, IRI(SH+"and")) {
		result = append(result, &AndConstraint{Shapes: g.RDFList(v)})
	}
	for _, v := range g.Objects(id, IRI(SH+"or")) {
		result = append(result, &OrConstraint{Shapes: g.RDFList(v)})
	}
	for _, v := range g.Objects(id, IRI(SH+"not")) {
		result = append(result, &NotConstraint{ShapeRef: v})
	}
	for _, v := range g.Objects(id, IRI(SH+"xone")) {
		result = append(result, &XoneConstraint{Shapes: g.RDFList(v)})
	}

	for _, v := range g.Objects(id, IRI(SH+"node")) {
		result = append(result, &NodeConstraint{ShapeRef: v})
	}

	if qvs := g.Objects(id, IRI(SH+"qualifiedValueShape")); len(qvs) > 0 {
		minCount := 0
		maxCount := -1
		disjoint := false
		if mc := g.Objects(id, IRI(SH+"qualifiedMinCount")); len(mc) > 0 {
			minCount = parseInt(mc[0])
		}
		if mc := g.Objects(id, IRI(SH+"qualifiedMaxCount")); len(mc) > 0 {
			maxCount = parseInt(mc[0])
		}
		if d := g.Objects(id, IRI(SH+"qualifiedValueShapesDisjoint")); len(d) > 0 {
			disjoint = d[0].Value() == "true"
		}

		var siblingShapes []Term
		if disjoint {
			propPred := IRI(SH + "property")
			for _, parent := range g.Subjects(propPred, id) {
				for _, sibling := range g.Objects(parent, propPred) {
					if sibling.Equal(id) {
						continue
					}
					for _, sibQvs := range g.Objects(sibling, IRI(SH+"qualifiedValueShape")) {
						siblingShapes = append(siblingShapes, sibQvs)
					}
				}
			}
		}

		for _, qv := range qvs {
			result = append(result, &QualifiedValueShapeConstraint{
				ShapeRef:                     qv,
				QualifiedMinCount:            minCount,
				QualifiedMaxCount:            maxCount,
				QualifiedValueShapesDisjoint: disjoint,
				SiblingShapes:                siblingShapes,
			})
		}
	}

	for _, v := range g.Objects(id, IRI(SH+"hasValue")) {
		result = append(result, &HasValueConstraint{Value: v})
	}

	for _, v := range g.Objects(id, IRI(SH+"in")) {
		result = append(result, &InConstraint{Values: g.RDFList(v)})
	}

	if s.Closed {
		var allowed []Term
		for _, ps := range s.Properties {
			if ps.Path != nil && ps.Path.Kind == PathPredicate {
				allowed = append(allowed, ps.Path.Pred)
			}
		}
		result = append(result, &ClosedConstraint{
			AllowedProperties: allowed,
			IgnoredProperties: s.IgnoredProperties,
		})
	}

	return result
}
