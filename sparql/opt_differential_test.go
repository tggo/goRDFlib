package sparql

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// optGraph is the data of the differential tests: a small tree under
// ex:root with a cycle, a class hierarchy, optional values, triple terms, a
// blank node, and ex:in links from nodes to graph names, all but one of them
// in optNamedGraphs.
func optGraph(t testing.TB) *graph.Graph {
	var sb strings.Builder
	sb.WriteString(`@prefix ex: <http://example.org/> .
ex:Kind0 ex:sub ex:Base . ex:Kind1 ex:sub ex:Base . ex:Base ex:sub ex:Top .
ex:Kind0 ex:label "zero" . ex:Kind1 ex:label "one" . ex:Kind2 ex:label "two" .
ex:q9_0 ex:part ex:p9 .
ex:p2 ex:note [ ex:text "a note" ] .
ex:p1 ex:a ex:m1 , ex:m2 . ex:m1 ex:b ex:v . ex:m2 ex:b ex:v .
ex:p1 ex:in ex:g1 . ex:q1_0 ex:in ex:g1 . ex:p2 ex:in ex:g2 . ex:p4 ex:in ex:g9 .
`)
	for i := range 10 {
		fmt.Fprintf(&sb, "ex:root ex:part ex:p%d . ex:p%d ex:type ex:Kind%d .\n", i, i, i%3)
		for j := range 2 {
			fmt.Fprintf(&sb, "ex:p%d ex:part ex:q%d_%d . ex:q%d_%d ex:type ex:Kind%d .\n", i, i, j, i, j, (i+j)%3)
		}
		if i%2 == 0 {
			fmt.Fprintf(&sb, "ex:p%d ex:opt %d .\n", i, i)
		}
		if i%4 == 0 {
			fmt.Fprintf(&sb, "ex:p%d ex:type ex:Extra .\n", i)
		}
		if i%3 != 2 {
			fmt.Fprintf(&sb, "ex:r%d ex:rt <<( ex:p%d ex:type ex:Kind%d )>> .\n", i, i, i%3)
		}
		if i%2 == 1 {
			fmt.Fprintf(&sb, "ex:p%d ex:ann <<( ex:p%d ex:type ex:Kind%d )>> .\n", i, i, i%3)
		}
	}
	g := graph.NewGraph()
	if err := turtle.Parse(t.Context(), g, strings.NewReader(sb.String())); err != nil {
		t.Fatal(err)
	}
	return g
}

// optNamedGraphs are the named graphs of the differential tests: two that
// restate and extend parts of optGraph, and an empty one.
var optNamedGraphs = func() map[string]*graph.Graph {
	named := map[string]string{
		"http://example.org/g1": `ex:p1 ex:part ex:q1_0 . ex:q1_0 ex:type ex:Kind1 . ex:p4 ex:opt 4 . ex:p1 ex:in ex:g2 .`,
		"http://example.org/g2": `ex:p1 ex:type ex:Kind0 . ex:q1_0 ex:part ex:p1 . ex:p2 ex:label "two" . ex:Kind0 ex:sub ex:Top .`,
		"http://example.org/g3": ``,
	}
	graphs := make(map[string]*graph.Graph, len(named))
	for name, data := range named {
		g := graph.NewGraph()
		if err := turtle.Parse(context.Background(), g, strings.NewReader("@prefix ex: <http://example.org/> .\n"+data)); err != nil {
			panic(err)
		}
		graphs[name] = g
	}
	return graphs
}()

// optGen builds random group patterns from a fixed vocabulary. Every query it
// writes is deterministic (no LIMIT without ORDER BY, no RAND), so two
// evaluations of one query must return the same rows.
type optGen struct {
	r     *rand.Rand
	depth int
}

func (o *optGen) pick(xs ...string) string { return xs[o.r.IntN(len(xs))] }

func (o *optGen) v() string { return o.pick("?a", "?b", "?c", "?d") }

// node is a variable most of the time, sometimes a constant.
func (o *optGen) node() string {
	if o.r.IntN(5) == 0 {
		return o.pick("ex:p1", "ex:p2", "ex:p4", "ex:Kind0", "ex:q1_0", "ex:Top")
	}
	return o.v()
}

func (o *optGen) triple() string {
	switch o.r.IntN(17) {
	case 0:
		return fmt.Sprintf("%s ex:part+ %s .", o.pick("?this", o.v()), o.node())
	case 1:
		return fmt.Sprintf("%s ex:type/ex:sub* %s .", o.v(), o.pick("ex:Top", "ex:Base", "?c"))
	case 2:
		return fmt.Sprintf("%s ^ex:part %s .", o.v(), o.v())
	case 3:
		return fmt.Sprintf("%s ex:a/ex:b %s .", o.v(), o.v())
	case 4:
		return fmt.Sprintf("%s ex:rt <<( %s ex:type %s )>> .", o.v(), o.pick("?a", "?b", "ex:p1", "ex:p3"), o.pick("?c", "?d", "ex:Kind0"))
	case 5:
		return fmt.Sprintf("%s ex:ann <<( %s ex:type %s )>> .", o.v(), o.v(), o.pick("?c", "ex:Kind1"))
	case 6:
		return fmt.Sprintf("%s ex:ann <<( %s ex:type <<( %s ex:q ex:z )>> )>> .", o.v(), o.v(), o.v())
	case 7:
		return fmt.Sprintf("_:n%d ex:text %s .", o.r.IntN(2), o.v())
	case 8:
		return fmt.Sprintf("%s ex:note _:n%d .", o.v(), o.r.IntN(2))
	// Paths the planner probes: a constant or pre-bound subject, a bound
	// object only, a literal subject (never probed), a repeated variable.
	case 9:
		return fmt.Sprintf("%s ex:part+ %s .", o.pick("ex:root", "?this", "ex:p1"), o.v())
	case 10:
		return fmt.Sprintf("%s ex:part/ex:type %s .", o.v(), o.pick("ex:Kind0", "ex:Kind2", "?this"))
	case 11:
		return fmt.Sprintf("%s ^ex:part* %s .", o.pick("ex:q1_0", "?this", "ex:p4"), o.v())
	case 12:
		return fmt.Sprintf("%s ^ex:label/ex:sub? %s .", o.pick(`"zero"`, `"two"`, "1"), o.v())
	case 13:
		x := o.v()
		return fmt.Sprintf("%s ex:part* %s .", x, x)
	}
	pred := o.pick("ex:part", "ex:type", "ex:opt", "ex:sub", "ex:label", "ex:part", "ex:type", "ex:in")
	return fmt.Sprintf("%s %s %s .", o.pick("?this", o.v(), o.v()), pred, o.node())
}

func (o *optGen) expr() string {
	switch o.r.IntN(7) {
	case 0:
		return fmt.Sprintf("%s != %s", o.v(), o.v())
	case 1:
		return fmt.Sprintf("BOUND(%s)", o.v())
	case 2:
		return fmt.Sprintf("!BOUND(%s)", o.v())
	case 3:
		return fmt.Sprintf("%s = %s", o.v(), o.pick("ex:p1", "ex:Kind0", "ex:p4"))
	case 4:
		return fmt.Sprintf("STR(%s) < STR(%s)", o.v(), o.v())
	case 5:
		return fmt.Sprintf("%s = ?this", o.v())
	}
	return fmt.Sprintf("%s IN (ex:p1, ex:p2, ex:Kind1)", o.v())
}

// group writes the elements of a group graph pattern, without braces.
func (o *optGen) group() string {
	o.depth++
	defer func() { o.depth-- }()
	var sb strings.Builder
	n := 1 + o.r.IntN(4)
	for range n {
		kind := o.r.IntN(16)
		if o.depth > 3 && kind > 4 {
			kind = 0
		}
		switch kind {
		case 0, 1, 2, 3, 4:
			sb.WriteString(o.triple() + " ")
		case 5:
			sb.WriteString("FILTER (" + o.expr() + ") ")
		case 6:
			sb.WriteString("OPTIONAL { " + o.group() + "} ")
		case 7:
			sb.WriteString("MINUS { " + o.group() + "} ")
		case 8:
			sb.WriteString("{ " + o.group() + "} UNION { " + o.group() + "} ")
		case 9:
			sb.WriteString("{ " + o.group() + "} ")
		case 10:
			sb.WriteString(fmt.Sprintf("BIND (%s AS %s) ", o.pick("1", "?a", "STR(?b)", "COALESCE(?c, 0)"), o.pick("?d", "?e")))
		case 11:
			sb.WriteString(o.pick("FILTER EXISTS ", "FILTER NOT EXISTS ") + "{ " + o.group() + "} ")
		case 12:
			sb.WriteString(fmt.Sprintf("{ SELECT DISTINCT %s %s WHERE { %s} } ", o.v(), o.pick("?b", "?c"), o.group()))
		case 13:
			sb.WriteString(fmt.Sprintf("VALUES %s { ex:p1 ex:p2 ex:Kind0 ex:nothing } ", o.v()))
		case 14:
			// A service sees no initial bindings (it is another endpoint),
			// so its group must not mention ?this: the textual form would
			// substitute inside it and the pre-bound form could not.
			inner := strings.ReplaceAll(o.group(), "?this", "?a")
			sb.WriteString(o.pick("SERVICE ex:svc ", "SERVICE SILENT ex:svc ") + "{ " + inner + "} ")
		case 15:
			// A variable graph name is often bound by an outer ex:in, and
			// ?this is never a graph.
			sb.WriteString("GRAPH " + o.pick("?a", "?b", "?g", "?this", "ex:g1", "ex:g2", "ex:nothing") + " { " + o.group() + "} ")
		}
	}
	return sb.String()
}

func (o *optGen) query() string {
	proj := o.pick("*", "*", "?a ?b", "?a ?c ?d")
	return exPrefix + fmt.Sprintf("SELECT %s WHERE { %s}", proj, o.group())
}

// optRows evaluates q and returns its rows sorted (the multiset), or the
// error text. budget is the solution budget; exceeding it is reported as
// skipped, since the optimised and plain evaluations build different numbers
// of intermediate solutions.
func optRows(g *graph.Graph, pq *ParsedQuery, pre map[string]term.Term) (rows []string, skipped bool, err error) {
	ctx := WithMaxSolutions(context.Background(), 60_000)
	// ex:svc is a second endpoint serving the same data.
	ctx = WithServiceHandler(ctx, func(ctx context.Context, c *ServiceCall) ([]map[string]term.Term, error) {
		res, err := EvalQueryContext(ctx, g, c.Select(), nil)
		if err != nil {
			return nil, err
		}
		return res.Bindings, nil
	})
	pq.NamedGraphs = optNamedGraphs
	res, err := EvalQueryContext(ctx, g, pq, pre)
	if errors.Is(err, ErrTooManySolutions) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return renderRows(res.Bindings), false, nil
}

// The optimisations (merging a join of BGPs, splitting a BGP into groups,
// substituting the left row into a pre-bound join, probing paths when
// planning, evaluating a join's right side once or with the left row
// substituted) are shortcuts: with them off the answers must be the same
// multisets. Random queries with joins, OPTIONAL, MINUS, UNION, EXISTS,
// subqueries, BIND, VALUES, paths, triple terms and blank node labels are
// evaluated four ways, from a fixed seed:
//
//	A  ?this pre-bound, optimisations on     B  ?this pre-bound, off
//	C  ?this written into the text, on       D  written into the text, off
//
// All four must be equal: A = B and C = D (the optimisations change nothing)
// and B = D (pre-binding is substitution, §5.3.1 of SHACL, with
// PreBindSubqueries).
func TestOptimisationsDoNotChangeAnswers(t *testing.T) {
	g := optGraph(t)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	defer setOpts(false)
	probes, joins, optionals, preBoundOptionals, graphs := 0, 0, 0, 0, 0
	pathProbeHook = func() { probes++ }
	joinRightSideHook = func() { joins++ }
	optionalRightSideHook = func(preBound bool) {
		optionals++
		if preBound {
			preBoundOptionals++
		}
	}
	graphPreBoundHook = func() { graphs++ }
	defer func() {
		pathProbeHook, joinRightSideHook, optionalRightSideHook, graphPreBoundHook = nil, nil, nil, nil
	}()
	n := 600
	if testing.Short() {
		n = 150
	}
	var compared, nonEmpty int
	for i := range n {
		gen := &optGen{r: rand.New(rand.NewPCG(20260101, uint64(i)))}
		q := gen.query()
		pq, err := Parse(q)
		if err != nil {
			continue
		}
		// The textual form substitutes ?this everywhere, subqueries that do
		// not project it included; that is PreBindSubqueries. Without it a
		// subquery's ?this is another variable (§18.2.1), and differs.
		pq.PreBindSubqueries = true
		textual := strings.ReplaceAll(q, "?this", "ex:root")
		pqT, err := Parse(textual)
		if err != nil {
			continue
		}
		setOpts(false)
		a, skA, errA := optRows(g, pq, root)
		c, skC, errC := optRows(g, pqT, nil)
		setOpts(true)
		b, skB, errB := optRows(g, pq, root)
		d, skD, errD := optRows(g, pqT, nil)
		setOpts(false)
		if skA || skB || skC || skD {
			continue
		}
		compared++
		if len(a) > 0 {
			nonEmpty++
		}
		if (errA == nil) != (errB == nil) || (errC == nil) != (errD == nil) {
			t.Errorf("query %d: errors differ between optimised and plain: %v / %v, %v / %v\n%s", i, errA, errB, errC, errD, q)
			continue
		}
		if errA != nil || errC != nil {
			continue
		}
		if !slices.Equal(a, b) {
			t.Errorf("query %d: pre-bound rows differ with the optimisations off\n%s\non  (%d): %v\noff (%d): %v", i, q, len(a), head(a), len(b), head(b))
		}
		if !slices.Equal(c, d) {
			t.Errorf("query %d: textual rows differ with the optimisations off\n%s\non  (%d): %v\noff (%d): %v", i, textual, len(c), head(c), len(d), head(d))
		}
		if !slices.Equal(a, c) {
			t.Errorf("query %d: the optimisations make the pre-bound and textual forms disagree\n%s\npre-bound (%d): %v\ntextual   (%d): %v", i, q, len(a), head(a), len(c), head(c))
		}
		if !slices.Equal(b, d) {
			t.Errorf("plain engine, query %d: pre-bound (%d rows) vs textual (%d rows)\n%s", i, len(b), len(d), q)
		}
	}
	t.Logf("compared %d random queries (%d with rows), %d path probes, %d joins with their right side evaluated once or substituted (%d of them OPTIONALs, %d pre-bound), %d pre-bound GRAPH patterns", compared, nonEmpty, probes, joins, optionals, preBoundOptionals, graphs)
	if probes < n/10 {
		t.Errorf("only %d path probes ran: the generator no longer exercises them", probes)
	}
	if joins < n/10 {
		t.Errorf("only %d joins had their right side evaluated once or substituted: the generator no longer exercises them", joins)
	}
	if optionals < n/20 {
		t.Errorf("only %d OPTIONALs had their right side evaluated once or substituted: the generator no longer exercises them", optionals)
	}
	if preBoundOptionals < n/40 {
		t.Errorf("only %d pre-bound OPTIONALs had their right side evaluated once or substituted: the generator no longer exercises them", preBoundOptionals)
	}
	if graphs < n/20 {
		t.Errorf("only %d GRAPH patterns were evaluated on the pre-bound path: the generator no longer exercises it", graphs)
	}
	if compared < n/3 || nonEmpty < compared/5 {
		t.Errorf("only %d of %d queries compared, %d with rows: the generator is too weak", compared, n, nonEmpty)
	}
}

func head(rows []string) []string {
	if len(rows) > 6 {
		return rows[:6]
	}
	return rows
}

// FuzzOptimisationsAgree: for any query text that parses, the optimised and
// plain evaluations agree, pre-bound or written into the text. The seeds are
// the generator's queries.
func FuzzOptimisationsAgree(f *testing.F) {
	for i := range 40 {
		gen := &optGen{r: rand.New(rand.NewPCG(7, uint64(i)))}
		f.Add(gen.query())
	}
	g := optGraph(f)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	f.Fuzz(func(t *testing.T, q string) {
		pq, err := Parse(q)
		if err != nil || pq.Type != "SELECT" && pq.Type != "" && pq.Type != "ASK" {
			return
		}
		defer setOpts(false)
		setOpts(false)
		a, skA, errA := optRows(g, pq, root)
		setOpts(true)
		b, skB, errB := optRows(g, pq, root)
		if skA || skB || errA != nil || errB != nil {
			return
		}
		if !slices.Equal(a, b) {
			t.Errorf("optimised and plain rows differ for\n%s\n%v\n%v", q, head(a), head(b))
		}
	})
}
