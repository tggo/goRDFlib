package nq

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
	"github.com/tggo/goRDFlib/store"
)

// Serialize writes the graph in N-Quads format. The whole graph is read and
// sorted before the first byte is written, so a term N-Quads cannot express or
// a store failure fails Serialize with nothing written.
func Serialize(ctx context.Context, g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	n, err := g.Len(ctx)
	if err != nil {
		return fmt.Errorf("nq: serialize: %w", err)
	}
	lines := make([]string, 0, n)

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

	var sb strings.Builder
	for t, err := range g.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return fmt.Errorf("nq: serialize: %w", err)
		}
		line, err := formatLine(&sb, t.Subject, t.Predicate, t.Object, graphSuffix)
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
