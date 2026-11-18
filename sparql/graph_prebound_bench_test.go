package sparql

import "testing"

// benchmarkExistsGraph runs q over 500 items and 500 named graphs with the
// pre-bound GRAPH shortcut (evalGraphPatternPreBound) on and off; it returns
// rows rows.
func benchmarkExistsGraph(b *testing.B, q string, rows int) {
	def, named, _ := existsGraphData(b, 500, 500)
	pq, err := Parse("PREFIX ex: <http://example.org/>\n" + q)
	if err != nil {
		b.Fatal(err)
	}
	pq.NamedGraphs = named
	defer func() { offGraphPreBound = false }()
	for _, off := range []bool{false, true} {
		name := "on"
		if off {
			name = "off"
		}
		b.Run(name, func(b *testing.B) {
			offGraphPreBound = off
			for range b.N {
				res, err := EvalQuery(def, pq, nil)
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

// The row binds the graph, so each EXISTS looks the item up in one graph.
func BenchmarkExistsGraphBound(b *testing.B) {
	benchmarkExistsGraph(b, `SELECT ?item WHERE { ?item ex:storedIn ?g . FILTER EXISTS { GRAPH ?g { ?item ex:label ?label } } }`, 500)
}

// The graph is unbound, so each EXISTS looks the item up in every graph.
func BenchmarkExistsGraphUnbound(b *testing.B) {
	benchmarkExistsGraph(b, `SELECT ?item WHERE { ?item ex:storedIn ?g . FILTER EXISTS { GRAPH ?h { ?item ex:label ?label } } }`, 500)
}

// The EXISTS body binds the graph before the GRAPH pattern, so the join
// substitutes each of the 50 special graphs into it.
func BenchmarkExistsGraphJoined(b *testing.B) {
	benchmarkExistsGraph(b, `SELECT ?item WHERE { ?item ex:storedIn ?g . FILTER EXISTS { ?h ex:kind ex:special . GRAPH ?h { ?item ex:label ?label } } }`, 50)
}
