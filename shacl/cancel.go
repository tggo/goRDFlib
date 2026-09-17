package shacl

import (
	"context"
	"errors"
	"fmt"
)

// Contexts (issue #35).
//
// ValidateContext, PrepareContext, Prepared.ValidateContext and
// ApplyRulesContext take a context.Context. It does two things:
//
//   - It reaches the stores. SPARQL constraints, SPARQL targets, sh:values,
//     rules and SHACL functions run their queries with sparql.EvalQueryContext,
//     and reading a graph built with NewGraphFromRDF goes through
//     store.ContextBinder, so an instrumented backend sees the caller's span.
//   - It stops the work. The context is checked before every focus node and
//     every rule, and a stopped run returns ErrCancelled together with
//     ctx.Err(), never a partial report: a report that is missing results
//     looks like a graph that conforms.
//
// The context travels with the run's recursion guard, because that is the one
// object every context derived during a validation already shares (see
// evalContext.guard). The functions without a context use context.Background.

// ErrCancelled is returned, wrapped together with the context's own error, when
// validation or rule application stops because its context is done. Both
// errors.Is(err, ErrCancelled) and errors.Is(err, context.Canceled) (or
// context.DeadlineExceeded) hold for it.
var ErrCancelled = errors.New("shacl: stopped because the context is done")

// stopped returns the error a run reports once ctx is done, or nil.
func stopped(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrCancelled, err)
	}
	return nil
}

func orBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// buildIndexes reads the graphs into their indexes with ctx bound to their
// stores. Everything after it reads the indexes, so this is the one full scan a
// store-backed graph gets, and it has to carry the caller's context.
func buildIndexes(ctx context.Context, graphs ...*Graph) error {
	for _, g := range graphs {
		if g != nil {
			g.ensureIndexesContext(ctx)
		}
	}
	return stopped(ctx)
}

// goContext returns the context the validation runs with.
func (ctx *evalContext) goContext() context.Context {
	if ctx == nil || ctx.guard == nil {
		return context.Background()
	}
	return orBackground(ctx.guard.ctx)
}

// goContext returns the context of the validation the expression runs in.
func (ctx *nodeExprContext) goContext() context.Context {
	if ctx == nil || ctx.guard == nil {
		return context.Background()
	}
	return orBackground(ctx.guard.ctx)
}
