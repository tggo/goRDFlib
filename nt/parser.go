package nt

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

// TripleHandler is the callback used by ParseStream. Returning a non-nil error
// aborts the parse and is propagated to the caller of ParseStream.
type TripleHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error

// Parse parses N-Triples format RDF into the given graph.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	return parseLines(r, opts, func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
		g.Add(s, p, o)
		return nil
	})
}

// ParseStream parses N-Triples format RDF and dispatches each parsed triple to
// the handler without populating any graph. Use this for streaming large inputs
// where holding the full graph in memory is not feasible. Returning an error
// from the handler aborts the parse.
func ParseStream(r io.Reader, h TripleHandler, opts ...Option) error {
	if h == nil {
		return fmt.Errorf("nt.ParseStream: handler must not be nil")
	}
	return parseLines(r, opts, h)
}

// parseLines is the shared scanner loop used by Parse and ParseStream.
func parseLines(r io.Reader, opts []Option, h TripleHandler) error {
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
		if err := parseNTLine(line, lineNum, h, cfg.provenance); err != nil {
			if cfg.errorHandler == nil {
				return err
			}
			fixedLine, retry := cfg.errorHandler(lineNum, line, err)
			if retry {
				if err2 := parseNTLine(fixedLine, lineNum, h, cfg.provenance); err2 != nil {
					return fmt.Errorf("line %d: retry failed: %w", lineNum, err2)
				}
			}
		}
	}
	return nil
}

func parseNTLine(line string, lineNum int, h TripleHandler, prov ProvenanceHandler) error {
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

	if !p.Expect('.') {
		return fmt.Errorf("line %d: expected '.'", lineNum)
	}

	if err := h(subj, pred, obj); err != nil {
		return err
	}
	if prov != nil {
		prov(subj, pred, obj, lineNum)
	}
	return nil
}
