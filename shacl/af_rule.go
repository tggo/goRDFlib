package shacl

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/tggo/goRDFlib/term"
)

// SHACL-AF rules (§7).
//
// A rule is attached to a shape with sh:rule and infers new triples about that
// shape's focus nodes. Two kinds are defined:
//
//	sh:TripleRule   sh:subject/sh:predicate/sh:object, each a node expression
//	sh:SPARQLRule   sh:construct, a CONSTRUCT query with $this pre-bound
//
// Execution order is the spec's, not a fixed point: shapes run in ascending
// sh:order, and within a shape its rules run in ascending sh:order, each once
// per focus node. Chaining is expressed by ordering — the spec's own example
// infers uncles at sh:order 1 so that cousins can be inferred from them at 2.
// WithRuleIteration re-runs the set to a fixed point instead, for shapes that
// rely on it.

// afRule is one sh:rule.
type afRule struct {
	shapeID     Term
	node        Term
	order       float64
	deactivated bool
	conditions  []Term // shape IDs the focus node must conform to

	// Exactly one of these is set.
	triple *tripleRule
	query  string // sh:construct, prefixes already applied
}

type tripleRule struct {
	subject   NodeExpr
	predicate NodeExpr
	object    NodeExpr
}

// ruleGroup is the rules of one shape, in execution order.
type ruleGroup struct {
	shapeID Term
	order   float64
	rules   []afRule
}

// applyRules runs every rule in the shapes graph against the data graph and
// returns how many triples were added.
func (ctx *afContext) applyRules() (int, error) {
	groups, err := ctx.loadRules()
	if err != nil && len(groups) == 0 {
		return 0, err
	}
	if ctx.loadErr != nil && err == nil {
		err = ctx.loadErr
	}
	if len(groups) == 0 {
		return 0, err
	}

	total := 0
	for _, group := range groups {
		added, groupErr := ctx.applyGroup(group)
		total += added
		if groupErr != nil && err == nil {
			err = groupErr
		}
	}
	return total, err
}

// applyGroup runs one shape's rules.
//
// Without WithRuleIteration this is a single ordered pass, which is what the
// spec describes and what pySHACL does by default. With it, the pass repeats
// while it keeps producing triples, bounded by the iteration limit.
func (ctx *afContext) applyGroup(group ruleGroup) (int, error) {
	total := 0
	for round := 0; ; round++ {
		if round >= ctx.cfg.ruleIterationLimit {
			return total, fmt.Errorf("%w: the rules of %s were still producing triples after %d rounds; raise it with WithRuleIterationLimit, or drop WithRuleIteration to make a single ordered pass",
				ErrRuleIterationLimit, group.shapeID, ctx.cfg.ruleIterationLimit)
		}

		// The evaluation context is rebuilt each round: a rule that inferred an
		// rdf:type changes which nodes the next round's targets select.
		eval := ctx.evalCtx()
		added := 0
		var firstErr error
		for i := range group.rules {
			n, err := ctx.applyRule(eval, group.shapeID, &group.rules[i])
			added += n
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}
		total += added

		if firstErr != nil {
			return total, firstErr
		}
		if added == 0 || !ctx.cfg.ruleIteration {
			return total, nil
		}
	}
}

func (ctx *afContext) applyRule(eval *evalContext, shapeID Term, rule *afRule) (int, error) {
	if rule.deactivated {
		return 0, nil
	}
	shape := eval.shapesMap[shapeID.String()]
	if shape == nil {
		return 0, nil
	}
	focusNodes := ctx.applicableFocusNodes(eval, shape, rule)
	if len(focusNodes) == 0 {
		return 0, nil
	}
	if rule.triple != nil {
		return ctx.applyTripleRule(eval, rule, focusNodes)
	}
	return ctx.applySPARQLRule(rule, focusNodes)
}

// applicableFocusNodes returns the shape's focus nodes that pass every
// sh:condition of the rule.
func (ctx *afContext) applicableFocusNodes(eval *evalContext, shape *Shape, rule *afRule) []Term {
	focusNodes := resolveTargets(eval, shape)
	if len(rule.conditions) == 0 {
		return focusNodes
	}

	// Conditions are resolved once, not per focus node: an anonymous condition
	// has to be parsed, and doing that inside the loop would re-parse it for
	// every node the shape targets.
	conditions := make([]*Shape, 0, len(rule.conditions))
	for _, cond := range rule.conditions {
		if s := resolveShape(eval, cond); s != nil {
			conditions = append(conditions, s)
			continue
		}
		// A condition that names something which is not a shape can never be
		// satisfied. Treating it as satisfied would silently widen the rule to
		// every focus node, which is the more damaging reading.
		ctx.cfg.report(fmt.Errorf("%w: the sh:condition %s of rule %s is not a shape, so the rule can never fire",
			ErrMalformedRule, cond, rule.node))
		return nil
	}

	var out []Term
	for _, fn := range focusNodes {
		ok := true
		for _, condShape := range conditions {
			if len(validateShapeOnNode(eval, condShape, fn)) > 0 {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, fn)
		}
	}
	return out
}

// resolveShape looks a shape up by node, parsing an anonymous one on demand.
//
// SHACL-AF lets a sh:condition or sh:filterShape be written inline as a blank
// node with constraints on it. Such a shape is not typed sh:NodeShape and so is
// not in the shapes map, but it is still a shape and must be evaluated.
func resolveShape(eval *evalContext, node Term) *Shape {
	if s, ok := eval.shapesMap[node.String()]; ok && s != nil {
		return s
	}
	if s := parseAdHocShape(eval.shapesGraph, node, eval.shapesMap); s != nil {
		return s
	}
	return nil
}

func (ctx *afContext) applyTripleRule(eval *evalContext, rule *afRule, focusNodes []Term) (int, error) {
	added := 0
	for _, fn := range focusNodes {
		exprCtx := &nodeExprContext{
			dataGraph:      ctx.dataGraph,
			shapesGraph:    ctx.shapesGraph,
			shapesMap:      eval.shapesMap,
			classInstances: eval.classInstances,
			focusNode:      fn,
			af:             ctx,
			guard:          eval.sharedGuard(),
		}
		subjects := rule.triple.subject.Eval(exprCtx)
		predicates := rule.triple.predicate.Eval(exprCtx)
		objects := rule.triple.object.Eval(exprCtx)

		for _, s := range subjects {
			if s.IsLiteral() {
				// A literal cannot be a subject; RDF has no way to state it.
				continue
			}
			for _, p := range predicates {
				if !p.IsIRI() {
					continue
				}
				for _, o := range objects {
					if ctx.addTriple(s, p, o) {
						added++
					}
				}
			}
		}
	}
	return added, nil
}

func (ctx *afContext) applySPARQLRule(rule *afRule, focusNodes []Term) (int, error) {
	added := 0
	var firstErr error
	for _, fn := range focusNodes {
		query, bindings := preBindThis(rule.query, fn)
		triples, err := executeSPARQLConstruct(ctx.dataGraph, query, bindings, ctx.functionsAtDepth(1))
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%w: sh:construct of %s: %w", ErrMalformedRule, rule.node, err)
			}
			continue
		}
		for _, t := range triples {
			if ctx.addTriple(t.Subject, t.Predicate, t.Object) {
				added++
			}
		}
	}
	return added, firstErr
}

// addTriple adds a triple to the data graph, reporting whether it was new.
func (ctx *afContext) addTriple(s, p, o Term) bool {
	if ctx.dataGraph.Has(&s, &p, &o) {
		return false
	}
	ctx.dataGraph.Add(s, p, o)
	return true
}

// preBindThis binds $this in a rule's query to the focus node.
//
// An IRI or literal focus node is spliced into the query text; a blank node
// cannot be, because a blank-node label written in a query is a fresh
// query-scoped node rather than a reference to the one in the data (SPARQL 1.1
// §4.1.4), so it is passed as an initial binding instead.
func preBindThis(query string, focusNode Term) (string, map[string]term.Term) {
	query = normalizeDollarVars(query)
	if focusNode.Kind() == TermBlankNode {
		return query, map[string]term.Term{"this": toTerm(focusNode)}
	}
	return replaceVar(query, "?this", termToSPARQL(focusNode)), nil
}

// loadRules reads every sh:rule in the shapes graph, grouped by the shape it is
// attached to and sorted into execution order.
func (ctx *afContext) loadRules() ([]ruleGroup, error) {
	g := ctx.shapesGraph
	rulePred := IRI(SHRule)

	byShape := make(map[string]*ruleGroup)
	var firstErr error
	for _, t := range g.All(nil, &rulePred, nil) {
		rule, err := ctx.parseRule(t.Subject, t.Object)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		key := t.Subject.String()
		group := byShape[key]
		if group == nil {
			group = &ruleGroup{shapeID: t.Subject, order: numericProperty(g, t.Subject, SHOrder)}
			byShape[key] = group
		}
		group.rules = append(group.rules, *rule)
	}

	groups := make([]ruleGroup, 0, len(byShape))
	for _, group := range byShape {
		sort.SliceStable(group.rules, func(i, j int) bool { return group.rules[i].order < group.rules[j].order })
		groups = append(groups, *group)
	}
	// Shapes are ordered by sh:order, then by identity so that two shapes with
	// the same order still run in a reproducible sequence.
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].order != groups[j].order {
			return groups[i].order < groups[j].order
		}
		return groups[i].shapeID.String() < groups[j].shapeID.String()
	})
	return groups, firstErr
}

func (ctx *afContext) parseRule(shapeID, node Term) (*afRule, error) {
	g := ctx.shapesGraph
	rule := &afRule{
		shapeID: shapeID,
		node:    node,
		order:   numericProperty(g, node, SHOrder),
	}
	if d := g.Objects(node, IRI(SH+"deactivated")); len(d) > 0 {
		rule.deactivated = d[0].Value() == "true"
	}
	// sh:condition may be given repeatedly or once as an RDF list.
	for _, c := range g.Objects(node, IRI(SHCondition)) {
		firstPred := IRI(RDFFirst)
		if g.Has(&c, &firstPred, nil) {
			rule.conditions = append(rule.conditions, g.RDFList(c)...)
			continue
		}
		rule.conditions = append(rule.conditions, c)
	}

	isTriple := g.HasType(node, IRI(SHTripleRule))
	isSPARQL := g.HasType(node, IRI(SHSPARQLRule))
	switch {
	case isTriple && isSPARQL:
		return nil, fmt.Errorf("%w: %s is declared both a sh:TripleRule and a sh:SPARQLRule", ErrMalformedRule, node)
	case isTriple:
		tr, err := ctx.parseTripleRule(node)
		if err != nil {
			return nil, err
		}
		rule.triple = tr
	case isSPARQL:
		construct := g.Objects(node, IRI(SHConstruct))
		if len(construct) == 0 {
			return nil, fmt.Errorf("%w: the sh:SPARQLRule %s has no sh:construct", ErrMalformedRule, node)
		}
		rule.query = resolvePrefixes(g, node) + construct[0].Value()
	default:
		return nil, fmt.Errorf("%w: %s is neither a sh:TripleRule nor a sh:SPARQLRule; a sh:rule must be typed as one of them", ErrMalformedRule, node)
	}
	return rule, nil
}

func (ctx *afContext) parseTripleRule(node Term) (*tripleRule, error) {
	g := ctx.shapesGraph
	tr := &tripleRule{}
	for _, part := range []struct {
		pred string
		dst  *NodeExpr
	}{
		{SHSubject, &tr.subject},
		{SHPredicate, &tr.predicate},
		{SHObject, &tr.object},
	} {
		vals := g.Objects(node, IRI(part.pred))
		if len(vals) == 0 {
			return nil, fmt.Errorf("%w: the sh:TripleRule %s has no %s", ErrMalformedRule, node, shortName(part.pred))
		}
		if len(vals) > 1 {
			return nil, fmt.Errorf("%w: the sh:TripleRule %s has %d values for %s; it must have exactly one", ErrMalformedRule, node, len(vals), shortName(part.pred))
		}
		expr, err := parseAFNodeExpr(ctx, vals[0], 0)
		if err != nil {
			return nil, fmt.Errorf("%s of %s: %w", shortName(part.pred), node, err)
		}
		*part.dst = expr
	}
	return tr, nil
}

// numericProperty reads a numeric property such as sh:order, defaulting to 0 as
// the spec requires when it is absent or not a number.
func numericProperty(g *Graph, node Term, pred string) float64 {
	vals := g.Objects(node, IRI(pred))
	if len(vals) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(vals[0].Value(), 64)
	if err != nil {
		return 0
	}
	return v
}

// shortName renders a SHACL IRI as sh:local for error messages.
func shortName(iri string) string {
	if len(iri) > len(SH) && iri[:len(SH)] == SH {
		return "sh:" + iri[len(SH):]
	}
	return iri
}
