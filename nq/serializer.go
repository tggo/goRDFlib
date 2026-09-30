package nq

import (
	"fmt"
	"io"
	"slices"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
	"github.com/tggo/goRDFlib/store"
)

// Serialize writes the graph in N-Quads format.
func Serialize(g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	lines := make([]string, 0, g.Len())
	var serErr error

	// Determine graph context once. Every identifier except the default graph's
	// is a graph name, a blank node included (N-Quads graphLabel ::= IRIREF |
	// BLANK_NODE_LABEL).
	var graphSuffix string
	if id := g.Identifier(); !store.IsDefaultGraph(id) {
		term, err := ntsyntax.Term(id)
		if err != nil {
			return err
		}
		graphSuffix = " " + term
	}

	g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
		var sb strings.Builder
		line, err := formatLine(&sb, t.Subject, t.Predicate, t.Object, graphSuffix)
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
