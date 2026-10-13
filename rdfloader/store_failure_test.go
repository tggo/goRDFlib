package rdfloader_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/rdfloader"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
)

// TestLoadReturnsStoreError: whichever parser the format dispatches to, a
// failing store is Load's error rather than a silently empty graph.
func TestLoadReturnsStoreError(t *testing.T) {
	docs := map[string]string{
		"d.ttl":    "<http://e/s> <http://e/p> <http://e/o> .\n",
		"d.nt":     "<http://e/s> <http://e/p> <http://e/o> .\n",
		"d.nq":     "<http://e/s> <http://e/p> <http://e/o> <http://e/g> .\n",
		"d.trig":   "<http://e/g> { <http://e/s> <http://e/p> <http://e/o> . }\n",
		"d.rdf":    `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="http://e/s"><rdf:value rdf:resource="http://e/o"/></rdf:Description></rdf:RDF>`,
		"d.jsonld": `{"@id":"http://e/s","http://e/p":{"@id":"http://e/o"}}`,
	}
	for name, content := range docs {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			fs := storetest.NewFailingStore(store.NewMemoryStore())
			fs.Fail(nil)
			g := graph.NewGraphFromStore(fs, nil)
			err := rdfloader.DefaultLoader().Load(context.Background(), g, "file://"+path)
			if !errors.Is(err, storetest.ErrInjected) {
				t.Fatalf("Load err = %v, want storetest.ErrInjected", err)
			}
		})
	}
}
