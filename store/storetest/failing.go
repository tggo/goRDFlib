package storetest

import (
	"context"
	"errors"
	"sync"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// ErrInjected is the error a FailingStore returns unless told otherwise.
var ErrInjected = errors.New("storetest: injected store failure")

// FailingStore wraps a Store and makes it fail on demand, so a layer built on
// a store can prove that a store error reaches its caller instead of reading
// as an empty graph (issue #52). It hides the wrapped store's optional
// interfaces; engines then take their general path.
//
// Safe for concurrent use if the wrapped store is.
type FailingStore struct {
	store.Store

	mu sync.Mutex
	// err fails every call when set.
	err error
	// readErr, when set, fails Triples once readsLeft triples have been
	// yielded in total, so an iteration can fail midway.
	readErr   error
	readsLeft int
}

// NewFailingStore wraps s; it behaves exactly like s until told to fail.
func NewFailingStore(s store.Store) *FailingStore {
	return &FailingStore{Store: s}
}

// Fail makes every following call return err (ErrInjected if nil).
func (f *FailingStore) Fail(err error) {
	if err == nil {
		err = ErrInjected
	}
	f.mu.Lock()
	f.err = err
	f.mu.Unlock()
}

// FailReadsAfter lets Triples yield n more triples in total, across calls,
// and then fail with err (ErrInjected if nil). Every other method keeps
// working, which is the shape of a connection lost in the middle of a read.
func (f *FailingStore) FailReadsAfter(n int, err error) {
	if err == nil {
		err = ErrInjected
	}
	f.mu.Lock()
	f.readErr, f.readsLeft = err, n
	f.mu.Unlock()
}

func (f *FailingStore) failure() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// takeRead accounts for one triple about to be yielded and returns the error
// to yield instead, if the read budget is spent.
func (f *FailingStore) takeRead() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if f.readErr == nil {
		return nil
	}
	if f.readsLeft <= 0 {
		return f.readErr
	}
	f.readsLeft--
	return nil
}

func (f *FailingStore) Add(ctx context.Context, t term.Triple, g term.Term) error {
	if err := f.failure(); err != nil {
		return err
	}
	return f.Store.Add(ctx, t, g)
}

func (f *FailingStore) AddN(ctx context.Context, quads []term.Quad) error {
	if err := f.failure(); err != nil {
		return err
	}
	return f.Store.AddN(ctx, quads)
}

func (f *FailingStore) Remove(ctx context.Context, p term.TriplePattern, g term.Term) error {
	if err := f.failure(); err != nil {
		return err
	}
	return f.Store.Remove(ctx, p, g)
}

func (f *FailingStore) Set(ctx context.Context, t term.Triple, g term.Term) error {
	if err := f.failure(); err != nil {
		return err
	}
	return f.Store.Set(ctx, t, g)
}

func (f *FailingStore) Triples(ctx context.Context, p term.TriplePattern, g term.Term) store.TripleIterator {
	if err := f.failure(); err != nil {
		return store.ErrorIterator[term.Triple](err)
	}
	inner := f.Store.Triples(ctx, p, g)
	return func(yield func(term.Triple, error) bool) {
		for t, err := range inner {
			if err == nil {
				err = f.takeRead()
			}
			if err != nil {
				yield(term.Triple{}, err)
				return
			}
			if !yield(t, nil) {
				return
			}
		}
	}
}

func (f *FailingStore) Len(ctx context.Context, g term.Term) (int, error) {
	if err := f.failure(); err != nil {
		return 0, err
	}
	return f.Store.Len(ctx, g)
}

func (f *FailingStore) Contexts(ctx context.Context, t *term.Triple) store.TermIterator {
	if err := f.failure(); err != nil {
		return store.ErrorIterator[term.Term](err)
	}
	return f.Store.Contexts(ctx, t)
}

func (f *FailingStore) Bind(ctx context.Context, prefix string, ns term.URIRef) error {
	if err := f.failure(); err != nil {
		return err
	}
	return f.Store.Bind(ctx, prefix, ns)
}

func (f *FailingStore) Namespace(ctx context.Context, prefix string) (term.URIRef, bool, error) {
	if err := f.failure(); err != nil {
		return term.URIRef{}, false, err
	}
	return f.Store.Namespace(ctx, prefix)
}

func (f *FailingStore) Prefix(ctx context.Context, ns term.URIRef) (string, bool, error) {
	if err := f.failure(); err != nil {
		return "", false, err
	}
	return f.Store.Prefix(ctx, ns)
}

func (f *FailingStore) Namespaces(ctx context.Context) store.NamespaceIterator {
	if err := f.failure(); err != nil {
		return store.ErrorIterator[store.NamespaceBinding](err)
	}
	return f.Store.Namespaces(ctx)
}

var _ store.Store = (*FailingStore)(nil)
