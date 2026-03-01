package rdflibgo

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// TurtleSerializer serializes a Graph to Turtle format.
// Ported from: rdflib.plugins.serializers.turtle.TurtleSerializer
type TurtleSerializer struct{}

func init() {
	RegisterSerializer("turtle", func() Serializer { return &TurtleSerializer{} })
	RegisterSerializer("ttl", func() Serializer { return &TurtleSerializer{} })
}

// Serialize writes the graph in Turtle format.
// Ported from: rdflib.plugins.serializers.turtle.TurtleSerializer.serialize
func (s *TurtleSerializer) Serialize(g *Graph, w io.Writer, base string) error {
	ts := newTurtleState(g)
	ts.base = base
	ts.preprocess()
	ts.orderSubjects()
	return ts.write(w)
}

// turtleState holds serialization state.
type turtleState struct {
	g    *Graph
	base string

	// subject → predicate → []object
	spoMap map[string]map[string][]Term

	// subject order
	subjects []Subject

	// reference count for each term (as object)
	refs map[string]int

	// namespace tracking: only emit used prefixes
	usedNS map[string]URIRef // prefix → namespace

	// set of BNode keys that are list heads
	listHeads map[string]bool

	// set of BNodes that are part of a list (internal nodes)
	listNodes map[string]bool

	// serialized BNodes (avoid duplicates)
	serialized map[string]bool
}

func newTurtleState(g *Graph) *turtleState {
	return &turtleState{
		g:          g,
		spoMap:     make(map[string]map[string][]Term),
		refs:       make(map[string]int),
		usedNS:     make(map[string]URIRef),
		listHeads:  make(map[string]bool),
		listNodes:  make(map[string]bool),
		serialized: make(map[string]bool),
	}
}

// preprocess collects triples, counts references, detects lists, and tracks used prefixes.
// Ported from: rdflib.plugins.serializers.turtle.TurtleSerializer.preprocessTriple
func (ts *turtleState) preprocess() {
	ts.g.Triples(nil, nil, nil)(func(t Triple) bool {
		sk := termKey(t.Subject)
		pk := termKey(t.Predicate)

		if ts.spoMap[sk] == nil {
			ts.spoMap[sk] = make(map[string][]Term)
		}
		ts.spoMap[sk][pk] = append(ts.spoMap[sk][pk], t.Object)

		// Count object references
		ts.refs[termKey(t.Object)]++

		// Track used namespaces
		ts.trackNS(t.Subject)
		ts.trackNS(t.Predicate)
		ts.trackNS(t.Object)

		return true
	})

	// Detect rdf:List patterns
	ts.detectLists()
}

// trackNS registers a namespace as used if the term is a URIRef with a known prefix.
func (ts *turtleState) trackNS(t Term) {
	u, ok := t.(URIRef)
	if !ok {
		return
	}
	uri := u.Value()
	ts.g.Namespaces()(func(prefix string, ns URIRef) bool {
		nsStr := ns.Value()
		if strings.HasPrefix(uri, nsStr) && len(uri) > len(nsStr) {
			ts.usedNS[prefix] = ns
		}
		return true
	})
}

// detectLists finds rdf:List patterns.
// Ported from: rdflib.plugins.serializers.turtle.RecursiveSerializer.isValidList
func (ts *turtleState) detectLists() {
	firstKey := termKey(RDF.First)
	restKey := termKey(RDF.Rest)
	nilKey := termKey(RDF.Nil)

	for sk, preds := range ts.spoMap {
		if _, hasFirst := preds[firstKey]; !hasFirst {
			continue
		}
		// Validate: each list node must have exactly rdf:first and rdf:rest
		if ts.isValidList(sk, firstKey, restKey, nilKey) {
			ts.listHeads[sk] = true
			// Mark internal nodes
			ts.markListNodes(sk, restKey, nilKey)
		}
	}
}

func (ts *turtleState) isValidList(sk, firstKey, restKey, nilKey string) bool {
	node := sk
	visited := make(map[string]bool)
	for node != nilKey {
		if visited[node] {
			return false
		}
		visited[node] = true
		preds := ts.spoMap[node]
		if preds == nil {
			return false
		}
		firsts := preds[firstKey]
		rests := preds[restKey]
		if len(firsts) != 1 || len(rests) != 1 {
			return false
		}
		// List node should only have rdf:first and rdf:rest (and optionally rdf:type rdf:List)
		allowedPreds := 0
		for pk := range preds {
			if pk == firstKey || pk == restKey {
				allowedPreds++
			} else {
				return false
			}
		}
		if allowedPreds != 2 {
			return false
		}
		node = termKey(rests[0])
	}
	return true
}

func (ts *turtleState) markListNodes(sk, restKey, nilKey string) {
	node := sk
	for node != nilKey {
		ts.listNodes[node] = true
		rests := ts.spoMap[node][restKey]
		if len(rests) == 0 {
			break
		}
		node = termKey(rests[0])
	}
}

// orderSubjects sorts subjects for deterministic output.
// Ported from: rdflib.plugins.serializers.turtle.TurtleSerializer.orderSubjects
func (ts *turtleState) orderSubjects() {
	typeKey := termKey(RDF.Type)
	classKey := termKey(RDFS.Class)

	var topSubjects []Subject
	var bnodeSubjects []Subject
	var otherSubjects []Subject

	for sk := range ts.spoMap {
		// Skip list internal nodes
		if ts.listNodes[sk] {
			continue
		}

		subj := ts.resolveSubject(sk)
		if subj == nil {
			continue
		}

		// Check if it has rdf:type rdfs:Class
		if preds, ok := ts.spoMap[sk]; ok {
			if objs, ok := preds[typeKey]; ok {
				for _, o := range objs {
					if termKey(o) == classKey {
						topSubjects = append(topSubjects, subj)
						goto next
					}
				}
			}
		}

		if _, isBNode := subj.(BNode); isBNode {
			bnodeSubjects = append(bnodeSubjects, subj)
		} else {
			otherSubjects = append(otherSubjects, subj)
		}
	next:
	}

	sortSubjects := func(ss []Subject) {
		slices.SortFunc(ss, func(a, b Subject) int {
			return strings.Compare(a.N3(), b.N3())
		})
	}
	sortSubjects(topSubjects)
	sortSubjects(otherSubjects)
	// BNodes sorted by ref count ascending, then by N3
	slices.SortFunc(bnodeSubjects, func(a, b Subject) int {
		ra, rb := ts.refs[termKey(a)], ts.refs[termKey(b)]
		if ra != rb {
			return ra - rb
		}
		return strings.Compare(a.N3(), b.N3())
	})

	ts.subjects = append(ts.subjects, topSubjects...)
	ts.subjects = append(ts.subjects, otherSubjects...)
	ts.subjects = append(ts.subjects, bnodeSubjects...)
}

// resolveSubject finds the original Subject term from an N3 key.
func (ts *turtleState) resolveSubject(sk string) Subject {
	var result Subject
	ts.g.Triples(nil, nil, nil)(func(t Triple) bool {
		if termKey(t.Subject) == sk {
			result = t.Subject
			return false
		}
		return true
	})
	return result
}

// write outputs the Turtle document.
func (ts *turtleState) write(w io.Writer) error {
	// @base
	if ts.base != "" {
		if _, err := fmt.Fprintf(w, "@base <%s> .\n", ts.base); err != nil {
			return err
		}
	}

	// @prefix declarations (sorted, only used)
	var prefixes []string
	for p := range ts.usedNS {
		prefixes = append(prefixes, p)
	}
	slices.Sort(prefixes)
	for _, p := range prefixes {
		if _, err := fmt.Fprintf(w, "@prefix %s: <%s> .\n", p, ts.usedNS[p].Value()); err != nil {
			return err
		}
	}
	if len(prefixes) > 0 || ts.base != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	// Subjects
	for i, subj := range ts.subjects {
		sk := termKey(subj)
		if ts.serialized[sk] {
			continue
		}
		if err := ts.writeSubject(w, subj); err != nil {
			return err
		}
		if i < len(ts.subjects)-1 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeSubject writes a single subject block.
func (ts *turtleState) writeSubject(w io.Writer, subj Subject) error {
	sk := termKey(subj)
	ts.serialized[sk] = true

	// Check if this BNode can be inlined (referenced 0 times)
	if _, isBNode := subj.(BNode); isBNode && ts.refs[sk] == 0 && !ts.listHeads[sk] {
		if _, err := fmt.Fprintf(w, "[]"); err != nil {
			return err
		}
		return ts.writePredicates(w, sk, " ")
	}

	label := ts.label(subj)
	if _, err := fmt.Fprintf(w, "%s", label); err != nil {
		return err
	}
	return ts.writePredicates(w, sk, " ")
}

// writePredicates writes the predicate-object list for a subject.
func (ts *turtleState) writePredicates(w io.Writer, sk string, indent string) error {
	preds := ts.spoMap[sk]
	if len(preds) == 0 {
		_, err := fmt.Fprintln(w, " .")
		return err
	}

	// Sort predicates: rdf:type first, then alphabetically
	sortedPreds := ts.sortPredicates(preds)

	for i, pk := range sortedPreds {
		objs := preds[pk]
		predLabel := ts.predLabel(pk)

		if i == 0 {
			if _, err := fmt.Fprintf(w, " %s", predLabel); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, " ;\n    %s", predLabel); err != nil {
				return err
			}
		}

		// Sort objects
		slices.SortFunc(objs, func(a, b Term) int {
			return CompareTerm(a, b)
		})

		for j, obj := range objs {
			if j > 0 {
				if _, err := fmt.Fprintf(w, ","); err != nil {
					return err
				}
			}
			objStr, err := ts.objectStr(w, obj)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, " %s", objStr); err != nil {
				return err
			}
		}
	}

	_, err := fmt.Fprintln(w, " .")
	return err
}

// sortPredicates returns predicate keys with rdf:type first, then rdfs:label, then alphabetical.
// Ported from: rdflib.plugins.serializers.turtle.TurtleSerializer.sortProperties
func (ts *turtleState) sortPredicates(preds map[string][]Term) []string {
	typeKey := termKey(RDF.Type)
	labelKey := termKey(RDFS.Label)

	var ordered []string
	var rest []string

	for pk := range preds {
		switch pk {
		case typeKey, labelKey:
			// handled separately
		default:
			rest = append(rest, pk)
		}
	}

	slices.Sort(rest)

	if _, ok := preds[typeKey]; ok {
		ordered = append(ordered, typeKey)
	}
	if _, ok := preds[labelKey]; ok {
		ordered = append(ordered, labelKey)
	}
	ordered = append(ordered, rest...)
	return ordered
}

// label returns the Turtle representation of a term in subject position.
func (ts *turtleState) label(t Term) string {
	switch v := t.(type) {
	case URIRef:
		return ts.qnameOrFull(v)
	case BNode:
		return v.N3()
	default:
		return t.N3()
	}
}

// predLabel returns the Turtle representation of a predicate.
// Ported from: uses "a" shorthand for rdf:type
func (ts *turtleState) predLabel(pk string) string {
	if pk == termKey(RDF.Type) {
		return "a"
	}
	// Try to resolve to a URIRef and get qname
	// pk is N3 form like <http://...>
	uri := strings.TrimPrefix(strings.TrimSuffix(pk, ">"), "<")
	u := NewURIRefUnsafe(uri)
	return ts.qnameOrFull(u)
}

// objectStr returns the Turtle representation of an object term.
func (ts *turtleState) objectStr(w io.Writer, t Term) (string, error) {
	switch v := t.(type) {
	case URIRef:
		return ts.qnameOrFull(v), nil
	case BNode:
		bk := termKey(v)
		// Check if it's a list head
		if ts.listHeads[bk] && !ts.serialized[bk] {
			return ts.listStr(v), nil
		}
		// Inline blank node if referenced only once and not yet serialized
		if ts.refs[bk] <= 1 && !ts.serialized[bk] {
			if preds := ts.spoMap[bk]; len(preds) > 0 {
				return ts.inlineBNode(v), nil
			}
		}
		return v.N3(), nil
	case Literal:
		return ts.literalStr(v), nil
	default:
		return t.N3(), nil
	}
}

// literalStr formats a literal for Turtle output.
// Ported from: rdflib.plugins.serializers.turtle — literal formatting
func (ts *turtleState) literalStr(l Literal) string {
	n3 := l.N3()
	// If N3 already uses shorthand (integer, boolean, decimal), use it
	if !strings.HasPrefix(n3, "\"") {
		return n3
	}
	// Try to use prefixed datatype
	if l.Language() == "" && l.Datatype() != (URIRef{}) && l.Datatype() != XSDString {
		// Replace ^^<full-uri> with ^^prefix:local
		dtN3 := l.Datatype().N3()
		dtQName := ts.qnameOrFull(l.Datatype())
		if dtQName != dtN3 {
			return strings.Replace(n3, "^^"+dtN3, "^^"+dtQName, 1)
		}
	}
	return n3
}

// qnameOrFull returns a prefixed name if possible, otherwise the full N3 form.
func (ts *turtleState) qnameOrFull(u URIRef) string {
	uri := u.Value()
	bestPrefix := ""
	bestNS := ""

	for p, ns := range ts.usedNS {
		nsStr := ns.Value()
		if strings.HasPrefix(uri, nsStr) && len(uri) > len(nsStr) {
			if len(nsStr) > len(bestNS) {
				bestPrefix = p
				bestNS = nsStr
			}
		}
	}
	if bestNS != "" {
		local := uri[len(bestNS):]
		// Verify local name is valid (no special chars)
		if isValidLocalName(local) {
			return bestPrefix + ":" + local
		}
	}
	return u.N3()
}

// isValidLocalName checks if a string is valid as a Turtle local name.
func isValidLocalName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c == '/' || c == ' ' || c == '<' || c == '>' || c == '{' || c == '}' || c == '"' || c == '?' || c == '#' {
			return false
		}
	}
	return true
}

// listStr serializes an rdf:List as Turtle collection syntax: ( item1 item2 ... )
func (ts *turtleState) listStr(head BNode) string {
	var items []string
	restKey := termKey(RDF.Rest)
	firstKey := termKey(RDF.First)
	nilKey := termKey(RDF.Nil)

	node := termKey(head)
	for node != nilKey {
		ts.serialized[node] = true
		firsts := ts.spoMap[node][firstKey]
		if len(firsts) > 0 {
			str, _ := ts.objectStr(nil, firsts[0])
			items = append(items, str)
		}
		rests := ts.spoMap[node][restKey]
		if len(rests) == 0 {
			break
		}
		node = termKey(rests[0])
	}

	return "( " + strings.Join(items, " ") + " )"
}

// inlineBNode serializes a blank node inline: [ pred1 obj1 ; pred2 obj2 ]
func (ts *turtleState) inlineBNode(b BNode) string {
	sk := termKey(b)
	ts.serialized[sk] = true
	preds := ts.spoMap[sk]

	sortedPreds := ts.sortPredicates(preds)
	var parts []string

	for _, pk := range sortedPreds {
		objs := preds[pk]
		predLabel := ts.predLabel(pk)

		slices.SortFunc(objs, func(a, b Term) int {
			return CompareTerm(a, b)
		})

		var objStrs []string
		for _, obj := range objs {
			str, _ := ts.objectStr(nil, obj)
			objStrs = append(objStrs, str)
		}
		parts = append(parts, predLabel+" "+strings.Join(objStrs, ", "))
	}

	return "[ " + strings.Join(parts, " ; ") + " ]"
}
