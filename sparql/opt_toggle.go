package sparql

// Switches for the evaluation shortcuts that must never change an answer:
// merging a join of BGPs, splitting a BGP into disjoint groups, substituting
// the left row into the right side of a pre-bound join. Tests turn them off
// to compare an optimised evaluation with the plain one
// (TestOptimisationsDoNotChangeAnswers); nothing else writes them, so they are
// read without synchronisation and must only be changed by a test that runs
// none in parallel.
var (
	offMergeJoinedBGPs   bool
	offBGPComponents     bool
	offPreBoundJoinSubst bool
)
