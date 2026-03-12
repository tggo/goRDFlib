package sqlitestore

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "modernc.org/sqlite" // pure Go SQLite driver

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// SQLiteStore implements store.Store using SQLite as a persistent backend.
// All methods are safe for concurrent use (WAL mode with busy_timeout).
type SQLiteStore struct {
	db *sql.DB
}

// Option configures a SQLiteStore.
type Option func(*options)

type options struct {
	dsn      string
	inMemory bool
}

// WithFile sets the SQLite database file path.
func WithFile(path string) Option {
	return func(o *options) { o.dsn = path }
}

// WithInMemory uses an in-memory database. Useful for testing.
func WithInMemory() Option {
	return func(o *options) { o.inMemory = true }
}

// New creates a SQLiteStore. Either WithFile or WithInMemory must be provided.
func New(opts ...Option) (*SQLiteStore, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	dsn := o.dsn
	if o.inMemory {
		// Use shared cache so all connections see the same in-memory database.
		dsn = "file::memory:?mode=memory&cache=shared"
	}
	if dsn == "" {
		return nil, fmt.Errorf("sqlitestore: no database path or in-memory flag")
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: open: %w", err)
	}

	// Configure for concurrent access.
	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA busy_timeout = 5000;
		PRAGMA synchronous = NORMAL;
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlitestore: pragmas: %w", err)
	}

	// Create schema.
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS triples (
			subject   TEXT NOT NULL,
			predicate TEXT NOT NULL,
			object    TEXT NOT NULL,
			graph     TEXT NOT NULL DEFAULT '',
			UNIQUE(subject, predicate, object, graph)
		);
		CREATE INDEX IF NOT EXISTS idx_spo ON triples(subject, predicate, object);
		CREATE INDEX IF NOT EXISTS idx_pos ON triples(predicate, object, subject);
		CREATE INDEX IF NOT EXISTS idx_osp ON triples(object, subject, predicate);
		CREATE INDEX IF NOT EXISTS idx_graph ON triples(graph);

		CREATE TABLE IF NOT EXISTS namespaces (
			prefix    TEXT PRIMARY KEY,
			namespace TEXT NOT NULL UNIQUE
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlitestore: schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the SQLite database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// ContextAware reports true — SQLiteStore supports named graphs.
func (s *SQLiteStore) ContextAware() bool { return true }

// TransactionAware reports true — SQLite supports ACID transactions.
func (s *SQLiteStore) TransactionAware() bool { return true }

// Add inserts a triple into the store.
func (s *SQLiteStore) Add(t term.Triple, ctx term.Term) {
	gk := graphKey(ctx)
	sk := term.TermKey(t.Subject)
	pk := term.TermKey(t.Predicate)
	ok := term.TermKey(t.Object)
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO triples (subject, predicate, object, graph) VALUES (?, ?, ?, ?)`,
		sk, pk, ok, gk,
	)
	if err != nil {
		log.Printf("sqlitestore: Add: %v", err)
	}
}

// AddN batch-inserts multiple quads.
func (s *SQLiteStore) AddN(quads []term.Quad) {
	if len(quads) == 0 {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("sqlitestore: AddN begin: %v", err)
		return
	}
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO triples (subject, predicate, object, graph) VALUES (?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		log.Printf("sqlitestore: AddN prepare: %v", err)
		return
	}
	defer stmt.Close()

	for _, q := range quads {
		gk := graphKey(q.Graph)
		_, err := stmt.Exec(term.TermKey(q.Subject), term.TermKey(q.Predicate), term.TermKey(q.Object), gk)
		if err != nil {
			tx.Rollback()
			log.Printf("sqlitestore: AddN exec: %v", err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("sqlitestore: AddN commit: %v", err)
	}
}

// Remove deletes triples matching the pattern from the given context.
func (s *SQLiteStore) Remove(pattern term.TriplePattern, ctx term.Term) {
	query := "DELETE FROM triples WHERE 1=1"
	args := []any{}

	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)

	if sk != "" {
		query += " AND subject = ?"
		args = append(args, sk)
	}
	if pk != "" {
		query += " AND predicate = ?"
		args = append(args, pk)
	}
	if ok != "" {
		query += " AND object = ?"
		args = append(args, ok)
	}

	gk := graphKey(ctx)
	query += " AND graph = ?"
	args = append(args, gk)

	if _, err := s.db.Exec(query, args...); err != nil {
		log.Printf("sqlitestore: Remove: %v", err)
	}
}

// Set atomically replaces triples matching (s, p, *) with the new triple.
func (s *SQLiteStore) Set(t term.Triple, ctx term.Term) {
	gk := graphKey(ctx)
	sk := term.TermKey(t.Subject)
	pk := term.TermKey(t.Predicate)
	ok := term.TermKey(t.Object)

	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("sqlitestore: Set begin: %v", err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM triples WHERE subject = ? AND predicate = ? AND graph = ?`, sk, pk, gk); err != nil {
		tx.Rollback()
		log.Printf("sqlitestore: Set delete: %v", err)
		return
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO triples (subject, predicate, object, graph) VALUES (?, ?, ?, ?)`, sk, pk, ok, gk); err != nil {
		tx.Rollback()
		log.Printf("sqlitestore: Set insert: %v", err)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("sqlitestore: Set commit: %v", err)
	}
}

// Triples returns an iterator over triples matching the pattern.
func (s *SQLiteStore) Triples(pattern term.TriplePattern, ctx term.Term) store.TripleIterator {
	return func(yield func(term.Triple) bool) {
		query, args := s.buildQuery(pattern, ctx)
		rows, err := s.db.Query(query, args...)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var sk, pk, ok string
			if err := rows.Scan(&sk, &pk, &ok); err != nil {
				continue
			}
			t, err := decodeRow(sk, pk, ok)
			if err != nil {
				continue
			}
			if !yield(t) {
				return
			}
		}
	}
}

// buildQuery constructs a SELECT query from the pattern.
func (s *SQLiteStore) buildQuery(pattern term.TriplePattern, ctx term.Term) (string, []any) {
	var sb strings.Builder
	sb.WriteString("SELECT subject, predicate, object FROM triples WHERE 1=1")
	args := []any{}

	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)

	if sk != "" {
		sb.WriteString(" AND subject = ?")
		args = append(args, sk)
	}
	if pk != "" {
		sb.WriteString(" AND predicate = ?")
		args = append(args, pk)
	}
	if ok != "" {
		sb.WriteString(" AND object = ?")
		args = append(args, ok)
	}

	gk := graphKey(ctx)
	sb.WriteString(" AND graph = ?")
	args = append(args, gk)

	return sb.String(), args
}

// Len returns the number of triples in the given context.
func (s *SQLiteStore) Len(ctx term.Term) int {
	gk := graphKey(ctx)
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM triples WHERE graph = ?", gk).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// Contexts returns an iterator over named graph URIs.
func (s *SQLiteStore) Contexts(triple *term.Triple) store.TermIterator {
	return func(yield func(term.Term) bool) {
		var query string
		var args []any
		if triple != nil {
			query = "SELECT DISTINCT graph FROM triples WHERE graph != '' AND subject = ? AND predicate = ? AND object = ?"
			args = []any{term.TermKey(triple.Subject), term.TermKey(triple.Predicate), term.TermKey(triple.Object)}
		} else {
			query = "SELECT DISTINCT graph FROM triples WHERE graph != ''"
		}

		rows, err := s.db.Query(query, args...)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var gk string
			if err := rows.Scan(&gk); err != nil {
				continue
			}
			t, err := term.TermFromKey(gk)
			if err != nil {
				continue
			}
			if !yield(t) {
				return
			}
		}
	}
}

// Bind associates a prefix with a namespace URI.
func (s *SQLiteStore) Bind(prefix string, namespace term.URIRef) {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO namespaces (prefix, namespace) VALUES (?, ?)`,
		prefix, namespace.Value(),
	)
	if err != nil {
		log.Printf("sqlitestore: Bind: %v", err)
	}
}

// Namespace returns the namespace URI for a prefix.
func (s *SQLiteStore) Namespace(prefix string) (term.URIRef, bool) {
	var ns string
	err := s.db.QueryRow("SELECT namespace FROM namespaces WHERE prefix = ?", prefix).Scan(&ns)
	if err != nil {
		return term.URIRef{}, false
	}
	return term.NewURIRefUnsafe(ns), true
}

// Prefix returns the prefix for a namespace URI.
func (s *SQLiteStore) Prefix(namespace term.URIRef) (string, bool) {
	var prefix string
	err := s.db.QueryRow("SELECT prefix FROM namespaces WHERE namespace = ?", namespace.Value()).Scan(&prefix)
	if err != nil {
		return "", false
	}
	return prefix, true
}

// Namespaces returns an iterator over all (prefix, namespace) bindings.
func (s *SQLiteStore) Namespaces() store.NamespaceIterator {
	return func(yield func(string, term.URIRef) bool) {
		rows, err := s.db.Query("SELECT prefix, namespace FROM namespaces")
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var prefix, ns string
			if err := rows.Scan(&prefix, &ns); err != nil {
				continue
			}
			if !yield(prefix, term.NewURIRefUnsafe(ns)) {
				return
			}
		}
	}
}

// graphKey returns the TermKey for a context term, or "" for the default graph.
func graphKey(ctx term.Term) string {
	if ctx == nil {
		return ""
	}
	if _, isBNode := ctx.(term.BNode); isBNode {
		return ""
	}
	return term.TermKey(ctx)
}

// decodeRow reconstructs a Triple from stored TermKey strings.
func decodeRow(sk, pk, ok string) (term.Triple, error) {
	s, err := term.TermFromKey(sk)
	if err != nil {
		return term.Triple{}, err
	}
	p, err := term.TermFromKey(pk)
	if err != nil {
		return term.Triple{}, err
	}
	o, err := term.TermFromKey(ok)
	if err != nil {
		return term.Triple{}, err
	}
	subj, isSubj := s.(term.Subject)
	if !isSubj {
		return term.Triple{}, fmt.Errorf("sqlitestore: subject is not Subject type")
	}
	pred, isPred := p.(term.URIRef)
	if !isPred {
		return term.Triple{}, fmt.Errorf("sqlitestore: predicate is not URIRef")
	}
	return term.Triple{Subject: subj, Predicate: pred, Object: o}, nil
}

// Compile-time check that SQLiteStore implements store.Store.
var _ store.Store = (*SQLiteStore)(nil)
