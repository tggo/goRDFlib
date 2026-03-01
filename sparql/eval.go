package sparql

import (
	"fmt"
	"slices"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// EvalQuery evaluates a parsed SPARQL query against a graph.
// Ported from: rdflib.plugins.sparql.evaluate.evalQuery
func EvalQuery(g *rdflibgo.Graph, q *ParsedQuery, initBindings map[string]rdflibgo.Term) (*Result, error) {
	// Evaluate WHERE clause
	solutions := evalPattern(g, q.Where, q.Prefixes)

	// Apply initial bindings filter
	if initBindings != nil {
		solutions = filterByBindings(solutions, initBindings)
	}

	switch q.Type {
	case "SELECT":
		return evalSelect(g, q, solutions)
	case "ASK":
		return &Result{Type: "ASK", AskResult: len(solutions) > 0}, nil
	case "CONSTRUCT":
		return evalConstruct(g, q, solutions)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.Type)
	}
}

func evalSelect(g *rdflibgo.Graph, q *ParsedQuery, solutions []map[string]rdflibgo.Term) (*Result, error) {
	// ORDER BY
	if len(q.OrderBy) > 0 {
		slices.SortFunc(solutions, func(a, b map[string]rdflibgo.Term) int {
			for _, ob := range q.OrderBy {
				va := evalExpr(ob.Expr, a, q.Prefixes)
				vb := evalExpr(ob.Expr, b, q.Prefixes)
				c := compareTermValues(va, vb)
				if ob.Desc {
					c = -c
				}
				if c != 0 {
					return c
				}
			}
			return 0
		})
	}

	// DISTINCT
	if q.Distinct {
		seen := make(map[string]bool)
		var unique []map[string]rdflibgo.Term
		for _, s := range solutions {
			k := solutionKey(s, q.Variables)
			if !seen[k] {
				seen[k] = true
				unique = append(unique, s)
			}
		}
		solutions = unique
	}

	// OFFSET
	if q.Offset > 0 {
		if q.Offset >= len(solutions) {
			solutions = nil
		} else {
			solutions = solutions[q.Offset:]
		}
	}

	// LIMIT
	if q.Limit >= 0 && q.Limit < len(solutions) {
		solutions = solutions[:q.Limit]
	}

	// Determine variables
	vars := q.Variables
	if vars == nil && len(solutions) > 0 {
		varSet := make(map[string]bool)
		for _, s := range solutions {
			for k := range s {
				varSet[k] = true
			}
		}
		for v := range varSet {
			vars = append(vars, v)
		}
		slices.Sort(vars)
	}

	// Project
	if vars != nil {
		projected := make([]map[string]rdflibgo.Term, len(solutions))
		for i, s := range solutions {
			row := make(map[string]rdflibgo.Term)
			for _, v := range vars {
				if val, ok := s[v]; ok {
					row[v] = val
				}
			}
			projected[i] = row
		}
		solutions = projected
	}

	return &Result{Type: "SELECT", Vars: vars, Bindings: solutions}, nil
}

func evalConstruct(g *rdflibgo.Graph, q *ParsedQuery, solutions []map[string]rdflibgo.Term) (*Result, error) {
	result := rdflibgo.NewGraph()
	for _, sol := range solutions {
		for _, tmpl := range q.Construct {
			s := resolveTemplateValue(tmpl.Subject, sol, q.Prefixes)
			p := resolveTemplateValue(tmpl.Predicate, sol, q.Prefixes)
			o := resolveTemplateValue(tmpl.Object, sol, q.Prefixes)
			if s == nil || p == nil || o == nil {
				continue
			}
			subj, okS := s.(rdflibgo.Subject)
			pred, okP := p.(rdflibgo.URIRef)
			if !okS || !okP {
				continue
			}
			result.Add(subj, pred, o)
		}
	}
	return &Result{Type: "CONSTRUCT", Graph: result}, nil
}

func resolveTemplateValue(s string, bindings map[string]rdflibgo.Term, prefixes map[string]string) rdflibgo.Term {
	if strings.HasPrefix(s, "?") {
		v := s[1:]
		if val, ok := bindings[v]; ok {
			return val
		}
		return nil
	}
	return resolveTermRef(s, prefixes)
}

func resolveTermRef(s string, prefixes map[string]string) rdflibgo.Term {
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return rdflibgo.NewURIRefUnsafe(s[1 : len(s)-1])
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		prefix := s[:idx]
		local := s[idx+1:]
		if ns, ok := prefixes[prefix]; ok {
			return rdflibgo.NewURIRefUnsafe(ns + local)
		}
	}
	return nil
}

// evalPattern evaluates a SPARQL pattern, returning solution bindings.
// Ported from: rdflib.plugins.sparql.evaluate.evalPart
func evalPattern(g *rdflibgo.Graph, pattern Pattern, prefixes map[string]string) []map[string]rdflibgo.Term {
	if pattern == nil {
		return []map[string]rdflibgo.Term{{}}
	}

	switch p := pattern.(type) {
	case *BGP:
		return evalBGP(g, p.Triples, map[string]rdflibgo.Term{}, prefixes)

	case *JoinPattern:
		left := evalPattern(g, p.Left, prefixes)
		var result []map[string]rdflibgo.Term
		for _, lb := range left {
			// Evaluate right with left bindings pushed in
			right := evalPatternWithBindings(g, p.Right, lb, prefixes)
			for _, rb := range right {
				result = append(result, mergeBindings(lb, rb))
			}
		}
		return result

	case *OptionalPattern:
		main := evalPattern(g, p.Main, prefixes)
		var result []map[string]rdflibgo.Term
		for _, mb := range main {
			opt := evalPatternWithBindings(g, p.Optional, mb, prefixes)
			if len(opt) > 0 {
				for _, ob := range opt {
					result = append(result, mergeBindings(mb, ob))
				}
			} else {
				result = append(result, mb)
			}
		}
		return result

	case *UnionPattern:
		left := evalPattern(g, p.Left, prefixes)
		right := evalPattern(g, p.Right, prefixes)
		return append(left, right...)

	case *FilterPattern:
		inner := evalPattern(g, p.Pattern, prefixes)
		var result []map[string]rdflibgo.Term
		for _, b := range inner {
			val := evalExpr(p.Expr, b, prefixes)
			if effectiveBooleanValue(val) {
				result = append(result, b)
			}
		}
		return result

	case *BindPattern:
		inner := evalPattern(g, p.Pattern, prefixes)
		var result []map[string]rdflibgo.Term
		for _, b := range inner {
			val := evalExpr(p.Expr, b, prefixes)
			nb := copyBindings(b)
			if val != nil {
				nb[p.Var] = val
			}
			result = append(result, nb)
		}
		return result

	case *ValuesPattern:
		var result []map[string]rdflibgo.Term
		for _, row := range p.Values {
			b := make(map[string]rdflibgo.Term)
			for i, v := range p.Vars {
				if i < len(row) {
					b[v] = row[i]
				}
			}
			result = append(result, b)
		}
		return result
	}

	return []map[string]rdflibgo.Term{{}}
}

func evalPatternWithBindings(g *rdflibgo.Graph, pattern Pattern, bindings map[string]rdflibgo.Term, prefixes map[string]string) []map[string]rdflibgo.Term {
	switch p := pattern.(type) {
	case *BGP:
		return evalBGP(g, p.Triples, bindings, prefixes)
	default:
		// For non-BGP patterns, evaluate and filter compatible
		results := evalPattern(g, pattern, prefixes)
		var compatible []map[string]rdflibgo.Term
		for _, r := range results {
			if isCompatible(bindings, r) {
				compatible = append(compatible, r)
			}
		}
		return compatible
	}
}

// evalBGP evaluates a Basic Graph Pattern.
// Ported from: rdflib.plugins.sparql.evaluate.evalBGP
func evalBGP(g *rdflibgo.Graph, triples []Triple, bindings map[string]rdflibgo.Term, prefixes map[string]string) []map[string]rdflibgo.Term {
	if len(triples) == 0 {
		return []map[string]rdflibgo.Term{bindings}
	}

	tp := triples[0]
	rest := triples[1:]

	// Resolve subject/predicate/object (substitute bound variables)
	var subj rdflibgo.Subject
	var pred *rdflibgo.URIRef
	var obj rdflibgo.Term

	sVal := resolvePatternTerm(tp.Subject, bindings, prefixes)
	if sVal != nil {
		if s, ok := sVal.(rdflibgo.Subject); ok {
			subj = s
		}
	}

	pVal := resolvePatternTerm(tp.Predicate, bindings, prefixes)
	if pVal != nil {
		if u, ok := pVal.(rdflibgo.URIRef); ok {
			pred = &u
		}
	}

	oVal := resolvePatternTerm(tp.Object, bindings, prefixes)
	obj = oVal

	var results []map[string]rdflibgo.Term
	g.Triples(subj, pred, obj)(func(t rdflibgo.Triple) bool {
		nb := copyBindings(bindings)

		// Bind unbound variables
		if strings.HasPrefix(tp.Subject, "?") {
			v := tp.Subject[1:]
			if _, bound := nb[v]; !bound {
				nb[v] = t.Subject
			}
		}
		if strings.HasPrefix(tp.Predicate, "?") {
			v := tp.Predicate[1:]
			if _, bound := nb[v]; !bound {
				nb[v] = t.Predicate
			}
		}
		if strings.HasPrefix(tp.Object, "?") {
			v := tp.Object[1:]
			if _, bound := nb[v]; !bound {
				nb[v] = t.Object
			}
		}

		// Recurse on remaining triples
		subResults := evalBGP(g, rest, nb, prefixes)
		results = append(results, subResults...)
		return true
	})

	return results
}

func resolvePatternTerm(s string, bindings map[string]rdflibgo.Term, prefixes map[string]string) rdflibgo.Term {
	if strings.HasPrefix(s, "?") {
		v := s[1:]
		if val, ok := bindings[v]; ok {
			return val
		}
		return nil // unbound variable = wildcard
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return rdflibgo.NewURIRefUnsafe(s[1 : len(s)-1])
	}
	if s == "true" {
		return rdflibgo.NewLiteral(true)
	}
	if s == "false" {
		return rdflibgo.NewLiteral(false)
	}
	if strings.HasPrefix(s, "\"") || strings.HasPrefix(s, "'") {
		return parseLiteralString(s)
	}
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9' || s[0] == '+' || s[0] == '-') {
		if strings.ContainsAny(s, "eE") {
			return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDDouble))
		}
		if strings.Contains(s, ".") {
			return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
		}
		return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		prefix := s[:idx]
		local := s[idx+1:]
		if ns, ok := prefixes[prefix]; ok {
			return rdflibgo.NewURIRefUnsafe(ns + local)
		}
	}
	return nil
}

// evalExpr evaluates a SPARQL expression.
// Ported from: rdflib.plugins.sparql.operators
func evalExpr(expr Expr, bindings map[string]rdflibgo.Term, prefixes map[string]string) rdflibgo.Term {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *VarExpr:
		return bindings[e.Name]

	case *LiteralExpr:
		return e.Value

	case *IRIExpr:
		return rdflibgo.NewURIRefUnsafe(e.Value)

	case *BinaryExpr:
		left := evalExpr(e.Left, bindings, prefixes)
		right := evalExpr(e.Right, bindings, prefixes)
		return evalBinaryOp(e.Op, left, right)

	case *UnaryExpr:
		arg := evalExpr(e.Arg, bindings, prefixes)
		return evalUnaryOp(e.Op, arg)

	case *FuncExpr:
		return evalFunc(e.Name, e.Args, bindings, prefixes)
	}

	return nil
}

func evalBinaryOp(op string, left, right rdflibgo.Term) rdflibgo.Term {
	switch op {
	case "=":
		if left == nil || right == nil {
			return rdflibgo.NewLiteral(false)
		}
		return rdflibgo.NewLiteral(left.N3() == right.N3())
	case "!=":
		if left == nil || right == nil {
			return rdflibgo.NewLiteral(true)
		}
		return rdflibgo.NewLiteral(left.N3() != right.N3())
	case "<", ">", "<=", ">=":
		c := compareTermValues(left, right)
		switch op {
		case "<":
			return rdflibgo.NewLiteral(c < 0)
		case ">":
			return rdflibgo.NewLiteral(c > 0)
		case "<=":
			return rdflibgo.NewLiteral(c <= 0)
		case ">=":
			return rdflibgo.NewLiteral(c >= 0)
		}
	case "&&":
		return rdflibgo.NewLiteral(effectiveBooleanValue(left) && effectiveBooleanValue(right))
	case "||":
		return rdflibgo.NewLiteral(effectiveBooleanValue(left) || effectiveBooleanValue(right))
	case "+", "-", "*", "/":
		lf := toFloat64(left)
		rf := toFloat64(right)
		var result float64
		switch op {
		case "+":
			result = lf + rf
		case "-":
			result = lf - rf
		case "*":
			result = lf * rf
		case "/":
			if rf == 0 {
				return nil
			}
			result = lf / rf
		}
		if isIntegral(left) && isIntegral(right) && op != "/" {
			return rdflibgo.NewLiteral(int(result))
		}
		return rdflibgo.NewLiteral(result)
	}
	return nil
}

func evalUnaryOp(op string, arg rdflibgo.Term) rdflibgo.Term {
	switch op {
	case "!":
		return rdflibgo.NewLiteral(!effectiveBooleanValue(arg))
	case "-":
		f := toFloat64(arg)
		if isIntegral(arg) {
			return rdflibgo.NewLiteral(int(-f))
		}
		return rdflibgo.NewLiteral(-f)
	}
	return nil
}

// --- Binding helpers ---

func mergeBindings(a, b map[string]rdflibgo.Term) map[string]rdflibgo.Term {
	result := make(map[string]rdflibgo.Term)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

func copyBindings(b map[string]rdflibgo.Term) map[string]rdflibgo.Term {
	result := make(map[string]rdflibgo.Term, len(b))
	for k, v := range b {
		result[k] = v
	}
	return result
}

func isCompatible(a, b map[string]rdflibgo.Term) bool {
	for k, va := range a {
		if vb, ok := b[k]; ok {
			if va.N3() != vb.N3() {
				return false
			}
		}
	}
	return true
}

func filterByBindings(solutions []map[string]rdflibgo.Term, bindings map[string]rdflibgo.Term) []map[string]rdflibgo.Term {
	var result []map[string]rdflibgo.Term
	for _, s := range solutions {
		match := true
		for k, v := range bindings {
			if sv, ok := s[k]; ok {
				if sv.N3() != v.N3() {
					match = false
					break
				}
			}
		}
		if match {
			result = append(result, s)
		}
	}
	return result
}

func solutionKey(s map[string]rdflibgo.Term, vars []string) string {
	var parts []string
	if vars == nil {
		for k, v := range s {
			parts = append(parts, k+"="+v.N3())
		}
		slices.Sort(parts)
	} else {
		for _, v := range vars {
			if val, ok := s[v]; ok {
				parts = append(parts, v+"="+val.N3())
			}
		}
	}
	return strings.Join(parts, "|")
}
