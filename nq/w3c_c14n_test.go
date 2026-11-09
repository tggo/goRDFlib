package nq_test

import (
	"bytes"
	"os"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/testdata/w3c"
)

const nq12C14NManifest = "../testdata/w3c/rdf-tests/rdf/rdf12/rdf-n-quads/c14n/manifest.ttl"

// TestW3CNQuads12C14N: stream the action through a canonical Writer, in
// document order with the source blank node labels, and expect exactly the
// result file.
func TestW3CNQuads12C14N(t *testing.T) {
	m, err := w3c.ParseManifest(nq12C14NManifest)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	for _, e := range m.Entries {
		t.Run(e.Name, func(t *testing.T) {
			if e.Type != w3c.RDFT+"TestNQuadsPositiveC14N" {
				t.Fatalf("unsupported test type %s: add a case for it; a runner must not drop a test silently", e.Type)
			}
			in, err := os.Open(e.Action)
			if err != nil {
				t.Fatal(err)
			}
			defer in.Close()
			var out bytes.Buffer
			w := nq.NewWriter(&out, nq.WithCanonical())
			err = nq.ParseStream(in, func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, g rdflibgo.Term) error {
				return w.Write(s, p, o, g)
			}, nq.WithPreserveBlankNodeIDs())
			if err != nil {
				t.Fatalf("parse action: %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(e.Result)
			if err != nil {
				t.Fatal(err)
			}
			if out.String() != string(want) {
				t.Errorf("got  %q\nwant %q", out.String(), want)
			}
		})
	}
}
