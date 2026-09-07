package jsonld_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/provenance"
	"github.com/tggo/goRDFlib/term"
)

func ex(local string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + local) }

// TestBlankNodeParseScope checks that one expansion has one blank-node scope,
// while another parse into the same graph cannot reuse those identities.
func TestBlankNodeParseScope(t *testing.T) {
	const doc = `[
  {"@id":"http://example.org/root", "http://example.org/link":{"@id":"_:shared"}},
  {"@id":"_:shared", "http://example.org/self":{"@id":"_:shared"}},
  {"http://example.org/name":"anonymous"}
]`
	for _, preserve := range []bool{false, true} {
		t.Run(map[bool]string{false: "scoped", true: "preserved"}[preserve], func(t *testing.T) {
			g := graph.NewGraph()
			idx := provenance.NewIndex()
			calls := 0
			opts := []jsonld.Option{jsonld.WithProvenance(func(s term.Subject, p term.URIRef, o term.Term, line int) {
				calls++
				if !g.Contains(s, p, o) {
					t.Error("callback terms do not identify an inserted triple")
				}
				if !s.Equal(ex("root")) || line != 2 {
					t.Errorf("unexpected provenance: %s at line %d", s.N3(), line)
				}
				idx.Triple(s, p, o, line)
			})}
			if preserve {
				opts = append(opts, jsonld.WithPreserveBlankNodeIDs())
			}
			for i := 0; i < 2; i++ {
				if err := jsonld.Parse(g, strings.NewReader(doc), opts...); err != nil {
					t.Fatal(err)
				}
			}
			want := 6
			if preserve {
				want = 3 // Compatibility preserves even the expander's anonymous labels.
			}
			if g.Len() != want || calls != 2 || idx.Len() != want/3 {
				t.Fatalf("graph/callback/index counts = %d/%d/%d, want %d/2/%d", g.Len(), calls, idx.Len(), want, want/3)
			}
			g.Triples(nil, nil, nil)(func(tr term.Triple) bool {
				if tr.Predicate == ex("link") {
					bn, ok := tr.Object.(term.BNode)
					if !ok || !g.Contains(bn, ex("self"), bn) {
						t.Error("blank-node reference differs from its declaration")
					}
					if line, ok := idx.Line(tr.Subject, tr.Predicate, tr.Object); !ok || line != 2 {
						t.Errorf("inserted blank-node object has no exact source line: %d (%v)", line, ok)
					}
				}
				return true
			})
			if g.Contains(term.NewBNode("shared"), ex("self"), term.NewBNode("shared")) {
				t.Error("JSON-gold should relabel the source ID even in compatibility mode")
			}
		})
	}
}

// TestProvenanceRelabelledBlankNodeCollision rejects an accidental match between
// a generated label and another node's source ID, including compatibility mode.
func TestProvenanceRelabelledBlankNodeCollision(t *testing.T) {
	const doc = `[
  {"http://example.org/name":"anonymous"},
  {"@id":"_:b0", "http://example.org/name":"labelled"}
]`
	for _, opts := range [][]jsonld.Option{nil, {jsonld.WithPreserveBlankNodeIDs()}} {
		g, idx := parseWithProvenance(t, doc, opts...)
		if g.Len() != 2 {
			t.Fatalf("graph has %d triples, want 2 separate nodes", g.Len())
		}
		if idx.Len() != 0 {
			t.Errorf("reported %d unverified blank-node source lines", idx.Len())
		}
	}
}

func parseWithProvenance(t *testing.T, doc string, opts ...jsonld.Option) (*graph.Graph, *provenance.Index) {
	t.Helper()
	g := graph.NewGraph()
	idx := provenance.NewIndex()
	opts = append(opts, jsonld.WithProvenance(idx.Triple))
	if err := jsonld.Parse(g, strings.NewReader(doc), opts...); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return g, idx
}

// TestProvenanceCompactIRIs is the ordinary case: a context with a prefix, and
// node objects identified by compact IRIs. The identifier written in the source
// is expanded through the document's own context, so the match is exact rather
// than a resemblance.
//
//	 1  {
//	 2    "@context": {"ex": "..."},
//	 3    "@graph": [
//	 4      {
//	 5        "@id": "ex:alice",
//	 ...
//	 9      {
//	10        "@id": "ex:bob",
func TestProvenanceCompactIRIs(t *testing.T) {
	const doc = `{
  "@context": {"ex": "http://example.org/"},
  "@graph": [
    {
      "@id": "ex:alice",
      "ex:name": "Alice",
      "ex:age": 34
    },
    {
      "@id": "ex:bob",
      "ex:name": "Bob"
    }
  ]
}
`
	_, idx := parseWithProvenance(t, doc)

	if got, ok := idx.SubjectLine(ex("alice")); !ok || got != 4 {
		t.Errorf("ex:alice reported line %d (%v), want 4 — where its node object opens", got, ok)
	}
	if got, ok := idx.SubjectLine(ex("bob")); !ok || got != 9 {
		t.Errorf("ex:bob reported line %d (%v), want 9", got, ok)
	}

	// Every triple of a node carries the node's line, since JSON-LD provenance
	// is per node object rather than per triple.
	if got, ok := idx.Line(ex("alice"), ex("name"), term.NewLiteral("Alice")); !ok || got != 4 {
		t.Errorf("ex:alice ex:name reported line %d (%v), want 4", got, ok)
	}
	if got, ok := idx.Line(ex("alice"), ex("age"), term.NewLiteral(34)); !ok || got != 4 {
		t.Errorf("ex:alice ex:age reported line %d (%v), want 4", got, ok)
	}
}

// TestProvenanceAbsoluteAndRelativeIDs covers identifiers written out in full
// and identifiers resolved against the base.
func TestProvenanceAbsoluteAndRelativeIDs(t *testing.T) {
	const doc = `{
  "@context": {"name": "http://example.org/name"},
  "@graph": [
    {
      "@id": "http://example.org/absolute",
      "name": "Absolute"
    },
    {
      "@id": "relative",
      "name": "Relative"
    }
  ]
}
`
	_, idx := parseWithProvenance(t, doc, jsonld.WithBase("http://example.org/"))

	if got, ok := idx.SubjectLine(ex("absolute")); !ok || got != 4 {
		t.Errorf("the absolute @id reported line %d (%v), want 4", got, ok)
	}
	if got, ok := idx.SubjectLine(ex("relative")); !ok || got != 8 {
		t.Errorf("the relative @id reported line %d (%v), want 8 — it must resolve against the base", got, ok)
	}
}

// TestProvenanceAliasedID covers a context that renames @id, which is common
// enough in hand-written JSON-LD that ignoring it would make the feature miss
// most of a real document.
func TestProvenanceAliasedID(t *testing.T) {
	const doc = `{
  "@context": {
    "id": "@id",
    "ex": "http://example.org/"
  },
  "id": "ex:carol",
  "ex:name": "Carol"
}
`
	_, idx := parseWithProvenance(t, doc)

	if got, ok := idx.SubjectLine(ex("carol")); !ok || got != 1 {
		t.Errorf(`the aliased "id" reported line %d (%v), want 1 — the node object is the whole document`, got, ok)
	}
}

// TestProvenanceAliasDeclaredAfterUse pins why the scan takes two passes: JSON
// object key order carries no meaning, so a context may be written below the
// node objects that use it.
func TestProvenanceAliasDeclaredAfterUse(t *testing.T) {
	const doc = `{
  "id": "http://example.org/dave",
  "http://example.org/name": "Dave",
  "@context": {"id": "@id"}
}
`
	_, idx := parseWithProvenance(t, doc)

	if got, ok := idx.SubjectLine(ex("dave")); !ok || got != 1 {
		t.Errorf("reported line %d (%v), want 1 — the alias is declared after its use", got, ok)
	}
}

// TestProvenanceSkipsBlankNodes checks that a node with no @id gets no line
// rather than a wrong one. Its label is invented by the expander and appears
// nowhere in the source.
func TestProvenanceSkipsBlankNodes(t *testing.T) {
	const doc = `{
  "@context": {"ex": "http://example.org/"},
  "@id": "ex:erin",
  "ex:knows": {
    "ex:name": "an anonymous friend"
  }
}
`
	g, idx := parseWithProvenance(t, doc)

	if got, ok := idx.SubjectLine(ex("erin")); !ok || got != 1 {
		t.Errorf("ex:erin reported line %d (%v), want 1", got, ok)
	}

	// The blank node's own triple must be absent from the index, not present
	// with a borrowed line.
	blanks := 0
	g.Triples(nil, nil, nil)(func(tr term.Triple) bool {
		if _, isBlank := tr.Subject.(term.BNode); isBlank {
			blanks++
			if line, ok := idx.Line(tr.Subject, tr.Predicate, tr.Object); ok {
				t.Errorf("a blank-node triple was given line %d; nothing in the source names it", line)
			}
		}
		return true
	})
	if blanks == 0 {
		t.Fatal("expected the nested node object to become a blank node")
	}
}

// TestProvenanceExplicitBlankNodeLabel covers a blank node the author did name.
// The expander relabels it without exposing its mapping, so nothing is reported.
// Matching the spelling of a generated label does not prove identity — what
// must not happen is a line attached to the wrong node.
func TestProvenanceExplicitBlankNodeLabel(t *testing.T) {
	const doc = `{
  "@context": {"ex": "http://example.org/"},
  "@id": "_:labelled",
  "ex:name": "Named blank node"
}
`
	g, idx := parseWithProvenance(t, doc)

	g.Triples(nil, nil, nil)(func(tr term.Triple) bool {
		line, ok := idx.Line(tr.Subject, tr.Predicate, tr.Object)
		if ok && line != 1 {
			t.Errorf("got line %d, want 1 or nothing at all", line)
		}
		return true
	})
}

// TestProvenanceIsOptIn checks that the parse is byte-identical without the
// option, which matters because the option makes Parse read its input twice.
func TestProvenanceIsOptIn(t *testing.T) {
	const doc = `{
  "@context": {"ex": "http://example.org/"},
  "@id": "ex:frank",
  "ex:name": "Frank"
}
`
	plain := graph.NewGraph()
	if err := jsonld.Parse(plain, strings.NewReader(doc)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	traced, idx := parseWithProvenance(t, doc)

	if plain.Len() != traced.Len() {
		t.Errorf("the option changed the graph: %d triples vs %d", plain.Len(), traced.Len())
	}
	if idx.Len() == 0 {
		t.Error("nothing was recorded")
	}
}

// TestProvenanceTopLevelArray covers a document that is an array of node
// objects, where the context sits on the first entry that carries one.
func TestProvenanceTopLevelArray(t *testing.T) {
	const doc = `[
  {
    "@context": {"ex": "http://example.org/"},
    "@id": "ex:grace",
    "ex:name": "Grace"
  },
  {
    "@context": {"ex": "http://example.org/"},
    "@id": "ex:heidi",
    "ex:name": "Heidi"
  }
]
`
	_, idx := parseWithProvenance(t, doc)

	if got, ok := idx.SubjectLine(ex("grace")); !ok || got != 2 {
		t.Errorf("ex:grace reported line %d (%v), want 2", got, ok)
	}
	if got, ok := idx.SubjectLine(ex("heidi")); !ok || got != 7 {
		t.Errorf("ex:heidi reported line %d (%v), want 7", got, ok)
	}
}

// TestProvenanceRepeatedNode checks that a node stated twice keeps the earlier
// block, which is the one a reader thinks of as its definition.
func TestProvenanceRepeatedNode(t *testing.T) {
	const doc = `{
  "@context": {"ex": "http://example.org/"},
  "@graph": [
    {
      "@id": "ex:ivan",
      "ex:name": "Ivan"
    },
    {
      "@id": "ex:ivan",
      "ex:age": 50
    }
  ]
}
`
	_, idx := parseWithProvenance(t, doc)

	if got, ok := idx.SubjectLine(ex("ivan")); !ok || got != 4 {
		t.Errorf("ex:ivan reported line %d (%v), want the first block at line 4", got, ok)
	}
}

// TestProvenanceNoContext covers a document with no context at all, where every
// key is already an absolute IRI.
func TestProvenanceNoContext(t *testing.T) {
	const doc = `{
  "@id": "http://example.org/judy",
  "http://example.org/name": "Judy"
}
`
	_, idx := parseWithProvenance(t, doc)

	if got, ok := idx.SubjectLine(ex("judy")); !ok || got != 1 {
		t.Errorf("reported line %d (%v), want 1", got, ok)
	}
}

// TestParseDoesNotPanicOnMalformedInput pins the recovery around json-gold.
//
// These documents made the processor dereference a nil URL rather than reject
// them, taking the process down with it — which for a parser routinely pointed
// at untrusted bytes is not an acceptable failure mode. Found by fuzzing.
func TestParseDoesNotPanicOnMalformedInput(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		base string
	}{
		{"percent as @id", `{"@id":"%"}`, "http://example.org/"},
		{"percent as @id, no base", `{"@id":"%"}`, ""},
		{"percent in a nested @id", `{"@graph":[{"@id":"%"}]}`, "http://example.org/"},
		{"bare percent term", `{"@context":{"p":"%"},"p":"v"}`, "http://example.org/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := graph.NewGraph()
			opts := []jsonld.Option{}
			if tc.base != "" {
				opts = append(opts, jsonld.WithBase(tc.base))
			}
			// Must return, not panic. Whether it errors is json-gold's call.
			err := jsonld.Parse(g, strings.NewReader(tc.doc), opts...)
			if err != nil && !errors.Is(err, jsonld.ErrProcessorPanic) {
				t.Logf("rejected with %v", err)
			}
		})
	}
}

// TestProcessorPanicIsIdentifiable checks that a caller can tell a crash apart
// from a rejection, which is the difference between "this document is wrong"
// and "report this upstream".
func TestProcessorPanicIsIdentifiable(t *testing.T) {
	g := graph.NewGraph()
	err := jsonld.Parse(g, strings.NewReader(`{"@id":"%"}`), jsonld.WithBase("http://example.org/"))
	if err == nil {
		t.Skip("json-gold no longer crashes on this input; the guard is still worth keeping")
	}
	if !errors.Is(err, jsonld.ErrProcessorPanic) {
		t.Fatalf("got %v, want an error wrapping ErrProcessorPanic", err)
	}
	if !strings.Contains(err.Error(), "goroutine") {
		t.Error("the error carries no stack; a bug report would have nothing to attach")
	}
}

// TestProvenanceContextForms covers the shapes a context can take, since each
// one is a separate branch of the alias scan and a missed branch means the
// feature silently reports nothing for that document.
func TestProvenanceContextForms(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want int
	}{
		{
			"context as an array",
			`{
  "@context": [
    {"id": "@id"},
    {"ex": "http://example.org/"}
  ],
  "id": "ex:arr",
  "ex:name": "Array context"
}
`, 1,
		},
		{
			"alias defined as an expanded term definition",
			`{
  "@context": {
    "id": {"@id": "@id"},
    "ex": "http://example.org/"
  },
  "id": "ex:expanded",
  "ex:name": "Expanded definition"
}
`, 1,
		},
		{
			"context nested inside a node object",
			`{
  "@context": {"ex": "http://example.org/"},
  "@id": "ex:outer",
  "ex:child": {
    "@context": {"id": "@id"},
    "id": "ex:inner",
    "ex:name": "Inner"
  }
}
`, 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, idx := parseWithProvenance(t, tc.doc)
			if idx.Len() == 0 {
				t.Fatal("nothing was recorded; an alias form was missed")
			}
		})
	}
}

// TestProvenanceScopedContextIsSilent pins the documented limit: only the
// top-level context is used to expand identifiers, so an identifier that only
// makes sense under a scoped context gets no line rather than a wrong one.
func TestProvenanceScopedContextIsSilent(t *testing.T) {
	const doc = `{
  "@context": {"ex": "http://example.org/"},
  "@id": "ex:outer",
  "ex:child": {
    "@context": {"other": "http://elsewhere.example/"},
    "@id": "other:scoped",
    "ex:name": "Scoped"
  }
}
`
	g, idx := parseWithProvenance(t, doc)

	// Whatever the scoped identifier expands to, it must not be given a line
	// that belongs to some other node.
	scoped := term.NewURIRefUnsafe("http://elsewhere.example/scoped")
	if line, ok := idx.SubjectLine(scoped); ok && line != 4 {
		t.Errorf("the scoped node got line %d, which is not where it is written", line)
	}
	if g.Len() == 0 {
		t.Fatal("expected triples")
	}
}

// TestProvenanceMalformedSourceIsSilent checks that a document the processor
// rejects produces no lines at all, rather than lines from a partial scan.
func TestProvenanceMalformedSourceIsSilent(t *testing.T) {
	for _, doc := range []string{`{"@id": "http://example.org/a"`, `{`, ``, `[1,2`} {
		g := graph.NewGraph()
		idx := provenance.NewIndex()
		err := jsonld.Parse(g, strings.NewReader(doc), jsonld.WithProvenance(idx.Triple))
		if err == nil && g.Len() == 0 {
			continue // parsed to nothing, which is fine
		}
		if err != nil && idx.Len() != 0 {
			t.Errorf("%q failed to parse but reported %d lines", doc, idx.Len())
		}
	}
}
