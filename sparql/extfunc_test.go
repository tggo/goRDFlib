package sparql

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/turtle"
)

const extFuncData = `
@prefix ex: <http://example.org/> .

ex:a ex:value 21 ; ex:name "alice" .
ex:b ex:value 3  ; ex:name "bob" .
`

func extFuncGraph(t *testing.T) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(extFuncData)); err != nil {
		t.Fatal(err)
	}
	return g
}

// doubleFn is the running example from the extfunc.go documentation.
func doubleFn(args []rdflibgo.Term) (rdflibgo.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("double: want 1 argument, got %d", len(args))
	}
	lit, ok := args[0].(rdflibgo.Literal)
	if !ok {
		return nil, fmt.Errorf("double: want a literal, got %T", args[0])
	}
	n, err := strconv.ParseFloat(lit.Lexical(), 64)
	if err != nil {
		return nil, fmt.Errorf("double: %w", err)
	}
	return rdflibgo.NewLiteral(int(n * 2)), nil
}

// registerForTest registers fn and removes it when the test ends, so the global
// registry does not leak between tests.
func registerForTest(t *testing.T, iri string, fn Function) {
	t.Helper()
	if err := RegisterFunction(iri, fn); err != nil {
		t.Fatalf("RegisterFunction(%s): %v", iri, err)
	}
	t.Cleanup(func() { UnregisterFunction(iri) })
}

func TestExtensionFunctionCalledByFullIRI(t *testing.T) {
	registerForTest(t, "http://example.org/fn#double", doubleFn)

	res, err := Query(extFuncGraph(t), `PREFIX ex: <http://example.org/>
		SELECT ?doubled WHERE {
			ex:a ex:value ?v .
			BIND(<http://example.org/fn#double>(?v) AS ?doubled)
		}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("want 1 row, got %d", len(res.Bindings))
	}
	if got := res.Bindings[0]["doubled"]; got == nil || got.(rdflibgo.Literal).Lexical() != "42" {
		t.Fatalf("want 42, got %v", got)
	}
}

func TestExtensionFunctionCalledByPrefixedName(t *testing.T) {
	registerForTest(t, "http://example.org/fn#double", doubleFn)

	res, err := Query(extFuncGraph(t), `PREFIX ex: <http://example.org/>
		PREFIX fn: <http://example.org/fn#>
		SELECT ?doubled WHERE {
			ex:a ex:value ?v .
			BIND(fn:double(?v) AS ?doubled)
		}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("want 1 row, got %d", len(res.Bindings))
	}
	if got := res.Bindings[0]["doubled"]; got == nil || got.(rdflibgo.Literal).Lexical() != "42" {
		t.Fatalf("want 42, got %v", got)
	}
}

func TestExtensionFunctionInFilter(t *testing.T) {
	registerForTest(t, "http://example.org/fn#double", doubleFn)

	res, err := Query(extFuncGraph(t), `PREFIX ex: <http://example.org/>
		PREFIX fn: <http://example.org/fn#>
		SELECT ?s WHERE {
			?s ex:value ?v .
			FILTER(fn:double(?v) > 10)
		}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("want 1 row (only ex:a doubles above 10), got %d: %v", len(res.Bindings), res.Bindings)
	}
	if got := res.Bindings[0]["s"].String(); got != "http://example.org/a" {
		t.Fatalf("want ex:a, got %s", got)
	}
}

// A function that returns an error yields an expression error, which makes the
// enclosing FILTER reject the row rather than failing the query.
func TestExtensionFunctionErrorIsExpressionError(t *testing.T) {
	registerForTest(t, "http://example.org/fn#boom", func([]rdflibgo.Term) (rdflibgo.Term, error) {
		return nil, errors.New("always fails")
	})

	res, err := Query(extFuncGraph(t), `PREFIX ex: <http://example.org/>
		PREFIX fn: <http://example.org/fn#>
		SELECT ?s WHERE {
			?s ex:value ?v .
			FILTER(fn:boom(?v))
		}`)
	if err != nil {
		t.Fatalf("an erroring extension function must not fail the query: %v", err)
	}
	if len(res.Bindings) != 0 {
		t.Fatalf("want 0 rows, got %d", len(res.Bindings))
	}
}

// Multiple arguments, and a non-literal argument reaching the function intact.
func TestExtensionFunctionMultipleArgs(t *testing.T) {
	registerForTest(t, "http://example.org/fn#describe", func(args []rdflibgo.Term) (rdflibgo.Term, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("want 2 args, got %d", len(args))
		}
		u, ok := args[0].(rdflibgo.URIRef)
		if !ok {
			return nil, fmt.Errorf("want an IRI, got %T", args[0])
		}
		lit, ok := args[1].(rdflibgo.Literal)
		if !ok {
			return nil, fmt.Errorf("want a literal, got %T", args[1])
		}
		return rdflibgo.NewLiteral(u.Value() + "=" + lit.Lexical()), nil
	})

	res, err := Query(extFuncGraph(t), `PREFIX ex: <http://example.org/>
		PREFIX fn: <http://example.org/fn#>
		SELECT ?d WHERE {
			?s ex:name ?n .
			FILTER(?n = "bob")
			BIND(fn:describe(?s, ?n) AS ?d)
		}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("want 1 row, got %d", len(res.Bindings))
	}
	if got := res.Bindings[0]["d"].(rdflibgo.Literal).Lexical(); got != "http://example.org/b=bob" {
		t.Fatalf("unexpected result: %s", got)
	}
}

// An unregistered IRI call must not match a built-in by accident and must not
// crash; it simply evaluates to unbound.
func TestUnregisteredExtensionFunctionIsUnbound(t *testing.T) {
	res, err := Query(extFuncGraph(t), `PREFIX ex: <http://example.org/>
		SELECT ?s ?x WHERE {
			?s ex:value ?v .
			BIND(<http://example.org/fn#missing>(?v) AS ?x)
		}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 2 {
		t.Fatalf("want 2 rows, got %d", len(res.Bindings))
	}
	for _, b := range res.Bindings {
		if v, ok := b["x"]; ok && v != nil {
			t.Fatalf("want ?x unbound, got %v", v)
		}
	}
}

// A bare IRI in an expression must keep working — the ArgList is optional.
func TestBareIRIInExpressionStillWorks(t *testing.T) {
	res, err := Query(extFuncGraph(t), `PREFIX ex: <http://example.org/>
		SELECT ?s WHERE {
			?s ex:value ?v .
			FILTER(?s = <http://example.org/a>)
		}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("want 1 row, got %d: %v", len(res.Bindings), res.Bindings)
	}
}

func TestRegisterFunctionValidation(t *testing.T) {
	const iri = "http://example.org/fn#dup"
	registerForTest(t, iri, doubleFn)

	if err := RegisterFunction(iri, doubleFn); !errors.Is(err, ErrFunctionRegistered) {
		t.Fatalf("want ErrFunctionRegistered, got %v", err)
	}
	for _, bad := range []string{"", "relative/path", "http://example.org/a b", "urn:x<y"} {
		if err := RegisterFunction(bad, doubleFn); !errors.Is(err, ErrInvalidFunctionIRI) {
			t.Errorf("RegisterFunction(%q): want ErrInvalidFunctionIRI, got %v", bad, err)
		}
	}
	if err := RegisterFunction("http://example.org/fn#nil", nil); !errors.Is(err, ErrInvalidFunctionIRI) {
		t.Errorf("nil function: want ErrInvalidFunctionIRI, got %v", err)
	}
}

func TestReplaceAndUnregisterFunction(t *testing.T) {
	const iri = "http://example.org/fn#replaceme"
	t.Cleanup(func() { UnregisterFunction(iri) })

	replaced, err := ReplaceFunction(iri, doubleFn)
	if err != nil {
		t.Fatal(err)
	}
	if replaced {
		t.Error("first ReplaceFunction reported a replacement")
	}
	replaced, err = ReplaceFunction(iri, doubleFn)
	if err != nil {
		t.Fatal(err)
	}
	if !replaced {
		t.Error("second ReplaceFunction did not report a replacement")
	}

	if _, ok := LookupFunction(iri); !ok {
		t.Error("LookupFunction did not find the registered function")
	}
	var found bool
	for _, got := range RegisteredFunctions() {
		if got == iri {
			found = true
		}
	}
	if !found {
		t.Error("RegisteredFunctions did not list the registered function")
	}

	if !UnregisterFunction(iri) {
		t.Error("UnregisterFunction did not report removing the function")
	}
	if UnregisterFunction(iri) {
		t.Error("UnregisterFunction reported removing an absent function")
	}
}

func TestMustRegisterFunctionPanicsOnDuplicate(t *testing.T) {
	const iri = "http://example.org/fn#must"
	MustRegisterFunction(iri, doubleFn)
	t.Cleanup(func() { UnregisterFunction(iri) })

	defer func() {
		if recover() == nil {
			t.Error("MustRegisterFunction did not panic on a duplicate IRI")
		}
	}()
	MustRegisterFunction(iri, doubleFn)
}

// The registry is documented as safe for concurrent use.
func TestExtensionFunctionRegistryConcurrent(t *testing.T) {
	registerForTest(t, "http://example.org/fn#double", doubleFn)
	g := extFuncGraph(t)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			iri := fmt.Sprintf("http://example.org/fn#tmp%d", i)
			if err := RegisterFunction(iri, doubleFn); err != nil {
				t.Error(err)
				return
			}
			defer UnregisterFunction(iri)
			_ = RegisteredFunctions()
			if _, err := Query(g, `PREFIX ex: <http://example.org/>
				PREFIX fn: <http://example.org/fn#>
				SELECT ?d WHERE { ?s ex:value ?v . BIND(fn:double(?v) AS ?d) }`); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
}
