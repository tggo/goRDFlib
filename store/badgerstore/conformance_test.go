package badgerstore_test

import (
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/badgerstore"
	"github.com/tggo/goRDFlib/store/storetest"
)

func TestConformance(t *testing.T) {
	dirs := map[store.Store]string{}

	storetest.Run(t, storetest.Config{
		New: func(t *testing.T) store.Store {
			dir := t.TempDir()
			s, err := badgerstore.New(badgerstore.WithDir(dir))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			t.Cleanup(func() { s.Close() })
			dirs[s] = dir
			return s
		},
		Reopen: func(t *testing.T, s store.Store) store.Store {
			dir := dirs[s]
			if err := s.(*badgerstore.BadgerStore).Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			reopened, err := badgerstore.New(badgerstore.WithDir(dir))
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			t.Cleanup(func() { reopened.Close() })
			return reopened
		},
	})
}
