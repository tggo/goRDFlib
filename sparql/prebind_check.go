package sparql

import (
	"errors"
	"fmt"
)

// ErrPreBindRestriction is wrapped by the errors of PreBindRestrictions.
var ErrPreBindRestriction = errors.New("sparql: query violates the restrictions on pre-bound variables")

// PreBindRestrictions checks the query against the restrictions SHACL-SPARQL
// (Appendix, "Pre-binding of Variables in SPARQL Queries") puts on a query that
// is executed with pre-bound variables, where vars are the potentially
// pre-bound variables, written without the marker:
//
//   - no MINUS clause,
//   - no VALUES clause that mentions a potentially pre-bound variable,
//   - no AS ?var, in a projection, GROUP BY or BIND, for a potentially pre-bound variable.
//
// SERVICE is not part of the check: the parser refuses it. It returns nil, or
// an error wrapping ErrPreBindRestriction that names the construct. It does not
// modify the query.
func (q *ParsedQuery) PreBindRestrictions(vars ...string) error {
	if q == nil || len(vars) == 0 {
		return nil
	}
	w := restrictionWalker{vars: make(map[string]bool, len(vars)), seen: map[*ParsedQuery]bool{}}
	for _, v := range vars {
		w.vars[v] = true
	}
	return w.query(q)
}

type restrictionWalker struct {
	vars map[string]bool
	seen map[*ParsedQuery]bool
}

func (w restrictionWalker) fail(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrPreBindRestriction, fmt.Sprintf(format, args...))
}

func (w restrictionWalker) query(q *ParsedQuery) error {
	if q == nil || w.seen[q] {
		return nil
	}
	w.seen[q] = true
	for _, pe := range q.ProjectExprs {
		if pe.Var != "" && w.vars[pe.Var] {
			return w.fail("AS ?%s assigns a pre-bound variable", pe.Var)
		}
		if err := w.expr(pe.Expr); err != nil {
			return err
		}
	}
	for _, a := range q.GroupByAliases {
		if a != "" && w.vars[a] {
			return w.fail("GROUP BY ... AS ?%s assigns a pre-bound variable", a)
		}
	}
	for _, e := range q.GroupBy {
		if err := w.expr(e); err != nil {
			return err
		}
	}
	for _, o := range q.OrderBy {
		if err := w.expr(o.Expr); err != nil {
			return err
		}
	}
	if err := w.expr(q.Having); err != nil {
		return err
	}
	return w.pattern(q.Where)
}

func (w restrictionWalker) pattern(p Pattern) error {
	switch x := p.(type) {
	case nil, *BGP:
		return nil
	case *JoinPattern:
		return firstErr(w.pattern(x.Left), func() error { return w.pattern(x.Right) })
	case *OptionalPattern:
		return firstErr(w.pattern(x.Main), func() error { return w.pattern(x.Optional) })
	case *UnionPattern:
		return firstErr(w.pattern(x.Left), func() error { return w.pattern(x.Right) })
	case *MinusPattern:
		return w.fail("MINUS")
	case *FilterPattern:
		return firstErr(w.pattern(x.Pattern), func() error { return w.expr(x.Expr) })
	case *BindPattern:
		if w.vars[x.Var] {
			return w.fail("BIND ... AS ?%s assigns a pre-bound variable", x.Var)
		}
		return firstErr(w.pattern(x.Pattern), func() error { return w.expr(x.Expr) })
	case *ValuesPattern:
		for _, v := range x.Vars {
			if w.vars[v] {
				return w.fail("VALUES mentions the pre-bound variable ?%s", v)
			}
		}
		return nil
	case *GraphPattern:
		return w.pattern(x.Pattern)
	case *SubqueryPattern:
		return w.query(x.Query)
	}
	return nil
}

func (w restrictionWalker) expr(e Expr) error {
	switch x := e.(type) {
	case *BinaryExpr:
		return firstErr(w.expr(x.Left), func() error { return w.expr(x.Right) })
	case *UnaryExpr:
		return w.expr(x.Arg)
	case *FuncExpr:
		for _, a := range x.Args {
			if err := w.expr(a); err != nil {
				return err
			}
		}
	case *ExistsExpr:
		return w.pattern(x.Pattern)
	}
	return nil
}

func firstErr(err error, next func() error) error {
	if err != nil {
		return err
	}
	return next()
}
