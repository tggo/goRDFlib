package sparql

import "testing"

// bgpComponents runs for every BGP evaluation, often once per row of an
// enclosing pattern; a BGP that does not split must cost no allocation. The
// scratch arrays once moved to the heap (1.4 KiB per call) when the walk of
// triple terms was made a recursive method; the plain case stays separate.
func TestBGPComponentsConnectedAllocatesNothing(t *testing.T) {
	tp := func(s, p, o string) Triple { return Triple{Subject: s, Predicate: p, Object: o} }
	triples := []Triple{tp("?a", "<p>", "?b"), tp("?b", "<p>", "?c"), tp("?c", "<q>", "<o>")}
	if allocs := testing.AllocsPerRun(200, func() {
		if bgpComponents(triples, nil) != nil {
			t.Fatal("split")
		}
	}); allocs != 0 {
		t.Errorf("%v allocations for a connected BGP, want 0", allocs)
	}
}
