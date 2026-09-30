package shacl

// AndConstraint implements sh:and.
type AndConstraint struct {
	Shapes []Term
	keys   []string // shapesMap keys of Shapes, set by the parser; see refKey
}

func (c *AndConstraint) ComponentIRI() string {
	return SH + "AndConstraintComponent"
}

func (c *AndConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		for i, sRef := range c.Shapes {
			s, ok := ctx.shapesMap[refKey(c.keys, i, sRef)]
			if !ok {
				continue
			}
			if !nodeConforms(ctx, s, vn) {
				results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
				break
			}
		}
	}
	return results
}

// OrConstraint implements sh:or.
type OrConstraint struct {
	Shapes []Term
	keys   []string // shapesMap keys of Shapes, set by the parser; see refKey
}

func (c *OrConstraint) ComponentIRI() string {
	return SH + "OrConstraintComponent"
}

func (c *OrConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		anyConforms := false
		for i, sRef := range c.Shapes {
			s, ok := ctx.shapesMap[refKey(c.keys, i, sRef)]
			if !ok {
				continue
			}
			if nodeConforms(ctx, s, vn) {
				anyConforms = true
				break
			}
		}
		if !anyConforms {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// NotConstraint implements sh:not.
type NotConstraint struct {
	ShapeRef Term
	key      string // shapesMap key of ShapeRef, set by the parser; see refKey1
}

func (c *NotConstraint) ComponentIRI() string {
	return SH + "NotConstraintComponent"
}

func (c *NotConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	s, ok := ctx.shapesMap[refKey1(c.key, c.ShapeRef)]
	if !ok {
		return nil
	}
	var results []ValidationResult
	for _, vn := range valueNodes {
		if nodeConforms(ctx, s, vn) {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// XoneConstraint implements sh:xone.
type XoneConstraint struct {
	Shapes []Term
	keys   []string // shapesMap keys of Shapes, set by the parser; see refKey
}

func (c *XoneConstraint) ComponentIRI() string {
	return SH + "XoneConstraintComponent"
}

func (c *XoneConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		count := 0
		for i, sRef := range c.Shapes {
			s, ok := ctx.shapesMap[refKey(c.keys, i, sRef)]
			if !ok {
				continue
			}
			if len(validateNodeAgainstShape(ctx, s, vn)) == 0 {
				count++
			}
		}
		if count != 1 {
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// shapesMap is keyed by Term.String of a shape's IRI or blank node, which
// allocates on every call. The parser computes the key of each reference once
// and keeps it on the constraint; a constraint built by hand (the exported
// fields are enough to make one) has no key and computes it, so the cache can
// only ever be an optimisation.

// refKey returns the shapesMap key of Shapes[i]: the parsed one when keys has
// it, Shapes[i].String() otherwise.
func refKey(keys []string, i int, t Term) string {
	if i < len(keys) {
		return keys[i]
	}
	return t.String()
}

// refKey1 returns the shapesMap key of a single shape reference: key when the
// parser set it, t.String() otherwise.
func refKey1(key string, t Term) string {
	if key != "" {
		return key
	}
	return t.String()
}

// refKeys returns the shapesMap keys of ts, for the parser to store.
func refKeys(ts []Term) []string {
	keys := make([]string, len(ts))
	for i, t := range ts {
		keys[i] = t.String()
	}
	return keys
}
