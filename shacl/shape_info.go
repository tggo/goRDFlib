package shacl

// ShapeInfo is a detached description of a parsed shape, without mutable engine
// state. It is safe for concurrent use when not modified; changing a copy does
// not change the prepared run. Path is the RDF node defining a property path.
type ShapeInfo struct {
	ID          Term
	IsProperty  bool
	Path        Term
	Deactivated bool
}

func shapeInfo(s *Shape) ShapeInfo {
	return ShapeInfo{ID: s.ID, IsProperty: s.IsProperty, Path: pathToTerm(s.Path), Deactivated: s.Deactivated}
}
