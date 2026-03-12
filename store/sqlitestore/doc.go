// Package sqlitestore provides a persistent store.Store implementation backed
// by SQLite via modernc.org/sqlite (pure Go, zero CGo).
//
// Triples are stored in a single table with three indexes (SPO, POS, OSP) for
// efficient pattern matching. Named graphs are supported via a graph column.
//
// All methods are safe for concurrent use (WAL mode with busy_timeout).
//
// Reference: https://www.sqlite.org/wal.html
package sqlitestore
