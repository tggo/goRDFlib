package store

import "github.com/tggo/goRDFlib/term"

// leaf is the innermost index level: third key → triple.
type leaf = smallMap[*term.Triple]

// level is the middle index level: second key → leaf.
type level = smallMap[*leaf]

// index is one of the three orderings: first key → level.
type index map[string]*level

// tripleIndex holds one graph's triples in three nested indices (SPO, POS,
// OSP). MemoryStore keeps one for the default graph and one per named graph, so
// every lookup, count and removal is confined to its graph by construction.
//
// The outer level is a Go map; the two inner levels are smallMaps, because in
// instance data they are nearly all singletons (issue #53).
//
// Not safe for concurrent use; MemoryStore guards it with its own lock.
type tripleIndex struct {
	// Keys are TermKey() strings.
	spo index // subject → predicate → object → triple
	pos index // predicate → object → subject → triple
	osp index // object → subject → predicate → triple

	count int
	// predCount holds the number of triples per predicate key, so Cardinality
	// can answer (?, p, ?) without walking every object of p.
	predCount map[string]int
}

func newTripleIndex() *tripleIndex {
	return &tripleIndex{
		spo:       make(index),
		pos:       make(index),
		osp:       make(index),
		predCount: make(map[string]int),
	}
}

// leaf returns the innermost level under k1, k2, or nil.
func (idx index) leaf(k1, k2 string) *leaf {
	l2 := idx[k1]
	if l2 == nil {
		return nil
	}
	l3, _ := l2.get(k2)
	return l3
}

// lookup returns the triple under k1, k2, k3.
func (idx index) lookup(k1, k2, k3 string) (*term.Triple, bool) {
	l3 := idx.leaf(k1, k2)
	if l3 == nil {
		return nil, false
	}
	return l3.get(k3)
}

// insert inserts t, creating intermediate levels as needed.
func (idx index) insert(k1, k2, k3 string, t *term.Triple) {
	l2 := idx[k1]
	if l2 == nil {
		l2 = &level{}
		idx[k1] = l2
	}
	l3, _ := l2.get(k2)
	if l3 == nil {
		l3 = &leaf{}
		l2.set(k2, l3)
	}
	l3.set(k3, t)
}

// delete removes the entry under k1, k2, k3 and any level it leaves empty.
func (idx index) delete(k1, k2, k3 string) {
	l2 := idx[k1]
	if l2 == nil {
		return
	}
	l3, _ := l2.get(k2)
	if l3 == nil {
		return
	}
	l3.delete(k3)
	if l3.len() == 0 {
		l2.delete(k2)
	}
	if l2.len() == 0 {
		delete(idx, k1)
	}
}

// add inserts a triple unless it is already present.
func (x *tripleIndex) add(t term.Triple) {
	sk, pk, ok := term.TermKey(t.Subject), term.TermKey(t.Predicate), term.TermKey(t.Object)

	if _, exists := x.spo.lookup(sk, pk, ok); exists {
		return
	}

	// One copy shared by all three indexes: a pointer slot is 8 bytes where
	// a Triple is 48, and most inner levels hold a single entry.
	tp := &t
	x.spo.insert(sk, pk, ok, tp)
	x.pos.insert(pk, ok, sk, tp)
	x.osp.insert(ok, sk, pk, tp)
	x.count++
	x.predCount[pk]++
}

// remove deletes a single triple from all three indices.
func (x *tripleIndex) remove(t term.Triple) {
	sk, pk, ok := term.TermKey(t.Subject), term.TermKey(t.Predicate), term.TermKey(t.Object)

	// Check existence in SPO first — only decrement count if triple actually exists
	if _, exists := x.spo.lookup(sk, pk, ok); !exists {
		return
	}
	x.spo.delete(sk, pk, ok)
	x.pos.delete(pk, ok, sk)
	x.osp.delete(ok, sk, pk)

	x.count--
	if x.predCount[pk]--; x.predCount[pk] == 0 {
		delete(x.predCount, pk)
	}
}

// removeMatching deletes every triple matching pattern.
func (x *tripleIndex) removeMatching(pattern term.TriplePattern) {
	var toRemove []term.Triple
	x.each(pattern, func(t term.Triple, _ error) bool {
		toRemove = append(toRemove, t)
		return true
	})
	for _, t := range toRemove {
		x.remove(t)
	}
}

// eachLeaf yields every triple in l.
func eachLeaf(l *leaf, yield func(term.Triple, error) bool) bool {
	if l == nil {
		return true
	}
	// Plain loops, not l.values(): this is the per-triple hot path and the
	// iterator closure cost ~8% on bound lookups.
	if l.m != nil {
		for _, t := range l.m {
			if !yield(*t, nil) {
				return false
			}
		}
		return true
	}
	for i := range l.s {
		if !yield(*l.s[i].v, nil) {
			return false
		}
	}
	return true
}

// eachLevel yields every triple under l.
func eachLevel(l *level, yield func(term.Triple, error) bool) bool {
	if l == nil {
		return true
	}
	for l3 := range l.values() {
		if !eachLeaf(l3, yield) {
			return false
		}
	}
	return true
}

// each yields the triples matching pattern until yield returns false. It
// reports whether iteration ran to the end. yield has the shape of a
// TripleIterator's (the error is always nil), so Triples hands the consumer's
// yield straight in: a wrapping closure per triple cost 25% on a plain scan.
func (x *tripleIndex) each(pattern term.TriplePattern, yield func(term.Triple, error) bool) bool {
	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)

	// Every case with two or three keys bound goes straight to the index
	// keyed by them. Iterating the outer map and filtering on the second
	// key instead turned (?, p, o) into a scan over every object of p.
	switch {
	case sk != "" && pk != "" && ok != "":
		if t, exists := x.spo.lookup(sk, pk, ok); exists {
			return yield(*t, nil)
		}
		return true
	case sk != "" && pk != "":
		return eachLeaf(x.spo.leaf(sk, pk), yield)
	case pk != "" && ok != "":
		return eachLeaf(x.pos.leaf(pk, ok), yield)
	case sk != "" && ok != "":
		return eachLeaf(x.osp.leaf(ok, sk), yield)
	case sk != "":
		return eachLevel(x.spo[sk], yield)
	case pk != "":
		return eachLevel(x.pos[pk], yield)
	case ok != "":
		return eachLevel(x.osp[ok], yield)
	default:
		for _, l2 := range x.spo {
			if !eachLevel(l2, yield) {
				return false
			}
		}
		return true
	}
}

// has reports whether the exact triple is present.
func (x *tripleIndex) has(t term.Triple) bool {
	_, exists := x.spo.lookup(term.TermKey(t.Subject), term.TermKey(t.Predicate), term.TermKey(t.Object))
	return exists
}

// levelCount counts the triples under l.
func levelCount(l *level) int {
	if l == nil {
		return 0
	}
	n := 0
	for l3 := range l.values() {
		n += l3.len()
	}
	return n
}

// leafLen is the number of triples in l.
func leafLen(l *leaf) int {
	if l == nil {
		return 0
	}
	return l.len()
}

// cardinality counts the triples matching pattern from index sizes.
func (x *tripleIndex) cardinality(pattern term.TriplePattern) int {
	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)
	switch {
	case sk != "" && pk != "" && ok != "":
		if _, exists := x.spo.lookup(sk, pk, ok); exists {
			return 1
		}
		return 0
	case sk != "" && pk != "":
		return leafLen(x.spo.leaf(sk, pk))
	case pk != "" && ok != "":
		return leafLen(x.pos.leaf(pk, ok))
	case sk != "" && ok != "":
		return leafLen(x.osp.leaf(ok, sk))
	case pk != "":
		return x.predCount[pk]
	case sk != "":
		return levelCount(x.spo[sk])
	case ok != "":
		return levelCount(x.osp[ok])
	default:
		return x.count
	}
}
