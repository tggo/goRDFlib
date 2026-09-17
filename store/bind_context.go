package store

import "context"

// ContextBinder is an optional interface for stores whose calls can carry a
// context.Context: a tracing span, a deadline, request-scoped credentials
// (issue #35). Store's methods take no context.Context, and adding one would
// break every backend, including the ones outside this module. So a context
// is attached to a view of the store instead.
//
// BindContext returns a Store over the same data whose calls all run with ctx.
// A write through the view is visible through the receiver and the other way
// round. The view is used for one operation (a query, an update, a
// validation), and it is as safe for concurrent use as the receiver.
//
// The view should implement the same optional interfaces as the receiver
// (QueryableStore, SnapshotStore, CardinalityStore, ReachabilityStore): the
// engines look for them on the store they are handed, so a view that drops one
// silently loses that optimisation.
//
// sparql.EvalQueryContext, sparql.EvalUpdateContext, paths.EvalContext,
// shacl.ValidateContext and reasoning.ExpandContext bind their context to the
// stores they read and write through this interface.
type ContextBinder interface {
	BindContext(ctx context.Context) Store
}

// BindContext returns st bound to ctx when st implements ContextBinder, and st
// itself otherwise. A nil ctx leaves st unbound.
func BindContext(ctx context.Context, st Store) Store {
	if ctx == nil {
		return st
	}
	if b, ok := st.(ContextBinder); ok {
		return b.BindContext(ctx)
	}
	return st
}
