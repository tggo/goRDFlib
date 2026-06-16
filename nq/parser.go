package nq

import (
	"errors"
	"fmt"
	"io"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
)

// ErrLineTooLong is reported (wrapped, with the line number) when a line exceeds
// the parser's byte cap. Detect it with errors.Is and retry with
// WithUnboundedLines (or a larger WithMaxLineLength). Re-exported from the
// internal line reader so callers outside the module can match against it.
var ErrLineTooLong = ntsyntax.ErrLineTooLong

// QuadHandler is called for each parsed quad. The graph term may be nil for triples
// without an explicit graph context.
type QuadHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term)

// StreamHandler is the callback used by ParseStream. Returning a non-nil error
// aborts the parse and is propagated to the caller of ParseStream. The graph
// term is nil for triples without an explicit graph context.
type StreamHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error

// Parse parses N-Quads format RDF into the given graph.
// Graph context is preserved and passed to the optional QuadHandler if configured via WithQuadHandler.
// When no QuadHandler is set, all triples are added to the given graph regardless of graph context.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	quadHandler := func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error {
		g.Add(s, p, o)
		return nil
	}
	return parseLines(r, opts, quadHandler, true)
}

// ParseStream parses N-Quads format RDF and dispatches each parsed quad to the
// handler without populating any graph. Use this for streaming large inputs
// (e.g. converting an N-Quads dump to another format on the fly) where holding
// the full graph in memory is not feasible. Returning an error from the handler
// aborts the parse.
func ParseStream(r io.Reader, h StreamHandler, opts ...Option) error {
	if h == nil {
		return fmt.Errorf("nq.ParseStream: handler must not be nil")
	}
	return parseLines(r, opts, h, false)
}

// parseLines is the shared scanner loop used by Parse and ParseStream.
// dispatchQuadHandler controls whether cfg.quadHandler (set via WithQuadHandler)
// is also invoked: Parse honors it for back-compat, ParseStream uses only `h`.
func parseLines(r io.Reader, opts []Option, h StreamHandler, dispatchQuadHandler bool) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	nextLine := ntsyntax.NewLineReader(r, cfg.maxLineLen, cfg.unbounded)
	lineNum := 0
	for {
		raw, err := nextLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, ntsyntax.ErrLineTooLong) {
				return fmt.Errorf("line %d: %w", lineNum+1, err)
			}
			return err
		}
		lineNum++
		line := strings.TrimSpace(raw)
		if line == "" || line[0] == '#' {
			continue
		}
		if err := parseNQLine(line, lineNum, h, cfg.quadHandler, dispatchQuadHandler, cfg.provenance); err != nil {
			if cfg.errorHandler == nil {
				return err
			}
			fixedLine, retry := cfg.errorHandler(lineNum, line, err)
			if retry {
				if err2 := parseNQLine(fixedLine, lineNum, h, cfg.quadHandler, dispatchQuadHandler, cfg.provenance); err2 != nil {
					return fmt.Errorf("line %d: retry failed: %w", lineNum, err2)
				}
			}
		}
	}
	return nil
}

func parseNQLine(line string, lineNum int, h StreamHandler, qh QuadHandler, dispatchQuadHandler bool, prov ProvenanceHandler) error {
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

	if dispatchQuadHandler && qh != nil {
		qh(subj, pred, obj, graphCtx)
	}
	if err := h(subj, pred, obj, graphCtx); err != nil {
		return err
	}
	if prov != nil {
		prov(subj, pred, obj, graphCtx, lineNum)
	}
	return nil
}
