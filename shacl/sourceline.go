package shacl

import (
	"github.com/tggo/goRDFlib/provenance"
	"github.com/tggo/goRDFlib/term"
)

// WithSourceLines makes validation fill in ValidationResult.SourceLine from an
// index built while the data graph was parsed.
//
//	idx := provenance.NewIndex()
//	turtle.Parse(data, r, "", turtle.WithProvenance(idx.Triple))
//	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
//
// The index must come from the *data* graph. Pointing at the shapes graph is
// not an error and not detectable — the lookups simply miss, and every result
// reports SourceLine 0.
//
// A nil index is ignored, so a caller can pass one conditionally without
// branching at the call site.
func WithSourceLines(idx *provenance.Index) Option {
	return func(c *config) { c.provenance = idx }
}

// SourceLineKind says how a result's SourceLine was arrived at, because the two
// answers are not equally precise and a caller rendering an error should be
// able to tell them apart.
type SourceLineKind int

const (
	// SourceLineNone: no line could be determined. Either no index was given,
	// or the triple was not parsed from text — inferred by SHACL-AF rules or by
	// the reasoner, or added programmatically.
	SourceLineNone SourceLineKind = iota

	// SourceLineTriple: the line of the offending triple itself. The value the
	// constraint rejected is on that line.
	SourceLineTriple

	// SourceLineFocusNode: the line where the focus node was first mentioned.
	// This is what a constraint about an absence reports — sh:minCount, a
	// missing sh:class — because the thing that is wrong was never written down
	// and so has no line of its own.
	SourceLineFocusNode
)

func (k SourceLineKind) String() string {
	switch k {
	case SourceLineTriple:
		return "triple"
	case SourceLineFocusNode:
		return "focus node"
	default:
		return "none"
	}
}

// annotateSourceLines fills SourceLine and SourceLineKind on every result,
// recursing into Details.
//
// It runs once over the finished report rather than at each of the eight places
// a ValidationResult is built. Constraints stay unaware of provenance, and a
// new constraint gets line reporting without doing anything.
func annotateSourceLines(results []ValidationResult, idx *provenance.Index) {
	if idx == nil {
		return
	}
	for i := range results {
		annotateOne(&results[i], idx)
		annotateSourceLines(results[i].Details, idx)
	}
}

func annotateOne(r *ValidationResult, idx *provenance.Index) {
	// The precise answer: the result names a focus node, a predicate path and
	// the value that failed, which together are exactly the triple in the file.
	//
	// ResultPath must be an IRI. A sequence, alternative or inverse path is a
	// blank node in the shapes graph, and the value it reached is several hops
	// from the focus node, so no single triple is the offender.
	if !r.FocusNode.IsNone() && r.ResultPath.IsIRI() && !r.Value.IsNone() {
		subj := toTerm(r.FocusNode)
		pred := toTerm(r.ResultPath)
		obj := toTerm(r.Value)
		if subj != nil && pred != nil && obj != nil {
			if line, ok := idx.Line(subj, pred, obj); ok {
				r.SourceLine = line
				r.SourceLineKind = SourceLineTriple
				return
			}
		}
	}

	// The fallback: where the focus node was first written. This is the answer
	// for every constraint about something absent — sh:minCount cannot point at
	// a triple, because the triple is the thing that is missing.
	if !r.FocusNode.IsNone() {
		if subj := toTerm(r.FocusNode); subj != nil {
			if line, ok := idx.SubjectLine(subj); ok {
				r.SourceLine = line
				r.SourceLineKind = SourceLineFocusNode
				return
			}
		}
	}

	r.SourceLine = 0
	r.SourceLineKind = SourceLineNone
}

// compile-time reminder that this file is the only place that converts a
// validation result back into the terms a provenance lookup needs.
var _ = func(t Term) term.Term { return toTerm(t) }
