package shacl

import (
	"strings"

	"github.com/tggo/goRDFlib/term"
)

// ---------- SingleLineConstraint (sh:singleLine) ----------

// SingleLineConstraint implements sh:singleLine.
// When SingleLine is true, each string literal value node must not contain
// line-breaking characters: \n (0x0A), \r (0x0D), \f (0x0C), or \v (0x0B).
type SingleLineConstraint struct {
	SingleLine bool
}

func (c *SingleLineConstraint) ComponentIRI() string {
	return SH + "SingleLineConstraintComponent"
}

func (c *SingleLineConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if !c.SingleLine {
		return nil
	}
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !vn.IsLiteral() {
			continue
		}
		v := vn.Value()
		if strings.ContainsAny(v, "\n\r\f\v") {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// ---------- SomeValueConstraint (sh:someValue) ----------

// SomeValueConstraint implements sh:someValue.
// At least one value node must conform to the referenced shape.
// If no value conforms, a single result is produced with no specific value.
type SomeValueConstraint struct {
	ShapeRef Term
}

func (c *SomeValueConstraint) ComponentIRI() string {
	return SH + "SomeValueConstraintComponent"
}

func (c *SomeValueConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	s, ok := ctx.shapesMap[c.ShapeRef.String()]
	if !ok {
		return nil
	}
	for _, vn := range valueNodes {
		if len(validateNodeAgainstShape(ctx, s, vn)) == 0 {
			return nil // at least one value conforms
		}
	}
	return []ValidationResult{makeResult(shape, focusNode, Term{}, c.ComponentIRI())}
}

// ---------- SubsetOfConstraint (sh:subsetOf) ----------

// SubsetOfConstraint implements sh:subsetOf.
// All value nodes must be present among the values obtained by evaluating
// OtherPath from the focus node.
type SubsetOfConstraint struct {
	OtherPath *PropertyPath
}

func (c *SubsetOfConstraint) ComponentIRI() string {
	return SH + "SubsetOfConstraintComponent"
}

func (c *SubsetOfConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	otherValues := evalPath(ctx.dataGraph, c.OtherPath, focusNode)
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !containsTerm(otherValues, vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// ---------- UniqueMembersConstraint (sh:uniqueMembers) ----------

// UniqueMembersConstraint implements sh:uniqueMembers.
// For each value node treated as an RDF list head, checks that all list
// members are unique. Reports violations for malformed lists (missing
// rdf:first) and for each duplicate member.
type UniqueMembersConstraint struct {
	UniqueMembers bool
}

func (c *UniqueMembersConstraint) ComponentIRI() string {
	return SH + "UniqueMembersConstraintComponent"
}

func (c *UniqueMembersConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if !c.UniqueMembers {
		return nil
	}
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !isWellFormedList(ctx.dataGraph, vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
			continue
		}
		members := ctx.dataGraph.RDFList(vn)
		seen := make(map[string]bool, len(members))
		var dups []ValidationResult
		for _, m := range members {
			key := m.TermKey()
			if seen[key] {
				dups = append(dups, makeResult(shape, focusNode, m, c.ComponentIRI()))
			}
			seen[key] = true
		}
		if len(dups) > 0 {
			r := makeResult(shape, focusNode, vn, c.ComponentIRI())
			r.Details = dups
			results = append(results, r)
		}
	}
	return results
}

// ---------- MemberShapeConstraint (sh:memberShape) ----------

// MemberShapeConstraint implements sh:memberShape.
// For each value node (an RDF list head), validates every list member against
// the referenced shape. If any member violates, a result is produced with the
// list head as value and the member violations as Details.
type MemberShapeConstraint struct {
	ShapeRef Term
}

func (c *MemberShapeConstraint) ComponentIRI() string {
	return SH + "MemberShapeConstraintComponent"
}

func (c *MemberShapeConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	s, ok := ctx.shapesMap[c.ShapeRef.String()]
	if !ok {
		return nil
	}
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !isWellFormedList(ctx.dataGraph, vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
			continue
		}
		members := ctx.dataGraph.RDFList(vn)
		var memberViolations []ValidationResult
		for _, m := range members {
			vr := validateNodeAgainstShape(ctx, s, m)
			memberViolations = append(memberViolations, vr...)
		}
		if len(memberViolations) > 0 {
			r := makeResult(shape, focusNode, vn, c.ComponentIRI())
			r.Details = memberViolations
			results = append(results, r)
		}
	}
	return results
}

// ---------- MinListLengthConstraint (sh:minListLength) ----------

// MinListLengthConstraint implements sh:minListLength.
// For each value node, counts the RDF list length. If the list is malformed
// or shorter than MinLength, a violation is reported.
type MinListLengthConstraint struct {
	MinLength int
}

func (c *MinListLengthConstraint) ComponentIRI() string {
	return SH + "MinListLengthConstraintComponent"
}

func (c *MinListLengthConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !isWellFormedList(ctx.dataGraph, vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
			continue
		}
		length := listLength(ctx.dataGraph, vn)
		if length < c.MinLength {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// ---------- MaxListLengthConstraint (sh:maxListLength) ----------

// MaxListLengthConstraint implements sh:maxListLength.
// For each value node, counts the RDF list length. If the list is malformed
// or longer than MaxLength, a violation is reported.
type MaxListLengthConstraint struct {
	MaxLength int
}

func (c *MaxListLengthConstraint) ComponentIRI() string {
	return SH + "MaxListLengthConstraintComponent"
}

func (c *MaxListLengthConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !isWellFormedList(ctx.dataGraph, vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
			continue
		}
		length := listLength(ctx.dataGraph, vn)
		if length > c.MaxLength {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// ---------- ReifierShapeConstraint (sh:reifierShape) ----------

// ReifierShapeConstraint implements sh:reifierShape.
// For each value node of the property, finds all reifiers of the triple
// (focusNode, pathPredicate, value) and validates each reifier against the
// referenced shape. A reifier is a node ?r where
// ?r rdf:reifies <<(focusNode pathPred value)>> exists in the data graph.
type ReifierShapeConstraint struct {
	ShapeRef            Term
	ReificationRequired bool
}

func (c *ReifierShapeConstraint) ComponentIRI() string {
	return SH + "ReifierShapeConstraintComponent"
}

func (c *ReifierShapeConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	s, ok := ctx.shapesMap[c.ShapeRef.String()]
	if !ok {
		return nil
	}

	// Determine the path predicate IRI from the shape's path.
	pathPred := Term{}
	if shape.Path != nil && shape.Path.Kind == PathPredicate {
		pathPred = shape.Path.Pred
	}
	if pathPred.IsNone() {
		return nil
	}

	var results []ValidationResult
	for _, vn := range valueNodes {
		reifiers := findReifiers(ctx.dataGraph, focusNode, pathPred, vn)
		if len(reifiers) == 0 && c.ReificationRequired {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
			continue
		}
		for _, reifier := range reifiers {
			vr := validateNodeAgainstShape(ctx, s, reifier)
			if len(vr) > 0 {
				r := makeResult(shape, focusNode, vn, c.ComponentIRI())
				r.Details = vr
				results = append(results, r)
			}
		}
	}
	return results
}

// ---------- List-valued constraint variants (SHACL 1.2) ----------

// DatatypeListConstraint implements sh:datatype when the value is a list of
// datatypes (OR semantics).
type DatatypeListConstraint struct {
	Datatypes []Term
}

func (c *DatatypeListConstraint) ComponentIRI() string {
	return SH + "DatatypeConstraintComponent"
}

func (c *DatatypeListConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		if !vn.IsLiteral() {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
			continue
		}
		matched := false
		for _, dt := range c.Datatypes {
			if vn.Datatype() == dt.Value() && isWellFormedLiteral(vn) {
				matched = true
				break
			}
		}
		if !matched {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// NodeKindListConstraint implements sh:nodeKind when the value is a list of nodeKinds (OR semantics).
type NodeKindListConstraint struct {
	NodeKinds []Term
}

func (c *NodeKindListConstraint) ComponentIRI() string {
	return SH + "NodeKindConstraintComponent"
}

func (c *NodeKindListConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		matched := false
		for _, nk := range c.NodeKinds {
			if matchesNodeKind(vn, nk.Value()) {
				matched = true
				break
			}
		}
		if !matched {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// ClassListConstraint implements sh:class when the value is a list of classes (OR semantics).
type ClassListConstraint struct {
	Classes []Term
}

func (c *ClassListConstraint) ComponentIRI() string {
	return SH + "ClassConstraintComponent"
}

func (c *ClassListConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		matched := false
		for _, cls := range c.Classes {
			if ctx.dataGraph.HasType(vn, cls) {
				matched = true
				break
			}
		}
		if !matched {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// ---------- Helper functions ----------

// isWellFormedList checks whether a node is a well-formed RDF list.
// A well-formed list is rdf:nil, or every cell has exactly rdf:first and rdf:rest,
// the chain terminates at rdf:nil, and contains no cycles.
func isWellFormedList(g *Graph, node Term) bool {
	nilTerm := IRI(RDFNil)
	if node.Equal(nilTerm) {
		return true
	}
	firstPred := IRI(RDFFirst)
	restPred := IRI(RDFRest)
	visited := make(map[string]bool)
	current := node
	for {
		if current.Equal(nilTerm) {
			return true
		}
		key := current.TermKey()
		if visited[key] {
			return false // cycle
		}
		visited[key] = true
		if !g.Has(&current, &firstPred, nil) {
			return false
		}
		rests := g.Objects(current, restPred)
		if len(rests) == 0 {
			return false // no rdf:rest → malformed
		}
		current = rests[0]
	}
}

// listLength counts the number of elements in an RDF list starting at head.
// Assumes isWellFormedList has already been checked.
func listLength(g *Graph, head Term) int {
	return len(g.RDFList(head))
}

// findReifiers finds all nodes that reify the triple (subject, predicate, object)
// via rdf:reifies with a TripleTerm object. It searches the underlying rdflibgo
// graph for triples of the form (?r, rdf:reifies, <<(subject predicate object)>>).
func findReifiers(g *Graph, subject, predicate, object Term) []Term {
	reifiesPred := term.NewURIRefUnsafe(RDF + "reifies")
	subj := toTerm(subject)
	pred := toTerm(predicate)
	obj := toTerm(object)

	if subj == nil || pred == nil || obj == nil {
		return nil
	}

	// Build the expected triple term to match against.
	predURI, ok := pred.(term.URIRef)
	if !ok {
		return nil
	}
	subjAsSubject, ok := subj.(term.Subject)
	if !ok {
		return nil
	}
	expectedTT := term.NewTripleTerm(subjAsSubject, predURI, obj)

	var reifiers []Term
	g.rdf().Triples(nil, &reifiesPred, nil)(func(t term.Triple) bool {
		if tt, ok := t.Object.(term.TripleTerm); ok {
			if tt.Equal(expectedTT) {
				reifiers = append(reifiers, fromRDFLib(t.Subject))
			}
		}
		return true
	})
	return reifiers
}
