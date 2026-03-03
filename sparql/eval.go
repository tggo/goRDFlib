package sparql

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// baseURIKey is used to pass the query base URI through the prefixes map.
const baseURIKey = "__base__"

// EvalQuery evaluates a parsed SPARQL query against a graph.
func EvalQuery(g *rdflibgo.Graph, q *ParsedQuery, initBindings map[string]rdflibgo.Term) (*Result, error) {
	if q.Prefixes == nil {
		q.Prefixes = make(map[string]string)
	}
	if q.BaseURI != "" {
		q.Prefixes[baseURIKey] = q.BaseURI
	}
	// Set query start time so NOW() returns a stable value per SPARQL spec.
	if _, ok := q.Prefixes[queryStartTimeKey]; !ok {
		q.Prefixes[queryStartTimeKey] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}
	solutions := evalPattern(g, q.Where, q.Prefixes, q.NamedGraphs)

	if initBindings != nil {
		solutions = filterByBindings(solutions, initBindings)
	}

	switch q.Type {
	case "SELECT":
		return evalSelect(g, q, solutions)
	case "ASK":
		return &Result{Type: "ASK", AskResult: len(solutions) > 0}, nil
	case "CONSTRUCT":
		// CONSTRUCT WHERE shorthand: derive template from WHERE BGP
		if len(q.Construct) == 0 && q.Where != nil {
			q.Construct = extractTemplateFromPattern(q.Where)
		}
		return evalConstruct(g, q, solutions)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.Type)
	}
}

func evalSelect(g *rdflibgo.Graph, q *ParsedQuery, solutions []map[string]rdflibgo.Term) (*Result, error) {
	// Aggregation
	if len(q.GroupBy) > 0 || hasAggregates(q) {
		solutions = evalAggregation(q, solutions)
	}

	// Project expressions
	if len(q.ProjectExprs) > 0 {
		for i, s := range solutions {
			ns := copyBindings(s)
			for _, pe := range q.ProjectExprs {
				val := evalExpr(pe.Expr, ns, q.Prefixes)
				if val != nil {
					ns[pe.Var] = val
				}
			}
			solutions[i] = ns
		}
	}

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

// --- Aggregation ---

func hasAggregates(q *ParsedQuery) bool {
	for _, pe := range q.ProjectExprs {
		if containsAggregate(pe.Expr) {
			return true
		}
	}
	return false
}

func containsAggregate(expr Expr) bool {
	switch e := expr.(type) {
	case *FuncExpr:
		if isAggregateFuncName(e.Name) {
			return true
		}
		for _, a := range e.Args {
			if containsAggregate(a) {
				return true
			}
		}
	case *BinaryExpr:
		return containsAggregate(e.Left) || containsAggregate(e.Right)
	case *UnaryExpr:
		return containsAggregate(e.Arg)
	}
	return false
}

func isAggregateFuncName(name string) bool {
	switch name {
	case "COUNT", "SUM", "AVG", "MIN", "MAX", "GROUP_CONCAT", "SAMPLE":
		return true
	}
	return false
}

func evalAggregation(q *ParsedQuery, solutions []map[string]rdflibgo.Term) []map[string]rdflibgo.Term {
	type group struct {
		keyBinds map[string]rdflibgo.Term
		members  []map[string]rdflibgo.Term
	}

	groups := make(map[string]*group)
	var order []string

	for _, s := range solutions {
		var keyParts []string
		keyBinds := make(map[string]rdflibgo.Term)
		for i, gExpr := range q.GroupBy {
			val := evalExpr(gExpr, s, q.Prefixes)
			if val != nil {
				keyParts = append(keyParts, val.N3())
			} else {
				keyParts = append(keyParts, "")
			}
			if ve, ok := gExpr.(*VarExpr); ok && val != nil {
				keyBinds[ve.Name] = val
			}
			// Check if this GROUP BY expr has an AS alias
			if val != nil && i < len(q.GroupByAliases) && q.GroupByAliases[i] != "" {
				keyBinds[q.GroupByAliases[i]] = val
			}
		}
		k := strings.Join(keyParts, "|")
		if _, exists := groups[k]; !exists {
			groups[k] = &group{keyBinds: keyBinds}
			order = append(order, k)
		}
		groups[k].members = append(groups[k].members, s)
	}

	// Empty input with aggregates but no explicit GROUP BY → one empty group
	if len(groups) == 0 && hasAggregates(q) && len(q.GroupBy) == 0 {
		groups[""] = &group{keyBinds: map[string]rdflibgo.Term{}}
		order = append(order, "")
	}

	var result []map[string]rdflibgo.Term
	for _, k := range order {
		grp := groups[k]
		row := copyBindings(grp.keyBinds)

		for _, pe := range q.ProjectExprs {
			val := evalAggExpr(pe.Expr, grp.members, q.Prefixes)
			if val != nil {
				row[pe.Var] = val
			}
		}

		if q.Having != nil {
			hval := evalAggExpr(q.Having, grp.members, q.Prefixes)
			if !effectiveBooleanValue(hval) {
				continue
			}
		}

		result = append(result, row)
	}
	return result
}

func evalAggExpr(expr Expr, group []map[string]rdflibgo.Term, prefixes map[string]string) rdflibgo.Term {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *FuncExpr:
		if isAggregateFuncName(e.Name) {
			return evalAggregate(e, group, prefixes)
		}
		if len(group) > 0 {
			return evalFunc(e.Name, e.Args, group[0], prefixes)
		}
		return nil
	case *BinaryExpr:
		left := evalAggExpr(e.Left, group, prefixes)
		right := evalAggExpr(e.Right, group, prefixes)
		return evalBinaryOp(e.Op, left, right)
	case *UnaryExpr:
		arg := evalAggExpr(e.Arg, group, prefixes)
		return evalUnaryOp(e.Op, arg)
	case *VarExpr:
		if len(group) > 0 {
			return group[0][e.Name]
		}
		return nil
	case *LiteralExpr:
		return e.Value
	case *IRIExpr:
		return rdflibgo.NewURIRefUnsafe(e.Value)
	}
	return nil
}

func evalAggregate(fe *FuncExpr, group []map[string]rdflibgo.Term, prefixes map[string]string) rdflibgo.Term {
	var vals []rdflibgo.Term
	hasError := false
	for _, s := range group {
		if fe.Star {
			vals = append(vals, rdflibgo.NewLiteral(1))
		} else if len(fe.Args) > 0 {
			v := evalExpr(fe.Args[0], s, prefixes)
			if v != nil {
				vals = append(vals, v)
				// For numeric aggregates, non-numeric values are errors
				if fe.Name == "SUM" || fe.Name == "AVG" {
					if !isNumericTerm(v) {
						hasError = true
					}
				}
			}
		}
	}

	if fe.Distinct {
		if fe.Star {
			// For COUNT(DISTINCT *), deduplicate based on full solution rows
			seen := make(map[string]bool)
			count := 0
			for _, s := range group {
				k := solutionKey(s, nil)
				if !seen[k] {
					seen[k] = true
					count++
				}
			}
			if fe.Name == "COUNT" {
				return rdflibgo.NewLiteral(count, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
			}
		}
		seen := make(map[string]bool)
		var unique []rdflibgo.Term
		for _, v := range vals {
			k := v.N3()
			if !seen[k] {
				seen[k] = true
				unique = append(unique, v)
			}
		}
		vals = unique
	}

	switch fe.Name {
	case "COUNT":
		return rdflibgo.NewLiteral(len(vals), rdflibgo.WithDatatype(rdflibgo.XSDInteger))
	case "SUM":
		if hasError {
			return nil
		}
		sum := 0.0
		allInt := true
		hasDecimal := false
		for _, v := range vals {
			sum += toFloat64(v)
			if !isIntegral(v) {
				allInt = false
			}
			if l, ok := v.(rdflibgo.Literal); ok && l.Datatype() == rdflibgo.XSDDecimal {
				hasDecimal = true
			}
		}
		if allInt {
			// Check for int64 overflow before casting
			if sum > float64(math.MaxInt64) || sum < float64(math.MinInt64) || math.IsNaN(sum) || math.IsInf(sum, 0) {
				return rdflibgo.NewLiteral(sum)
			}
			return rdflibgo.NewLiteral(int64(sum), rdflibgo.WithDatatype(rdflibgo.XSDInteger))
		}
		if hasDecimal {
			return rdflibgo.NewLiteral(formatDecimal(sum), rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
		}
		return rdflibgo.NewLiteral(sum)
	case "AVG":
		if hasError {
			return nil
		}
		if len(vals) == 0 {
			return rdflibgo.NewLiteral(0, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
		}
		sum := 0.0
		hasDecimal := false
		for _, v := range vals {
			sum += toFloat64(v)
			if l, ok := v.(rdflibgo.Literal); ok && l.Datatype() == rdflibgo.XSDDecimal {
				hasDecimal = true
			}
		}
		avg := sum / float64(len(vals))
		if hasDecimal {
			return rdflibgo.NewLiteral(formatDecimal(avg), rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
		}
		return rdflibgo.NewLiteral(avg)
	case "MIN":
		if len(vals) == 0 {
			return nil
		}
		m := vals[0]
		for _, v := range vals[1:] {
			if compareTermValues(v, m) < 0 {
				m = v
			}
		}
		return m
	case "MAX":
		if len(vals) == 0 {
			return nil
		}
		m := vals[0]
		for _, v := range vals[1:] {
			if compareTermValues(v, m) > 0 {
				m = v
			}
		}
		return m
	case "GROUP_CONCAT":
		var parts []string
		for _, v := range vals {
			parts = append(parts, termString(v))
		}
		return rdflibgo.NewLiteral(strings.Join(parts, fe.Separator))
	case "SAMPLE":
		if len(vals) > 0 {
			return vals[0]
		}
		return nil
	}
	return nil
}

// --- CONSTRUCT ---

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

func extractTemplateFromPattern(p Pattern) []TripleTemplate {
	switch pat := p.(type) {
	case *BGP:
		var tmpl []TripleTemplate
		for _, t := range pat.Triples {
			tmpl = append(tmpl, TripleTemplate{Subject: t.Subject, Predicate: t.Predicate, Object: t.Object})
		}
		return tmpl
	case *JoinPattern:
		return append(extractTemplateFromPattern(pat.Left), extractTemplateFromPattern(pat.Right)...)
	}
	return nil
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
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		iri := s[1 : len(s)-1]
		// Resolve relative IRI against base
		if base, ok := prefixes[baseURIKey]; ok && iri != "" && !strings.Contains(iri, "://") {
			iri = base + iri
		}
		return rdflibgo.NewURIRefUnsafe(iri)
	}
	if strings.HasPrefix(s, "_:") {
		label := s[2:]
		if scope, ok := prefixes["__bnode_scope__"]; ok {
			label = scope + label
		}
		return rdflibgo.NewBNode(label)
	}
	if strings.HasPrefix(s, "\"") || strings.HasPrefix(s, "'") {
		return parseLiteralString(s)
	}
	if s == "true" {
		return rdflibgo.NewLiteral(true)
	}
	if s == "false" {
		return rdflibgo.NewLiteral(false)
	}
	// Numeric
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
		local = unescapePNLocal(local)
		if ns, ok := prefixes[prefix]; ok {
			return rdflibgo.NewURIRefUnsafe(ns + local)
		}
	}
	return nil
}

// --- Pattern evaluation ---

func evalPattern(g *rdflibgo.Graph, pattern Pattern, prefixes map[string]string, namedGraphs map[string]*rdflibgo.Graph) []map[string]rdflibgo.Term {
	if pattern == nil {
		return []map[string]rdflibgo.Term{{}}
	}

	switch p := pattern.(type) {
	case *BGP:
		return evalBGP(g, p.Triples, map[string]rdflibgo.Term{}, prefixes)

	case *JoinPattern:
		left := evalPattern(g, p.Left, prefixes, namedGraphs)
		var result []map[string]rdflibgo.Term
		for _, lb := range left {
			right := evalPatternWithBindings(g, p.Right, lb, prefixes, namedGraphs)
			for _, rb := range right {
				result = append(result, mergeBindings(lb, rb))
			}
		}
		return result

	case *OptionalPattern:
		main := evalPattern(g, p.Main, prefixes, namedGraphs)
		var result []map[string]rdflibgo.Term
		for _, mb := range main {
			opt := evalPatternWithBindings(g, p.Optional, mb, prefixes, namedGraphs)
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
		left := evalPattern(g, p.Left, prefixes, namedGraphs)
		right := evalPattern(g, p.Right, prefixes, namedGraphs)
		return append(left, right...)

	case *FilterPattern:
		inner := evalPattern(g, p.Pattern, prefixes, namedGraphs)
		var result []map[string]rdflibgo.Term
		for _, b := range inner {
			var val rdflibgo.Term
			if containsExists(p.Expr) {
				val = evalExprWithGraph(p.Expr, b, prefixes, g, namedGraphs)
			} else {
				val = evalExpr(p.Expr, b, prefixes)
			}
			if effectiveBooleanValue(val) {
				result = append(result, b)
			}
		}
		return result

	case *BindPattern:
		inner := evalPattern(g, p.Pattern, prefixes, namedGraphs)
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
		base, _ := prefixes[baseURIKey]
		for _, row := range p.Values {
			b := make(map[string]rdflibgo.Term)
			for i, v := range p.Vars {
				if i < len(row) && row[i] != nil {
					val := row[i]
					// Resolve relative IRIs in VALUES against base
					if u, ok := val.(rdflibgo.URIRef); ok && base != "" && !strings.Contains(u.Value(), ":") {
						val = rdflibgo.NewURIRefUnsafe(base + u.Value())
					}
					b[v] = val
				}
			}
			result = append(result, b)
		}
		return result

	case *MinusPattern:
		left := evalPattern(g, p.Left, prefixes, namedGraphs)
		right := evalPattern(g, p.Right, prefixes, namedGraphs)
		var result []map[string]rdflibgo.Term
		for _, lb := range left {
			excluded := false
			for _, rb := range right {
				if minusCompatible(lb, rb) {
					excluded = true
					break
				}
			}
			if !excluded {
				result = append(result, lb)
			}
		}
		return result

	case *GraphPattern:
		return evalGraphPattern(g, p, prefixes, namedGraphs)

	case *SubqueryPattern:
		subQ := *p.Query // shallow copy; deep-copy mutable fields below
		// Deep-copy NamedGraphs to avoid mutating the original AST
		if namedGraphs != nil {
			ng := make(map[string]*rdflibgo.Graph, len(namedGraphs))
			for k, v := range namedGraphs {
				ng[k] = v
			}
			subQ.NamedGraphs = ng
		}
		subResult, err := EvalQuery(g, &subQ, nil)
		if err != nil {
			return nil
		}
		if subResult.Type == "SELECT" {
			return subResult.Bindings
		}
		return nil
	}

	return []map[string]rdflibgo.Term{{}}
}

func evalGraphPattern(g *rdflibgo.Graph, gp *GraphPattern, prefixes map[string]string, namedGraphs map[string]*rdflibgo.Graph) []map[string]rdflibgo.Term {
	if namedGraphs == nil {
		// No named graphs available — evaluate against default graph
		return evalPattern(g, gp.Pattern, prefixes, namedGraphs)
	}

	graphName := gp.Name // e.g., "?g" or "<http://...>"
	isVar := strings.HasPrefix(graphName, "?")

	if isVar {
		// GRAPH ?g { ... } — iterate over all named graphs
		varName := graphName[1:]
		var results []map[string]rdflibgo.Term
		for name, namedG := range namedGraphs {
			graphURI := rdflibgo.NewURIRefUnsafe(name)
			// Push graph binding into inner pattern evaluation
			initBindings := map[string]rdflibgo.Term{varName: graphURI}
			inner := evalPatternWithBindings(namedG, gp.Pattern, initBindings, prefixes, namedGraphs)
			for _, b := range inner {
				nb := copyBindings(b)
				nb[varName] = graphURI
				results = append(results, nb)
			}
		}
		return results
	}

	// GRAPH <specific-uri> { ... }
	resolved := resolvePatternTerm(graphName, nil, prefixes)
	if resolved == nil {
		return nil
	}
	graphIRI := ""
	if u, ok := resolved.(rdflibgo.URIRef); ok {
		graphIRI = u.Value()
	} else {
		graphIRI = resolved.String()
	}

	if namedG, ok := namedGraphs[graphIRI]; ok {
		return evalPattern(namedG, gp.Pattern, prefixes, namedGraphs)
	}
	return nil
}

func minusCompatible(a, b map[string]rdflibgo.Term) bool {
	shared := false
	for k, va := range a {
		if vb, ok := b[k]; ok {
			shared = true
			if va.N3() != vb.N3() {
				return false
			}
		}
	}
	return shared
}

func evalPatternWithBindings(g *rdflibgo.Graph, pattern Pattern, bindings map[string]rdflibgo.Term, prefixes map[string]string, namedGraphs map[string]*rdflibgo.Graph) []map[string]rdflibgo.Term {
	switch p := pattern.(type) {
	case *BGP:
		return evalBGP(g, p.Triples, bindings, prefixes)
	default:
		results := evalPattern(g, pattern, prefixes, namedGraphs)
		var compatible []map[string]rdflibgo.Term
		for _, r := range results {
			if isCompatible(bindings, r) {
				compatible = append(compatible, r)
			}
		}
		return compatible
	}
}

// --- BGP evaluation ---

func evalBGP(g *rdflibgo.Graph, triples []Triple, bindings map[string]rdflibgo.Term, prefixes map[string]string) []map[string]rdflibgo.Term {
	if len(triples) == 0 {
		return []map[string]rdflibgo.Term{bindings}
	}

	tp := triples[0]
	rest := triples[1:]

	if tp.PredicatePath != nil {
		return evalPathTriple(g, tp, rest, bindings, prefixes)
	}

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

		subResults := evalBGP(g, rest, nb, prefixes)
		results = append(results, subResults...)
		return true
	})

	return results
}

func evalPathTriple(g *rdflibgo.Graph, tp Triple, rest []Triple, bindings map[string]rdflibgo.Term, prefixes map[string]string) []map[string]rdflibgo.Term {
	gg := (*graph.Graph)(g)

	var subj term.Subject
	sVal := resolvePatternTerm(tp.Subject, bindings, prefixes)
	if sVal != nil {
		if s, ok := sVal.(term.Subject); ok {
			subj = s
		}
	}

	var obj term.Term
	oVal := resolvePatternTerm(tp.Object, bindings, prefixes)
	obj = oVal

	var results []map[string]rdflibgo.Term
	tp.PredicatePath.Eval(gg, subj, obj)(func(s, o term.Term) bool {
		nb := copyBindings(bindings)

		if strings.HasPrefix(tp.Subject, "?") {
			v := tp.Subject[1:]
			if _, bound := nb[v]; !bound {
				nb[v] = s
			}
		}
		if strings.HasPrefix(tp.Object, "?") {
			v := tp.Object[1:]
			if _, bound := nb[v]; !bound {
				nb[v] = o
			}
		}

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
		return nil
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		iri := s[1 : len(s)-1]
		// Resolve relative IRI against base
		if !strings.Contains(iri, ":") {
			if base, ok := prefixes[baseURIKey]; ok {
				iri = base + iri
			}
		}
		return rdflibgo.NewURIRefUnsafe(iri)
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

// --- Expression evaluation ---

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
		iri := e.Value
		if !strings.Contains(iri, ":") {
			if base, ok := prefixes[baseURIKey]; ok {
				iri = base + iri
			}
		}
		return rdflibgo.NewURIRefUnsafe(iri)
	case *BinaryExpr:
		left := evalExpr(e.Left, bindings, prefixes)
		right := evalExpr(e.Right, bindings, prefixes)
		return evalBinaryOp(e.Op, left, right)
	case *UnaryExpr:
		arg := evalExpr(e.Arg, bindings, prefixes)
		return evalUnaryOp(e.Op, arg)
	case *FuncExpr:
		return evalFunc(e.Name, e.Args, bindings, prefixes)
	case *ExistsExpr:
		return nil // needs graph; handled via evalExprWithGraph
	}

	return nil
}

func evalExprWithGraph(expr Expr, bindings map[string]rdflibgo.Term, prefixes map[string]string, g *rdflibgo.Graph, namedGraphs map[string]*rdflibgo.Graph) rdflibgo.Term {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ExistsExpr:
		results := evalPatternWithBindings(g, e.Pattern, bindings, prefixes, namedGraphs)
		exists := len(results) > 0
		if e.Not {
			exists = !exists
		}
		return rdflibgo.NewLiteral(exists)
	case *BinaryExpr:
		left := evalExprWithGraph(e.Left, bindings, prefixes, g, namedGraphs)
		right := evalExprWithGraph(e.Right, bindings, prefixes, g, namedGraphs)
		return evalBinaryOp(e.Op, left, right)
	case *UnaryExpr:
		arg := evalExprWithGraph(e.Arg, bindings, prefixes, g, namedGraphs)
		return evalUnaryOp(e.Op, arg)
	default:
		return evalExpr(expr, bindings, prefixes)
	}
}

func containsExists(expr Expr) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *ExistsExpr:
		return true
	case *BinaryExpr:
		return containsExists(e.Left) || containsExists(e.Right)
	case *UnaryExpr:
		return containsExists(e.Arg)
	case *FuncExpr:
		for _, a := range e.Args {
			if containsExists(a) {
				return true
			}
		}
	}
	return false
}

func evalBinaryOp(op string, left, right rdflibgo.Term) rdflibgo.Term {
	switch op {
	case "=":
		if left == nil || right == nil {
			return rdflibgo.NewLiteral(false)
		}
		return rdflibgo.NewLiteral(termValuesEqual(left, right))
	case "!=":
		if left == nil || right == nil {
			return rdflibgo.NewLiteral(true)
		}
		return rdflibgo.NewLiteral(!termValuesEqual(left, right))
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
		if left == nil || right == nil {
			return nil
		}
		// Both must be numeric
		if !isNumericTerm(left) || !isNumericTerm(right) {
			return nil
		}
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
			return rdflibgo.NewLiteral(int64(result))
		}
		if isDecimal(left) || isDecimal(right) {
			return rdflibgo.NewLiteral(formatDecimal(result), rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
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
			return rdflibgo.NewLiteral(int64(-f))
		}
		return rdflibgo.NewLiteral(-f)
	}
	return nil
}

// --- Binding helpers ---

func mergeBindings(a, b map[string]rdflibgo.Term) map[string]rdflibgo.Term {
	result := make(map[string]rdflibgo.Term, len(a)+len(b))
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

func termValuesEqual(a, b rdflibgo.Term) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	la, aIsLit := a.(rdflibgo.Literal)
	lb, bIsLit := b.(rdflibgo.Literal)

	if aIsLit && bIsLit {
		fa, errA := strconv.ParseFloat(la.Lexical(), 64)
		fb, errB := strconv.ParseFloat(lb.Lexical(), 64)
		if errA == nil && errB == nil && isNumericDatatype(la.Datatype()) && isNumericDatatype(lb.Datatype()) {
			// NaN != NaN per SPARQL/XSD spec
			if math.IsNaN(fa) || math.IsNaN(fb) {
				return false
			}
			return fa == fb
		}
		if la.Language() != "" || lb.Language() != "" {
			return la.Lexical() == lb.Lexical() && la.Language() == lb.Language()
		}
		return la.Lexical() == lb.Lexical()
	}

	return a.N3() == b.N3()
}

func isNumericTerm(t rdflibgo.Term) bool {
	if l, ok := t.(rdflibgo.Literal); ok {
		return isNumericDatatype(l.Datatype())
	}
	return false
}

func isDecimal(t rdflibgo.Term) bool {
	if l, ok := t.(rdflibgo.Literal); ok {
		return l.Datatype() == rdflibgo.XSDDecimal
	}
	return false
}

func formatDecimal(f float64) string {
	// Use limited precision to avoid float64 artifacts like 11.100000000000001
	s := strconv.FormatFloat(f, 'f', 10, 64)
	// Trim trailing zeros but keep at least one decimal place
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		if strings.HasSuffix(s, ".") {
			s += "0"
		}
	} else {
		s += ".0"
	}
	return s
}

func isNumericDatatype(dt rdflibgo.URIRef) bool {
	return dt == rdflibgo.XSDInteger || dt == rdflibgo.XSDInt || dt == rdflibgo.XSDLong ||
		dt == rdflibgo.XSDFloat || dt == rdflibgo.XSDDouble || dt == rdflibgo.XSDDecimal
}
