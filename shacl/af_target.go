package shacl

import "fmt"

// SHACL-AF custom targets (§4).
//
// sh:target points a shape at a target definition instead of one of Core's
// sh:targetClass/sh:targetNode/... properties. Two forms are defined:
//
//	sh:target [ a sh:SPARQLTarget ; sh:select "SELECT ?this WHERE { ... }" ]
//	sh:target [ a ex:MyTargetType ; ex:param "value" ]
//
// The second form instantiates a target type — a sh:SPARQLTargetType declaring
// sh:parameter and a sh:select whose parameter variables are bound from the
// instance's property values, exactly like a SPARQL-based constraint component.

// loadAFTargets resolves every sh:target in the shapes graph, keyed by the
// shape the target is attached to.
//
// It runs once, when the AF context is built, and the result is reused by every
// pass that needs targets — validation and each round of the rule pass. That is
// what keeps the two passes selecting the same focus nodes; resolving per pass
// once made rules see Core targets only. Only the AF layer calls it: without
// SHACL-AF a sh:target is an unrecognised property, which is what a Core-only
// engine must treat it as.
func (ctx *afContext) loadAFTargets() (map[string][]Target, error) {
	g := ctx.shapesGraph
	pred := IRI(SHTarget)
	targets := make(map[string][]Target)
	var firstErr error

	for _, t := range g.All(nil, &pred, nil) {
		query, err := ctx.resolveTarget(t.Object)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if query == "" {
			continue
		}
		key := t.Subject.String()
		targets[key] = append(targets[key], Target{Kind: TargetSPARQL, Select: query})
	}
	return targets, firstErr
}

// resolveTarget turns one sh:target value into a SELECT query whose first
// projected variable names the target nodes.
func (ctx *afContext) resolveTarget(node Term) (string, error) {
	g := ctx.shapesGraph

	// A sh:SPARQLTarget carries its own query.
	if g.HasType(node, IRI(SHSPARQLTarget)) {
		sel := g.Objects(node, IRI(SH+"select"))
		if len(sel) == 0 {
			return "", fmt.Errorf("%w: the sh:SPARQLTarget %s has no sh:select", ErrAdvancedFeatures, node)
		}
		return resolvePrefixes(g, node) + normalizeDollarVars(sel[0].Value()), nil
	}

	// Otherwise the node instantiates a target type: its rdf:type names a
	// sh:SPARQLTargetType whose query is shared by every instance, and the
	// instance supplies the parameter values.
	for _, typ := range g.Objects(node, IRI(RDFType)) {
		if !g.HasType(typ, IRI(SHSPARQLTargetType)) {
			continue
		}
		sel := g.Objects(typ, IRI(SH+"select"))
		if len(sel) == 0 {
			return "", fmt.Errorf("%w: the sh:SPARQLTargetType %s has no sh:select", ErrAdvancedFeatures, typ)
		}
		query := resolvePrefixes(g, typ) + normalizeDollarVars(sel[0].Value())
		return ctx.bindTargetParameters(g, typ, node, query)
	}

	return "", fmt.Errorf("%w: the sh:target %s is neither a sh:SPARQLTarget nor an instance of a sh:SPARQLTargetType", ErrAdvancedFeatures, node)
}

// bindTargetParameters substitutes a target type's declared parameters with the
// values the instance gives them.
//
// A parameter declared with sh:path ex:class is written ?class in the query and
// takes its value from the instance's ex:class property.
func (ctx *afContext) bindTargetParameters(g *Graph, typeNode, instance Term, query string) (string, error) {
	bindings := make(map[string]string)
	for _, p := range g.Objects(typeNode, IRI(SH+"parameter")) {
		paths := g.Objects(p, IRI(SH+"path"))
		if len(paths) == 0 {
			continue
		}
		name := localName(paths[0].Value())
		if name == "" {
			continue
		}
		vals := g.Objects(instance, paths[0])
		if len(vals) == 0 {
			optional := g.Objects(p, IRI(SHOptional))
			if len(optional) > 0 && optional[0].Value() == "true" {
				continue
			}
			return "", fmt.Errorf("%w: the target %s gives no value for the required parameter %s of %s",
				ErrAdvancedFeatures, instance, paths[0], typeNode)
		}
		bindings[name] = termToSPARQL(vals[0])
	}
	if len(bindings) == 0 {
		return query, nil
	}
	return preBindQuery(query, bindings), nil
}
