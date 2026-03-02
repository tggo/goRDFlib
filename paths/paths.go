package paths

import (
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// Path is a SPARQL-style property path that can be evaluated against a graph.
// Ported from: rdflib.paths.Path
type Path interface {
	// Eval returns matching (subject, object) pairs.
	Eval(g *graph.Graph, subj term.Subject, obj term.Term) func(yield func(term.Term, term.Term) bool)
	pathString() string
}

// --- InvPath: ^p ---
// Ported from: rdflib.paths.InvPath

type InvPath struct {
	Arg Path
}

func Inv(p Path) *InvPath {
	return &InvPath{Arg: p}
}

func (p *InvPath) pathString() string { return "^(" + p.Arg.pathString() + ")" }

func (p *InvPath) Eval(g *graph.Graph, subj term.Subject, obj term.Term) func(yield func(term.Term, term.Term) bool) {
	return func(yield func(term.Term, term.Term) bool) {
		// Swap subject/object, evaluate inner, then swap back
		var objSubj term.Subject
		if obj != nil {
			if s, ok := obj.(term.Subject); ok {
				objSubj = s
			}
		}
		p.Arg.Eval(g, objSubj, subj)(func(s, o term.Term) bool {
			return yield(o, s)
		})
	}
}

// --- SequencePath: p1/p2 ---
// Ported from: rdflib.paths.SequencePath

type SequencePath struct {
	Args []Path
}

func Sequence(paths ...Path) *SequencePath {
	return &SequencePath{Args: paths}
}

func (p *SequencePath) pathString() string {
	s := ""
	for i, a := range p.Args {
		if i > 0 {
			s += "/"
		}
		s += a.pathString()
	}
	return s
}

func (p *SequencePath) Eval(g *graph.Graph, subj term.Subject, obj term.Term) func(yield func(term.Term, term.Term) bool) {
	return func(yield func(term.Term, term.Term) bool) {
		if len(p.Args) == 0 {
			return
		}
		if len(p.Args) == 1 {
			p.Args[0].Eval(g, subj, obj)(yield)
			return
		}
		// Evaluate first path, then chain remaining
		rest := &SequencePath{Args: p.Args[1:]}
		p.Args[0].Eval(g, subj, nil)(func(s1, mid term.Term) bool {
			midSubj, ok := mid.(term.Subject)
			if !ok {
				return true
			}
			cont := true
			rest.Eval(g, midSubj, obj)(func(_, o term.Term) bool {
				if !yield(s1, o) {
					cont = false
					return false
				}
				return true
			})
			return cont
		})
	}
}

// --- AlternativePath: p1|p2 ---
// Ported from: rdflib.paths.AlternativePath

type AlternativePath struct {
	Args []Path
}

func Alternative(paths ...Path) *AlternativePath {
	return &AlternativePath{Args: paths}
}

func (p *AlternativePath) pathString() string {
	s := ""
	for i, a := range p.Args {
		if i > 0 {
			s += "|"
		}
		s += a.pathString()
	}
	return s
}

func (p *AlternativePath) Eval(g *graph.Graph, subj term.Subject, obj term.Term) func(yield func(term.Term, term.Term) bool) {
	return func(yield func(term.Term, term.Term) bool) {
		for _, alt := range p.Args {
			cont := true
			alt.Eval(g, subj, obj)(func(s, o term.Term) bool {
				if !yield(s, o) {
					cont = false
					return false
				}
				return true
			})
			if !cont {
				return
			}
		}
	}
}

// --- MulPath: p*, p+, p? ---
// Ported from: rdflib.paths.MulPath

type MulPath struct {
	Path Path
	Zero bool // include identity (zero-length)
	More bool // allow repetition (transitive closure)
}

// ZeroOrMore creates a p* path.
func ZeroOrMore(p Path) *MulPath {
	return &MulPath{Path: p, Zero: true, More: true}
}

// OneOrMore creates a p+ path.
func OneOrMore(p Path) *MulPath {
	return &MulPath{Path: p, Zero: false, More: true}
}

// ZeroOrOne creates a p? path.
func ZeroOrOne(p Path) *MulPath {
	return &MulPath{Path: p, Zero: true, More: false}
}

func (p *MulPath) pathString() string {
	mod := ""
	switch {
	case p.Zero && p.More:
		mod = "*"
	case !p.Zero && p.More:
		mod = "+"
	case p.Zero && !p.More:
		mod = "?"
	}
	return "(" + p.Path.pathString() + ")" + mod
}

func (p *MulPath) Eval(g *graph.Graph, subj term.Subject, obj term.Term) func(yield func(term.Term, term.Term) bool) {
	return func(yield func(term.Term, term.Term) bool) {
		done := make(map[string]bool)

		emit := func(s, o term.Term) bool {
			k := term.TermKey(s) + "|" + term.TermKey(o)
			if done[k] {
				return true
			}
			done[k] = true
			return yield(s, o)
		}

		if subj != nil {
			// Forward evaluation from a known subject
			if p.Zero {
				if obj == nil || term.TermKey(subj) == term.TermKey(obj) {
					if !emit(subj, subj) {
						return
					}
				}
			}
			seen := make(map[string]bool)
			p.fwdFrom(g, subj, subj, obj, seen, emit)
		} else if obj != nil {
			// Backward evaluation to a known object
			if p.Zero {
				if s, ok := obj.(term.Subject); ok {
					if !emit(obj, obj) {
						return
					}
					seen := make(map[string]bool)
					p.bwdTo(g, s, obj, seen, emit)
				}
			} else {
				if s, ok := obj.(term.Subject); ok {
					seen := make(map[string]bool)
					p.bwdTo(g, s, obj, seen, emit)
				}
			}
		} else {
			// No constraints: evaluate from all nodes
			if p.Zero {
				for _, n := range g.AllNodes() {
					if !emit(n, n) {
						return
					}
				}
			}
			for _, n := range g.AllNodes() {
				if s, ok := n.(term.Subject); ok {
					seen := make(map[string]bool)
					p.fwdFrom(g, s, s, nil, seen, emit)
				}
			}
		}
	}
}

// fwdFrom traverses forward from node, emitting (origin, reachable) pairs.
func (p *MulPath) fwdFrom(g *graph.Graph, origin term.Term, node term.Subject, obj term.Term, seen map[string]bool, emit func(term.Term, term.Term) bool) {
	k := term.TermKey(node)
	if seen[k] {
		return
	}
	seen[k] = true

	p.Path.Eval(g, node, obj)(func(_, o term.Term) bool {
		if !emit(origin, o) {
			return false
		}
		if p.More {
			if next, ok := o.(term.Subject); ok {
				p.fwdFrom(g, origin, next, obj, seen, emit)
			}
		}
		return true
	})
}

// bwdTo traverses backward from node, emitting (reachable, target) pairs.
func (p *MulPath) bwdTo(g *graph.Graph, node term.Subject, target term.Term, seen map[string]bool, emit func(term.Term, term.Term) bool) {
	k := term.TermKey(node)
	if seen[k] {
		return
	}
	seen[k] = true

	p.Path.Eval(g, nil, node)(func(s, _ term.Term) bool {
		if !emit(s, target) {
			return false
		}
		if p.More {
			if prev, ok := s.(term.Subject); ok {
				p.bwdTo(g, prev, target, seen, emit)
			}
		}
		return true
	})
}

// --- NegatedPath: !p ---
// Ported from: rdflib.paths.NegatedPath

type NegatedPath struct {
	Excluded []term.URIRef
}

func Negated(excluded ...term.URIRef) *NegatedPath {
	return &NegatedPath{Excluded: excluded}
}

func (p *NegatedPath) pathString() string {
	s := "!("
	for i, u := range p.Excluded {
		if i > 0 {
			s += "|"
		}
		s += u.N3()
	}
	return s + ")"
}

func (p *NegatedPath) Eval(g *graph.Graph, subj term.Subject, obj term.Term) func(yield func(term.Term, term.Term) bool) {
	excluded := make(map[string]bool)
	for _, u := range p.Excluded {
		excluded[term.TermKey(u)] = true
	}

	return func(yield func(term.Term, term.Term) bool) {
		g.Triples(subj, nil, obj)(func(t term.Triple) bool {
			if !excluded[term.TermKey(t.Predicate)] {
				return yield(t.Subject, t.Object)
			}
			return true
		})
	}
}

// --- URIRefPath: wraps a single URIRef as a Path ---

// URIRefPath wraps a URIRef to implement the Path interface.
type URIRefPath struct {
	URI term.URIRef
}

func (p URIRefPath) pathString() string { return p.URI.N3() }

func (p URIRefPath) Eval(g *graph.Graph, subj term.Subject, obj term.Term) func(yield func(term.Term, term.Term) bool) {
	u := p.URI
	return func(yield func(term.Term, term.Term) bool) {
		g.Triples(subj, &u, obj)(func(t term.Triple) bool {
			return yield(t.Subject, t.Object)
		})
	}
}

// --- Path construction DSL ---

// AsPath converts a URIRef to a Path.
func AsPath(u term.URIRef) URIRefPath {
	return URIRefPath{URI: u}
}

// Slash creates a SequencePath: self / other.
func (p URIRefPath) Slash(other Path) *SequencePath {
	return Sequence(p, other)
}

// Or creates an AlternativePath: self | other.
func (p URIRefPath) Or(other Path) *AlternativePath {
	return Alternative(p, other)
}
