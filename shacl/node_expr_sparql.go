package shacl

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// SPARQL function expressions (SHACL 1.2 Node Expressions, "SPARQL
// Functions"): [ sparql:abs ( -42 ) ] applies the SPARQL function ABS to the
// arguments' single output nodes. The function is evaluated by the SPARQL
// engine itself, with the arguments as initial bindings, so it has exactly
// the semantics of the SPARQL function, including its errors.

// sparqlFunction describes how a sparql:NAME function is written in SPARQL.
type sparqlFunction struct {
	minArgs, maxArgs int // maxArgs < 0: any number
	form             func(n int) string

	// handlesUnbound: an argument without output nodes is an unbound value
	// handed to the function instead of making the result empty.
	handlesUnbound bool
	// absorbsErrors: a failed argument is an unbound value as well, since the
	// SPARQL function is defined on errors (coalesce skips them, if ignores
	// its unselected branch, logical-and/or follow the truth tables).
	absorbsErrors bool
}

func infixForm(op string) func(int) string {
	return func(int) string { return "(?a0 " + op + " ?a1)" }
}

func prefixForm(op string) func(int) string {
	return func(int) string { return "(" + op + "?a0)" }
}

// callForm writes NAME(?a0, ?a1, ...).
func callForm(name string) func(int) string {
	return func(n int) string {
		var b strings.Builder
		b.WriteString(name)
		b.WriteByte('(')
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("?a" + strconv.Itoa(i))
		}
		b.WriteByte(')')
		return b.String()
	}
}

func fixed(n int, form func(int) string) sparqlFunction {
	return sparqlFunction{minArgs: n, maxArgs: n, form: form}
}

// sparqlFunctions is the library of shnex-sparql.ttl, keyed by local name.
var sparqlFunctions = buildSPARQLFunctions()

func buildSPARQLFunctions() map[string]sparqlFunction {
	m := map[string]sparqlFunction{
		// algebraic operators
		"plus":     fixed(2, infixForm("+")),
		"subtract": fixed(2, infixForm("-")),
		"multiply": fixed(2, infixForm("*")),
		"divide":   fixed(2, infixForm("/")),
		// unary-minus/-plus are written with a space: "(-?a0)" would read as
		// a negative number for some inputs of a lexer, "(- ?a0)" never does.
		"unary-minus": fixed(1, prefixForm("- ")),
		"unary-plus":  fixed(1, prefixForm("+ ")),

		// comparison
		"equals":                fixed(2, infixForm("=")),
		"not-equals":            fixed(2, infixForm("!=")),
		"greater-than":          fixed(2, infixForm(">")),
		"less-than":             fixed(2, infixForm("<")),
		"greater-than-or-equal": fixed(2, infixForm(">=")),
		"less-than-or-equal":    fixed(2, infixForm("<=")),
		"sameTerm":              fixed(2, callForm("SAMETERM")),
		// SPARQL 1.2 sameValue: value equality that is never an error; two
		// terms the engine cannot compare by value are the same value only if
		// they are the same term.
		"sameValue": fixed(2, func(int) string { return "COALESCE((?a0 = ?a1), SAMETERM(?a0, ?a1))" }),

		// forms that handle unbound values
		"logical-not": fixed(1, prefixForm("!")),
		"bound":       {minArgs: 1, maxArgs: 1, form: callForm("BOUND"), handlesUnbound: true},
		"if":          {minArgs: 3, maxArgs: 3, form: callForm("IF"), handlesUnbound: true, absorbsErrors: true},
		"coalesce":    {minArgs: 0, maxArgs: -1, form: callForm("COALESCE"), handlesUnbound: true, absorbsErrors: true},
		"logical-or":  {minArgs: 2, maxArgs: 2, form: infixForm("||"), handlesUnbound: true, absorbsErrors: true},
		"logical-and": {minArgs: 2, maxArgs: 2, form: infixForm("&&"), handlesUnbound: true, absorbsErrors: true},

		// variadic and optional-argument functions
		"concat":     {minArgs: 0, maxArgs: -1, form: callForm("CONCAT")},
		"bnode":      {minArgs: 0, maxArgs: 1, form: callForm("BNODE")},
		"regex":      {minArgs: 2, maxArgs: 3, form: callForm("REGEX")},
		"replace":    {minArgs: 3, maxArgs: 4, form: callForm("REPLACE")},
		"substr":     {minArgs: 2, maxArgs: 3, form: callForm("SUBSTR")},
		"triple":     fixed(3, callForm("TRIPLE")),
		"strlangdir": fixed(3, callForm("STRLANGDIR")),
	}
	for name, sparqlName := range map[string]string{
		// no arguments
		"now": "NOW", "rand": "RAND", "uuid": "UUID", "struuid": "STRUUID",
	} {
		m[name] = fixed(0, callForm(sparqlName))
	}
	for name, sparqlName := range map[string]string{
		"abs": "ABS", "ceil": "CEIL", "floor": "FLOOR", "round": "ROUND",
		"isBlank": "isBLANK", "isIRI": "isIRI", "isURI": "isURI", "isLiteral": "isLITERAL",
		"isNumeric": "isNUMERIC", "isTriple": "isTRIPLE",
		"str": "STR", "lang": "LANG", "langdir": "LANGDIR", "hasLang": "hasLANG",
		"hasLangdir": "hasLANGDIR", "datatype": "DATATYPE", "iri": "IRI", "uri": "URI",
		"encode": "ENCODE_FOR_URI", "lcase": "LCASE", "ucase": "UCASE", "strlen": "STRLEN",
		"md5": "MD5", "sha1": "SHA1", "sha256": "SHA256", "sha384": "SHA384", "sha512": "SHA512",
		"year": "YEAR", "month": "MONTH", "day": "DAY", "hours": "HOURS",
		"minutes": "MINUTES", "seconds": "SECONDS", "timezone": "TIMEZONE", "tz": "TZ",
		"subject": "SUBJECT", "predicate": "PREDICATE", "object": "OBJECT",
	} {
		m[name] = fixed(1, callForm(sparqlName))
	}
	for name, sparqlName := range map[string]string{
		"contains": "CONTAINS", "langMatches": "LANGMATCHES", "strafter": "STRAFTER",
		"strbefore": "STRBEFORE", "strends": "STRENDS", "strstarts": "STRSTARTS",
		"strdt": "STRDT", "strlang": "STRLANG",
	} {
		m[name] = fixed(2, callForm(sparqlName))
	}
	return m
}

// SPARQLFunctionExpr is [ sparql:NAME ( args... ) ].
type SPARQLFunctionExpr struct {
	Name string // local name, e.g. "abs"
	Args []NodeExpr
}

func (e *SPARQLFunctionExpr) Eval(ctx *nodeExprContext) []Term { return dropFailure(e.Evaluate(ctx)) }

func (e *SPARQLFunctionExpr) Evaluate(ctx *nodeExprContext) ([]Term, error) {
	fn, ok := sparqlFunctions[e.Name]
	if !ok {
		return nil, failf("sparql:%s is not a SPARQL function of the node expression library", e.Name)
	}
	if len(e.Args) < fn.minArgs || (fn.maxArgs >= 0 && len(e.Args) > fn.maxArgs) {
		return nil, failf("sparql:%s takes %s arguments, not %d", e.Name, arityText(fn), len(e.Args))
	}
	// Both the lazy forms (if) and the strict ones evaluate every argument:
	// sparql:if ignores the unselected branch only when it is an error, not
	// when evaluating it is merely expensive, and an error must be seen to be
	// ignored.
	args := evalListArguments(ctx, e.Args)
	if fn.absorbsErrors {
		for i := range args {
			if args[i].err != nil {
				args[i] = argument{}
			}
		}
	} else if err := firstArgumentFailure(args); err != nil {
		return nil, err
	}
	if !fn.handlesUnbound && !allBound(args) {
		return nil, nil
	}
	vals := make([]Term, len(args))
	bound := make([]bool, len(args))
	for i, a := range args {
		vals[i], bound[i] = a.node, a.bound
	}
	res, err := applySPARQLFunction(ctx.goContext(), fn, e.Name, vals, bound)
	if err != nil {
		return nil, err
	}
	return []Term{res}, nil
}

func arityText(fn sparqlFunction) string {
	switch {
	case fn.maxArgs < 0:
		return "any number of"
	case fn.minArgs == fn.maxArgs:
		return strconv.Itoa(fn.minArgs)
	}
	return fmt.Sprintf("%d to %d", fn.minArgs, fn.maxArgs)
}

// callSPARQLFunction applies the library function name to bound arguments.
func callSPARQLFunction(ctx context.Context, name string, args []Term) (Term, error) {
	fn, ok := sparqlFunctions[name]
	if !ok {
		return Term{}, failf("sparql:%s is not a SPARQL function of the node expression library", name)
	}
	bound := make([]bool, len(args))
	for i := range bound {
		bound[i] = true
	}
	return applySPARQLFunction(ctx, fn, name, args, bound)
}

// sparqlFunctionQueries caches the parsed SELECT of each (function, arity).
var sparqlFunctionQueries sync.Map // string -> *sparql.ParsedQuery

// noData is the graph the function queries run on: they read no data.
var noData = sync.OnceValue(func() *graph.Graph { return graph.NewGraph() })

// applySPARQLFunction evaluates fn on the arguments with the SPARQL engine.
// An argument that is not bound is left out of the initial bindings, which is
// an unbound variable. An error of the function, and an unbound result (which
// is how SPARQL reports an error in BIND), is an ErrNodeExprFailure.
func applySPARQLFunction(ctx context.Context, fn sparqlFunction, name string, vals []Term, bound []bool) (Term, error) {
	key := name + "/" + strconv.Itoa(len(vals))
	var pq *sparql.ParsedQuery
	if cached, ok := sparqlFunctionQueries.Load(key); ok {
		pq = cached.(*sparql.ParsedQuery)
	} else {
		var err error
		pq, err = sparql.Parse("SELECT ?r WHERE { BIND(" + fn.form(len(vals)) + " AS ?r) }")
		if err != nil {
			return Term{}, failf("sparql:%s cannot be evaluated: %w", name, err)
		}
		sparqlFunctionQueries.Store(key, pq)
	}
	init := make(map[string]term.Term, len(vals))
	for i, v := range vals {
		if !bound[i] {
			continue
		}
		t := toTerm(v)
		if t == nil {
			return Term{}, failf("sparql:%s cannot take %s as an argument", name, v)
		}
		init["a"+strconv.Itoa(i)] = t
	}
	res, err := sparql.EvalQueryContext(ctx, noData(), pq, init)
	if err != nil {
		return Term{}, failf("sparql:%s: %w", name, err)
	}
	if len(res.Bindings) != 1 || res.Bindings[0]["r"] == nil {
		return Term{}, failf("sparql:%s is a SPARQL error for %s", name, describeArguments(vals, bound))
	}
	return fromRDFLib(res.Bindings[0]["r"]), nil
}

func describeArguments(vals []Term, bound []bool) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		if bound[i] {
			parts[i] = v.String()
		} else {
			parts[i] = "(unbound)"
		}
	}
	return "(" + strings.Join(parts, " ") + ")"
}
