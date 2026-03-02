package shacl

import (
	"fmt"
	"strings"
	"testing"
)

var (
	benchReport ValidationReport
	benchTerms2 []Term
	benchCmp    int
	benchCmpOk  bool
)

func BenchmarkValidate(b *testing.B) {
	type benchCase struct {
		name      string
		numShapes int
		numNodes  int
	}
	cases := []benchCase{
		{"small_5shapes_50nodes", 5, 50},
		{"medium_20shapes_500nodes", 20, 500},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			// Build shapes graph using Turtle
			var shapesTTL strings.Builder
			shapesTTL.WriteString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
`)
			for i := 0; i < tc.numShapes; i++ {
				fmt.Fprintf(&shapesTTL, `
ex:Shape%d a sh:NodeShape ;
    sh:targetClass ex:Type%d ;
    sh:nodeKind sh:IRI .
`, i, i)
			}
			shapesGraph, err := LoadTurtleString(shapesTTL.String(), "http://example.org/shapes")
			if err != nil {
				b.Fatal(err)
			}

			// Build data graph programmatically for speed
			dataGraph := NewGraph()
			typePred := IRI(RDFType)
			for i := 0; i < tc.numShapes; i++ {
				cls := IRI(fmt.Sprintf("http://example.org/Type%d", i))
				nodesPerShape := tc.numNodes / tc.numShapes
				for j := 0; j < nodesPerShape; j++ {
					node := IRI(fmt.Sprintf("http://example.org/inst_%d_%d", i, j))
					dataGraph.Add(node, typePred, cls)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchReport = Validate(dataGraph, shapesGraph)
			}
		})
	}
}

func BenchmarkEvalPath_Predicate(b *testing.B) {
	g := NewGraph()
	pred := IRI(ex + "name")
	focus := IRI(ex + "node0")
	g.Add(focus, pred, Literal("Alice", "", ""))
	g.Add(focus, pred, Literal("Bob", "", ""))

	path := &PropertyPath{Kind: PathPredicate, Pred: pred}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchTerms2 = evalPath(g, path, focus)
	}
}

func BenchmarkEvalPath_TransitiveClose(b *testing.B) {
	for _, chainLen := range []int{5, 20, 100} {
		b.Run(fmt.Sprintf("chain_%d", chainLen), func(b *testing.B) {
			g := NewGraph()
			pred := IRI(ex + "next")
			for i := 0; i < chainLen; i++ {
				from := IRI(fmt.Sprintf("%snode%d", ex, i))
				to := IRI(fmt.Sprintf("%snode%d", ex, i+1))
				g.Add(from, pred, to)
			}

			path := &PropertyPath{
				Kind: PathZeroOrMore,
				Sub:  &PropertyPath{Kind: PathPredicate, Pred: pred},
			}
			focus := IRI(ex + "node0")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchTerms2 = evalPath(g, path, focus)
			}
		})
	}
}

func BenchmarkCompareLiterals(b *testing.B) {
	b.Run("numeric", func(b *testing.B) {
		a := Literal("42", XSD+"integer", "")
		c := Literal("100", XSD+"integer", "")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchCmp, benchCmpOk = compareLiterals(a, c)
		}
	})

	b.Run("date", func(b *testing.B) {
		a := Literal("2024-01-15", XSD+"date", "")
		c := Literal("2024-06-30", XSD+"date", "")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchCmp, benchCmpOk = compareLiterals(a, c)
		}
	})

	b.Run("string", func(b *testing.B) {
		a := Literal("apple", XSD+"string", "")
		c := Literal("banana", XSD+"string", "")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchCmp, benchCmpOk = compareLiterals(a, c)
		}
	})
}
