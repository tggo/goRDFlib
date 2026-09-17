// Package shaclwalk discovers structural shape applications on a prepared SHACL
// run. It follows sh:property, sh:node, sh:and, sh:or, sh:xone, and sh:not without
// using constraint results to choose branches. It does not calculate form state.
package shaclwalk

import (
	"slices"

	"github.com/tggo/goRDFlib/shacl"
)

// Walk emits balanced enter/leave events, including for properties without values.
// It skips deactivated shapes and does not enter rule conditions, target definitions,
// or other shape-valued parameters. Targets and navigation use the prepared engine;
// walking does not rerun rules or execute constraints to choose structural branches.
// Re-entering an active (shape, focus) pair emits Cycle instead of descending;
// completed applications may be revisited through other branches or origins.
// The prepared run is not safe for concurrent use. A nil visitor does no work.
func Walk(prepared *shacl.Prepared, visit func(Event)) {
	if visit == nil {
		return
	}
	var stack []Event
	active := make(map[[2]shacl.Term]struct{})
	for shape := range prepared.Shapes() {
		for _, focus := range prepared.Targets(shape.ID) {
			stack = append(stack, Event{SelectedShape: shape.ID, SelectedFocus: focus, Shape: shape.ID, Focus: focus})
			for len(stack) > 0 {
				event := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				key := [2]shacl.Term{event.Shape, event.Focus}
				if event.Phase == Leave {
					delete(active, key)
					visit(event)
					continue
				}
				if _, exists := active[key]; exists {
					event.Phase = Cycle
					visit(event)
					continue
				}
				current, ok := prepared.Shape(event.Shape)
				if !ok || current.Deactivated {
					continue
				}
				active[key] = struct{}{}
				visit(event)
				leave := event
				leave.Phase = Leave
				stack = append(stack, leave)
				start := len(stack)
				var values []shacl.Term
				selected := false
				for via, child := range prepared.ShapeLinks(event.Shape) {
					info, ok := prepared.Shape(child)
					if !ok || info.Deactivated {
						continue
					}
					if !selected {
						values = prepared.ValueNodes(event.Shape, event.Focus)
						selected = true
					}
					for _, value := range values {
						stack = append(stack, Event{
							SelectedShape: event.SelectedShape, SelectedFocus: event.SelectedFocus,
							Shape: child, Focus: value, Via: via,
						})
					}
				}
				slices.Reverse(stack[start:])
			}
		}
	}
}
