package shacl

import (
	"context"
	"testing"

	"github.com/tggo/goRDFlib/graph"
)

func rgLen(t testing.TB, g *graph.Graph) int {
	t.Helper()
	n, err := g.Len(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}
