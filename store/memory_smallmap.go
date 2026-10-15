package store

import "iter"

// smallMapMax is the number of entries a smallMap keeps in a slice before it
// switches to a Go map. A Go map allocates a group of 8 slots up front, so
// below this size a linear scan is both smaller and about as fast.
const smallMapMax = 8

// smallMap is a string-keyed map that holds up to smallMapMax entries in a
// slice and promotes itself to a Go map beyond that. The inner levels of the
// MemoryStore indexes are almost all singletons in instance data (a unique
// literal object has one subject and one predicate), and a singleton Go map
// cost several hundred bytes where this costs a few dozen (issue #53).
//
// Iteration order is unspecified. Not safe for concurrent use.
type smallMap[V any] struct {
	s []smallEntry[V]
	m map[string]V // non-nil once promoted; s is nil from then on
}

type smallEntry[V any] struct {
	k string
	v V
}

func (x *smallMap[V]) len() int {
	if x.m != nil {
		return len(x.m)
	}
	return len(x.s)
}

func (x *smallMap[V]) get(k string) (V, bool) {
	if x.m != nil {
		v, ok := x.m[k]
		return v, ok
	}
	for i := range x.s {
		if x.s[i].k == k {
			return x.s[i].v, true
		}
	}
	var zero V
	return zero, false
}

func (x *smallMap[V]) set(k string, v V) {
	if x.m != nil {
		x.m[k] = v
		return
	}
	for i := range x.s {
		if x.s[i].k == k {
			x.s[i].v = v
			return
		}
	}
	if len(x.s) < smallMapMax {
		x.s = append(x.s, smallEntry[V]{k, v})
		return
	}
	x.m = make(map[string]V, 2*smallMapMax)
	for _, e := range x.s {
		x.m[e.k] = e.v
	}
	x.m[k] = v
	x.s = nil
}

func (x *smallMap[V]) delete(k string) {
	if x.m != nil {
		delete(x.m, k)
		return
	}
	for i := range x.s {
		if x.s[i].k == k {
			last := len(x.s) - 1
			x.s[i] = x.s[last]
			x.s[last] = smallEntry[V]{} // drop references for the GC
			x.s = x.s[:last]
			return
		}
	}
}

// values yields every value.
func (x *smallMap[V]) values() iter.Seq[V] {
	return func(yield func(V) bool) {
		if x.m != nil {
			for _, v := range x.m {
				if !yield(v) {
					return
				}
			}
			return
		}
		for i := range x.s {
			if !yield(x.s[i].v) {
				return
			}
		}
	}
}

// all yields every key and value. A nil smallMap yields nothing.
func (x *smallMap[V]) all() iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		if x == nil {
			return
		}
		if x.m != nil {
			for k, v := range x.m {
				if !yield(k, v) {
					return
				}
			}
			return
		}
		for i := range x.s {
			if !yield(x.s[i].k, x.s[i].v) {
				return
			}
		}
	}
}
