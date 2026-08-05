package shacl

import (
	"math"
	"sort"
	"strconv"
)

// SHNEX is the namespace for SHACL Node Expressions.
const SHNEX = "http://www.w3.org/ns/shacl-node-expr#"

// NodeExpr represents a SHACL node expression that can be evaluated
// to produce a list of terms.
type NodeExpr interface {
	Eval(ctx *nodeExprContext) []Term
}

// nodeExprContext holds the evaluation context for node expressions.
type nodeExprContext struct {
	dataGraph *Graph
	shapesMap map[string]*Shape
	focusNode Term
	vars      map[string]Term // additional bound variables

	// The remaining fields are only populated for SHACL-AF expressions, which
	// can call functions and validate against a shape. SHACL 1.2's shnex:
	// expressions leave them nil.
	shapesGraph    *Graph
	classInstances map[string][]Term
	af             *afContext
}

// ---------- ConstantExpr ----------

// ConstantExpr returns the constant term as a single-element list.
type ConstantExpr struct {
	Value Term
}

func (e *ConstantExpr) Eval(ctx *nodeExprContext) []Term {
	return []Term{e.Value}
}

// ---------- ListExpr ----------

// ListExpr returns the list members as-is.
// An empty RDF list ( ) returns [ rdf:nil ].
type ListExpr struct {
	Members []NodeExpr
	IsNil   bool // true when the expression was rdf:nil (empty list)
}

func (e *ListExpr) Eval(ctx *nodeExprContext) []Term {
	if e.IsNil {
		return []Term{IRI(RDFNil)}
	}
	var result []Term
	for _, m := range e.Members {
		result = append(result, m.Eval(ctx)...)
	}
	return result
}

// ---------- EmptyExpr ----------

// EmptyExpr returns an empty list (blank node with no shnex properties).
type EmptyExpr struct{}

func (e *EmptyExpr) Eval(ctx *nodeExprContext) []Term {
	return nil
}

// ---------- VarExpr ----------

// VarExpr looks up a variable by name.
type VarExpr struct {
	Name string
}

func (e *VarExpr) Eval(ctx *nodeExprContext) []Term {
	if e.Name == "focusNode" {
		if ctx.focusNode.IsNone() {
			return nil
		}
		return []Term{ctx.focusNode}
	}
	if v, ok := ctx.vars[e.Name]; ok {
		return []Term{v}
	}
	return nil
}

// ---------- PathValuesExpr ----------

// PathValuesExpr evaluates a property path from a focus node.
type PathValuesExpr struct {
	Path      *PropertyPath
	FocusNode NodeExpr // optional shnex:focusNode override
}

func (e *PathValuesExpr) Eval(ctx *nodeExprContext) []Term {
	focusNodes := []Term{ctx.focusNode}
	if e.FocusNode != nil {
		focusNodes = e.FocusNode.Eval(ctx)
	}
	var result []Term
	for _, fn := range focusNodes {
		result = append(result, evalPath(ctx.dataGraph, e.Path, fn)...)
	}
	return result
}

// ---------- ConcatExpr ----------

// ConcatExpr concatenates the results of multiple expressions.
type ConcatExpr struct {
	Members []NodeExpr
}

func (e *ConcatExpr) Eval(ctx *nodeExprContext) []Term {
	var result []Term
	for _, m := range e.Members {
		result = append(result, m.Eval(ctx)...)
	}
	return result
}

// ---------- DistinctExpr ----------

// DistinctExpr removes duplicates (term equality) preserving order.
type DistinctExpr struct {
	Nodes NodeExpr
}

func (e *DistinctExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	seen := make(map[string]bool, len(nodes))
	var result []Term
	for _, n := range nodes {
		key := n.TermKey()
		if !seen[key] {
			seen[key] = true
			result = append(result, n)
		}
	}
	return result
}

// ---------- CountExpr ----------

// CountExpr returns the count of input nodes as an xsd:integer.
type CountExpr struct {
	Nodes NodeExpr
}

func (e *CountExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	return []Term{Literal(strconv.Itoa(len(nodes)), XSD+"integer", "")}
}

// ---------- SumExpr ----------

// SumExpr returns the sum of numeric input nodes.
type SumExpr struct {
	Nodes   NodeExpr
	FlatMap NodeExpr // optional shnex:flatMap
}

func (e *SumExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.getNodes(ctx)
	if len(nodes) == 0 {
		return []Term{Literal("0", XSD+"integer", "")}
	}
	sum := 0.0
	hasDecimal := false
	for _, n := range nodes {
		v, ok := parseNumeric(n)
		if !ok {
			continue
		}
		sum += v
		dt := n.Datatype()
		if dt == XSD+"decimal" || dt == XSD+"double" || dt == XSD+"float" {
			hasDecimal = true
		}
	}
	if hasDecimal {
		s := strconv.FormatFloat(sum, 'f', -1, 64)
		if !containsDot(s) {
			s += ".0"
		}
		return []Term{Literal(s, XSD+"decimal", "")}
	}
	return []Term{Literal(strconv.Itoa(int(sum)), XSD+"integer", "")}
}

func (e *SumExpr) getNodes(ctx *nodeExprContext) []Term {
	if e.FlatMap != nil {
		return evalFlatMap(ctx, e.Nodes, e.FlatMap)
	}
	return e.Nodes.Eval(ctx)
}

// ---------- MinExpr ----------

// MinExpr returns the minimum numeric value from input nodes.
type MinExpr struct {
	Nodes NodeExpr
}

func (e *MinExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	if len(nodes) == 0 {
		return nil
	}
	minVal := math.MaxFloat64
	var minTerm Term
	found := false
	for _, n := range nodes {
		v, ok := parseNumeric(n)
		if !ok {
			continue
		}
		if !found || v < minVal {
			minVal = v
			minTerm = n
			found = true
		}
	}
	if !found {
		return nil
	}
	return []Term{minTerm}
}

// ---------- MaxExpr ----------

// MaxExpr returns the maximum numeric value from input nodes.
type MaxExpr struct {
	Nodes NodeExpr
}

func (e *MaxExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	if len(nodes) == 0 {
		return nil
	}
	maxVal := -math.MaxFloat64
	var maxTerm Term
	found := false
	for _, n := range nodes {
		v, ok := parseNumeric(n)
		if !ok {
			continue
		}
		if !found || v > maxVal {
			maxVal = v
			maxTerm = n
			found = true
		}
	}
	if !found {
		return nil
	}
	return []Term{maxTerm}
}

// ---------- ExistsExpr ----------

// ExistsExpr returns true/false depending on whether the input is non-empty.
type ExistsExpr struct {
	Nodes NodeExpr
}

func (e *ExistsExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	if len(nodes) > 0 {
		return []Term{Literal("true", XSD+"boolean", "")}
	}
	return []Term{Literal("false", XSD+"boolean", "")}
}

// ---------- FilterShapeExpr ----------

// FilterShapeExpr filters input nodes by conformance to a shape.
type FilterShapeExpr struct {
	ShapeRef Term
	Nodes    NodeExpr
}

func (e *FilterShapeExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	shape := ctx.shapesMap[e.ShapeRef.String()]
	if shape == nil {
		// Parse ad-hoc shape from the graph
		shape = parseAdHocShape(ctx.dataGraph, e.ShapeRef, ctx.shapesMap)
	}
	if shape == nil {
		return nodes // no constraints = all pass
	}
	eCtx := &evalContext{
		dataGraph:      ctx.dataGraph,
		shapesGraph:    ctx.dataGraph,
		shapesMap:      ctx.shapesMap,
		classInstances: buildClassIndex(ctx.dataGraph),
	}
	var result []Term
	for _, n := range nodes {
		if len(validateNodeAgainstShape(eCtx, shape, n)) == 0 {
			result = append(result, n)
		}
	}
	return result
}

// ---------- FindFirstExpr ----------

// FindFirstExpr returns the first input node that conforms to a shape.
type FindFirstExpr struct {
	ShapeRef Term
	Nodes    NodeExpr
}

func (e *FindFirstExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	if len(nodes) == 0 {
		return nil
	}
	shape := ctx.shapesMap[e.ShapeRef.String()]
	if shape == nil {
		shape = parseAdHocShape(ctx.dataGraph, e.ShapeRef, ctx.shapesMap)
	}
	if shape == nil {
		// No constraints = first node passes
		return []Term{nodes[0]}
	}
	eCtx := &evalContext{
		dataGraph:      ctx.dataGraph,
		shapesGraph:    ctx.dataGraph,
		shapesMap:      ctx.shapesMap,
		classInstances: buildClassIndex(ctx.dataGraph),
	}
	for _, n := range nodes {
		if len(validateNodeAgainstShape(eCtx, shape, n)) == 0 {
			return []Term{n}
		}
	}
	return nil
}

// ---------- MatchAllExpr ----------

// MatchAllExpr returns true if all input nodes conform to a shape, false otherwise.
type MatchAllExpr struct {
	ShapeRef Term
	Nodes    NodeExpr
}

func (e *MatchAllExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	shape := ctx.shapesMap[e.ShapeRef.String()]
	if shape == nil {
		shape = parseAdHocShape(ctx.dataGraph, e.ShapeRef, ctx.shapesMap)
	}
	if shape == nil {
		return []Term{Literal("true", XSD+"boolean", "")}
	}
	eCtx := &evalContext{
		dataGraph:      ctx.dataGraph,
		shapesGraph:    ctx.dataGraph,
		shapesMap:      ctx.shapesMap,
		classInstances: buildClassIndex(ctx.dataGraph),
	}
	for _, n := range nodes {
		if len(validateNodeAgainstShape(eCtx, shape, n)) > 0 {
			return []Term{Literal("false", XSD+"boolean", "")}
		}
	}
	return []Term{Literal("true", XSD+"boolean", "")}
}

// ---------- NodesMatchingExpr ----------

// NodesMatchingExpr returns all nodes from the data graph that conform to a shape.
type NodesMatchingExpr struct {
	ShapeRef Term
}

func (e *NodesMatchingExpr) Eval(ctx *nodeExprContext) []Term {
	shape := ctx.shapesMap[e.ShapeRef.String()]
	if shape == nil {
		shape = parseAdHocShape(ctx.dataGraph, e.ShapeRef, ctx.shapesMap)
	}
	if shape == nil {
		return allNodes(ctx.dataGraph)
	}
	eCtx := &evalContext{
		dataGraph:      ctx.dataGraph,
		shapesGraph:    ctx.dataGraph,
		shapesMap:      ctx.shapesMap,
		classInstances: buildClassIndex(ctx.dataGraph),
	}
	candidates := allNodes(ctx.dataGraph)
	var result []Term
	for _, n := range candidates {
		if len(validateNodeAgainstShape(eCtx, shape, n)) == 0 {
			result = append(result, n)
		}
	}
	return result
}

// ---------- InstancesOfExpr ----------

// InstancesOfExpr returns all instances of a class (including subclasses).
type InstancesOfExpr struct {
	Class Term
}

func (e *InstancesOfExpr) Eval(ctx *nodeExprContext) []Term {
	classIdx := buildClassIndex(ctx.dataGraph)
	seen := make(map[string]bool)
	var result []Term
	addInstances := func(cls Term) {
		for _, inst := range classIdx[cls.TermKey()] {
			key := inst.TermKey()
			if !seen[key] {
				seen[key] = true
				result = append(result, inst)
			}
		}
	}
	addInstances(e.Class)
	for _, sub := range subClasses(ctx.dataGraph, e.Class) {
		addInstances(sub)
	}
	return result
}

// ---------- IntersectionExpr ----------

// IntersectionExpr returns nodes that appear in all member expressions (term equality).
type IntersectionExpr struct {
	Members []NodeExpr
}

func (e *IntersectionExpr) Eval(ctx *nodeExprContext) []Term {
	if len(e.Members) == 0 {
		return nil
	}
	// Start with first set
	first := e.Members[0].Eval(ctx)
	if len(first) == 0 {
		return nil
	}
	// Count occurrences in each set
	current := make(map[string]bool, len(first))
	for _, t := range first {
		current[t.TermKey()] = true
	}
	for i := 1; i < len(e.Members); i++ {
		next := make(map[string]bool)
		for _, t := range e.Members[i].Eval(ctx) {
			key := t.TermKey()
			if current[key] {
				next[key] = true
			}
		}
		current = next
	}
	// Preserve order from first set, deduplicate
	seen := make(map[string]bool, len(current))
	var result []Term
	for _, t := range first {
		key := t.TermKey()
		if current[key] && !seen[key] {
			seen[key] = true
			result = append(result, t)
		}
	}
	return result
}

// ---------- FlatMapExpr ----------

// FlatMapExpr evaluates an expression for each input node and concatenates results.
type FlatMapExpr struct {
	Nodes   NodeExpr
	MapExpr NodeExpr
}

func (e *FlatMapExpr) Eval(ctx *nodeExprContext) []Term {
	return evalFlatMap(ctx, e.Nodes, e.MapExpr)
}

func evalFlatMap(ctx *nodeExprContext, nodesExpr, mapExpr NodeExpr) []Term {
	nodes := nodesExpr.Eval(ctx)
	var result []Term
	for _, n := range nodes {
		subCtx := &nodeExprContext{
			dataGraph: ctx.dataGraph,
			shapesMap: ctx.shapesMap,
			focusNode: n,
			vars:      ctx.vars,
		}
		result = append(result, mapExpr.Eval(subCtx)...)
	}
	return result
}

// ---------- RemoveExpr ----------

// RemoveExpr removes terms from the input that match terms in the remove set (term equality).
type RemoveExpr struct {
	Nodes    NodeExpr
	ToRemove NodeExpr
}

func (e *RemoveExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	remove := e.ToRemove.Eval(ctx)
	removeSet := make(map[string]bool, len(remove))
	for _, r := range remove {
		removeSet[r.TermKey()] = true
	}
	var result []Term
	for _, n := range nodes {
		if !removeSet[n.TermKey()] {
			result = append(result, n)
		}
	}
	return result
}

// ---------- IfExpr ----------

// IfExpr evaluates condition; if truthy returns then-branch, else returns else-branch.
type IfExpr struct {
	Condition NodeExpr
	Then      NodeExpr
	Else      NodeExpr // may be nil
}

func (e *IfExpr) Eval(ctx *nodeExprContext) []Term {
	condResult := e.Condition.Eval(ctx)
	if isTruthy(condResult) {
		if e.Then != nil {
			return e.Then.Eval(ctx)
		}
		return nil
	}
	if e.Else != nil {
		return e.Else.Eval(ctx)
	}
	return nil
}

// ---------- LimitExpr ----------

// LimitExpr takes only the first N results.
type LimitExpr struct {
	Nodes NodeExpr
	Limit int
}

func (e *LimitExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	if e.Limit >= len(nodes) {
		return nodes
	}
	return nodes[:e.Limit]
}

// ---------- OffsetExpr ----------

// OffsetExpr skips the first N results.
type OffsetExpr struct {
	Nodes  NodeExpr
	Offset int
}

func (e *OffsetExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	if e.Offset >= len(nodes) {
		return nil
	}
	return nodes[e.Offset:]
}

// ---------- OrderByExpr ----------

// OrderByExpr sorts input nodes by a key expression.
type OrderByExpr struct {
	Nodes   NodeExpr
	KeyExpr NodeExpr
	Desc    bool
}

func (e *OrderByExpr) Eval(ctx *nodeExprContext) []Term {
	nodes := e.Nodes.Eval(ctx)
	if len(nodes) <= 1 {
		return nodes
	}
	// Compute sort keys for each node
	type entry struct {
		node Term
		key  []Term
	}
	entries := make([]entry, len(nodes))
	for i, n := range nodes {
		subCtx := &nodeExprContext{
			dataGraph: ctx.dataGraph,
			shapesMap: ctx.shapesMap,
			focusNode: n,
			vars:      ctx.vars,
		}
		entries[i] = entry{node: n, key: e.KeyExpr.Eval(subCtx)}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		ki := entries[i].key
		kj := entries[j].key
		cmp := compareTermLists(ki, kj)
		if e.Desc {
			return cmp > 0
		}
		return cmp < 0
	})
	result := make([]Term, len(entries))
	for i, ent := range entries {
		result[i] = ent.node
	}
	return result
}

// ---------- Helper functions ----------

func parseNumeric(t Term) (float64, bool) {
	if !t.IsLiteral() {
		return 0, false
	}
	v, err := strconv.ParseFloat(t.Value(), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func containsDot(s string) bool {
	for _, c := range s {
		if c == '.' {
			return true
		}
	}
	return false
}

// isTruthy checks if an expression result is truthy.
// A single xsd:boolean "true" literal is truthy; a non-empty list of non-false values is truthy.
func isTruthy(terms []Term) bool {
	if len(terms) == 0 {
		return false
	}
	for _, t := range terms {
		if t.IsLiteral() && t.Value() == "false" && (t.Datatype() == XSD+"boolean" || t.Datatype() == "") {
			return false
		}
	}
	return true
}

// compareTermLists compares two term lists for ordering.
// Returns -1, 0, or 1. Empty/nil lists sort before non-empty.
func compareTermLists(a, b []Term) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return -1
	}
	if len(b) == 0 {
		return 1
	}
	return compareTerm(a[0], b[0])
}

// compareTerm compares two terms for ordering.
func compareTerm(a, b Term) int {
	// Try numeric comparison first
	va, okA := parseNumeric(a)
	vb, okB := parseNumeric(b)
	if okA && okB {
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
		return 0
	}
	// Fall back to string comparison
	sa := a.Value()
	sb := b.Value()
	if sa < sb {
		return -1
	}
	if sa > sb {
		return 1
	}
	return 0
}

// parseAdHocShape parses constraints from an RDF node that may define
// inline shape constraints (like sh:minInclusive, sh:datatype, etc.)
// without being declared as a shape. Used by filterShape, findFirst, matchAll.
func parseAdHocShape(g *Graph, node Term, shapesMap map[string]*Shape) *Shape {
	if s, ok := shapesMap[node.String()]; ok {
		return s
	}
	s := &Shape{ID: node}
	s.Constraints = parseConstraints(g, s, shapesMap)
	s.Properties = parsePropertyShapes(g, s.ID, shapesMap)
	if len(s.Constraints) == 0 && len(s.Properties) == 0 {
		return nil
	}
	shapesMap[node.String()] = s
	return s
}

// parsePropertyShapes parses sh:property shapes for ad-hoc shapes.
func parsePropertyShapes(g *Graph, shapeID Term, shapesMap map[string]*Shape) []*Shape {
	propPred := IRI(SH + "property")
	props := g.Objects(shapeID, propPred)
	var result []*Shape
	for _, p := range props {
		ps := &Shape{
			ID:         p,
			IsProperty: true,
		}
		pathVals := g.Objects(p, IRI(SH+"path"))
		if len(pathVals) > 0 {
			ps.Path = parsePath(g, pathVals[0])
		}
		ps.Constraints = parseConstraints(g, ps, shapesMap)
		if len(ps.Constraints) > 0 || ps.Path != nil {
			result = append(result, ps)
		}
	}
	return result
}
