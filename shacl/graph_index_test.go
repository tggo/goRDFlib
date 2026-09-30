package shacl

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// indexTestTerms returns terms that share lexical values across kinds and
// literal forms: the pairs an index key must keep apart.
func indexTestTerms() []Term {
	return []Term{
		IRI("x"),
		BlankNode("x"),
		Literal("x", "", ""),
		Literal("x", "", "en"),
		Literal("x", "", "en--ltr"),
		Literal("x", "", "en--rtl"),
		Literal("x", XSD+"token", ""),
		Literal("L:x^^"+XSD+"string", "", ""), // lexical form that imitates a key
		IRI("L:x^^" + XSD + "string"),
		Literal("1", XSD+"integer", ""),
		Literal("1", XSD+"decimal", ""),
		Literal("1", XSD+"double", ""),
	}
}

// TestGraphIndexKeepsLookalikesApart (terms compare by TermKey, since the graph
// adds rdf:langString to a language-tagged literal): a term that shares its lexical value with
// another of a different kind, datatype, language or direction is a different
// term, and every index must say so.
func TestGraphIndexKeepsLookalikesApart(t *testing.T) {
	terms := indexTestTerms()
	g := NewGraph()
	pred := IRI("http://example.org/p")
	subjects := make([]Term, len(terms))
	for i, o := range terms {
		subjects[i] = IRI(fmt.Sprintf("http://example.org/s%d", i))
		g.Add(subjects[i], pred, o)
	}
	for i, o := range terms {
		subs := g.Subjects(pred, o)
		if len(subs) != 1 || subs[0].TermKey() != subjects[i].TermKey() {
			t.Errorf("Subjects(p, %s) = %v, want only %s", o, subs, subjects[i])
		}
		objs := g.Objects(subjects[i], pred)
		if len(objs) != 1 || objs[0].TermKey() != o.TermKey() {
			t.Errorf("Objects(%s, p) = %v, want only %s", subjects[i], objs, o)
		}
		for j, s := range subjects {
			if got, want := g.Has(&s, &pred, &o), i == j; got != want {
				t.Errorf("Has(%s, p, %s) = %v, want %v", s, o, got, want)
			}
		}
	}
	// The same lexical value as a subject: <x> and _:x are two nodes.
	g.Add(IRI("x"), pred, Literal("iri", "", ""))
	g.Add(BlankNode("x"), pred, Literal("bnode", "", ""))
	if got := g.Objects(IRI("x"), pred); len(got) != 1 || got[0].Value() != "iri" {
		t.Errorf("Objects(<x>, p) = %v", got)
	}
	if got := g.Objects(BlankNode("x"), pred); len(got) != 1 || got[0].Value() != "bnode" {
		t.Errorf("Objects(_:x, p) = %v", got)
	}
}

// TestGraphIndexAgreesWithScan builds random graphs out of lookalike terms and
// requires every indexed lookup to return what a scan of Triples() finds by
// comparing TermKey, the identity the indexes used before.
func TestGraphIndexAgreesWithScan(t *testing.T) {
	pool := indexTestTerms()
	preds := []Term{IRI("http://example.org/p"), IRI("http://example.org/q"), IRI("x")}
	subjects := []Term{IRI("x"), BlankNode("x"), IRI("y"), BlankNode("y")}
	rng := rand.New(rand.NewPCG(1, 2))
	for round := range 40 {
		g := NewGraph()
		for range rng.IntN(40) + 1 {
			g.Add(subjects[rng.IntN(len(subjects))], preds[rng.IntN(len(preds))], pool[rng.IntN(len(pool))])
		}
		all := g.Triples()
		same := func(a, b Term) bool { return a.TermKey() == b.TermKey() }
		count := func(match func(Triple) bool) (n int) {
			for _, tr := range all {
				if match(tr) {
					n++
				}
			}
			return n
		}
		for _, s := range subjects {
			for _, p := range preds {
				want := count(func(tr Triple) bool { return same(tr.Subject, s) && same(tr.Predicate, p) })
				if got := len(g.Objects(s, p)); got != want {
					t.Fatalf("round %d: Objects(%s, %s) = %d, scan = %d", round, s, p, got, want)
				}
				for _, o := range pool {
					want := count(func(tr Triple) bool {
						return same(tr.Subject, s) && same(tr.Predicate, p) && same(tr.Object, o)
					})
					if got := len(g.All(&s, &p, &o)); got != min(want, 1) {
						t.Fatalf("round %d: All(%s, %s, %s) = %d, scan = %d", round, s, p, o, got, want)
					}
				}
			}
		}
		for _, p := range preds {
			if got, want := len(g.All(nil, &p, nil)), count(func(tr Triple) bool { return same(tr.Predicate, p) }); got != want {
				t.Fatalf("round %d: All(_, %s, _) = %d, scan = %d", round, p, got, want)
			}
			for _, o := range pool {
				want := count(func(tr Triple) bool { return same(tr.Predicate, p) && same(tr.Object, o) })
				if got := len(g.Subjects(p, o)); got != want {
					t.Fatalf("round %d: Subjects(%s, %s) = %d, scan = %d", round, p, o, got, want)
				}
				if got := len(g.All(nil, &p, &o)); got != want {
					t.Fatalf("round %d: All(_, %s, %s) = %d, scan = %d", round, p, o, got, want)
				}
			}
		}
	}
}
