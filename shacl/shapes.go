package shacl

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
	IgnoredProperties []Term
}

// Target represents a target declaration on a shape.
type Target struct {
	Kind  TargetKind
	Value Term
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
	// TargetImplicitClass represents an implicit class target (shape is also an rdfs:Class).
	TargetImplicitClass
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
	for _, pred := range []string{"not", "node", "qualifiedValueShape"} {
		p := IRI(SH + pred)
		for _, t := range g.All(nil, &p, nil) {
			getOrCreate(shapes, t.Object)
		}
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

	// Pass 2: parse constraints (now all shapes have their paths resolved)
	for _, s := range shapes {
		s.Constraints = parseConstraints(g, s, shapes)
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

	for _, tn := range g.Objects(id, IRI(SH+"targetNode")) {
		s.Targets = append(s.Targets, Target{Kind: TargetNode, Value: tn})
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

	// Implicit class targets: if shape is also an rdfs:Class
	typePred := IRI(RDFType)
	rdfsClass := IRI(RDFSClass)
	if g.Has(&id, &typePred, &rdfsClass) || g.HasType(id, rdfsClass) {
		if !s.IsProperty {
			s.Targets = append(s.Targets, Target{Kind: TargetImplicitClass, Value: id})
		}
	}

	if paths := g.Objects(id, IRI(SH+"path")); len(paths) > 0 {
		s.Path = parsePath(g, paths[0])
		s.IsProperty = true
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
		s.Closed = vals[0].Value() == "true"
	}
	for _, ip := range g.Objects(id, IRI(SH+"ignoredProperties")) {
		s.IgnoredProperties = g.RDFList(ip)
	}
}
