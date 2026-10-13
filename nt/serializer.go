package nt

import (
	"context"
	"fmt"
	"io"
	"slices"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
)

// Errors Serialize returns for terms N-Triples cannot express. They are the
// same values nq.Serialize returns, so errors.Is works with either package.
var (
	// ErrRelativeIRI: N-Triples 1.1 §2.2 allows only absolute IRIs.
	ErrRelativeIRI = ntsyntax.ErrRelativeIRI
	// ErrInvalidUTF8: an IRI or literal is not valid UTF-8.
	ErrInvalidUTF8 = ntsyntax.ErrInvalidUTF8
	// ErrInvalidIRI: an IRI contains a character no IRI may contain (space,
	// control characters, <>"{}|^`\).
	ErrInvalidIRI = ntsyntax.ErrInvalidIRI
)

// Serialize writes the graph in N-Triples format. It fails, writing nothing,
// when a term cannot be expressed in N-Triples (see ErrRelativeIRI,
// ErrInvalidUTF8 and ErrInvalidIRI) or when the store fails while the graph is
// read: the whole graph is read and sorted before the first byte is written, so
// a store error never leaves a truncated document behind.
func Serialize(ctx context.Context, g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	n, err := g.Len(ctx)
	if err != nil {
		return fmt.Errorf("nt: serialize: %w", err)
	}
	lines := make([]string, 0, n)
	for t, err := range g.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return fmt.Errorf("nt: serialize: %w", err)
		}
		line, err := ntTriple(t)
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	slices.Sort(lines)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func ntTriple(t rdflibgo.Triple) (string, error) {
	s, err := ntsyntax.Term(t.Subject)
	if err != nil {
		return "", err
	}
	p, err := ntsyntax.Term(t.Predicate)
	if err != nil {
		return "", err
	}
	o, err := ntsyntax.Term(t.Object)
	if err != nil {
		return "", err
	}
	return s + " " + p + " " + o + " .", nil
}
