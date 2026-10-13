package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite" // pure Go SQLite driver

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// SQLiteStore implements store.Store using SQLite as a persistent backend.
// All methods are safe for concurrent use (WAL mode with busy_timeout).
// Driver failures (a closed database, a locked file, a full disk) are
// returned, wrapped, never swallowed; the context is passed to database/sql,
// so a cancelled call stops the statement.
type SQLiteStore struct {
	db    *sql.DB
	stmts [8]*sql.Stmt // indexed by pattern bitmask: bit0=subject, bit1=predicate, bit2=object
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

	db, err := sql.Open("sqlite", dsnWithPragmas(dsn))
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: open: %w", err)
	}

	// sql.Open is lazy, so nothing has touched the file yet; ping to surface an
	// unusable path here rather than on the first silent write.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlitestore: open %q: %w", dsn, err)
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

	s := &SQLiteStore{db: db}

	// Pre-prepare all 8 SELECT query patterns (graph always required).
	// mask bit0=subject, bit1=predicate, bit2=object.
	for mask := 0; mask < 8; mask++ {
		q := "SELECT subject, predicate, object FROM triples WHERE graph = ?"
		if mask&1 != 0 {
			q += " AND subject = ?"
		}
		if mask&2 != 0 {
			q += " AND predicate = ?"
		}
		if mask&4 != 0 {
			q += " AND object = ?"
		}
		stmt, err := db.Prepare(q)
		if err != nil {
			// Close any already-prepared statements before returning.
			for i := 0; i < mask; i++ {
				s.stmts[i].Close()
			}
			db.Close()
			return nil, fmt.Errorf("sqlitestore: prepare stmts[%d]: %w", mask, err)
		}
		s.stmts[mask] = stmt
	}

	return s, nil
}

// Close closes all prepared statements and the SQLite database.
func (s *SQLiteStore) Close() error {
	for _, stmt := range s.stmts {
		if stmt != nil {
			stmt.Close()
		}
	}
	return s.db.Close()
}

// ContextAware reports true — SQLiteStore supports named graphs.
func (s *SQLiteStore) ContextAware() bool { return true }

// TransactionAware reports true — SQLite supports ACID transactions.
func (s *SQLiteStore) TransactionAware() bool { return true }

// Add inserts a triple into the store.
func (s *SQLiteStore) Add(ctx context.Context, t term.Triple, graph term.Term) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	gk := graphKey(graph)
	sk := term.TermKey(t.Subject)
	pk := term.TermKey(t.Predicate)
	ok := term.TermKey(t.Object)
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO triples (subject, predicate, object, graph) VALUES (?, ?, ?, ?)`,
		sk, pk, ok, gk,
	)
	if err != nil {
		return fmt.Errorf("sqlitestore: Add: %w", err)
	}
	return nil
}

// AddN batch-inserts multiple quads in one transaction: either all of them are
// stored or, on error, none.
func (s *SQLiteStore) AddN(ctx context.Context, quads []term.Quad) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(quads) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlitestore: AddN begin: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO triples (subject, predicate, object, graph) VALUES (?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("sqlitestore: AddN prepare: %w", err)
	}
	defer stmt.Close()

	for _, q := range quads {
		gk := graphKey(q.Graph)
		_, err := stmt.ExecContext(ctx, term.TermKey(q.Subject), term.TermKey(q.Predicate), term.TermKey(q.Object), gk)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("sqlitestore: AddN exec: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlitestore: AddN commit: %w", err)
	}
	return nil
}

// Remove deletes triples matching the pattern from the given context.
func (s *SQLiteStore) Remove(ctx context.Context, pattern term.TriplePattern, graph term.Term) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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

	gk := graphKey(graph)
	query += " AND graph = ?"
	args = append(args, gk)

	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("sqlitestore: Remove: %w", err)
	}
	return nil
}

// Set atomically replaces triples matching (s, p, *) with the new triple.
func (s *SQLiteStore) Set(ctx context.Context, t term.Triple, graph term.Term) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	gk := graphKey(graph)
	sk := term.TermKey(t.Subject)
	pk := term.TermKey(t.Predicate)
	ok := term.TermKey(t.Object)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlitestore: Set begin: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM triples WHERE subject = ? AND predicate = ? AND graph = ?`, sk, pk, gk); err != nil {
		tx.Rollback()
		return fmt.Errorf("sqlitestore: Set delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO triples (subject, predicate, object, graph) VALUES (?, ?, ?, ?)`, sk, pk, ok, gk); err != nil {
		tx.Rollback()
		return fmt.Errorf("sqlitestore: Set insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlitestore: Set commit: %w", err)
	}
	return nil
}

// Triples returns an iterator over triples matching the pattern.
func (s *SQLiteStore) Triples(ctx context.Context, pattern term.TriplePattern, graph term.Term) store.TripleIterator {
	return func(yield func(term.Triple, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(term.Triple{}, err)
			return
		}
		sk := term.OptTermKey(pattern.Subject)
		pk := term.OptPredKey(pattern.Predicate)
		ok := term.OptTermKey(pattern.Object)

		mask := 0
		if sk != "" {
			mask |= 1
		}
		if pk != "" {
			mask |= 2
		}
		if ok != "" {
			mask |= 4
		}

		gk := graphKey(graph)

		// Build args: graph always first, then subject/predicate/object as bound.
		args := make([]any, 0, 4)
		args = append(args, gk)
		if mask&1 != 0 {
			args = append(args, sk)
		}
		if mask&2 != 0 {
			args = append(args, pk)
		}
		if mask&4 != 0 {
			args = append(args, ok)
		}

		rows, err := s.stmts[mask].QueryContext(ctx, args...)
		if err != nil {
			yield(term.Triple{}, fmt.Errorf("sqlitestore: Triples: %w", err))
			return
		}
		yieldRows(rows, "Triples", yield)
	}
}

// yieldRows streams (subject, predicate, object) rows to yield and closes
// rows. A scan, decode or driver failure is yielded once as an error and ends
// the iteration: a row that cannot be read must not vanish from the result.
func yieldRows(rows *sql.Rows, op string, yield func(term.Triple, error) bool) {
	defer rows.Close()
	fail := func(err error) {
		yield(term.Triple{}, fmt.Errorf("sqlitestore: %s: %w", op, err))
	}
	for rows.Next() {
		var sk, pk, ok string
		if err := rows.Scan(&sk, &pk, &ok); err != nil {
			fail(err)
			return
		}
		t, err := decodeRow(sk, pk, ok)
		if err != nil {
			fail(err)
			return
		}
		if !yield(t, nil) {
			return
		}
	}
	if err := rows.Err(); err != nil {
		fail(err)
	}
}

// whereClause builds the " WHERE …" condition shared by SELECT, COUNT and
// EXISTS queries.
func whereClause(pattern term.TriplePattern, graph term.Term) (string, []any) {
	var sb strings.Builder
	sb.WriteString(" WHERE 1=1")
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

	sb.WriteString(" AND graph = ?")
	args = append(args, graphKey(graph))
	return sb.String(), args
}

// buildQuery constructs a SELECT query from the pattern.
func (s *SQLiteStore) buildQuery(pattern term.TriplePattern, graph term.Term) (string, []any) {
	where, args := whereClause(pattern, graph)
	return "SELECT subject, predicate, object FROM triples" + where, args
}

// Len returns the number of triples in the given context.
func (s *SQLiteStore) Len(ctx context.Context, graph term.Term) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	gk := graphKey(graph)
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM triples WHERE graph = ?", gk).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlitestore: Len: %w", err)
	}
	return count, nil
}

// Contexts returns an iterator over named graph URIs.
func (s *SQLiteStore) Contexts(ctx context.Context, triple *term.Triple) store.TermIterator {
	return func(yield func(term.Term, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(nil, err)
			return
		}
		fail := func(err error) {
			yield(nil, fmt.Errorf("sqlitestore: Contexts: %w", err))
		}
		var query string
		var args []any
		if triple != nil {
			query = "SELECT DISTINCT graph FROM triples WHERE graph != '' AND subject = ? AND predicate = ? AND object = ?"
			args = []any{term.TermKey(triple.Subject), term.TermKey(triple.Predicate), term.TermKey(triple.Object)}
		} else {
			query = "SELECT DISTINCT graph FROM triples WHERE graph != ''"
		}

		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			fail(err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var gk string
			if err := rows.Scan(&gk); err != nil {
				fail(err)
				return
			}
			t, err := term.TermFromKey(gk)
			if err != nil {
				fail(fmt.Errorf("undecodable graph key %q: %w", gk, err))
				return
			}
			if !yield(t, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			fail(err)
		}
	}
}

// Bind associates a prefix with a namespace URI.
func (s *SQLiteStore) Bind(ctx context.Context, prefix string, namespace term.URIRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO namespaces (prefix, namespace) VALUES (?, ?)`,
		prefix, namespace.Value(),
	)
	if err != nil {
		return fmt.Errorf("sqlitestore: Bind: %w", err)
	}
	return nil
}

// Namespace returns the namespace URI for a prefix.
func (s *SQLiteStore) Namespace(ctx context.Context, prefix string) (term.URIRef, bool, error) {
	if err := ctx.Err(); err != nil {
		return term.URIRef{}, false, err
	}
	var ns string
	err := s.db.QueryRowContext(ctx, "SELECT namespace FROM namespaces WHERE prefix = ?", prefix).Scan(&ns)
	if errors.Is(err, sql.ErrNoRows) {
		return term.URIRef{}, false, nil
	}
	if err != nil {
		return term.URIRef{}, false, fmt.Errorf("sqlitestore: Namespace: %w", err)
	}
	return term.NewURIRefUnsafe(ns), true, nil
}

// Prefix returns the prefix for a namespace URI.
func (s *SQLiteStore) Prefix(ctx context.Context, namespace term.URIRef) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	var prefix string
	err := s.db.QueryRowContext(ctx, "SELECT prefix FROM namespaces WHERE namespace = ?", namespace.Value()).Scan(&prefix)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("sqlitestore: Prefix: %w", err)
	}
	return prefix, true, nil
}

// Namespaces returns an iterator over all (prefix, namespace) bindings.
func (s *SQLiteStore) Namespaces(ctx context.Context) store.NamespaceIterator {
	return func(yield func(store.NamespaceBinding, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(store.NamespaceBinding{}, err)
			return
		}
		fail := func(err error) {
			yield(store.NamespaceBinding{}, fmt.Errorf("sqlitestore: Namespaces: %w", err))
		}
		rows, err := s.db.QueryContext(ctx, "SELECT prefix, namespace FROM namespaces")
		if err != nil {
			fail(err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var prefix, ns string
			if err := rows.Scan(&prefix, &ns); err != nil {
				fail(err)
				return
			}
			if !yield(store.NamespaceBinding{Prefix: prefix, Namespace: term.NewURIRefUnsafe(ns)}, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			fail(err)
		}
	}
}

// TriplesWithLimit returns an iterator over triples matching the pattern with
// store-level LIMIT and OFFSET applied.
func (s *SQLiteStore) TriplesWithLimit(ctx context.Context, pattern term.TriplePattern, graph term.Term, limit, offset int) store.TripleIterator {
	return func(yield func(term.Triple, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(term.Triple{}, err)
			return
		}
		query, args := s.buildQuery(pattern, graph)
		// A non-positive limit means "no limit", matching MemoryStore. SQLite
		// spells that as a negative LIMIT; passing 0 through would return
		// nothing, which is the opposite of what every other backend does.
		if limit <= 0 {
			limit = -1
		}
		if offset < 0 {
			offset = 0
		}
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)

		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			yield(term.Triple{}, fmt.Errorf("sqlitestore: TriplesWithLimit: %w", err))
			return
		}
		yieldRows(rows, "TriplesWithLimit", yield)
	}
}

// Count returns the number of triples matching the pattern without
// materializing them.
func (s *SQLiteStore) Count(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	where, args := whereClause(pattern, graph)
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM triples"+where, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlitestore: Count: %w", err)
	}
	return count, nil
}

// Exists returns true if at least one triple matches the pattern.
func (s *SQLiteStore) Exists(ctx context.Context, pattern term.TriplePattern, graph term.Term) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	where, args := whereClause(pattern, graph)
	var dummy int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM triples"+where+" LIMIT 1", args...).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("sqlitestore: Exists: %w", err)
	}
	return true, nil
}

// dsnWithPragmas returns dsn with the connection pragmas attached as DSN
// parameters.
//
// They have to travel with the DSN rather than be executed once after opening:
// database/sql hands out a pool of connections, and a PRAGMA applies only to
// the connection that ran it. Every connection after the first therefore had no
// busy_timeout, so the moment two goroutines wrote at the same time the second
// failed immediately with SQLITE_BUSY (once silently, before Store could
// report a write error). Attaching the pragmas here makes them apply to every connection the pool
// opens, which is what makes the documented "safe for concurrent use" true.
func dsnWithPragmas(dsn string) string {
	const pragmas = "_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"

	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&" + pragmas
	}
	return dsn + "?" + pragmas
}

// graphKey returns the TermKey for a context term, or "" for the default graph
// (a nil context or store.DefaultGraph). A blank node names a graph like an IRI
// does.
//
// Rows written before blank nodes named graphs are unaffected: a blank-node
// context was stored under the empty graph key, which is still the default graph.
func graphKey(ctx term.Term) string {
	if store.IsDefaultGraph(ctx) {
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

// Compile-time check that SQLiteStore implements store.QueryableStore.
var _ store.QueryableStore = (*SQLiteStore)(nil)
