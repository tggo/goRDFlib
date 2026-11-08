package shacl

import (
	"reflect"
	"testing"

	"github.com/tggo/goRDFlib/graph"
)

// An overlay hands back the base's own slice when it adds nothing to a
// lookup. A caller that appends to it must not write into the base index's
// spare capacity, where the base and every other overlay read: the first
// append would be visible to (or overwritten by) the second.
func TestOverlayReturnedSlicesAreIsolated(t *testing.T) {
	ctx := t.Context()
	// Three values: the index slice grows 1, 2, 4, so it has spare capacity.
	ttl := overlayPrefixes + `ex:r ex:part ex:a , ex:b , ex:c . ex:a ex:size 1 . ex:b ex:size 1 . ex:c ex:size 1 .
ex:p1 ex:rel ex:t . ex:p2 ex:rel ex:t . ex:p3 ex:rel ex:t .`
	base, err := NewGraphFromRDF(ctx, overlayParse(t, graph.NewGraph(), ttl), "")
	if err != nil {
		t.Fatal(err)
	}
	// Each overlay adds an unrelated triple, so lookups of ex:r / ex:t reach
	// the "overlay part is empty" branch.
	mk := func(extra string) *Graph {
		ov, err := base.Overlay(ctx, overlayTriples(t, overlayParse(t, graph.NewGraph(), overlayPrefixes+extra)))
		if err != nil {
			t.Fatal(err)
		}
		return ov
	}
	ov1, ov2 := mk(`ex:q ex:other ex:z .`), mk(`ex:q ex:other ex:y .`)
	nested := func() *Graph {
		o, err := ov1.Overlay(ctx, overlayTriples(t, overlayParse(t, graph.NewGraph(), overlayPrefixes+`ex:q2 ex:other ex:w .`)))
		if err != nil {
			t.Fatal(err)
		}
		return o
	}()

	r, part, rel, tt := IRI("http://example.org/r"), IRI("http://example.org/part"), IRI("http://example.org/rel"), IRI("http://example.org/t")
	junk1, junk2 := IRI("http://example.org/junk1"), IRI("http://example.org/junk2")

	lookups := map[string]func(g *Graph) []Term{
		"Objects":  func(g *Graph) []Term { return g.Objects(r, part) },
		"Subjects": func(g *Graph) []Term { return g.Subjects(rel, tt) },
		"All": func(g *Graph) []Term {
			var out []Term
			for _, tr := range g.All(&r, &part, nil) {
				out = append(out, tr.Object)
			}
			return out
		},
	}
	// the base index's own slices, with their spare capacity
	rawObjects := base.ensureIndexes().spo[indexKey(r)][indexKey(part)]
	rawSubjects := base.ensureIndexes().pos[indexKey(rel)][indexKey(tt)]
	if cap(rawObjects) <= len(rawObjects) || cap(rawSubjects) <= len(rawSubjects) {
		t.Fatalf("test setup: index slices have no spare capacity (%d/%d, %d/%d)",
			len(rawObjects), cap(rawObjects), len(rawSubjects), cap(rawSubjects))
	}
	spareClean := func(raw []Term) bool {
		for _, x := range raw[len(raw):cap(raw)] {
			if !reflect.DeepEqual(x, Term{}) {
				return false
			}
		}
		return true
	}

	for name, look := range lookups {
		baseBefore := append([]Term(nil), look(base)...)
		for gname, g := range map[string]*Graph{"base": base, "ov1": ov1, "ov2": ov2, "nested": nested} {
			a := look(g)
			b := look(g)
			if len(a) != len(baseBefore) {
				t.Fatalf("%s/%s: got %d, want %d", name, gname, len(a), len(baseBefore))
			}
			a = append(a, junk1)
			b = append(b, junk2)
			if a[len(a)-1] != junk1 {
				t.Errorf("%s/%s: an append was overwritten by another caller's append", name, gname)
			}
			if !reflect.DeepEqual(look(g), baseBefore) {
				t.Errorf("%s/%s: lookup changed after a caller appended", name, gname)
			}
		}
		for gname, g := range map[string]*Graph{"base": base, "ov1": ov1, "ov2": ov2, "nested": nested} {
			if got := look(g); !reflect.DeepEqual(got, baseBefore) {
				t.Errorf("%s/%s: %v, want %v", name, gname, got, baseBefore)
			}
		}
	}
	if !spareClean(rawObjects) || !spareClean(rawSubjects) {
		t.Error("an append wrote into the base index's spare capacity")
	}
}
