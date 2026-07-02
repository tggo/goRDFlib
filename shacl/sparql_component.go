package shacl

import "github.com/tggo/goRDFlib/term"

// SPARQLComponentConstraint implements custom SPARQL-based constraint components
// defined via sh:ConstraintComponent with sh:validator / sh:nodeValidator / sh:propertyValidator.
type SPARQLComponentConstraint struct {
	ComponentNode Term            // the IRI of the sh:ConstraintComponent
	Parameters    []paramDef      // sh:parameter definitions
	ParamValues   map[string]Term // parameter path IRI → value from the shape
	Validator     *validatorDef   // selected validator (node/property/generic)
	Prefixes      string          // PREFIX preamble for the validator query
}

type paramDef struct {
	Path     Term // sh:path value (the parameter's predicate)
	Optional bool // sh:optional
}

type validatorDef struct {
	IsASK    bool   // true=ASK, false=SELECT
	Query    string // sh:ask or sh:select
	Messages []Term // sh:message on the validator
}

func (c *SPARQLComponentConstraint) ComponentIRI() string {
	return c.ComponentNode.Value()
}

func (c *SPARQLComponentConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if c.Validator == nil {
		return nil
	}

	fullQuery := normalizeDollarVars(c.Prefixes + c.Validator.Query)

	if c.Validator.IsASK {
		return c.evaluateASK(ctx, shape, focusNode, valueNodes, fullQuery)
	}
	return c.evaluateSELECT(ctx, shape, focusNode, valueNodes, fullQuery)
}

func (c *SPARQLComponentConstraint) evaluateASK(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term, queryTemplate string) []ValidationResult {
	var results []ValidationResult

	for _, value := range valueNodes {
		bindings, initBindings := c.buildBindings(shape, focusNode, value)
		query := preBindQuery(queryTemplate, bindings)

		askResult, err := executeSPARQLAsk(ctx.dataGraph, query, initBindings, nil)
		if err != nil {
			r := makeResult(shape, focusNode, value, c.ComponentIRI())
			if len(c.Validator.Messages) > 0 {
				r.ResultMessages = c.Validator.Messages
			}
			results = append(results, r)
			continue
		}

		// ASK returns true if the value conforms; false = violation
		if !askResult {
			r := makeResult(shape, focusNode, value, c.ComponentIRI())
			if len(c.Validator.Messages) > 0 {
				r.ResultMessages = c.Validator.Messages
			}
			results = append(results, r)
		}
	}
	return results
}

func (c *SPARQLComponentConstraint) evaluateSELECT(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term, queryTemplate string) []ValidationResult {
	bindings, initBindings := c.buildBindings(shape, focusNode, Term{})
	query := preBindQuery(queryTemplate, bindings)

	rows, err := executeSPARQL(ctx.dataGraph, query, initBindings, nil)
	if err != nil {
		r := makeResult(shape, focusNode, focusNode, c.ComponentIRI())
		if len(c.Validator.Messages) > 0 {
			r.ResultMessages = c.Validator.Messages
		}
		return []ValidationResult{r}
	}

	var results []ValidationResult
	for _, row := range rows {
		value := focusNode
		if v, ok := row["value"]; ok && !v.IsNone() {
			value = v
		}
		r := makeResult(shape, focusNode, value, c.ComponentIRI())
		if p, ok := row["path"]; ok && !p.IsNone() {
			r.ResultPath = p
		}
		if len(c.Validator.Messages) > 0 {
			r.ResultMessages = c.Validator.Messages
		}
		results = append(results, r)
	}
	return results
}

// buildBindings returns the pre-bound variables split into textual bindings
// (IRI/literal values spliced into query text) and initial bindings
// (blank-node values, which cannot be referenced by label in SPARQL query
// text — see preBindTerm).
func (c *SPARQLComponentConstraint) buildBindings(shape *Shape, focusNode, value Term) (map[string]string, map[string]term.Term) {
	bindings := map[string]string{}
	initBindings := map[string]term.Term{}
	preBindTerm(bindings, initBindings, "this", focusNode)

	if !value.IsNone() {
		preBindTerm(bindings, initBindings, "value", value)
	}

	if shape.Path != nil && shape.Path.Kind == PathPredicate {
		bindings["PATH"] = termToSPARQL(shape.Path.Pred)
	}

	preBindTerm(bindings, initBindings, "currentShape", shape.ID)

	// Bind parameter values
	for _, param := range c.Parameters {
		paramName := localName(param.Path.Value())
		if v, ok := c.ParamValues[param.Path.Value()]; ok {
			preBindTerm(bindings, initBindings, paramName, v)
		}
	}

	return bindings, initBindings
}

// parseConstraintComponents finds all sh:ConstraintComponent definitions in
// the shapes graph and returns constraints for shapes that use them.
func parseConstraintComponents(g *Graph) []componentDef {
	typePred := IRI(RDFType)

	constraintCompType := IRI(SH + "ConstraintComponent")
	var components []componentDef

	// Direct instances
	for _, t := range g.All(nil, &typePred, &constraintCompType) {
		if cd := parseOneComponent(g, t.Subject); cd != nil {
			components = append(components, *cd)
		}
	}

	// Instances of subclasses of sh:ConstraintComponent
	subClassPred := IRI(RDFSSubClassOf)
	for _, t := range g.All(nil, &subClassPred, &constraintCompType) {
		subClass := t.Subject
		for _, inst := range g.All(nil, &typePred, &subClass) {
			if cd := parseOneComponent(g, inst.Subject); cd != nil {
				components = append(components, *cd)
			}
		}
	}

	return components
}

type componentDef struct {
	Node       Term
	Parameters []paramDef
	Validator  *validatorDef // sh:validator (generic)
	NodeVal    *validatorDef // sh:nodeValidator
	PropVal    *validatorDef // sh:propertyValidator
}

func parseOneComponent(g *Graph, node Term) *componentDef {
	cd := &componentDef{Node: node}

	paramPred := IRI(SH + "parameter")
	for _, pn := range g.Objects(node, paramPred) {
		pd := paramDef{}
		if paths := g.Objects(pn, IRI(SH+"path")); len(paths) > 0 {
			pd.Path = paths[0]
		} else {
			continue
		}
		if opts := g.Objects(pn, IRI(SH+"optional")); len(opts) > 0 {
			pd.Optional = opts[0].Value() == "true"
		}
		cd.Parameters = append(cd.Parameters, pd)
	}

	if len(cd.Parameters) == 0 {
		return nil
	}

	cd.Validator = parseValidator(g, node, SH+"validator")
	cd.NodeVal = parseValidator(g, node, SH+"nodeValidator")
	cd.PropVal = parseValidator(g, node, SH+"propertyValidator")

	if cd.Validator == nil && cd.NodeVal == nil && cd.PropVal == nil {
		return nil
	}

	return cd
}

func parseValidator(g *Graph, compNode Term, pred string) *validatorDef {
	vals := g.Objects(compNode, IRI(pred))
	if len(vals) == 0 {
		return nil
	}
	vn := vals[0]
	vd := &validatorDef{}

	if ask := g.Objects(vn, IRI(SH+"ask")); len(ask) > 0 {
		vd.IsASK = true
		vd.Query = ask[0].Value()
	} else if sel := g.Objects(vn, IRI(SH+"select")); len(sel) > 0 {
		vd.IsASK = false
		vd.Query = sel[0].Value()
	} else {
		return nil
	}

	vd.Messages = g.Objects(vn, IRI(SH+"message"))

	return vd
}

// buildComponentConstraints creates SPARQLComponentConstraints for a given shape
// based on discovered constraint component definitions.
func buildComponentConstraints(g *Graph, s *Shape, components []componentDef) []Constraint {
	var result []Constraint

	for _, cd := range components {
		paramValues := make(map[string]Term)
		allRequired := true
		for _, param := range cd.Parameters {
			vals := g.Objects(s.ID, param.Path)
			if len(vals) > 0 {
				paramValues[param.Path.Value()] = vals[0]
			} else if !param.Optional {
				allRequired = false
				break
			}
		}
		if !allRequired || len(paramValues) == 0 {
			continue
		}

		var validator *validatorDef
		if s.IsProperty {
			if cd.PropVal != nil {
				validator = cd.PropVal
			} else if cd.Validator != nil {
				validator = cd.Validator
			}
		} else {
			if cd.NodeVal != nil {
				validator = cd.NodeVal
			} else if cd.Validator != nil {
				validator = cd.Validator
			}
		}
		if validator == nil {
			continue
		}

		prefixes := resolvePrefixesForValidator(g, cd, validator)

		result = append(result, &SPARQLComponentConstraint{
			ComponentNode: cd.Node,
			Parameters:    cd.Parameters,
			ParamValues:   paramValues,
			Validator:     validator,
			Prefixes:      prefixes,
		})
	}

	return result
}

func resolvePrefixesForValidator(g *Graph, cd componentDef, vd *validatorDef) string {
	var pred string
	if vd == cd.Validator {
		pred = SH + "validator"
	} else if vd == cd.NodeVal {
		pred = SH + "nodeValidator"
	} else {
		pred = SH + "propertyValidator"
	}

	vals := g.Objects(cd.Node, IRI(pred))
	if len(vals) == 0 {
		return ""
	}
	vn := vals[0]

	if prefs := g.Objects(vn, IRI(SH+"prefixes")); len(prefs) > 0 {
		return resolvePrefixes(g, prefs[0])
	}
	return ""
}
