package testutil

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// graphsEqual runs AssertGraphEqual on a throwaway T and reports its verdict.
func graphsEqual(exp, act *graph.Graph) bool {
	rec := &testing.T{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		AssertGraphEqual(rec, exp, act)
	}()
	<-done
	return !rec.Failed()
}

func TestAssertGraphEqualSeesDatatypeAndLanguage(t *testing.T) {
	s := term.NewURIRefUnsafe("urn:s")
	p := term.NewURIRefUnsafe("urn:p")
	one := func(o term.Term, bnodeSubject bool) *graph.Graph {
		g := graph.NewGraph()
		if bnodeSubject {
			g.Add(term.NewBNode("b"), p, o)
		} else {
			g.Add(s, p, o)
		}
		return g
	}
	pairs := []struct {
		name string
		a, b term.Literal
	}{
		{"decimal vs double with the same shorthand", term.NewLiteral("1.5e3", term.WithDatatype(term.XSDDecimal)), term.NewLiteral("1.5e3", term.WithDatatype(term.XSDDouble))},
		{"language vs direction", term.NewLiteral("x", term.WithLang("ar")), term.NewLiteral("x", term.WithLang("ar"), term.WithDir("rtl"))},
		{"integer vs string", term.NewLiteral("1", term.WithDatatype(term.XSDInteger)), term.NewLiteral("1")},
	}
	for _, pr := range pairs {
		for _, bnode := range []bool{false, true} {
			if graphsEqual(one(pr.a, bnode), one(pr.b, bnode)) {
				t.Errorf("%s (blank-node subject %v): graphs compared equal", pr.name, bnode)
			}
		}
		if !graphsEqual(one(pr.a, false), one(pr.a, false)) {
			t.Errorf("%s: identical graphs compared unequal", pr.name)
		}
	}
}
