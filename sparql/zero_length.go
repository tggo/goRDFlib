package sparql

import (
	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/term"
)

// Zero-length path steps and terms the graph does not hold.
//
// SPARQL 1.1 §18.5 gives `?s p* ?d` (both ends variables) the zero-length
// solutions of the nodes of the active graph only, and `{ VALUES ?d { "x" }
// ?s p* ?d }` is the join (§18.5, Join) of that with VALUES: "x" is no node,
// so the join is empty. Substituting the bound value into the path first
// would match a zero-length step on "x" and give a row. A term written in the
// pattern, or an initial binding (a constant substituted into the query,
// SHACL pre-binding), is different: Path(X, p*, Y) with X a term yields X
// whether or not the graph holds it (the W3C property-path tests "? with start
// being a constant on the empty dataset").
//
// So a pair (x, x) of a path whose endpoint is a variable bound by the
// solution so far (an earlier pattern, VALUES, BIND, a join) counts only when
// x is a node of the graph. The check is by pair, after the path is evaluated,
// so it applies to the local traversal and the store pushdown alike, and to
// the pre-bound join's substitution as much as to the nested loops.

// rowBound reports whether pat is a variable that bindings holds and that is
// not one of the evaluation's initial bindings: a value that came from the
// solution being built. A triple term written with such a variable inside
// (<<( ?b ex:p ex:o )>>) counts too: substituting the row into it makes a term
// the graph need not hold, exactly as for a bare variable.
func rowBound(ec *evalCtx, pat string, bindings map[string]rdflibgo.Term) bool {
	// Most endpoints are constants or unbound variables: answer those without
	// a closure, since this runs for every path pattern evaluated.
	switch {
	case len(pat) == 0:
		return false
	case pat[0] == '?':
		val, ok := bindings[pat[1:]]
		if !ok {
			return false
		}
		if ec != nil {
			if pv, isPre := ec.pre[pat[1:]]; isPre && term.TermKey(pv) == term.TermKey(val) {
				return false
			}
		}
		return true
	case pat[0] != '<' || len(pat) < 3 || pat[1] != '<':
		return false
	}
	found := false
	collectTermVars(pat, func(v string) {
		val, ok := bindings[v]
		if !ok {
			return
		}
		if ec != nil {
			if pv, isPre := ec.pre[v]; isPre && term.TermKey(pv) == term.TermKey(val) {
				return
			}
		}
		found = true
	})
	return found
}

// zeroLengthMatches reports whether the pair (x, x) of the path pattern tp is
// a match: unless x is the value of a row-bound endpoint, always; for such an
// endpoint, when x is a node of the graph (a subject or an object), or when
// the other endpoint is a constant for x itself (constKeys are the keys of the
// subject and object constants, empty for a variable): the path side alone
// then yields (x, x), whatever the graph holds, and the join keeps it.
func (ec *evalCtx) zeroLengthMatches(g *rdflibgo.Graph, tp Triple, bindings map[string]rdflibgo.Term, x term.Term, constKeys [2]string, prefixes map[string]string) bool {
	xk := term.TermKey(x)
	if constKeys[0] == xk || constKeys[1] == xk {
		return true
	}
	for _, pat := range [2]string{tp.Subject, tp.Object} {
		if !rowBound(ec, pat, bindings) {
			continue
		}
		if rv := resolvePatternTerm(pat, bindings, prefixes); rv == nil || term.TermKey(rv) != xk {
			continue
		}
		return isGraphNode(ec, g, x)
	}
	return true
}

// isGraphNode reports whether t is the subject or the object of a triple of g.
func isGraphNode(ec *evalCtx, g *rdflibgo.Graph, t term.Term) bool {
	ctx := ec.context()
	if s, ok := t.(term.Subject); ok {
		for _, err := range g.Triples(ctx, s, nil, nil) {
			ec.fail(err)
			return err == nil
		}
	}
	for _, err := range g.Triples(ctx, nil, nil, t) {
		ec.fail(err)
		return err == nil
	}
	return false
}

// pathEndChecks is what eachPathSolution needs to know about the endpoints of a
// path pattern beyond the path's own pairs.
type pathEndChecks struct {
	// subjTT and objTT: the endpoint is a triple term with variables that no
	// earlier pattern bound. The path runs open at that end and the pair's end
	// is matched against the pattern, binding its variables.
	subjTT, objTT bool
	// checkZero: an endpoint was bound from the row, so a zero-length pair
	// needs zeroLengthMatches. constKeys are the keys of the endpoints that
	// are constants (empty for a variable).
	checkZero bool
	constKeys [2]string
}

// newPathEndChecks returns nil when neither check can apply, which is almost
// always: no triple term endpoint and no endpoint bound from the row.
func newPathEndChecks(ec *evalCtx, tp Triple, bindings map[string]rdflibgo.Term, sVal, oVal term.Term, subjOpen, objOpen bool) *pathEndChecks {
	subjTT := subjOpen && len(tp.Subject) > 3 && tp.Subject[0] == '<' && tp.Subject[1] == '<'
	objTT := objOpen && len(tp.Object) > 3 && tp.Object[0] == '<' && tp.Object[1] == '<'
	rowS, rowO := rowBound(ec, tp.Subject, bindings), rowBound(ec, tp.Object, bindings)
	if !subjTT && !objTT && !rowS && !rowO {
		return nil
	}
	c := &pathEndChecks{subjTT: subjTT, objTT: objTT, checkZero: rowS || rowO}
	if c.checkZero {
		if sVal != nil && !rowS {
			c.constKeys[0] = term.TermKey(sVal)
		}
		if oVal != nil && !rowO {
			c.constKeys[1] = term.TermKey(oVal)
		}
	}
	return c
}
