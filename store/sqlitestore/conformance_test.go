package sqlitestore_test

import (
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/sqlitestore"
	"github.com/tggo/goRDFlib/store/storetest"
)

func TestConformance(t *testing.T) {
	// Each subtest gets its own file so that a shared in-memory cache cannot
	// leak state between them, and so Reopen has something to reopen.
	paths := map[store.Store]string{}

	storetest.Run(t, storetest.Config{
		New: func(t *testing.T) store.Store {
			path := t.TempDir() + "/conformance.db"
			s, err := sqlitestore.New(sqlitestore.WithFile(path))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			t.Cleanup(func() { s.Close() })
			paths[s] = path
			return s
		},
		Reopen: func(t *testing.T, s store.Store) store.Store {
			path := paths[s]
			if err := s.(*sqlitestore.SQLiteStore).Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			reopened, err := sqlitestore.New(sqlitestore.WithFile(path))
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			t.Cleanup(func() { reopened.Close() })
			return reopened
		},
	})
}
