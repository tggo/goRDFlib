package paths

import (
	"context"
	"errors"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// stopCheckInterval is how many traversal steps pass between two polls of the
// context. Polling on every step costs a channel select per triple; a step is
// a few hundred nanoseconds, so 1024 steps keep the reaction to a cancellation
// well under a millisecond.
const stopCheckInterval = 1024

// stopper carries a traversal's context and its first failure: the context's
// error once it is done, or an error from the store. The internal eval
// functions yield plain (subject, object) pairs and have no error result, so
// failure travels as state: once stop reports true it keeps doing so, and
// every level of a nested traversal unwinds without polling again. Eval
// reports the error at the end.
//
// A stopper is not safe for concurrent use; one traversal runs on one
// goroutine.
type stopper struct {
	ctx  context.Context
	done <-chan struct{}
	n    uint32
	err  error
}

func newStopper(ctx context.Context) *stopper {
	return &stopper{ctx: ctx, done: ctx.Done()}
}

// stop counts one traversal step and reports whether the traversal must end.
func (s *stopper) stop() bool {
	if s.err != nil {
		return true
	}
	if s.done == nil {
		return false
	}
	s.n++
	if s.n%stopCheckInterval != 0 {
		return false
	}
	return s.poll()
}

// halted reports whether the traversal has already been stopped, without
// counting a step.
func (s *stopper) halted() bool { return s.err != nil }

// poll checks the context now.
func (s *stopper) poll() bool {
	if s.err != nil {
		return true
	}
	if s.done == nil {
		return false
	}
	select {
	case <-s.done:
		s.err = s.ctx.Err()
	default:
	}
	return s.err != nil
}

// fail records err as the traversal's failure, keeping the first one.
func (s *stopper) fail(err error) {
	if s.err == nil {
		s.err = err
	}
}

// triples yields the matching triples to yield, and records a store error on
// the stopper.
func (s *stopper) triples(g *graph.Graph, subj term.Subject, p *term.URIRef, obj term.Term, yield func(term.Triple) bool) {
	for t, err := range g.Triples(s.ctx, subj, p, obj) {
		if err != nil {
			s.fail(err)
			return
		}
		if s.stop() || !yield(t) {
			return
		}
	}
}

// run evaluates p with ctx and turns the traversal into a TermPairIterator:
// the pairs, then the failure, if any, as a final (zero, err) pair. The
// context is polled every stopCheckInterval steps, including steps that emit
// nothing, such as a repetition walking a long chain towards an object that is
// not on it.
func run(ctx context.Context, p Path, g *graph.Graph, subj term.Subject, obj term.Term) store.TermPairIterator {
	return func(yield func(store.TermPair, error) bool) {
		st := newStopper(ctx)
		if st.poll() {
			yield(store.TermPair{}, st.err)
			return
		}
		stopped := false
		p.eval(st, g, subj, obj)(func(s, o term.Term) bool {
			if !yield(store.TermPair{First: s, Second: o}, nil) {
				stopped = true
				return false
			}
			return true
		})
		if !stopped && st.err != nil {
			yield(store.TermPair{}, st.err)
		}
	}
}

// isContextErr reports whether err is a context's error, which a store
// returns when it was asked with a done context.
func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
