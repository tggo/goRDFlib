package store_test

import (
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
)

// MemoryStore is the reference against which every other backend is measured,
// so it runs the shared conformance suite first.
func TestConformance(t *testing.T) {
	storetest.Run(t, storetest.Config{
		New: func(t *testing.T) store.Store { return store.NewMemoryStore() },
	})
}
