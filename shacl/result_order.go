package shacl

import (
	"sort"
	"strings"
)

// Report order.
//
// SHACL 1.0 §3.6 defines the validation report as an RDF graph, so its results
// have no order. The Go report is a slice, though, and callers print it, diff
// it and compare it in tests. Its order used to follow Go map iteration — over
// the parsed shapes and over the graph indexes — and changed from one run to
// the next on the same input (pySHACL #304).
//
// Validate therefore sorts the finished report, and every Details slice in it,
// with orderResults.

// orderResults sorts results, and recursively their details, into a stable
// order: by focus node, then result path, constraint component, value,
// severity, messages, source shape and source constraint.
//
// Blank node labels are compared last. A parser gives blank nodes fresh labels
// on every parse, so a label cannot order results the same way across two
// loads of one document; everything else can. Two results that differ only in
// blank node labels are still ordered consistently for one pair of loaded
// graphs.
func orderResults(results []ValidationResult) {
	if len(results) == 0 {
		return
	}
	type keyed struct {
		key resultOrderKey
		r   ValidationResult
	}
	ks := make([]keyed, len(results))
	for i := range results {
		r := results[i]
		orderResults(r.Details)
		if len(r.ResultMessages) > 1 {
			// The slice is shared with the shape (and through it the graph
			// index), so it is copied before sorting. A shape's messages come
			// from the graph in no particular order.
			msgs := make([]Term, len(r.ResultMessages))
			copy(msgs, r.ResultMessages)
			sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].TermKey() < msgs[j].TermKey() })
			r.ResultMessages = msgs
		}
		ks[i] = keyed{key: newResultOrderKey(&r), r: r}
	}
	sort.SliceStable(ks, func(i, j int) bool {
		if ks[i].key.stable != ks[j].key.stable {
			return ks[i].key.stable < ks[j].key.stable
		}
		return ks[i].key.labels < ks[j].key.labels
	})
	for i := range ks {
		results[i] = ks[i].r
	}
}

type resultOrderKey struct {
	stable string // blank nodes reduced to their kind
	labels string // blank node labels, the tiebreak
}

func newResultOrderKey(r *ValidationResult) resultOrderKey {
	var stable, labels strings.Builder
	add := func(t Term) {
		if t.IsBlank() {
			stable.WriteString("B:")
			labels.WriteString(t.Value())
		} else {
			stable.WriteString(t.TermKey())
		}
		stable.WriteByte(0)
		labels.WriteByte(0)
	}
	add(r.FocusNode)
	add(r.ResultPath)
	add(r.SourceConstraintComponent)
	add(r.Value)
	add(r.ResultSeverity)
	for _, m := range r.ResultMessages {
		add(m)
	}
	stable.WriteByte(1) // end of the variable-length message list
	add(r.SourceShape)
	add(r.SourceConstraint)
	return resultOrderKey{stable: stable.String(), labels: labels.String()}
}
