package sparql

import (
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// evalExpr evaluates an expression without a graph: EXISTS evaluates to an
// error. Use evalExprWithGraph wherever the active graph is known.
func evalExpr(expr Expr, bindings map[string]rdflibgo.Term, prefixes map[string]string) rdflibgo.Term {
	return evalExprWithGraph(nil, expr, bindings, prefixes, nil, nil)
}

// evalExprWithGraph evaluates an expression against a solution. g and
// namedGraphs are the active dataset for EXISTS, which SPARQL 1.1 §17.4.1.4
// defines as a built-in usable anywhere an expression is: in FILTER and BIND,
// in SELECT expressions, GROUP BY, HAVING, ORDER BY and inside function
// arguments. The graph is passed down through every sub-expression. It
// returns nil for an evaluation error.
func evalExprWithGraph(ec *evalCtx, expr Expr, bindings map[string]rdflibgo.Term, prefixes map[string]string, g *rdflibgo.Graph, namedGraphs map[string]*rdflibgo.Graph) rdflibgo.Term {
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
				iri = resolveRelativeIRI(base, iri)
			}
		}
		return rdflibgo.NewURIRefUnsafe(iri)
	case *BinaryExpr:
		left := evalExprWithGraph(ec, e.Left, bindings, prefixes, g, namedGraphs)
		right := evalExprWithGraph(ec, e.Right, bindings, prefixes, g, namedGraphs)
		return evalBinaryOp(e.Op, left, right)
	case *UnaryExpr:
		return evalUnaryOp(e.Op, evalExprWithGraph(ec, e.Arg, bindings, prefixes, g, namedGraphs))
	case *FuncExpr:
		// A registered extension function (SPARQL 1.1 §17.6) wins over the
		// built-in table, so callers can override e.g. an xsd: cast.
		if res, handled := evalExtensionFunc(ec, e, bindings, prefixes, g, namedGraphs); handled {
			return res
		}
		return evalFuncWithGraph(ec, e.Name, e.Args, bindings, prefixes, g, namedGraphs)
	case *ExistsExpr:
		if g == nil {
			return nil // no active graph to evaluate the pattern against
		}
		exists := len(evalPatternWithBindings(ec, g, e.Pattern, bindings, prefixes, namedGraphs)) > 0
		if e.Not {
			exists = !exists
		}
		return rdflibgo.NewLiteral(exists)
	}
	return nil
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
