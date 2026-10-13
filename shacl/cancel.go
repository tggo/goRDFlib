package shacl

import (
	"context"
	"errors"
	"fmt"

	"github.com/tggo/goRDFlib/graph"
)

// Contexts (issue #35).
//
// ValidateContext, PrepareContext, Prepared.ValidateContext and
// ApplyRulesContext take a context.Context. It does two things:
//
//   - It reaches the stores. SPARQL constraints, SPARQL targets, sh:values,
//     rules and SHACL functions run their queries with sparql.EvalQueryContext,
//     and a graph built with NewGraphFromRDF is read with the caller's
//     context, so an instrumented backend sees the caller's span.
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

// buildIndexes reads the graphs into their indexes with ctx. Everything after
// it reads the indexes, so this is the one full scan a store-backed graph gets,
// it has to carry the caller's context, and a store error here is the error of
// the run: a graph that could not be read must never validate as empty.
func buildIndexes(ctx context.Context, graphs ...*Graph) error {
	for _, g := range graphs {
		if g != nil {
			if _, err := g.ensureIndexesContext(ctx); err != nil {
				return err
			}
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

// ErrQueryFailed is wrapped around an error the SPARQL engine returned while
// evaluating a query that parsed. For a graph backed by a store this is how a
// failing store reaches the caller: validation returns it, never a report,
// because a constraint that could not be evaluated must not look satisfied.
var ErrQueryFailed = errors.New("shacl: a SPARQL query could not be evaluated")

// fail records err as the failure of the run when it is a query evaluation
// error; the first one wins. Parse errors are not recorded: they are findings
// about the shapes graph and are reported as before.
func (g *recursionGuard) fail(err error) {
	if g != nil && g.err == nil && errors.Is(err, ErrQueryFailed) {
		g.err = err
	}
}

// finished is the check a run makes before it trusts what it computed: the
// context is still live and no query failed on the way.
func finished(goctx context.Context, ctx *evalContext) error {
	if err := stopped(goctx); err != nil {
		return err
	}
	if ctx.guard != nil && ctx.guard.err != nil {
		return ctx.guard.err
	}
	return nil
}

// rdfOf returns the rdflib graph behind g for a query, with ctx.
func rdfOf(ctx context.Context, g *Graph) (*graph.Graph, error) {
	rg, err := g.rdf(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}
	return rg, nil
}
