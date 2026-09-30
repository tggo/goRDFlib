package nq

import (
	"bufio"
	"io"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
	"github.com/tggo/goRDFlib/store"
)

// Writer writes N-Quads one statement at a time, for output that is produced
// incrementally or gathered from several graphs into one stream. Serialize
// buffers and sorts a whole graph; Writer writes in the order it is given and
// holds nothing but its output buffer.
//
// A term Serialize would refuse (one the N-Quads parser would reject) is
// refused here too: that statement is not written and its error is returned.
// Errors from the underlying writer are sticky, so after one every call
// returns it. Call Flush when done.
//
// Blank nodes are written with their labels, so the same BNode written in two
// calls is the same node in the output. Not safe for concurrent use.
type Writer struct {
	w   *bufio.Writer
	sb  strings.Builder
	err error
}

// NewWriter returns a Writer that writes to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: bufio.NewWriter(w)}
}

// Write writes one statement. A nil graph, or store.DefaultGraph, puts it in
// the default graph; any other IRI or blank node is written as its graph name.
func (w *Writer) Write(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error {
	if w.err != nil {
		return w.err
	}
	var suffix string
	if graph != nil && !store.IsDefaultGraph(graph) {
		g, err := ntsyntax.Term(graph)
		if err != nil {
			return err
		}
		suffix = " " + g
	}
	line, err := formatLine(&w.sb, s, p, o, suffix)
	if err != nil {
		return err
	}
	if _, err := w.w.WriteString(line); err != nil {
		w.err = err
		return err
	}
	if err := w.w.WriteByte('\n'); err != nil {
		w.err = err
	}
	return w.err
}

// WriteQuad writes q; see Write.
func (w *Writer) WriteQuad(q rdflibgo.Quad) error {
	var graph rdflibgo.Term
	if q.Graph != nil {
		graph = q.Graph
	}
	return w.Write(q.Subject, q.Predicate, q.Object, graph)
}

// WriteGraph writes every triple of g in g's graph (its Identifier), in the
// store's iteration order. It stops at the first error.
func (w *Writer) WriteGraph(g *rdflibgo.Graph) error {
	id := g.Identifier()
	var err error
	g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
		err = w.Write(t.Subject, t.Predicate, t.Object, id)
		return err == nil
	})
	return err
}

// Flush writes any buffered output to the underlying writer.
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	w.err = w.w.Flush()
	return w.err
}

// formatLine renders one statement without the trailing newline, reusing sb.
// suffix is empty or " <graph>".
func formatLine(sb *strings.Builder, s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, suffix string) (string, error) {
	st, err := ntsyntax.Term(s)
	if err != nil {
		return "", err
	}
	pt, err := ntsyntax.Term(p)
	if err != nil {
		return "", err
	}
	ot, err := ntsyntax.Term(o)
	if err != nil {
		return "", err
	}
	sb.Reset()
	sb.Grow(len(st) + len(pt) + len(ot) + len(suffix) + 4)
	sb.WriteString(st)
	sb.WriteByte(' ')
	sb.WriteString(pt)
	sb.WriteByte(' ')
	sb.WriteString(ot)
	sb.WriteString(suffix)
	sb.WriteString(" .")
	return sb.String(), nil
}
