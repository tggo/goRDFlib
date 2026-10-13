package reasoning

import (
	"context"
	"errors"
	"fmt"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// ErrCancelled is returned, wrapped together with the context's own error,
// when ExpandContext or ExpandCheckContext stops because its context is done.
// Both errors.Is(err, ErrCancelled) and errors.Is(err, context.Canceled) (or
// context.DeadlineExceeded) hold for it.
var ErrCancelled = errors.New("reasoning: stopped because the context is done")

// stopCheckInterval is how many triples a rule pass scans between two polls of
// the context.
const stopCheckInterval = 1024

// stopper tells a closure when it must end: its context is done, or the store
// failed. It also owns every store access of an engine, so the first failure
// is recorded in one place and every loop unwinds through halted. Not safe for
// concurrent use.
type stopper struct {
	ctx  context.Context
	done <-chan struct{}
	n    uint32
	err  error
}

func newStopper(ctx context.Context) *stopper {
	if ctx == nil {
		ctx = context.Background()
	}
	return &stopper{ctx: ctx, done: ctx.Done()}
}

// fail records the first failure of a store call. A failure caused by the
// context is reported like any other stop, as ErrCancelled.
func (s *stopper) fail(op string, err error) {
	if s.err != nil {
		return
	}
	if cerr := s.ctx.Err(); cerr != nil {
		s.err = fmt.Errorf("%w: %w", ErrCancelled, cerr)
		return
	}
	s.err = fmt.Errorf("reasoning: %s: %w", op, err)
}

// triples calls fn for each triple matching the pattern until fn returns
// false, the closure is stopped, or the store reports an error (recorded).
func (s *stopper) triples(g *graph.Graph, sub term.Subject, p *term.URIRef, o term.Term, fn func(term.Triple) bool) {
	if s.err != nil {
		return
	}
	for t, err := range g.Triples(s.ctx, sub, p, o) {
		if err != nil {
			s.fail("reading the graph", err)
			return
		}
		if !fn(t) {
			return
		}
	}
}

// contains reports whether the graph holds the triple; false once stopped.
func (s *stopper) contains(g *graph.Graph, sub term.Subject, p term.URIRef, o term.Term) bool {
	if s.err != nil {
		return false
	}
	ok, err := g.Contains(s.ctx, sub, p, o)
	if err != nil {
		s.fail("reading the graph", err)
		return false
	}
	return ok
}

// add writes a triple to the graph; after a failure nothing is written.
func (s *stopper) add(g *graph.Graph, sub term.Subject, p term.URIRef, o term.Term) {
	if s.err != nil {
		return
	}
	if err := g.Add(s.ctx, sub, p, o); err != nil {
		s.fail("adding an inferred triple", err)
	}
}

// length returns the graph size, 0 once stopped.
func (s *stopper) length(g *graph.Graph) int {
	if s.err != nil {
		return 0
	}
	n, err := g.Len(s.ctx)
	if err != nil {
		s.fail("counting triples", err)
		return 0
	}
	return n
}

// tick counts one triple scanned and reports whether the closure must end.
func (s *stopper) tick() bool {
	if s == nil {
		return false
	}
	if s.err != nil {
		return true
	}
	s.n++
	if s.n%stopCheckInterval != 0 {
		return false
	}
	return s.poll()
}

// poll checks the context now. Once it has reported true it keeps doing so.
func (s *stopper) poll() bool {
	if s == nil {
		return false
	}
	if s.err != nil {
		return true
	}
	select {
	case <-s.done:
		s.err = fmt.Errorf("%w: %w", ErrCancelled, s.ctx.Err())
		return true
	default:
		return false
	}
}

// halted reports whether the closure has been stopped, without counting.
func (s *stopper) halted() bool { return s != nil && s.err != nil }

// failure is the error a stopped closure reports, nil otherwise.
func (s *stopper) failure() error {
	if s == nil {
		return nil
	}
	return s.err
}

// ExpandContext is Expand with a context. ctx reaches every store read and
// write, and the closure stops once ctx is done: it is checked every few thousand triples scanned and before each
// batch of inferred triples is added.
//
// A store error ends the expansion the same way and is returned wrapped; it is
// never reported as success. A stopped expansion returns the number of triples added so far and an error
// matching ErrCancelled and ctx.Err(). The triples added stay in the graph.
// Each one is a valid entailment, but the graph is not closed.
func ExpandContext(ctx context.Context, g *graph.Graph, r Regime) (int, error) {
	n, _, err := ExpandCheckContext(ctx, g, r)
	return n, err
}
