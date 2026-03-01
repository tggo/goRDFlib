package nt

import (
	"fmt"
	"io"
	"slices"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
)

// Serialize writes the graph in N-Triples format.
func Serialize(g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	lines := make([]string, 0, g.Len())
	var serErr error
	g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
		line, err := ntTriple(t)
		if err != nil {
			serErr = err
			return false
		}
		lines = append(lines, line)
		return true
	})
	if serErr != nil {
		return serErr
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
