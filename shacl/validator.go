package shacl

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/tggo/goRDFlib/term"
)

// ErrMalformedTarget is reported when a SPARQL-based target cannot be run: a
// sh:targetNode carrying a broken sh:select (SHACL 1.2), or a sh:SPARQLTarget
// carrying one (SHACL-AF). Such a target selects no focus nodes, which reads
// exactly like a target that legitimately matches nothing — and for a sh:rule
// it becomes an inference that silently does not happen. Target resolution has
// no error return, so this reaches the caller through WithErrorHandler.
var ErrMalformedTarget = errors.New("shacl: malformed target")

// Validate validates dataGraph against shapesGraph and returns a validation report.
//
// With no options this is SHACL Core plus SHACL-SPARQL and SHACL 1.2, which is
// what it has always been. Pass WithAdvancedFeatures to additionally interpret
// SHACL-AF: rules run first and validation sees the triples they infer, custom
// targets are resolved, and SHACL functions become callable. The caller's data
// graph is never modified — rules are applied to a copy.
//
// Validate has no error return, so problems that are not validation results —
// a malformed rule, a rule set that does not terminate — are only visible
// through WithErrorHandler. Use ApplyRules when the inference itself is what
// matters.
//
// Results are sorted by focus node, then path, constraint component and value,
// so the same input gives the same report order on every run; see
// orderResults for how blank nodes are handled.
func Validate(dataGraph, shapesGraph *Graph, opts ...Option) ValidationReport {
	return Prepare(dataGraph, shapesGraph, opts...).Validate()
}

// ValidateContext is Validate with a context. The context reaches the stores
// the validation reads (SPARQL constraints and targets, rules, SHACL functions,
// and a graph wrapped with NewGraphFromRDF), and stops the run once it is done.
// A stopped run returns an empty report and an error matching ErrCancelled and
// ctx.Err() under errors.Is, never a partial report.
func ValidateContext(ctx context.Context, dataGraph, shapesGraph *Graph, opts ...Option) (ValidationReport, error) {
	p, err := PrepareContext(ctx, dataGraph, shapesGraph, opts...)
	if err != nil {
		return ValidationReport{}, err
	}
	return p.ValidateContext(ctx)
}

// Validate checks the prepared graph without rerunning rules or reparsing shapes.
// Each call uses its own recursion state and preserves ordinary report behavior.
func (p *Prepared) Validate() ValidationReport {
	report, _ := p.ValidateContext(context.Background())
	return report
}

// ValidateContext is Prepared.Validate with a context; see ValidateContext.
// The context is checked before each focus node, and once more at the end,
// because a store that gave up on a read reports it as no match.
func (p *Prepared) ValidateContext(goctx context.Context) (ValidationReport, error) {
	if err := stopped(goctx); err != nil {
		return ValidationReport{}, err
	}
	ctx := p.evaluation(goctx)
	var allResults []ValidationResult

	// Shapes are visited in the order of their keys rather than map order, so
	// that anything reported to the error handler along the way arrives in the
	// same sequence on every run.
	for _, s := range shapesInOrder(ctx.shapesMap) {
		if s.Deactivated {
			continue
		}

		targets := p.targetNodes(ctx, s)
		if len(targets) == 0 {
			continue
		}

		for _, focusNode := range targets {
			if err := stopped(goctx); err != nil {
				return ValidationReport{}, err
			}
			results := validateShapeOnNode(ctx, s, focusNode)
			allResults = append(allResults, results...)
		}
	}
	if err := stopped(goctx); err != nil {
		return ValidationReport{}, err
	}

	orderResults(allResults)

	// Source lines are filled in once over the finished report rather than at
	// each place a result is built, so a constraint never has to know that
	// provenance exists. Costs nothing when WithSourceLines was not passed.
	annotateSourceLines(allResults, ctx.cfg.provenance)

	// SHACL 1.2: sh:Debug and sh:Trace severities don't affect sh:conforms
	conforms := true
	for _, r := range allResults {
		sev := r.ResultSeverity.Value()
		if sev != SH+"Debug" && sev != SH+"Trace" {
			conforms = false
			break
		}
	}

	return ValidationReport{
		Conforms: conforms,
		Results:  allResults,
	}, nil
}

// shapesInOrder returns the parsed shapes sorted by their map key.
func shapesInOrder(shapes map[string]*Shape) []*Shape {
	keys := make([]string, 0, len(shapes))
	for k := range shapes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*Shape, len(keys))
	for i, k := range keys {
		out[i] = shapes[k]
	}
	return out
}

func validateShapeOnNode(ctx *evalContext, s *Shape, focusNode Term) []ValidationResult {
	var results []ValidationResult

	if s.IsProperty && s.Path != nil {
		results = append(results, validatePropertyShape(ctx, s, focusNode)...)
	} else {
		if !ctx.enter(s, focusNode) {
			return nil // see recursionGuard
		}
		defer ctx.leave(s, focusNode)
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
	if !ctx.enter(s, focusNode) {
		return nil // see recursionGuard
	}
	defer ctx.leave(s, focusNode)

	var results []ValidationResult
	valueNodes := propertyValueNodes(ctx, s, focusNode)

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

func propertyValueNodes(ctx *evalContext, s *Shape, focusNode Term) []Term {
	if s.Values != nil {
		// SHACL 1.2: sh:values — compute value nodes via SPARQL
		return evalSPARQLValues(ctx, s.Values, focusNode)
	}
	return evalPath(ctx.dataGraph, s.Path, focusNode)
}

// evalSPARQLValues computes value nodes using a SPARQL query or expression.
func evalSPARQLValues(ctx *evalContext, v *SPARQLValues, focusNode Term) []Term {
	var query string
	if v.Select != "" {
		query = v.Prefixes + v.Select
	} else if v.Expr != "" {
		query = v.Prefixes + "SELECT (" + v.Expr + " AS ?value) WHERE { }"
	} else {
		return nil
	}
	// Pre-bind $this. IRI/literal focus nodes are substituted textually; a
	// blank-node focus node must be bound via initial bindings, since its
	// label cannot be referenced in SPARQL query text (see preBindTerm).
	var initBindings map[string]term.Term
	if focusNode.Kind() == TermBlankNode {
		query = replaceVar(query, "$this", "?this")
		initBindings = map[string]term.Term{"this": toTerm(focusNode)}
	} else {
		thisVal := termToSPARQL(focusNode)
		query = replaceVar(query, "$this", thisVal)
		query = replaceVar(query, "?this", thisVal)
	}
	rows, err := executeSPARQL(ctx.goContext(), ctx.dataGraph, query, initBindings, nil, ctx.sparqlFuncs())
	if err != nil {
		return nil
	}
	var values []Term
	for _, row := range rows {
		for _, val := range row {
			values = append(values, val)
			break
		}
	}
	return values
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

// allNodes returns all unique subjects and objects in the data graph.
func allNodes(g *Graph) []Term {
	seen := make(map[string]bool)
	var nodes []Term
	for _, t := range g.Triples() {
		if k := t.Subject.TermKey(); !seen[k] {
			seen[k] = true
			nodes = append(nodes, t.Subject)
		}
		if k := t.Object.TermKey(); !seen[k] {
			seen[k] = true
			nodes = append(nodes, t.Object)
		}
	}
	return nodes
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
		case TargetSPARQL:
			query := tgt.Select
			results, err := executeSPARQL(ctx.goContext(), ctx.dataGraph, query, nil, nil, ctx.sparqlFuncs())
			if err != nil {
				// A target that selects nothing is indistinguishable from one
				// that is broken, and for a rule it turns into an inference
				// that silently does not happen.
				ctx.report(fmt.Errorf("%w: the SPARQL target of %s: %w", ErrMalformedTarget, s.ID, err))
				continue
			}
			for _, row := range results {
				// First bound variable is the target node
				for _, v := range row {
					addTarget(v)
					break
				}
			}
		case TargetWhere:
			// sh:targetWhere: target nodes are all nodes in the data graph that
			// conform to the shape described by the targetWhere value.
			twShape := ctx.shapesMap[tgt.Value.String()]
			if twShape != nil {
				candidates := allNodes(ctx.dataGraph)
				for _, node := range candidates {
					results := validateShapeOnNode(ctx, twShape, node)
					if len(results) == 0 {
						addTarget(node)
					}
				}
			}
		}
	}

	// SHACL 1.2: sh:shape — nodes in the data graph that declare sh:shape targeting this shape
	shapePred := IRI(SH + "shape")
	shapeID := s.ID
	for _, t := range ctx.dataGraph.All(nil, &shapePred, &shapeID) {
		addTarget(t.Subject)
	}

	return targets
}
