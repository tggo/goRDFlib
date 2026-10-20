package store

import "github.com/tggo/goRDFlib/term"

// leaf is the innermost index level: third key → triple.
type leaf = smallMap[*term.Triple]

// node is what a middle level holds under a second key: the triple itself
// while there is exactly one, a leaf from the second on. In instance data
// almost every middle entry is a singleton, and a leaf for it cost a struct
// and a slice (~56 bytes) per index per triple; the triple already carries
// the third key, since every TermKey is cached on its term.
type node struct {
	one  *term.Triple // set when the entry holds a single triple
	many *leaf        // set otherwise
}

// level is the middle index level: second key → node.
type level = smallMap[node]

// top is what an index holds under a first key, on the same principle as
// node: a unique subject or object needs no level of its own.
type top struct {
	one  *term.Triple
	many *level
}

// Positions of a term in a triple, naming an index's second and third keys.
const (
	posS = iota
	posP
	posO
)

// keyAt returns the TermKey of t's term at pos. It is the key the triple was
// inserted under, since both come from TermKey of the same term. Terms built
// by their constructors cache the key, so this does not allocate; a
// hand-built Literal recomputes it, which is slower but still the same key.
func keyAt(t *term.Triple, pos uint8) string {
	switch pos {
	case posS:
		return term.TermKey(t.Subject)
	case posP:
		return term.TermKey(t.Predicate)
	default:
		return term.TermKey(t.Object)
	}
}

// index is one of the three orderings: first key → second key → third key.
type index struct {
	m      map[string]top
	p2, p3 uint8 // positions of the second and third keys
}

func newIndex(p2, p3 uint8) index {
	return index{m: make(map[string]top), p2: p2, p3: p3}
}

// tripleIndex holds one graph's triples in three nested indices (SPO, POS,
// OSP). MemoryStore keeps one for the default graph and one per named graph, so
// every lookup, count and removal is confined to its graph by construction.
//
// The outer level is a Go map; the two inner levels are smallMaps, because in
// instance data they are nearly all singletons (issue #53), and a singleton is
// held as the triple itself (top, node).
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
		spo:       newIndex(posP, posO),
		pos:       newIndex(posO, posS),
		osp:       newIndex(posS, posP),
		predCount: make(map[string]int),
	}
}

// lookup returns the triple under k1, k2, k3.
func (idx index) lookup(k1, k2, k3 string) (*term.Triple, bool) {
	e, ok := idx.m[k1]
	if !ok {
		return nil, false
	}
	if e.one != nil {
		if keyAt(e.one, idx.p2) == k2 && keyAt(e.one, idx.p3) == k3 {
			return e.one, true
		}
		return nil, false
	}
	n, ok := e.many.get(k2)
	if !ok {
		return nil, false
	}
	return n.lookup(k3, idx.p3)
}

func (n node) lookup(k3 string, p3 uint8) (*term.Triple, bool) {
	if n.one != nil {
		if keyAt(n.one, p3) == k3 {
			return n.one, true
		}
		return nil, false
	}
	return n.many.get(k3)
}

// pair returns the node under k1, k2; its zero value if there is none.
func (idx index) pair(k1, k2 string) node {
	e, ok := idx.m[k1]
	if !ok {
		return node{}
	}
	if e.one != nil {
		if keyAt(e.one, idx.p2) == k2 {
			return node{one: e.one}
		}
		return node{}
	}
	n, _ := e.many.get(k2)
	return n
}

// insert inserts t, which must not be present yet, creating levels as needed.
func (idx index) insert(k1 string, t *term.Triple) {
	e, ok := idx.m[k1]
	if !ok {
		idx.m[k1] = top{one: t}
		return
	}
	if e.one != nil {
		l2 := &level{}
		l2.set(keyAt(e.one, idx.p2), node{one: e.one})
		e = top{many: l2}
		idx.m[k1] = e
	}
	k2 := keyAt(t, idx.p2)
	n, ok := e.many.get(k2)
	switch {
	case !ok:
		n = node{one: t}
	case n.one != nil:
		l3 := &leaf{}
		l3.set(keyAt(n.one, idx.p3), n.one)
		l3.set(keyAt(t, idx.p3), t)
		n = node{many: l3}
	default:
		n.many.set(keyAt(t, idx.p3), t)
		return
	}
	e.many.set(k2, n)
}

// delete removes the triple under k1, k2, k3, which must be present, and
// folds a level left with one triple back into that triple.
func (idx index) delete(k1, k2, k3 string) {
	e := idx.m[k1]
	if e.one != nil {
		delete(idx.m, k1)
		return
	}
	n, _ := e.many.get(k2)
	if n.one != nil {
		e.many.delete(k2)
	} else {
		n.many.delete(k3)
		if n.many.len() == 1 {
			for _, t := range n.many.all() {
				e.many.set(k2, node{one: t})
			}
		}
	}
	switch e.many.len() {
	case 0:
		delete(idx.m, k1)
	case 1:
		for _, n := range e.many.all() {
			if n.one != nil {
				idx.m[k1] = top{one: n.one}
			}
		}
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
	x.spo.insert(sk, tp)
	x.pos.insert(pk, tp)
	x.osp.insert(ok, tp)
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

// eachNode yields every triple in n.
func eachNode(n node, yield func(term.Triple, error) bool) bool {
	if n.one != nil {
		return yield(*n.one, nil)
	}
	l := n.many
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

// eachTop yields every triple under e.
func eachTop(e top, yield func(term.Triple, error) bool) bool {
	if e.one != nil {
		return yield(*e.one, nil)
	}
	if e.many == nil {
		return true
	}
	for n := range e.many.values() {
		if !eachNode(n, yield) {
			return false
		}
	}
	return true
}

// under yields every triple under k1, as a pointer into the index.
func (idx index) under(k1 string) func(func(*term.Triple) bool) {
	return func(yield func(*term.Triple) bool) {
		eachTop(idx.m[k1], func(t term.Triple, _ error) bool {
			return yield(&t)
		})
	}
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
		return eachNode(x.spo.pair(sk, pk), yield)
	case pk != "" && ok != "":
		return eachNode(x.pos.pair(pk, ok), yield)
	case sk != "" && ok != "":
		return eachNode(x.osp.pair(ok, sk), yield)
	case sk != "":
		return eachTop(x.spo.m[sk], yield)
	case pk != "":
		return eachTop(x.pos.m[pk], yield)
	case ok != "":
		return eachTop(x.osp.m[ok], yield)
	default:
		for _, e := range x.spo.m {
			if !eachTop(e, yield) {
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

// len is the number of triples in n.
func (n node) len() int {
	if n.one != nil {
		return 1
	}
	if n.many == nil {
		return 0
	}
	return n.many.len()
}

// len is the number of triples under e.
func (e top) len() int {
	if e.one != nil {
		return 1
	}
	if e.many == nil {
		return 0
	}
	c := 0
	for n := range e.many.values() {
		c += n.len()
	}
	return c
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
		return x.spo.pair(sk, pk).len()
	case pk != "" && ok != "":
		return x.pos.pair(pk, ok).len()
	case sk != "" && ok != "":
		return x.osp.pair(ok, sk).len()
	case pk != "":
		return x.predCount[pk]
	case sk != "":
		return x.spo.m[sk].len()
	case ok != "":
		return x.osp.m[ok].len()
	default:
		return x.count
	}
}
