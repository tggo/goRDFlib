package sparql

import (
	"context"
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/turtle"
)

// joinBenchGraph has 2,000 operations, each selecting one of 2,000 fields.
// Even fields come from one of 10 subgraphs, and every field is in one schema;
// subgraphs and the schema are each part of a system.
func joinBenchGraph(b *testing.B) *rdflibgo.Graph {
	var sb strings.Builder
	sb.WriteString("@prefix ex: <http://example.org/> .\n")
	const n = 2000
	for i := range n {
		fmt.Fprintf(&sb, "ex:op%d ex:selects ex:field%d .\n", i, i)
		fmt.Fprintf(&sb, "ex:field%d ex:inSchema ex:schema .\n", i)
		if i%2 == 0 {
			fmt.Fprintf(&sb, "ex:field%d ex:fromSubgraph ex:subgraph%d .\n", i, i%10)
		}
	}
	for k := range 10 {
		fmt.Fprintf(&sb, "ex:subgraph%d ex:partOf ex:system%d .\n", k, k)
	}
	sb.WriteString("ex:schema ex:partOf ex:gateway .\n")
	g := rdflibgo.NewGraph()
	if err := turtle.Parse(context.Background(), g, strings.NewReader(sb.String())); err != nil {
		b.Fatal(err)
	}
	return g
}

// benchmarkJoin runs q with the join shortcuts (joinSide) on and off.
func benchmarkJoin(b *testing.B, q string, rows int) {
	g := joinBenchGraph(b)
	defer func() { offJoinRightSide = false }()
	for _, off := range []bool{false, true} {
		name := "on"
		if off {
			name = "off"
		}
		b.Run(name, func(b *testing.B) {
			offJoinRightSide = off
			for range b.N {
				res, err := Query(g, "PREFIX ex: <http://example.org/>\n"+q)
				if err != nil {
					b.Fatal(err)
				}
				if len(res.Bindings) != rows {
					b.Fatalf("%d rows, want %d", len(res.Bindings), rows)
				}
			}
		})
	}
}

// An expression-free UNION on the right takes each operation's field by
// substitution.
func BenchmarkJoinWithUnion(b *testing.B) {
	benchmarkJoin(b, `SELECT ?op ?server WHERE {
  ?op ex:selects ?field .
  { ?field ex:fromSubgraph ?subgraph . ?subgraph ex:partOf ?server }
  UNION
  { ?field ex:inSchema ?schema . ?schema ex:partOf ?server }
}`, 3000)
}

// A group with a FILTER on the right is evaluated once and joined with every
// operation.
func BenchmarkJoinWithFilteredGroup(b *testing.B) {
	benchmarkJoin(b, `SELECT ?op ?server WHERE {
  ?op ex:selects ?field .
  { ?field ex:inSchema ?schema . ?schema ex:partOf ?server FILTER NOT EXISTS { ?field ex:fromSubgraph ?any } }
}`, 1000)
}
