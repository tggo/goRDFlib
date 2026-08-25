package jsonld_test

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/provenance"
	"github.com/tggo/goRDFlib/term"
)

func ex(local string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + local) }

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
// Whether the expander keeps the label is its business; if it does, the line is
// recovered, and if it does not, nothing is reported. Either is correct — what
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
