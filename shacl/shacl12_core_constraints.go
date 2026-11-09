package shacl

import (
	"sort"
	"strconv"
	"strings"
)

// RootClassConstraint implements sh:rootClass (SHACL 1.2 Core): every value
// node must be an IRI that is one of Classes or a SHACL subclass of one of them
// in the data graph.
type RootClassConstraint struct {
	Classes []Term
}

func (c *RootClassConstraint) ComponentIRI() string {
	return SH + "RootClassConstraintComponent"
}

func (c *RootClassConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		ok := false
		if vn.IsIRI() {
			for _, cls := range c.Classes {
				if ctx.dataGraph.IsSubClassOf(vn, cls) {
					ok = true
					break
				}
			}
		}
		if !ok {
			if ctx.guard != nil && ctx.guard.quick {
				return quickFailure
			}
			results = append(results, makeResult(shape, focusNode, vn, c.ComponentIRI()))
		}
	}
	return results
}

// UniqueValuesForConstraint implements sh:uniqueValuesFor (SHACL 1.2 Core): a
// value node must not have exactly the same values for Properties as another
// target node of the shape.
//
// The targets are those of the shape itself, also when the shape was reached
// through sh:node. A value node with no value for any of the properties is
// never a violation. Following the core spec's example and tests, the result
// carries no sh:value: what is duplicated is a combination of values, not a
// value node.
type UniqueValuesForConstraint struct {
	Properties []Term
}

func (c *UniqueValuesForConstraint) ComponentIRI() string {
	return SH + "UniqueValuesForConstraintComponent"
}

// key returns the comparison key of node: the exact value set of each property
// in turn. ok is false when node has no value for any of the properties.
func (c *UniqueValuesForConstraint) key(ctx *evalContext, node Term) (key string, ok bool) {
	var sb strings.Builder
	for _, p := range c.Properties {
		vals := ctx.dataGraph.Objects(node, p)
		keys := make([]string, len(vals))
		for i, v := range vals {
			keys[i] = v.TermKey()
		}
		sort.Strings(keys)
		keys = dedupSorted(keys)
		if len(keys) > 0 {
			ok = true
		}
		// Length prefixes keep one property's value set apart from the next.
		sb.WriteString(strconv.Itoa(len(keys)))
		for _, k := range keys {
			sb.WriteByte(0)
			sb.WriteString(strconv.Itoa(len(k)))
			sb.WriteByte(':')
			sb.WriteString(k)
		}
		sb.WriteByte(1)
	}
	return sb.String(), ok
}

func dedupSorted(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func (c *UniqueValuesForConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if len(c.Properties) == 0 {
		return nil
	}
	g := ctx.sharedGuard()
	idx, built := g.uniqueIndex[c]
	if !built {
		idx = make(map[string][]Term)
		for _, t := range resolveTargets(ctx, shape) {
			if k, ok := c.key(ctx, t); ok {
				idx[k] = append(idx[k], t)
			}
		}
		if g.uniqueIndex == nil {
			g.uniqueIndex = make(map[*UniqueValuesForConstraint]map[string][]Term)
		}
		g.uniqueIndex[c] = idx
	}
	var results []ValidationResult
	for _, vn := range valueNodes {
		k, ok := c.key(ctx, vn)
		if !ok {
			continue
		}
		for _, other := range idx[k] {
			if !other.Equal(vn) {
				if g.quick {
					return quickFailure
				}
				results = append(results, makeResult(shape, focusNode, Term{}, c.ComponentIRI()))
				break
			}
		}
	}
	return results
}
