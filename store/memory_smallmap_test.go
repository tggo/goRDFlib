package store

import (
	"math/rand/v2"
	"strconv"
	"testing"
)

// TestSmallMapMatchesMap drives a smallMap and a Go map with the same random
// operations across the slice → map promotion and checks they agree.
func TestSmallMapMatchesMap(t *testing.T) {
	r := rand.New(rand.NewPCG(53, 53))
	for round := range 200 {
		var sm smallMap[int]
		ref := make(map[string]int)
		keys := 1 + r.IntN(3*smallMapMax)
		for op := range 300 {
			k := strconv.Itoa(r.IntN(keys))
			switch r.IntN(3) {
			case 0, 1:
				sm.set(k, op)
				ref[k] = op
			default:
				sm.delete(k)
				delete(ref, k)
			}
			if sm.len() != len(ref) {
				t.Fatalf("round %d op %d: len %d, want %d", round, op, sm.len(), len(ref))
			}
		}
		for k, want := range ref {
			if got, ok := sm.get(k); !ok || got != want {
				t.Fatalf("round %d: get(%q) = %d,%v, want %d", round, k, got, ok, want)
			}
		}
		seen := 0
		for k, v := range sm.all() {
			if ref[k] != v {
				t.Fatalf("round %d: all yields %q=%d, want %d", round, k, v, ref[k])
			}
			seen++
		}
		if seen != len(ref) {
			t.Fatalf("round %d: all yielded %d entries, want %d", round, seen, len(ref))
		}
	}
}
