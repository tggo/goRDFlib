package shacl

import (
	"errors"
	"fmt"
)

// reportMalformed sends a query failure of constraint c to WithErrorHandler
// as ErrMalformedConstraint, once per run. An evaluation failure
// (ErrQueryFailed) is not a malformed constraint: the run records it on the
// guard and fails instead.
func (ctx *evalContext) reportMalformed(c any, node Term, err error) {
	if errors.Is(err, ErrQueryFailed) || ctx.guard == nil || ctx.cfg == nil {
		return
	}
	if _, seen := ctx.guard.malformed[c]; seen {
		return
	}
	if ctx.guard.malformed == nil {
		ctx.guard.malformed = make(map[any]struct{})
	}
	ctx.guard.malformed[c] = struct{}{}
	ctx.cfg.report(fmt.Errorf("%w: %s: %w", ErrMalformedConstraint, node, err))
}
