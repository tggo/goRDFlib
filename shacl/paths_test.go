package shacl

import (
	"sort"
	"testing"
)

const (
	ex = "http://example.org/"
)

// helper to load turtle or fail
func mustGraph(t *testing.T, ttl string) *Graph {
	t.Helper()
	g, err := LoadTurtleString(ttl, "http://example.org/")
	if err != nil {
		t.Fatalf("failed to parse turtle: %v", err)
	}
	return g
}

func mustGraphJsonld(t *testing.T, jsonld string) *Graph {
	t.Helper()
	g, err := LoadJsonLDString(jsonld, "http://example.org/")
	if err != nil {
		t.Fatalf("failed to parse jsonld: %v", err)
	}
	return g
}

// helper to extract sorted IRI values from terms
func termValues(terms []Term) []string {
	var vals []string
	for _, t := range terms {
		vals = append(vals, t.Value())
	}
	sort.Strings(vals)
	return vals
}

func assertTermValues(t *testing.T, got []Term, want []string) {
	t.Helper()
	gotVals := termValues(got)
	sort.Strings(want)
	if len(gotVals) != len(want) {
		t.Fatalf("got %d terms %v, want %d terms %v", len(gotVals), gotVals, len(want), want)
	}
	for i := range gotVals {
		if gotVals[i] != want[i] {
			t.Errorf("term[%d] = %q, want %q", i, gotVals[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// PathKind.String()
// ---------------------------------------------------------------------------

func TestPathKind_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind PathKind
		want string
	}{
		{PathPredicate, "predicate"},
		{PathInverse, "inverse"},
		{PathSequence, "sequence"},
		{PathAlternative, "alternative"},
		{PathZeroOrMore, "zeroOrMore"},
		{PathOneOrMore, "oneOrMore"},
		{PathZeroOrOne, "zeroOrOne"},
		{PathKind(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("PathKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// parsePath
// ---------------------------------------------------------------------------

func TestParsePath_Predicate(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `@prefix ex: <http://example.org/> . ex:a ex:p ex:b .`)
	node := IRI(ex + "p")
	path := parsePath(g, node)
	if path.Kind != PathPredicate {
		t.Fatalf("kind = %v, want PathPredicate", path.Kind)
	}
	if !path.Pred.Equal(node) {
		t.Fatalf("pred = %v, want %v", path.Pred, node)
	}
}

func TestParsePath_Inverse(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		_:p sh:inversePath ex:parent .
	`)
	// Find actual blank node from graph
	var bnode Term
	for _, tr := range g.Triples() {
		if tr.Predicate.Value() == SH+"inversePath" {
			bnode = tr.Subject
			break
		}
	}
	path := parsePath(g, bnode)
	if path.Kind != PathInverse {
		t.Fatalf("kind = %v, want PathInverse", path.Kind)
	}
	if path.Sub == nil || path.Sub.Kind != PathPredicate {
		t.Fatalf("sub = %v, want predicate sub-path", path.Sub)
	}
	if path.Sub.Pred.Value() != ex+"parent" {
		t.Fatalf("sub.Pred = %v, want ex:parent", path.Sub.Pred)
	}
}

func TestParsePath_Alternative(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		_:p sh:alternativePath ( ex:name ex:label ) .
	`)
	var bnode Term
	for _, tr := range g.Triples() {
		if tr.Predicate.Value() == SH+"alternativePath" {
			bnode = tr.Subject
			break
		}
	}
	path := parsePath(g, bnode)
	if path.Kind != PathAlternative {
		t.Fatalf("kind = %v, want PathAlternative", path.Kind)
	}
	if len(path.Elements) != 2 {
		t.Fatalf("len(elements) = %d, want 2", len(path.Elements))
	}
	if path.Elements[0].Pred.Value() != ex+"name" {
		t.Errorf("elements[0] = %v, want ex:name", path.Elements[0].Pred)
	}
	if path.Elements[1].Pred.Value() != ex+"label" {
		t.Errorf("elements[1] = %v, want ex:label", path.Elements[1].Pred)
	}
}

func TestParsePath_Sequence(t *testing.T) {
	t.Parallel()
	// A sequence path is an RDF list used directly as the path node
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:shape sh:path ( ex:knows ex:name ) .
	`)
	// Find the list head (object of sh:path)
	objs := g.Objects(IRI(ex+"shape"), IRI(SH+"path"))
	if len(objs) == 0 {
		t.Fatal("no sh:path found")
	}
	path := parsePath(g, objs[0])
	if path.Kind != PathSequence {
		t.Fatalf("kind = %v, want PathSequence", path.Kind)
	}
	if len(path.Elements) != 2 {
		t.Fatalf("len(elements) = %d, want 2", len(path.Elements))
	}
	if path.Elements[0].Pred.Value() != ex+"knows" {
		t.Errorf("elements[0] = %v, want ex:knows", path.Elements[0].Pred)
	}
	if path.Elements[1].Pred.Value() != ex+"name" {
		t.Errorf("elements[1] = %v, want ex:name", path.Elements[1].Pred)
	}
}

func TestParsePath_ZeroOrMore(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		_:p sh:zeroOrMorePath ex:knows .
	`)
	var bnode Term
	for _, tr := range g.Triples() {
		if tr.Predicate.Value() == SH+"zeroOrMorePath" {
			bnode = tr.Subject
			break
		}
	}
	path := parsePath(g, bnode)
	if path.Kind != PathZeroOrMore {
		t.Fatalf("kind = %v, want PathZeroOrMore", path.Kind)
	}
	if path.Sub == nil || path.Sub.Pred.Value() != ex+"knows" {
		t.Fatalf("sub = %v, want ex:knows predicate", path.Sub)
	}
}

func TestParsePath_OneOrMore(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		_:p sh:oneOrMorePath ex:knows .
	`)
	var bnode Term
	for _, tr := range g.Triples() {
		if tr.Predicate.Value() == SH+"oneOrMorePath" {
			bnode = tr.Subject
			break
		}
	}
	path := parsePath(g, bnode)
	if path.Kind != PathOneOrMore {
		t.Fatalf("kind = %v, want PathOneOrMore", path.Kind)
	}
	if path.Sub == nil || path.Sub.Pred.Value() != ex+"knows" {
		t.Fatalf("sub = %v, want ex:knows predicate", path.Sub)
	}
}

func TestParsePath_ZeroOrOne(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		_:p sh:zeroOrOnePath ex:knows .
	`)
	var bnode Term
	for _, tr := range g.Triples() {
		if tr.Predicate.Value() == SH+"zeroOrOnePath" {
			bnode = tr.Subject
			break
		}
	}
	path := parsePath(g, bnode)
	if path.Kind != PathZeroOrOne {
		t.Fatalf("kind = %v, want PathZeroOrOne", path.Kind)
	}
	if path.Sub == nil || path.Sub.Pred.Value() != ex+"knows" {
		t.Fatalf("sub = %v, want ex:knows predicate", path.Sub)
	}
}

// ---------------------------------------------------------------------------
// evalPath — predicate
// ---------------------------------------------------------------------------

func TestEvalPath_Predicate(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:alice ex:knows ex:bob .
		ex:alice ex:knows ex:carol .
		ex:bob ex:knows ex:dave .
	`)
	path := &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "knows")}
	got := evalPath(g, path, IRI(ex+"alice"))
	assertTermValues(t, got, []string{ex + "bob", ex + "carol"})
}

func TestEvalPath_Nil(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `@prefix ex: <http://example.org/> . ex:a ex:p ex:b .`)
	got := evalPath(g, nil, IRI(ex+"a"))
	if len(got) != 0 {
		t.Fatalf("expected nil result for nil path, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// evalPath — inverse
// ---------------------------------------------------------------------------

func TestEvalPath_InversePredicate(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:alice ex:parent ex:bob .
		ex:carol ex:parent ex:bob .
		ex:dave ex:parent ex:eve .
	`)
	path := &PropertyPath{
		Kind: PathInverse,
		Sub:  &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "parent")},
	}
	got := evalPath(g, path, IRI(ex+"bob"))
	assertTermValues(t, got, []string{ex + "alice", ex + "carol"})
}

func TestEvalInversePath_GeneralCase(t *testing.T) {
	t.Parallel()
	// Inverse of a sequence path: find nodes whose sequence path reaches focus
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:alice ex:knows ex:bob .
		ex:bob ex:name "Bob" .
		ex:carol ex:knows ex:dave .
		ex:dave ex:name "Dave" .
	`)
	seqPath := &PropertyPath{
		Kind: PathSequence,
		Elements: []*PropertyPath{
			{Kind: PathPredicate, Pred: IRI(ex + "knows")},
			{Kind: PathPredicate, Pred: IRI(ex + "name")},
		},
	}
	path := &PropertyPath{Kind: PathInverse, Sub: seqPath}
	got := evalPath(g, path, Literal("Bob", "", ""))
	assertTermValues(t, got, []string{ex + "alice"})
}

// ---------------------------------------------------------------------------
// evalPath — sequence
// ---------------------------------------------------------------------------

func TestEvalPath_Sequence(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:alice ex:knows ex:bob .
		ex:bob ex:name "Bob" .
	`)
	path := &PropertyPath{
		Kind: PathSequence,
		Elements: []*PropertyPath{
			{Kind: PathPredicate, Pred: IRI(ex + "knows")},
			{Kind: PathPredicate, Pred: IRI(ex + "name")},
		},
	}
	got := evalPath(g, path, IRI(ex+"alice"))
	if len(got) != 1 || got[0].Value() != "Bob" {
		t.Fatalf("got %v, want [\"Bob\"]", got)
	}
}

func TestEvalPath_Sequence_MultiStep(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:p ex:b .
		ex:a ex:p ex:c .
		ex:b ex:q ex:d .
		ex:c ex:q ex:d .
		ex:c ex:q ex:e .
	`)
	path := &PropertyPath{
		Kind: PathSequence,
		Elements: []*PropertyPath{
			{Kind: PathPredicate, Pred: IRI(ex + "p")},
			{Kind: PathPredicate, Pred: IRI(ex + "q")},
		},
	}
	got := evalPath(g, path, IRI(ex+"a"))
	// d reachable via both b and c, but deduped; e reachable via c
	assertTermValues(t, got, []string{ex + "d", ex + "e"})
}

// ---------------------------------------------------------------------------
// evalPath — alternative
// ---------------------------------------------------------------------------

func TestEvalPath_Alternative(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:alice ex:name "Alice" .
		ex:alice ex:label "A" .
	`)
	path := &PropertyPath{
		Kind: PathAlternative,
		Elements: []*PropertyPath{
			{Kind: PathPredicate, Pred: IRI(ex + "name")},
			{Kind: PathPredicate, Pred: IRI(ex + "label")},
		},
	}
	got := evalPath(g, path, IRI(ex+"alice"))
	if len(got) != 2 {
		t.Fatalf("got %d terms, want 2", len(got))
	}
	vals := termValues(got)
	if vals[0] != "A" || vals[1] != "Alice" {
		t.Errorf("got %v, want [A, Alice]", vals)
	}
}

func TestEvalPath_Alternative_Dedup(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:alice ex:name "Alice" .
		ex:alice ex:label "Alice" .
	`)
	path := &PropertyPath{
		Kind: PathAlternative,
		Elements: []*PropertyPath{
			{Kind: PathPredicate, Pred: IRI(ex + "name")},
			{Kind: PathPredicate, Pred: IRI(ex + "label")},
		},
	}
	got := evalPath(g, path, IRI(ex+"alice"))
	// Same literal "Alice" from both paths, should be deduped
	if len(got) != 1 {
		t.Fatalf("got %d terms, want 1 (deduped)", len(got))
	}
}

// ---------------------------------------------------------------------------
// transitiveClose (zeroOrMore, oneOrMore)
// ---------------------------------------------------------------------------

func TestTransitiveClose_ZeroOrMore(t *testing.T) {
	t.Parallel()
	// Chain: a -> b -> c -> d
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:knows ex:b .
		ex:b ex:knows ex:c .
		ex:c ex:knows ex:d .
	`)
	sub := &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "knows")}
	got := transitiveClose(g, sub, IRI(ex+"a"), true)
	// includeSelf=true -> a, b, c, d
	assertTermValues(t, got, []string{ex + "a", ex + "b", ex + "c", ex + "d"})
}

func TestTransitiveClose_OneOrMore(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:knows ex:b .
		ex:b ex:knows ex:c .
		ex:c ex:knows ex:d .
	`)
	sub := &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "knows")}
	got := transitiveClose(g, sub, IRI(ex+"a"), false)
	// includeSelf=false -> b, c, d (not a)
	assertTermValues(t, got, []string{ex + "b", ex + "c", ex + "d"})
}

func TestTransitiveClose_Cycle(t *testing.T) {
	t.Parallel()
	// a -> b -> c -> a (cycle)
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:knows ex:b .
		ex:b ex:knows ex:c .
		ex:c ex:knows ex:a .
	`)
	sub := &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "knows")}

	// zeroOrMore: should include all three plus self (self is a, already in cycle)
	got := transitiveClose(g, sub, IRI(ex+"a"), true)
	assertTermValues(t, got, []string{ex + "a", ex + "b", ex + "c"})

	// oneOrMore: focus is marked visited before BFS, so c->a won't re-add a
	got = transitiveClose(g, sub, IRI(ex+"a"), false)
	assertTermValues(t, got, []string{ex + "b", ex + "c"})
}

func TestTransitiveClose_NoEdges(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:other ex:b .
	`)
	sub := &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "knows")}

	// zeroOrMore with no outgoing edges: just self
	got := transitiveClose(g, sub, IRI(ex+"a"), true)
	assertTermValues(t, got, []string{ex + "a"})

	// oneOrMore with no outgoing edges: empty
	got = transitiveClose(g, sub, IRI(ex+"a"), false)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", termValues(got))
	}
}

func TestTransitiveClose_Diamond(t *testing.T) {
	t.Parallel()
	// a -> b, a -> c, b -> d, c -> d
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:p ex:b .
		ex:a ex:p ex:c .
		ex:b ex:p ex:d .
		ex:c ex:p ex:d .
	`)
	sub := &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "p")}
	got := transitiveClose(g, sub, IRI(ex+"a"), false)
	// d should appear once despite two paths
	assertTermValues(t, got, []string{ex + "b", ex + "c", ex + "d"})
}

// ---------------------------------------------------------------------------
// evalZeroOrOnePath
// ---------------------------------------------------------------------------

func TestEvalZeroOrOnePath(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:knows ex:b .
		ex:a ex:knows ex:c .
	`)
	sub := &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "knows")}
	got := evalZeroOrOnePath(g, sub, IRI(ex+"a"))
	// self + one step: a, b, c
	assertTermValues(t, got, []string{ex + "a", ex + "b", ex + "c"})
}

func TestEvalZeroOrOnePath_SelfInResult(t *testing.T) {
	t.Parallel()
	// If step leads back to self, should still be deduped
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:knows ex:a .
		ex:a ex:knows ex:b .
	`)
	sub := &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "knows")}
	got := evalZeroOrOnePath(g, sub, IRI(ex+"a"))
	assertTermValues(t, got, []string{ex + "a", ex + "b"})
}

func TestEvalZeroOrOnePath_NoEdges(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:other ex:b .
	`)
	sub := &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "knows")}
	got := evalZeroOrOnePath(g, sub, IRI(ex+"a"))
	// Just self
	assertTermValues(t, got, []string{ex + "a"})
}

// ---------------------------------------------------------------------------
// evalPath integration via zeroOrMore/oneOrMore/zeroOrOne kinds
// ---------------------------------------------------------------------------

func TestEvalPath_ZeroOrMore(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:p ex:b .
		ex:b ex:p ex:c .
	`)
	path := &PropertyPath{
		Kind: PathZeroOrMore,
		Sub:  &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "p")},
	}
	got := evalPath(g, path, IRI(ex+"a"))
	assertTermValues(t, got, []string{ex + "a", ex + "b", ex + "c"})
}

func TestEvalPath_OneOrMore(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:p ex:b .
		ex:b ex:p ex:c .
	`)
	path := &PropertyPath{
		Kind: PathOneOrMore,
		Sub:  &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "p")},
	}
	got := evalPath(g, path, IRI(ex+"a"))
	assertTermValues(t, got, []string{ex + "b", ex + "c"})
}

func TestEvalPath_ZeroOrOne(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix ex: <http://example.org/> .
		ex:a ex:p ex:b .
		ex:b ex:p ex:c .
	`)
	path := &PropertyPath{
		Kind: PathZeroOrOne,
		Sub:  &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "p")},
	}
	got := evalPath(g, path, IRI(ex+"a"))
	// Only self + one step, not transitive
	assertTermValues(t, got, []string{ex + "a", ex + "b"})
}

// ---------------------------------------------------------------------------
// pathToTerm
// ---------------------------------------------------------------------------

func TestPathToTerm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path *PropertyPath
		want Term
	}{
		{
			name: "nil path returns none term",
			path: nil,
			// pathToTerm returns Term{} which is TermNone
		},
		{
			name: "predicate path returns Pred",
			path: &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "p"), Node: IRI(ex + "p")},
			want: IRI(ex + "p"),
		},
		{
			name: "inverse path returns Node",
			path: &PropertyPath{
				Kind: PathInverse,
				Sub:  &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "p")},
				Node: BlankNode("x"),
			},
			want: BlankNode("x"),
		},
		{
			name: "zeroOrMore returns Node",
			path: &PropertyPath{
				Kind: PathZeroOrMore,
				Sub:  &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "p")},
				Node: BlankNode("y"),
			},
			want: BlankNode("y"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pathToTerm(tc.path)
			if got.Kind() != tc.want.Kind() || got.Value() != tc.want.Value() {
				t.Errorf("pathToTerm() = %v (kind=%v), want %v (kind=%v)", got, got.Kind(), tc.want, tc.want.Kind())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parsePath + evalPath end-to-end via Turtle
// ---------------------------------------------------------------------------

func TestParseAndEval_InversePath(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:shape sh:path [ sh:inversePath ex:parent ] .
		ex:alice ex:parent ex:bob .
		ex:carol ex:parent ex:bob .
	`)
	objs := g.Objects(IRI(ex+"shape"), IRI(SH+"path"))
	if len(objs) == 0 {
		t.Fatal("no sh:path")
	}
	path := parsePath(g, objs[0])
	if path.Kind != PathInverse {
		t.Fatalf("kind = %v, want PathInverse", path.Kind)
	}
	got := evalPath(g, path, IRI(ex+"bob"))
	assertTermValues(t, got, []string{ex + "alice", ex + "carol"})
}

func TestParseAndEval_SequencePath(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:shape sh:path ( ex:knows ex:name ) .
		ex:alice ex:knows ex:bob .
		ex:bob ex:name "Bob" .
	`)
	objs := g.Objects(IRI(ex+"shape"), IRI(SH+"path"))
	path := parsePath(g, objs[0])
	got := evalPath(g, path, IRI(ex+"alice"))
	if len(got) != 1 || got[0].Value() != "Bob" {
		t.Fatalf("got %v, want [Bob]", got)
	}
}

func TestParseAndEval_AlternativePath(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:shape sh:path [ sh:alternativePath ( ex:name ex:label ) ] .
		ex:alice ex:name "Alice" .
		ex:alice ex:label "A" .
	`)
	objs := g.Objects(IRI(ex+"shape"), IRI(SH+"path"))
	path := parsePath(g, objs[0])
	if path.Kind != PathAlternative {
		t.Fatalf("kind = %v, want PathAlternative", path.Kind)
	}
	got := evalPath(g, path, IRI(ex+"alice"))
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
}

func TestParseAndEval_ZeroOrMorePath(t *testing.T) {
	t.Parallel()
	g := mustGraph(t, `
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:shape sh:path [ sh:zeroOrMorePath ex:knows ] .
		ex:a ex:knows ex:b .
		ex:b ex:knows ex:c .
	`)
	objs := g.Objects(IRI(ex+"shape"), IRI(SH+"path"))
	path := parsePath(g, objs[0])
	got := evalPath(g, path, IRI(ex+"a"))
	assertTermValues(t, got, []string{ex + "a", ex + "b", ex + "c"})
}
