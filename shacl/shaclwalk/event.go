package shaclwalk

import "github.com/tggo/goRDFlib/shacl"

// Event identifies a structural shape application and its target-selected origin.
// It contains only values and is safe for concurrent use when not modified. Via
// is the incoming SHACL predicate, or the zero term for a target-selected shape.
type Event struct {
	Phase         Phase
	SelectedShape shacl.Term
	SelectedFocus shacl.Term
	Shape         shacl.Term
	Focus         shacl.Term
	Via           shacl.Term
}
