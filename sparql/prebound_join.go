package sparql

import (
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Joins on the pre-bound path (evalPatternPreBound).
//
// The right side of a Join is evaluated with the pre-bound constants only, so
// that an expression in it cannot see the left row (SPARQL 1.1 §18.2.1). A
// right side without expressions can take the left row as constants instead,
// and one that does not depend on the row need not be evaluated once per row.

// expressionFree reports whether p is a BGP, or a UNION or join of such
// patterns: a pattern no FILTER, BIND, OPTIONAL condition or subquery can
// observe a variable of, so that substituting values into it only restricts
// its solutions.
func expressionFree(p Pattern) bool {
	switch p := p.(type) {
	case *BGP:
		return true
	case *UnionPattern:
		return expressionFree(p.Left) && expressionFree(p.Right)
	case *JoinPattern:
		return expressionFree(p.Left) && expressionFree(p.Right)
	}
	return false
}

// rightSide is what a pre-bound join needs to know about its right pattern,
// worked out once per evaluation (evalCtx.rightSideOf): a join under OPTIONAL
// or EXISTS is evaluated again for every row of the enclosing pattern, and
// walking the pattern for its variables each time was a map per evaluation.
type rightSide struct {
	// free is expressionFree(p).
	free bool
	// vars holds every variable p mentions, wherever it stands: subject,
	// predicate or object, an endpoint of a property path, or inside a triple
	// term at any depth. A missed variable is not a wrong answer (the join
	// then takes the path that joins on compatibility), only a slower one,
	// but a complete list is what lets the substitution happen at all.
	vars []string
	// set is vars as a set, kept only when it is too long to scan.
	set map[string]struct{}
}

// maxScannedVars is the length up to which a row is checked against vars by
// scanning it: a handful of string compares beat hashing every key of a row.
const maxScannedVars = 8

// rightSideOf returns the rightSide of p, remembered on ec by pattern.
func (ec *evalCtx) rightSideOf(p Pattern) *rightSide {
	if ec != nil {
		if rs, ok := ec.rights[p]; ok {
			return rs
		}
	}
	rs := &rightSide{free: expressionFree(p)}
	if rs.free {
		seen := map[string]struct{}{}
		collectRightVars(p, func(v string) {
			if _, dup := seen[v]; !dup {
				seen[v] = struct{}{}
				rs.vars = append(rs.vars, v)
			}
		})
		if len(rs.vars) > maxScannedVars {
			rs.set = seen
		}
	}
	if ec != nil {
		if ec.rights == nil {
			ec.rights = make(map[Pattern]*rightSide)
		}
		ec.rights[p] = rs
	}
	return rs
}

// collectRightVars calls add for every variable of an expression-free
// pattern (a BGP, or a UNION or join of them).
func collectRightVars(p Pattern, add func(string)) {
	switch p := p.(type) {
	case *BGP:
		for _, t := range p.Triples {
			collectTermVars(t.Subject, add)
			collectTermVars(t.Predicate, add)
			collectTermVars(t.Object, add)
		}
	case *UnionPattern:
		collectRightVars(p.Left, add)
		collectRightVars(p.Right, add)
	case *JoinPattern:
		collectRightVars(p.Left, add)
		collectRightVars(p.Right, add)
	}
}

// collectTermVars calls add for the variable s names, or for every variable
// of the triple term s, nested ones included.
func collectTermVars(s string, add func(string)) {
	switch {
	case strings.HasPrefix(s, "?"):
		add(s[1:])
	case strings.HasPrefix(s, "<<( ") && strings.HasSuffix(s, " )>>") && len(s) >= 8:
		for _, part := range splitTripleTermParts(s[4 : len(s)-4]) {
			collectTermVars(part, add)
		}
	}
}

// bindsAVariableOf reports whether a row of left binds a variable that the
// expression-free pattern described by rs mentions and pre does not already
// bind.
func (rs *rightSide) bindsAVariableOf(left []map[string]rdflibgo.Term, pre map[string]rdflibgo.Term) bool {
	if len(rs.vars) == 0 {
		return false
	}
	for _, lb := range left {
		if rs.set != nil {
			for k := range lb {
				if _, ok := rs.set[k]; ok && !preBinds(pre, k) {
					return true
				}
			}
			continue
		}
		for _, v := range rs.vars {
			if _, ok := lb[v]; ok && !preBinds(pre, v) {
				return true
			}
		}
	}
	return false
}

func preBinds(pre map[string]rdflibgo.Term, v string) bool {
	_, ok := pre[v]
	return ok
}

// xsdNamespace is the namespace of the XSD constructor functions (casts),
// which are deterministic.
const xsdNamespace = "http://www.w3.org/2001/XMLSchema#"

// callsVolatileFunction reports whether evaluating p twice may give different
// solutions: it calls RAND, UUID, STRUUID or BNODE, or an extension function,
// whose results the engine cannot vouch for. A pattern it does not know is
// assumed to.
func callsVolatileFunction(p Pattern) bool {
	return volatileWalker{seen: map[*ParsedQuery]bool{}}.pattern(p)
}

// volatileWalker walks patterns, expressions and subqueries; seen guards
// against a cyclic subquery graph assembled by hand.
type volatileWalker struct {
	seen map[*ParsedQuery]bool
}

func (w volatileWalker) pattern(p Pattern) bool {
	switch x := p.(type) {
	case nil, *BGP, *ValuesPattern:
		return false
	case *JoinPattern:
		return w.pattern(x.Left) || w.pattern(x.Right)
	case *OptionalPattern:
		return w.pattern(x.Main) || w.pattern(x.Optional)
	case *UnionPattern:
		return w.pattern(x.Left) || w.pattern(x.Right)
	case *MinusPattern:
		return w.pattern(x.Left) || w.pattern(x.Right)
	case *FilterPattern:
		return w.pattern(x.Pattern) || w.expr(x.Expr)
	case *BindPattern:
		return w.pattern(x.Pattern) || w.expr(x.Expr)
	case *GraphPattern:
		return w.pattern(x.Pattern)
	case *SubqueryPattern:
		return w.query(x.Query)
	}
	return true
}

func (w volatileWalker) query(q *ParsedQuery) bool {
	if q == nil || w.seen[q] {
		return false
	}
	w.seen[q] = true
	for _, pe := range q.ProjectExprs {
		if w.expr(pe.Expr) {
			return true
		}
	}
	for _, o := range q.OrderBy {
		if w.expr(o.Expr) {
			return true
		}
	}
	for _, e := range q.GroupBy {
		if w.expr(e) {
			return true
		}
	}
	return w.expr(q.Having) || w.pattern(q.Where)
}

func (w volatileWalker) expr(e Expr) bool {
	switch x := e.(type) {
	case nil, *VarExpr, *LiteralExpr, *IRIExpr:
		return false
	case *BinaryExpr:
		return w.expr(x.Left) || w.expr(x.Right)
	case *UnaryExpr:
		return w.expr(x.Arg)
	case *ExistsExpr:
		return w.pattern(x.Pattern)
	case *FuncExpr:
		switch {
		case x.Name == "RAND", x.Name == "UUID", x.Name == "STRUUID", x.Name == "BNODE":
			return true
		case x.IRI != "" && !strings.HasPrefix(x.IRI, xsdNamespace):
			return true
		}
		for _, a := range x.Args {
			if w.expr(a) {
				return true
			}
		}
		return false
	}
	return true
}
