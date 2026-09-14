package store

import "github.com/tggo/goRDFlib/term"

// tripleIndex holds one graph's triples in three nested-map indices (SPO, POS,
// OSP). MemoryStore keeps one for the default graph and one per named graph, so
// every lookup, count and removal is confined to its graph by construction.
//
// Not safe for concurrent use; MemoryStore guards it with its own lock.
type tripleIndex struct {
	// Keys are TermKey() strings for map-key compatibility.
	spo map[string]map[string]map[string]term.Triple // subject → predicate → object → triple
	pos map[string]map[string]map[string]term.Triple // predicate → object → subject → triple
	osp map[string]map[string]map[string]term.Triple // object → subject → predicate → triple

	count int
	// predCount holds the number of triples per predicate key, so Cardinality
	// can answer (?, p, ?) without walking every object of p.
	predCount map[string]int
}

func newTripleIndex() *tripleIndex {
	return &tripleIndex{
		spo:       make(map[string]map[string]map[string]term.Triple),
		pos:       make(map[string]map[string]map[string]term.Triple),
		osp:       make(map[string]map[string]map[string]term.Triple),
		predCount: make(map[string]int),
	}
}

// add inserts a triple unless it is already present.
func (x *tripleIndex) add(t term.Triple) {
	sk, pk, ok := term.TermKey(t.Subject), term.TermKey(t.Predicate), term.TermKey(t.Object)

	if _, exists := x.spo[sk][pk][ok]; exists {
		return
	}

	ensureInsert(x.spo, sk, pk, ok, t)
	ensureInsert(x.pos, pk, ok, sk, t)
	ensureInsert(x.osp, ok, sk, pk, t)
	x.count++
	x.predCount[pk]++
}

// ensureInsert inserts t into a 3-level nested map, creating intermediate maps as needed.
func ensureInsert(idx map[string]map[string]map[string]term.Triple, k1, k2, k3 string, t term.Triple) {
	if idx[k1] == nil {
		idx[k1] = make(map[string]map[string]term.Triple)
	}
	if idx[k1][k2] == nil {
		idx[k1][k2] = make(map[string]term.Triple)
	}
	idx[k1][k2][k3] = t
}

// remove deletes a single triple from all three indices.
func (x *tripleIndex) remove(t term.Triple) {
	sk, pk, ok := term.TermKey(t.Subject), term.TermKey(t.Predicate), term.TermKey(t.Object)

	// Check existence in SPO first — only decrement count if triple actually exists
	po, exists := x.spo[sk]
	if !exists {
		return
	}
	o, exists := po[pk]
	if !exists {
		return
	}
	if _, exists := o[ok]; !exists {
		return
	}
	delete(o, ok)
	if len(o) == 0 {
		delete(po, pk)
	}
	if len(po) == 0 {
		delete(x.spo, sk)
	}

	if os, exists := x.pos[pk]; exists {
		if s, exists := os[ok]; exists {
			delete(s, sk)
			if len(s) == 0 {
				delete(os, ok)
			}
			if len(os) == 0 {
				delete(x.pos, pk)
			}
		}
	}

	if sp, exists := x.osp[ok]; exists {
		if p, exists := sp[sk]; exists {
			delete(p, pk)
			if len(p) == 0 {
				delete(sp, sk)
			}
			if len(sp) == 0 {
				delete(x.osp, ok)
			}
		}
	}

	x.count--
	if x.predCount[pk]--; x.predCount[pk] == 0 {
		delete(x.predCount, pk)
	}
}

// removeMatching deletes every triple matching pattern.
func (x *tripleIndex) removeMatching(pattern term.TriplePattern) {
	var toRemove []term.Triple
	x.each(pattern, func(t term.Triple) bool {
		toRemove = append(toRemove, t)
		return true
	})
	for _, t := range toRemove {
		x.remove(t)
	}
}

// each yields the triples matching pattern until yield returns false. It
// reports whether iteration ran to the end.
func (x *tripleIndex) each(pattern term.TriplePattern, yield func(term.Triple) bool) bool {
	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)

	// Every case with two or three keys bound goes straight to the index
	// keyed by them. Iterating the outer map and filtering on the second
	// key instead turned (?, p, o) into a scan over every object of p.
	switch {
	case sk != "" && pk != "" && ok != "":
		if t, exists := x.spo[sk][pk][ok]; exists {
			return yield(t)
		}

	case sk != "" && pk != "":
		for _, t := range x.spo[sk][pk] {
			if !yield(t) {
				return false
			}
		}

	case pk != "" && ok != "":
		for _, t := range x.pos[pk][ok] {
			if !yield(t) {
				return false
			}
		}

	case sk != "" && ok != "":
		for _, t := range x.osp[ok][sk] {
			if !yield(t) {
				return false
			}
		}

	case sk != "":
		for _, o := range x.spo[sk] {
			for _, t := range o {
				if !yield(t) {
					return false
				}
			}
		}

	case pk != "":
		for _, s := range x.pos[pk] {
			for _, t := range s {
				if !yield(t) {
					return false
				}
			}
		}

	case ok != "":
		for _, p := range x.osp[ok] {
			for _, t := range p {
				if !yield(t) {
					return false
				}
			}
		}

	default:
		for _, po := range x.spo {
			for _, o := range po {
				for _, t := range o {
					if !yield(t) {
						return false
					}
				}
			}
		}
	}
	return true
}

// has reports whether the exact triple is present.
func (x *tripleIndex) has(t term.Triple) bool {
	_, exists := x.spo[term.TermKey(t.Subject)][term.TermKey(t.Predicate)][term.TermKey(t.Object)]
	return exists
}

// cardinality counts the triples matching pattern from index sizes.
func (x *tripleIndex) cardinality(pattern term.TriplePattern) int {
	sk := term.OptTermKey(pattern.Subject)
	pk := term.OptPredKey(pattern.Predicate)
	ok := term.OptTermKey(pattern.Object)
	switch {
	case sk != "" && pk != "" && ok != "":
		if _, exists := x.spo[sk][pk][ok]; exists {
			return 1
		}
		return 0
	case sk != "" && pk != "":
		return len(x.spo[sk][pk])
	case pk != "" && ok != "":
		return len(x.pos[pk][ok])
	case sk != "" && ok != "":
		return len(x.osp[ok][sk])
	case pk != "":
		return x.predCount[pk]
	case sk != "":
		n := 0
		for _, o := range x.spo[sk] {
			n += len(o)
		}
		return n
	case ok != "":
		n := 0
		for _, p := range x.osp[ok] {
			n += len(p)
		}
		return n
	default:
		return x.count
	}
}
