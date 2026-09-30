package nq

import (
	"bytes"
	"errors"
	"sort"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
	"github.com/tggo/goRDFlib/store"
)

// What Writer writes reads back as the same statements in the same graphs:
// default (nil and store.DefaultGraph), an IRI graph, a blank-node graph.
func TestWriterRoundTrip(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1, _ := rdflibgo.NewURIRef("http://example.org/g1")
	bg := rdflibgo.NewBNode()
	lit := rdflibgo.NewLiteral("a \"quoted\" value with spaces")

	var buf bytes.Buffer
	w := NewWriter(&buf)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(w.Write(s, p, lit, nil))
	must(w.Write(s, p, s, store.DefaultGraph))
	must(w.Write(s, p, lit, g1))
	must(w.WriteQuad(rdflibgo.Quad{Triple: rdflibgo.Triple{Subject: s, Predicate: p, Object: bg}, Graph: bg}))
	must(w.Flush())

	var got []string
	err := ParseStream(strings.NewReader(buf.String()), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, g rdflibgo.Term) error {
		gs := "default"
		if g != nil {
			gs = g.N3()
		}
		got = append(got, gs+" "+o.N3())
		return nil
	}, WithPreserveBlankNodeIDs())
	must(err)
	sort.Strings(got)
	want := []string{
		"<http://example.org/g1> " + lit.N3(),
		bg.N3() + " " + bg.N3(),
		"default " + lit.N3(),
		"default <http://example.org/s>",
	}
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("round trip:\n got %q\nwant %q", got, want)
	}
}

// WriteGraph writes a named graph's triples labelled with its identifier.
func TestWriterWriteGraph(t *testing.T) {
	id, _ := rdflibgo.NewURIRef("http://example.org/g")
	g := rdflibgo.NewGraph(rdflibgo.WithIdentifier(id))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("x"))
	g.Add(s, p, rdflibgo.NewLiteral("y"))

	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WriteGraph(g); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), buf.String())
	}
	for _, l := range lines {
		if !strings.HasSuffix(l, " <http://example.org/g> .") {
			t.Fatalf("line not in graph g: %q", l)
		}
	}
}

// A term the parser would reject is refused and nothing of it is written;
// the writer stays usable.
func TestWriterRefusesUnwritableTerm(t *testing.T) {
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	rel, err := rdflibgo.NewURIRef("relative")
	if err != nil {
		t.Skip("NewURIRef rejects relative IRIs; nothing to refuse")
	}
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(rel, p, p, nil); !errors.Is(err, ntsyntax.ErrRelativeIRI) {
		t.Fatalf("err = %v, want ErrRelativeIRI", err)
	}
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	if err := w.Write(s, p, p, nil); err != nil {
		t.Fatalf("writer unusable after a refused term: %v", err)
	}
	_ = w.Flush()
	if strings.Count(buf.String(), "\n") != 1 {
		t.Fatalf("want exactly the valid statement, got %q", buf.String())
	}
}

// An error from the underlying writer is sticky.
func TestWriterStickyError(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	w := NewWriter(&failWriter{})
	_ = w.Write(s, s, s, nil)
	first := w.Flush()
	if first == nil {
		t.Fatal("Flush on a failing writer returned nil")
	}
	if err := w.Write(s, s, s, nil); !errors.Is(err, first) {
		t.Fatalf("after failure Write = %v, want %v", err, first)
	}
}
