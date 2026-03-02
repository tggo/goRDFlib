package nq

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
)

// QuadHandler is called for each parsed quad. The graph term may be nil for triples
// without an explicit graph context.
type QuadHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term)

// Parse parses N-Quads format RDF into the given graph.
// Graph context is preserved and passed to the optional QuadHandler if configured via WithQuadHandler.
// When no QuadHandler is set, all triples are added to the given graph regardless of graph context.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if err := parseNQLine(g, line, lineNum, cfg.quadHandler); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func parseNQLine(g *rdflibgo.Graph, line string, lineNum int, handler QuadHandler) error {
	p := &ntsyntax.LineParser{Line: line, Pos: 0, LineNum: lineNum}

	subj, err := p.ReadSubject()
	if err != nil {
		return err
	}
	p.SkipSpaces()

	pred, err := p.ReadPredicate()
	if err != nil {
		return err
	}
	p.SkipSpaces()

	obj, err := p.ReadObject()
	if err != nil {
		return err
	}
	p.SkipSpaces()

	// Optional 4th element: graph context
	graphCtx, err := p.ReadGraphLabel()
	if err != nil {
		return err
	}
	p.SkipSpaces()

	if !p.Expect('.') {
		return fmt.Errorf("line %d: expected '.'", lineNum)
	}

	if handler != nil {
		handler(subj, pred, obj, graphCtx)
	}

	g.Add(subj, pred, obj)
	return nil
}
