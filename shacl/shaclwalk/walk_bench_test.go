package shaclwalk_test

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/shacl/shaclwalk"
)

func BenchmarkWalk(b *testing.B) {
	data := shacl.NewGraph()
	for i := range 200 {
		product := iri(fmt.Sprintf("product%d", i))
		data.Add(product, shacl.IRI(shacl.RDFType), iri("Product"))
		data.Add(product, iri("language"), iri("sharedMap"))
	}
	shapes := graph(b, `
ex:Root a sh:NodeShape ; sh:targetClass ex:Product ;
    sh:property [ sh:path ex:language ; sh:node [ sh:property ex:Name ] ] .
ex:Name sh:path ex:nl ; sh:minCount 1 .
`)
	for _, prepareEachTime := range []bool{false, true} {
		name := "prepared"
		if prepareEachTime {
			name = "prepare_and_walk"
		}
		b.Run(name, func(b *testing.B) {
			prepared := shacl.Prepare(data, shapes)
			var count int
			visit := func(event shaclwalk.Event) {
				if event.Phase == shaclwalk.Enter && event.Shape == iri("Name") {
					count++
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				count = 0
				if prepareEachTime {
					prepared = shacl.Prepare(data, shapes)
				}
				shaclwalk.Walk(prepared, visit)
			}
			if count != 200 {
				b.Fatalf("applications = %d, want 200", count)
			}
		})
	}
}
