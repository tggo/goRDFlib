package sqlitestore

import (
	"github.com/tggo/goRDFlib/plugin"
	"github.com/tggo/goRDFlib/store"
)

// init registers the "sqlite" store type with the plugin registry.
// The factory creates an in-memory SQLiteStore by default, since file paths
// cannot be passed through the plugin interface. For persistent storage,
// use New(WithFile(...)) directly.
func init() {
	plugin.RegisterStore("sqlite", func() store.Store {
		s, err := New(WithInMemory())
		if err != nil {
			panic("sqlitestore: " + err.Error())
		}
		return s
	})
}
