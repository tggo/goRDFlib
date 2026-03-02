package shacl

// Validate validates dataGraph against shapesGraph and returns a validation report.
func Validate(dataGraph, shapesGraph *Graph) ValidationReport {
	shapes := parseShapes(shapesGraph)

	ctx := &evalContext{
		dataGraph:      dataGraph,
		shapesGraph:    shapesGraph,
		shapesMap:      shapes,
		classInstances: buildClassIndex(dataGraph),
	}

	var allResults []ValidationResult

	for _, s := range shapes {
		if s.Deactivated {
			continue
		}

		targets := resolveTargets(ctx, s)
		if len(targets) == 0 {
			continue
		}

		for _, focusNode := range targets {
			results := validateShapeOnNode(ctx, s, focusNode)
			allResults = append(allResults, results...)
		}
	}

	return ValidationReport{
		Conforms: len(allResults) == 0,
		Results:  allResults,
	}
}

func validateShapeOnNode(ctx *evalContext, s *Shape, focusNode Term) []ValidationResult {
	var results []ValidationResult

	if s.IsProperty && s.Path != nil {
		results = append(results, validatePropertyShape(ctx, s, focusNode)...)
	} else {
		valueNodes := []Term{focusNode}
		for _, c := range s.Constraints {
			results = append(results, c.Evaluate(ctx, s, focusNode, valueNodes)...)
		}
		for _, ps := range s.Properties {
			if ps.Deactivated {
				continue
			}
			results = append(results, validatePropertyShape(ctx, ps, focusNode)...)
		}
	}

	return results
}

func validatePropertyShape(ctx *evalContext, s *Shape, focusNode Term) []ValidationResult {
	var results []ValidationResult
	valueNodes := evalPath(ctx.dataGraph, s.Path, focusNode)

	for _, c := range s.Constraints {
		results = append(results, c.Evaluate(ctx, s, focusNode, valueNodes)...)
	}

	for _, ps := range s.Properties {
		if ps.Deactivated {
			continue
		}
		for _, vn := range valueNodes {
			results = append(results, validatePropertyShape(ctx, ps, vn)...)
		}
	}

	return results
}

// validateNodeAgainstShape validates a single node against a shape (used by logical constraints).
func validateNodeAgainstShape(ctx *evalContext, s *Shape, node Term) []ValidationResult {
	return validateShapeOnNode(ctx, s, node)
}

// buildClassIndex creates a map from class TermKey to instances (subjects with that rdf:type).
func buildClassIndex(g *Graph) map[string][]Term {
	typePred := IRI(RDFType)
	idx := make(map[string][]Term)
	for _, t := range g.All(nil, &typePred, nil) {
		key := t.Object.TermKey()
		idx[key] = append(idx[key], t.Subject)
	}
	return idx
}

// subClasses returns all classes that are rdfs:subClassOf the given class (transitive).
func subClasses(g *Graph, class Term) []Term {
	subClassPred := IRI(RDFSSubClassOf)
	// Find all classes where ?sub rdfs:subClassOf class (reverse lookup), then recurse.
	visited := map[string]bool{class.TermKey(): true}
	queue := []Term{class}
	var result []Term
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		// Find all ?sub where ?sub rdfs:subClassOf cur
		for _, t := range g.All(nil, &subClassPred, &cur) {
			k := t.Subject.TermKey()
			if !visited[k] {
				visited[k] = true
				result = append(result, t.Subject)
				queue = append(queue, t.Subject)
			}
		}
	}
	return result
}

func resolveTargets(ctx *evalContext, s *Shape) []Term {
	seen := make(map[string]bool)
	var targets []Term

	addTarget := func(t Term) {
		key := t.TermKey()
		if !seen[key] {
			seen[key] = true
			targets = append(targets, t)
		}
	}

	for _, tgt := range s.Targets {
		switch tgt.Kind {
		case TargetNode:
			addTarget(tgt.Value)
		case TargetClass, TargetImplicitClass:
			// Direct instances from pre-built index
			for _, inst := range ctx.classInstances[tgt.Value.TermKey()] {
				addTarget(inst)
			}
			// Instances of subclasses
			for _, sub := range subClasses(ctx.dataGraph, tgt.Value) {
				for _, inst := range ctx.classInstances[sub.TermKey()] {
					addTarget(inst)
				}
			}
		case TargetSubjectsOf:
			pred := tgt.Value
			for _, t := range ctx.dataGraph.All(nil, &pred, nil) {
				addTarget(t.Subject)
			}
		case TargetObjectsOf:
			pred := tgt.Value
			for _, t := range ctx.dataGraph.All(nil, &pred, nil) {
				addTarget(t.Object)
			}
		}
	}

	return targets
}
