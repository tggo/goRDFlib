package shacl

import "strings"

// parseNodeExpr parses a node expression from an RDF graph node.
// The expression node can be:
//   - An IRI, literal or triple term → ConstantExpr
//   - An RDF list → ListExpr (evaluates each member)
//   - A blank node → the function whose key parameter it has, a list
//     parameter function call, or a call of a function g declares
func parseNodeExpr(g *Graph, node Term) NodeExpr {
	// rdf:nil → empty list expression
	if node.IsIRI() && node.Value() == RDFNil {
		return &ListExpr{IsNil: true}
	}

	// Literal and triple term → constant
	if node.IsLiteral() || node.IsTriple() {
		return &ConstantExpr{Value: node}
	}

	// RDF list check (non-blank node that is an IRI with no shnex properties is a constant)
	firstPred := IRI(RDFFirst)
	if g.Has(&node, &firstPred, nil) {
		return &ListExpr{Members: parseNodeExprList(g, node)}
	}

	// IRI (not rdf:nil, not a list head) → constant
	if node.IsIRI() {
		return &ConstantExpr{Value: node}
	}

	// Blank node — check for shnex:* properties
	return parseBlankNodeExpr(g, node)
}

// parseNodeExprList parses the members of the SHACL list at head.
func parseNodeExprList(g *Graph, head Term) []NodeExpr {
	items := g.RDFList(head)
	members := make([]NodeExpr, len(items))
	for i, item := range items {
		members[i] = parseNodeExpr(g, item)
	}
	return members
}

// parseListArguments reads the argument of a list parameter function: the
// members of a SHACL list, or, when the object is not a list, the one
// argument it is (SHACL 1.2 Node Expressions, "List Parameter Functions").
// An object that is a list is always the list of arguments; to pass a list
// expression as the only argument it is wrapped in a list of one.
func parseListArguments(g *Graph, o Term) []NodeExpr {
	if o.IsIRI() && o.Value() == RDFNil {
		return nil
	}
	if g.Has(&o, &Term{kind: TermIRI, value: RDFFirst}, nil) {
		return parseNodeExprList(g, o)
	}
	return []NodeExpr{parseNodeExpr(g, o)}
}

// first returns the first value of p at node.
func first(g *Graph, node Term, p string) (Term, bool) {
	if vals := g.Objects(node, IRI(p)); len(vals) > 0 {
		return vals[0], true
	}
	return Term{}, false
}

// parseBlankNodeExpr parses a blank node as a node expression.
func parseBlankNodeExpr(g *Graph, node Term) NodeExpr {
	triples := g.All(&node, nil, nil)
	if len(triples) == 0 {
		return &EmptyExpr{} // a blank node with no triples is the empty expression
	}
	fns := declaredFunctions(g)

	// A list parameter function call is a blank node with a single triple.
	if len(triples) == 1 {
		if e := parseListFunctionCall(g, fns, triples[0]); e != nil {
			return e
		}
	}

	// shnex:var
	if v, ok := first(g, node, SHNEX+"var"); ok {
		return &VarExpr{Name: v.Value()}
	}

	// shnex:arg
	if v, ok := first(g, node, SHNEX+"arg"); ok {
		key, valid := argKey(v)
		if !valid {
			return &IllFormedExpr{Node: node}
		}
		return &ArgExpr{Key: key}
	}

	// shnex:pathValues
	if v, ok := first(g, node, SHNEX+"pathValues"); ok {
		path := parsePath(g, v)
		var focusNodeExpr NodeExpr
		if fn, ok := first(g, node, SHNEX+"focusNode"); ok {
			focusNodeExpr = parseNodeExpr(g, fn)
		}
		return &PathValuesExpr{Path: path, FocusNode: focusNodeExpr}
	}

	// Expressions with one node expression as their key parameter.
	for _, k := range []struct {
		prop string
		make func(NodeExpr) NodeExpr
	}{
		{"count", func(n NodeExpr) NodeExpr { return &CountExpr{Nodes: n} }},
		{"sum", func(n NodeExpr) NodeExpr { return &SumExpr{Nodes: n} }},
		{"min", func(n NodeExpr) NodeExpr { return &MinExpr{Nodes: n} }},
		{"max", func(n NodeExpr) NodeExpr { return &MaxExpr{Nodes: n} }},
		{"exists", func(n NodeExpr) NodeExpr { return &ExistsExpr{Nodes: n} }},
		{"distinct", func(n NodeExpr) NodeExpr { return &DistinctExpr{Nodes: n} }},
		{"instancesOf", func(n NodeExpr) NodeExpr { return &InstancesOfExpr{Classes: n} }},
	} {
		if v, ok := first(g, node, SHNEX+k.prop); ok {
			return k.make(parseNodeExpr(g, v))
		}
	}

	// Expressions whose key parameter is a SHACL list of node expressions.
	if v, ok := first(g, node, SHNEX+"concat"); ok {
		return &ConcatExpr{Members: parseNodeExprList(g, v)}
	}
	if v, ok := first(g, node, SHNEX+"intersection"); ok {
		return &IntersectionExpr{Members: parseNodeExprList(g, v)}
	}

	// Expressions with a shape as their key parameter.
	if v, ok := first(g, node, SHNEX+"filterShape"); ok {
		return &FilterShapeExpr{ShapeRef: v, Nodes: parseNodesArg(g, node)}
	}
	if v, ok := first(g, node, SHNEX+"findFirst"); ok {
		return &FindFirstExpr{ShapeRef: v, Nodes: optionalNodesArg(g, node)}
	}
	if v, ok := first(g, node, SHNEX+"matchAll"); ok {
		return &MatchAllExpr{ShapeRef: v, Nodes: optionalNodesArg(g, node)}
	}
	if v, ok := first(g, node, SHNEX+"nodesMatching"); ok {
		return &NodesMatchingExpr{ShapeRef: v}
	}

	// shnex:if
	if v, ok := first(g, node, SHNEX+"if"); ok {
		e := &IfExpr{Condition: parseNodeExpr(g, v)}
		if t, ok := first(g, node, SHNEX+"then"); ok {
			e.Then = parseNodeExpr(g, t)
		}
		if t, ok := first(g, node, SHNEX+"else"); ok {
			e.Else = parseNodeExpr(g, t)
		}
		return e
	}

	// shnex:remove + shnex:nodes
	if v, ok := first(g, node, SHNEX+"remove"); ok {
		return &RemoveExpr{Nodes: parseNodesArg(g, node), ToRemove: parseNodeExpr(g, v)}
	}

	// Expressions that use shnex:nodes as primary with modifiers
	// shnex:flatMap + shnex:nodes
	if v, ok := first(g, node, SHNEX+"flatMap"); ok {
		return &FlatMapExpr{Nodes: optionalNodesArg(g, node), MapExpr: parseNodeExpr(g, v)}
	}

	// shnex:orderBy + shnex:nodes
	if v, ok := first(g, node, SHNEX+"orderBy"); ok {
		desc := false
		if d, ok := first(g, node, SHNEX+"desc"); ok {
			desc = d.Value() == "true"
		}
		return &OrderByExpr{Nodes: parseNodesArg(g, node), KeyExpr: parseNodeExpr(g, v), Desc: desc}
	}

	// shnex:nodes + shnex:limit
	if v, ok := first(g, node, SHNEX+"limit"); ok {
		return &LimitExpr{Nodes: parseNodesArg(g, node), Limit: v}
	}

	// shnex:nodes + shnex:offset
	if v, ok := first(g, node, SHNEX+"offset"); ok {
		return &OffsetExpr{Nodes: parseNodesArg(g, node), Offset: v}
	}

	// A call of a custom named parameter function.
	if fns != nil {
		for _, t := range triples {
			if fn, ok := fns.named[t.Predicate.Value()]; ok {
				args := make(map[string]NodeExpr, len(fns.namedParams[fn]))
				for _, p := range fns.namedParams[fn] {
					if v, ok := first(g, node, p.Value()); ok {
						key, _ := argKey(p)
						args[key] = parseNodeExpr(g, v)
					}
				}
				return &CustomNamedExpr{Function: fn.iri, Body: fn.body, Args: args}
			}
		}
	}

	// shnex:nodes alone (without modifiers)
	if v, ok := first(g, node, SHNEX+"nodes"); ok {
		return parseNodeExpr(g, v)
	}

	return &IllFormedExpr{Node: node}
}

// parseListFunctionCall reads t, the only triple of a blank node, as a call of
// a list parameter function: a sparql: function, shnex:conformsToShape, or a
// sh:ListParameterExpressionFunction the shapes graph declares. It returns nil
// when the predicate is none of them.
func parseListFunctionCall(g *Graph, fns *customFunctions, t Triple) NodeExpr {
	p := t.Predicate.Value()
	switch {
	case strings.HasPrefix(p, SPARQLNS):
		name := strings.TrimPrefix(p, SPARQLNS)
		if _, ok := sparqlFunctions[name]; ok {
			return &SPARQLFunctionExpr{Name: name, Args: parseListArguments(g, t.Object)}
		}
	case p == SHNEX+"conformsToShape":
		return &ConformsToShapeExpr{Args: parseListArguments(g, t.Object)}
	}
	if fns != nil {
		if fn, ok := fns.list[p]; ok {
			return &CustomListExpr{Function: fn.iri, Body: fn.body, Args: parseListArguments(g, t.Object)}
		}
	}
	return nil
}

// parseNodesArg parses the shnex:nodes argument, defaulting to an empty expression.
func parseNodesArg(g *Graph, node Term) NodeExpr {
	if n := optionalNodesArg(g, node); n != nil {
		return n
	}
	return &EmptyExpr{}
}

// optionalNodesArg parses shnex:nodes; nil when it is absent, which the
// expressions that default to the focus node read as such.
func optionalNodesArg(g *Graph, node Term) NodeExpr {
	if n, ok := first(g, node, SHNEX+"nodes"); ok {
		return parseNodeExpr(g, n)
	}
	return nil
}
