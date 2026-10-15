package jsonld

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	rdflibgo "github.com/tggo/goRDFlib"
)

// ErrNodeTooLong is returned by ParseNodes for an input line longer than the
// line limit. Raise the limit with WithMaxNodeSize.
var ErrNodeTooLong = errors.New("jsonld: node line exceeds the maximum size; raise it with WithMaxNodeSize")

// defaultMaxNodeSize bounds one line of ParseNodes input.
const defaultMaxNodeSize = 64 << 20

// NodeErrorHandler decides what happens to a node ParseNodes could not
// expand: return nil to skip it and continue, or an error to stop with it.
// line is the node's 1-based line in the input.
type NodeErrorHandler func(line int, node []byte, err error) error

// WithNodeErrorHandler sets the handler ParseNodes calls for a node that
// fails to expand. Without it the first failure stops ParseNodes. Errors
// from the store or the context are never passed to it.
func WithNodeErrorHandler(h NodeErrorHandler) Option {
	return func(c *config) { c.nodeErrorHandler = h }
}

// WithMaxNodeSize sets the longest line ParseNodes accepts (default 64 MiB).
func WithMaxNodeSize(n int) Option {
	return func(c *config) { c.maxNodeSize = n }
}

// ParseNodes reads newline-delimited JSON from r, one JSON-LD node object per
// line, all sharing context, and adds their triples to g in batches as the
// lines are read (see NodeParser for the semantics and the forms context may
// take). Blank lines are skipped. An error names the line it came from, and
// provenance reports the line of the node that described each subject.
// Options are those of Parse plus WithNodeErrorHandler and WithMaxNodeSize.
//
// A bad line without a WithNodeErrorHandler stops ParseNodes after the good
// lines before it have been added. A store failure or a done context stops it
// at once; batches delivered before stay in g.
func ParseNodes(ctx context.Context, g *rdflibgo.Graph, r io.Reader, context any, opts ...Option) error {
	return ParseNodesStream(ctx, r, context, func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
		if err := g.Add(ctx, s, p, o); err != nil {
			return fmt.Errorf("adding to graph: %w", err)
		}
		return nil
	}, opts...)
}

// handlerError marks an error from the triple handler (the store, for
// ParseNodes): it is never a bad node, so a NodeErrorHandler never sees it.
type handlerError struct{ err error }

func (e handlerError) Error() string { return e.err.Error() }
func (e handlerError) Unwrap() error { return e.err }

// ParseNodesStream is ParseNodes delivering the triples to h instead of a
// graph.
func ParseNodesStream(ctx context.Context, r io.Reader, context any, h TripleHandler, opts ...Option) error {
	if h == nil {
		return ErrNilHandler
	}
	p, err := NewNodeParser(context, func(s rdflibgo.Subject, pr rdflibgo.URIRef, o rdflibgo.Term) error {
		if err := h(s, pr, o); err != nil {
			return handlerError{err}
		}
		return nil
	}, opts...)
	if err != nil {
		return err
	}
	maxSize := p.cfg.maxNodeSize
	if maxSize <= 0 {
		maxSize = defaultMaxNodeSize
	}
	sc := bufio.NewScanner(r)
	// The limit is the larger of maxSize and the buffer's capacity.
	sc.Buffer(make([]byte, 0, min(64<<10, maxSize)), maxSize)
	line := 0
	for sc.Scan() {
		line++
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		err := p.parseNode(ctx, b, line)
		if err == nil {
			continue
		}
		var he handlerError
		if errors.As(err, &he) {
			// A batch delivery: the store failed, not this node.
			return fmt.Errorf("jsonld: delivering the batch ending at line %d: %w", line, he.err)
		}
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			return fmt.Errorf("jsonld: line %d: %w", line, err)
		}
		err = fmt.Errorf("jsonld: line %d: %w", line, err)
		if h := p.cfg.nodeErrorHandler; h != nil {
			if err = h(line, b, errors.Unwrap(err)); err == nil {
				continue
			}
		}
		// Stopping at a bad node: the good nodes before it are delivered,
		// as a line-by-line parser would have delivered them.
		return errors.Join(err, p.flushOnStop(ctx))
	}
	if err := sc.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return fmt.Errorf("%w: line %d is longer than %d bytes", ErrNodeTooLong, line+1, maxSize)
		}
		return fmt.Errorf("jsonld: reading input: %w", err)
	}
	return p.flushOnStop(ctx)
}

// flushOnStop delivers what the parser holds, naming a store failure as one.
func (p *NodeParser) flushOnStop(ctx context.Context) error {
	if err := p.Flush(ctx); err != nil {
		var he handlerError
		if errors.As(err, &he) {
			return fmt.Errorf("jsonld: delivering the last batch: %w", he.err)
		}
		return fmt.Errorf("jsonld: %w", err)
	}
	return nil
}
