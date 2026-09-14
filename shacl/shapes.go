package shacl

import (
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// Shape represents either a NodeShape or a PropertyShape.
type Shape struct {
	ID          Term
	IsProperty  bool
	Path        *PropertyPath
	Targets     []Target
	Constraints []Constraint
	Severity    Term
	Deactivated bool
	Messages    []Term

	Properties []*Shape // nested sh:property shapes

	Closed            bool
	ClosedByTypes     bool
	IgnoredProperties []Term

	// SHACL 1.2: sh:values — SPARQL-computed value nodes override path evaluation.
	Values *SPARQLValues
}

// SPARQLValues represents sh:values with sh:select or sh:sparqlExpr.
type SPARQLValues struct {
	Select   string // SPARQL SELECT query (with prefixes prepended)
	Expr     string // SPARQL expression (wrapped into SELECT)
	Prefixes string // resolved PREFIX declarations
}

// Target represents a target declaration on a shape.
type Target struct {
	Kind   TargetKind
	Value  Term
	Select string // SPARQL query for TargetSPARQL
}

// TargetKind distinguishes the different SHACL target types.
type TargetKind int

const (
	// TargetNode represents sh:targetNode.
	TargetNode TargetKind = iota
	// TargetClass represents sh:targetClass.
	TargetClass
	// TargetSubjectsOf represents sh:targetSubjectsOf.
	TargetSubjectsOf
	// TargetObjectsOf represents sh:targetObjectsOf.
	TargetObjectsOf
	// TargetImplicitClass represents an implicit class target (shape is also an rdfs:Class or sh:ShapeClass).
	TargetImplicitClass
	// TargetWhere represents sh:targetWhere (SHACL 1.2).
	TargetWhere
	// TargetSPARQL represents a SPARQL-based target (sh:targetNode/sh:targetClass with sh:select).
	TargetSPARQL
)

// String returns a human-readable name for the target kind.
func (k TargetKind) String() string {
	switch k {
	case TargetNode:
		return "targetNode"
	case TargetClass:
		return "targetClass"
	case TargetSubjectsOf:
		return "targetSubjectsOf"
	case TargetObjectsOf:
		return "targetObjectsOf"
	case TargetImplicitClass:
		return "implicitClassTarget"
	case TargetWhere:
		return "targetWhere"
	}
	return "unknown"
}

// Constraint is a single constraint component to evaluate.
type Constraint interface {
	Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult
	ComponentIRI() string
}

// evalContext provides access to graphs and shape lookup during evaluation.
type evalContext struct {
	dataGraph      *Graph
	shapesGraph    *Graph
	shapesMap      map[string]*Shape
	classInstances map[string][]Term // class TermKey → instances with that rdf:type

	// cfg carries the caller's options, and with them the error handler. It is
	// how evaluation paths that cannot return an error — target resolution —
	// still make a failure visible. Nil in the internal contexts built to
	// evaluate a shape on its own terms, where no target is resolved.
	cfg *config

	// af is set only when SHACL-AF is enabled. It carries the functions the
	// shapes graph declares, which have to reach every SPARQL query run on
	// its behalf. Nil means AF is off and no AF vocabulary is interpreted.
	af *afContext

	// guard tracks the (shape, focus node) pairs being validated on the current
	// path, so a recursive shape over cyclic data terminates. Every context
	// derived from this one — including the node expression contexts in
	// between — must share it, or a cycle that passes through a context
	// boundary starts with an empty set each time round and never ends.
	guard *recursionGuard
}

// recursionGuard records which (shape, focus node) pairs are in progress.
//
// SHACL 1.0 §3.4.3 leaves the validation of recursive shapes undefined and up
// to the processor. This one takes the coinductive reading: a pair that is
// already being validated further up the current path is assumed to conform
// when it is reached again. A cycle therefore contributes no results of its
// own, while anything that fails anywhere along it is still reported by the
// visit that reaches it first. The set is per path, not a memo: a pair is
// removed once its validation returns, so the same pair reached again on a
// different branch is validated in full.
//
// The pair is keyed by shape ID, not *Shape, because SHACL-AF re-parses the
// shapes graph per rule round and parses anonymous shapes on demand, so one
// shape can have several *Shape values.
//
// Not safe for concurrent use; a validation run is single-goroutine.
type recursionGuard struct {
	active map[guardKey]struct{}
}

// guardKey identifies a (shape, focus node) pair. Term is comparable, so the
// pair is a map key as it is; building a string key from TermKey allocated on
// every shape entered and doubled the allocations of a validation run.
type guardKey struct {
	shape, node Term
}

// sharedGuard returns ctx's guard, creating it on first use, so that a context
// derived from ctx can be given the same one.
func (ctx *evalContext) sharedGuard() *recursionGuard {
	if ctx.guard == nil {
		ctx.guard = &recursionGuard{active: make(map[guardKey]struct{})}
	}
	return ctx.guard
}

// enter marks (s, node) as in progress. It returns false when the pair already
// is, in which case the caller must treat it as conforming and must not call
// leave.
func (ctx *evalContext) enter(s *Shape, node Term) bool {
	g := ctx.sharedGuard()
	k := guardKey{s.ID, node}
	if _, busy := g.active[k]; busy {
		return false
	}
	g.active[k] = struct{}{}
	return true
}

func (ctx *evalContext) leave(s *Shape, node Term) {
	delete(ctx.guard.active, guardKey{s.ID, node})
}

// report hands err to the caller's error handler, if one was installed.
func (ctx *evalContext) report(err error) {
	if err == nil || ctx == nil {
		return
	}
	switch {
	case ctx.cfg != nil:
		ctx.cfg.report(err)
	case ctx.af != nil:
		ctx.af.cfg.report(err)
	}
}

// sparqlFuncs returns the SHACL functions to bind to a query run for this
// validation, or nil when SHACL-AF is not enabled.
func (ctx *evalContext) sparqlFuncs() map[string]sparql.Function {
	if ctx == nil || ctx.af == nil {
		return nil
	}
	return ctx.af.functionsAtDepth(1)
}

// parseShapes extracts all NodeShapes and PropertyShapes from the shapes graph.
func parseShapes(g *Graph) map[string]*Shape {
	shapes := make(map[string]*Shape)

	typePred := IRI(RDFType)
	nodeShapeType := IRI(SH + "NodeShape")
	propShapeType := IRI(SH + "PropertyShape")

	for _, t := range g.All(nil, &typePred, &nodeShapeType) {
		s := getOrCreate(shapes, t.Subject)
		s.IsProperty = false
	}
	for _, t := range g.All(nil, &typePred, &propShapeType) {
		s := getOrCreate(shapes, t.Subject)
		s.IsProperty = true
	}
	// Discover sh:ShapeClass shapes (implicit class targets, SHACL 1.2)
	shapeClassType := IRI(SH + "ShapeClass")
	for _, t := range g.All(nil, &typePred, &shapeClassType) {
		s := getOrCreate(shapes, t.Subject)
		s.IsProperty = false
	}

	propPred := IRI(SH + "property")
	for _, t := range g.All(nil, &propPred, nil) {
		ps := getOrCreate(shapes, t.Object)
		ps.IsProperty = true
	}

	pathPred := IRI(SH + "path")
	for _, t := range g.All(nil, &pathPred, nil) {
		ps := getOrCreate(shapes, t.Subject)
		ps.IsProperty = true
	}

	for _, pred := range []string{"targetNode", "targetClass", "targetSubjectsOf", "targetObjectsOf"} {
		p := IRI(SH + pred)
		for _, t := range g.All(nil, &p, nil) {
			getOrCreate(shapes, t.Subject)
		}
	}

	// Discover shapes referenced in logical/shape constraints (may be blank nodes)
	for _, pred := range []string{"and", "or", "xone"} {
		p := IRI(SH + pred)
		for _, t := range g.All(nil, &p, nil) {
			items := g.RDFList(t.Object)
			for _, item := range items {
				getOrCreate(shapes, item)
			}
		}
	}
	for _, pred := range []string{"not", "node", "qualifiedValueShape", "someValue", "memberShape", "reifierShape", "nodeByExpression"} {
		p := IRI(SH + pred)
		for _, t := range g.All(nil, &p, nil) {
			getOrCreate(shapes, t.Object)
		}
	}

	// Discover sh:targetWhere shapes
	twPred := IRI(SH + "targetWhere")
	for _, t := range g.All(nil, &twPred, nil) {
		getOrCreate(shapes, t.Subject)
		// The targetWhere value is a shape-like node — discover it for its constraints
		twNode := t.Object
		twShape := getOrCreate(shapes, twNode)
		twShape.IsProperty = false
	}

	// Pass 1: parse basic shape info (paths, targets, properties) — iterate until stable
	parsed := make(map[string]bool)
	for {
		found := false
		for key, s := range shapes {
			if !parsed[key] {
				parseShapeBasic(g, s, shapes)
				parsed[key] = true
				found = true
			}
		}
		if !found {
			break
		}
	}

	// Parse custom constraint components
	components := parseConstraintComponents(g)

	// Pass 2: parse constraints (now all shapes have their paths resolved)
	for _, s := range shapes {
		s.Constraints = parseConstraints(g, s, shapes)
		// Add custom component constraints
		if len(components) > 0 {
			s.Constraints = append(s.Constraints, buildComponentConstraints(g, s, components)...)
		}
		// SHACL 1.2: Apply annotation-based overrides (sh:severity, sh:deactivated on individual constraints)
		applyConstraintAnnotations(g, s)
		// SHACL 1.2: Apply annotation-based overrides on sh:property references
		applyPropertyAnnotations(g, s)
	}

	return shapes
}

func getOrCreate(shapes map[string]*Shape, id Term) *Shape {
	key := id.String()
	if s, ok := shapes[key]; ok {
		return s
	}
	s := &Shape{ID: id, Severity: SHViolation}
	shapes[key] = s
	return s
}

func parseShapeBasic(g *Graph, s *Shape, shapes map[string]*Shape) {
	id := s.ID

	selectPred := IRI(SH + "select")
	for _, tn := range g.Objects(id, IRI(SH+"targetNode")) {
		// SHACL 1.2: SPARQL-based target (blank node with sh:select)
		if sels := g.Objects(tn, selectPred); len(sels) > 0 {
			prefixes := resolvePrefixes(g, firstOrNone(g.Objects(tn, IRI(SH+"prefixes"))))
			s.Targets = append(s.Targets, Target{Kind: TargetSPARQL, Select: prefixes + sels[0].Value()})
		} else {
			s.Targets = append(s.Targets, Target{Kind: TargetNode, Value: tn})
		}
	}
	for _, tc := range g.Objects(id, IRI(SH+"targetClass")) {
		s.Targets = append(s.Targets, Target{Kind: TargetClass, Value: tc})
	}
	for _, ts := range g.Objects(id, IRI(SH+"targetSubjectsOf")) {
		s.Targets = append(s.Targets, Target{Kind: TargetSubjectsOf, Value: ts})
	}
	for _, to := range g.Objects(id, IRI(SH+"targetObjectsOf")) {
		s.Targets = append(s.Targets, Target{Kind: TargetObjectsOf, Value: to})
	}

	// Implicit class targets: if shape is also an rdfs:Class or sh:ShapeClass
	typePred := IRI(RDFType)
	rdfsClass := IRI(RDFSClass)
	shShapeClass := IRI(SH + "ShapeClass")
	if g.Has(&id, &typePred, &rdfsClass) || g.HasType(id, rdfsClass) ||
		g.Has(&id, &typePred, &shShapeClass) {
		if !s.IsProperty {
			s.Targets = append(s.Targets, Target{Kind: TargetImplicitClass, Value: id})
		}
	}

	// sh:targetWhere (SHACL 1.2)
	for _, tw := range g.Objects(id, IRI(SH+"targetWhere")) {
		s.Targets = append(s.Targets, Target{Kind: TargetWhere, Value: tw})
	}

	if paths := g.Objects(id, IRI(SH+"path")); len(paths) > 0 {
		s.Path = parsePath(g, paths[0])
		s.IsProperty = true
	}

	// SHACL 1.2: sh:values — SPARQL-computed value nodes
	if valNodes := g.Objects(id, IRI(SH+"values")); len(valNodes) > 0 {
		vn := valNodes[0]
		prefixes := resolvePrefixes(g, firstOrNone(g.Objects(vn, IRI(SH+"prefixes"))))
		if sels := g.Objects(vn, selectPred); len(sels) > 0 {
			s.Values = &SPARQLValues{Select: sels[0].Value(), Prefixes: prefixes}
		} else if exprs := g.Objects(vn, IRI(SH+"sparqlExpr")); len(exprs) > 0 {
			s.Values = &SPARQLValues{Expr: exprs[0].Value(), Prefixes: prefixes}
		}
	}

	if sevs := g.Objects(id, IRI(SH+"severity")); len(sevs) > 0 {
		s.Severity = sevs[0]
	}

	if deacts := g.Objects(id, IRI(SH+"deactivated")); len(deacts) > 0 {
		s.Deactivated = deacts[0].Value() == "true"
	}

	s.Messages = g.Objects(id, IRI(SH+"message"))

	propPred := IRI(SH + "property")
	for _, pn := range g.Objects(id, propPred) {
		ps := getOrCreate(shapes, pn)
		ps.IsProperty = true
		s.Properties = append(s.Properties, ps)
	}

	if vals := g.Objects(id, IRI(SH+"closed")); len(vals) > 0 {
		v := vals[0]
		if v.Value() == "true" {
			s.Closed = true
		} else if v.IsIRI() && v.Value() == SH+"ByTypes" {
			s.ClosedByTypes = true
		}
	}
	for _, ip := range g.Objects(id, IRI(SH+"ignoredProperties")) {
		s.IgnoredProperties = g.RDFList(ip)
	}
}

// applyConstraintAnnotations checks for RDF-star annotations on constraint triples
// that override sh:severity or sh:deactivated for individual constraints.
// E.g., sh:datatype xsd:integer {| sh:severity sh:Warning |} overrides the result
// severity for that specific constraint.
func applyConstraintAnnotations(g *Graph, s *Shape) {
	annotations := findAnnotations(g, s.ID)
	if len(annotations) == 0 {
		return
	}

	for i, c := range s.Constraints {
		ann, ok := matchConstraintAnnotation(c, annotations)
		if !ok {
			continue
		}
		if deact, exists := ann[SH+"deactivated"]; exists && deact.Value() == "true" {
			// Replace with a no-op constraint
			s.Constraints[i] = &deactivatedConstraint{}
			continue
		}
		if sev, exists := ann[SH+"severity"]; exists {
			// Wrap with severity override
			s.Constraints[i] = &severityOverrideConstraint{inner: c, severity: sev}
		}
	}
}

// applyPropertyAnnotations checks for annotations on sh:property references.
// E.g., sh:property ex:Shape2 {| sh:deactivated true |} means that property shape
// should be skipped during validation.
func applyPropertyAnnotations(g *Graph, s *Shape) {
	annotations := findAnnotations(g, s.ID)
	if len(annotations) == 0 {
		return
	}

	propPred := IRI(SH + "property")
	for _, ps := range s.Properties {
		// Check if there's an annotation on (shapeID, sh:property, propShapeID)
		for _, ann := range annotations {
			if ann.predicate.Equal(propPred) && ann.object.Equal(ps.ID) {
				if deact, exists := ann.props[SH+"deactivated"]; exists && deact.Value() == "true" {
					ps.Deactivated = true
				}
				if sev, exists := ann.props[SH+"severity"]; exists {
					ps.Severity = sev
				}
			}
		}
	}
}

type tripleAnnotation struct {
	predicate Term
	object    Term
	props     map[string]Term // annotation property IRI → value
}

// findAnnotations discovers RDF-star annotations on triples with the given subject.
// An annotation is: _:r rdf:reifies <<(subject pred obj)>> ; sh:severity ?val ; sh:deactivated ?val .
func findAnnotations(g *Graph, subject Term) []tripleAnnotation {
	reifiesPred := IRI(RDF + "reifies")
	sevPred := IRI(SH + "severity")
	deactPred := IRI(SH + "deactivated")

	var annotations []tripleAnnotation

	// Find all reifiers in the graph that reference triples with our subject
	for _, t := range g.All(nil, &reifiesPred, nil) {
		reifier := t.Subject
		// The object should be a triple term — but in our shacl Term model,
		// triple terms are not directly representable. We need to use the
		// underlying graph to check.
		// Since our Graph wrapper converts TripleTerms to regular terms
		// via fromRDFLib, we can't directly inspect them through the wrapper.
		// Instead, use the annotation properties on the reifier to identify them.
		// The reifier node will have sh:severity or sh:deactivated properties.
		props := make(map[string]Term)
		if sevs := g.Objects(reifier, sevPred); len(sevs) > 0 {
			props[SH+"severity"] = sevs[0]
		}
		if deacts := g.Objects(reifier, deactPred); len(deacts) > 0 {
			props[SH+"deactivated"] = deacts[0]
		}
		if len(props) == 0 {
			continue
		}

		// Now we need to figure out what triple this reifier annotates.
		// The object of rdf:reifies is a TripleTerm. We need to extract its
		// subject/predicate/object. Use the underlying graph.
		ann := extractAnnotatedTriple(g, reifier, subject)
		if ann != nil {
			ann.props = props
			annotations = append(annotations, *ann)
		}
	}
	return annotations
}

// extractAnnotatedTriple extracts the predicate and object from a reifier that
// annotates a triple with the given subject. Returns nil if the reifier doesn't
// annotate a triple with this subject.
func extractAnnotatedTriple(g *Graph, reifier, expectedSubject Term) *tripleAnnotation {
	reifiesPredURI := toURIRef(IRI(RDF + "reifies"))
	reifierSubj := toSubject(reifier)
	if reifierSubj == nil {
		return nil
	}

	var result *tripleAnnotation
	g.g.Triples(reifierSubj, &reifiesPredURI, nil)(func(t term.Triple) bool {
		tt, ok := t.Object.(term.TripleTerm)
		if !ok {
			return true
		}
		subj := fromRDFLib(tt.Subject())
		if !subj.Equal(expectedSubject) {
			return true
		}
		result = &tripleAnnotation{
			predicate: fromRDFLib(tt.Predicate()),
			object:    fromRDFLib(tt.Object()),
		}
		return false // stop after first match
	})
	return result
}

// matchConstraintAnnotation tries to match a constraint to its annotation.
func matchConstraintAnnotation(c Constraint, annotations []tripleAnnotation) (map[string]Term, bool) {
	iri := c.ComponentIRI()
	for _, ann := range annotations {
		// Match based on the SHACL constraint predicate
		predIRI := ann.predicate.Value()
		switch {
		case iri == SH+"DatatypeConstraintComponent" && predIRI == SH+"datatype":
			return ann.props, true
		case iri == SH+"ClassConstraintComponent" && predIRI == SH+"class":
			return ann.props, true
		case iri == SH+"NodeKindConstraintComponent" && predIRI == SH+"nodeKind":
			return ann.props, true
		case iri == SH+"MinCountConstraintComponent" && predIRI == SH+"minCount":
			return ann.props, true
		case iri == SH+"MaxCountConstraintComponent" && predIRI == SH+"maxCount":
			return ann.props, true
		case iri == SH+"PatternConstraintComponent" && predIRI == SH+"pattern":
			return ann.props, true
		case iri == SH+"HasValueConstraintComponent" && predIRI == SH+"hasValue":
			return ann.props, true
		case iri == SH+"InConstraintComponent" && predIRI == SH+"in":
			return ann.props, true
		}
	}
	return nil, false
}

// deactivatedConstraint is a no-op constraint used when an annotation deactivates a constraint.
type deactivatedConstraint struct{}

func (c *deactivatedConstraint) ComponentIRI() string { return "" }
func (c *deactivatedConstraint) Evaluate(_ *evalContext, _ *Shape, _ Term, _ []Term) []ValidationResult {
	return nil
}

// severityOverrideConstraint wraps a constraint and overrides the result severity.
type severityOverrideConstraint struct {
	inner    Constraint
	severity Term
}

func (c *severityOverrideConstraint) ComponentIRI() string { return c.inner.ComponentIRI() }
func (c *severityOverrideConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	results := c.inner.Evaluate(ctx, shape, focusNode, valueNodes)
	for i := range results {
		results[i].ResultSeverity = c.severity
	}
	return results
}
