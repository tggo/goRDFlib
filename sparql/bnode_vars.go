package sparql

import "strings"

// blankLabelVarPrefix names the variable a blank-node label in a query
// pattern becomes. A label there is a variable that cannot be projected
// (SPARQL 1.1 §4.1.4, §18.2.1: "blank nodes ... act as variables"); before
// this rewrite every occurrence was an independent wildcard, so
// `_:b ex:p ?o . _:b ex:q ?z` was a cross product. The leading "." keeps it
// internal (isInternalVar: hidden from SELECT *), and ".bnode" makes a
// CONSTRUCT WHERE template turn an unbound one into a fresh blank node, as it
// does for []. ":" cannot appear in a generated ".bnodeN", so no collision.
const blankLabelVarPrefix = "?.bnode:"

// blankLabelsToVars rewrites every blank-node label in the triple patterns of
// p (BGPs, path endpoints, triple terms, and the patterns of EXISTS, MINUS,
// OPTIONAL, subqueries) into a variable. Templates are not patterns and are
// never passed here.
func blankLabelsToVars(p Pattern) Pattern {
	switch x := p.(type) {
	case *BGP:
		for i := range x.Triples {
			t := &x.Triples[i]
			t.Subject = blankLabelTerm(t.Subject)
			t.Predicate = blankLabelTerm(t.Predicate)
			t.Object = blankLabelTerm(t.Object)
		}
	case *JoinPattern:
		blankLabelsToVars(x.Left)
		blankLabelsToVars(x.Right)
	case *OptionalPattern:
		blankLabelsToVars(x.Main)
		blankLabelsToVars(x.Optional)
	case *UnionPattern:
		blankLabelsToVars(x.Left)
		blankLabelsToVars(x.Right)
	case *MinusPattern:
		blankLabelsToVars(x.Left)
		blankLabelsToVars(x.Right)
	case *FilterPattern:
		blankLabelsToVars(x.Pattern)
		blankLabelsInExpr(x.Expr)
	case *BindPattern:
		blankLabelsToVars(x.Pattern)
		blankLabelsInExpr(x.Expr)
	case *GraphPattern:
		blankLabelsToVars(x.Pattern)
	case *SubqueryPattern:
		blankLabelsInQuery(x.Query)
	}
	return p
}

func blankLabelsInQuery(q *ParsedQuery) {
	if q == nil {
		return
	}
	blankLabelsToVars(q.Where)
	for _, pe := range q.ProjectExprs {
		blankLabelsInExpr(pe.Expr)
	}
	for _, e := range q.GroupBy {
		blankLabelsInExpr(e)
	}
	blankLabelsInExpr(q.Having)
	for _, o := range q.OrderBy {
		blankLabelsInExpr(o.Expr)
	}
}

func blankLabelsInExpr(e Expr) {
	switch x := e.(type) {
	case *ExistsExpr:
		blankLabelsToVars(x.Pattern)
	case *BinaryExpr:
		blankLabelsInExpr(x.Left)
		blankLabelsInExpr(x.Right)
	case *UnaryExpr:
		blankLabelsInExpr(x.Arg)
	case *FuncExpr:
		for _, a := range x.Args {
			blankLabelsInExpr(a)
		}
	}
}

// blankLabelTerm rewrites one pattern position; inside a triple term it
// rewrites every component, recursively.
func blankLabelTerm(s string) string {
	switch {
	case strings.HasPrefix(s, "_:"):
		return blankLabelVarPrefix + s[2:]
	case strings.HasPrefix(s, "<<( ") && strings.HasSuffix(s, " )>>") && strings.Contains(s, "_:"):
		parts := splitTripleTermParts(s[4 : len(s)-4])
		for i, part := range parts {
			parts[i] = blankLabelTerm(part)
		}
		return "<<( " + strings.Join(parts, " ") + " )>>"
	}
	return s
}

func isVar(s string) bool { return strings.HasPrefix(s, "?") }
