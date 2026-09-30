package shacl

import (
	"context"
	"testing"
)

// forEachRefConstraint calls fn with every sh:and/or/xone/not/node constraint
// of every parsed shape.
func forEachRefConstraint(shapes map[string]*Shape, fn func(c Constraint)) {
	for _, s := range shapes {
		for _, c := range s.Constraints {
			switch c.(type) {
			case *AndConstraint, *OrConstraint, *XoneConstraint, *NotConstraint, *NodeConstraint:
				fn(c)
			}
		}
	}
}

// TestParsedConstraintsCarryTheirShapeKeys: the parser precomputes the
// shapesMap key of every shape reference, and it is the key Term.String gives.
func TestParsedConstraintsCarryTheirShapeKeys(t *testing.T) {
	shapes, err := LoadTurtleString(quickShapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parseShapes(shapes)
	seen := 0
	check := func(refs []Term, keys []string) {
		if len(keys) != len(refs) {
			t.Errorf("%d references, %d keys", len(refs), len(keys))
			return
		}
		for i, r := range refs {
			if keys[i] != r.String() {
				t.Errorf("key %q, want %q", keys[i], r.String())
			}
		}
		seen++
	}
	forEachRefConstraint(parsed, func(c Constraint) {
		switch c := c.(type) {
		case *AndConstraint:
			check(c.Shapes, c.keys)
		case *OrConstraint:
			check(c.Shapes, c.keys)
		case *XoneConstraint:
			check(c.Shapes, c.keys)
		case *NotConstraint:
			check([]Term{c.ShapeRef}, []string{c.key})
		case *NodeConstraint:
			check([]Term{c.ShapeRef}, []string{c.key})
		}
	})
	if seen < 8 {
		t.Fatalf("only %d reference constraints inspected", seen)
	}
}

// TestConstraintWithoutShapeKeyStillResolves: a constraint built by hand has no
// precomputed key and must behave exactly like a parsed one.
func TestConstraintWithoutShapeKeyStillResolves(t *testing.T) {
	shapes, err := LoadTurtleString(quickShapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := LoadTurtleString(quickDataTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	want := Validate(data, shapes)

	compiled := parseShapes(shapes)
	stripped := 0
	forEachRefConstraint(compiled, func(c Constraint) {
		switch c := c.(type) {
		case *AndConstraint:
			c.keys = nil
		case *OrConstraint:
			c.keys = nil
		case *XoneConstraint:
			c.keys = nil
		case *NotConstraint:
			c.key = ""
		case *NodeConstraint:
			c.key = ""
		}
		stripped++
	})
	if stripped < 8 {
		t.Fatalf("only %d reference constraints stripped", stripped)
	}
	c := &CompiledShapes{shapesGraph: shapes, shapes: compiled}
	got, err := c.ValidateContext(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if match, details := CompareReports(want, got); !match {
		t.Errorf("report without precomputed keys differs:\n%s", details)
	}
	if len(want.Results) == 0 {
		t.Fatal("test data yields no violations; nothing was compared")
	}
}
