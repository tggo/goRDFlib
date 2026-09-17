package reasoning

import (
	"context"
	"errors"
	"fmt"

	"github.com/tggo/goRDFlib/graph"
)

// ErrCancelled is returned, wrapped together with the context's own error,
// when ExpandContext or ExpandCheckContext stops because its context is done.
// Both errors.Is(err, ErrCancelled) and errors.Is(err, context.Canceled) (or
// context.DeadlineExceeded) hold for it.
var ErrCancelled = errors.New("reasoning: stopped because the context is done")

// stopCheckInterval is how many triples a rule pass scans between two polls of
// the context.
const stopCheckInterval = 1024

// stopper tells a closure when its context is done. A nil *stopper never stops;
// the functions without a context use one. Not safe for concurrent use.
type stopper struct {
	ctx  context.Context
	done <-chan struct{}
	n    uint32
	err  error
}

func newStopper(ctx context.Context) *stopper {
	if ctx == nil || ctx.Done() == nil {
		return nil
	}
	return &stopper{ctx: ctx, done: ctx.Done()}
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

// ExpandContext is Expand with a context. The graph is read and written with
// ctx bound to its store (store.ContextBinder), and the closure stops once ctx
// is done: it is checked every few thousand triples scanned and before each
// batch of inferred triples is added.
//
// A stopped expansion returns the number of triples added so far and an error
// matching ErrCancelled and ctx.Err(). The triples added stay in the graph.
// Each one is a valid entailment, but the graph is not closed.
func ExpandContext(ctx context.Context, g *graph.Graph, r Regime) (int, error) {
	n, _, err := ExpandCheckContext(ctx, g, r)
	return n, err
}
