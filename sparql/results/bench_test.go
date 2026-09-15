package results_test

import (
	"io"
	"strconv"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
)

// benchResult is a 100k-row SELECT with an IRI, a language-tagged literal,
// a typed integer and a blank node per row (1000 distinct blank nodes).
func benchResult() *sparql.Result {
	const n = 100_000
	rows := make([]map[string]rdflibgo.Term, n)
	for i := range rows {
		rows[i] = row{
			"s":    iri("http://example.org/resource/" + strconv.Itoa(i)),
			"name": lit("Name number "+strconv.Itoa(i)+" with \"quotes\"", rdflibgo.WithLang("en")),
			"n":    typed(strconv.Itoa(i), xsd+"integer"),
			"b":    bnode("N" + strconv.Itoa(i%1000)),
		}
	}
	return selectResult([]string{"s", "name", "n", "b"}, rows...)
}

func BenchmarkWrite100k(b *testing.B) {
	r := benchResult()
	for _, f := range []results.Format{results.FormatJSON, results.FormatXML, results.FormatCSV, results.FormatTSV} {
		b.Run(f.String(), func(b *testing.B) {
			cw := &countingWriter{}
			b.ReportAllocs()
			for b.Loop() {
				if err := results.Write(cw, f, r); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(cw.writes)/float64(b.N), "writes/op")
		})
	}
}

// BenchmarkStreamRow is the per-row cost of the streaming writer.
func BenchmarkStreamRow(b *testing.B) {
	r := benchResult()
	for _, f := range []results.Format{results.FormatJSON, results.FormatTSV} {
		b.Run(f.String(), func(b *testing.B) {
			w, _ := results.NewWriter(io.Discard, f, r.Vars)
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				if err := w.WriteRow(r.Bindings[i%len(r.Bindings)]); err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	}
}
