package sparql

// setOpts turns the optimisations (opt_toggle.go) on or off together.
func setOpts(off bool) {
	offMergeJoinedBGPs, offBGPComponents, offPreBoundJoinSubst, offPathProbes, offJoinRightSide, offGraphPreBound = off, off, off, off, off, off
}
