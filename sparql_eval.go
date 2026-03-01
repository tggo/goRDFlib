package rdflibgo

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// EvalQuery evaluates a parsed SPARQL query against a graph.
// Ported from: rdflib.plugins.sparql.evaluate.evalQuery
func EvalQuery(g *Graph, q *SPARQLQuery, initBindings map[string]Term) (*SPARQLResult, error) {
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
		return &SPARQLResult{Type: "ASK", AskResult: len(solutions) > 0}, nil
	case "CONSTRUCT":
		return evalConstruct(g, q, solutions)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.Type)
	}
}

func evalSelect(g *Graph, q *SPARQLQuery, solutions []map[string]Term) (*SPARQLResult, error) {
	// ORDER BY
	if len(q.OrderBy) > 0 {
		slices.SortFunc(solutions, func(a, b map[string]Term) int {
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
		var unique []map[string]Term
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
		projected := make([]map[string]Term, len(solutions))
		for i, s := range solutions {
			row := make(map[string]Term)
			for _, v := range vars {
				if val, ok := s[v]; ok {
					row[v] = val
				}
			}
			projected[i] = row
		}
		solutions = projected
	}

	return &SPARQLResult{Type: "SELECT", Vars: vars, Bindings: solutions}, nil
}

func evalConstruct(g *Graph, q *SPARQLQuery, solutions []map[string]Term) (*SPARQLResult, error) {
	result := NewGraph()
	for _, sol := range solutions {
		for _, tmpl := range q.Construct {
			s := resolveTemplateValue(tmpl.Subject, sol, q.Prefixes)
			p := resolveTemplateValue(tmpl.Predicate, sol, q.Prefixes)
			o := resolveTemplateValue(tmpl.Object, sol, q.Prefixes)
			if s == nil || p == nil || o == nil {
				continue
			}
			subj, okS := s.(Subject)
			pred, okP := p.(URIRef)
			if !okS || !okP {
				continue
			}
			result.Add(subj, pred, o)
		}
	}
	return &SPARQLResult{Type: "CONSTRUCT", Graph: result}, nil
}

func resolveTemplateValue(s string, bindings map[string]Term, prefixes map[string]string) Term {
	if strings.HasPrefix(s, "?") {
		v := s[1:]
		if val, ok := bindings[v]; ok {
			return val
		}
		return nil
	}
	return resolveTermRef(s, prefixes)
}

func resolveTermRef(s string, prefixes map[string]string) Term {
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return NewURIRefUnsafe(s[1 : len(s)-1])
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		prefix := s[:idx]
		local := s[idx+1:]
		if ns, ok := prefixes[prefix]; ok {
			return NewURIRefUnsafe(ns + local)
		}
	}
	return nil
}

// evalPattern evaluates a SPARQL pattern, returning solution bindings.
// Ported from: rdflib.plugins.sparql.evaluate.evalPart
func evalPattern(g *Graph, pattern SPARQLPattern, prefixes map[string]string) []map[string]Term {
	if pattern == nil {
		return []map[string]Term{{}}
	}

	switch p := pattern.(type) {
	case *BGP:
		return evalBGP(g, p.Triples, map[string]Term{}, prefixes)

	case *JoinPattern:
		left := evalPattern(g, p.Left, prefixes)
		var result []map[string]Term
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
		var result []map[string]Term
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
		var result []map[string]Term
		for _, b := range inner {
			val := evalExpr(p.Expr, b, prefixes)
			if effectiveBooleanValue(val) {
				result = append(result, b)
			}
		}
		return result

	case *BindPattern:
		inner := evalPattern(g, p.Pattern, prefixes)
		var result []map[string]Term
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
		var result []map[string]Term
		for _, row := range p.Values {
			b := make(map[string]Term)
			for i, v := range p.Vars {
				if i < len(row) {
					b[v] = row[i]
				}
			}
			result = append(result, b)
		}
		return result
	}

	return []map[string]Term{{}}
}

func evalPatternWithBindings(g *Graph, pattern SPARQLPattern, bindings map[string]Term, prefixes map[string]string) []map[string]Term {
	switch p := pattern.(type) {
	case *BGP:
		return evalBGP(g, p.Triples, bindings, prefixes)
	default:
		// For non-BGP patterns, evaluate and filter compatible
		results := evalPattern(g, pattern, prefixes)
		var compatible []map[string]Term
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
func evalBGP(g *Graph, triples []SPARQLTriple, bindings map[string]Term, prefixes map[string]string) []map[string]Term {
	if len(triples) == 0 {
		return []map[string]Term{bindings}
	}

	tp := triples[0]
	rest := triples[1:]

	// Resolve subject/predicate/object (substitute bound variables)
	var subj Subject
	var pred *URIRef
	var obj Term

	sVal := resolvePatternTerm(tp.Subject, bindings, prefixes)
	if sVal != nil {
		if s, ok := sVal.(Subject); ok {
			subj = s
		}
	}

	pVal := resolvePatternTerm(tp.Predicate, bindings, prefixes)
	if pVal != nil {
		if u, ok := pVal.(URIRef); ok {
			pred = &u
		}
	}

	oVal := resolvePatternTerm(tp.Object, bindings, prefixes)
	obj = oVal

	var results []map[string]Term
	g.Triples(subj, pred, obj)(func(t Triple) bool {
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

func resolvePatternTerm(s string, bindings map[string]Term, prefixes map[string]string) Term {
	if strings.HasPrefix(s, "?") {
		v := s[1:]
		if val, ok := bindings[v]; ok {
			return val
		}
		return nil // unbound variable = wildcard
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return NewURIRefUnsafe(s[1 : len(s)-1])
	}
	if s == "true" {
		return NewLiteral(true)
	}
	if s == "false" {
		return NewLiteral(false)
	}
	if strings.HasPrefix(s, "\"") || strings.HasPrefix(s, "'") {
		return parseLiteralString(s)
	}
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9' || s[0] == '+' || s[0] == '-') {
		if strings.ContainsAny(s, "eE") {
			return NewLiteral(s, WithDatatype(XSDDouble))
		}
		if strings.Contains(s, ".") {
			return NewLiteral(s, WithDatatype(XSDDecimal))
		}
		return NewLiteral(s, WithDatatype(XSDInteger))
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		prefix := s[:idx]
		local := s[idx+1:]
		if ns, ok := prefixes[prefix]; ok {
			return NewURIRefUnsafe(ns + local)
		}
	}
	return nil
}

// evalExpr evaluates a SPARQL expression.
// Ported from: rdflib.plugins.sparql.operators
func evalExpr(expr SPARQLExpr, bindings map[string]Term, prefixes map[string]string) Term {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *VarExpr:
		return bindings[e.Name]

	case *LiteralExpr:
		return e.Value

	case *IRIExpr:
		return NewURIRefUnsafe(e.Value)

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

func evalBinaryOp(op string, left, right Term) Term {
	switch op {
	case "=":
		if left == nil || right == nil {
			return NewLiteral(false)
		}
		return NewLiteral(left.N3() == right.N3())
	case "!=":
		if left == nil || right == nil {
			return NewLiteral(true)
		}
		return NewLiteral(left.N3() != right.N3())
	case "<", ">", "<=", ">=":
		c := compareTermValues(left, right)
		switch op {
		case "<":
			return NewLiteral(c < 0)
		case ">":
			return NewLiteral(c > 0)
		case "<=":
			return NewLiteral(c <= 0)
		case ">=":
			return NewLiteral(c >= 0)
		}
	case "&&":
		return NewLiteral(effectiveBooleanValue(left) && effectiveBooleanValue(right))
	case "||":
		return NewLiteral(effectiveBooleanValue(left) || effectiveBooleanValue(right))
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
			return NewLiteral(int(result))
		}
		return NewLiteral(result)
	}
	return nil
}

func evalUnaryOp(op string, arg Term) Term {
	switch op {
	case "!":
		return NewLiteral(!effectiveBooleanValue(arg))
	case "-":
		f := toFloat64(arg)
		if isIntegral(arg) {
			return NewLiteral(int(-f))
		}
		return NewLiteral(-f)
	}
	return nil
}

// evalFunc evaluates a SPARQL built-in function.
// Ported from: rdflib.plugins.sparql.operators
func evalFunc(name string, args []SPARQLExpr, bindings map[string]Term, prefixes map[string]string) Term {
	evalArgs := func() []Term {
		var vals []Term
		for _, a := range args {
			vals = append(vals, evalExpr(a, bindings, prefixes))
		}
		return vals
	}

	switch name {
	// Term constructors
	case "BOUND":
		if len(args) == 1 {
			if v, ok := args[0].(*VarExpr); ok {
				_, exists := bindings[v.Name]
				return NewLiteral(exists)
			}
		}
		return NewLiteral(false)

	case "ISIRI", "ISURI":
		vals := evalArgs()
		if len(vals) == 1 {
			_, ok := vals[0].(URIRef)
			return NewLiteral(ok)
		}
	case "ISBLANK":
		vals := evalArgs()
		if len(vals) == 1 {
			_, ok := vals[0].(BNode)
			return NewLiteral(ok)
		}
	case "ISLITERAL":
		vals := evalArgs()
		if len(vals) == 1 {
			_, ok := vals[0].(Literal)
			return NewLiteral(ok)
		}
	case "ISNUMERIC":
		vals := evalArgs()
		if len(vals) == 1 {
			if l, ok := vals[0].(Literal); ok {
				dt := l.Datatype()
				return NewLiteral(dt == XSDInteger || dt == XSDFloat || dt == XSDDouble || dt == XSDDecimal)
			}
		}
		return NewLiteral(false)

	// String functions
	case "STR":
		vals := evalArgs()
		if len(vals) == 1 && vals[0] != nil {
			return NewLiteral(vals[0].String())
		}
	case "STRLEN":
		vals := evalArgs()
		if len(vals) == 1 {
			return NewLiteral(len(termString(vals[0])))
		}
	case "SUBSTR":
		vals := evalArgs()
		if len(vals) < 1 {
			return nil
		}
		s := termString(vals[0])
		if len(vals) >= 2 {
			start := int(toFloat64(vals[1])) - 1
			if start < 0 {
				start = 0
			}
			if start >= len(s) {
				return NewLiteral("")
			}
			if len(vals) >= 3 {
				length := int(toFloat64(vals[2]))
				end := start + length
				if end > len(s) {
					end = len(s)
				}
				return NewLiteral(s[start:end])
			}
			return NewLiteral(s[start:])
		}
	case "UCASE":
		vals := evalArgs()
		if len(vals) == 1 {
			return NewLiteral(strings.ToUpper(termString(vals[0])))
		}
	case "LCASE":
		vals := evalArgs()
		if len(vals) == 1 {
			return NewLiteral(strings.ToLower(termString(vals[0])))
		}
	case "STRSTARTS":
		vals := evalArgs()
		if len(vals) == 2 {
			return NewLiteral(strings.HasPrefix(termString(vals[0]), termString(vals[1])))
		}
	case "STRENDS":
		vals := evalArgs()
		if len(vals) == 2 {
			return NewLiteral(strings.HasSuffix(termString(vals[0]), termString(vals[1])))
		}
	case "CONTAINS":
		vals := evalArgs()
		if len(vals) == 2 {
			return NewLiteral(strings.Contains(termString(vals[0]), termString(vals[1])))
		}
	case "CONCAT":
		vals := evalArgs()
		var sb strings.Builder
		for _, v := range vals {
			sb.WriteString(termString(v))
		}
		return NewLiteral(sb.String())
	case "REGEX":
		vals := evalArgs()
		if len(vals) >= 2 {
			pattern := termString(vals[1])
			flags := ""
			if len(vals) >= 3 {
				flags = termString(vals[2])
			}
			if strings.Contains(flags, "i") {
				pattern = "(?i)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return NewLiteral(false)
			}
			return NewLiteral(re.MatchString(termString(vals[0])))
		}
	case "REPLACE":
		vals := evalArgs()
		if len(vals) >= 3 {
			pattern := termString(vals[1])
			replacement := termString(vals[2])
			flags := ""
			if len(vals) >= 4 {
				flags = termString(vals[3])
			}
			if strings.Contains(flags, "i") {
				pattern = "(?i)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return vals[0]
			}
			return NewLiteral(re.ReplaceAllString(termString(vals[0]), replacement))
		}

	// Term accessors
	case "LANG":
		vals := evalArgs()
		if len(vals) == 1 {
			if l, ok := vals[0].(Literal); ok {
				return NewLiteral(l.Language())
			}
		}
		return NewLiteral("")
	case "DATATYPE":
		vals := evalArgs()
		if len(vals) == 1 {
			if l, ok := vals[0].(Literal); ok {
				return l.Datatype()
			}
		}

	// Numeric
	case "ABS":
		vals := evalArgs()
		if len(vals) == 1 {
			return NewLiteral(math.Abs(toFloat64(vals[0])))
		}
	case "ROUND":
		vals := evalArgs()
		if len(vals) == 1 {
			return NewLiteral(math.Round(toFloat64(vals[0])))
		}
	case "CEIL":
		vals := evalArgs()
		if len(vals) == 1 {
			return NewLiteral(math.Ceil(toFloat64(vals[0])))
		}
	case "FLOOR":
		vals := evalArgs()
		if len(vals) == 1 {
			return NewLiteral(math.Floor(toFloat64(vals[0])))
		}

	// Hash
	case "MD5":
		vals := evalArgs()
		if len(vals) == 1 {
			h := md5.Sum([]byte(termString(vals[0])))
			return NewLiteral(fmt.Sprintf("%x", h))
		}
	case "SHA1":
		vals := evalArgs()
		if len(vals) == 1 {
			h := sha1.Sum([]byte(termString(vals[0])))
			return NewLiteral(fmt.Sprintf("%x", h))
		}
	case "SHA256":
		vals := evalArgs()
		if len(vals) == 1 {
			h := sha256.Sum256([]byte(termString(vals[0])))
			return NewLiteral(fmt.Sprintf("%x", h))
		}

	// Conditional
	case "IF":
		if len(args) == 3 {
			cond := evalExpr(args[0], bindings, prefixes)
			if effectiveBooleanValue(cond) {
				return evalExpr(args[1], bindings, prefixes)
			}
			return evalExpr(args[2], bindings, prefixes)
		}
	case "COALESCE":
		for _, a := range args {
			v := evalExpr(a, bindings, prefixes)
			if v != nil {
				return v
			}
		}
		return nil
	case "SAMETERM":
		vals := evalArgs()
		if len(vals) == 2 && vals[0] != nil && vals[1] != nil {
			return NewLiteral(vals[0].N3() == vals[1].N3())
		}
		return NewLiteral(false)
	}

	return nil
}

// --- Helpers ---

func effectiveBooleanValue(t Term) bool {
	if t == nil {
		return false
	}
	if l, ok := t.(Literal); ok {
		switch l.Datatype() {
		case XSDBoolean:
			return l.Lexical() == "true"
		case XSDInteger, XSDInt, XSDLong:
			v, _ := strconv.ParseInt(l.Lexical(), 10, 64)
			return v != 0
		case XSDFloat, XSDDouble, XSDDecimal:
			v, _ := strconv.ParseFloat(l.Lexical(), 64)
			return v != 0
		case XSDString:
			return l.Lexical() != ""
		default:
			return l.Lexical() != ""
		}
	}
	return true
}

func toFloat64(t Term) float64 {
	if t == nil {
		return 0
	}
	if l, ok := t.(Literal); ok {
		f, _ := strconv.ParseFloat(l.Lexical(), 64)
		return f
	}
	return 0
}

func isIntegral(t Term) bool {
	if l, ok := t.(Literal); ok {
		return l.Datatype() == XSDInteger || l.Datatype() == XSDInt || l.Datatype() == XSDLong
	}
	return false
}

func termString(t Term) string {
	if t == nil {
		return ""
	}
	return t.String()
}

func compareTermValues(a, b Term) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	la, okA := a.(Literal)
	lb, okB := b.(Literal)
	if okA && okB {
		fa, errA := strconv.ParseFloat(la.Lexical(), 64)
		fb, errB := strconv.ParseFloat(lb.Lexical(), 64)
		if errA == nil && errB == nil {
			if fa < fb {
				return -1
			}
			if fa > fb {
				return 1
			}
			return 0
		}
	}
	return strings.Compare(a.N3(), b.N3())
}

func mergeBindings(a, b map[string]Term) map[string]Term {
	result := make(map[string]Term)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

func copyBindings(b map[string]Term) map[string]Term {
	result := make(map[string]Term, len(b))
	for k, v := range b {
		result[k] = v
	}
	return result
}

func isCompatible(a, b map[string]Term) bool {
	for k, va := range a {
		if vb, ok := b[k]; ok {
			if va.N3() != vb.N3() {
				return false
			}
		}
	}
	return true
}

func filterByBindings(solutions []map[string]Term, bindings map[string]Term) []map[string]Term {
	var result []map[string]Term
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

func solutionKey(s map[string]Term, vars []string) string {
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
