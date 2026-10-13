package badgerstore

import (
	"context"
	"errors"
	"fmt"
	"sync"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// BadgerStore implements store.Store using Badger as a persistent KV backend.
// All methods are safe for concurrent use (delegated to Badger's MVCC), except
// on a read snapshot returned by ReadSnapshot. Badger's own failures (a closed
// database, a full disk, a corrupt value) are returned, wrapped, never
// swallowed; a done context is refused before any I/O.
type BadgerStore struct {
	db *badger.DB
	// snap is set on a read snapshot (see ReadSnapshot): every read uses this
	// one transaction and writes are refused.
	snap *badger.Txn
}

// Option configures a BadgerStore.
type Option func(*badger.Options)

// WithInMemory enables in-memory mode (no disk persistence). Useful for testing.
func WithInMemory() Option {
	return func(o *badger.Options) { o.InMemory = true }
}

// WithDir sets the directory for the database files.
func WithDir(dir string) Option {
	return func(o *badger.Options) {
		o.Dir = dir
		o.ValueDir = dir
	}
}

// WithReadOnly opens the database in read-only mode.
func WithReadOnly() Option {
	return func(o *badger.Options) { o.ReadOnly = true }
}

// WithLogger sets the Badger logger. Nil disables logging.
func WithLogger(l badger.Logger) Option {
	return func(o *badger.Options) { o.Logger = l }
}

// New creates a BadgerStore. At minimum, either WithDir or WithInMemory must
// be provided. Returns an error if the database cannot be opened.
func New(opts ...Option) (*BadgerStore, error) {
	bopts := badger.DefaultOptions("")
	bopts.Logger = nil // silence badger logs by default
	for _, o := range opts {
		o(&bopts)
	}
	db, err := badger.Open(bopts)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{db: db}, nil
}

// pollEvery is how many index entries a scan visits between looks at the
// context. Badger cannot be interrupted, so a scan polls.
const pollEvery = 1024

// Close closes the Badger database. Must be called when the store is no longer needed.
func (s *BadgerStore) Close() error {
	if s.snap != nil {
		return errors.New("badgerstore: Close on a read snapshot; call its release function instead")
	}
	return s.db.Close()
}

// ReadSnapshot returns a read-only view of the store backed by one Badger read
// transaction, and the function that releases it. Queries evaluate thousands
// of lookups; opening a transaction for each one made concurrent queries spend
// most of their time in Badger's transaction bookkeeping.
//
// The snapshot sees the data as of the call, is not safe for concurrent use,
// and panics on writes. Hold it only for the duration
// of a query: an open read transaction keeps old versions from being
// garbage-collected.
func (s *BadgerStore) ReadSnapshot(ctx context.Context) (store.Store, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if s.snap != nil {
		return s, func() {}, nil
	}
	if s.db.IsClosed() {
		return nil, nil, fmt.Errorf("badgerstore: ReadSnapshot: %w", badger.ErrDBClosed)
	}
	txn := s.db.NewTransaction(false)
	var once sync.Once
	return &BadgerStore{db: s.db, snap: txn}, func() { once.Do(txn.Discard) }, nil
}

// read runs fn in the snapshot's transaction, or in a new read transaction when
// s is not a snapshot.
func (s *BadgerStore) read(fn func(txn *badger.Txn) error) error {
	if s.snap != nil {
		return fn(s.snap)
	}
	return s.db.View(fn)
}

// begin refuses a done context and a closed database; for a write on a read
// snapshot, which is a programming error, it panics. Every method calls it
// first, so a refused call has touched nothing.
func (s *BadgerStore) begin(ctx context.Context, write bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if write && s.snap != nil {
		panic("badgerstore: write on a read snapshot")
	}
	if s.snap == nil && s.db.IsClosed() {
		return fmt.Errorf("badgerstore: %w", badger.ErrDBClosed)
	}
	return nil
}

// ContextAware reports true — BadgerStore supports named graphs.
func (s *BadgerStore) ContextAware() bool { return true }

// TransactionAware reports true — Badger supports ACID transactions.
func (s *BadgerStore) TransactionAware() bool { return true }

// Add inserts a triple into the store, associated with the given context.
func (s *BadgerStore) Add(ctx context.Context, t term.Triple, graph term.Term) error {
	if err := s.begin(ctx, true); err != nil {
		return err
	}
	gk := graphKey(graph)
	sk := term.TermKey(t.Subject)
	pk := term.TermKey(t.Predicate)
	ok := term.TermKey(t.Object)
	val := encodeTriple(t)

	err := s.db.Update(func(txn *badger.Txn) error {
		// Check for duplicate.
		key := spoKey(gk, sk, pk, ok)
		if _, err := txn.Get(key); err == nil {
			return nil // already exists
		} else if !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}

		if err := txn.Set(key, val); err != nil {
			return err
		}
		if err := txn.Set(posKey(gk, pk, ok, sk), val); err != nil {
			return err
		}
		if err := txn.Set(ospKey(gk, ok, sk, pk), val); err != nil {
			return err
		}
		// Track context if named graph.
		if gk != "" {
			if err := txn.Set(ctxKey(gk), nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("badgerstore: Add: %w", err)
	}
	return nil
}

// AddN batch-inserts multiple quads.
func (s *BadgerStore) AddN(ctx context.Context, quads []term.Quad) error {
	if err := s.begin(ctx, true); err != nil {
		return err
	}
	if len(quads) == 0 {
		return nil
	}
	wb := s.db.NewWriteBatch()
	defer wb.Cancel()

	for i, q := range quads {
		if i%pollEvery == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		gk := graphKey(q.Graph)
		sk := term.TermKey(q.Subject)
		pk := term.TermKey(q.Predicate)
		ok := term.TermKey(q.Object)
		val := encodeTriple(q.Triple)

		if err := wb.Set(spoKey(gk, sk, pk, ok), val); err != nil {
			return fmt.Errorf("badgerstore: AddN: %w", err)
		}
		if err := wb.Set(posKey(gk, pk, ok, sk), val); err != nil {
			return fmt.Errorf("badgerstore: AddN: %w", err)
		}
		if err := wb.Set(ospKey(gk, ok, sk, pk), val); err != nil {
			return fmt.Errorf("badgerstore: AddN: %w", err)
		}
		if gk != "" {
			if err := wb.Set(ctxKey(gk), nil); err != nil {
				return fmt.Errorf("badgerstore: AddN: %w", err)
			}
		}
	}
	if err := wb.Flush(); err != nil {
		return fmt.Errorf("badgerstore: AddN flush: %w", err)
	}
	return nil
}

// Remove deletes triples matching the pattern from the given context.
func (s *BadgerStore) Remove(ctx context.Context, pattern term.TriplePattern, graph term.Term) error {
	if err := s.begin(ctx, true); err != nil {
		return err
	}
	gk := graphKey(graph)

	// Collect matching triples first, then delete.
	var toRemove []tripleKeys
	err := s.read(func(txn *badger.Txn) error {
		return s.scanTriples(ctx, txn, pattern, gk, func(sk, pk, ok string, _ term.Triple) bool {
			toRemove = append(toRemove, tripleKeys{gk: gk, sk: sk, pk: pk, ok: ok})
			return true
		})
	})
	if err != nil {
		return fmt.Errorf("badgerstore: Remove scan: %w", err)
	}

	if len(toRemove) == 0 {
		return nil
	}
	// The scan may have been cut short by the context; do not delete on a
	// partial list.
	if err := ctx.Err(); err != nil {
		return err
	}

	err = s.db.Update(func(txn *badger.Txn) error {
		for _, tk := range toRemove {
			if err := txn.Delete(spoKey(tk.gk, tk.sk, tk.pk, tk.ok)); err != nil {
				return err
			}
			if err := txn.Delete(posKey(tk.gk, tk.pk, tk.ok, tk.sk)); err != nil {
				return err
			}
			if err := txn.Delete(ospKey(tk.gk, tk.ok, tk.sk, tk.pk)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("badgerstore: Remove delete: %w", err)
	}
	return nil
}

// tripleKeys holds the key components for a triple in a specific graph.
type tripleKeys struct {
	gk, sk, pk, ok string
}

// Set atomically replaces triples matching (s, p, *) with the new triple.
func (s *BadgerStore) Set(ctx context.Context, t term.Triple, graph term.Term) error {
	if err := s.begin(ctx, true); err != nil {
		return err
	}
	gk := graphKey(graph)
	sk := term.TermKey(t.Subject)
	pk := term.TermKey(t.Predicate)
	val := encodeTriple(t)
	newOK := term.TermKey(t.Object)

	err := s.db.Update(func(txn *badger.Txn) error {
		// Find and delete existing (s, p, *) triples.
		pattern := term.TriplePattern{Subject: t.Subject, Predicate: &t.Predicate}
		var toDelete []string // object keys to delete
		if err := s.scanTriplesInTxn(ctx, txn, pattern, gk, func(_, _, ok string, _ term.Triple) bool {
			toDelete = append(toDelete, ok)
			return true
		}); err != nil {
			return err
		}

		for _, oldOK := range toDelete {
			if err := txn.Delete(spoKey(gk, sk, pk, oldOK)); err != nil {
				return err
			}
			if err := txn.Delete(posKey(gk, pk, oldOK, sk)); err != nil {
				return err
			}
			if err := txn.Delete(ospKey(gk, oldOK, sk, pk)); err != nil {
				return err
			}
		}

		// Insert new triple.
		if err := txn.Set(spoKey(gk, sk, pk, newOK), val); err != nil {
			return err
		}
		if err := txn.Set(posKey(gk, pk, newOK, sk), val); err != nil {
			return err
		}
		if err := txn.Set(ospKey(gk, newOK, sk, pk), val); err != nil {
			return err
		}
		if gk != "" {
			if err := txn.Set(ctxKey(gk), nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("badgerstore: Set: %w", err)
	}
	return nil
}

// Triples returns an iterator over triples matching the pattern in the given context.
func (s *BadgerStore) Triples(ctx context.Context, pattern term.TriplePattern, graph term.Term) store.TripleIterator {
	return func(yield func(term.Triple, error) bool) {
		if err := s.begin(ctx, false); err != nil {
			yield(term.Triple{}, err)
			return
		}
		gk := graphKey(graph)
		err := s.read(func(txn *badger.Txn) error {
			return s.scanTripleValues(ctx, txn, pattern, gk, func(t term.Triple) bool {
				return yield(t, nil)
			})
		})
		if err != nil {
			yield(term.Triple{}, fmt.Errorf("badgerstore: Triples: %w", err))
		}
	}
}

// Len returns the number of triples in the given context (nil = default graph).
func (s *BadgerStore) Len(ctx context.Context, graph term.Term) (int, error) {
	if err := s.begin(ctx, false); err != nil {
		return 0, err
	}
	gk := graphKey(graph)
	count := 0
	err := s.read(func(txn *badger.Txn) error {
		var err error
		count, err = countPrefix(ctx, txn, makePrefixKey(pfxSPO, gk))
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("badgerstore: Len: %w", err)
	}
	return count, nil
}

// countPrefix counts the keys under prefix without reading values.
func countPrefix(ctx context.Context, txn *badger.Txn, prefix []byte) (int, error) {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Prefix = prefix
	it := txn.NewIterator(opts)
	defer it.Close()
	count := 0
	for it.Seek(prefix); it.Valid(); it.Next() {
		if count%pollEvery == 0 {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
		}
		count++
	}
	return count, nil
}

// Contexts returns an iterator over named graph URIs, optionally filtered by
// a triple that must exist in the graph.
func (s *BadgerStore) Contexts(ctx context.Context, triple *term.Triple) store.TermIterator {
	return func(yield func(term.Term, error) bool) {
		if err := s.begin(ctx, false); err != nil {
			yield(nil, err)
			return
		}
		err := s.read(func(txn *badger.Txn) error {
			prefix := []byte{pfxCTX, sep}
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			opts.Prefix = prefix
			it := txn.NewIterator(opts)
			defer it.Close()
			n := 0
			for it.Seek(prefix); it.Valid(); it.Next() {
				if n++; n%pollEvery == 0 {
					if err := ctx.Err(); err != nil {
						return err
					}
				}
				key := it.Item().Key()
				gk := string(key[2:]) // skip "c\x00"
				if gk == "" {
					continue
				}

				// Reconstruct the context term.
				ctxTerm, err := term.TermFromKey(gk)
				if err != nil {
					return fmt.Errorf("undecodable graph key %q: %w", gk, err)
				}

				// If a triple filter is specified, check membership.
				if triple != nil {
					sk := term.TermKey(triple.Subject)
					pk := term.TermKey(triple.Predicate)
					ok := term.TermKey(triple.Object)
					if _, err := txn.Get(spoKey(gk, sk, pk, ok)); err != nil {
						if errors.Is(err, badger.ErrKeyNotFound) {
							continue
						}
						return err
					}
				}

				if !yield(ctxTerm, nil) {
					return nil
				}
			}
			return nil
		})
		if err != nil {
			yield(nil, fmt.Errorf("badgerstore: Contexts: %w", err))
		}
	}
}

// Bind associates a prefix with a namespace URI.
func (s *BadgerStore) Bind(ctx context.Context, prefix string, namespace term.URIRef) error {
	if err := s.begin(ctx, true); err != nil {
		return err
	}
	err := s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(nsKey(prefix), []byte(namespace.Value())); err != nil {
			return err
		}
		return txn.Set(nuKey(namespace.Value()), []byte(prefix))
	})
	if err != nil {
		return fmt.Errorf("badgerstore: Bind: %w", err)
	}
	return nil
}

// Namespace returns the namespace URI for a prefix.
func (s *BadgerStore) Namespace(ctx context.Context, prefix string) (term.URIRef, bool, error) {
	if err := s.begin(ctx, false); err != nil {
		return term.URIRef{}, false, err
	}
	var ns term.URIRef
	var found bool
	err := s.read(func(txn *badger.Txn) error {
		item, err := txn.Get(nsKey(prefix))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			ns = term.NewURIRefUnsafe(string(val))
			found = true
			return nil
		})
	})
	if err != nil {
		return term.URIRef{}, false, fmt.Errorf("badgerstore: Namespace: %w", err)
	}
	return ns, found, nil
}

// Prefix returns the prefix for a namespace URI.
func (s *BadgerStore) Prefix(ctx context.Context, namespace term.URIRef) (string, bool, error) {
	if err := s.begin(ctx, false); err != nil {
		return "", false, err
	}
	var prefix string
	var found bool
	err := s.read(func(txn *badger.Txn) error {
		item, err := txn.Get(nuKey(namespace.Value()))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			prefix = string(val)
			found = true
			return nil
		})
	})
	if err != nil {
		return "", false, fmt.Errorf("badgerstore: Prefix: %w", err)
	}
	return prefix, found, nil
}

// Namespaces returns an iterator over all (prefix, namespace) bindings.
func (s *BadgerStore) Namespaces(ctx context.Context) store.NamespaceIterator {
	return func(yield func(store.NamespaceBinding, error) bool) {
		if err := s.begin(ctx, false); err != nil {
			yield(store.NamespaceBinding{}, err)
			return
		}
		err := s.read(func(txn *badger.Txn) error {
			prefix := []byte{pfxNS, sep}
			opts := badger.DefaultIteratorOptions
			opts.Prefix = prefix
			it := txn.NewIterator(opts)
			defer it.Close()
			for it.Seek(prefix); it.Valid(); it.Next() {
				item := it.Item()
				nsPrefix := string(item.Key()[2:]) // skip "n\x00"
				var nsURI string
				if err := item.Value(func(val []byte) error {
					nsURI = string(val)
					return nil
				}); err != nil {
					return err
				}
				if !yield(store.NamespaceBinding{Prefix: nsPrefix, Namespace: term.NewURIRefUnsafe(nsURI)}, nil) {
					return nil
				}
			}
			return nil
		})
		if err != nil {
			yield(store.NamespaceBinding{}, fmt.Errorf("badgerstore: Namespaces: %w", err))
		}
	}
}

// scanTriples scans triples matching a pattern in a read-only transaction.
func (s *BadgerStore) scanTriples(ctx context.Context, txn *badger.Txn, pattern term.TriplePattern, gk string, fn func(sk, pk, ok string, t term.Triple) bool) error {
	return s.scanTriplesInTxn(ctx, txn, pattern, gk, fn)
}

// scanTriplesInTxn performs triple scanning within a given transaction.
// It selects the optimal index based on which pattern fields are set.
func (s *BadgerStore) scanTriplesInTxn(ctx context.Context, txn *badger.Txn, pattern term.TriplePattern, gk string, fn func(sk, pk, ok string, t term.Triple) bool) error {
	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)

	if sk != "" && pk != "" && ok != "" {
		// Exact lookup.
		item, err := txn.Get(spoKey(gk, sk, pk, ok))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			t, err := decodeTriple(val)
			if err != nil {
				return err
			}
			fn(sk, pk, ok, t)
			return nil
		})
	}

	return iteratePrefix(ctx, txn, scanPrefix(pattern, gk), func(t term.Triple) bool {
		return fn(term.TermKey(t.Subject), term.TermKey(t.Predicate), term.TermKey(t.Object), t)
	})
}

// scanTripleValues is scanTriplesInTxn for callers that only need the triples:
// it skips building the three term keys per match, which Triples and
// TriplesWithLimit never read.
func (s *BadgerStore) scanTripleValues(ctx context.Context, txn *badger.Txn, pattern term.TriplePattern, gk string, fn func(t term.Triple) bool) error {
	if pattern.Subject != nil && pattern.Predicate != nil && pattern.Object != nil {
		return s.scanTriplesInTxn(ctx, txn, pattern, gk, func(_, _, _ string, t term.Triple) bool { return fn(t) })
	}
	return iteratePrefix(ctx, txn, scanPrefix(pattern, gk), fn)
}

// scanPrefix returns the index prefix scanTriplesInTxn scans for a pattern that
// is not fully bound.
func scanPrefix(pattern term.TriplePattern, gk string) []byte {
	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)
	switch {
	case sk != "" && pk != "":
		return makePrefixKey(pfxSPO, gk, sk, pk)
	case sk != "" && ok != "":
		return makePrefixKey(pfxOSP, gk, ok, sk)
	case sk != "":
		return makePrefixKey(pfxSPO, gk, sk)
	case pk != "" && ok != "":
		return makePrefixKey(pfxPOS, gk, pk, ok)
	case pk != "":
		return makePrefixKey(pfxPOS, gk, pk)
	case ok != "":
		return makePrefixKey(pfxOSP, gk, ok)
	default:
		return makePrefixKey(pfxSPO, gk)
	}
}

// iteratePrefix decodes every triple stored under prefix until fn returns
// false. A value that cannot be decoded is an error: skipping it would make the
// triple vanish from every read without a trace. The context is polled every
// pollEvery entries.
//
// Values are not prefetched. Badger's default iterator reads values ahead in
// background goroutines, which pays off for long scans of large values; a
// query mostly does short index lookups of a few small values, and there the
// goroutines and their wait groups cost more than the reads.
func iteratePrefix(ctx context.Context, txn *badger.Txn, prefix []byte, fn func(t term.Triple) bool) error {
	opts := badger.DefaultIteratorOptions
	opts.Prefix = prefix
	opts.PrefetchValues = false
	it := txn.NewIterator(opts)
	defer it.Close()

	n := 0
	for it.Seek(prefix); it.Valid(); it.Next() {
		if n++; n%pollEvery == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		var t term.Triple
		err := it.Item().Value(func(val []byte) error {
			var decErr error
			t, decErr = decodeTriple(val)
			return decErr
		})
		if err != nil {
			return fmt.Errorf("undecodable triple under %q: %w", prefix, err)
		}
		if !fn(t) {
			return nil
		}
	}
	return nil
}

// TriplesWithLimit returns an iterator over triples matching the pattern in the
// given context, skipping the first offset matches and yielding at most limit
// triples. limit <= 0 means no limit.
func (s *BadgerStore) TriplesWithLimit(ctx context.Context, pattern term.TriplePattern, graph term.Term, limit, offset int) store.TripleIterator {
	return func(yield func(term.Triple, error) bool) {
		if err := s.begin(ctx, false); err != nil {
			yield(term.Triple{}, err)
			return
		}
		gk := graphKey(graph)
		skipped := 0
		yielded := 0
		err := s.read(func(txn *badger.Txn) error {
			return s.scanTripleValues(ctx, txn, pattern, gk, func(t term.Triple) bool {
				if skipped < offset {
					skipped++
					return true // skip this triple
				}
				if limit > 0 && yielded >= limit {
					return false // stop scanning
				}
				yielded++
				return yield(t, nil)
			})
		})
		if err != nil {
			yield(term.Triple{}, fmt.Errorf("badgerstore: TriplesWithLimit: %w", err))
		}
	}
}

// Count returns the number of triples matching the pattern in the given context.
// For exact lookups (all three terms bound) it checks key existence only.
// For prefix-based patterns it performs a key-only scan to avoid value decoding.
func (s *BadgerStore) Count(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error) {
	if err := s.begin(ctx, false); err != nil {
		return 0, err
	}
	gk := graphKey(graph)
	count := 0
	err := s.read(func(txn *badger.Txn) error {
		// Exact lookup: just check key existence.
		if pattern.Subject != nil && pattern.Predicate != nil && pattern.Object != nil {
			ok, err := hasKey(txn, spoKey(gk, term.TermKey(pattern.Subject), term.OptPredKey(pattern.Predicate), term.TermKey(pattern.Object)))
			if ok {
				count = 1
			}
			return err
		}
		// Prefix-based scan: key-only iteration avoids value decoding.
		var err error
		count, err = countPrefix(ctx, txn, scanPrefix(pattern, gk))
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("badgerstore: Count: %w", err)
	}
	return count, nil
}

// Exists reports whether at least one triple matches the pattern in the given context.
// For exact lookups (all three terms bound) it checks key existence only.
// For prefix-based patterns it performs a key-only scan and returns on first match.
func (s *BadgerStore) Exists(ctx context.Context, pattern term.TriplePattern, graph term.Term) (bool, error) {
	if err := s.begin(ctx, false); err != nil {
		return false, err
	}
	gk := graphKey(graph)
	found := false
	err := s.read(func(txn *badger.Txn) error {
		// Exact lookup: just check key existence.
		if pattern.Subject != nil && pattern.Predicate != nil && pattern.Object != nil {
			var err error
			found, err = hasKey(txn, spoKey(gk, term.TermKey(pattern.Subject), term.OptPredKey(pattern.Predicate), term.TermKey(pattern.Object)))
			return err
		}
		// Prefix-based scan: key-only iteration, stop on first hit.
		prefix := scanPrefix(pattern, gk)
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()
		it.Seek(prefix)
		found = it.Valid()
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("badgerstore: Exists: %w", err)
	}
	return found, nil
}

// hasKey reports whether key exists; only a failure other than "not found" is
// an error.
func hasKey(txn *badger.Txn, key []byte) (bool, error) {
	_, err := txn.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Compile-time check that BadgerStore implements store.Store.
var _ store.Store = (*BadgerStore)(nil)

// Compile-time check that BadgerStore implements store.QueryableStore.
var _ store.QueryableStore = (*BadgerStore)(nil)

// Compile-time check: BadgerStore must implement SnapshotStore.
var _ store.SnapshotStore = (*BadgerStore)(nil)
