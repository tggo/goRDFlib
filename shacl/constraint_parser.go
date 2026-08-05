package shacl

func parseConstraints(g *Graph, s *Shape, shapes map[string]*Shape) []Constraint {
	var result []Constraint
	id := s.ID

	// sh:class — SHACL 1.2: may be a list of classes (OR semantics)
	for _, v := range g.Objects(id, IRI(SH+"class")) {
		firstPred := IRI(RDFFirst)
		if g.Has(&v, &firstPred, nil) {
			items := g.RDFList(v)
			result = append(result, &ClassListConstraint{Classes: items})
		} else {
			result = append(result, &ClassConstraint{Class: v})
		}
	}

	// sh:datatype — SHACL 1.2: may be a list of datatypes (OR semantics)
	for _, v := range g.Objects(id, IRI(SH+"datatype")) {
		firstPred := IRI(RDFFirst)
		if g.Has(&v, &firstPred, nil) {
			items := g.RDFList(v)
			result = append(result, &DatatypeListConstraint{Datatypes: items})
		} else {
			result = append(result, &DatatypeConstraint{Datatype: v})
		}
	}

	// sh:nodeKind — SHACL 1.2: may be a list of nodeKinds (OR semantics)
	for _, v := range g.Objects(id, IRI(SH+"nodeKind")) {
		firstPred := IRI(RDFFirst)
		if g.Has(&v, &firstPred, nil) {
			items := g.RDFList(v)
			result = append(result, &NodeKindListConstraint{NodeKinds: items})
		} else {
			result = append(result, &NodeKindConstraint{NodeKind: v})
		}
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

	// sh:equals, sh:disjoint, sh:lessThan, sh:lessThanOrEquals
	// SHACL 1.2: value may be a property path (list = sequence path)
	for _, v := range g.Objects(id, IRI(SH+"equals")) {
		result = append(result, makePairConstraint("equals", g, v))
	}
	for _, v := range g.Objects(id, IRI(SH+"disjoint")) {
		result = append(result, makePairConstraint("disjoint", g, v))
	}
	for _, v := range g.Objects(id, IRI(SH+"lessThan")) {
		result = append(result, makePairConstraint("lessThan", g, v))
	}
	for _, v := range g.Objects(id, IRI(SH+"lessThanOrEquals")) {
		result = append(result, makePairConstraint("lessThanOrEquals", g, v))
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

	// SHACL 1.2: new constraints
	for _, v := range g.Objects(id, IRI(SH+"singleLine")) {
		if v.Value() == "true" {
			result = append(result, &SingleLineConstraint{SingleLine: true})
		}
	}

	for _, v := range g.Objects(id, IRI(SH+"someValue")) {
		result = append(result, &SomeValueConstraint{ShapeRef: v})
	}

	for _, v := range g.Objects(id, IRI(SH+"subsetOf")) {
		result = append(result, &SubsetOfConstraint{OtherPath: parsePath(g, v)})
	}

	for _, v := range g.Objects(id, IRI(SH+"uniqueMembers")) {
		if v.Value() == "true" {
			result = append(result, &UniqueMembersConstraint{UniqueMembers: true})
		}
	}

	for _, v := range g.Objects(id, IRI(SH+"memberShape")) {
		result = append(result, &MemberShapeConstraint{ShapeRef: v})
	}

	for _, v := range g.Objects(id, IRI(SH+"minListLength")) {
		result = append(result, &MinListLengthConstraint{MinLength: parseInt(v)})
	}
	for _, v := range g.Objects(id, IRI(SH+"maxListLength")) {
		result = append(result, &MaxListLengthConstraint{MaxLength: parseInt(v)})
	}

	for _, v := range g.Objects(id, IRI(SH+"reifierShape")) {
		reifReq := false
		if rr := g.Objects(id, IRI(SH+"reificationRequired")); len(rr) > 0 {
			reifReq = rr[0].Value() == "true"
		}
		result = append(result, &ReifierShapeConstraint{ShapeRef: v, ReificationRequired: reifReq})
	}

	// sh:expression (SHACL 1.2 Node Expressions)
	for _, v := range g.Objects(id, IRI(SH+"expression")) {
		result = append(result, &ExpressionConstraint{ExprNode: v})
	}

	// sh:nodeByExpression (SHACL 1.2 Node Expressions)
	for _, v := range g.Objects(id, IRI(SH+"nodeByExpression")) {
		result = append(result, &NodeByExpressionConstraint{ShapeRef: v})
	}

	// sh:sparql constraints
	for _, v := range g.Objects(id, IRI(SH+"sparql")) {
		sc := &SPARQLConstraint{Node: v}
		if sel := g.Objects(v, IRI(SH+"select")); len(sel) > 0 {
			sc.Select = sel[0].Value()
		} else {
			continue
		}
		sc.Prefixes = resolvePrefixes(g, firstOrNone(g.Objects(v, IRI(SH+"prefixes"))))
		sc.Messages = g.Objects(v, IRI(SH+"message"))
		if deact := g.Objects(v, IRI(SH+"deactivated")); len(deact) > 0 {
			sc.Deactivated = deact[0].Value() == "true"
		}
		result = append(result, sc)
	}

	// sh:closed / sh:closed sh:ByTypes
	if s.Closed {
		allowed := collectClosedAllowed(g, s)
		result = append(result, &ClosedConstraint{
			AllowedProperties: allowed,
			IgnoredProperties: s.IgnoredProperties,
		})
	} else if s.ClosedByTypes {
		result = append(result, &ClosedByTypesConstraint{
			Shape: s,
		})
	}

	return result
}

// makePairConstraint creates a pair constraint (equals/disjoint/lessThan/lessThanOrEquals)
// that supports both simple IRI properties and property path sequences (SHACL 1.2).
func makePairConstraint(kind string, g *Graph, v Term) Constraint {
	path := parsePath(g, v)
	if path.Kind == PathPredicate {
		// Simple IRI — use original constraint types
		switch kind {
		case "equals":
			return &EqualsConstraint{Path: v}
		case "disjoint":
			return &DisjointConstraint{Path: v}
		case "lessThan":
			return &LessThanConstraint{Path: v}
		case "lessThanOrEquals":
			return &LessThanOrEqualsConstraint{Path: v}
		}
	}
	// Property path sequence — use path-based constraint
	return &PairPathConstraint{Kind: kind, OtherPath: path}
}

// collectClosedAllowed gathers all allowed properties for a sh:closed shape.
func collectClosedAllowed(g *Graph, s *Shape) []Term {
	var allowed []Term
	for _, ps := range s.Properties {
		if ps.Path != nil && ps.Path.Kind == PathPredicate {
			allowed = append(allowed, ps.Path.Pred)
		}
	}
	return allowed
}
