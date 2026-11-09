package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/tggo/goRDFlib/term"
)

// ErrReadOnly is returned by every write of a store that cannot be written to,
// such as a UnionStore.
var ErrReadOnly = errors.New("store: read-only")

// UnionStore is a read-only Store that reads as the union of two stores: a
// base, which is typically large and shared, and a small over store holding
// extra triples. It lets many short-lived views share one big store without
// copying it or writing into it:
//
//	view := store.NewUnion(shared, extra) // fill extra first
//
// Reads follow the Store conventions for each layer separately and merge the
// results: Triples yields the base's matches, then the over store's matches
// the base does not hold (a triple present in both layers is one triple, not
// two); Len counts the same set; Contexts and Namespaces are the union of both
// layers, the base winning a prefix both bind. A call made with a done context
// returns its error, as for every Store.
//
// Every write (Add, AddN, Remove, Set, Bind) returns ErrReadOnly. The caller
// fills the over store before building the union and must not write to either
// layer while the union is read, nor mutate the base while views are in use:
// the union has no snapshot of its own.
//
// The union implements none of the optional interfaces (QueryableStore,
// CardinalityStore, ...): their promises do not survive dedup between layers.
// The planner and the engines fall back to Triples.
//
// UnionStore is safe for concurrent use if both layers are. Dedup costs one
// bound lookup in the base per triple of the over store that a read visits, so
// it is meant for an over store that is small.
type UnionStore struct {
	base, over Store
}

// NewUnion returns a read-only store over the union of base and over. Both
// must be non-nil.
func NewUnion(base, over Store) *UnionStore {
	if base == nil || over == nil {
		panic("store: NewUnion needs two stores")
	}
	return &UnionStore{base: base, over: over}
}

// ContextAware reports whether the base supports named graphs.
func (u *UnionStore) ContextAware() bool { return u.base.ContextAware() }

// TransactionAware is false: a union has no transactions.
func (u *UnionStore) TransactionAware() bool { return false }

// held reports whether the base holds t in graph.
func (u *UnionStore) held(ctx context.Context, t term.Triple, graph term.Term) (bool, error) {
	p := t.Predicate
	for _, err := range u.base.Triples(ctx, term.TriplePattern{Subject: t.Subject, Predicate: &p, Object: t.Object}, graph) {
		return err == nil, err
	}
	return false, nil
}

// Triples yields the base's matches, then the over store's matches that the
// base does not hold.
func (u *UnionStore) Triples(ctx context.Context, pattern term.TriplePattern, graph term.Term) TripleIterator {
	if err := ctx.Err(); err != nil {
		return ErrorIterator[term.Triple](err)
	}
	return func(yield func(term.Triple, error) bool) {
		for t, err := range u.base.Triples(ctx, pattern, graph) {
			if !yield(t, err) || err != nil {
				return
			}
		}
		for t, err := range u.over.Triples(ctx, pattern, graph) {
			if err != nil {
				yield(term.Triple{}, err)
				return
			}
			dup, err := u.held(ctx, t, graph)
			if err != nil {
				yield(term.Triple{}, err)
				return
			}
			if dup {
				continue
			}
			if !yield(t, nil) {
				return
			}
		}
	}
}

// Len counts the distinct triples of both layers in graph.
func (u *UnionStore) Len(ctx context.Context, graph term.Term) (int, error) {
	n, err := u.base.Len(ctx, graph)
	if err != nil {
		return 0, err
	}
	for t, err := range u.over.Triples(ctx, term.TriplePattern{}, graph) {
		if err != nil {
			return 0, err
		}
		dup, err := u.held(ctx, t, graph)
		if err != nil {
			return 0, err
		}
		if !dup {
			n++
		}
	}
	return n, nil
}

// Contexts yields the named graphs of both layers, each once.
func (u *UnionStore) Contexts(ctx context.Context, triple *term.Triple) TermIterator {
	if err := ctx.Err(); err != nil {
		return ErrorIterator[term.Term](err)
	}
	return func(yield func(term.Term, error) bool) {
		seen := make(map[string]struct{})
		for _, layer := range [2]Store{u.base, u.over} {
			for c, err := range layer.Contexts(ctx, triple) {
				if err != nil {
					yield(nil, err)
					return
				}
				k := term.TermKey(c)
				if _, dup := seen[k]; dup {
					continue
				}
				seen[k] = struct{}{}
				if !yield(c, nil) {
					return
				}
			}
		}
	}
}

// Namespace looks the prefix up in the base, then in the over store.
func (u *UnionStore) Namespace(ctx context.Context, prefix string) (term.URIRef, bool, error) {
	if ns, ok, err := u.base.Namespace(ctx, prefix); err != nil || ok {
		return ns, ok, err
	}
	return u.over.Namespace(ctx, prefix)
}

// Prefix looks the namespace up in the base, then in the over store.
func (u *UnionStore) Prefix(ctx context.Context, namespace term.URIRef) (string, bool, error) {
	if p, ok, err := u.base.Prefix(ctx, namespace); err != nil || ok {
		return p, ok, err
	}
	return u.over.Prefix(ctx, namespace)
}

// Namespaces yields the base's bindings, then the over store's whose prefix
// the base does not bind.
func (u *UnionStore) Namespaces(ctx context.Context) NamespaceIterator {
	if err := ctx.Err(); err != nil {
		return ErrorIterator[NamespaceBinding](err)
	}
	return func(yield func(NamespaceBinding, error) bool) {
		seen := make(map[string]struct{})
		for _, layer := range [2]Store{u.base, u.over} {
			for b, err := range layer.Namespaces(ctx) {
				if err != nil {
					yield(NamespaceBinding{}, err)
					return
				}
				if _, dup := seen[b.Prefix]; dup {
					continue
				}
				seen[b.Prefix] = struct{}{}
				if !yield(b, nil) {
					return
				}
			}
		}
	}
}

func (u *UnionStore) refuse(ctx context.Context, what string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%w: %s on a union of two stores", ErrReadOnly, what)
}

// Add returns ErrReadOnly.
func (u *UnionStore) Add(ctx context.Context, _ term.Triple, _ term.Term) error {
	return u.refuse(ctx, "Add")
}

// AddN returns ErrReadOnly.
func (u *UnionStore) AddN(ctx context.Context, _ []term.Quad) error {
	return u.refuse(ctx, "AddN")
}

// Remove returns ErrReadOnly.
func (u *UnionStore) Remove(ctx context.Context, _ term.TriplePattern, _ term.Term) error {
	return u.refuse(ctx, "Remove")
}

// Set returns ErrReadOnly.
func (u *UnionStore) Set(ctx context.Context, _ term.Triple, _ term.Term) error {
	return u.refuse(ctx, "Set")
}

// Bind returns ErrReadOnly.
func (u *UnionStore) Bind(ctx context.Context, _ string, _ term.URIRef) error {
	return u.refuse(ctx, "Bind")
}
