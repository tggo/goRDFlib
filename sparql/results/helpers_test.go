package results_test

import (
	"bytes"
	"fmt"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/sparql/results"
)

var (
	iri   = rdflibgo.NewURIRefUnsafe
	bnode = func(id string) rdflibgo.BNode { return rdflibgo.NewBNode(id) }
	lit   = func(s string, opts ...rdflibgo.LiteralOption) rdflibgo.Literal {
		return rdflibgo.NewLiteral(s, opts...)
	}
	typed = func(s, dt string) rdflibgo.Literal {
		return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(dt)))
	}
	triple = func(s rdflibgo.Subject, p string, o rdflibgo.Term) rdflibgo.TripleTerm {
		return rdflibgo.NewTripleTerm(s, rdflibgo.NewURIRefUnsafe(p), o)
	}
)

type row = map[string]rdflibgo.Term

func selectResult(vars []string, rows ...row) *sparql.Result {
	return &sparql.Result{Type: "SELECT", Vars: vars, Bindings: rows}
}

func write(t testing.TB, f results.Format, r *sparql.Result) string {
	t.Helper()
	var buf bytes.Buffer
	if err := results.Write(&buf, f, r); err != nil {
		t.Fatalf("Write %s: %v", f, err)
	}
	return buf.String()
}

// sameResult compares two SELECT results exactly: same variables in the same
// order, same rows in the same order, terms identical (lexical form,
// datatype, language, direction), and blank nodes related by one bijection
// over the whole document. A missing key and a nil value both mean unbound.
func sameResult(want, got *sparql.Result) error {
	if want.Type != got.Type {
		return fmt.Errorf("type %q, want %q", got.Type, want.Type)
	}
	if want.Type == "ASK" {
		if want.AskResult != got.AskResult {
			return fmt.Errorf("boolean %v, want %v", got.AskResult, want.AskResult)
		}
		return nil
	}
	if fmt.Sprint(want.Vars) != fmt.Sprint(got.Vars) {
		return fmt.Errorf("vars %v, want %v", got.Vars, want.Vars)
	}
	if len(want.Bindings) != len(got.Bindings) {
		return fmt.Errorf("%d rows, want %d", len(got.Bindings), len(want.Bindings))
	}
	b := newBijection()
	for i := range want.Bindings {
		for _, v := range want.Vars {
			w, g := want.Bindings[i][v], got.Bindings[i][v]
			if !b.equal(w, g) {
				return fmt.Errorf("row %d ?%s: got %v, want %v", i+1, v, describe(g), describe(w))
			}
		}
		for k, g := range got.Bindings[i] {
			if g != nil && want.Bindings[i][k] == nil {
				return fmt.Errorf("row %d: unexpected binding ?%s = %v", i+1, k, describe(g))
			}
		}
	}
	return nil
}

func describe(t rdflibgo.Term) string {
	if l, ok := t.(rdflibgo.Literal); ok {
		return fmt.Sprintf("%q lang=%q dir=%q dt=%s", l.Lexical(), l.Language(), l.Dir(), l.Datatype().Value())
	}
	if t == nil {
		return "unbound"
	}
	return t.N3()
}

type bijection struct{ fwd, rev map[string]string }

func newBijection() *bijection {
	return &bijection{fwd: map[string]string{}, rev: map[string]string{}}
}

func (b *bijection) equal(w, g rdflibgo.Term) bool {
	if w == nil || g == nil {
		return w == nil && g == nil
	}
	switch w := w.(type) {
	case rdflibgo.BNode:
		gb, ok := g.(rdflibgo.BNode)
		if !ok {
			return false
		}
		if m, ok := b.fwd[w.Value()]; ok {
			return m == gb.Value()
		}
		if _, ok := b.rev[gb.Value()]; ok {
			return false
		}
		b.fwd[w.Value()], b.rev[gb.Value()] = gb.Value(), w.Value()
		return true
	case rdflibgo.TripleTerm:
		gt, ok := g.(rdflibgo.TripleTerm)
		return ok && b.equal(w.Subject(), gt.Subject()) && b.equal(w.Predicate(), gt.Predicate()) && b.equal(w.Object(), gt.Object())
	}
	return w.Equal(g)
}
