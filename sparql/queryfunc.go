package sparql

// Query-scoped extension functions.
//
// RegisterFunction binds a function to an IRI process-wide. That is the right
// model for a fixed vocabulary such as a GeoSPARQL layer, but not for functions
// that are declared by the data a caller happens to be processing — SHACL's
// sh:SPARQLFunction, for instance, defines an IRI-named function inside a shapes
// graph, and two shapes graphs may legitimately define the same IRI differently.
// Registering those globally would leak one caller's definitions into another's
// queries and make concurrent use of two shapes graphs racy.
//
// BindFunctions attaches a function table to one parsed query instead. A call
// bound this way takes precedence over the global registry, and nothing outside
// the query is affected.

// BindFunctions binds query-scoped extension functions to q, matching calls by
// the IRI they were written with — either <iri>(...) or a prefixed name that
// resolves to iri. It returns the number of call sites bound, so a caller can
// tell a typo'd IRI from a function that is simply never called.
//
// A bound function takes precedence over one registered globally with
// RegisterFunction for the same IRI. Names in funcs that the query never calls
// are ignored; calls whose IRI is not in funcs keep falling back to the global
// registry.
//
// BindFunctions mutates q, so call it on a ParsedQuery you own — one you just
// parsed, or one no other goroutine is evaluating. Binding must happen before
// EvalQuery; the binding is then fixed for the lifetime of q, which makes
// concurrent evaluation of the bound query safe.
//
// Sub-SELECTs are bound as well, so a function is callable at any depth.
func (q *ParsedQuery) BindFunctions(funcs map[string]Function) int {
	if q == nil || len(funcs) == 0 {
		return 0
	}
	b := &funcBinder{funcs: funcs, seen: make(map[*ParsedQuery]bool)}
	b.query(q)
	return b.bound
}

// funcBinder walks a query's expression trees. seen guards against a cyclic
// sub-query graph, which the parser does not build but a caller could assemble
// by hand; without it such a value would recurse forever.
type funcBinder struct {
	funcs map[string]Function
	seen  map[*ParsedQuery]bool
	bound int
}

func (b *funcBinder) query(q *ParsedQuery) {
	if q == nil || b.seen[q] {
		return
	}
	b.seen[q] = true

	for i := range q.ProjectExprs {
		b.expr(q.ProjectExprs[i].Expr)
	}
	for i := range q.OrderBy {
		b.expr(q.OrderBy[i].Expr)
	}
	for _, e := range q.GroupBy {
		b.expr(e)
	}
	b.expr(q.Having)
	b.pattern(q.Where)
}

func (b *funcBinder) expr(e Expr) {
	switch x := e.(type) {
	case nil:
		return
	case *BinaryExpr:
		b.expr(x.Left)
		b.expr(x.Right)
	case *UnaryExpr:
		b.expr(x.Arg)
	case *FuncExpr:
		// Arguments are walked whether or not this call itself is bound: a
		// built-in may take a bound function's result as an argument.
		for _, a := range x.Args {
			b.expr(a)
		}
		if x.IRI == "" {
			return
		}
		if fn, ok := b.funcs[x.IRI]; ok {
			x.Fn = fn
			b.bound++
		}
	case *ExistsExpr:
		b.pattern(x.Pattern)
	}
}

func (b *funcBinder) pattern(p Pattern) {
	switch x := p.(type) {
	case nil:
		return
	case *JoinPattern:
		b.pattern(x.Left)
		b.pattern(x.Right)
	case *OptionalPattern:
		b.pattern(x.Main)
		b.pattern(x.Optional)
	case *UnionPattern:
		b.pattern(x.Left)
		b.pattern(x.Right)
	case *MinusPattern:
		b.pattern(x.Left)
		b.pattern(x.Right)
	case *FilterPattern:
		b.pattern(x.Pattern)
		b.expr(x.Expr)
	case *BindPattern:
		b.pattern(x.Pattern)
		b.expr(x.Expr)
	case *GraphPattern:
		b.pattern(x.Pattern)
	case *SubqueryPattern:
		b.query(x.Query)
	}
}
