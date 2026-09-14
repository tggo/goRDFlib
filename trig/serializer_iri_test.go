package trig

import (
	"bytes"
	"errors"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// The serializer must never write an IRIREF the TriG grammar forbids.
func TestSerializeDatasetRejectsUnwritableIRI(t *testing.T) {
	p := rdflibgo.NewURIRefUnsafe("http://e/p")
	bad := rdflibgo.NewURIRefUnsafe("http://e/a\tb")
	cases := map[string]func(ds *graph.Dataset){
		"subject": func(ds *graph.Dataset) { ds.DefaultContext().Add(bad, p, p) },
		"object":  func(ds *graph.Dataset) { ds.DefaultContext().Add(p, p, rdflibgo.NewURIRefUnsafe("http://e/a|b")) },
		"datatype": func(ds *graph.Dataset) {
			ds.DefaultContext().Add(p, p, rdflibgo.NewLiteral("v", rdflibgo.WithDatatype(bad)))
		},
		"triple term": func(ds *graph.Dataset) { ds.DefaultContext().Add(p, p, rdflibgo.NewTripleTerm(bad, p, p)) },
		"graph name":  func(ds *graph.Dataset) { ds.Graph(bad).Add(p, p, p) },
	}
	for name, add := range cases {
		ds := graph.NewDataset()
		add(ds)
		var b bytes.Buffer
		if err := SerializeDataset(ds, &b); !errors.Is(err, rdflibgo.ErrInvalidIRI) {
			t.Errorf("%s: err = %v, want ErrInvalidIRI; output:\n%s", name, err, b.String())
		}
	}
	ds := graph.NewDataset()
	ds.DefaultContext().Add(p, p, p)
	if err := SerializeDataset(ds, &bytes.Buffer{}, WithBase("http://e/\x01")); !errors.Is(err, rdflibgo.ErrInvalidIRI) {
		t.Errorf("base: err = %v, want ErrInvalidIRI", err)
	}
}
