package bnodes

import (
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

func TestScopeIdentity(t *testing.T) {
	a, b := New(false), New(false)
	for _, label := range []string{"same", "a_b", "é", "N00000000000000000000000000000000", "g.label"} {
		if !a.Label(label).Equal(a.Label(label)) {
			t.Fatalf("unstable label %q", label)
		}
		if a.Label(label).Equal(b.Label(label)) || a.Label(label).Equal(term.NewBNode(label)) {
			t.Fatalf("unscoped label %q", label)
		}
		if !New(true).Label(label).Equal(term.NewBNode(label)) {
			t.Fatalf("preservation changed %q", label)
		}
	}
	if a.Label("a_b").Equal(a.Label("ab")) {
		t.Fatal("distinct labels collided")
	}
}

// TestScopeLabelDoesNotGrow: feeding a scoped identity back in as a label must
// yield an identity of the same size, or labels grow with every parse →
// serialize → parse hop.
func TestScopeLabelDoesNotGrow(t *testing.T) {
	label := "b1"
	width := len(term.NewBNode().Value())
	for hop := 0; hop < 3; hop++ {
		label = New(false).Label(label).Value()
		if len(label) != width {
			t.Fatalf("hop %d: label %q has %d bytes, want the fixed width %d", hop, label, len(label), width)
		}
	}
}

func TestScopeConcurrentParses(t *testing.T) {
	const count = 64
	nodes := make(chan term.BNode, count)
	var wg sync.WaitGroup
	for range count {
		wg.Go(func() { nodes <- New(false).Label("same") })
	}
	wg.Wait()
	close(nodes)
	seen := make(map[term.BNode]bool, count)
	for node := range nodes {
		if seen[node] {
			t.Fatal("concurrent scopes collided")
		}
		seen[node] = true
	}
}

func BenchmarkScopeLabel(b *testing.B) {
	scope := New(false)
	b.ReportAllocs()
	for b.Loop() {
		_ = scope.Label("same")
	}
}
