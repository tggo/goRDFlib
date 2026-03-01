package nq

import (
	"fmt"
	"io"
	"slices"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
)

// Serialize writes the graph in N-Quads format.
func Serialize(g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	lines := make([]string, 0, g.Len())
	var serErr error

	// Determine graph context once
	var graphSuffix string
	if id, ok := g.Identifier().(rdflibgo.URIRef); ok {
		term, err := ntsyntax.Term(id)
		if err != nil {
			return err
		}
		graphSuffix = " " + term
	}

	g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
		var sb strings.Builder
		s, err := ntsyntax.Term(t.Subject)
		if err != nil {
			serErr = err
			return false
		}
		p, err := ntsyntax.Term(t.Predicate)
		if err != nil {
			serErr = err
			return false
		}
		o, err := ntsyntax.Term(t.Object)
		if err != nil {
			serErr = err
			return false
		}
		sb.Grow(len(s) + len(p) + len(o) + len(graphSuffix) + 6)
		sb.WriteString(s)
		sb.WriteByte(' ')
		sb.WriteString(p)
		sb.WriteByte(' ')
		sb.WriteString(o)
		sb.WriteString(graphSuffix)
		sb.WriteString(" .")
		lines = append(lines, sb.String())
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
