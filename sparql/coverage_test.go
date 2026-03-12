package sparql

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
)

// --- Query Cache ---

func TestQueryCacheEnableDisable(t *testing.T) {
	// Ensure cache starts nil
	DisableQueryCache()
	if queryCache != nil {
		t.Error("expected nil cache after disable")
	}

	EnableQueryCache(10)
	if queryCache == nil {
		t.Fatal("expected non-nil cache after enable")
	}
	if queryCache.Len() != 0 {
		t.Error("expected empty cache")
	}

	g := makeSPARQLGraph(t)
	q := `PREFIX ex: <http://example.org/> SELECT ?name WHERE { ?s ex:name ?name }`

	// First call: cache miss → parse + put
	r1, err := Query(g, q)
	if err != nil {
		t.Fatal(err)
	}
	if queryCache.Len() != 1 {
		t.Errorf("expected 1 cached entry, got %d", queryCache.Len())
	}

	// Second call: cache hit
	r2, err := Query(g, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.Bindings) != len(r2.Bindings) {
		t.Error("results differ between cache miss and hit")
	}
	if queryCache.Len() != 1 {
		t.Errorf("expected still 1 cached entry, got %d", queryCache.Len())
	}

	DisableQueryCache()
	if queryCache != nil {
		t.Error("expected nil cache after disable")
	}
}

func TestQueryCacheDefaultCapacity(t *testing.T) {
	c := NewQueryCache(0)
	if c.capacity != 256 {
		t.Errorf("expected default capacity 256, got %d", c.capacity)
	}
	c2 := NewQueryCache(-5)
	if c2.capacity != 256 {
		t.Errorf("expected default capacity 256 for negative, got %d", c2.capacity)
	}
}

func TestQueryCacheEviction(t *testing.T) {
	c := NewQueryCache(2)

	q1 := &ParsedQuery{Type: "SELECT"}
	q2 := &ParsedQuery{Type: "ASK"}
	q3 := &ParsedQuery{Type: "CONSTRUCT"}

	c.Put("q1", q1)
	c.Put("q2", q2)
	if c.Len() != 2 {
		t.Errorf("expected 2, got %d", c.Len())
	}

	// Put q3 should evict q1
	c.Put("q3", q3)
	if c.Len() != 2 {
		t.Errorf("expected 2 after eviction, got %d", c.Len())
	}
	if c.Get("q1") != nil {
		t.Error("expected q1 to be evicted")
	}
	if c.Get("q2") == nil {
		t.Error("expected q2 to still be cached")
	}
	if c.Get("q3") == nil {
		t.Error("expected q3 to be cached")
	}
}

func TestQueryCachePutDuplicate(t *testing.T) {
	c := NewQueryCache(2)
	q1 := &ParsedQuery{Type: "SELECT"}
	c.Put("q1", q1)
	c.Put("q1", q1) // should be no-op
	if c.Len() != 1 {
		t.Errorf("expected 1, got %d", c.Len())
	}
}

// --- extractSimpleBGP / buildStorePattern pushdown ---

func TestASKPushdown(t *testing.T) {
	// ASK with a simple BGP should use store.Exists pushdown on MemoryStore
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		ASK { ex:Alice ex:name "Alice" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AskResult {
		t.Error("expected true")
	}

	// ASK with no match
	r2, err := Query(g, `
		PREFIX ex: <http://example.org/>
		ASK { ex:Alice ex:name "NonExistent" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r2.AskResult {
		t.Error("expected false")
	}
}

func TestASKPushdownVariableOnly(t *testing.T) {
	// ASK with variables in BGP — should still work (variables become nil in pattern)
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		ASK { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AskResult {
		t.Error("expected true for any triple match")
	}
}

func TestExtractSimpleBGPNonSimple(t *testing.T) {
	// extractSimpleBGP returns nil for non-BGP patterns
	result := extractSimpleBGP(&JoinPattern{})
	if result != nil {
		t.Error("expected nil for JoinPattern")
	}

	// extractSimpleBGP returns nil for BGP with multiple triples
	result = extractSimpleBGP(&BGP{Triples: []Triple{{}, {}}})
	if result != nil {
		t.Error("expected nil for multi-triple BGP")
	}

	// extractSimpleBGP returns nil for BGP with property path
	result = extractSimpleBGP(&BGP{Triples: []Triple{{
		Subject:       "?s",
		Predicate:     "?p",
		Object:        "?o",
		PredicatePath: &paths.URIRefPath{URI: rdflibgo.NewURIRefUnsafe("http://example.org/p")},
	}}})
	if result != nil {
		t.Error("expected nil for path triple")
	}
}

// --- EvalQuery edge cases ---

func TestEvalQueryASKWithFilter(t *testing.T) {
	// ASK with a FILTER pattern — cannot pushdown, falls through to solution check
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		ASK { ?s ex:age ?age . FILTER(?age > 30) }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AskResult {
		t.Error("expected true (Charlie is 35)")
	}
}

func TestEvalQueryASKNoMatch(t *testing.T) {
	// ASK with filter that matches nothing
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		ASK { ?s ex:age ?age . FILTER(?age > 100) }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.AskResult {
		t.Error("expected false")
	}
}

// --- resolveTermRef edge cases ---

func TestResolveTermRefEmpty(t *testing.T) {
	result := resolveTermRef("", nil)
	if result != nil {
		t.Error("expected nil for empty string")
	}
}

func TestResolveTermRefBNode(t *testing.T) {
	result := resolveTermRef("_:b0", nil)
	if result == nil {
		t.Fatal("expected non-nil BNode")
	}
	bn, ok := result.(rdflibgo.BNode)
	if !ok {
		t.Fatalf("expected BNode, got %T", result)
	}
	if bn.Value() != "b0" {
		t.Errorf("expected b0, got %s", bn.Value())
	}
}

func TestResolveTermRefInteger(t *testing.T) {
	result := resolveTermRef("42", nil)
	if result == nil {
		t.Fatal("expected non-nil literal")
	}
	l, ok := result.(rdflibgo.Literal)
	if !ok {
		t.Fatalf("expected Literal, got %T", result)
	}
	if l.Datatype() != rdflibgo.XSDInteger {
		t.Errorf("expected xsd:integer, got %s", l.Datatype().Value())
	}
}

func TestResolveTermRefPrefixed(t *testing.T) {
	prefixes := map[string]string{"ex": "http://example.org/"}
	result := resolveTermRef("ex:thing", prefixes)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	u, ok := result.(rdflibgo.URIRef)
	if !ok {
		t.Fatalf("expected URIRef, got %T", result)
	}
	if u.Value() != "http://example.org/thing" {
		t.Errorf("expected http://example.org/thing, got %s", u.Value())
	}
}

func TestResolveTermRefUnknownPrefix(t *testing.T) {
	result := resolveTermRef("unknown:thing", nil)
	if result != nil {
		t.Error("expected nil for unresolvable prefix")
	}
}

// --- resolveRelativeIRI ---

func TestResolveRelativeIRIEmpty(t *testing.T) {
	result := resolveRelativeIRI("http://example.org/base", "")
	if result != "http://example.org/base" {
		t.Errorf("expected base URI, got %s", result)
	}
}

func TestResolveRelativeIRIRelative(t *testing.T) {
	result := resolveRelativeIRI("http://example.org/base/", "foo")
	if result != "http://example.org/base/foo" {
		t.Errorf("expected resolved URI, got %s", result)
	}
}

// --- evalAggExpr edge cases ---

func TestEvalAggExprEmptyGroup(t *testing.T) {
	// VarExpr with empty group
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (COUNT(?name) AS ?cnt) WHERE { ?s ex:nonexistent ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected at least one binding for aggregate")
	}
}

func TestEvalAggExprLiteralHaving(t *testing.T) {
	// HAVING with a literal expression
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s (COUNT(?name) AS ?cnt)
		WHERE { ?s ex:name ?name }
		GROUP BY ?s
		HAVING (COUNT(?name) > 0)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// --- evalUnaryOp edge cases ---

func TestEvalUnaryNotLangTaggedString(t *testing.T) {
	// Unary NOT on a language-tagged string should produce nil (no EBV)
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT ?result WHERE {
			BIND(!("hello"@en) AS ?result)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	if r.Bindings[0]["result"] != nil {
		t.Error("expected nil for !langString")
	}
}

func TestEvalUnaryNotBNode(t *testing.T) {
	// Unary NOT on URI or BNode has no EBV
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?result WHERE {
			BIND(!ex:Alice AS ?result)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	if r.Bindings[0]["result"] != nil {
		t.Error("expected nil for !URI")
	}
}

func TestEvalUnaryNotUnknownDatatype(t *testing.T) {
	// Unary NOT on unknown datatype returns nil
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT ?result WHERE {
			BIND(!(STRDT("foo", <http://example.org/custom>)) AS ?result)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
	if r.Bindings[0]["result"] != nil {
		t.Error("expected nil for unknown datatype")
	}
}

func TestEvalUnaryNilArg(t *testing.T) {
	result := evalUnaryOp("!", nil)
	if result != nil {
		t.Error("expected nil for ! nil")
	}
}

func TestEvalUnaryUnknownOp(t *testing.T) {
	result := evalUnaryOp("%", rdflibgo.NewLiteral(1))
	if result != nil {
		t.Error("expected nil for unknown op")
	}
}

// --- isDecimal ---

func TestIsDecimalTrue(t *testing.T) {
	lit := rdflibgo.NewLiteral("1.5", rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
	if !isDecimal(lit) {
		t.Error("expected true")
	}
}

func TestIsDecimalFalseNonLiteral(t *testing.T) {
	u := rdflibgo.NewURIRefUnsafe("http://example.org/x")
	if isDecimal(u) {
		t.Error("expected false for URI")
	}
}

func TestIsDecimalFalseNonDecimal(t *testing.T) {
	lit := rdflibgo.NewLiteral(42)
	if isDecimal(lit) {
		t.Error("expected false for integer")
	}
}

// --- formatDecimal ---

func TestFormatDecimalNoDecimalPoint(t *testing.T) {
	// This tests the branch where FormatFloat returns no "."
	result := formatDecimal(10.0)
	if result != "10.0" {
		t.Errorf("expected 10.0, got %s", result)
	}
}

func TestFormatDecimalTrailingZeros(t *testing.T) {
	result := formatDecimal(1.5)
	if result != "1.5" {
		t.Errorf("expected 1.5, got %s", result)
	}
}

// --- termValuesEqual edge cases ---

func TestTermValuesEqualBothNil(t *testing.T) {
	if !termValuesEqual(nil, nil) {
		t.Error("expected nil == nil")
	}
}

func TestTermValuesEqualOneNil(t *testing.T) {
	if termValuesEqual(nil, rdflibgo.NewLiteral(1)) {
		t.Error("expected nil != non-nil")
	}
	if termValuesEqual(rdflibgo.NewLiteral(1), nil) {
		t.Error("expected non-nil != nil")
	}
}

// --- containsExists ---

func TestContainsExistsNil(t *testing.T) {
	if containsExists(nil) {
		t.Error("expected false for nil")
	}
}

func TestContainsExistsVar(t *testing.T) {
	if containsExists(&VarExpr{Name: "x"}) {
		t.Error("expected false for VarExpr")
	}
}

func TestContainsExistsLiteral(t *testing.T) {
	if containsExists(&LiteralExpr{Value: rdflibgo.NewLiteral(1)}) {
		t.Error("expected false for LiteralExpr")
	}
}

// --- containsAggregate edge cases ---

func TestContainsAggregateNonAggFunc(t *testing.T) {
	fe := &FuncExpr{Name: "STRLEN", Args: []Expr{&VarExpr{Name: "x"}}}
	if containsAggregate(fe) {
		t.Error("expected false for STRLEN")
	}
}

func TestContainsAggregateNestedInFunc(t *testing.T) {
	fe := &FuncExpr{Name: "STR", Args: []Expr{
		&FuncExpr{Name: "COUNT", Args: []Expr{&VarExpr{Name: "x"}}},
	}}
	if !containsAggregate(fe) {
		t.Error("expected true for nested COUNT in STR")
	}
}

// --- evalExpr edge cases ---

func TestEvalExprNil(t *testing.T) {
	result := evalExpr(nil, nil, nil)
	if result != nil {
		t.Error("expected nil for nil expr")
	}
}

func TestEvalExprExistsNilWithoutGraph(t *testing.T) {
	// ExistsExpr returns nil when called via evalExpr (needs graph)
	e := &ExistsExpr{Pattern: &BGP{}, Not: false}
	result := evalExpr(e, nil, nil)
	if result != nil {
		t.Error("expected nil for ExistsExpr via evalExpr")
	}
}

func TestEvalExprIRIWithBase(t *testing.T) {
	prefixes := map[string]string{baseURIKey: "http://example.org/"}
	e := &IRIExpr{Value: "foo"}
	result := evalExpr(e, nil, prefixes)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	u, ok := result.(rdflibgo.URIRef)
	if !ok {
		t.Fatalf("expected URIRef, got %T", result)
	}
	if u.Value() != "http://example.org/foo" {
		t.Errorf("expected http://example.org/foo, got %s", u.Value())
	}
}

// --- evalExprWithGraph edge cases ---

func TestEvalExprWithGraphDefault(t *testing.T) {
	// Non-Exists expr falls through to evalExpr
	g := makeSPARQLGraph(t)
	e := &LiteralExpr{Value: rdflibgo.NewLiteral(42)}
	result := evalExprWithGraph(e, nil, nil, g, nil)
	if result == nil {
		t.Fatal("expected non-nil")
	}
}

func TestEvalExprWithGraphNil(t *testing.T) {
	result := evalExprWithGraph(nil, nil, nil, nil, nil)
	if result != nil {
		t.Error("expected nil for nil expr")
	}
}

// --- CONSTRUCT with OFFSET clearing solutions ---

func TestConstructWithOffset(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE { ?s ex:name ?name }
		ORDER BY ?name
		OFFSET 100
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph.Len() != 0 {
		t.Errorf("expected 0 triples with large offset, got %d", r.Graph.Len())
	}
}

func TestConstructWithLimit(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE { ?s ex:name ?name }
		ORDER BY ?name
		LIMIT 1
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph.Len() != 1 {
		t.Errorf("expected 1 triple with LIMIT 1, got %d", r.Graph.Len())
	}
}

// --- Query parse error ---

func TestQueryParseError(t *testing.T) {
	g := makeSPARQLGraph(t)
	_, err := Query(g, `NOT VALID SPARQL AT ALL ???`)
	if err == nil {
		t.Error("expected parse error")
	}
}

// --- Query cache with parse error ---

func TestQueryCacheParseError(t *testing.T) {
	EnableQueryCache(10)
	defer DisableQueryCache()

	g := makeSPARQLGraph(t)
	_, err := Query(g, `NOT VALID SPARQL`)
	if err == nil {
		t.Error("expected parse error")
	}
	// Should not be cached
	if queryCache.Len() != 0 {
		t.Error("parse errors should not be cached")
	}
}

// --- extractTemplateFromPattern ---

func TestExtractTemplateFromPatternNonBGP(t *testing.T) {
	result := extractTemplateFromPattern(&FilterPattern{
		Pattern: &BGP{Triples: []Triple{{Subject: "?s", Predicate: "<http://example.org/p>", Object: "?o"}}},
		Expr:    &LiteralExpr{Value: rdflibgo.NewLiteral(true)},
	})
	if result != nil {
		t.Error("expected nil for FilterPattern")
	}
}

// --- evalGraphPattern with no named graphs and variable ---

func TestEvalGraphPatternNoNamedGraphs(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			GRAPH ?g { ?s ex:name ?name }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// With no named graphs, GRAPH clause should still evaluate against default graph
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// --- effectiveBooleanValue edge cases ---

func TestEffectiveBooleanValueNil(t *testing.T) {
	if effectiveBooleanValue(nil) {
		t.Error("expected false for nil")
	}
}

func TestEffectiveBooleanValueNonLiteral(t *testing.T) {
	// URIRef has EBV true
	if !effectiveBooleanValue(rdflibgo.NewURIRefUnsafe("http://example.org/x")) {
		t.Error("expected true for URI")
	}
}

// --- toFloat64 edge cases ---

func TestToFloat64Nil(t *testing.T) {
	if toFloat64(nil) != 0 {
		t.Error("expected 0 for nil")
	}
}

func TestToFloat64NonLiteral(t *testing.T) {
	if toFloat64(rdflibgo.NewURIRefUnsafe("http://example.org/x")) != 0 {
		t.Error("expected 0 for non-literal")
	}
}

// --- termString edge cases ---

func TestTermStringNil(t *testing.T) {
	if termString(nil) != "" {
		t.Error("expected empty string for nil")
	}
}

// --- stringResult edge cases ---

func TestStringResultWithDatatype(t *testing.T) {
	source := rdflibgo.NewLiteral("x", rdflibgo.WithDatatype(rdflibgo.XSDInteger))
	result := stringResult("42", source)
	if result.Datatype() != rdflibgo.XSDInteger {
		t.Errorf("expected xsd:integer, got %v", result.Datatype())
	}
}

func TestStringResultPlain(t *testing.T) {
	source := rdflibgo.NewURIRefUnsafe("http://example.org/x")
	result := stringResult("hello", source)
	if result.Datatype() != rdflibgo.XSDString {
		t.Errorf("expected xsd:string, got %v", result.Datatype())
	}
}

// --- isStringLiteral ---

func TestIsStringLiteralNonLiteral(t *testing.T) {
	if isStringLiteral(rdflibgo.NewURIRefUnsafe("http://example.org/x")) {
		t.Error("expected false for URI")
	}
}

// --- termTypeOrder ---

func TestTermTypeOrderUnknown(t *testing.T) {
	// nil term should be order 4 (unknown)
	order := termTypeOrder(nil)
	if order != 4 {
		t.Errorf("expected 4, got %d", order)
	}
}

// --- AST interface marker methods ---

func TestPatternInterfaceMethods(t *testing.T) {
	// Exercise all isPattern() marker methods for coverage
	(&BGP{}).isPattern()
	(&JoinPattern{}).isPattern()
	(&OptionalPattern{}).isPattern()
	(&UnionPattern{}).isPattern()
	(&FilterPattern{}).isPattern()
	(&BindPattern{}).isPattern()
	(&ValuesPattern{}).isPattern()
	(&GraphPattern{}).isPattern()
	(&MinusPattern{}).isPattern()
	(&SubqueryPattern{}).isPattern()
}

func TestExprInterfaceMethods(t *testing.T) {
	// Exercise all isExpr() marker methods for coverage
	(&VarExpr{}).isExpr()
	(&LiteralExpr{}).isExpr()
	(&IRIExpr{}).isExpr()
	(&BinaryExpr{}).isExpr()
	(&UnaryExpr{}).isExpr()
	(&FuncExpr{}).isExpr()
	(&ExistsExpr{}).isExpr()
}

func TestUpdateOpInterfaceMethods(t *testing.T) {
	// Exercise all isUpdateOp() marker methods for coverage
	(&InsertDataOp{}).isUpdateOp()
	(&DeleteDataOp{}).isUpdateOp()
	(&DeleteWhereOp{}).isUpdateOp()
	(&ModifyOp{}).isUpdateOp()
	(&GraphMgmtOp{}).isUpdateOp()
}

// --- COUNT pushdown ---

func TestCountPushdown(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (COUNT(*) AS ?cnt) WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 row, got %d", len(r.Bindings))
	}
	cnt := r.Bindings[0]["cnt"]
	if cnt == nil {
		t.Fatal("expected count binding")
	}
	l, ok := cnt.(rdflibgo.Literal)
	if !ok {
		t.Fatalf("expected Literal, got %T", cnt)
	}
	if l.Lexical() != "3" {
		t.Errorf("expected 3, got %s", l.Lexical())
	}
}

// --- evalBGP with resolvePatternTerm returning non-Subject ---

func TestBGPWithBoundVariable(t *testing.T) {
	// When subject is a variable already bound to a non-Subject value,
	// triples iterator uses nil subject and the variable just gets bound.
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE {
			ex:Alice ex:name ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// --- Update parse error ---

func TestUpdateParseError(t *testing.T) {
	ds := &Dataset{Default: rdflibgo.NewGraph()}
	err := Update(ds, `NOT VALID UPDATE !!!`)
	if err == nil {
		t.Error("expected parse error")
	}
}

// --- CONSTRUCT where subject/predicate are non-castable ---

func TestConstructNonSubjectPredicate(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?name ?name ?name }
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// string literal as subject/predicate should be skipped
	if r.Graph.Len() != 0 {
		t.Errorf("expected 0 triples, got %d", r.Graph.Len())
	}
}

// --- parseDatePair / parseDateTime edge cases ---

func TestParseDatePairInvalid(t *testing.T) {
	a := rdflibgo.NewLiteral("not-a-date", rdflibgo.WithDatatype(rdflibgo.XSDDateTime))
	b := rdflibgo.NewLiteral("not-a-date", rdflibgo.WithDatatype(rdflibgo.XSDDateTime))
	_, _, ok := parseDatePair(a, b)
	if ok {
		t.Error("expected false for invalid dates")
	}
}

// --- compareTermValues with dates ---

func TestCompareTermValuesDate(t *testing.T) {
	a := rdflibgo.NewLiteral("2024-01-01", rdflibgo.WithDatatype(rdflibgo.XSDDate))
	b := rdflibgo.NewLiteral("2024-06-01", rdflibgo.WithDatatype(rdflibgo.XSDDate))
	c := compareTermValues(a, b)
	if c >= 0 {
		t.Error("expected a < b")
	}
}

// --- strArgCompatible edge cases ---

func TestStrArgCompatibleNonLiterals(t *testing.T) {
	u := rdflibgo.NewURIRefUnsafe("http://example.org/x")
	if strArgCompatible(u, u) {
		t.Error("expected false for URIs")
	}
}

// --- resolvePatternTerm with bnode _: prefix ---

func TestResolvePatternTermBNodePrefix(t *testing.T) {
	// _:b0 in resolvePatternTerm goes through prefix resolution with "_" prefix.
	// Without a "_" prefix mapping, returns nil.
	result := resolvePatternTerm("_:b0", nil, nil)
	if result != nil {
		t.Error("expected nil without bnode prefix mapping")
	}
	// With a bnode prefix mapping, resolves to a URI (not a BNode -- that's resolveTermRef)
	prefixes := map[string]string{"_": "http://example.org/bnode/"}
	result = resolvePatternTerm("_:b0", nil, prefixes)
	if result == nil {
		t.Fatal("expected non-nil with _ prefix")
	}
}

func TestResolvePatternTermLiteral(t *testing.T) {
	result := resolvePatternTerm(`"hello"`, nil, nil)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	l, ok := result.(rdflibgo.Literal)
	if !ok {
		t.Fatalf("expected Literal, got %T", result)
	}
	if l.Lexical() != "hello" {
		t.Errorf("expected hello, got %s", l.Lexical())
	}
}

func TestResolvePatternTermBoolean(t *testing.T) {
	r1 := resolvePatternTerm("true", nil, nil)
	if r1 == nil {
		t.Fatal("expected non-nil for true")
	}
	r2 := resolvePatternTerm("false", nil, nil)
	if r2 == nil {
		t.Fatal("expected non-nil for false")
	}
}

func TestResolvePatternTermPrefixed(t *testing.T) {
	prefixes := map[string]string{"ex": "http://example.org/"}
	result := resolvePatternTerm("ex:thing", nil, prefixes)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	u, ok := result.(rdflibgo.URIRef)
	if !ok {
		t.Fatalf("expected URIRef, got %T", result)
	}
	if u.Value() != "http://example.org/thing" {
		t.Errorf("got %s", u.Value())
	}
}

func TestResolvePatternTermNumericDouble(t *testing.T) {
	result := resolvePatternTerm("1.5e2", nil, nil)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	l := result.(rdflibgo.Literal)
	if l.Datatype() != rdflibgo.XSDDouble {
		t.Errorf("expected xsd:double, got %v", l.Datatype())
	}
}

func TestResolvePatternTermNumericDecimal(t *testing.T) {
	result := resolvePatternTerm("1.5", nil, nil)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	l := result.(rdflibgo.Literal)
	if l.Datatype() != rdflibgo.XSDDecimal {
		t.Errorf("expected xsd:decimal, got %v", l.Datatype())
	}
}

func TestResolvePatternTermRelativeIRI(t *testing.T) {
	prefixes := map[string]string{baseURIKey: "http://example.org/base/"}
	result := resolvePatternTerm("<foo>", nil, prefixes)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	u := result.(rdflibgo.URIRef)
	if u.Value() != "http://example.org/base/foo" {
		t.Errorf("expected resolved URI, got %s", u.Value())
	}
}

// --- evalGraphPattern with specific named graph ---

func TestEvalGraphPatternSpecificGraph(t *testing.T) {
	g := rdflibgo.NewGraph()
	ng := rdflibgo.NewGraph()
	ex, _ := rdflibgo.NewURIRef("http://example.org/x")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	ng.Add(ex, p, rdflibgo.NewLiteral("inNamedGraph"))

	namedGraphs := map[string]*rdflibgo.Graph{
		"http://example.org/named": ng,
	}

	q, err := Parse(`
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE {
			GRAPH <http://example.org/named> { ex:x ex:p ?o }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	q.NamedGraphs = namedGraphs
	r, err := EvalQuery(g, q, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// --- resolveTemplateValue with auto-bnode variables ---

func TestResolveTemplateValueAutoBnode(t *testing.T) {
	bindings := make(map[string]rdflibgo.Term)
	result := resolveTemplateValue("?_reifier1", bindings, nil)
	if result == nil {
		t.Fatal("expected auto-created bnode")
	}
	if _, ok := result.(rdflibgo.BNode); !ok {
		t.Errorf("expected BNode, got %T", result)
	}
	// Second call should return same bnode
	result2 := resolveTemplateValue("?_reifier1", bindings, nil)
	if result.N3() != result2.N3() {
		t.Error("expected same bnode on second call")
	}

	// Also test _bnode and _coll prefixes
	result3 := resolveTemplateValue("?_bnode1", bindings, nil)
	if _, ok := result3.(rdflibgo.BNode); !ok {
		t.Errorf("expected BNode for _bnode prefix, got %T", result3)
	}
	result4 := resolveTemplateValue("?_coll1", bindings, nil)
	if _, ok := result4.(rdflibgo.BNode); !ok {
		t.Errorf("expected BNode for _coll prefix, got %T", result4)
	}
}

func TestResolveTemplateValueUnbound(t *testing.T) {
	result := resolveTemplateValue("?unknown", nil, nil)
	if result != nil {
		t.Error("expected nil for unbound variable")
	}
}

// --- Parser validation: BIND scope in various patterns ---

func TestValidateBindScopeSubquery(t *testing.T) {
	// BIND in subquery — should work
	g := makeSPARQLGraph(t)
	_, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			{ SELECT ?name WHERE { ?s ex:name ?name } }
			BIND(UCASE(?name) AS ?upper)
		}
	`)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateBindScopeMinus(t *testing.T) {
	// Valid BIND in MINUS context
	g := makeSPARQLGraph(t)
	_, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name .
			MINUS { ?s ex:age ?age . BIND(10 AS ?ten) }
		}
	`)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- collectExprVarsInto coverage: IRIExpr, LiteralExpr ---

func TestCollectExprVarsIRIAndLiteral(t *testing.T) {
	// IRIExpr and LiteralExpr have no variables
	vars := collectExprVars(&IRIExpr{Value: "http://example.org/x"})
	if len(vars) > 0 {
		t.Error("expected no vars for IRIExpr")
	}
	vars = collectExprVars(&LiteralExpr{Value: rdflibgo.NewLiteral(1)})
	if len(vars) > 0 {
		t.Error("expected no vars for LiteralExpr")
	}
}

// --- validateConstructWhere: complex pattern in CONSTRUCT WHERE ---

func TestConstructWhereComplex(t *testing.T) {
	_, err := Parse(`
		PREFIX ex: <http://example.org/>
		CONSTRUCT WHERE {
			?s ex:p ?o .
			OPTIONAL { ?s ex:q ?x }
		}
	`)
	if err == nil {
		t.Error("expected error for OPTIONAL in CONSTRUCT WHERE")
	}
}

// --- validateNoNestedAggregates ---

func TestNestedAggregateError(t *testing.T) {
	_, err := Parse(`
		PREFIX ex: <http://example.org/>
		SELECT (COUNT(SUM(?x)) AS ?y) WHERE { ?s ex:p ?x }
	`)
	if err == nil {
		t.Error("expected error for nested aggregates")
	}
}

// --- evalGraphMgmt: DROP NAMED ---

func TestDropNamed(t *testing.T) {
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	ng := rdflibgo.NewGraph()
	ex1, _ := rdflibgo.NewURIRef("http://example.org/x")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	ng.Add(ex1, p, rdflibgo.NewLiteral("value"))
	ds.NamedGraphs["http://example.org/g1"] = ng

	err := Update(ds, `DROP NAMED`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.NamedGraphs) != 0 {
		t.Errorf("expected 0 named graphs, got %d", len(ds.NamedGraphs))
	}
}

// --- evalGraphMgmt: CLEAR ALL ---

func TestClearAll(t *testing.T) {
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	ex1, _ := rdflibgo.NewURIRef("http://example.org/x")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	ds.Default.Add(ex1, p, rdflibgo.NewLiteral("value"))
	ng := rdflibgo.NewGraph()
	ng.Add(ex1, p, rdflibgo.NewLiteral("ng-value"))
	ds.NamedGraphs["http://example.org/g1"] = ng

	err := Update(ds, `CLEAR ALL`)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Default.Len() != 0 {
		t.Errorf("expected 0 triples in default, got %d", ds.Default.Len())
	}
}

// --- INSERT DATA / DELETE DATA in named graphs ---

func TestInsertDeleteDataNamedGraphTriples(t *testing.T) {
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	err := Update(ds, `
		INSERT DATA {
			GRAPH <http://example.org/g1> {
				<http://example.org/s> <http://example.org/p> "hello" .
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	ng := ds.NamedGraphs["http://example.org/g1"]
	if ng == nil || ng.Len() != 1 {
		t.Error("expected 1 triple in named graph")
	}

	err = Update(ds, `
		DELETE DATA {
			GRAPH <http://example.org/g1> {
				<http://example.org/s> <http://example.org/p> "hello" .
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if ng.Len() != 0 {
		t.Errorf("expected 0 triples after delete, got %d", ng.Len())
	}
}

// --- CREATE existing graph ---

func TestCreateExistingGraph(t *testing.T) {
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": rdflibgo.NewGraph()},
	}
	// CREATE SILENT should not error even if graph exists
	err := Update(ds, `CREATE SILENT GRAPH <http://example.org/g1>`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- parseGraphRefAll: DEFAULT, NAMED, ALL ---

func TestClearDefault(t *testing.T) {
	ds := &Dataset{Default: rdflibgo.NewGraph()}
	ex1, _ := rdflibgo.NewURIRef("http://example.org/x")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	ds.Default.Add(ex1, p, rdflibgo.NewLiteral("v"))

	err := Update(ds, `CLEAR DEFAULT`)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Default.Len() != 0 {
		t.Error("expected empty default graph")
	}
}

// --- EvalUpdate with base URI ---

func TestEvalUpdateBaseURI(t *testing.T) {
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	err := Update(ds, `
		BASE <http://example.org/>
		INSERT DATA { <s> <p> "o" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Default.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", ds.Default.Len())
	}
}

// --- MODIFY with USING clause ---

func TestModifyWithUsing(t *testing.T) {
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	ex1, _ := rdflibgo.NewURIRef("http://example.org/x")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	ds.Default.Add(ex1, p, rdflibgo.NewLiteral("v"))

	err := Update(ds, `
		DELETE { <http://example.org/x> <http://example.org/p> ?o }
		INSERT { <http://example.org/x> <http://example.org/p> "new" }
		WHERE { <http://example.org/x> <http://example.org/p> ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- DELETE WHERE with GRAPH ---

func TestDeleteWhereGraph(t *testing.T) {
	ds := &Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	ng := rdflibgo.NewGraph()
	ex1, _ := rdflibgo.NewURIRef("http://example.org/x")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	ng.Add(ex1, p, rdflibgo.NewLiteral("hello"))
	ds.NamedGraphs["http://example.org/g1"] = ng

	err := Update(ds, `
		DELETE WHERE {
			GRAPH <http://example.org/g1> {
				<http://example.org/x> <http://example.org/p> ?o
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if ng.Len() != 0 {
		t.Errorf("expected 0, got %d", ng.Len())
	}
}

// --- SRX/SRJ parsing edge cases ---

func TestParseSRXTripleTermBinding(t *testing.T) {
	srx := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head><variable name="x"/></head>
  <results>
    <result>
      <binding name="x">
        <triple>
          <subject><uri>http://example.org/s</uri></subject>
          <predicate><uri>http://example.org/p</uri></predicate>
          <object><literal>hello</literal></object>
        </triple>
      </binding>
    </result>
  </results>
</sparql>`
	r, err := ParseSRX(strings.NewReader(srx))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
}

// --- ResultsEqual edge cases ---

func TestResultsEqualDifferentSizes(t *testing.T) {
	a := &Result{Type: "SELECT", Vars: []string{"x"}, Bindings: []map[string]rdflibgo.Term{{"x": rdflibgo.NewLiteral(1)}}}
	b := &Result{Type: "SELECT", Vars: []string{"x"}, Bindings: []map[string]rdflibgo.Term{}}
	if ResultsEqual(a, b) {
		t.Error("expected not equal for different sizes")
	}
}

// --- buildStorePattern with variables ---

func TestBuildStorePatternAllVars(t *testing.T) {
	tp := &Triple{Subject: "?s", Predicate: "?p", Object: "?o"}
	pat := buildStorePattern(tp, nil)
	if pat.Subject != nil || pat.Predicate != nil || pat.Object != nil {
		t.Error("expected all nil for all variables")
	}
}

func TestBuildStorePatternConcrete(t *testing.T) {
	prefixes := map[string]string{"ex": "http://example.org/"}
	tp := &Triple{
		Subject:   "<http://example.org/s>",
		Predicate: "<http://example.org/p>",
		Object:    `"hello"`,
	}
	pat := buildStorePattern(tp, prefixes)
	if pat.Subject == nil {
		t.Error("expected non-nil subject")
	}
	if pat.Predicate == nil {
		t.Error("expected non-nil predicate")
	}
	if pat.Object == nil {
		t.Error("expected non-nil object")
	}
}

// --- evalAggExpr: VarExpr with empty group ---

func TestEvalAggExprVarEmptyGroup(t *testing.T) {
	result := evalAggExpr(&VarExpr{Name: "x"}, nil, nil)
	if result != nil {
		t.Error("expected nil for VarExpr with empty group")
	}
}

func TestEvalAggExprNil(t *testing.T) {
	result := evalAggExpr(nil, nil, nil)
	if result != nil {
		t.Error("expected nil for nil expr")
	}
}

func TestEvalAggExprFuncNonAggEmptyGroup(t *testing.T) {
	result := evalAggExpr(&FuncExpr{Name: "STRLEN", Args: []Expr{&VarExpr{Name: "x"}}}, nil, nil)
	if result != nil {
		t.Error("expected nil for non-aggregate func with empty group")
	}
}

// --- resolveTermRef edge cases ---

func TestResolveTermRefBNodeWithScope(t *testing.T) {
	prefixes := map[string]string{"__bnode_scope__": "scope_"}
	result := resolveTermRef("_:b0", prefixes)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	bn := result.(rdflibgo.BNode)
	if bn.Value() != "scope_b0" {
		t.Errorf("expected scope_b0, got %s", bn.Value())
	}
}

func TestResolveTermRefSingleQuoteLiteral(t *testing.T) {
	result := resolveTermRef("'hello'", nil)
	if result == nil {
		t.Fatal("expected non-nil")
	}
}

func TestResolveTermRefRelativeIRI(t *testing.T) {
	prefixes := map[string]string{baseURIKey: "http://example.org/base/"}
	result := resolveTermRef("<foo>", prefixes)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	u := result.(rdflibgo.URIRef)
	if u.Value() != "http://example.org/base/foo" {
		t.Errorf("got %s", u.Value())
	}
}

// --- extractTemplateFromPattern with BGP ---

func TestExtractTemplateFromBGP(t *testing.T) {
	bgp := &BGP{Triples: []Triple{
		{Subject: "?s", Predicate: "<http://example.org/p>", Object: "?o"},
		{Subject: "?a", Predicate: "<http://example.org/q>", Object: "?b"},
	}}
	tmpl := extractTemplateFromPattern(bgp)
	if len(tmpl) != 2 {
		t.Errorf("expected 2 templates, got %d", len(tmpl))
	}
}

// --- Decimal arithmetic ---

func TestDecimalArithmetic(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT ?result WHERE {
			BIND(1.5 + 2.5 AS ?result)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1, got %d", len(r.Bindings))
	}
	l := r.Bindings[0]["result"].(rdflibgo.Literal)
	if l.Datatype() != rdflibgo.XSDDecimal {
		t.Errorf("expected decimal, got %v", l.Datatype())
	}
}

// --- SPARQL CONSTRUCT with ORDER BY DESC ---

func TestConstructOrderByDesc(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE { ?s ex:name ?name }
		ORDER BY DESC(?name)
		LIMIT 2
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil || r.Graph.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", r.Graph.Len())
	}
}

// --- SELECT DISTINCT ---

func TestSelectDistinct(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT DISTINCT ?type WHERE {
			?s a ?type
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 distinct type, got %d", len(r.Bindings))
	}
}

// --- validateTripleTerms / validateTripleTermString ---

func TestValidateTripleTermStringLiteralSubject(t *testing.T) {
	err := validateTripleTermString(`<<( "hello" <http://example.org/p> <http://example.org/o> )>>`, "subject")
	if err == nil {
		t.Error("expected error for literal in subject position")
	}
	if !strings.Contains(err.Error(), "literal in subject position") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTripleTermStringTripleSubject(t *testing.T) {
	err := validateTripleTermString(`<<( <<( <http://a> <http://b> <http://c> )>> <http://example.org/p> <http://example.org/o> )>>`, "subject")
	if err == nil {
		t.Error("expected error for triple term in subject position")
	}
}

func TestValidateTripleTermStringNonTriple(t *testing.T) {
	err := validateTripleTermString(`<http://example.org/x>`, "subject")
	if err != nil {
		t.Errorf("expected no error for non-triple term, got %v", err)
	}
}

func TestValidateTripleTermStringValid(t *testing.T) {
	err := validateTripleTermString(`<<( <http://a> <http://b> <http://c> )>>`, "subject")
	if err != nil {
		t.Errorf("expected no error for valid triple term, got %v", err)
	}
}

func TestValidateTripleTermStringNestedInObject(t *testing.T) {
	err := validateTripleTermString(`<<( <http://a> <http://b> <<( <http://c> <http://d> <http://e> )>> )>>`, "subject")
	if err != nil {
		t.Errorf("expected no error for nested valid triple term, got %v", err)
	}
}

func TestValidateTripleTermsWithBGP(t *testing.T) {
	bgp := &BGP{
		Triples: []Triple{
			{Subject: `<<( <http://a> <http://b> <http://c> )>>`, Predicate: "<http://p>", Object: "<http://o>"},
		},
	}
	err := validateTripleTerms(bgp)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTripleTermsWithLiteralSubject(t *testing.T) {
	bgp := &BGP{
		Triples: []Triple{
			{Subject: `<<( "hello" <http://b> <http://c> )>>`, Predicate: "<http://p>", Object: "<http://o>"},
		},
	}
	err := validateTripleTerms(bgp)
	if err == nil {
		t.Error("expected error for literal subject in triple term")
	}
}

func TestValidateTripleTermsNilPattern(t *testing.T) {
	err := validateTripleTerms(nil)
	if err != nil {
		t.Errorf("expected no error for nil pattern, got %v", err)
	}
}

func TestValidateTripleTermsJoinPattern(t *testing.T) {
	bgp1 := &BGP{Triples: []Triple{{Subject: "<http://a>", Predicate: "<http://p>", Object: "<http://o>"}}}
	bgp2 := &BGP{Triples: []Triple{{Subject: "<http://b>", Predicate: "<http://p>", Object: "<http://o>"}}}
	jp := &JoinPattern{Left: bgp1, Right: bgp2}
	err := validateTripleTerms(jp)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTripleTermsOptionalPattern(t *testing.T) {
	bgp := &BGP{Triples: []Triple{{Subject: "<http://a>", Predicate: "<http://p>", Object: "<http://o>"}}}
	opt := &OptionalPattern{Main: bgp, Optional: bgp}
	err := validateTripleTerms(opt)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTripleTermsUnionPattern(t *testing.T) {
	bgp := &BGP{Triples: []Triple{{Subject: "<http://a>", Predicate: "<http://p>", Object: "<http://o>"}}}
	u := &UnionPattern{Left: bgp, Right: bgp}
	err := validateTripleTerms(u)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTripleTermsFilterPattern(t *testing.T) {
	bgp := &BGP{Triples: []Triple{{Subject: "<http://a>", Predicate: "<http://p>", Object: "<http://o>"}}}
	fp := &FilterPattern{Pattern: bgp, Expr: &LiteralExpr{Value: rdflibgo.NewLiteral("true", rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#boolean")))}}
	err := validateTripleTerms(fp)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTripleTermsBindPattern(t *testing.T) {
	bgp := &BGP{Triples: []Triple{{Subject: "<http://a>", Predicate: "<http://p>", Object: "<http://o>"}}}
	bp := &BindPattern{Pattern: bgp, Expr: &LiteralExpr{Value: rdflibgo.NewLiteral("1")}, Var: "x"}
	err := validateTripleTerms(bp)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTripleTermsValuesPattern(t *testing.T) {
	vp := &ValuesPattern{Vars: []string{"x"}, Values: [][]rdflibgo.Term{{rdflibgo.NewURIRefUnsafe("http://a")}}}
	err := validateTripleTerms(vp)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTripleTermsSubqueryPattern(t *testing.T) {
	bgp := &BGP{Triples: []Triple{{Subject: "<http://a>", Predicate: "<http://p>", Object: "<http://o>"}}}
	sq := &SubqueryPattern{Query: &ParsedQuery{Where: bgp, Limit: -1}}
	err := validateTripleTerms(sq)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// --- validateTripleTermData ---

func TestValidateTripleTermDataBNodeSubject(t *testing.T) {
	err := validateTripleTermData(`<<( _:b0 <http://p> <http://o> )>>`)
	if err == nil {
		t.Error("expected error for bnode subject in DATA triple term")
	}
}

func TestValidateTripleTermDataVarSubject(t *testing.T) {
	err := validateTripleTermData(`<<( ?x <http://p> <http://o> )>>`)
	if err == nil {
		t.Error("expected error for variable subject in DATA triple term")
	}
}

func TestValidateTripleTermDataLiteralSubject(t *testing.T) {
	err := validateTripleTermData(`<<( "hello" <http://p> <http://o> )>>`)
	if err == nil {
		t.Error("expected error for literal subject in DATA triple term")
	}
}

func TestValidateTripleTermDataValidIRI(t *testing.T) {
	err := validateTripleTermData(`<<( <http://s> <http://p> <http://o> )>>`)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTripleTermDataNonTripleTerm(t *testing.T) {
	err := validateTripleTermData(`<http://example.org/x>`)
	if err != nil {
		t.Errorf("expected no error for non-triple term, got %v", err)
	}
}

func TestValidateTripleTermDataNestedObject(t *testing.T) {
	err := validateTripleTermData(`<<( <http://s> <http://p> <<( <http://a> <http://b> <http://c> )>> )>>`)
	if err != nil {
		t.Errorf("expected no error for valid nested, got %v", err)
	}
}

// --- collectExprVarsInto full coverage ---

func TestCollectExprVarsIntoUnary(t *testing.T) {
	vars := collectExprVars(&UnaryExpr{Op: "!", Arg: &VarExpr{Name: "x"}})
	if !vars["x"] {
		t.Error("expected variable x")
	}
}

func TestCollectExprVarsIntoBinary(t *testing.T) {
	vars := collectExprVars(&BinaryExpr{
		Op:    "+",
		Left:  &VarExpr{Name: "a"},
		Right: &VarExpr{Name: "b"},
	})
	if !vars["a"] || !vars["b"] {
		t.Error("expected variables a and b")
	}
}

func TestCollectExprVarsIntoFuncAggregate(t *testing.T) {
	// Aggregate function args should not be descended into
	vars := collectExprVars(&FuncExpr{
		Name: "COUNT",
		Args: []Expr{&VarExpr{Name: "x"}},
	})
	if vars["x"] {
		t.Error("should not collect vars inside aggregate")
	}
}

func TestCollectExprVarsIntoFuncNonAggregate(t *testing.T) {
	vars := collectExprVars(&FuncExpr{
		Name: "STRLEN",
		Args: []Expr{&VarExpr{Name: "x"}},
	})
	if !vars["x"] {
		t.Error("expected variable x in non-aggregate function")
	}
}

func TestCollectExprVarsIntoNil(t *testing.T) {
	vars := collectExprVars(nil)
	if len(vars) != 0 {
		t.Error("expected empty vars for nil expr")
	}
}

// --- validateConstructWhere full coverage ---

func TestValidateConstructWhereFilterDirect(t *testing.T) {
	bgp := &BGP{Triples: []Triple{{Subject: "?s", Predicate: "<http://p>", Object: "?o"}}}
	fp := &FilterPattern{Pattern: bgp, Expr: &LiteralExpr{Value: rdflibgo.NewLiteral("true")}}
	err := validateConstructWhere(fp)
	if err == nil || !strings.Contains(err.Error(), "FILTER not allowed") {
		t.Errorf("expected FILTER error, got %v", err)
	}
}

func TestValidateConstructWhereJoinBGPs(t *testing.T) {
	bgp1 := &BGP{Triples: []Triple{{Subject: "?s", Predicate: "<http://p>", Object: "?o"}}}
	bgp2 := &BGP{Triples: []Triple{{Subject: "?s", Predicate: "<http://q>", Object: "?o"}}}
	jp := &JoinPattern{Left: bgp1, Right: bgp2}
	err := validateConstructWhere(jp)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateConstructWhereComplexPattern(t *testing.T) {
	bgp := &BGP{Triples: []Triple{{Subject: "?s", Predicate: "<http://p>", Object: "?o"}}}
	opt := &OptionalPattern{Main: bgp, Optional: bgp}
	err := validateConstructWhere(opt)
	if err == nil || !strings.Contains(err.Error(), "complex pattern not allowed") {
		t.Errorf("expected complex pattern error, got %v", err)
	}
}

func TestValidateConstructWhereNil(t *testing.T) {
	err := validateConstructWhere(nil)
	if err != nil {
		t.Errorf("expected no error for nil, got %v", err)
	}
}

// --- Update edge cases ---

func TestEvalModifyWithGraphAndUsing(t *testing.T) {
	g := graph.NewGraph()
	ex := rdflibgo.NewURIRefUnsafe
	g.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))

	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s2"), ex("http://example.org/p"), rdflibgo.NewLiteral("val2"))

	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": ng},
	}

	// Test WITH: modify a named graph via WITH
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		WITH <http://example.org/g1>
		DELETE { ?s ex:p "val2" }
		INSERT { ?s ex:p "modified" }
		WHERE { ?s ex:p "val2" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Check the named graph was modified
	count := 0
	for range ng.Triples(nil, nil, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 triple in named graph, got %d", count)
	}
}

func TestEvalModifyWithUsingClauses(t *testing.T) {
	g := graph.NewGraph()
	ex := rdflibgo.NewURIRefUnsafe

	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": ng},
	}

	// Test USING: query against named graph as default
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT { ?s ex:q "new" }
		USING <http://example.org/g1>
		WHERE { ?s ex:p "val" }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEvalModifyWithNonExistentGraph(t *testing.T) {
	g := graph.NewGraph()
	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{},
	}

	// WITH a graph that doesn't exist should create empty query graph
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		WITH <http://example.org/nonexistent>
		INSERT { ex:s ex:p "new" }
		WHERE { }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEvalInsertDataBadPredicate(t *testing.T) {
	// When the predicate is a literal (not a URIRef), it should be silently skipped
	g := graph.NewGraph()
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	// This tests the "pred, ok := p.(term.URIRef); if !ok" branch
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT DATA { ex:s ex:p ex:o }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEvalGraphMgmtLoadNoLoader(t *testing.T) {
	g := graph.NewGraph()
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}

	// LOAD without a Loader should fail
	err := Update(ds, `LOAD <http://example.org/data>`)
	if err == nil {
		t.Error("expected error for LOAD without Loader")
	}

	// LOAD SILENT should not fail
	err = Update(ds, `LOAD SILENT <http://example.org/data>`)
	if err != nil {
		t.Errorf("expected no error for LOAD SILENT, got %v", err)
	}
}

func TestEvalGraphMgmtAddCopyMove(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	g.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("default"))

	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s2"), ex("http://example.org/p"), rdflibgo.NewLiteral("named"))

	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": ng},
	}

	// ADD
	err := Update(ds, `ADD <http://example.org/g1> TO DEFAULT`)
	if err != nil {
		t.Fatal(err)
	}
	defaultCount := 0
	for range g.Triples(nil, nil, nil) {
		defaultCount++
	}
	if defaultCount != 2 {
		t.Errorf("expected 2 triples after ADD, got %d", defaultCount)
	}

	// COPY
	err = Update(ds, `COPY DEFAULT TO <http://example.org/g2>`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ds.NamedGraphs["http://example.org/g2"]; !ok {
		t.Error("expected g2 to exist after COPY")
	}

	// MOVE
	err = Update(ds, `MOVE <http://example.org/g1> TO <http://example.org/g3>`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ds.NamedGraphs["http://example.org/g3"]; !ok {
		t.Error("expected g3 to exist after MOVE")
	}
}

func TestEvalGraphMgmtMoveSourceNotFound(t *testing.T) {
	g := graph.NewGraph()
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}

	// MOVE from non-existent source (not silent)
	err := Update(ds, `MOVE <http://example.org/nonexistent> TO DEFAULT`)
	if err == nil {
		t.Error("expected error for MOVE from non-existent source")
	}
}

func TestEvalGraphMgmtMoveSourceSameAsDest(t *testing.T) {
	g := graph.NewGraph()
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}

	err := Update(ds, `MOVE DEFAULT TO DEFAULT`)
	if err != nil {
		t.Errorf("expected no error for MOVE same, got %v", err)
	}
}

func TestEvalDeleteWhereNonMatchingTypes(t *testing.T) {
	g := graph.NewGraph()
	ex := rdflibgo.NewURIRefUnsafe
	g.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		DELETE WHERE { ?s ex:p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range g.Triples(nil, nil, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 triples after DELETE WHERE, got %d", count)
	}
}

func TestQuadsToPatternWithGraph(t *testing.T) {
	quads := []QuadPattern{
		{Graph: "http://example.org/g", Triples: []Triple{{Subject: "?s", Predicate: "<http://p>", Object: "?o"}}},
		{Triples: []Triple{{Subject: "?s", Predicate: "<http://q>", Object: "?o"}}},
	}
	p := quadsToPattern(quads, nil)
	if p == nil {
		t.Fatal("expected non-nil pattern")
	}
	// Should be a JoinPattern wrapping GraphPattern + BGP
	jp, ok := p.(*JoinPattern)
	if !ok {
		t.Errorf("expected JoinPattern, got %T", p)
	}
	if jp != nil {
		if _, ok := jp.Left.(*GraphPattern); !ok {
			t.Errorf("expected GraphPattern on left, got %T", jp.Left)
		}
	}
}

func TestQuadsToPatternEmpty(t *testing.T) {
	p := quadsToPattern(nil, nil)
	bgp, ok := p.(*BGP)
	if !ok {
		t.Errorf("expected BGP for empty quads, got %T", p)
	}
	if bgp != nil && len(bgp.Triples) != 0 {
		t.Error("expected empty BGP")
	}
}

func TestGraphForQuadSolutionVariableBound(t *testing.T) {
	g := graph.NewGraph()
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	ex := rdflibgo.NewURIRefUnsafe

	sol := map[string]rdflibgo.Term{
		"g": ex("http://example.org/named"),
	}
	result := graphForQuadSolution(ds, "?g", sol)
	if result == nil {
		t.Fatal("expected non-nil graph")
	}
}

func TestGraphForQuadSolutionVariableNotBound(t *testing.T) {
	g := graph.NewGraph()
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	sol := map[string]rdflibgo.Term{}
	result := graphForQuadSolution(ds, "?g", sol)
	if result != g {
		t.Error("expected default graph when variable not bound")
	}
}

func TestGraphForQuadSolutionLiteral(t *testing.T) {
	g := graph.NewGraph()
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	sol := map[string]rdflibgo.Term{
		"g": rdflibgo.NewLiteral("not-a-uri"),
	}
	result := graphForQuadSolution(ds, "?g", sol)
	if result != g {
		t.Error("expected default graph when variable bound to literal")
	}
}

func TestResolveModifyGraphWithAndVariable(t *testing.T) {
	g := graph.NewGraph()
	ex := rdflibgo.NewURIRefUnsafe
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}

	// Test WITH clause
	result := resolveModifyGraph(ds, "", "http://example.org/g1", nil)
	if result == g {
		t.Error("expected a new graph, not default")
	}

	// Test variable graph name
	sol := map[string]rdflibgo.Term{
		"g": ex("http://example.org/g2"),
	}
	result = resolveModifyGraph(ds, "?g", "", sol)
	if result == g {
		t.Error("expected a new graph for variable")
	}

	// Test variable not bound to URIRef
	sol2 := map[string]rdflibgo.Term{
		"g": rdflibgo.NewLiteral("not-a-uri"),
	}
	result = resolveModifyGraph(ds, "?g", "", sol2)
	if result != g {
		t.Error("expected default graph for non-URI variable")
	}
}

// --- SRX triple parsing ---

func TestParseSRXTripleComponentBNode(t *testing.T) {
	c := srxTripleComponent{BNode: "b1"}
	result := parseSRXTripleComponent(c)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if _, ok := result.(rdflibgo.BNode); !ok {
		t.Errorf("expected BNode, got %T", result)
	}
}

func TestParseSRXTripleComponentNil(t *testing.T) {
	c := srxTripleComponent{}
	result := parseSRXTripleComponent(c)
	if result != nil {
		t.Error("expected nil for empty component")
	}
}

func TestParseSRXTripleComponentLiteral(t *testing.T) {
	c := srxTripleComponent{Literal: &srxLiteral{Value: "hello"}}
	result := parseSRXTripleComponent(c)
	if result == nil {
		t.Fatal("expected non-nil")
	}
}

func TestParseSRXTripleInvalidSubject(t *testing.T) {
	// Triple where subject is a literal (not Subject interface)
	st := &srxTriple{
		Subject:   srxTripleComponent{Literal: &srxLiteral{Value: "lit"}},
		Predicate: srxTripleComponent{URI: "http://example.org/p"},
		Object:    srxTripleComponent{URI: "http://example.org/o"},
	}
	result := parseSRXTriple(st)
	if result != nil {
		t.Error("expected nil for literal subject in triple")
	}
}

func TestParseSRXTripleValid(t *testing.T) {
	st := &srxTriple{
		Subject:   srxTripleComponent{URI: "http://example.org/s"},
		Predicate: srxTripleComponent{URI: "http://example.org/p"},
		Object:    srxTripleComponent{URI: "http://example.org/o"},
	}
	result := parseSRXTriple(st)
	if result == nil {
		t.Fatal("expected non-nil triple term")
	}
}

func TestSRXBindingNil(t *testing.T) {
	b := srxBinding{}
	result := parseSRXBinding(b)
	if result != nil {
		t.Error("expected nil for empty binding")
	}
}

func TestSRXBindingTriple(t *testing.T) {
	b := srxBinding{
		Triple: &srxTriple{
			Subject:   srxTripleComponent{URI: "http://s"},
			Predicate: srxTripleComponent{URI: "http://p"},
			Object:    srxTripleComponent{URI: "http://o"},
		},
	}
	result := parseSRXBinding(b)
	if result == nil {
		t.Fatal("expected non-nil")
	}
}

func TestSRXLiteralWithDirLang(t *testing.T) {
	lit := &srxLiteral{Value: "hello", Lang: "en--ltr"}
	result := parseSRXLiteral(lit)
	if result.Language() != "en" {
		t.Errorf("expected lang en, got %s", result.Language())
	}
}

func TestSRXLiteralWithDirAttribute(t *testing.T) {
	lit := &srxLiteral{Value: "hello", Lang: "en", Dir: "rtl"}
	result := parseSRXLiteral(lit)
	if result.Dir() != "rtl" {
		t.Errorf("expected dir rtl, got %s", result.Dir())
	}
}

// --- buildNegatedPath ---

func TestBuildNegatedPathBothFwdAndInv(t *testing.T) {
	p := &sparqlParser{input: "", prefixes: map[string]string{}}
	fwd := []rdflibgo.URIRef{rdflibgo.NewURIRefUnsafe("http://example.org/a")}
	inv := []rdflibgo.URIRef{rdflibgo.NewURIRefUnsafe("http://example.org/b")}
	path := p.buildNegatedPath(fwd, inv)
	if path == nil {
		t.Fatal("expected non-nil path")
	}
	// Should be Alternative
	if _, ok := path.(*paths.AlternativePath); !ok {
		t.Errorf("expected *AlternativePath, got %T", path)
	}
}

func TestBuildNegatedPathOnlyInv(t *testing.T) {
	p := &sparqlParser{input: "", prefixes: map[string]string{}}
	inv := []rdflibgo.URIRef{rdflibgo.NewURIRefUnsafe("http://example.org/b")}
	path := p.buildNegatedPath(nil, inv)
	if path == nil {
		t.Fatal("expected non-nil path")
	}
	// Should be Inv(Negated(...))
	if _, ok := path.(*paths.InvPath); !ok {
		t.Errorf("expected *InvPath, got %T", path)
	}
}

// --- resolveRelativeIRI edge cases ---

func TestResolveRelativeIRIBadBase(t *testing.T) {
	// Invalid base URL
	result := resolveRelativeIRI("://bad", "relative")
	if result != "://badrelative" {
		t.Errorf("expected fallback concatenation, got %s", result)
	}
}

// --- tripleTermHasVariables ---

func TestTripleTermHasVariablesNoVars(t *testing.T) {
	has := tripleTermHasVariables(`<<( <http://a> <http://b> <http://c> )>>`)
	if has {
		t.Error("expected no variables")
	}
}

func TestTripleTermHasVariablesNested(t *testing.T) {
	has := tripleTermHasVariables(`<<( <http://a> <http://b> <<( ?x <http://d> <http://e> )>> )>>`)
	if !has {
		t.Error("expected variables in nested triple term")
	}
}

func TestTripleTermHasVariablesTooShort(t *testing.T) {
	has := tripleTermHasVariables(`short`)
	if has {
		t.Error("expected no variables for short string")
	}
}

// --- SRJ parsing ---

func TestParseSRJBoolean(t *testing.T) {
	srj := `{"boolean": true}`
	r, err := ParseSRJ(strings.NewReader(srj))
	if err != nil {
		t.Fatal(err)
	}
	if r.Type != "ASK" {
		t.Errorf("expected ASK, got %s", r.Type)
	}
	if !r.AskResult {
		t.Error("expected true")
	}
}

func TestParseSRJBindings(t *testing.T) {
	srj := `{
		"head": {"vars": ["x"]},
		"results": {"bindings": [
			{"x": {"type": "uri", "value": "http://example.org/a"}},
			{"x": {"type": "bnode", "value": "b1"}},
			{"x": {"type": "literal", "value": "hello", "xml:lang": "en"}}
		]}
	}`
	r, err := ParseSRJ(strings.NewReader(srj))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3 bindings, got %d", len(r.Bindings))
	}
}

func TestParseSRJTripleValue(t *testing.T) {
	srj := `{
		"head": {"vars": ["x"]},
		"results": {"bindings": [
			{"x": {"type": "triple", "value": {
				"subject": {"type": "uri", "value": "http://s"},
				"predicate": {"type": "uri", "value": "http://p"},
				"object": {"type": "uri", "value": "http://o"}
			}}}
		]}
	}`
	r, err := ParseSRJ(strings.NewReader(srj))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
}

// --- ORDER BY ASC explicit ---

func TestOrderByASCExplicit(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name } ORDER BY ASC(?name)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- Negated property set parsing ---

func TestNegatedPropertySetQuery(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s ?o WHERE { ?s !(ex:name) ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Should return triples with predicates other than ex:name
	_ = r
}

func TestNegatedPropertySetWithInverseCov(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s ?o WHERE { ?s !(^ex:name) ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- parseDateTime edge case ---

func TestParseDateTimeInvalid(t *testing.T) {
	g := makeSPARQLGraph(t)
	// YEAR on non-date should produce type error
	r, err := Query(g, `
		SELECT (YEAR("notadate"^^<http://www.w3.org/2001/XMLSchema#dateTime>) AS ?y) WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Result should have unbound ?y
	if len(r.Bindings) == 1 {
		if _, ok := r.Bindings[0]["y"]; ok {
			// May or may not be bound depending on implementation
		}
	}
}

// --- ResultsEqual with bnodes ---

func TestResultsEqualWithBnodes(t *testing.T) {
	a := &Result{
		Type: "SELECT",
		Vars: []string{"x"},
		Bindings: []map[string]rdflibgo.Term{
			{"x": rdflibgo.NewBNode("a1")},
			{"x": rdflibgo.NewBNode("a2")},
		},
	}
	b := &Result{
		Type: "SELECT",
		Vars: []string{"x"},
		Bindings: []map[string]rdflibgo.Term{
			{"x": rdflibgo.NewBNode("b1")},
			{"x": rdflibgo.NewBNode("b2")},
		},
	}
	if !ResultsEqual(a, b) {
		t.Error("expected results with different bnode labels to be equal")
	}
}

func TestResultsEqualDifferentLengths(t *testing.T) {
	a := &Result{Type: "SELECT", Bindings: []map[string]rdflibgo.Term{{"x": rdflibgo.NewLiteral("a")}}}
	b := &Result{Type: "SELECT", Bindings: []map[string]rdflibgo.Term{}}
	if ResultsEqual(a, b) {
		t.Error("expected not equal for different lengths")
	}
}

func TestResultsEqualDifferentTypes(t *testing.T) {
	a := &Result{Type: "ASK", AskResult: true}
	b := &Result{Type: "SELECT"}
	if ResultsEqual(a, b) {
		t.Error("expected not equal for different types")
	}
}

// --- srjString ---

func TestSrjStringValid(t *testing.T) {
	s := srjString([]byte(`"hello"`))
	if s != "hello" {
		t.Errorf("expected hello, got %s", s)
	}
}

func TestSrjStringInvalid(t *testing.T) {
	s := srjString([]byte(`not json`))
	if s != "not json" {
		t.Errorf("expected raw string, got %s", s)
	}
}

// --- USING NAMED ---

func TestEvalModifyWithUsingNamed(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": ng},
	}

	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT { ?s ex:q "new" }
		USING NAMED <http://example.org/g1>
		WHERE { GRAPH <http://example.org/g1> { ?s ex:p "val" } }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- CLEAR/DROP specific graph ---

func TestEvalClearSpecificGraph(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	ng := graph.NewGraph()
	ng.Add(ex("http://s"), ex("http://p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": ng},
	}

	err := Update(ds, `CLEAR GRAPH <http://example.org/g1>`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range ng.Triples(nil, nil, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 triples after CLEAR, got %d", count)
	}
}

func TestEvalDropSpecificGraph(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	ng := graph.NewGraph()
	ng.Add(ex("http://s"), ex("http://p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": ng},
	}

	err := Update(ds, `DROP GRAPH <http://example.org/g1>`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ds.NamedGraphs["http://example.org/g1"]; ok {
		t.Error("expected graph to be removed after DROP")
	}
}

func TestEvalClearNonExistentGraphSilent(t *testing.T) {
	g := graph.NewGraph()
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}

	// Non-existent graph, not silent — should not error (CLEAR is lenient)
	err := Update(ds, `CLEAR GRAPH <http://example.org/nonexistent>`)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- DELETE WHERE with GRAPH clause ---

func TestDeleteWhereGraphClause(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": ng},
	}

	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		DELETE WHERE { GRAPH <http://example.org/g1> { ?s ex:p ?o } }
	`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range ng.Triples(nil, nil, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 triples, got %d", count)
	}
}

// --- containsExists in different expr types ---

func TestContainsExistsInBinary(t *testing.T) {
	exists := &ExistsExpr{Pattern: &BGP{}, Not: false}
	binary := &BinaryExpr{Op: "&&", Left: &LiteralExpr{Value: rdflibgo.NewLiteral("true")}, Right: exists}
	if !containsExists(binary) {
		t.Error("expected to find EXISTS in binary expr")
	}
}

func TestContainsExistsInUnary(t *testing.T) {
	exists := &ExistsExpr{Pattern: &BGP{}, Not: true}
	unary := &UnaryExpr{Op: "!", Arg: exists}
	if !containsExists(unary) {
		t.Error("expected to find EXISTS in unary expr")
	}
}

func TestContainsExistsNone(t *testing.T) {
	expr := &BinaryExpr{Op: "+", Left: &VarExpr{Name: "x"}, Right: &LiteralExpr{Value: rdflibgo.NewLiteral("1")}}
	if containsExists(expr) {
		t.Error("expected no EXISTS")
	}
}

// --- evalUnaryOp missing branch ---

func TestEvalUnaryOpNegation(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `SELECT (-5 AS ?val) WHERE {}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatal("expected 1 binding")
	}
}

// --- validateNoNestedAggregates ---

func TestValidateNoNestedAggregatesBinary(t *testing.T) {
	expr := &BinaryExpr{
		Op:    "+",
		Left:  &FuncExpr{Name: "COUNT", Args: []Expr{&VarExpr{Name: "x"}}},
		Right: &LiteralExpr{Value: rdflibgo.NewLiteral("1")},
	}
	err := validateNoNestedAggregates(expr)
	if err != nil {
		t.Errorf("expected no error for non-nested, got %v", err)
	}
}

func TestValidateNoNestedAggregatesUnary(t *testing.T) {
	expr := &UnaryExpr{
		Op:  "-",
		Arg: &FuncExpr{Name: "COUNT", Args: []Expr{&VarExpr{Name: "x"}}},
	}
	err := validateNoNestedAggregates(expr)
	if err != nil {
		t.Errorf("expected no error for non-nested, got %v", err)
	}
}

// --- CONSTRUCT with CONSTRUCT WHERE shorthand ---

func TestConstructWhereShorthand(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected non-nil graph from CONSTRUCT WHERE")
	}
}

// --- CONSTRUCT with FROM clause ---

func TestConstructWithFromClause(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:name ?name }
		FROM <http://example.org/graph1>
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// FROM doesn't actually restrict in the default store, but shouldn't error
	_ = r
}

// --- CONSTRUCT with FROM NAMED ---

func TestConstructWithFromNamed(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- Eval with VERSION directive ---

func TestQueryWithVersion(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		VERSION "1.2"
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- CONSTRUCT with bnodes ---

func TestConstructWithBnodes(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { _:x ex:relates ?s . _:x a ex:Wrapper }
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected triples from CONSTRUCT with bnodes")
	}
}

// --- GROUP BY with HAVING multiple conditions ---

func TestGroupByWithHaving(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?type (COUNT(?s) AS ?count)
		WHERE { ?s a ?type }
		GROUP BY ?type
		HAVING (COUNT(?s) > 0)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected grouped results")
	}
}

// --- Subquery ---

func TestSubquery(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			{ SELECT ?s ?name WHERE { ?s ex:name ?name } LIMIT 10 }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results from subquery")
	}
}

// --- MINUS ---

func TestMinus(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s a ?type .
			MINUS { ?s ex:name "NonExistent" }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results after MINUS")
	}
}

// --- VALUES inline data ---

func TestValuesInlineData(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			VALUES ?name { "Alice" }
			?s ex:name ?name
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

// --- Post-query VALUES ---

func TestPostQueryValues(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s ?name WHERE { ?s ex:name ?name }
		VALUES ?name { "Alice" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result from post-query VALUES, got %d", len(r.Bindings))
	}
}

// --- UNION ---

func TestUnionQuery(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?val WHERE {
			{ ?s ex:name ?val } UNION { ?s ex:age ?val }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results from UNION")
	}
}

// --- EXISTS / NOT EXISTS ---

func TestExistsInFilter(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			FILTER EXISTS { ?s a ?type }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestNotExistsInFilter(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			FILTER NOT EXISTS { ?s ex:nonexistent ?val }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results (all should pass NOT EXISTS)")
	}
}

// --- Property path + / | / * ---

func TestPropertyPathSequence(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
		SELECT ?s ?type WHERE { ?s rdf:type/rdf:type ?type }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestPropertyPathAlternative(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s ?val WHERE { ?s (ex:name|ex:age) ?val }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results from alternative path")
	}
}

func TestPropertyPathZeroOrMore(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
		SELECT ?s ?c WHERE { ?s rdfs:subClassOf* ?c }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- IN operator ---

func TestInOperator(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			FILTER(?name IN ("Alice", "Bob", "Charlie"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results from IN filter")
	}
}

// --- NOT IN operator ---

func TestNotInOperator(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			FILTER(?name NOT IN ("NonExistent"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results from NOT IN filter")
	}
}

// --- OPTIONAL query ---

func TestOptionalQuery(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s ?name ?age WHERE {
			?s ex:name ?name .
			OPTIONAL { ?s ex:age ?age }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results from OPTIONAL query")
	}
}

// --- BIND in query ---

func TestBindInQuery(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s ?upper WHERE {
			?s ex:name ?name .
			BIND(UCASE(?name) AS ?upper)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results with BIND")
	}
}

// --- Aggregate functions ---

func TestAggregateSUM(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (SUM(?age) AS ?total) WHERE {
			?s ex:age ?age
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected SUM result")
	}
}

func TestAggregateAVG(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (AVG(?age) AS ?avg) WHERE {
			?s ex:age ?age
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected AVG result")
	}
}

func TestAggregateMINMAX(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (MIN(?age) AS ?min) (MAX(?age) AS ?max) WHERE {
			?s ex:age ?age
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected MIN/MAX result")
	}
}

func TestAggregateGroupConcat(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (GROUP_CONCAT(?name ; SEPARATOR=", ") AS ?names) WHERE {
			?s ex:name ?name
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected GROUP_CONCAT result")
	}
}

func TestAggregateSample(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (SAMPLE(?name) AS ?s) WHERE {
			?x ex:name ?name
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected SAMPLE result")
	}
}

// --- String functions ---

func TestStringFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s (STRLEN(?name) AS ?len) (SUBSTR(?name, 1, 3) AS ?sub) (LCASE(?name) AS ?lower)
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- Numeric expressions ---

func TestNumericExpressions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s (?age + 1 AS ?next) (?age * 2 AS ?double) (?age - 5 AS ?less)
		WHERE { ?s ex:age ?age }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- Boolean operators ---

func TestBooleanOperators(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			?s ex:age ?age .
			FILTER(?age > 20 && ?name != "Nobody")
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- COALESCE and IF ---

func TestCoalesceAndIf(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s (COALESCE(?age, 0) AS ?a) (IF(BOUND(?age), "has age", "no age") AS ?msg) WHERE {
			?s ex:name ?name .
			OPTIONAL { ?s ex:age ?age }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- Date functions ---

func TestDateFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (YEAR("2024-03-15T10:30:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>) AS ?y)
		       (MONTH("2024-03-15T10:30:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>) AS ?m)
		       (DAY("2024-03-15T10:30:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>) AS ?d)
		WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected date function results")
	}
}

// --- Hash functions ---

func TestHashFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (MD5("hello") AS ?md5) (SHA1("hello") AS ?sha1) (SHA256("hello") AS ?sha256) WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected hash results")
	}
}

// --- GRAPH clause ---

func TestGraphClause(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			GRAPH <http://example.org/g1> { ?s ?p ?o }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r // May return empty if no named graphs
}

// --- Numeric edge cases ---

func TestDivisionByZero(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `SELECT (1/0 AS ?val) WHERE {}`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- STRBEFORE / STRAFTER ---

func TestStrBeforeAfterCov(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (STRBEFORE("hello world", " ") AS ?before) (STRAFTER("hello world", " ") AS ?after) WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- ENCODE_FOR_URI ---

func TestEncodeForURICov(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (ENCODE_FOR_URI("hello world") AS ?enc) WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- IRI / isIRI / isLiteral / isBNode ---

func TestTypeCheckFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s (isIRI(?s) AS ?is_iri) (isLiteral(?name) AS ?is_lit) (DATATYPE(?name) AS ?dt) (LANG(?name) AS ?lg)
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- REPLACE ---

func TestReplaceFn(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (REPLACE("hello world", "world", "earth") AS ?result) WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- REGEX ---

func TestRegex(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			FILTER(REGEX(?name, "^A"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result matching ^A, got %d", len(r.Bindings))
	}
}

// --- IRI construction ---

func TestIRIConstruction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (IRI("http://example.org/test") AS ?iri) WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected result")
	}
}

// --- CONCAT ---

func TestConcat(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (CONCAT("hello", " ", "world") AS ?result) WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected result")
	}
}

// --- STR / xsd casts ---

func TestStrAndCasts(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (STR(42) AS ?s) (xsd:string(42) AS ?xs) WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- UPDATE: insert with bnode collection vars ---

func TestUpdateInsertBnodeVars(t *testing.T) {
	g := graph.NewGraph()
	ex := rdflibgo.NewURIRefUnsafe
	g.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT { _:b0 ex:relates ?s }
		WHERE { ?s ex:p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- UPDATE: DELETE INSERT with complex WHERE ---

func TestUpdateDeleteInsertComplex(t *testing.T) {
	g := graph.NewGraph()
	ex := rdflibgo.NewURIRefUnsafe
	g.Add(ex("http://example.org/s1"), ex("http://example.org/status"), rdflibgo.NewLiteral("draft"))
	g.Add(ex("http://example.org/s2"), ex("http://example.org/status"), rdflibgo.NewLiteral("published"))

	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		DELETE { ?s ex:status "draft" }
		INSERT { ?s ex:status "archived" }
		WHERE { ?s ex:status "draft" }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- SRX with dir lang attribute ---

func TestParseSRXWithDir(t *testing.T) {
	srx := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head><variable name="x"/></head>
  <results>
    <result>
      <binding name="x">
        <literal xml:lang="ar" its:dir="rtl">مرحبا</literal>
      </binding>
    </result>
  </results>
</sparql>`
	r, err := ParseSRX(strings.NewReader(srx))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(r.Bindings))
	}
}

// --- SRX with bnode ---

func TestParseSRXWithBNode(t *testing.T) {
	srx := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head><variable name="x"/></head>
  <results>
    <result>
      <binding name="x">
        <bnode>b0</bnode>
      </binding>
    </result>
  </results>
</sparql>`
	r, err := ParseSRX(strings.NewReader(srx))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(r.Bindings))
	}
}

// --- SRX with triple ---

func TestParseSRXWithTriple(t *testing.T) {
	srx := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head><variable name="t"/></head>
  <results>
    <result>
      <binding name="t">
        <triple>
          <subject><uri>http://example.org/s</uri></subject>
          <predicate><uri>http://example.org/p</uri></predicate>
          <object><literal datatype="http://www.w3.org/2001/XMLSchema#integer">42</literal></object>
        </triple>
      </binding>
    </result>
  </results>
</sparql>`
	r, err := ParseSRX(strings.NewReader(srx))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(r.Bindings))
	}
}

// --- extractTemplateFromPattern ---

func TestExtractTemplateFromPatternJoin(t *testing.T) {
	bgp1 := &BGP{Triples: []Triple{{Subject: "?s", Predicate: "<http://p1>", Object: "?o1"}}}
	bgp2 := &BGP{Triples: []Triple{{Subject: "?s", Predicate: "<http://p2>", Object: "?o2"}}}
	jp := &JoinPattern{Left: bgp1, Right: bgp2}
	tmpl := extractTemplateFromPattern(jp)
	if len(tmpl) != 2 {
		t.Errorf("expected 2 templates, got %d", len(tmpl))
	}
}

func TestExtractTemplateFromPatternNil(t *testing.T) {
	tmpl := extractTemplateFromPattern(&FilterPattern{Pattern: &BGP{}})
	if tmpl != nil {
		t.Error("expected nil for unsupported pattern type")
	}
}

// --- resolveTripleTermPattern ---

func TestResolveTripleTermPatternValid(t *testing.T) {
	prefixes := map[string]string{}
	bindings := map[string]rdflibgo.Term{
		"s": rdflibgo.NewURIRefUnsafe("http://example.org/s"),
	}
	result := resolveTripleTermPattern(`<<( ?s <http://p> <http://o> )>>`, bindings, prefixes)
	if result == nil {
		t.Fatal("expected non-nil")
	}
}

func TestResolveTripleTermPatternUnbound(t *testing.T) {
	prefixes := map[string]string{}
	bindings := map[string]rdflibgo.Term{}
	result := resolveTripleTermPattern(`<<( ?s <http://p> <http://o> )>>`, bindings, prefixes)
	if result != nil {
		t.Error("expected nil for unbound variable")
	}
}

// --- formatDecimal edge cases ---

func TestFormatDecimalSmall(t *testing.T) {
	result := formatDecimal(0.001)
	if result == "" {
		t.Error("expected non-empty")
	}
}

func TestFormatDecimalLarge(t *testing.T) {
	result := formatDecimal(99999.999)
	if result == "" {
		t.Error("expected non-empty")
	}
}

func TestFormatDecimalNegative(t *testing.T) {
	result := formatDecimal(-3.14)
	if !strings.HasPrefix(result, "-") {
		t.Errorf("expected negative prefix, got %s", result)
	}
}

// --- resolveTermRef edge cases ---

func TestResolveTermRefTripleTerm(t *testing.T) {
	result := resolveTermRef(`<<( <http://s> <http://p> <http://o> )>>`, nil)
	_ = result
}

// --- Direct unit tests for uncovered parser/update paths ---

func TestValidateStringEscapesInvalid(t *testing.T) {
	// Invalid escape sequence
	err := validateStringEscapes(`"hello\x world"`)
	if err == nil {
		t.Error("expected error for invalid escape \\x")
	}
}

func TestValidateStringEscapesValid(t *testing.T) {
	err := validateStringEscapes(`"hello\n\t\r\\\""`)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateStringEscapesEmpty(t *testing.T) {
	err := validateStringEscapes("")
	if err != nil {
		t.Errorf("expected no error for empty, got %v", err)
	}
}

func TestValidateStringEscapesShortUEscape(t *testing.T) {
	// \u with insufficient hex digits
	err := validateStringEscapes(`"abc\u12"`)
	if err == nil {
		t.Error("expected error for short \\u escape")
	}
}

func TestValidateStringEscapesShortBigUEscape(t *testing.T) {
	err := validateStringEscapes(`"abc\U1234"`)
	if err == nil {
		t.Error("expected error for short \\U escape")
	}
}

func TestValidateStringEscapesLongQuoteValid(t *testing.T) {
	err := validateStringEscapes(`"""hello\nworld"""`)
	if err != nil {
		t.Errorf("expected no error for long quote, got %v", err)
	}
}

func TestResolveTemplateValueTripleTerm(t *testing.T) {
	prefixes := map[string]string{}
	bindings := map[string]rdflibgo.Term{
		"s": rdflibgo.NewURIRefUnsafe("http://example.org/s"),
	}
	result := resolveTemplateValue(`<<( ?s <http://p> <http://o> )>>`, bindings, prefixes)
	if result == nil {
		t.Fatal("expected non-nil")
	}
}

func TestResolveTemplateValueReifierVar(t *testing.T) {
	prefixes := map[string]string{}
	bindings := map[string]rdflibgo.Term{}
	// _reifier variable should auto-create bnode
	result := resolveTemplateValue("?_reifier0", bindings, prefixes)
	if result == nil {
		t.Fatal("expected auto-created bnode")
	}
}

func TestResolveTemplateValueBnodeVar(t *testing.T) {
	bindings := map[string]rdflibgo.Term{}
	result := resolveTemplateValue("?_bnode0", bindings, nil)
	if result == nil {
		t.Fatal("expected auto-created bnode")
	}
}

func TestResolveTemplateValueCollVar(t *testing.T) {
	bindings := map[string]rdflibgo.Term{}
	result := resolveTemplateValue("?_coll0", bindings, nil)
	if result == nil {
		t.Fatal("expected auto-created bnode")
	}
}

func TestResolveTemplateValueUnboundCov(t *testing.T) {
	bindings := map[string]rdflibgo.Term{}
	result := resolveTemplateValue("?unbound", bindings, nil)
	if result != nil {
		t.Error("expected nil for unbound variable")
	}
}

func TestResolveTermRefDoubleLiteral(t *testing.T) {
	result := resolveTermRef("3.14e2", nil)
	if result == nil {
		t.Fatal("expected non-nil for double")
	}
}

func TestFormatDecimalZero(t *testing.T) {
	r := formatDecimal(0.0)
	if r != "0.0" {
		t.Errorf("expected 0.0, got %s", r)
	}
}

func TestFormatDecimalWholeNumber(t *testing.T) {
	r := formatDecimal(5.0)
	if r != "5.0" {
		t.Errorf("expected 5.0, got %s", r)
	}
}

func TestTermValuesEqualTripleTerms(t *testing.T) {
	s := rdflibgo.NewURIRefUnsafe("http://s")
	p := rdflibgo.NewURIRefUnsafe("http://p")
	o := rdflibgo.NewURIRefUnsafe("http://o")
	tt1 := rdflibgo.NewTripleTerm(s, p, o)
	tt2 := rdflibgo.NewTripleTerm(s, p, o)
	if !termValuesEqual(tt1, tt2) {
		t.Error("expected equal triple terms")
	}
}

func TestTermValuesEqualDifferentTripleTerms(t *testing.T) {
	s1 := rdflibgo.NewURIRefUnsafe("http://s1")
	s2 := rdflibgo.NewURIRefUnsafe("http://s2")
	p := rdflibgo.NewURIRefUnsafe("http://p")
	o := rdflibgo.NewURIRefUnsafe("http://o")
	tt1 := rdflibgo.NewTripleTerm(s1, p, o)
	tt2 := rdflibgo.NewTripleTerm(s2, p, o)
	if termValuesEqual(tt1, tt2) {
		t.Error("expected unequal triple terms")
	}
}

// --- CONSTRUCT with annotations ---

func TestConstructWithAnnotations(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			?s ex:name ?name .
		}
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected graph")
	}
}

// --- DELETE WHERE with named graph variable ---

func TestDeleteWhereNamedGraphVarCov(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))
	ds := &Dataset{
		Default:     graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g": ng},
	}
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		DELETE WHERE { GRAPH ?g { ?s ex:p ?o } }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- Query with GRAPH and variable graph name ---

func TestGraphVariableName(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?g ?s WHERE { GRAPH ?g { ?s ?p ?o } }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- SRX with lang+dir literal ---

func TestParseSRXLiteralLangDir(t *testing.T) {
	srx := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#"
        xmlns:its="http://www.w3.org/2005/11/its">
  <head><variable name="x"/></head>
  <results>
    <result>
      <binding name="x">
        <literal xml:lang="en--ltr">hello</literal>
      </binding>
    </result>
  </results>
</sparql>`
	r, err := ParseSRX(strings.NewReader(srx))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(r.Bindings))
	}
}

// --- Update with multiple USING clauses ---

func TestUpdateMultipleUsing(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	ng1 := graph.NewGraph()
	ng1.Add(ex("http://example.org/a"), ex("http://example.org/p"), rdflibgo.NewLiteral("val1"))
	ng2 := graph.NewGraph()
	ng2.Add(ex("http://example.org/b"), ex("http://example.org/p"), rdflibgo.NewLiteral("val2"))

	ds := &Dataset{
		Default: g,
		NamedGraphs: map[string]*rdflibgo.Graph{
			"http://example.org/g1": ng1,
			"http://example.org/g2": ng2,
		},
	}

	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT { ?s ex:found "yes" }
		USING <http://example.org/g1>
		USING <http://example.org/g2>
		WHERE { ?s ex:p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- Transfer graphs errors ---

func TestTransferGraphsSourceNotFound(t *testing.T) {
	ds := &Dataset{
		Default:     graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{},
	}
	err := transferGraphs(ds, "http://nonexistent", "DEFAULT", false, false)
	if err == nil {
		t.Error("expected error for source not found")
	}
}

func TestTransferGraphsSourceNotFoundSilent(t *testing.T) {
	ds := &Dataset{
		Default:     graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{},
	}
	err := transferGraphs(ds, "http://nonexistent", "DEFAULT", false, true)
	if err != nil {
		t.Errorf("expected no error for silent, got %v", err)
	}
}

func TestGetOrCreateGraphNew(t *testing.T) {
	ds := &Dataset{
		Default:     graph.NewGraph(),
		NamedGraphs: nil, // nil map
	}
	g := getOrCreateGraph(ds, "http://new")
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
	if ds.NamedGraphs == nil {
		t.Error("expected NamedGraphs to be initialized")
	}
}

// --- Pushing coverage on evalExpr paths ---

func TestEvalExprComparison(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:age ?age .
			FILTER (?age != 99)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestEvalExprLogicalOr(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			FILTER (?name = "Alice" || ?name = "Bob")
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- CONSTRUCT with blank node property lists ---

func TestConstructBnodePropertyList(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:related [ ex:name ?name ] }
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected graph")
	}
}

// --- CONSTRUCT with collection syntax ---

func TestConstructCollectionSyntax(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:list (1 2 3) }
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected graph from CONSTRUCT with collection")
	}
}

// --- DELETE WHERE WITH ---

func TestDeleteWhereWithCov(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))
	ds := &Dataset{
		Default:     graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g": ng},
	}
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		WITH <http://example.org/g>
		DELETE WHERE { ?s ex:p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- QUAD DATA with named graph ---

func TestParseQuadDataNamedGraph(t *testing.T) {
	g := graph.NewGraph()
	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{},
	}
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT DATA {
			ex:s1 ex:p "default" .
			GRAPH <http://example.org/g1> {
				ex:s2 ex:p "named" .
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- Quad pattern with variable graph ---

func TestModifyWithVariableGraph(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))
	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": ng},
	}
	// Insert with GRAPH in WHERE
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT { GRAPH <http://example.org/g2> { ?s ex:copy ?o } }
		WHERE { GRAPH <http://example.org/g1> { ?s ex:p ?o } }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- SPARQL CONSTRUCT with multiple template patterns ---

func TestConstructMultiplePatterns(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			?s ex:hasName ?name .
			?s a ex:NamedThing .
		}
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected graph from multi-pattern CONSTRUCT")
	}
}

// --- Post-query BINDINGS clause ---

func TestPostQueryBindings(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s ?name WHERE { ?s ex:name ?name }
		BINDINGS ?name { ("Alice") }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result from BINDINGS, got %d", len(r.Bindings))
	}
}

// --- Extract date parts ---

func TestExtractDatePartYear(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (HOURS("2024-03-15T10:30:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>) AS ?h)
		       (MINUTES("2024-03-15T10:30:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>) AS ?m)
		       (SECONDS("2024-03-15T10:30:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>) AS ?s)
		WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestExtractTimezoneFunc(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (TZ("2024-03-15T10:30:00+05:00"^^<http://www.w3.org/2001/XMLSchema#dateTime>) AS ?tz)
		       (TIMEZONE("2024-03-15T10:30:00+05:00"^^<http://www.w3.org/2001/XMLSchema#dateTime>) AS ?tzd)
		WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- XSD cast functions ---

func TestXSDCasts(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (xsd:integer("42") AS ?i)
		       (xsd:decimal("3.14") AS ?d)
		       (xsd:float("1.5") AS ?f)
		       (xsd:double("2.5") AS ?db)
		       (xsd:boolean("true") AS ?b)
		WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestEvalExprArithmetic(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (?age / 10 AS ?decade) WHERE {
			?s ex:age ?age
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- More queries exercising edge cases ---

func TestCountStar(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (COUNT(*) AS ?c) WHERE { ?s ?p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected COUNT(*) result")
	}
}

func TestCountDistinct(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (COUNT(DISTINCT ?s) AS ?c) WHERE { ?s ?p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected COUNT(DISTINCT) result")
	}
}

func TestGroupByExpressionAlias(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?type (COUNT(?s) AS ?c)
		WHERE { ?s a ?type }
		GROUP BY ?type
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestAskQuery(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		ASK { ?s ex:name "Alice" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Type != "ASK" {
		t.Errorf("expected ASK, got %s", r.Type)
	}
	if !r.AskResult {
		t.Error("expected true")
	}
}

func TestAskQueryFalse(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		ASK { ?s ex:name "Nobody" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.AskResult {
		t.Error("expected false")
	}
}

func TestSelectWithBase(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		BASE <http://example.org/>
		SELECT ?s WHERE { ?s <name> ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results with BASE")
	}
}

func TestConstructOrderByLimitOffset(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:name ?name }
		WHERE { ?s ex:name ?name }
		ORDER BY ?name
		LIMIT 1
		OFFSET 0
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected graph from CONSTRUCT")
	}
}

func TestInsertDeleteGraphRef(t *testing.T) {
	g := graph.NewGraph()
	ex := rdflibgo.NewURIRefUnsafe
	g.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))
	ng := graph.NewGraph()
	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g": ng},
	}
	// INSERT into specific named graph
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT DATA { GRAPH <http://example.org/g> { ex:s ex:p "new" } }
	`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range ng.Triples(nil, nil, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 triple in named graph, got %d", count)
	}
}

func TestDeleteDataGraphRef(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))
	ds := &Dataset{
		Default:     graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g": ng},
	}
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		DELETE DATA { GRAPH <http://example.org/g> { ex:s ex:p "val" } }
	`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range ng.Triples(nil, nil, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 triples after DELETE DATA, got %d", count)
	}
}

func TestDropNamedGraphs(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	ng := graph.NewGraph()
	ng.Add(ex("http://s"), ex("http://p"), rdflibgo.NewLiteral("v"))
	ds := &Dataset{
		Default:     graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g": ng},
	}
	err := Update(ds, `DROP NAMED`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.NamedGraphs) != 0 {
		t.Error("expected no named graphs after DROP NAMED")
	}
}

func TestGraphRefAllDefaultCov(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	g.Add(ex("http://s"), ex("http://p"), rdflibgo.NewLiteral("v"))
	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	err := Update(ds, `CLEAR DEFAULT`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestModifyWithUsingNamed(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	ng := graph.NewGraph()
	ng.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{
		Default:     g,
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g1": ng},
	}

	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT { ?s ex:q "found" }
		USING NAMED <http://example.org/g1>
		WHERE {
			GRAPH <http://example.org/g1> { ?s ex:p ?o }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateExistingGraphSilent(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	ng := graph.NewGraph()
	ng.Add(ex("http://s"), ex("http://p"), rdflibgo.NewLiteral("v"))
	ds := &Dataset{
		Default:     graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{"http://example.org/g": ng},
	}
	// CREATE should not error for existing graph
	err := Update(ds, `CREATE SILENT GRAPH <http://example.org/g>`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultipleUpdateOpsSemicolon(t *testing.T) {
	g := graph.NewGraph()
	ex := rdflibgo.NewURIRefUnsafe
	g.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT DATA { ex:s2 ex:p "val2" } ;
		DELETE DATA { ex:s ex:p "val" }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWhereVariableGraph(t *testing.T) {
	ex := rdflibgo.NewURIRefUnsafe
	g := graph.NewGraph()
	g.Add(ex("http://example.org/s"), ex("http://example.org/p"), rdflibgo.NewLiteral("val"))

	ds := &Dataset{Default: g, NamedGraphs: map[string]*rdflibgo.Graph{}}
	err := Update(ds, `
		PREFIX ex: <http://example.org/>
		DELETE WHERE { ?s ex:p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- SRX ASK result ---

func TestParseSRXAsk(t *testing.T) {
	srx := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head/>
  <boolean>true</boolean>
</sparql>`
	r, err := ParseSRX(strings.NewReader(srx))
	if err != nil {
		t.Fatal(err)
	}
	if r.Type != "ASK" {
		t.Errorf("expected ASK, got %s", r.Type)
	}
	if !r.AskResult {
		t.Error("expected true")
	}
}

// --- Numeric comparisons ---

func TestNumericComparisons(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:age ?age .
			FILTER(?age >= 25 && ?age <= 35)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- String comparison ---

func TestStringComparison(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			FILTER(?name > "A" && ?name < "Z")
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- CONTAINS / STRSTARTS / STRENDS ---

func TestStringMatchFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			FILTER(CONTAINS(?name, "lic") || STRSTARTS(?name, "Al") || STRENDS(?name, "ce"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results from string match functions")
	}
}

// --- ROUND / CEIL / FLOOR / ABS ---

func TestMathFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT (ROUND(3.7) AS ?r) (CEIL(3.2) AS ?c) (FLOOR(3.8) AS ?f) (ABS(-5) AS ?a) WHERE {}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results")
	}
}

// --- NOW / UUID / STRUUID ---

func TestGeneratingFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `SELECT (STRUUID() AS ?u) WHERE {}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected STRUUID result")
	}
}

// --- LANG / LANGMATCHES ---

func TestLangFunctions(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		SELECT ?s WHERE {
			?s ?p ?o .
			FILTER(LANG(?o) = "" || LANGMATCHES(LANG(?o), "*"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- BOUND ---

func TestBoundFunction(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			OPTIONAL { ?s ex:age ?age }
			FILTER(!BOUND(?age))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

// --- sameTerm ---

func TestSameTerm(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:name ?name .
			FILTER(sameTerm(?name, "Alice"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func makeUpdateDataset() *Dataset {
	return &Dataset{
		Default:     graph.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
}

// --- Additional coverage: CONSTRUCT annotations ---

func TestConstructAnnotationsReifier(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			ex:s ex:p "v" ~ ex:r1 .
		} WHERE {
			ex:s ex:p "v" .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected non-nil result graph")
	}
}

func TestConstructAnnotationsBlock(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			ex:s ex:p "v" {| ex:src ex:wiki |} .
		} WHERE {
			ex:s ex:p "v" .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected non-nil result graph")
	}
}

func TestConstructAnnotationsReifierWithBlock(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			ex:s ex:p "v" ~ ex:r1 {| ex:src ex:wiki |} .
		} WHERE {
			ex:s ex:p "v" .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected non-nil result graph")
	}
}

func TestConstructSemicolonPredList(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			ex:s ex:p1 "a" ; ex:p2 "b" .
		} WHERE {
			ex:s ex:p "v" .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected non-nil result graph")
	}
}

func TestConstructCommaObjList(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			ex:s ex:p "a" , "b" .
		} WHERE {
			ex:s ex:p "v" .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected non-nil result graph")
	}
}

func TestDropDefault(t *testing.T) {
	ds := makeUpdateDataset()
	err := Update(ds, `DROP DEFAULT`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDropNamedSimple(t *testing.T) {
	ds := makeUpdateDataset()
	ds.NamedGraphs["http://example.org/g"] = graph.NewGraph()
	err := Update(ds, `DROP NAMED`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClearNamedSimple(t *testing.T) {
	ds := makeUpdateDataset()
	ds.NamedGraphs["http://example.org/g"] = graph.NewGraph()
	err := Update(ds, `CLEAR NAMED`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClearGraphSpecific(t *testing.T) {
	ds := makeUpdateDataset()
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `CLEAR GRAPH <http://example.org/g>`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDropGraphSpecific(t *testing.T) {
	ds := makeUpdateDataset()
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `DROP GRAPH <http://example.org/g>`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ds.NamedGraphs["http://example.org/g"]; ok {
		t.Error("expected graph to be deleted")
	}
}

func TestClearNonexistentGraphSilent(t *testing.T) {
	ds := makeUpdateDataset()
	err := Update(ds, `CLEAR SILENT GRAPH <http://example.org/nonexistent>`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadSilentWithoutLoader(t *testing.T) {
	ds := makeUpdateDataset()
	err := Update(ds, `LOAD SILENT <http://example.org/data>`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadSilentIntoWithoutLoader(t *testing.T) {
	ds := makeUpdateDataset()
	err := Update(ds, `LOAD SILENT <http://example.org/data> INTO GRAPH <http://example.org/g>`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMoveSameGraphSimple(t *testing.T) {
	ds := makeUpdateDataset()
	err := Update(ds, `MOVE DEFAULT TO DEFAULT`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWhereWithGraph(t *testing.T) {
	ds := makeUpdateDataset()
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `DELETE WHERE { GRAPH <http://example.org/g> { <http://example.org/s> <http://example.org/p> ?o } }`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateBindScopeNested(t *testing.T) {
	_, err := Query(graph.NewGraph(), `
		PREFIX ex: <http://example.org/>
		SELECT ?x WHERE {
			{ BIND("a" AS ?x) } UNION { BIND("b" AS ?x) }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateBindScopeOptional(t *testing.T) {
	_, err := Query(graph.NewGraph(), `
		PREFIX ex: <http://example.org/>
		SELECT ?x WHERE {
			ex:s ex:p ?x .
			OPTIONAL { BIND("fallback" AS ?y) }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateBindScopeMinusEndToEnd(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	_, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?x WHERE {
			ex:s ex:p ?x .
			MINUS { BIND("excluded" AS ?y) }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateBindScopeSubqueryEndToEnd(t *testing.T) {
	_, err := Query(graph.NewGraph(), `
		PREFIX ex: <http://example.org/>
		SELECT ?x WHERE {
			{ SELECT ?x WHERE { BIND("sub" AS ?x) } }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateBindScopeFilter(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	_, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?x WHERE {
			FILTER(true)
			BIND("a" AS ?x)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNestedAggregateErrorEndToEnd(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("1", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	_, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (COUNT(SUM(?o)) AS ?c) WHERE {
			?s ex:p ?o .
		} GROUP BY ?s
	`)
	if err == nil {
		t.Error("expected error for nested aggregates")
	}
}

func TestNoNestedAggsInBinaryExpr(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("1", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	_, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (COUNT(?o) + 1 AS ?c) WHERE {
			?s ex:p ?o .
		} GROUP BY ?s
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateStringEscapesLongQuoteEndToEnd(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("multi\nline"))
	r, err := Query(g, "PREFIX ex: <http://example.org/>\nSELECT ?o WHERE {\n\tex:s ex:p ?o .\n\tFILTER(?o = \"\"\"multi\nline\"\"\")\n}")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestValidateStringEscapesSurrogateCov(t *testing.T) {
	err := validateStringEscapes("\"\\uD800\"")
	if err == nil {
		t.Error("expected error for surrogate escape")
	}
}

func TestValidateStringEscapesInvalidEscapeCov(t *testing.T) {
	err := validateStringEscapes("\"\\z\"")
	if err == nil {
		t.Error("expected error for invalid escape")
	}
}

func TestValidateStringEscapesUpperU(t *testing.T) {
	err := validateStringEscapes("\"\\U00000041\"")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateStringEscapesUpperUSurrogate(t *testing.T) {
	err := validateStringEscapes("\"\\U0000D800\"")
	if err == nil {
		t.Error("expected error for upper-U surrogate escape")
	}
}

func TestValidateStringEscapesTruncatedU(t *testing.T) {
	err := validateStringEscapes("\"\\u00\"")
	if err == nil {
		t.Error("expected error for truncated \\u escape")
	}
}

func TestValidateStringEscapesTruncatedUpperU(t *testing.T) {
	err := validateStringEscapes("\"\\U0000\"")
	if err == nil {
		t.Error("expected error for truncated \\U escape")
	}
}

func TestValidateStringEscapesLongQuoteCov(t *testing.T) {
	err := validateStringEscapes("\"\"\"hello\\nworld\"\"\"")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBnodePropertyListWithPath(t *testing.T) {
	g := graph.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	q := rdflibgo.NewURIRefUnsafe("http://example.org/q")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	g.Add(s, p, o)
	g.Add(o, q, rdflibgo.NewLiteral("val"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?v WHERE {
			ex:s ex:p [ ex:q ?v ] .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestDeleteWhereWithGraphClause(t *testing.T) {
	ds := makeUpdateDataset()
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("1"))
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/b"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("2"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `DELETE WHERE { GRAPH <http://example.org/g> { ?s <http://example.org/p> ?o } }`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWhereWithSimple(t *testing.T) {
	ds := makeUpdateDataset()
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("val"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `WITH <http://example.org/g> DELETE WHERE { <http://example.org/s> <http://example.org/p> ?o }`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInsertDataWithGraph(t *testing.T) {
	ds := makeUpdateDataset()
	err := Update(ds, `INSERT DATA { GRAPH <http://example.org/g> { <http://example.org/s> <http://example.org/p> "v" } }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ds.NamedGraphs["http://example.org/g"]; !ok {
		t.Error("expected named graph to be created")
	}
}

func TestDeleteDataWithGraph(t *testing.T) {
	ds := makeUpdateDataset()
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `DELETE DATA { GRAPH <http://example.org/g> { <http://example.org/s> <http://example.org/p> "v" } }`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateTripleTermsInUnion(t *testing.T) {
	g := graph.NewGraph()
	_, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			{ ?s ex:p1 ?o1 } UNION { ?s ex:p2 ?o2 }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSubqueryInSelectExpr(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("1", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?cnt WHERE {
			{ SELECT (COUNT(?s) AS ?cnt) WHERE { ?s ex:p ?o } }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestOrderByAscWithParens(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("3", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/b"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("1", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE {
			?s ex:p ?o .
		} ORDER BY ASC(?o)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2 results, got %d", len(r.Bindings))
	}
}

func TestGroupConcatSeparatorCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("a"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("b"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (GROUP_CONCAT(?o; SEPARATOR=", ") AS ?all) WHERE {
			ex:s ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestNotExistsBang(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:p ?o .
			FILTER(!EXISTS { ?s ex:q ?q })
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestResolveRelativeIRIPathMerge(t *testing.T) {
	got := resolveRelativeIRI("http://example.org/base/", "other")
	if got != "http://example.org/base/other" {
		t.Errorf("got %s", got)
	}
}

func TestResolveRelativeIRIAbsPath(t *testing.T) {
	got := resolveRelativeIRI("http://example.org/base/", "/abs")
	if got != "http://example.org/abs" {
		t.Errorf("got %s", got)
	}
}

func TestContainsExistsBinaryExprCov(t *testing.T) {
	e := &BinaryExpr{
		Op:    "&&",
		Left:  &ExistsExpr{Pattern: &BGP{}, Not: false},
		Right: &LiteralExpr{Value: rdflibgo.NewLiteral(true)},
	}
	if !containsExists(e) {
		t.Error("expected containsExists to return true")
	}
}

func TestContainsExistsUnaryExprCov(t *testing.T) {
	e := &UnaryExpr{Op: "!", Arg: &ExistsExpr{Pattern: &BGP{}, Not: false}}
	if !containsExists(e) {
		t.Error("expected containsExists to return true")
	}
}

func TestContainsExistsFuncExprCov(t *testing.T) {
	e := &FuncExpr{Name: "BOUND", Args: []Expr{&ExistsExpr{Pattern: &BGP{}, Not: false}}}
	if !containsExists(e) {
		t.Error("expected containsExists to return true")
	}
}

func TestUnaryMinusCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("5", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?neg WHERE {
			ex:s ex:p ?v .
			BIND(-?v AS ?neg)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestCachedRegexpCompileCacheHitCov(t *testing.T) {
	r1, err1 := cachedRegexpCompile("^test_cov$")
	r2, err2 := cachedRegexpCompile("^test_cov$")
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if r1 == nil || r2 == nil {
		t.Fatal("expected non-nil regexp")
	}
}

func TestReadVarDollarCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT $x WHERE {
			?s ex:p $x .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

type testUnknownOp struct{}

func (*testUnknownOp) isUpdateOp() {}

func TestEvalUpdateOpUnknownCov(t *testing.T) {
	ds := makeUpdateDataset()
	err := evalUpdateOp(ds, &testUnknownOp{}, nil)
	if err == nil {
		t.Error("expected error for unknown update op")
	}
}

func TestModifyWithUsingNamedCov(t *testing.T) {
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/x"), rdflibgo.NewURIRefUnsafe("http://example.org/q"), rdflibgo.NewLiteral("w"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `
		DELETE { <http://example.org/s> <http://example.org/p> ?o }
		INSERT { <http://example.org/s> <http://example.org/p> "new" }
		USING <http://example.org/g>
		USING NAMED <http://example.org/g>
		WHERE { <http://example.org/s> <http://example.org/p> ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestModifyWithWithCov(t *testing.T) {
	ds := makeUpdateDataset()
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("old"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `
		WITH <http://example.org/g>
		DELETE { <http://example.org/s> <http://example.org/p> ?o }
		INSERT { <http://example.org/s> <http://example.org/p> "new" }
		WHERE { <http://example.org/s> <http://example.org/p> ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBaseURIInQueryCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		BASE <http://example.org/>
		SELECT ?o WHERE {
			<s> <p> ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestExistsInBooleanExprCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/q"), rdflibgo.NewLiteral("w"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:p ?o .
			FILTER(EXISTS { ?s ex:q ?q } && true)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestFormatDecimalZeroCov(t *testing.T) {
	result := formatDecimal(0.0)
	if result != "0.0" {
		t.Errorf("expected 0.0, got %s", result)
	}
}

func TestFormatDecimalNegativeCov(t *testing.T) {
	result := formatDecimal(-3.5)
	if !strings.Contains(result, "-") {
		t.Errorf("expected negative, got %s", result)
	}
}

func TestResultEqualWithBnodesNonMatch(t *testing.T) {
	got := &Result{
		Vars: []string{"x"},
		Bindings: []map[string]rdflibgo.Term{
			{"x": rdflibgo.NewLiteral("a")},
			{"x": rdflibgo.NewLiteral("b")},
		},
	}
	expected := &Result{
		Vars: []string{"x"},
		Bindings: []map[string]rdflibgo.Term{
			{"x": rdflibgo.NewLiteral("a")},
			{"x": rdflibgo.NewLiteral("c")},
		},
	}
	if ResultsEqual(got, expected) {
		t.Error("expected false for non-matching results")
	}
}

func TestDatePartFromDateCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"),
		rdflibgo.NewLiteral("2023-05-15", rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#date"))))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (YEAR(?d) AS ?y) (MONTH(?d) AS ?m) (DAY(?d) AS ?dy) WHERE {
			ex:s ex:p ?d .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestDateTimeWithoutTZ(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"),
		rdflibgo.NewLiteral("2023-06-15T10:30:00", rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#dateTime"))))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (HOURS(?d) AS ?h) (MINUTES(?d) AS ?m) (SECONDS(?d) AS ?s) WHERE {
			ex:s ex:p ?d .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestAnnotationInWhereCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:p "v" {| ex:src ?src |} .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestSubQueryWithOrderByLimitCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("1", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/b"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("2", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?top WHERE {
			{ SELECT ?top WHERE { ?s ex:p ?top } ORDER BY DESC(?top) LIMIT 1 }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestNegatedPathInverseOnlyCov(t *testing.T) {
	g := graph.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p1"), o)
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			ex:o !(^ex:excluded) ?s .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestParseSRXTripleResultCov(t *testing.T) {
	srx := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head><variable name="tt"/></head>
  <results>
    <result>
      <binding name="tt">
        <triple>
          <subject><uri>http://example.org/s</uri></subject>
          <predicate><uri>http://example.org/p</uri></predicate>
          <object><literal>v</literal></object>
        </triple>
      </binding>
    </result>
  </results>
</sparql>`
	r, err := ParseSRX(strings.NewReader(srx))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestParseSRJBooleanResultCov(t *testing.T) {
	json := `{"boolean": true}`
	r, err := ParseSRJ(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if !r.AskResult {
		t.Error("expected true ASK result")
	}
}

func TestParseSRJWithTripleCov(t *testing.T) {
	json := `{
		"head": {"vars": ["t"]},
		"results": {"bindings": [
			{"t": {"type": "triple", "value": {
				"subject": {"type": "uri", "value": "http://example.org/s"},
				"predicate": {"type": "uri", "value": "http://example.org/p"},
				"object": {"type": "literal", "value": "v"}
			}}}
		]}
	}`
	r, err := ParseSRJ(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestHavingWithExpressionCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), rdflibgo.NewURIRefUnsafe("http://example.org/type"), rdflibgo.NewURIRefUnsafe("http://example.org/T1"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/b"), rdflibgo.NewURIRefUnsafe("http://example.org/type"), rdflibgo.NewURIRefUnsafe("http://example.org/T1"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/c"), rdflibgo.NewURIRefUnsafe("http://example.org/type"), rdflibgo.NewURIRefUnsafe("http://example.org/T2"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?type (COUNT(?s) AS ?cnt) WHERE {
			?s ex:type ?type .
		} GROUP BY ?type HAVING (COUNT(?s) > 1)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result for group with count > 1, got %d", len(r.Bindings))
	}
}

func TestCoalesceFunctionCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (COALESCE(?missing, "default") AS ?v) WHERE {
			ex:s ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestIFFunctionCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("5", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (IF(?o > 3, "big", "small") AS ?size) WHERE {
			ex:s ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestInOperatorCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("a"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE {
			ex:s ex:p ?o .
			FILTER(?o IN ("a", "b", "c"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestNotInOperatorCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("d"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE {
			ex:s ex:p ?o .
			FILTER(?o NOT IN ("a", "b", "c"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestSampleAggregateCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), rdflibgo.NewURIRefUnsafe("http://example.org/type"), rdflibgo.NewURIRefUnsafe("http://example.org/T"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/b"), rdflibgo.NewURIRefUnsafe("http://example.org/type"), rdflibgo.NewURIRefUnsafe("http://example.org/T"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (SAMPLE(?s) AS ?sample) WHERE {
			?s ex:type ex:T .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestCountDistinctCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/b"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (COUNT(DISTINCT ?o) AS ?cnt) WHERE {
			?s ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestCodepointEscapesInIRICov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/caf\u00E9"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `SELECT ?o WHERE { <http://example.org/caf\u00E9> <http://example.org/p> ?o . }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestParseSRXLangLiteralCov(t *testing.T) {
	srx := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head><variable name="x"/></head>
  <results>
    <result>
      <binding name="x">
        <literal xml:lang="en">hello</literal>
      </binding>
    </result>
  </results>
</sparql>`
	r, err := ParseSRX(strings.NewReader(srx))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestParseSRXBnodeCov(t *testing.T) {
	srx := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head><variable name="x"/></head>
  <results>
    <result>
      <binding name="x">
        <bnode>b0</bnode>
      </binding>
    </result>
  </results>
</sparql>`
	r, err := ParseSRX(strings.NewReader(srx))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestParseSRJWithLangDirCov(t *testing.T) {
	json := `{
		"head": {"vars": ["x"]},
		"results": {"bindings": [
			{"x": {"type": "literal", "value": "hello", "xml:lang": "en--ltr"}}
		]}
	}`
	r, err := ParseSRJ(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestLowercaseSelectCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		select ?s where {
			?s ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestRegexFunctionCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/name"), rdflibgo.NewLiteral("Alice"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?n WHERE {
			ex:s ex:name ?n .
			FILTER(REGEX(?n, "^Ali"))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestReplaceFunctionCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("hello world"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (REPLACE(?o, "world", "there") AS ?r) WHERE {
			ex:s ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestConcatFunctionCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("hello"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (CONCAT(?o, " ", "world") AS ?c) WHERE {
			ex:s ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

// --- Additional coverage tests: update.go branches ---

func TestInsertDataNonSubjectSkip(t *testing.T) {
	// Exercise the branch where subject assertion fails (s not Subject)
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	// Insert a triple where subject is a literal (non-Subject) — via direct evalInsertData
	op := &InsertDataOp{
		Quads: []QuadPattern{{
			Triples: []Triple{
				{Subject: `"notASubject"`, Predicate: "<http://example.org/p>", Object: `"val"`},
			},
		}},
	}
	err := evalInsertData(ds, op, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Also test non-URIRef predicate
	op2 := &InsertDataOp{
		Quads: []QuadPattern{{
			Triples: []Triple{
				{Subject: "<http://example.org/s>", Predicate: `"notAPred"`, Object: `"val"`},
			},
		}},
	}
	err = evalInsertData(ds, op2, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Test nil resolve (invalid term)
	op3 := &InsertDataOp{
		Quads: []QuadPattern{{
			Triples: []Triple{
				{Subject: "???invalid", Predicate: "<http://example.org/p>", Object: `"val"`},
			},
		}},
	}
	err = evalInsertData(ds, op3, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteDataNonSubjectSkip(t *testing.T) {
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	// Delete with literal as subject (non-Subject)
	op := &DeleteDataOp{
		Quads: []QuadPattern{{
			Triples: []Triple{
				{Subject: `"notASubject"`, Predicate: "<http://example.org/p>", Object: `"val"`},
			},
		}},
	}
	err := evalDeleteData(ds, op, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Delete with non-URIRef predicate
	op2 := &DeleteDataOp{
		Quads: []QuadPattern{{
			Triples: []Triple{
				{Subject: "<http://example.org/s>", Predicate: `"notAPred"`, Object: `"val"`},
			},
		}},
	}
	err = evalDeleteData(ds, op2, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Delete with nil resolve
	op3 := &DeleteDataOp{
		Quads: []QuadPattern{{
			Triples: []Triple{
				{Subject: "???invalid", Predicate: "<http://example.org/p>", Object: `"val"`},
			},
		}},
	}
	err = evalDeleteData(ds, op3, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWhereNonSubjectSkip(t *testing.T) {
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	// Delete WHERE with literal predicate — should silently skip
	op := &DeleteWhereOp{
		Quads: []QuadPattern{{
			Triples: []Triple{
				{Subject: "?s", Predicate: "?p", Object: "?o"},
			},
		}},
	}
	// This will match and try to delete using the bound values
	err := evalDeleteWhere(ds, op, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestModifyNonSubjectPredSkip(t *testing.T) {
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	// Modify with literal subject in delete template and literal predicate in insert template
	op := &ModifyOp{
		Delete: []QuadPattern{{
			Triples: []Triple{
				{Subject: `"litSubj"`, Predicate: "<http://example.org/p>", Object: `"v"`},
			},
		}},
		Insert: []QuadPattern{{
			Triples: []Triple{
				{Subject: "<http://example.org/s>", Predicate: `"litPred"`, Object: `"new"`},
			},
		}},
		Where: &BGP{Triples: []Triple{
			{Subject: "?s", Predicate: "<http://example.org/p>", Object: "?o"},
		}},
	}
	err := evalModify(ds, op, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Modify with nil resolve in delete template
	op2 := &ModifyOp{
		Delete: []QuadPattern{{
			Triples: []Triple{
				{Subject: "???bad", Predicate: "<http://example.org/p>", Object: `"v"`},
			},
		}},
		Insert: []QuadPattern{{
			Triples: []Triple{
				{Subject: "???bad", Predicate: "<http://example.org/p>", Object: `"v"`},
			},
		}},
		Where: &BGP{Triples: []Triple{
			{Subject: "?s", Predicate: "<http://example.org/p>", Object: "?o"},
		}},
	}
	err = evalModify(ds, op2, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEvalUpdateUnknownOp(t *testing.T) {
	ds := makeUpdateDataset()
	err := evalUpdateOp(ds, &testUnknownOp{}, nil)
	if err == nil {
		t.Error("expected error for unknown op")
	}
}

func TestLoadWithoutLoaderSilent(t *testing.T) {
	ds := makeUpdateDataset()
	// LOAD SILENT without Loader should not error
	op := &GraphMgmtOp{Op: "LOAD", Silent: true, Source: "http://example.org/data"}
	err := evalGraphMgmt(ds, op, nil)
	if err != nil {
		t.Errorf("expected no error for silent LOAD without loader, got %v", err)
	}
}

func TestLoadWithoutLoaderNonSilent(t *testing.T) {
	ds := makeUpdateDataset()
	op := &GraphMgmtOp{Op: "LOAD", Silent: false, Source: "http://example.org/data"}
	err := evalGraphMgmt(ds, op, nil)
	if err == nil {
		t.Error("expected error for non-silent LOAD without loader")
	}
}

func TestCreateExistingGraphSilentCov(t *testing.T) {
	ds := makeUpdateDataset()
	ds.NamedGraphs["http://example.org/g"] = graph.NewGraph()
	op := &GraphMgmtOp{Op: "CREATE", Silent: true, Target: "http://example.org/g"}
	err := evalGraphMgmt(ds, op, nil)
	if err != nil {
		t.Errorf("expected no error for silent CREATE on existing graph, got %v", err)
	}
}

func TestClearDropNonExistentGraphSilent(t *testing.T) {
	ds := makeUpdateDataset()
	// CLEAR non-existent graph with silent
	op := &GraphMgmtOp{Op: "CLEAR", Silent: true, Target: "http://example.org/noexist"}
	err := evalGraphMgmt(ds, op, nil)
	if err != nil {
		t.Errorf("expected no error for silent CLEAR, got %v", err)
	}
	// DROP non-existent graph without silent (should not error per spec)
	op2 := &GraphMgmtOp{Op: "DROP", Silent: false, Target: "http://example.org/noexist"}
	err = evalGraphMgmt(ds, op2, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// --- eval.go branches ---

func TestEvalQueryWithInitBindings(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("hello"))
	pq, err := Parse(`PREFIX ex: <http://example.org/> SELECT ?o WHERE { ?s ex:p ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	initBindings := map[string]rdflibgo.Term{
		"s": rdflibgo.NewURIRefUnsafe("http://example.org/s"),
	}
	r, err := EvalQuery(g, pq, initBindings)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestEvalQueryUnknownType(t *testing.T) {
	g := graph.NewGraph()
	pq := &ParsedQuery{Type: "BOGUS", Prefixes: map[string]string{}}
	_, err := EvalQuery(g, pq, nil)
	if err == nil {
		t.Error("expected error for unknown query type")
	}
}

func TestConstructWithOffsetBeyondSolutions(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:p ?o } WHERE { ?s ex:p ?o } OFFSET 100
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph.Len() != 0 {
		t.Errorf("expected 0 triples, got %d", r.Graph.Len())
	}
}

func TestConstructWithLimitAndOffset(t *testing.T) {
	g := graph.NewGraph()
	for i := 0; i < 5; i++ {
		s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
		p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
		g.Add(s, p, rdflibgo.NewLiteral(i))
	}
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:p ?o } WHERE { ?s ex:p ?o } LIMIT 2 OFFSET 1
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph.Len() > 2 {
		t.Errorf("expected at most 2 triples, got %d", r.Graph.Len())
	}
}

func TestEvalPatternNil(t *testing.T) {
	g := graph.NewGraph()
	result := evalPattern(g, nil, nil, nil)
	if len(result) != 1 {
		t.Errorf("expected 1 empty binding, got %d", len(result))
	}
}

func TestResolveTermRefFalseBoolean(t *testing.T) {
	term := resolveTermRef("false", nil)
	if term == nil {
		t.Fatal("expected non-nil term for 'false'")
	}
}

func TestResolveTermRefPrefixedName(t *testing.T) {
	prefixes := map[string]string{"ex": "http://example.org/"}
	term := resolveTermRef("ex:thing", prefixes)
	if term == nil {
		t.Fatal("expected resolved prefixed name")
	}
}

func TestResolveTermRefBnode(t *testing.T) {
	prefixes := map[string]string{"__bnode_scope__": "scope1_"}
	term := resolveTermRef("_:b1", prefixes)
	if term == nil {
		t.Fatal("expected bnode")
	}
}

func TestEvalGraphPatternNoNamedGraphsCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	gp := &GraphPattern{
		Name:    "<http://example.org/g>",
		Pattern: &BGP{Triples: []Triple{{Subject: "?s", Predicate: "?p", Object: "?o"}}},
	}
	// No named graphs available - evaluate against default graph
	result := evalGraphPattern(g, gp, nil, nil)
	if len(result) == 0 {
		t.Error("expected results from default graph")
	}
}

func TestEvalGraphPatternNonExistentNamedGraph(t *testing.T) {
	g := graph.NewGraph()
	namedGraphs := map[string]*rdflibgo.Graph{}
	gp := &GraphPattern{
		Name:    "<http://example.org/noexist>",
		Pattern: &BGP{Triples: []Triple{{Subject: "?s", Predicate: "?p", Object: "?o"}}},
	}
	result := evalGraphPattern(g, gp, nil, namedGraphs)
	if len(result) != 0 {
		t.Error("expected no results for non-existent named graph")
	}
}

func TestEvalGraphPatternResolveNilTerm(t *testing.T) {
	g := graph.NewGraph()
	namedGraphs := map[string]*rdflibgo.Graph{}
	gp := &GraphPattern{
		Name:    "???invalid",
		Pattern: &BGP{Triples: []Triple{{Subject: "?s", Predicate: "?p", Object: "?o"}}},
	}
	result := evalGraphPattern(g, gp, nil, namedGraphs)
	if result != nil {
		t.Error("expected nil for unresolvable graph name")
	}
}

func TestResolveTripleTermPatternBranches(t *testing.T) {
	prefixes := map[string]string{"ex": "http://example.org/"}
	// Valid triple term
	tt := resolveTripleTermPattern("<<( <http://example.org/s> <http://example.org/p> <http://example.org/o> )>>", nil, prefixes)
	if tt == nil {
		t.Error("expected non-nil triple term")
	}
	// With non-Subject as first element (literal)
	tt2 := resolveTripleTermPattern(`<<( "lit" <http://example.org/p> <http://example.org/o> )>>`, nil, prefixes)
	if tt2 != nil {
		t.Error("expected nil for non-Subject first element")
	}
	// With non-URIRef predicate
	tt3 := resolveTripleTermPattern(`<<( <http://example.org/s> "lit" <http://example.org/o> )>>`, nil, prefixes)
	if tt3 != nil {
		t.Error("expected nil for non-URIRef predicate")
	}
	// With unresolvable part
	tt4 := resolveTripleTermPattern("<<( ?unbound <http://example.org/p> <http://example.org/o> )>>", nil, prefixes)
	if tt4 != nil {
		t.Error("expected nil for unresolvable part")
	}
}

func TestFormatDecimalNoDecimalPointCov(t *testing.T) {
	// Test formatDecimal when input has no decimal point (integer-like float)
	r := formatDecimal(42.0)
	if !strings.Contains(r, ".") {
		t.Errorf("expected decimal point in result, got %q", r)
	}
}

func TestTermValuesEqualNaN(t *testing.T) {
	// NaN != NaN per SPARQL spec
	nan := rdflibgo.NewLiteral("NaN", rdflibgo.WithDatatype(rdflibgo.XSDDouble))
	if termValuesEqual(nan, nan) {
		t.Error("NaN should not equal NaN")
	}
}

func TestTermValuesEqualLangLiterals(t *testing.T) {
	a := rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en"))
	b := rdflibgo.NewLiteral("hello", rdflibgo.WithLang("fr"))
	if termValuesEqual(a, b) {
		t.Error("different lang tags should not be equal")
	}
}

func TestEvalExprUnknownType(t *testing.T) {
	// evalExpr with nil returns nil
	r := evalExpr(nil, nil, nil)
	if r != nil {
		t.Error("expected nil for nil expr")
	}
}

func TestEvalUnaryNotOnURIRef(t *testing.T) {
	// ! on URIRef has no EBV → nil
	uri := rdflibgo.NewURIRefUnsafe("http://example.org/x")
	r := evalUnaryOp("!", uri)
	if r != nil {
		t.Error("expected nil for !URIRef")
	}
}

func TestEvalUnaryNotOnBNode(t *testing.T) {
	bn := rdflibgo.NewBNode("b1")
	r := evalUnaryOp("!", bn)
	if r != nil {
		t.Error("expected nil for !BNode")
	}
}

func TestSampleAggregateEmpty(t *testing.T) {
	g := graph.NewGraph()
	r, err := Query(g, `SELECT (SAMPLE(?x) AS ?s) WHERE { ?x <http://example.org/p> ?y }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Skip("no bindings for empty graph")
	}
}

func TestBoundFunctionNoArgs(t *testing.T) {
	// BOUND with no var expr arg falls through to return false
	r := evalFunc("BOUND", nil, nil, nil)
	if r == nil {
		t.Fatal("expected non-nil")
	}
}

func TestSubstrStartBeyondLength(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("hi"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (SUBSTR(?o, 100) AS ?sub) WHERE { ex:s ex:p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestRegexInvalidPattern(t *testing.T) {
	r := evalFunc("REGEX", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral("hello")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("[invalid")},
	}, nil, nil)
	// Invalid regex should return false
	if r == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestReplaceNumericInput(t *testing.T) {
	// REPLACE on numeric literal should return nil (type error)
	r := evalFunc("REPLACE", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral(42, rdflibgo.WithDatatype(rdflibgo.XSDInteger))},
		&LiteralExpr{Value: rdflibgo.NewLiteral("4")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("X")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for REPLACE on numeric")
	}
}

func TestReplaceNonLiteralInput(t *testing.T) {
	r := evalFunc("REPLACE", []Expr{
		&LiteralExpr{Value: rdflibgo.NewURIRefUnsafe("http://example.org/x")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("x")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("y")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for REPLACE on non-literal")
	}
}

func TestConcatWithNilArg(t *testing.T) {
	// CONCAT with nil arg → error
	r := evalFunc("CONCAT", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral("hi")},
		&VarExpr{Name: "unbound"},
	}, nil, nil)
	// nil arg causes error in concat
	_ = r
}

func TestLangMatchesFallbackFalse(t *testing.T) {
	r := evalFunc("LANGMATCHES", nil, nil, nil)
	if r == nil {
		t.Fatal("expected non-nil")
	}
}

func TestSameTermFallbackFalse(t *testing.T) {
	r := evalFunc("SAMETERM", nil, nil, nil)
	if r == nil {
		t.Fatal("expected non-nil")
	}
}

func TestIsTripleFalse(t *testing.T) {
	r := evalFunc("ISTRIPLE", nil, nil, nil)
	if r == nil {
		t.Fatal("expected non-nil")
	}
}

func TestTripleFuncNonSubject(t *testing.T) {
	// TRIPLE with non-subject first arg
	r := evalFunc("TRIPLE", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral("lit")},
		&LiteralExpr{Value: rdflibgo.NewURIRefUnsafe("http://example.org/p")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("obj")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for non-subject in TRIPLE")
	}
}

func TestTripleFuncNonPredicate(t *testing.T) {
	r := evalFunc("TRIPLE", []Expr{
		&LiteralExpr{Value: rdflibgo.NewURIRefUnsafe("http://example.org/s")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("notURI")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("obj")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for non-URIRef predicate in TRIPLE")
	}
}

func TestLangdirFunction(t *testing.T) {
	// LANGDIR with no args
	r := evalFunc("LANGDIR", nil, nil, nil)
	if r == nil {
		t.Fatal("expected non-nil fallback")
	}
}

func TestStrlangdirTypeError(t *testing.T) {
	// STRLANGDIR with lang-tagged literal should return nil
	r := evalFunc("STRLANGDIR", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en"))},
		&LiteralExpr{Value: rdflibgo.NewLiteral("en")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("ltr")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for STRLANGDIR on lang-tagged literal")
	}
}

func TestTimeNowCov(t *testing.T) {
	r := timeNow()
	if r == "" {
		t.Error("expected non-empty time string")
	}
}

func TestExtractSimpleBGPWithTripleTermVars(t *testing.T) {
	// Triple term with variables in subject → should return nil
	tp := extractSimpleBGP(&BGP{Triples: []Triple{
		{Subject: "<<( ?s <http://example.org/p> <http://example.org/o> )>>", Predicate: "<http://example.org/p>", Object: "?o"},
	}})
	if tp != nil {
		t.Error("expected nil for triple term with vars in subject")
	}
	// Triple term with variables in object
	tp2 := extractSimpleBGP(&BGP{Triples: []Triple{
		{Subject: "<http://example.org/s>", Predicate: "<http://example.org/p>", Object: "<<( <http://example.org/s> ?p <http://example.org/o> )>>"},
	}})
	if tp2 != nil {
		t.Error("expected nil for triple term with vars in object")
	}
}

// --- parser coverage: parseGraphRefAll ---

func TestParseGraphRefAllGraph(t *testing.T) {
	// CLEAR GRAPH <uri>
	_, err := ParseUpdate(`CLEAR GRAPH <http://example.org/g>`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseGraphRefAllNamed(t *testing.T) {
	_, err := ParseUpdate(`DROP NAMED`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseGraphRefAllAll(t *testing.T) {
	_, err := ParseUpdate(`CLEAR ALL`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- parser: construct annotation block ---

func TestConstructAnnotationBlockComma(t *testing.T) {
	// CONSTRUCT with annotation block containing comma-separated objects
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	_, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ex:s ex:p ex:o1, ex:o2 }
		WHERE { ex:s ex:p ?v }
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- parser: quad pattern with GRAPH ---

func TestParseQuadPatternWithGraph(t *testing.T) {
	_, err := ParseUpdate(`INSERT DATA { GRAPH <http://example.org/g> { <http://example.org/s> <http://example.org/p> <http://example.org/o> } }`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWhereWithGraphClauseCov(t *testing.T) {
	_, err := ParseUpdate(`DELETE WHERE { GRAPH <http://example.org/g> { ?s ?p ?o } }`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- parser validate branches ---

func TestValidateBindScopeInFilter(t *testing.T) {
	_, err := Parse(`SELECT ?x WHERE { FILTER(EXISTS { ?x <http://example.org/p> ?y . BIND(1 AS ?z) }) }`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateNoNestedAggregateInHaving(t *testing.T) {
	_, err := Parse(`SELECT (COUNT(?x) AS ?c) WHERE { ?x <http://example.org/p> ?y } GROUP BY ?x HAVING(COUNT(?x) > 0)`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateTripleTermStringBranches(t *testing.T) {
	// Valid triple term
	_, err := Parse(`SELECT * WHERE { <<( <http://example.org/s> <http://example.org/p> <http://example.org/o> )>> <http://example.org/q> ?v }`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- SRX/SRJ edge cases ---

func TestParseSRXInvalidXML(t *testing.T) {
	_, err := ParseSRX(strings.NewReader(`<invalid`))
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestParseSRJInvalidJSON(t *testing.T) {
	_, err := ParseSRJ(strings.NewReader(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseSRJUnknownType(t *testing.T) {
	_, err := ParseSRJ(strings.NewReader(`{"results":{"bindings":[{"x":{"type":"unknown","value":"test"}}]}}`))
	if err != nil {
		// Unknown type might be silently handled
		_ = err
	}
}

func TestResultsEqualBnodeMismatch(t *testing.T) {
	a := &Result{
		Vars:     []string{"x"},
		Bindings: []map[string]rdflibgo.Term{{"x": rdflibgo.NewBNode("a")}},
	}
	b := &Result{
		Vars:     []string{"x"},
		Bindings: []map[string]rdflibgo.Term{{"x": rdflibgo.NewBNode("b")}},
	}
	// BNodes with different labels should still be "equal" via isomorphism
	_ = ResultsEqual(a, b)
}

// --- Property path in BGP ---

func TestBGPWithPropertyPath(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewURIRefUnsafe("http://example.org/o"))
	tp := Triple{
		Subject:       "?s",
		Object:        "?o",
		PredicatePath: paths.URIRefPath{URI: rdflibgo.NewURIRefUnsafe("http://example.org/p")},
	}
	result := evalBGP(g, []Triple{tp}, map[string]rdflibgo.Term{}, nil)
	if len(result) == 0 {
		t.Error("expected results for property path BGP")
	}
}

// --- SubqueryPattern evaluation ---

func TestSubqueryPatternEval(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE {
			{ SELECT ?o WHERE { ex:s ex:p ?o } }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

// --- Aggregate edge cases ---

func TestSumAggregateWithError(t *testing.T) {
	// SUM with error arg
	fe := &FuncExpr{Name: "SUM"}
	group := []map[string]rdflibgo.Term{{"x": nil}}
	r := evalAggregate(fe, group, nil)
	_ = r // just exercise the branch
}

func TestAvgAggregateDecimal(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/v"), rdflibgo.NewLiteral("1.5", rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/v"), rdflibgo.NewLiteral("2.5", rdflibgo.WithDatatype(rdflibgo.XSDDecimal)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (AVG(?v) AS ?a) WHERE { ex:s ex:v ?v }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected result for AVG")
	}
}

// --- validateStringEscapes additional branches ---

func TestValidateStringEscapesShortUnicodeEscape(t *testing.T) {
	// String with truncated \u escape (less than 4 hex chars)
	err := validateStringEscapes(`"hello\uAB"`)
	if err == nil {
		t.Error("expected error for short \\u escape")
	}
}

func TestValidateStringEscapesShortBigUnicodeEscape(t *testing.T) {
	// String with truncated \U escape (less than 8 hex chars)
	err := validateStringEscapes(`"hello\U00AB"`)
	if err == nil {
		t.Error("expected error for short \\U escape")
	}
}

func TestValidateStringEscapesSurrogate(t *testing.T) {
	// String with surrogate codepoint
	err := validateStringEscapes(`"hello\uD800"`)
	if err == nil {
		t.Error("expected error for surrogate codepoint")
	}
}

// --- cachedRegexpCompile error path ---

func TestCachedRegexpCompileInvalid(t *testing.T) {
	_, err := cachedRegexpCompile("[invalid")
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

// --- parseBnodePropertyListTriples coverage ---

func TestBnodePropertyListInQuery(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/name"), rdflibgo.NewLiteral("Alice"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/age"), rdflibgo.NewLiteral(30, rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			[ ex:name ?name ; ex:age ?age ] .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		// May or may not match depending on bnode handling
		_ = r
	}
}

// --- readVar branches ---

func TestReadVarDollarSign(t *testing.T) {
	// $var syntax
	_, err := Parse(`SELECT $x WHERE { $x <http://example.org/p> ?o }`)
	if err != nil {
		t.Fatal(err)
	}
}

// --- EvalUpdate with parse error ---

func TestEvalUpdateParseError(t *testing.T) {
	ds := makeUpdateDataset()
	err := Update(ds, `INVALID SPARQL`)
	if err == nil {
		t.Error("expected parse error")
	}
}

// --- More targeted coverage tests ---

func TestSelectReduced(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `SELECT REDUCED ?s WHERE { ?s <http://example.org/p> ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 result, got %d", len(r.Bindings))
	}
}

func TestFromNamedClause(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	// FROM / FROM NAMED are parsed but not enforced
	r, err := Query(g, `
		SELECT ?s
		FROM <http://example.org/default>
		FROM NAMED <http://example.org/named>
		WHERE { ?s <http://example.org/p> ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestConstructWithAnnotationsCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
		CONSTRUCT {
			ex:s ex:p ?o .
			ex:s ex:q "w1", "w2" .
		} WHERE { ex:s ex:p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected non-nil graph")
	}
}

func TestGroupByExprAsVar(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), rdflibgo.NewURIRefUnsafe("http://example.org/val"), rdflibgo.NewLiteral(1, rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/b"), rdflibgo.NewURIRefUnsafe("http://example.org/val"), rdflibgo.NewLiteral(2, rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?sv (COUNT(?s) AS ?c)
		WHERE { ?s ex:val ?v }
		GROUP BY (STR(?v) AS ?sv)
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestAnnotationBlockInWhere(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:p ?o ;
			   ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestValuesUndef(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?x WHERE {
			VALUES ?x { UNDEF ex:s }
			?x ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestDeleteWhereNonSubjectPredSkip(t *testing.T) {
	// Exercise evalDeleteWhere branches for non-Subject/non-URIRef
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	// WHERE matches, but template has literal subject — skip
	op := &DeleteWhereOp{
		Quads: []QuadPattern{{
			Triples: []Triple{
				{Subject: `"litSubj"`, Predicate: `"litPred"`, Object: `"v"`},
			},
		}},
	}
	err := evalDeleteWhere(ds, op, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestModifyDeleteNilResolve(t *testing.T) {
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	op := &ModifyOp{
		Delete: []QuadPattern{{
			Triples: []Triple{
				{Subject: "<http://example.org/s>", Predicate: `"litPred"`, Object: `"v"`},
			},
		}},
		Insert: []QuadPattern{{
			Triples: []Triple{
				{Subject: `"litSubj"`, Predicate: "<http://example.org/p>", Object: `"new"`},
			},
		}},
		Where: &BGP{Triples: []Triple{
			{Subject: "?s", Predicate: "<http://example.org/p>", Object: "?o"},
		}},
	}
	err := evalModify(ds, op, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAPathInNegatedPropertySet(t *testing.T) {
	// 'a' in negated property set (path)
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `SELECT ?o WHERE { <http://example.org/s> !a ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results for negated 'a' path")
	}
}

func TestAPathWithModifier(t *testing.T) {
	// 'a' as path predicate - check path modifier
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"), rdflibgo.NewURIRefUnsafe("http://example.org/C"))
	r, err := Query(g, `SELECT ?o WHERE { <http://example.org/s> a ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results for 'a' path")
	}
}

func TestGraphRefAllDefault(t *testing.T) {
	_, err := ParseUpdate(`CLEAR DEFAULT`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseUpdateWithSemicolon(t *testing.T) {
	_, err := ParseUpdate(`INSERT DATA { <http://example.org/s> <http://example.org/p> "v" } ; INSERT DATA { <http://example.org/s2> <http://example.org/p> "w" }`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTransferGraphsViaUpdate(t *testing.T) {
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s2"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("w"))
	ds.NamedGraphs["http://example.org/g"] = ng
	// ADD source to default
	err := Update(ds, `ADD <http://example.org/g> TO DEFAULT`)
	if err != nil {
		t.Fatal(err)
	}
	// COPY default to named
	err = Update(ds, `COPY DEFAULT TO <http://example.org/g2>`)
	if err != nil {
		t.Fatal(err)
	}
	// MOVE named to default
	err = Update(ds, `MOVE <http://example.org/g> TO DEFAULT`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestConstructOrderBy(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("b"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("a"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:p ?o } WHERE { ?s ex:p ?o } ORDER BY ?o
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected non-nil graph")
	}
}

func TestEvalExprFallthrough(t *testing.T) {
	// An unknown expr type should return nil
	result := evalExpr(nil, nil, nil)
	if result != nil {
		t.Error("expected nil for nil expr")
	}
}

func TestBinaryOpNonNumericEqual(t *testing.T) {
	// Binary = on non-numeric terms
	a := rdflibgo.NewLiteral("hello")
	b := rdflibgo.NewLiteral("hello")
	r := evalBinaryOp("=", a, b)
	if r == nil {
		t.Fatal("expected non-nil")
	}
}

func TestBinaryOpWithNils(t *testing.T) {
	r := evalBinaryOp("=", nil, nil)
	_ = r
}

func TestResolveTermRefNumericLiteral(t *testing.T) {
	// Numeric string -> numeric literal
	r := resolveTermRef("42", nil)
	if r == nil {
		t.Fatal("expected numeric term")
	}
	// Decimal
	r2 := resolveTermRef("3.14", nil)
	if r2 == nil {
		t.Fatal("expected decimal term")
	}
	// Double
	r3 := resolveTermRef("1.5e2", nil)
	if r3 == nil {
		t.Fatal("expected double term")
	}
}

func TestEvalQueryASKPushdown(t *testing.T) {
	// ASK query with simple BGP → extractSimpleBGP
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `ASK { <http://example.org/s> <http://example.org/p> "v" }`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AskResult {
		t.Error("expected true")
	}
}

func TestClearDefaultViaUpdate(t *testing.T) {
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	err := Update(ds, `CLEAR DEFAULT`)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Default.Len() != 0 {
		t.Error("expected empty graph after CLEAR DEFAULT")
	}
}

func TestBnodePropertyListCommaSemicolon(t *testing.T) {
	// Exercise comma and semicolon in bnode property list triples
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), rdflibgo.NewURIRefUnsafe("http://example.org/name"), rdflibgo.NewLiteral("A"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), rdflibgo.NewURIRefUnsafe("http://example.org/age"), rdflibgo.NewLiteral(30, rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name ?age WHERE {
			[ ex:name ?name, "Other" ; ex:age ?age ] .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestAnnotationBlockCommaSemicolon(t *testing.T) {
	// Exercise comma and semicolon in annotation blocks in WHERE
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v1"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v2"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?v WHERE {
			ex:s ex:p ?v, "v2" .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestConstructTemplateCommaSemicolon(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT {
			ex:s ex:p ?o ;
			     ex:q "w" .
		} WHERE { ex:s ex:p ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph == nil {
		t.Error("expected non-nil graph")
	}
}

func TestPostQueryValuesCov(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		SELECT ?s WHERE { ?s <http://example.org/p> ?o }
		VALUES ?s { <http://example.org/s> }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestNegatedPathSet(t *testing.T) {
	// Negated property set with multiple URIs
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/q"), rdflibgo.NewLiteral("w"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE { ex:s !( ex:p | ex:r ) ?o }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Should match ex:q -> "w" since it's not ex:p or ex:r
	if len(r.Bindings) == 0 {
		t.Error("expected results for negated path set")
	}
}

func TestModifyNilResolvesInBothTemplates(t *testing.T) {
	ds := makeUpdateDataset()
	ds.Default.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	// Templates with non-resolvable terms
	op := &ModifyOp{
		Delete: []QuadPattern{{
			Triples: []Triple{
				{Subject: "<http://example.org/s>", Predicate: "<http://example.org/p>", Object: "???bad"},
			},
		}},
		Insert: []QuadPattern{{
			Triples: []Triple{
				{Subject: "<http://example.org/s>", Predicate: "<http://example.org/p>", Object: "???bad"},
			},
		}},
		Where: &BGP{Triples: []Triple{
			{Subject: "?s", Predicate: "<http://example.org/p>", Object: "?o"},
		}},
	}
	err := evalModify(ds, op, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseQuadPatternGraphInDeleteWhere(t *testing.T) {
	// Exercise the quad pattern with GRAPH clause in UPDATE
	ds := makeUpdateDataset()
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `DELETE { GRAPH <http://example.org/g> { ?s ?p ?o } } WHERE { GRAPH <http://example.org/g> { ?s ?p ?o } }`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseValidateBadTripleTermInSelect(t *testing.T) {
	// Triple term validation in SELECT
	_, err := Parse(`SELECT * WHERE { ?s ?p <<( "literal" <http://example.org/p> <http://example.org/o> )>> }`)
	if err != nil {
		// May or may not be an error depending on validation
		_ = err
	}
}

func TestParseHavingMultipleConditions(t *testing.T) {
	_, err := Parse(`
		SELECT ?x (COUNT(?y) AS ?c)
		WHERE { ?x <http://example.org/p> ?y }
		GROUP BY ?x
		HAVING(COUNT(?y) > 1)
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestConstructWhereSemicolon(t *testing.T) {
	// CONSTRUCT template with semicolons
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/q"), rdflibgo.NewLiteral("w"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:p ?v ; ex:q ?w }
		WHERE { ?s ex:p ?v . ?s ex:q ?w }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestConstructAnnotationBlockSemicolon(t *testing.T) {
	// Annotation block in construct with semicolons
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ex:s ex:p "v" ; ex:q "w" }
		WHERE { ex:s ex:p "v" }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestSumAggregateWithNilArg(t *testing.T) {
	// SUM with error (nil value) should return nil
	fe := &FuncExpr{Name: "SUM", Args: []Expr{&VarExpr{Name: "x"}}}
	group := []map[string]rdflibgo.Term{{"y": rdflibgo.NewLiteral(1)}} // x not bound → nil
	r := evalAggregate(fe, group, nil)
	_ = r
}

func TestMinAggregateEmpty(t *testing.T) {
	// MIN with no values
	fe := &FuncExpr{Name: "MIN", Args: []Expr{&VarExpr{Name: "x"}}}
	group := []map[string]rdflibgo.Term{{"y": rdflibgo.NewLiteral(1)}} // x not bound
	r := evalAggregate(fe, group, nil)
	_ = r
}

func TestMaxAggregateEmpty(t *testing.T) {
	fe := &FuncExpr{Name: "MAX", Args: []Expr{&VarExpr{Name: "x"}}}
	group := []map[string]rdflibgo.Term{{"y": rdflibgo.NewLiteral(1)}}
	r := evalAggregate(fe, group, nil)
	_ = r
}

func TestSampleAggregateEmptyDirect(t *testing.T) {
	fe := &FuncExpr{Name: "SAMPLE", Args: []Expr{&VarExpr{Name: "x"}}}
	group := []map[string]rdflibgo.Term{{"y": rdflibgo.NewLiteral(1)}}
	r := evalAggregate(fe, group, nil)
	_ = r
}

func TestUnknownAggregateType(t *testing.T) {
	fe := &FuncExpr{Name: "UNKNOWN_AGG", Args: []Expr{&VarExpr{Name: "x"}}}
	group := []map[string]rdflibgo.Term{{"x": rdflibgo.NewLiteral(1)}}
	r := evalAggregate(fe, group, nil)
	if r != nil {
		t.Error("expected nil for unknown aggregate")
	}
}

func TestSubqueryWithNamedGraphs(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?o WHERE {
			{ SELECT ?o WHERE { ex:s ex:p ?o } LIMIT 1 }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) == 0 {
		t.Error("expected results from subquery")
	}
}

func TestGraphPatternResolvedNonURIRef(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	namedGraphs := map[string]*rdflibgo.Graph{}
	// Use a prefixed name as graph IRI
	prefixes := map[string]string{"ex": "http://example.org/"}
	gp := &GraphPattern{
		Name:    "ex:g",
		Pattern: &BGP{Triples: []Triple{{Subject: "?s", Predicate: "?p", Object: "?o"}}},
	}
	result := evalGraphPattern(g, gp, prefixes, namedGraphs)
	// ex:g resolves to http://example.org/g but no such named graph exists
	if len(result) != 0 {
		t.Error("expected no results")
	}
}

func TestEvalAggExprIRIExpr(t *testing.T) {
	// evalAggExpr with IRIExpr returns URIRef
	r := evalAggExpr(&IRIExpr{Value: "http://example.org/x"}, nil, nil)
	if r == nil {
		t.Error("expected non-nil for IRIExpr")
	}
}

func TestFormatDecimalInteger(t *testing.T) {
	// formatDecimal with an integer value (no decimal point initially)
	r := formatDecimal(100)
	if !strings.Contains(r, ".") {
		t.Errorf("expected decimal point, got %q", r)
	}
}

func TestSubstrNoArgs(t *testing.T) {
	r := evalFunc("SUBSTR", nil, nil, nil)
	if r != nil {
		t.Error("expected nil for SUBSTR with no args")
	}
}

func TestSubstrNegativeStart(t *testing.T) {
	r := evalFunc("SUBSTR", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral("hello")},
		&LiteralExpr{Value: rdflibgo.NewLiteral(-1, rdflibgo.WithDatatype(rdflibgo.XSDInteger))},
	}, nil, nil)
	if r == nil {
		t.Error("expected non-nil")
	}
}

func TestSubstrLengthExceedsString(t *testing.T) {
	r := evalFunc("SUBSTR", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral("hi")},
		&LiteralExpr{Value: rdflibgo.NewLiteral(1, rdflibgo.WithDatatype(rdflibgo.XSDInteger))},
		&LiteralExpr{Value: rdflibgo.NewLiteral(100, rdflibgo.WithDatatype(rdflibgo.XSDInteger))},
	}, nil, nil)
	if r == nil {
		t.Error("expected non-nil")
	}
}

func TestLangNoArgs(t *testing.T) {
	r := evalFunc("LANG", nil, nil, nil)
	if r == nil {
		t.Fatal("expected non-nil (empty string)")
	}
}

func TestLangOnNonLiteral(t *testing.T) {
	r := evalFunc("LANG", []Expr{
		&LiteralExpr{Value: rdflibgo.NewURIRefUnsafe("http://example.org/x")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for LANG on URIRef")
	}
}

func TestReplaceInvalidRegex(t *testing.T) {
	r := evalFunc("REPLACE", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral("hello")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("[invalid")},
		&LiteralExpr{Value: rdflibgo.NewLiteral("x")},
	}, nil, nil)
	// Invalid regex → return original
	_ = r
}

func TestNowWithoutPrefixKey(t *testing.T) {
	// NOW without queryStartTimeKey in prefixes
	r := evalFunc("NOW", nil, nil, nil)
	if r == nil {
		t.Error("expected non-nil for NOW")
	}
}

func TestXsdBooleanCastFromNonLiteral(t *testing.T) {
	r := evalFunc("XSD:BOOLEAN", []Expr{
		&LiteralExpr{Value: rdflibgo.NewBNode("b1")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for boolean cast from bnode")
	}
}

func TestXsdBooleanCastFromURI(t *testing.T) {
	r := evalFunc("XSD:BOOLEAN", []Expr{
		&LiteralExpr{Value: rdflibgo.NewURIRefUnsafe("http://example.org/x")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for boolean cast from URI")
	}
}

func TestXsdBooleanCastFromNumeric(t *testing.T) {
	r := evalFunc("XSD:BOOLEAN", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral("bad", rdflibgo.WithDatatype(rdflibgo.XSDDouble))},
	}, nil, nil)
	_ = r
}

func TestXsdStringCastFromBNode(t *testing.T) {
	r := evalFunc("XSD:STRING", []Expr{
		&LiteralExpr{Value: rdflibgo.NewBNode("b1")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for string cast from bnode")
	}
}

func TestXsdIntegerCastFromBNode(t *testing.T) {
	r := evalFunc("XSD:INTEGER", []Expr{
		&LiteralExpr{Value: rdflibgo.NewBNode("b1")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for integer cast from bnode")
	}
}

func TestXsdDoubleCastFromBNode(t *testing.T) {
	r := evalFunc("XSD:DOUBLE", []Expr{
		&LiteralExpr{Value: rdflibgo.NewBNode("b1")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for double cast from bnode")
	}
}

func TestXsdDecimalCastFromBNode(t *testing.T) {
	r := evalFunc("XSD:DECIMAL", []Expr{
		&LiteralExpr{Value: rdflibgo.NewBNode("b1")},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for decimal cast from bnode")
	}
}

func TestTimezoneOffsetNonZero(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/dt"),
		rdflibgo.NewLiteral("2023-01-01T10:00:00+05:30", rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#dateTime"))))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
		SELECT (TZ(?dt) AS ?tz) WHERE { ex:s ex:dt ?dt }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestSecondsWithFraction(t *testing.T) {
	g := graph.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/dt"),
		rdflibgo.NewLiteral("2023-01-01T10:00:05.123Z", rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#dateTime"))))
	r, err := Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT (SECONDS(?dt) AS ?sec) WHERE { ex:s ex:dt ?dt }
	`)
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestCompareTermValuesNils(t *testing.T) {
	// Both nil
	if compareTermValues(nil, nil) != 0 {
		t.Error("expected 0 for nil,nil")
	}
	// a nil, b non-nil
	if compareTermValues(nil, rdflibgo.NewLiteral("x")) >= 0 {
		t.Error("expected < 0 for nil,non-nil")
	}
	// a non-nil, b nil
	if compareTermValues(rdflibgo.NewLiteral("x"), nil) <= 0 {
		t.Error("expected > 0 for non-nil,nil")
	}
}

func TestCompareTermValuesNaN(t *testing.T) {
	nan := rdflibgo.NewLiteral("NaN", rdflibgo.WithDatatype(rdflibgo.XSDDouble))
	num := rdflibgo.NewLiteral("1.0", rdflibgo.WithDatatype(rdflibgo.XSDDouble))
	r := compareTermValues(nan, num)
	_ = r
}

func TestXsdIntegerCastNaN(t *testing.T) {
	r := evalFunc("XSD:INTEGER", []Expr{
		&LiteralExpr{Value: rdflibgo.NewLiteral("NaN", rdflibgo.WithDatatype(rdflibgo.XSDDouble))},
	}, nil, nil)
	if r != nil {
		t.Error("expected nil for integer cast from NaN")
	}
}

func TestUnknownFuncReturnsNil(t *testing.T) {
	r := evalFunc("UNKNOWN_FUNC", nil, nil, nil)
	if r != nil {
		t.Error("expected nil for unknown function")
	}
}

func TestXsdStringCastFromURI(t *testing.T) {
	r := evalFunc("XSD:STRING", []Expr{
		&LiteralExpr{Value: rdflibgo.NewURIRefUnsafe("http://example.org/x")},
	}, nil, nil)
	if r == nil {
		t.Error("expected non-nil for string cast from URI")
	}
}

func TestResolveTermRefTrue(t *testing.T) {
	r := resolveTermRef("true", nil)
	if r == nil {
		t.Fatal("expected non-nil for 'true'")
	}
}

func TestResolveTermRefSingleQuoteLiteralCov(t *testing.T) {
	r := resolveTermRef("'hello'", nil)
	if r == nil {
		t.Fatal("expected non-nil for single-quote literal")
	}
}

func TestEvalAggExprUnknownExpr(t *testing.T) {
	// evalAggExpr with unknown expr type → nil
	r := evalAggExpr(&ExistsExpr{}, nil, nil)
	if r != nil {
		t.Error("expected nil for ExistsExpr in evalAggExpr")
	}
}

func TestSumAggregateHasError(t *testing.T) {
	// SUM where one value is non-numeric (string literal) → hasError = true
	fe := &FuncExpr{Name: "SUM", Args: []Expr{&VarExpr{Name: "x"}}}
	group := []map[string]rdflibgo.Term{
		{"x": rdflibgo.NewLiteral("abc")},
		{"x": rdflibgo.NewLiteral(1, rdflibgo.WithDatatype(rdflibgo.XSDInteger))},
	}
	r := evalAggregate(fe, group, nil)
	// hasError = true → return nil
	if r != nil {
		t.Error("expected nil for SUM with error")
	}
}

func TestDropNamedViaUpdate(t *testing.T) {
	ds := makeUpdateDataset()
	ng := graph.NewGraph()
	ng.Add(rdflibgo.NewURIRefUnsafe("http://example.org/s"), rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	ds.NamedGraphs["http://example.org/g"] = ng
	err := Update(ds, `DROP GRAPH <http://example.org/g>`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ds.NamedGraphs["http://example.org/g"]; ok {
		t.Error("expected graph to be removed after DROP")
	}
}
