package shacl

import (
	"slices"

	"github.com/tggo/goRDFlib/sparql"
)

// List operator expressions of SHACL 1.2 Node Expressions: they take the
// output nodes of other expressions and produce a list from them.

// nodesOrFocus evaluates the input of an expression whose shnex:nodes is
// optional: the focus node when it is omitted.
func nodesOrFocus(ctx *nodeExprContext, nodes NodeExpr) ([]Term, error) {
	if nodes == nil {
		return ctx.single(), nil
	}
	return evalNodeExpr(ctx, nodes)
}

// ---------- ConcatExpr ----------

// ConcatExpr concatenates the results of multiple expressions.
type ConcatExpr struct {
	Members []NodeExpr
}

func (e *ConcatExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *ConcatExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	return concatAll(ctx, e.Members)
}

// ---------- DistinctExpr ----------

// DistinctExpr removes duplicates (term equality) preserving order.
type DistinctExpr struct {
	Nodes NodeExpr
}

func (e *DistinctExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *DistinctExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	nodes, err := evalNodeExpr(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(nodes))
	var result []Term
	for _, n := range nodes {
		key := n.TermKey()
		if !seen[key] {
			seen[key] = true
			result = append(result, n)
		}
	}
	return result, nil
}

// ---------- IntersectionExpr ----------

// IntersectionExpr returns nodes that appear in all member expressions (term equality).
type IntersectionExpr struct {
	Members []NodeExpr
}

func (e *IntersectionExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *IntersectionExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	if len(e.Members) == 0 {
		return nil, nil
	}
	// Every member is evaluated, even once the intersection is empty: a
	// failure of a later member is a failure of the expression.
	first, err := evalNodeExpr(ctx, e.Members[0])
	if err != nil {
		return nil, err
	}
	current := make(map[string]bool, len(first))
	for _, t := range first {
		current[t.TermKey()] = true
	}
	for _, m := range e.Members[1:] {
		nodes, err := evalNodeExpr(ctx, m)
		if err != nil {
			return nil, err
		}
		next := make(map[string]bool, len(current))
		for _, t := range nodes {
			if key := t.TermKey(); current[key] {
				next[key] = true
			}
		}
		current = next
	}
	// Preserve order from first set, deduplicate
	seen := make(map[string]bool, len(current))
	var result []Term
	for _, t := range first {
		key := t.TermKey()
		if current[key] && !seen[key] {
			seen[key] = true
			result = append(result, t)
		}
	}
	return result, nil
}

// ---------- RemoveExpr ----------

// RemoveExpr removes terms from the input that match terms in the remove set (term equality).
type RemoveExpr struct {
	Nodes    NodeExpr
	ToRemove NodeExpr
}

func (e *RemoveExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *RemoveExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	remove, err := evalNodeExpr(ctx, e.ToRemove)
	if err != nil {
		return nil, err
	}
	nodes, err := evalNodeExpr(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	removeSet := make(map[string]bool, len(remove))
	for _, r := range remove {
		removeSet[r.TermKey()] = true
	}
	var result []Term
	for _, n := range nodes {
		if !removeSet[n.TermKey()] {
			result = append(result, n)
		}
	}
	return result, nil
}

// ---------- ExistsExpr ----------

// ExistsExpr returns true/false depending on whether the input is non-empty.
type ExistsExpr struct {
	Nodes NodeExpr
}

func (e *ExistsExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *ExistsExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	nodes, err := evalNodeExpr(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	return []Term{boolTerm(len(nodes) > 0)}, nil
}

// ---------- IfExpr ----------

// IfExpr evaluates condition; the then branch if it is exactly ( true ), the
// else branch otherwise. A branch is evaluated only when it is chosen, so a
// failing branch that is not chosen is no failure.
type IfExpr struct {
	Condition NodeExpr
	Then      NodeExpr // may be nil
	Else      NodeExpr // may be nil
}

func (e *IfExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *IfExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	cond, err := evalNodeExpr(ctx, e.Condition)
	if err != nil {
		return nil, err
	}
	branch := e.Else
	if isTrueOnly(cond) {
		branch = e.Then
	}
	if branch == nil {
		return nil, nil
	}
	return evalNodeExpr(ctx, branch)
}

// ---------- LimitExpr ----------

// LimitExpr takes only the first N results.
type LimitExpr struct {
	Nodes NodeExpr
	Limit Term // the shnex:limit value, an xsd:integer
}

func (e *LimitExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *LimitExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	n, err := countArg("shnex:limit", e.Limit)
	if err != nil {
		return nil, err
	}
	nodes, err := evalNodeExpr(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	if n >= len(nodes) {
		return nodes, nil
	}
	return nodes[:n], nil
}

// ---------- OffsetExpr ----------

// OffsetExpr skips the first N results.
type OffsetExpr struct {
	Nodes  NodeExpr
	Offset Term // the shnex:offset value, an xsd:integer
}

func (e *OffsetExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *OffsetExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	n, err := countArg("shnex:offset", e.Offset)
	if err != nil {
		return nil, err
	}
	nodes, err := evalNodeExpr(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	if n >= len(nodes) {
		return nil, nil
	}
	return nodes[n:], nil
}

// countArg reads the non-negative xsd:integer of shnex:limit / shnex:offset.
func countArg(name string, t Term) (int, error) {
	n, ok := integerValue(t)
	if !ok || n < 0 {
		return 0, failf("%s must be a non-negative xsd:integer, not %s", name, t)
	}
	return n, nil
}

// ---------- OrderByExpr ----------

// OrderByExpr sorts input nodes by a key expression.
type OrderByExpr struct {
	Nodes   NodeExpr
	KeyExpr NodeExpr
	Desc    bool
}

func (e *OrderByExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

// Evaluate sorts by the first output node of the key expression evaluated at
// each node, in the order of SPARQL ORDER BY: a node without a key sorts
// before every key, and blank nodes, IRIs and literals follow in that order.
// Descending is the reverse of the ascending list.
func (e *OrderByExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	nodes, err := evalNodeExpr(ctx, e.Nodes)
	if err != nil {
		return nil, err
	}
	type entry struct {
		node Term
		key  Term
	}
	entries := make([]entry, len(nodes))
	for i, n := range nodes {
		keys, err := evalNodeExpr(ctx.at(n), e.KeyExpr)
		if err != nil {
			return nil, err
		}
		entries[i].node = n
		if len(keys) > 0 {
			entries[i].key = keys[0]
		}
	}
	slices.SortStableFunc(entries, func(a, b entry) int {
		return sparql.CompareTerms(toTerm(a.key), toTerm(b.key))
	})
	result := make([]Term, len(entries))
	for i, ent := range entries {
		result[i] = ent.node
	}
	if e.Desc {
		slices.Reverse(result)
	}
	return result, nil
}

// ---------- FlatMapExpr ----------

// FlatMapExpr evaluates an expression for each input node and concatenates results.
type FlatMapExpr struct {
	Nodes   NodeExpr // nil: the focus node
	MapExpr NodeExpr
}

func (e *FlatMapExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *FlatMapExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	return evalFlatMap(ctx, e.Nodes, e.MapExpr)
}

func evalFlatMap(ctx *nodeExprContext, nodesExpr, mapExpr NodeExpr) ([]Term, error) {
	nodes, err := nodesOrFocus(ctx, nodesExpr)
	if err != nil {
		return nil, err
	}
	var result []Term
	for _, n := range nodes {
		mapped, err := evalNodeExpr(ctx.at(n), mapExpr)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped...)
	}
	return result, nil
}
