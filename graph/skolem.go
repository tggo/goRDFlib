package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"

	"github.com/tggo/goRDFlib/term"
)

// DefaultSkolemBasepath is the path RDF 1.1 §3.5 registers for Skolem IRIs.
const DefaultSkolemBasepath = ".well-known/genid/"

// skolemRounds bounds signature refinement. Each round lets a signature see
// one more hop of the graph; a partition that is still splitting after this
// many rounds is used as it stands (see Skolemize).
const skolemRounds = 16

// SkolemOption configures Skolemize.
type SkolemOption func(*skolemConfig)

type skolemConfig struct {
	basepath string
	stable   bool
}

// WithSkolemBasepath replaces DefaultSkolemBasepath.
func WithSkolemBasepath(p string) SkolemOption {
	return func(c *skolemConfig) { c.basepath = p }
}

// WithStableSkolemIDs derives each Skolem IRI from the blank node's place in
// the graph instead of from its label.
//
// Blank node labels are minted fresh by every parse, so label-based Skolem IRIs
// differ each time the same document is read. With this option the same graph
// yields the same IRIs on every run, whatever the labels and the triple order:
// a node is identified by the hash of the triples around it, refined round by
// round with its blank neighbours' hashes (as in an isomorphism check).
//
// Blank nodes whose surroundings are indistinguishable get a numbered suffix.
// When they are genuinely interchangeable, as in the usual case, which one gets
// which number does not change the output. Graphs with a symmetric pattern
// beyond what the refinement can separate may number them differently between
// runs. Blank nodes inside triple terms are replaced but contribute only their
// label to the hashes.
func WithStableSkolemIDs() SkolemOption {
	return func(c *skolemConfig) { c.stable = true }
}

// Skolemize returns a copy of g with every blank node replaced by a Skolem IRI
// authority + basepath + id (RDF 1.1 §3.5). The copy keeps g's identifier and
// namespace bindings; g is not modified. DeSkolemize reverses it.
//
// Ported from: rdflib.graph.Graph.skolemize (whole-graph form), plus
// WithStableSkolemIDs.
func (g *Graph) Skolemize(authority string, opts ...SkolemOption) *Graph {
	cfg := skolemConfig{basepath: DefaultSkolemBasepath}
	for _, o := range opts {
		o(&cfg)
	}
	prefix := skolemPrefix(authority, cfg.basepath)

	triples := g.collect()
	var ids map[string]string
	if cfg.stable {
		ids = stableBNodeIDs(triples)
	}
	mint := func(b term.BNode) term.URIRef {
		id := b.Value()
		// A blank node seen only inside a triple term has no signature and
		// keeps its label.
		if stable, ok := ids[id]; ok {
			id = stable
		}
		return term.NewURIRefUnsafe(prefix + id)
	}

	out := g.emptyCopy()
	for _, t := range triples {
		out.Add(skolemSubject(t.Subject, mint), t.Predicate, skolemTerm(t.Object, mint))
	}
	return out
}

// DeSkolemize returns a copy of g in which every IRI under authority +
// basepath is replaced by a blank node, the same IRI always by the same node.
//
// Ported from: rdflib.graph.Graph.de_skolemize
func (g *Graph) DeSkolemize(authority string, opts ...SkolemOption) *Graph {
	cfg := skolemConfig{basepath: DefaultSkolemBasepath}
	for _, o := range opts {
		o(&cfg)
	}
	prefix := skolemPrefix(authority, cfg.basepath)
	nodes := make(map[string]term.BNode)
	unmint := func(t term.Term) term.Term {
		u, ok := t.(term.URIRef)
		if !ok || !strings.HasPrefix(u.Value(), prefix) {
			return t
		}
		b, ok := nodes[u.Value()]
		if !ok {
			b = term.NewBNode()
			nodes[u.Value()] = b
		}
		return b
	}
	out := g.emptyCopy()
	for _, t := range g.collect() {
		out.Add(unmint(t.Subject).(term.Subject), t.Predicate, unmint(t.Object))
	}
	return out
}

func skolemPrefix(authority, basepath string) string {
	if !strings.HasSuffix(authority, "/") {
		authority += "/"
	}
	if basepath != "" && !strings.HasSuffix(basepath, "/") {
		basepath += "/"
	}
	return authority + basepath
}

func (g *Graph) collect() []term.Triple {
	var out []term.Triple
	g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		out = append(out, t)
		return true
	})
	return out
}

func (g *Graph) emptyCopy() *Graph {
	out := NewGraph(WithIdentifier(g.Identifier()))
	g.Namespaces()(func(prefix string, ns term.URIRef) bool {
		out.Bind(prefix, ns)
		return true
	})
	return out
}

func skolemSubject(s term.Subject, mint func(term.BNode) term.URIRef) term.Subject {
	return skolemTerm(s, mint).(term.Subject)
}

func skolemTerm(t term.Term, mint func(term.BNode) term.URIRef) term.Term {
	switch v := t.(type) {
	case term.BNode:
		return mint(v)
	case term.TripleTerm:
		return term.NewTripleTerm(skolemSubject(v.Subject(), mint), v.Predicate(), skolemTerm(v.Object(), mint))
	}
	return t
}

// stableBNodeIDs maps each blank node label in triples to an id that depends
// only on the graph's structure; see WithStableSkolemIDs.
func stableBNodeIDs(triples []term.Triple) map[string]string {
	// Blank-node positions per triple, and every blank node's triples.
	incident := make(map[string][]int)
	for i, t := range triples {
		if b, ok := t.Subject.(term.BNode); ok {
			incident[b.Value()] = append(incident[b.Value()], i)
		}
		if b, ok := t.Object.(term.BNode); ok && (t.Subject != t.Object) {
			incident[b.Value()] = append(incident[b.Value()], i)
		}
	}
	labels := make([]string, 0, len(incident))
	for l := range incident {
		labels = append(labels, l)
	}
	slices.Sort(labels)

	sig := make(map[string]string, len(labels))
	for _, l := range labels {
		sig[l] = ""
	}
	// A term as seen from node self: blank nodes by their current signature,
	// self as a marker so a loop is distinguishable from an edge.
	repr := func(t term.Term, self string) string {
		if b, ok := t.(term.BNode); ok {
			if b.Value() == self {
				return "@self"
			}
			return "_:" + sig[b.Value()]
		}
		return t.N3()
	}
	classes := 1
	var parts []string
	for round := 0; round < skolemRounds; round++ {
		next := make(map[string]string, len(labels))
		for _, l := range labels {
			parts = parts[:0]
			for _, i := range incident[l] {
				t := triples[i]
				parts = append(parts, repr(t.Subject, l)+" "+t.Predicate.N3()+" "+repr(t.Object, l))
			}
			slices.Sort(parts)
			h := sha256.Sum256([]byte(strings.Join(parts, "\n")))
			next[l] = hex.EncodeToString(h[:16])
		}
		sig = next
		n := countDistinct(sig)
		if n == classes && round > 0 {
			break
		}
		classes = n
	}

	// Nodes that share a signature get suffixes in label order.
	byLabel := make(map[string]string, len(labels))
	seen := make(map[string]int, len(labels))
	for _, l := range labels {
		s := sig[l]
		k := seen[s]
		seen[s] = k + 1
		if k == 0 {
			byLabel[l] = s
		} else {
			byLabel[l] = s + "-" + strconv.Itoa(k)
		}
	}
	return byLabel
}

func countDistinct(m map[string]string) int {
	set := make(map[string]struct{}, len(m))
	for _, v := range m {
		set[v] = struct{}{}
	}
	return len(set)
}
