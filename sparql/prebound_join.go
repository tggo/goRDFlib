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

// bindsAVariableOf reports whether a row of left binds a variable that the
// expression-free pattern p mentions and pre does not already bind.
func bindsAVariableOf(left []map[string]rdflibgo.Term, p Pattern, pre map[string]rdflibgo.Term) bool {
	vars := map[string]bool{}
	var walk func(Pattern)
	walk = func(p Pattern) {
		switch p := p.(type) {
		case *BGP:
			for _, t := range p.Triples {
				for _, s := range [3]string{t.Subject, t.Predicate, t.Object} {
					if v, ok := strings.CutPrefix(s, "?"); ok {
						if _, bound := pre[v]; !bound {
							vars[v] = true
						}
					}
				}
			}
		case *UnionPattern:
			walk(p.Left)
			walk(p.Right)
		case *JoinPattern:
			walk(p.Left)
			walk(p.Right)
		}
	}
	walk(p)
	for _, lb := range left {
		for k := range lb {
			if vars[k] {
				return true
			}
		}
	}
	return false
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
