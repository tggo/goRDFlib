package shaclwalk

// Phase distinguishes entering, leaving, and an edge to an active application.
// Cycle has no matching Enter or Leave and does not assert conformance.
// Phase values are safe for concurrent use when not modified.
type Phase uint8

const (
	Enter Phase = iota
	Leave
	Cycle
)
