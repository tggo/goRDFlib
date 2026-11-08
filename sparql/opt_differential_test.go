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
// ex:root with a cycle, a class hierarchy, optional values, triple terms and
// a blank node.
func optGraph(t testing.TB) *graph.Graph {
	var sb strings.Builder
	sb.WriteString(`@prefix ex: <http://example.org/> .
ex:Kind0 ex:sub ex:Base . ex:Kind1 ex:sub ex:Base . ex:Base ex:sub ex:Top .
ex:Kind0 ex:label "zero" . ex:Kind1 ex:label "one" . ex:Kind2 ex:label "two" .
ex:q9_0 ex:part ex:p9 .
ex:p2 ex:note [ ex:text "a note" ] .
ex:p1 ex:a ex:m1 , ex:m2 . ex:m1 ex:b ex:v . ex:m2 ex:b ex:v .
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
	switch o.r.IntN(12) {
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
	}
	pred := o.pick("ex:part", "ex:type", "ex:opt", "ex:sub", "ex:label", "ex:part", "ex:type")
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
		kind := o.r.IntN(14)
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
// planning) are shortcuts: with them off the answers must be the same
// multisets. Random queries with joins, OPTIONAL, MINUS, UNION, EXISTS,
// subqueries, BIND, VALUES, paths, triple terms and blank node labels are
// evaluated four ways, from a fixed seed:
//
//	A  ?this pre-bound, optimisations on     B  ?this pre-bound, off
//	C  ?this written into the text, on       D  written into the text, off
//
// A must equal B and C must equal D. A must equal C too unless the plain
// engine's own two paths already disagree (B != D), which is not something an
// optimisation did.
func TestOptimisationsDoNotChangeAnswers(t *testing.T) {
	g := optGraph(t)
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	defer setOpts(false)
	n := 600
	if testing.Short() {
		n = 150
	}
	var compared, nonEmpty, pathDiffer int
	for i := range n {
		gen := &optGen{r: rand.New(rand.NewPCG(20260101, uint64(i)))}
		q := gen.query()
		pq, err := Parse(q)
		if err != nil {
			continue
		}
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
		if slices.Equal(b, d) && !slices.Equal(a, c) {
			t.Errorf("query %d: the optimisations make the pre-bound and textual forms disagree\n%s\npre-bound (%d): %v\ntextual   (%d): %v", i, q, len(a), head(a), len(c), head(c))
		}
		if !slices.Equal(b, d) {
			t.Logf("plain engine, query %d: pre-bound (%d rows) vs textual (%d rows)\n%s", i, len(b), len(d), q)
			pathDiffer++
		}
	}
	t.Logf("compared %d random queries (%d with rows); the plain engine's pre-bound and textual forms differ on %d", compared, nonEmpty, pathDiffer)
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
