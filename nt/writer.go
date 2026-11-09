package nt

import (
	"bufio"
	"context"
	"fmt"
	"io"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Writer writes N-Triples one statement at a time, in the order it is given
// and holding nothing but its output buffer. Serialize, by contrast, buffers
// and sorts a whole graph. It is the N-Triples twin of nq.Writer.
//
// A term Serialize would refuse (one the N-Triples parser would reject) is
// refused here too: that statement is not written and its error is returned.
// Errors from the underlying writer are sticky, so after one every call
// returns it. Call Flush when done. WithCanonical is the only option it
// honours.
//
// Blank nodes are written with their labels. Not safe for concurrent use.
type Writer struct {
	w    *bufio.Writer
	term func(rdflibgo.Term) (string, error)
	err  error
}

// NewWriter returns a Writer that writes to w.
func NewWriter(w io.Writer, opts ...Option) *Writer {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	return &Writer{w: bufio.NewWriter(w), term: termFn(&cfg)}
}

// Write writes one triple.
func (w *Writer) Write(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
	if w.err != nil {
		return w.err
	}
	line, err := ntTriple(rdflibgo.Triple{Subject: s, Predicate: p, Object: o}, w.term)
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

// WriteGraph writes every triple of g in the store's iteration order. It
// stops at the first error, including a store failure: statements already
// written stay in the buffer, so a caller that gets an error must discard the
// output rather than Flush it as a document.
func (w *Writer) WriteGraph(ctx context.Context, g *rdflibgo.Graph) error {
	for t, err := range g.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return fmt.Errorf("nt: write graph: %w", err)
		}
		if err := w.Write(t.Subject, t.Predicate, t.Object); err != nil {
			return err
		}
	}
	return nil
}

// Flush writes any buffered output to the underlying writer.
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	w.err = w.w.Flush()
	return w.err
}
