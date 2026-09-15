package paths

import (
	"context"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// stopCheckInterval is how many traversal steps pass between two polls of the
// context. Polling on every step costs a channel select per triple; a step is
// a few hundred nanoseconds, so 1024 steps keep the reaction to a cancellation
// well under a millisecond.
const stopCheckInterval = 1024

// stopper tells a traversal when its context is done. A nil *stopper never
// stops, which is what Eval passes, so the uncancellable path pays one nil
// check per step.
//
// A stopper is not safe for concurrent use; one traversal runs on one
// goroutine.
type stopper struct {
	done    <-chan struct{}
	n       uint32
	stopped bool
}

// stop counts one traversal step and reports whether the traversal must end.
// Once it has reported true it keeps doing so, which is what lets every level
// of a nested traversal unwind without polling the context again.
func (s *stopper) stop() bool {
	if s == nil {
		return false
	}
	if s.stopped {
		return true
	}
	s.n++
	if s.n%stopCheckInterval != 0 {
		return false
	}
	return s.poll()
}

// halted reports whether the traversal has already been stopped, without
// counting a step.
func (s *stopper) halted() bool { return s != nil && s.stopped }

func (s *stopper) poll() bool {
	select {
	case <-s.done:
		s.stopped = true
	default:
	}
	return s.stopped
}

// EvalContext evaluates p like p.Eval, and stops the traversal once ctx is
// done. The context is polled every stopCheckInterval steps, including steps
// that emit nothing, such as a repetition walking a long chain towards an
// object that is not on it.
//
// The iterator has no error result. After it returns, a nil ctx.Err()
// guarantees the pairs yielded were the complete answer; a non-nil one means
// they may be a prefix of it.
//
// A transitive repetition that is pushed down to a store.ReachabilityStore
// runs as one store call, which has no context and is not interrupted; the
// pairs it returns are still emitted with cancellation checks.
func EvalContext(ctx context.Context, p Path, g *graph.Graph, subj term.Subject, obj term.Term) func(yield func(term.Term, term.Term) bool) {
	done := ctx.Done()
	if done == nil {
		return p.Eval(g, subj, obj)
	}
	return func(yield func(term.Term, term.Term) bool) {
		st := &stopper{done: done}
		if st.poll() {
			return
		}
		p.eval(st, g, subj, obj)(yield)
	}
}
