package shacl

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// ErrIllFormedShape is reported for a shape the SHACL spec calls ill-formed,
// such as a property shape without exactly one sh:path (§2.3.2). It reaches the
// caller through WithErrorHandler, or as the error of CompileShapes,
// PrepareContext and ValidateContext under WithStrictShapes.
var ErrIllFormedShape = errors.New("shacl: ill-formed shape")

// WithStrictShapes makes an ill-formed shape (ErrIllFormedShape) a hard error:
// CompileShapes, PrepareContext and ValidateContext return it instead of
// validating. Validate and Prepare have no error result, so for them it is
// still only reported through WithErrorHandler.
//
// Without it, ill-formed shapes are reported to the error handler and
// validation goes ahead with them as they are, which is what earlier versions
// did silently.
func WithStrictShapes() Option {
	return func(c *config) { c.strictShapes = true }
}

// lenientShapes undoes WithStrictShapes for the entry points that have no
// error result: there an ill-formed shape is reported, not returned.
func lenientShapes(c *config) { c.strictShapes = false }

// illFormedShapes checks the parsed shapes against the well-formedness rules
// that change what a shape means. The result is in shape key order, so a
// handler sees the same sequence on every run.
func illFormedShapes(g *Graph, shapes map[string]*Shape) []error {
	var errs []error
	pathPred := IRI(SH + "path")
	for _, key := range slices.Sorted(maps.Keys(shapes)) {
		s := shapes[key]
		if !s.IsProperty {
			continue
		}
		paths := g.Objects(s.ID, pathPred)
		switch {
		case len(paths) == 0:
			errs = append(errs, fmt.Errorf("%w: property shape %s%s has no sh:path; a property shape needs exactly one (SHACL 2.3.2)",
				ErrIllFormedShape, s.ID, shapeParents(g, s.ID)))
		case len(paths) > 1:
			errs = append(errs, fmt.Errorf("%w: property shape %s%s has %d values for sh:path; it must have exactly one (SHACL 2.3.2)",
				ErrIllFormedShape, s.ID, shapeParents(g, s.ID), len(paths)))
		case paths[0].IsLiteral():
			errs = append(errs, fmt.Errorf("%w: property shape %s%s has the literal %s as sh:path; a path is an IRI or a blank node (SHACL 2.3.1)",
				ErrIllFormedShape, s.ID, shapeParents(g, s.ID), paths[0]))
		}
	}
	return errs
}

// shapeParents names the shapes that point at id through sh:property, so a
// blank-node property shape can be found in the shapes file.
func shapeParents(g *Graph, id Term) string {
	parents := g.Subjects(IRI(SH+"property"), id)
	if len(parents) == 0 {
		return ""
	}
	return fmt.Sprintf(" (the sh:property of %s)", parents[0])
}

// checkShapes reports problems to the error handler and, under
// WithStrictShapes, returns them joined.
func (c *config) checkShapes(problems []error) error {
	if len(problems) == 0 {
		return nil
	}
	if c.strictShapes {
		return errors.Join(problems...)
	}
	for _, err := range problems {
		c.report(err)
	}
	return nil
}
