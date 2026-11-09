package sparql

import (
	"slices"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// The right side of a pre-bound join gets the left row substituted only when
// it is a BGP, or a UNION or join of BGPs. Everything else must not see the
// row: a variable bound only on the left is out of scope in a sibling group
// (SPARQL 1.1 §18.2.1, the bind10 rule). Each query puts ?p on the left only
// and uses it on the right in a place that would behave differently if it
// were visible. The rows must equal the same query with ?this written into
// the text (the plain path), and equal the rows with the substitution off.
func TestPreBoundJoinRightSideScoping(t *testing.T) {
	g := preboundGraph(t)
	if err := turtle.Parse(t.Context(), g, strings.NewReader(`@prefix ex: <http://example.org/> .
ex:r0 ex:rt <<( ex:p0 ex:type ex:Kind0 )>> . ex:r1 ex:rt <<( ex:p1 ex:type ex:Kind1 )>> .
ex:r2 ex:rt <<( ex:p2 ex:type ex:Kind2 )>> . ex:r3 ex:rt <<( ex:p9 ex:type ex:Kind0 )>> .
ex:r4 ex:rt <<( ex:p4 ex:type <<( ex:p4 ex:x ex:y )>> )>> .
`)); err != nil {
		t.Fatal(err)
	}
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	defer setOpts(false)
	nonEmpty := 0
	for _, c := range []struct{ name, query string }{
		// expressions on the right that name ?p: ?p is unbound there
		{"bind", `SELECT ?p ?q WHERE { ?this ex:part ?p { BIND (?p AS ?q) } }`},
		{"bind with a coalesce", `SELECT ?p ?q WHERE { ?this ex:part ?p { ?x ex:type ex:Kind1 BIND (COALESCE(?p, ex:none) AS ?q) } }`},
		{"filter equals", `SELECT ?p ?x WHERE { ?this ex:part ?p { ?x ex:type ex:Kind1 FILTER (?x = ?p) } }`},
		{"filter not bound", `SELECT ?p ?x WHERE { ?this ex:part ?p { ?x ex:type ex:Kind1 FILTER (!BOUND(?p)) } }`},
		{"filter over a shared variable", `SELECT ?p ?k WHERE { ?this ex:part ?p { ?p ex:type ?k FILTER (?k != ex:Extra) } }`},
		{"optional with a filter on the left variable", `SELECT ?p ?k ?l WHERE { ?this ex:part ?p { ?p ex:type ?k OPTIONAL { ?k ex:label ?l FILTER (?p != ex:p0) } } }`},
		{"optional whose filter tests ?p is not bound", `SELECT ?p ?x ?l WHERE { ?this ex:part ?p { ?x ex:type ex:Kind1 OPTIONAL { ?x ex:opt ?l FILTER (!BOUND(?p)) } } }`},
		{"minus on a left variable", `SELECT ?p ?k WHERE { ?this ex:part ?p { ?p ex:type ?k MINUS { ?p ex:type ex:Extra } } }`},
		{"minus with a filter on the left variable", `SELECT ?p ?x WHERE { ?this ex:part ?p { ?x ex:type ex:Kind1 MINUS { ?x ex:opt ?o FILTER (?p != ex:p0) } } }`},
		{"subquery not projecting ?p", `SELECT ?p ?k WHERE { ?this ex:part ?p { SELECT ?k WHERE { ?q ex:type ?k FILTER (?q = ?p) } } }`},
		{"subquery projecting ?p", `SELECT ?p ?k WHERE { ?this ex:part ?p { SELECT ?p ?k WHERE { ?p ex:type ?k } } }`},
		{"subquery aggregate over ?p", `SELECT ?p ?n WHERE { ?this ex:part ?p { SELECT (COUNT(?p) AS ?n) WHERE { ?p ex:type ex:Kind0 } } }`},
		{"exists naming the left variable", `SELECT ?p ?x WHERE { ?this ex:part ?p { ?x ex:type ex:Kind1 FILTER EXISTS { ?x ex:opt ?p } } }`},
		{"not exists naming the left variable", `SELECT ?p ?x WHERE { ?this ex:part ?p { ?x ex:type ex:Kind1 FILTER NOT EXISTS { ?x ex:type ?p } } }`},
		{"exists with a nested join", `SELECT ?p ?x WHERE { ?this ex:part ?p { ?x ex:type ?k FILTER EXISTS { ?x ex:type ?k FILTER (?k != ex:Extra) ?x ex:opt ?o } } }`},
		{"union with a filter in a branch", `SELECT ?p ?x WHERE { ?this ex:part ?p { ?x ex:type ex:Kind1 FILTER (?x = ?p) } UNION { ?p ex:type ?x } }`},
		{"values on the left variable", `SELECT ?p ?v WHERE { ?this ex:part ?p VALUES ?v { ex:p1 ex:p5 } }`},
		{"values sharing the left variable", `SELECT ?p WHERE { ?this ex:part ?p { VALUES ?p { ex:p1 ex:p5 ex:none } } }`},
		// BGPs the row is substituted into
		{"path endpoint, sequence", `SELECT ?p ?v WHERE { ?this ex:part ?p FILTER (?p = ex:p1) ?p ex:a/ex:b ?v }`},
		{"path endpoint, inverse", `SELECT ?p ?v WHERE { ?this ex:part ?p FILTER (?p = ex:p1) ?v ^(ex:a/ex:b) ?p }`},
		{"path endpoint, transitive", `SELECT ?p ?x WHERE { ?this ex:part ?p FILTER (?p = ex:p1) ?p ex:a+ ?x }`},
		{"path both ends on the left", `SELECT ?p ?q WHERE { ?this ex:part ?p . ?this ex:part ?q FILTER (?p != ?q) ?p ex:type/^ex:type ?q }`},
		{"left variable only inside a triple term", `SELECT ?p ?r WHERE { ?this ex:part ?p FILTER (?p != ex:p3) ?r ex:rt <<( ?p ex:type ?k )>> }`},
		{"left variable only inside a nested triple term", `SELECT ?p ?r WHERE { ?this ex:part ?p FILTER (?p != ex:p3) ?r ex:rt <<( ?p ex:type <<( ?p ex:x ex:y )>> )>> }`},
		{"left variable in a triple term and outside", `SELECT ?p ?r ?k WHERE { ?this ex:part ?p FILTER (?p != ex:p3) ?r ex:rt <<( ?p ex:type ?k )>> . ?p ex:type ?k }`},
		{"left variable as the object inside a triple term", `SELECT ?k ?r WHERE { ?this ex:part ?p . ?p ex:type ?k FILTER (?k != ex:Extra) ?r ex:rt <<( ?x ex:type ?k )>> }`},
		{"union of bgps naming the variable differently", `SELECT ?p ?z WHERE { ?this ex:part ?p FILTER (?p != ex:p0) { ?p ex:opt ?z } UNION { ?z ex:type ?p } }`},
		{"join of bgps, one shares the left variable", `SELECT ?p ?k ?l WHERE { ?this ex:part ?p FILTER (?p != ex:p0) { ?p ex:type ?k } { ?k ex:label ?l } }`},
		{"blank node label in the right side", `SELECT ?p ?s WHERE { ?this ex:part ?p FILTER (?p = ex:p2) { ?p ex:note _:n . _:n ex:text ?s } }`},
		{"a predicate variable on the left", `SELECT ?p ?q ?o WHERE { ex:p4 ?p ?o1 FILTER (?p = ex:type) ?q ?p ?o }`},
	} {
		t.Run(c.name, func(t *testing.T) {
			setOpts(false)
			want := preboundRows(t, g, strings.ReplaceAll(c.query, "?this", "ex:root"), nil)
			on := preboundRows(t, g, c.query, root)
			setOpts(true)
			off := preboundRows(t, g, c.query, root)
			setOpts(false)
			if len(want) > 0 {
				nonEmpty++
			}
			if !slices.Equal(on, want) {
				t.Errorf("pre-bound rows differ from the textual form:\n%v\n%v", on, want)
			}
			if !slices.Equal(on, off) {
				t.Errorf("rows differ with the optimisations off:\n%v\n%v", on, off)
			}
		})
	}
	if nonEmpty < 20 {
		t.Errorf("only %d queries have rows: most of the table checks nothing", nonEmpty)
	}
}

// Pre-bound values reach expressions on the right as constants, the left row
// does not: ?this = ex:root holds in a right-hand FILTER, ?p = ... does not.
func TestPreBoundJoinConstantsStillReachTheRightSide(t *testing.T) {
	g := preboundGraph(t)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	got := preboundRows(t, g, `SELECT ?p ?k WHERE { ?this ex:part ?p { ?p ex:type ?k FILTER (?this = ex:root) } }`, root)
	if len(got) == 0 {
		t.Fatal("a pre-bound constant is not visible to the right side")
	}
	got = preboundRows(t, g, `SELECT ?p ?k WHERE { ?this ex:part ?p { ?k ex:label ?l FILTER (BOUND(?p)) } }`, root)
	if len(got) != 0 {
		t.Errorf("the left row is visible to a right-hand FILTER: %v", got)
	}
}

// The substitution is decided from the variables of the right side, found
// wherever they stand. The list is built once per evaluation and the check
// allocates nothing.
func TestRightSideVariables(t *testing.T) {
	parse := func(q string) Pattern {
		pq, err := Parse(exPrefix + q)
		if err != nil {
			t.Fatal(err)
		}
		return pq.Where
	}
	for _, c := range []struct {
		where string
		want  []string
	}{
		{`SELECT * WHERE { ?a ex:p ?b }`, []string{"a", "b"}},
		{`SELECT * WHERE { ?a ?p ?a }`, []string{"a", "p"}},
		{`SELECT * WHERE { ?a ex:p+ ?b }`, []string{"a", "b"}},
		{`SELECT * WHERE { ?r ex:rt <<( ?a ex:type ?k )>> }`, []string{"r", "a", "k"}},
		{`SELECT * WHERE { ?r ex:rt <<( ex:s ex:type <<( ?z ex:x ?w )>> )>> }`, []string{"r", "z", "w"}},
		{`SELECT * WHERE { { ?a ex:p ?b } UNION { ?c ex:p ?a } }`, []string{"a", "b", "c"}},
		{`SELECT * WHERE { ?a ex:p ?b FILTER (?a != ?b) }`, nil}, // not expression-free
	} {
		pq, err := Parse(exPrefix + c.where)
		if err != nil {
			t.Fatal(err)
		}
		p := pq.Where
		ec := newEvalCtx(t.Context())
		rs := ec.rightSideOf(p)
		if len(c.want) == 0 {
			if rs.free {
				t.Errorf("%s: reported expression-free", c.where)
			}
			continue
		}
		got := slices.Clone(rs.vars)
		slices.Sort(got)
		want := slices.Clone(c.want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: variables %v, want %v", c.where, got, want)
		}
		if ec.rightSideOf(p) != rs {
			t.Errorf("%s: the right side was analysed again", c.where)
		}
	}

	// A row binding one of the variables is found; one that does not, or
	// whose only match is pre-bound, is not; neither allocates.
	rightP := parse(`SELECT * WHERE { ?r ex:rt <<( ?a ex:type ?k )>> }`)
	ec := newEvalCtx(t.Context())
	rs := ec.rightSideOf(rightP)
	row := func(kv ...string) map[string]term.Term {
		m := map[string]term.Term{}
		for _, k := range kv {
			m[k] = term.NewURIRefUnsafe("http://example.org/" + k)
		}
		return m
	}
	for _, c := range []struct {
		name string
		left []map[string]term.Term
		pre  map[string]term.Term
		want bool
	}{
		{"binds one inside a triple term", []map[string]term.Term{row("a")}, nil, true},
		{"binds an unrelated one", []map[string]term.Term{row("z")}, nil, false},
		{"pre-bound already", []map[string]term.Term{row("a")}, row("a"), false},
		{"second row binds it", []map[string]term.Term{row("z"), row("z", "k")}, nil, true},
		{"no rows", nil, nil, false},
	} {
		if got := rs.bindsAVariableOf(c.left, c.pre); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	left, pre := []map[string]term.Term{row("a", "z")}, row("q")
	if allocs := testing.AllocsPerRun(100, func() { rs.bindsAVariableOf(left, pre) }); allocs != 0 {
		t.Errorf("%v allocations per check, want 0", allocs)
	}
	// more variables than are scanned: the set is used
	big := ec.rightSideOf(parse(`SELECT * WHERE { ?a ex:p ?b . ?c ex:p ?d . ?e ex:p ?f . ?g ex:p ?h . ?i ex:p ?j }`))
	if big.set == nil {
		t.Error("a right side with many variables has no set")
	}
	if !big.bindsAVariableOf([]map[string]term.Term{row("j")}, nil) || big.bindsAVariableOf([]map[string]term.Term{row("z")}, nil) {
		t.Error("the set lookup is wrong")
	}
	if allocs := testing.AllocsPerRun(100, func() { big.bindsAVariableOf(left, pre) }); allocs != 0 {
		t.Errorf("%v allocations per check with a set, want 0", allocs)
	}
}
