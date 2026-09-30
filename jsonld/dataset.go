package jsonld

import (
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/bnodes"
	"github.com/tggo/goRDFlib/term"

	"github.com/piprate/json-gold/ld"
)

// statement is one triple read from json-gold's output; graph names are
// dropped (see datasetStatements).
type statement struct {
	s rdflibgo.Subject
	p rdflibgo.URIRef
	o rdflibgo.Term
}

// datasetStatements converts json-gold's RDFDataset straight into statements,
// skipping the N-Quads text round trip. ok is false when any node would need
// the N-Quads parser's judgement (an ill-formed IRI, an unusual label, a bad
// language tag); the caller then takes the text path, which reports exactly
// what it always has. Graph names are ignored, as in the text path.
//
// Every "return nil, false" below is an eligibility rule with a case in
// TestDatasetFastPathEligibility; TestDatasetPathsAgreeOnParse and
// TestDatasetPathsAgreeOnCorpus compare the two paths end to end. A statement
// that could trip the text path's line-length cap also leaves the fast path,
// unless unbounded lines were requested.
func datasetStatements(ds *ld.RDFDataset, preserve, unbounded bool) (stmts []statement, ok bool) {
	n := 0
	for _, qs := range ds.Graphs {
		n += len(qs)
	}
	stmts = make([]statement, 0, n)
	scope := bnodes.New(preserve)
	iris := make(map[string]rdflibgo.URIRef, 64)
	iri := func(v string) (rdflibgo.URIRef, bool) {
		if u, hit := iris[v]; hit {
			return u, true
		}
		if !absoluteIRI(v) {
			return rdflibgo.URIRef{}, false
		}
		u, err := rdflibgo.NewURIRef(v)
		if err != nil {
			return rdflibgo.URIRef{}, false
		}
		iris[v] = u
		return u, true
	}
	bnode := func(v string) (rdflibgo.BNode, bool) {
		if !simpleBNodeLabel(v) {
			return rdflibgo.BNode{}, false
		}
		return scope.Label(v[2:]), true
	}
	for _, qs := range ds.Graphs {
		for _, q := range qs {
			var s rdflibgo.Subject
			switch sn := q.Subject.(type) {
			case ld.IRI:
				u, good := iri(sn.Value)
				if !good {
					return nil, false
				}
				s = u
			case ld.BlankNode:
				b, good := bnode(sn.Attribute)
				if !good {
					return nil, false
				}
				s = b
			default:
				return nil, false
			}
			// A blank-node predicate (generalized RDF) is not an ld.IRI, so its
			// value is empty here and fails the absolute-IRI check below.
			pn, _ := q.Predicate.(ld.IRI)
			p, good := iri(pn.Value)
			if !good {
				return nil, false
			}
			var o rdflibgo.Term
			switch on := q.Object.(type) {
			case ld.IRI:
				u, good := iri(on.Value)
				if !good {
					return nil, false
				}
				o = u
			case ld.BlankNode:
				b, good := bnode(on.Attribute)
				if !good {
					return nil, false
				}
				o = b
			case ld.Literal:
				switch {
				case on.Datatype == ld.RDFLangString:
					if !term.ValidLanguageTag(on.Language) {
						return nil, false
					}
					o = rdflibgo.NewLiteral(on.Value, rdflibgo.WithLang(on.Language))
				case on.Language != "":
					return nil, false
				case on.Datatype == ld.XSDString:
					o = rdflibgo.NewLiteral(on.Value)
				default:
					// Checked like any other IRI, as the N-Quads parser does.
					dt, good := iri(on.Datatype)
					if !good {
						return nil, false
					}
					o = rdflibgo.NewLiteral(on.Value, rdflibgo.WithDatatype(dt))
				}
			default:
				return nil, false
			}
			if !unbounded && quadSize(q) > maxFastLine {
				// The text path would enforce its line-length cap here.
				return nil, false
			}
			stmts = append(stmts, statement{s, p, o})
		}
	}
	return stmts, true
}

// absoluteIRI reports whether s starts with a URI scheme followed by ":",
// the test the N-Quads parser applies before it accepts an IRI.
func absoluteIRI(s string) bool {
	colon := strings.IndexByte(s, ':')
	if colon <= 0 {
		return false
	}
	for i := 0; i < colon; i++ {
		ch := s[i]
		letter := ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
		if i == 0 && !letter {
			return false
		}
		if !letter && !(ch >= '0' && ch <= '9') && ch != '+' && ch != '-' && ch != '.' {
			return false
		}
	}
	return true
}

// simpleBNodeLabel accepts "_:" + [A-Za-z0-9_-]+ (json-gold's own _:b0 style).
func simpleBNodeLabel(v string) bool {
	if len(v) < 3 || v[0] != '_' || v[1] != ':' {
		return false
	}
	for i := 2; i < len(v); i++ {
		c := v[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// maxFastLine is a conservative bound (half the default 64 KiB line cap,
// allowing for escaping growth) under which a statement cannot trip the
// N-Quads line-length limit.
const maxFastLine = 28 * 1024

// quadSize is the length of the variable parts of q, a lower bound on the
// length of its N-Quads line.
func quadSize(q *ld.Quad) int {
	n := len(q.Subject.GetValue()) + len(q.Predicate.GetValue()) + len(q.Object.GetValue())
	if l, ok := q.Object.(ld.Literal); ok {
		n += len(l.Datatype) + len(l.Language)
	}
	return n
}
