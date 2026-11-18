package sparql

// Switches for the evaluation shortcuts that must never change an answer:
// merging a join of BGPs, splitting a BGP into disjoint groups, substituting
// the left row into the right side of a pre-bound join, probing property
// paths when ordering a BGP, evaluating the right side of a join once or
// with the left row substituted (joinSide), and evaluating a pre-bound GRAPH
// pattern with the pre-bound values in the graphs they select
// (evalGraphPatternPreBound). Tests turn them off
// to compare an optimised evaluation with the plain one
// (TestOptimisationsDoNotChangeAnswers); nothing else writes them, so they are
// read without synchronisation and must only be changed by a test that runs
// none in parallel.
var (
	offMergeJoinedBGPs   bool
	offBGPComponents     bool
	offPreBoundJoinSubst bool
	offPathProbes        bool
	offJoinRightSide     bool
	offGraphPreBound     bool
)

// graphPreBoundHook, when set, is called for every GRAPH pattern evaluated on
// the pre-bound path (evalGraphPatternPreBound), so a test can prove the
// queries it compares take that path. Like the hooks below, it is nil
// otherwise, and only a test that runs none in parallel may set it.
var graphPreBoundHook func()

// pathProbeHook, when set, is called for every path probe the BGP planner
// runs. Tests use it to prove the probes they compare actually happen; it is
// nil otherwise, and like the switches above only a test that runs none in
// parallel may set it.
var pathProbeHook func()

// joinRightSideHook, when set, is called for every join whose right side is
// evaluated once or with the left row substituted instead of in full per row
// (joinSide). Like pathProbeHook, it is nil otherwise, and only a test that
// runs none in parallel may set it.
var joinRightSideHook func()

// optionalRightSideHook is joinRightSideHook for the right side of an
// OPTIONAL (optionalSide), so a test can prove OPTIONALs take that path too,
// with and without initial bindings.
var optionalRightSideHook func(preBound bool)
