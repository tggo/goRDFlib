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
	ordered := p.ordered
	if ordered == nil {
		ordered = shapesInOrder(ctx.shapesMap)
	}
	for _, s := range ordered {
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
		quick := ctx.guard.quick
		for _, c := range s.Constraints {
			results = append(results, c.Evaluate(ctx, s, focusNode, valueNodes)...)
			if quick && len(results) > 0 {
				return results
			}
		}
		for _, ps := range s.Properties {
			if ps.Deactivated {
				continue
			}
			results = append(results, validatePropertyShape(ctx, ps, focusNode)...)
			if quick && len(results) > 0 {
				return results
			}
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

	quick := ctx.guard.quick
	for _, c := range s.Constraints {
		results = append(results, c.Evaluate(ctx, s, focusNode, valueNodes)...)
		if quick && len(results) > 0 {
			return results
		}
	}

	for _, ps := range s.Properties {
		if ps.Deactivated {
			continue
		}
		for _, vn := range valueNodes {
			results = append(results, validatePropertyShape(ctx, ps, vn)...)
			if quick && len(results) > 0 {
				return results
			}
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
// quickFailure is what a constraint returns in quick mode instead of building a
// result nobody will read. It is shared, carries no data, and must never be
// modified or reach a report.
var quickFailure = []ValidationResult{{}}

// nodeConforms reports whether node conforms to s. It is
// len(validateNodeAgainstShape(ctx, s, node)) == 0 without the cost of the
// violations: in quick mode (recursionGuard.quick, inherited by everything
// evaluated below, since every derived context shares the guard) validation
// returns at the first violation and a constraint may return quickFailure
// instead of a built result. Only sh:or, sh:and and sh:not, which read nothing
// but emptiness, call it; any caller that needs the results, sh:node for its
// Details for one, must use validateNodeAgainstShape.
func nodeConforms(ctx *evalContext, s *Shape, node Term) bool {
	g := ctx.sharedGuard()
	prev := g.quick
	g.quick = true
	ok := len(validateShapeOnNode(ctx, s, node)) == 0
	g.quick = prev
	return ok
}

func validateNodeAgainstShape(ctx *evalContext, s *Shape, node Term) []ValidationResult {
	return validateShapeOnNode(ctx, s, node)
}

// instancesOf returns the subjects with rdf:type class, straight from the data
// graph's POS index. Validation used to build a class -> instances map of the
// whole graph on every run, to read a few entries of it.
func (ctx *evalContext) instancesOf(class Term) []Term {
	return ctx.dataGraph.Subjects(IRI(RDFType), class)
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
		for _, sub := range g.Subjects(subClassPred, cur) {
			k := sub.TermKey()
			if !visited[k] {
				visited[k] = true
				result = append(result, sub)
				queue = append(queue, sub)
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
	// seen is allocated by the first target: most shapes of a large shapes
	// graph, property shapes among them, select nothing, and a map per shape
	// per document is the cost. It is keyed like the graph indexes (ikey).
	var seen map[ikey]struct{}
	var targets []Term

	addTarget := func(t Term) {
		key := indexKey(t)
		if _, dup := seen[key]; !dup {
			if seen == nil {
				seen = make(map[ikey]struct{}, 8)
			}
			seen[key] = struct{}{}
			targets = append(targets, t)
		}
	}

	for _, tgt := range s.Targets {
		switch tgt.Kind {
		case TargetNode:
			addTarget(tgt.Value)
		case TargetClass, TargetImplicitClass:
			// Direct instances from pre-built index
			for _, inst := range ctx.instancesOf(tgt.Value) {
				addTarget(inst)
			}
			// Instances of subclasses
			for _, sub := range subClasses(ctx.dataGraph, tgt.Value) {
				for _, inst := range ctx.instancesOf(sub) {
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
	for _, sub := range ctx.dataGraph.Subjects(shapePred, shapeID) {
		addTarget(sub)
	}

	return targets
}
