package shacl

import (
	"fmt"
	"sort"
	"strings"
)

// WithDefaultMessages fills ResultMessages of every result that has none with
// a human-readable message generated from its constraint component and the
// shape's parameters, e.g. "Less than 1 values on ex:alice->ex:name" — what
// pySHACL and TopBraid produce and what most UIs display (issue #47).
//
// SHACL does not require a message, and a message from the shapes graph (or
// a SPARQL ?message binding) always wins: only results that would otherwise
// be empty are filled. Names are shortened with the prefixes declared in the
// shapes and data graphs. Messages are plain literals in English.
//
// Off by default, because it changes ResultMessages for every caller that
// compares or serializes reports.
func WithDefaultMessages() Option {
	return func(c *config) { c.defaultMessages = true }
}

// annotateDefaultMessages runs once over the finished report, after ordering,
// so it never changes the order of results. Like annotateSourceLines, it keeps
// constraints unaware that default messages exist.
func annotateDefaultMessages(results []ValidationResult, ctx *evalContext) {
	if !ctx.cfg.defaultMessages || len(results) == 0 {
		return
	}
	m := messager{shapes: ctx.shapesGraph, names: newNamer(ctx.shapesGraph, ctx.dataGraph)}
	m.annotate(results)
}

type messager struct {
	shapes *Graph
	names  namer
}

func (m messager) annotate(results []ValidationResult) {
	for i := range results {
		r := &results[i]
		if len(r.ResultMessages) == 0 {
			if msg := m.message(r); msg != "" {
				r.ResultMessages = []Term{Literal(msg, "", "")}
			}
		}
		m.annotate(r.Details)
	}
}

// message builds the default text for r, or "" for a result that names no
// component.
func (m messager) message(r *ValidationResult) string {
	comp := r.SourceConstraintComponent.Value()
	if !strings.HasPrefix(comp, SH) {
		if comp == "" {
			return ""
		}
		return fmt.Sprintf("Constraint %s violated on %s", m.names.name(r.SourceConstraintComponent), m.where(r))
	}
	param := func(p string) string { return m.param(r.SourceShape, p) }
	v, on := m.value(r), m.where(r)
	switch strings.TrimSuffix(comp[len(SH):], "ConstraintComponent") {
	case "MinCount":
		return fmt.Sprintf("Less than %s values on %s", param("minCount"), on)
	case "MaxCount":
		return fmt.Sprintf("More than %s values on %s", param("maxCount"), on)
	case "QualifiedMinCount":
		return fmt.Sprintf("Less than %s values on %s conform to %s", param("qualifiedMinCount"), on, param("qualifiedValueShape"))
	case "QualifiedMaxCount":
		return fmt.Sprintf("More than %s values on %s conform to %s", param("qualifiedMaxCount"), on, param("qualifiedValueShape"))
	case "Class":
		return fmt.Sprintf("Value %s on %s does not have class %s", v, on, param("class"))
	case "Datatype":
		return fmt.Sprintf("Value %s on %s is not a literal of datatype %s", v, on, param("datatype"))
	case "NodeKind":
		return fmt.Sprintf("Value %s on %s is not of node kind %s", v, on, param("nodeKind"))
	case "MinLength":
		return fmt.Sprintf("Value %s on %s is shorter than %s characters", v, on, param("minLength"))
	case "MaxLength":
		return fmt.Sprintf("Value %s on %s is longer than %s characters", v, on, param("maxLength"))
	case "Pattern":
		return fmt.Sprintf("Value %s on %s does not match pattern %s", v, on, param("pattern"))
	case "MinInclusive":
		return fmt.Sprintf("Value %s on %s is not >= %s", v, on, param("minInclusive"))
	case "MinExclusive":
		return fmt.Sprintf("Value %s on %s is not > %s", v, on, param("minExclusive"))
	case "MaxInclusive":
		return fmt.Sprintf("Value %s on %s is not <= %s", v, on, param("maxInclusive"))
	case "MaxExclusive":
		return fmt.Sprintf("Value %s on %s is not < %s", v, on, param("maxExclusive"))
	case "In":
		return fmt.Sprintf("Value %s on %s is not in %s", v, on, param("in"))
	case "LanguageIn":
		return fmt.Sprintf("Language of %s on %s is not in %s", v, on, param("languageIn"))
	case "UniqueLang":
		return fmt.Sprintf("More than one value with the same language tag on %s", on)
	case "HasValue":
		return fmt.Sprintf("Missing expected value %s on %s", param("hasValue"), on)
	case "Equals":
		return fmt.Sprintf("Value %s on %s is not also a value of %s", v, on, param("equals"))
	case "Disjoint":
		return fmt.Sprintf("Value %s on %s is also a value of %s", v, on, param("disjoint"))
	case "LessThan":
		return fmt.Sprintf("Value %s on %s is not less than the values of %s", v, on, param("lessThan"))
	case "LessThanOrEquals":
		return fmt.Sprintf("Value %s on %s is not less than or equal to the values of %s", v, on, param("lessThanOrEquals"))
	case "Node":
		return fmt.Sprintf("Value %s on %s does not conform to %s", v, on, param("node"))
	case "Property":
		return fmt.Sprintf("Value %s on %s does not conform to property shape %s", v, on, param("property"))
	case "Not":
		return fmt.Sprintf("Value %s on %s conforms to %s, which it must not", v, on, param("not"))
	case "And":
		return fmt.Sprintf("Value %s on %s does not conform to all shapes in sh:and", v, on)
	case "Or":
		return fmt.Sprintf("Value %s on %s does not conform to any shape in sh:or", v, on)
	case "Xone":
		return fmt.Sprintf("Value %s on %s does not conform to exactly one shape in sh:xone", v, on)
	case "Closed":
		return fmt.Sprintf("Predicate %s is not allowed on %s (closed shape)", m.names.name(r.ResultPath), m.names.name(r.FocusNode))
	}
	return fmt.Sprintf("%s violated on %s", m.names.name(r.SourceConstraintComponent), on)
}

// where is "focus->path", or the focus alone for a node shape.
func (m messager) where(r *ValidationResult) string {
	if r.ResultPath.IsNone() {
		return m.names.name(r.FocusNode)
	}
	return m.names.name(r.FocusNode) + "->" + m.path(r.ResultPath)
}

func (m messager) path(p Term) string {
	if p.IsIRI() {
		return m.names.name(p)
	}
	return "a complex path"
}

func (m messager) value(r *ValidationResult) string {
	if r.Value.IsNone() {
		return m.names.name(r.FocusNode)
	}
	return m.names.name(r.Value)
}

// param renders the value of the shape's sh:<p>. A list (sh:in, sh:languageIn)
// is written in parentheses; several values, which only shape-valued
// parameters have, are joined with "or".
func (m messager) param(shape Term, p string) string {
	vals := m.shapes.Objects(shape, IRI(SH+p))
	if len(vals) == 0 {
		return "?"
	}
	if p == "in" || p == "languageIn" {
		items := m.shapes.RDFList(vals[0])
		parts := make([]string, len(items))
		for i, it := range items {
			parts[i] = m.names.name(it)
		}
		return "(" + strings.Join(parts, " ") + ")"
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		if v.IsBlank() {
			parts[i] = "an anonymous shape"
		} else {
			parts[i] = m.names.name(v)
		}
	}
	return strings.Join(parts, " or ")
}

// namer shortens IRIs with declared prefixes, longest namespace first.
type namer struct{ ns []nsBinding }

type nsBinding struct{ prefix, ns string }

func newNamer(graphs ...*Graph) namer {
	seen := make(map[string]bool)
	var n namer
	add := func(prefix, ns string) {
		if ns != "" && !seen[ns] {
			seen[ns] = true
			n.ns = append(n.ns, nsBinding{prefix, ns})
		}
	}
	for _, g := range graphs {
		if g == nil {
			continue
		}
		for prefix, ns := range g.namespaces() {
			add(prefix, ns.Value())
		}
	}
	add("sh", SH)
	add("xsd", "http://www.w3.org/2001/XMLSchema#")
	add("rdf", "http://www.w3.org/1999/02/22-rdf-syntax-ns#")
	sort.Slice(n.ns, func(i, j int) bool {
		if len(n.ns[i].ns) != len(n.ns[j].ns) {
			return len(n.ns[i].ns) > len(n.ns[j].ns)
		}
		return n.ns[i].prefix < n.ns[j].prefix
	})
	return n
}

func (n namer) name(t Term) string {
	switch {
	case t.IsNone():
		return "?"
	case t.IsIRI():
		v := t.Value()
		for _, b := range n.ns {
			if local, ok := strings.CutPrefix(v, b.ns); ok && local != "" && !strings.ContainsAny(local, "/#?:") {
				return b.prefix + ":" + local
			}
		}
		return "<" + v + ">"
	case t.IsLiteral():
		switch strings.TrimPrefix(t.Datatype(), "http://www.w3.org/2001/XMLSchema#") {
		case "", "string":
			return t.String()
		case "integer", "decimal", "double", "boolean":
			return t.Value() // written bare, as Turtle would
		}
		if t.Language() != "" {
			return t.String()
		}
		return fmt.Sprintf("%q^^%s", t.Value(), n.name(IRI(t.Datatype())))
	}
	return t.String()
}
