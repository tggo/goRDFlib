package jsonld

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
)

const storeErrDoc = `{"@context":{"@vocab":"http://e/"},"@graph":[{"@id":"http://e/a","p":"1"},{"@id":"http://e/b","p":"2"}]}`

func TestParseReturnsStoreError(t *testing.T) {
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	fs.Fail(nil)
	err := Parse(t.Context(), graph.NewGraphFromStore(fs, nil), strings.NewReader(storeErrDoc))
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("Parse error = %v, want ErrInjected", err)
	}
}

func TestSerializeReturnsStoreErrorAndWritesNothing(t *testing.T) {
	mem := store.NewMemoryStore()
	if err := Parse(t.Context(), graph.NewGraphFromStore(mem, nil), strings.NewReader(storeErrDoc)); err != nil {
		t.Fatal(err)
	}
	for name, setup := range map[string]func(*storetest.FailingStore){
		"mid-read":  func(f *storetest.FailingStore) { f.FailReadsAfter(1, nil) },
		"all calls": func(f *storetest.FailingStore) { f.Fail(nil) },
	} {
		t.Run(name, func(t *testing.T) {
			fs := storetest.NewFailingStore(mem)
			setup(fs)
			var buf bytes.Buffer
			err := Serialize(t.Context(), graph.NewGraphFromStore(fs, nil), &buf)
			if !errors.Is(err, storetest.ErrInjected) {
				t.Fatalf("Serialize error = %v, want ErrInjected", err)
			}
			if buf.Len() != 0 {
				t.Errorf("Serialize wrote %q before failing", buf.String())
			}
		})
	}
}
