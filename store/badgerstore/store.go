package badgerstore

import (
	"errors"
	"log"
	"sync"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// BadgerStore implements store.Store using Badger as a persistent KV backend.
// All methods are safe for concurrent use (delegated to Badger's MVCC), except
// on a read snapshot returned by ReadSnapshot.
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
// and panics on writes. Hold it only for the duration of a query: an open read
// transaction keeps old versions from being garbage-collected.
func (s *BadgerStore) ReadSnapshot() (store.Store, func()) {
	if s.snap != nil {
		return s, func() {}
	}
	txn := s.db.NewTransaction(false)
	var once sync.Once
	return &BadgerStore{db: s.db, snap: txn}, func() { once.Do(txn.Discard) }
}

// read runs fn in the snapshot's transaction, or in a new read transaction when
// s is not a snapshot.
func (s *BadgerStore) read(fn func(txn *badger.Txn) error) error {
	if s.snap != nil {
		return fn(s.snap)
	}
	return s.db.View(fn)
}

// writable panics when s is a read snapshot.
func (s *BadgerStore) writable(op string) {
	if s.snap != nil {
		panic("badgerstore: " + op + " on a read snapshot")
	}
}

// ContextAware reports true — BadgerStore supports named graphs.
func (s *BadgerStore) ContextAware() bool { return true }

// TransactionAware reports true — Badger supports ACID transactions.
func (s *BadgerStore) TransactionAware() bool { return true }

// Add inserts a triple into the store, associated with the given context.
func (s *BadgerStore) Add(t term.Triple, ctx term.Term) {
	s.writable("Add")
	gk := graphKey(ctx)
	sk := term.TermKey(t.Subject)
	pk := term.TermKey(t.Predicate)
	ok := term.TermKey(t.Object)
	val := encodeTriple(t)

	err := s.db.Update(func(txn *badger.Txn) error {
		// Check for duplicate.
		key := spoKey(gk, sk, pk, ok)
		if _, err := txn.Get(key); err == nil {
			return nil // already exists
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
	// store.Store interface does not return errors from write operations.
	if err != nil {
		log.Printf("badgerstore: Add: %v", err)
	}
}

// AddN batch-inserts multiple quads.
func (s *BadgerStore) AddN(quads []term.Quad) {
	s.writable("AddN")
	if len(quads) == 0 {
		return
	}
	wb := s.db.NewWriteBatch()
	defer wb.Cancel()

	for _, q := range quads {
		gk := graphKey(q.Graph)
		sk := term.TermKey(q.Subject)
		pk := term.TermKey(q.Predicate)
		ok := term.TermKey(q.Object)
		val := encodeTriple(q.Triple)

		if err := wb.Set(spoKey(gk, sk, pk, ok), val); err != nil {
			log.Printf("badgerstore: AddN: %v", err)
			return
		}
		if err := wb.Set(posKey(gk, pk, ok, sk), val); err != nil {
			log.Printf("badgerstore: AddN: %v", err)
			return
		}
		if err := wb.Set(ospKey(gk, ok, sk, pk), val); err != nil {
			log.Printf("badgerstore: AddN: %v", err)
			return
		}
		if gk != "" {
			if err := wb.Set(ctxKey(gk), nil); err != nil {
				log.Printf("badgerstore: AddN: %v", err)
				return
			}
		}
	}
	if err := wb.Flush(); err != nil {
		log.Printf("badgerstore: AddN flush: %v", err)
	}
}

// Remove deletes triples matching the pattern from the given context.
func (s *BadgerStore) Remove(pattern term.TriplePattern, ctx term.Term) {
	s.writable("Remove")
	gk := graphKey(ctx)

	// Collect matching triples first, then delete.
	var toRemove []tripleKeys
	err := s.read(func(txn *badger.Txn) error {
		return s.scanTriples(txn, pattern, gk, func(sk, pk, ok string, _ term.Triple) bool {
			toRemove = append(toRemove, tripleKeys{gk: gk, sk: sk, pk: pk, ok: ok})
			return true
		})
	})
	if err != nil {
		log.Printf("badgerstore: Remove scan: %v", err)
		return
	}

	if len(toRemove) == 0 {
		return
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
		log.Printf("badgerstore: Remove delete: %v", err)
	}
}

// tripleKeys holds the key components for a triple in a specific graph.
type tripleKeys struct {
	gk, sk, pk, ok string
}

// Set atomically replaces triples matching (s, p, *) with the new triple.
func (s *BadgerStore) Set(t term.Triple, ctx term.Term) {
	s.writable("Set")
	gk := graphKey(ctx)
	sk := term.TermKey(t.Subject)
	pk := term.TermKey(t.Predicate)
	val := encodeTriple(t)
	newOK := term.TermKey(t.Object)

	err := s.db.Update(func(txn *badger.Txn) error {
		// Find and delete existing (s, p, *) triples.
		pattern := term.TriplePattern{Subject: t.Subject, Predicate: &t.Predicate}
		var toDelete []string // object keys to delete
		if err := s.scanTriplesInTxn(txn, pattern, gk, func(_, _, ok string, _ term.Triple) bool {
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
		log.Printf("badgerstore: Set: %v", err)
	}
}

// Triples returns an iterator over triples matching the pattern in the given context.
func (s *BadgerStore) Triples(pattern term.TriplePattern, ctx term.Term) store.TripleIterator {
	return func(yield func(term.Triple) bool) {
		gk := graphKey(ctx)
		_ = s.read(func(txn *badger.Txn) error {
			return s.scanTripleValues(txn, pattern, gk, func(t term.Triple) bool {
				return yield(t)
			})
		})
	}
}

// Len returns the number of triples in the given context (nil = default graph).
func (s *BadgerStore) Len(ctx term.Term) int {
	gk := graphKey(ctx)
	count := 0
	_ = s.read(func(txn *badger.Txn) error {
		prefix := makePrefixKey(pfxSPO, gk)
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.Valid(); it.Next() {
			count++
		}
		return nil
	})
	return count
}

// Contexts returns an iterator over named graph URIs, optionally filtered by
// a triple that must exist in the graph.
func (s *BadgerStore) Contexts(triple *term.Triple) store.TermIterator {
	return func(yield func(term.Term) bool) {
		_ = s.read(func(txn *badger.Txn) error {
			prefix := []byte{pfxCTX, sep}
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			opts.Prefix = prefix
			it := txn.NewIterator(opts)
			defer it.Close()
			for it.Seek(prefix); it.Valid(); it.Next() {
				key := it.Item().Key()
				gk := string(key[2:]) // skip "c\x00"
				if gk == "" {
					continue
				}

				// Reconstruct the context term.
				ctxTerm, err := term.TermFromKey(gk)
				if err != nil {
					continue
				}

				// If a triple filter is specified, check membership.
				if triple != nil {
					found := false
					sk := term.TermKey(triple.Subject)
					pk := term.TermKey(triple.Predicate)
					ok := term.TermKey(triple.Object)
					checkKey := spoKey(gk, sk, pk, ok)
					if _, err := txn.Get(checkKey); err == nil {
						found = true
					}
					if !found {
						continue
					}
				}

				if !yield(ctxTerm) {
					return nil
				}
			}
			return nil
		})
	}
}

// Bind associates a prefix with a namespace URI.
func (s *BadgerStore) Bind(prefix string, namespace term.URIRef) {
	s.writable("Bind")
	err := s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(nsKey(prefix), []byte(namespace.Value())); err != nil {
			return err
		}
		return txn.Set(nuKey(namespace.Value()), []byte(prefix))
	})
	if err != nil {
		log.Printf("badgerstore: Bind: %v", err)
	}
}

// Namespace returns the namespace URI for a prefix.
func (s *BadgerStore) Namespace(prefix string) (term.URIRef, bool) {
	var ns term.URIRef
	var found bool
	_ = s.read(func(txn *badger.Txn) error {
		item, err := txn.Get(nsKey(prefix))
		if err != nil {
			return nil
		}
		return item.Value(func(val []byte) error {
			ns = term.NewURIRefUnsafe(string(val))
			found = true
			return nil
		})
	})
	return ns, found
}

// Prefix returns the prefix for a namespace URI.
func (s *BadgerStore) Prefix(namespace term.URIRef) (string, bool) {
	var prefix string
	var found bool
	_ = s.read(func(txn *badger.Txn) error {
		item, err := txn.Get(nuKey(namespace.Value()))
		if err != nil {
			return nil
		}
		return item.Value(func(val []byte) error {
			prefix = string(val)
			found = true
			return nil
		})
	})
	return prefix, found
}

// Namespaces returns an iterator over all (prefix, namespace) bindings.
func (s *BadgerStore) Namespaces() store.NamespaceIterator {
	return func(yield func(string, term.URIRef) bool) {
		_ = s.read(func(txn *badger.Txn) error {
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
					continue
				}
				if !yield(nsPrefix, term.NewURIRefUnsafe(nsURI)) {
					return nil
				}
			}
			return nil
		})
	}
}

// scanTriples scans triples matching a pattern in a read-only transaction.
func (s *BadgerStore) scanTriples(txn *badger.Txn, pattern term.TriplePattern, gk string, fn func(sk, pk, ok string, t term.Triple) bool) error {
	return s.scanTriplesInTxn(txn, pattern, gk, fn)
}

// scanTriplesInTxn performs triple scanning within a given transaction.
// It selects the optimal index based on which pattern fields are set.
func (s *BadgerStore) scanTriplesInTxn(txn *badger.Txn, pattern term.TriplePattern, gk string, fn func(sk, pk, ok string, t term.Triple) bool) error {
	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)

	var prefix []byte
	switch {
	case sk != "" && pk != "" && ok != "":
		// Exact lookup.
		key := spoKey(gk, sk, pk, ok)
		item, err := txn.Get(key)
		if err != nil {
			return nil // not found
		}
		return item.Value(func(val []byte) error {
			t, err := decodeTriple(val)
			if err != nil {
				return err
			}
			fn(sk, pk, ok, t)
			return nil
		})

	case sk != "" && pk != "":
		prefix = makePrefixKey(pfxSPO, gk, sk, pk)
	case sk != "" && ok != "":
		// Use OSP: scan o|gk|ok|sk| to find all predicates
		prefix = makePrefixKey(pfxOSP, gk, ok, sk)
	case sk != "":
		prefix = makePrefixKey(pfxSPO, gk, sk)
	case pk != "" && ok != "":
		prefix = makePrefixKey(pfxPOS, gk, pk, ok)
	case pk != "":
		prefix = makePrefixKey(pfxPOS, gk, pk)
	case ok != "":
		prefix = makePrefixKey(pfxOSP, gk, ok)
	default:
		prefix = makePrefixKey(pfxSPO, gk)
	}

	return iteratePrefix(txn, prefix, func(t term.Triple) bool {
		return fn(term.TermKey(t.Subject), term.TermKey(t.Predicate), term.TermKey(t.Object), t)
	})
}

// scanTripleValues is scanTriplesInTxn for callers that only need the triples:
// it skips building the three term keys per match, which Triples and
// TriplesWithLimit never read.
func (s *BadgerStore) scanTripleValues(txn *badger.Txn, pattern term.TriplePattern, gk string, fn func(t term.Triple) bool) error {
	if pattern.Subject != nil && pattern.Predicate != nil && pattern.Object != nil {
		return s.scanTriplesInTxn(txn, pattern, gk, func(_, _, _ string, t term.Triple) bool { return fn(t) })
	}
	return iteratePrefix(txn, scanPrefix(pattern, gk), fn)
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

// iteratePrefix decodes every triple stored under prefix until fn returns false.
//
// Values are not prefetched. Badger's default iterator reads values ahead in
// background goroutines, which pays off for long scans of large values; a
// query mostly does short index lookups of a few small values, and there the
// goroutines and their wait groups cost more than the reads.
func iteratePrefix(txn *badger.Txn, prefix []byte, fn func(t term.Triple) bool) error {
	opts := badger.DefaultIteratorOptions
	opts.Prefix = prefix
	opts.PrefetchValues = false
	it := txn.NewIterator(opts)
	defer it.Close()

	for it.Seek(prefix); it.Valid(); it.Next() {
		var t term.Triple
		err := it.Item().Value(func(val []byte) error {
			var decErr error
			t, decErr = decodeTriple(val)
			return decErr
		})
		if err != nil {
			continue
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
func (s *BadgerStore) TriplesWithLimit(pattern term.TriplePattern, ctx term.Term, limit, offset int) store.TripleIterator {
	return func(yield func(term.Triple) bool) {
		gk := graphKey(ctx)
		skipped := 0
		yielded := 0
		_ = s.read(func(txn *badger.Txn) error {
			return s.scanTripleValues(txn, pattern, gk, func(t term.Triple) bool {
				if skipped < offset {
					skipped++
					return true // skip this triple
				}
				if limit > 0 && yielded >= limit {
					return false // stop scanning
				}
				yielded++
				return yield(t)
			})
		})
	}
}

// Count returns the number of triples matching the pattern in the given context.
// For exact lookups (all three terms bound) it checks key existence only.
// For prefix-based patterns it performs a key-only scan to avoid value decoding.
func (s *BadgerStore) Count(pattern term.TriplePattern, ctx term.Term) int {
	gk := graphKey(ctx)
	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)

	count := 0
	_ = s.read(func(txn *badger.Txn) error {
		// Exact lookup: just check key existence.
		if sk != "" && pk != "" && ok != "" {
			if _, err := txn.Get(spoKey(gk, sk, pk, ok)); err == nil {
				count = 1
			}
			return nil
		}

		// Prefix-based scan: key-only iteration avoids value decoding.
		var prefix []byte
		switch {
		case sk != "" && pk != "":
			prefix = makePrefixKey(pfxSPO, gk, sk, pk)
		case sk != "" && ok != "":
			prefix = makePrefixKey(pfxOSP, gk, ok, sk)
		case sk != "":
			prefix = makePrefixKey(pfxSPO, gk, sk)
		case pk != "" && ok != "":
			prefix = makePrefixKey(pfxPOS, gk, pk, ok)
		case pk != "":
			prefix = makePrefixKey(pfxPOS, gk, pk)
		case ok != "":
			prefix = makePrefixKey(pfxOSP, gk, ok)
		default:
			prefix = makePrefixKey(pfxSPO, gk)
		}

		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.Valid(); it.Next() {
			count++
		}
		return nil
	})
	return count
}

// Exists reports whether at least one triple matches the pattern in the given context.
// For exact lookups (all three terms bound) it checks key existence only.
// For prefix-based patterns it performs a key-only scan and returns on first match.
func (s *BadgerStore) Exists(pattern term.TriplePattern, ctx term.Term) bool {
	gk := graphKey(ctx)
	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)

	found := false
	_ = s.read(func(txn *badger.Txn) error {
		// Exact lookup: just check key existence.
		if sk != "" && pk != "" && ok != "" {
			if _, err := txn.Get(spoKey(gk, sk, pk, ok)); err == nil {
				found = true
			}
			return nil
		}

		// Prefix-based scan: key-only iteration, stop on first hit.
		var prefix []byte
		switch {
		case sk != "" && pk != "":
			prefix = makePrefixKey(pfxSPO, gk, sk, pk)
		case sk != "" && ok != "":
			prefix = makePrefixKey(pfxOSP, gk, ok, sk)
		case sk != "":
			prefix = makePrefixKey(pfxSPO, gk, sk)
		case pk != "" && ok != "":
			prefix = makePrefixKey(pfxPOS, gk, pk, ok)
		case pk != "":
			prefix = makePrefixKey(pfxPOS, gk, pk)
		case ok != "":
			prefix = makePrefixKey(pfxOSP, gk, ok)
		default:
			prefix = makePrefixKey(pfxSPO, gk)
		}

		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()
		if it.Seek(prefix); it.Valid() {
			found = true
		}
		return nil
	})
	return found
}

// Compile-time check that BadgerStore implements store.Store.
var _ store.Store = (*BadgerStore)(nil)

// Compile-time check that BadgerStore implements store.QueryableStore.
var _ store.QueryableStore = (*BadgerStore)(nil)

// Compile-time check: BadgerStore must implement SnapshotStore.
var _ store.SnapshotStore = (*BadgerStore)(nil)
