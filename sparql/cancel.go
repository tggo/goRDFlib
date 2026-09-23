package sparql

import (
	"context"
	"errors"
	"fmt"
	"slices"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Cancellation.
//
// QueryContext, EvalQueryContext, UpdateContext and EvalUpdateContext stop
// evaluating once their context is done. The evaluator does not return errors
// from its inner functions, so cancellation travels as state instead: every
// evaluation carries an *evalCtx, hot loops call its stop method, and once stop
// has reported true every loop above it unwinds with whatever it has. The
// public entry point then discards that partial result and returns the
// context's error.
//
// Polling a context is a channel select. stop does it once every
// cancelCheckInterval calls, and not at all for a context that can never be
// done (context.Background, which the functions without a context use), so the
// check costs a counter increment per row where it is enabled and a nil
// comparison where it is not.

// ErrQueryCancelled is returned, wrapped together with the context's own
// error, when a query or update stops because its context is done. Both
// errors.Is(err, ErrQueryCancelled) and errors.Is(err, context.Canceled) (or
// context.DeadlineExceeded) hold for such an error.
var ErrQueryCancelled = errors.New("sparql: evaluation stopped because the context is done")

// cancelCheckInterval is how many stop calls pass between two polls of the
// context. A row costs at least a few hundred nanoseconds, so 1024 rows keep
// the reaction to a cancellation around a millisecond.
const cancelCheckInterval = 1024

// evalCtx is the cancellation state of one query or update evaluation. A nil
// *evalCtx never stops; internal callers and tests that have no context pass
// nil.
//
// It is not safe for concurrent use: one evaluation runs on one goroutine.
type evalCtx struct {
	ctx context.Context
	// done is ctx.Done(), nil when ctx can never be done.
	done <-chan struct{}
	n    uint32
	// err is the context's error once stop has reported true. It is sticky.
	err error
	// bound caches graphs with ctx bound to their store; see bind.
	bound map[*rdflibgo.Graph]*rdflibgo.Graph
}

func newEvalCtx(ctx context.Context) *evalCtx {
	if ctx == nil {
		ctx = context.Background()
	}
	return &evalCtx{ctx: ctx, done: ctx.Done()}
}

// stop counts one unit of work and reports whether evaluation must end.
func (e *evalCtx) stop() bool {
	if e == nil || e.done == nil {
		return false
	}
	if e.err != nil {
		return true
	}
	e.n++
	if e.n%cancelCheckInterval != 0 {
		return false
	}
	return e.poll()
}

// poll checks the context now, without counting.
func (e *evalCtx) poll() bool {
	if e == nil || e.done == nil {
		return false
	}
	if e.err != nil {
		return true
	}
	select {
	case <-e.done:
		e.err = e.ctx.Err()
		return true
	default:
		return false
	}
}

// cancellable reports whether the context can ever be done.
func (e *evalCtx) cancellable() bool { return e != nil && e.done != nil }

// failure returns the error a public entry point reports, or nil when
// evaluation was not stopped.
func (e *evalCtx) failure() error {
	if e == nil || e.err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrQueryCancelled, e.err)
}

// sortAborted is the panic value sortSolutions uses to leave slices.SortFunc,
// which has no other way to stop early.
type sortAborted struct{}

// sortSolutions sorts solutions with cmp, and gives up when ec stops. Without
// that, ORDER BY over a large result would keep evaluating its expressions for
// n·log n comparisons after the cancellation. The order of an aborted sort is
// unspecified; the caller discards the result.
func sortSolutions(ec *evalCtx, solutions []map[string]rdflibgo.Term, cmp func(a, b map[string]rdflibgo.Term) int) {
	if !ec.cancellable() {
		slices.SortFunc(solutions, cmp)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(sortAborted); !ok {
				panic(r)
			}
		}
	}()
	slices.SortFunc(solutions, func(a, b map[string]rdflibgo.Term) int {
		if ec.stop() {
			panic(sortAborted{})
		}
		return cmp(a, b)
	})
}

// EvalQueryContext evaluates a parsed SPARQL query against a graph, and stops
// once ctx is done.
//
// A stopped evaluation returns a nil *Result and an error that matches
// ErrQueryCancelled and ctx.Err() under errors.Is. An evaluation that finished
// before it noticed the cancellation returns its complete result. The context
// is polled every few hundred to few thousand units of work (rows, triples
// matched, path steps, sort comparisons), so a cancellation is noticed within
// about a millisecond of CPU time. Two things are not interrupted: a single
// call into a store (for example a transitive path pushed down to a
// store.ReachabilityStore, or a COUNT pushed down to a store.QueryableStore),
// and a single call to an extension function registered without a context
// (Function; a ContextFunction receives ctx).
//
// A store that implements store.ContextBinder is bound to ctx for the whole
// evaluation, so its calls carry ctx: a tracing span, a deadline, credentials.
//
// Like EvalQuery, it never mutates q.
func EvalQueryContext(ctx context.Context, g *rdflibgo.Graph, q *ParsedQuery, initBindings map[string]rdflibgo.Term) (*Result, error) {
	ec := newEvalCtx(ctx)
	if ec.poll() {
		return nil, ec.failure()
	}
	g, q = ec.bindQuery(g, q)
	g, q, release := snapshotQueryGraphs(g, q)
	defer release()
	// After binding and snapshotting, never before: a FROM clause merges the
	// graphs it names, and that read must go through the bound view and the
	// store's read snapshot like every other read the query makes.
	g, q, derr := applyDatasetClause(ec, g, q)
	if derr != nil {
		return nil, derr
	}
	res, err := evalQuery(ec, g, q, initBindings)
	if len(ec.bound) > 0 {
		// A bound store sees the cancellation too, and Store cannot report
		// errors: a lookup it abandoned looks like a lookup that matched
		// nothing. So a query that used one does not trust a result finished
		// after ctx was done, even if evaluation never polled in between.
		ec.poll()
	}
	if ferr := ec.failure(); ferr != nil {
		return nil, ferr
	}
	return res, err
}

// context returns the evaluation's context, Background for a nil evalCtx.
func (e *evalCtx) context() context.Context {
	if e == nil {
		return context.Background()
	}
	return e.ctx
}
