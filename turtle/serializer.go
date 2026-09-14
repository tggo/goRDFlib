package turtle

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/term"
)

// Serialize writes the graph in Turtle format.
//
// By default the output is compact. Pass WithPretty or WithIndent for an
// indented layout, and WithMaxNestDepth to change how deeply blank nodes are
// nested inline.
//
// Serialize is safe for concurrent use provided the graph is not mutated
// concurrently; it holds all of its state locally.
func Serialize(g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	ts := newTurtleState(g)
	ts.base = cfg.base
	ts.pretty = cfg.pretty
	ts.indentUnit = cfg.indentUnit()
	ts.maxNestDepth = cfg.nestDepth()
	if cfg.base != "" {
		ts.checkIRI(cfg.base)
	}
	ts.preprocess()
	if ts.err != nil {
		return ts.err
	}
	ts.orderSubjects()
	return ts.write(w)
}

// termKey returns a string key for a term (its N3 representation).
func termKey(t rdflibgo.Term) string {
	return t.N3()
}

// turtleState holds serialization state.
type turtleState struct {
	g    *rdflibgo.Graph
	base string

	// subject -> predicate -> []object
	spoMap map[string]map[string][]rdflibgo.Term

	// subject order
	subjects []rdflibgo.Subject

	// reference count for each term (as object)
	refs map[string]int

	// namespace tracking: only emit used prefixes
	usedNS map[string]rdflibgo.URIRef // prefix -> namespace

	// blank nodes that can be written as (part of) a collection; see
	// serializer_lists.go
	listCells map[string]bool

	// serialized BNodes (avoid duplicates)
	serialized map[string]bool

	// subjectMap maps N3 key -> Subject for O(1) lookup
	subjectMap map[string]rdflibgo.Subject

	// layout
	pretty       bool
	indentUnit   string
	indentCache  []string
	maxNestDepth int
	depth        int // current inline nesting depth

	// nodes whose inline form was suppressed by maxNestDepth; they are emitted
	// as top-level statements so no triple is lost.
	deferred    []string
	deferredSet map[string]bool

	// precomputed term keys (N3 of well-known terms)
	typeKey, labelKey string
	firstKey, restKey string
	nilKey, classKey  string
	qnameCache        map[string]string
	predCache         map[string]string
	nsSeen            map[string]bool

	// err is the first IRI found that Turtle cannot represent.
	err error
}

func newTurtleState(g *rdflibgo.Graph) *turtleState {
	return &turtleState{
		g:           g,
		spoMap:      make(map[string]map[string][]rdflibgo.Term),
		refs:        make(map[string]int),
		usedNS:      make(map[string]rdflibgo.URIRef),
		listCells:   make(map[string]bool),
		serialized:  make(map[string]bool),
		subjectMap:  make(map[string]rdflibgo.Subject),
		deferredSet: make(map[string]bool),
		indentCache: []string{""},
		qnameCache:  make(map[string]string),
		predCache:   make(map[string]string),
		nsSeen:      make(map[string]bool),

		typeKey:  termKey(rdflibgo.RDF.Type),
		labelKey: termKey(rdflibgo.RDFS.Label),
		firstKey: termKey(rdflibgo.RDF.First),
		restKey:  termKey(rdflibgo.RDF.Rest),
		nilKey:   termKey(rdflibgo.RDF.Nil),
		classKey: termKey(rdflibgo.RDFS.Class),

		maxNestDepth: defaultMaxNestDepth,
		indentUnit:   spaces(defaultIndentWidth),
	}
}

// indent returns the indentation prefix for the given nesting level.
func (ts *turtleState) indent(level int) string {
	if level <= 0 {
		return ""
	}
	for len(ts.indentCache) <= level {
		ts.indentCache = append(ts.indentCache, ts.indentCache[len(ts.indentCache)-1]+ts.indentUnit)
	}
	return ts.indentCache[level]
}

// preprocess collects triples, counts references, detects lists, and tracks used prefixes.
func (ts *turtleState) preprocess() {
	ts.g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
		sk := termKey(t.Subject)
		pk := termKey(t.Predicate)

		if ts.spoMap[sk] == nil {
			ts.spoMap[sk] = make(map[string][]rdflibgo.Term)
		}
		ts.subjectMap[sk] = t.Subject
		ts.spoMap[sk][pk] = append(ts.spoMap[sk][pk], t.Object)

		// Count object references
		ts.refs[termKey(t.Object)]++

		// Track used namespaces
		ts.trackNS(t.Subject)
		ts.trackNS(t.Predicate)
		ts.trackNS(t.Object)
		if l, ok := t.Object.(rdflibgo.Literal); ok {
			ts.checkIRI(l.Datatype().Value())
		}

		return true
	})

	// Detect rdf:List patterns
	ts.detectLists()
}

// trackNS registers a namespace as used if the term is a URIRef with a known prefix.
func (ts *turtleState) trackNS(t rdflibgo.Term) {
	if tt, ok := t.(rdflibgo.TripleTerm); ok {
		ts.trackNS(tt.Subject())
		ts.trackNS(tt.Predicate())
		ts.trackNS(tt.Object())
		return
	}
	u, ok := t.(rdflibgo.URIRef)
	if !ok {
		return
	}
	uri := u.Value()
	// Every triple passes three terms through here and the same IRIs recur
	// constantly; scanning the namespace table once per distinct IRI keeps this
	// off the hot path.
	if ts.nsSeen[uri] {
		return
	}
	ts.nsSeen[uri] = true
	ts.checkIRI(uri)
	ts.g.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
		nsStr := ns.Value()
		if strings.HasPrefix(uri, nsStr) && len(uri) > len(nsStr) && isValidPrefixName(prefix) {
			ts.checkIRI(nsStr)
			ts.usedNS[prefix] = ns
		}
		return true
	})
}

// checkIRI records an error for an IRI that no Turtle IRIREF can hold.
// Turtle 1.1 [18] excludes #x00-#x20 and <>"{}|^`\ from IRIREF, and a \u
// escape of one of them is rejected too, so there is no way to write it;
// emitting it anyway produced a document that does not parse.
func (ts *turtleState) checkIRI(uri string) {
	if ts.err == nil && !term.ValidIRI(uri) {
		ts.err = fmt.Errorf("turtle: cannot serialize IRI %q: %w: Turtle IRIs cannot contain spaces, control characters or <>\"{}|^`\\, not even escaped; percent-encode them", uri, rdflibgo.ErrInvalidIRI)
	}
}

// orderSubjects sorts subjects for deterministic output.
func (ts *turtleState) orderSubjects() {
	typeKey, classKey := ts.typeKey, ts.classKey

	var topSubjects []rdflibgo.Subject
	var bnodeSubjects []rdflibgo.Subject
	var otherSubjects []rdflibgo.Subject

	for sk := range ts.spoMap {
		// List cells are written by their single reference.
		if ts.listCells[sk] {
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

		if _, isBNode := subj.(rdflibgo.BNode); isBNode {
			bnodeSubjects = append(bnodeSubjects, subj)
		} else {
			otherSubjects = append(otherSubjects, subj)
		}
	next:
	}

	sortSubjects := func(ss []rdflibgo.Subject) {
		slices.SortFunc(ss, func(a, b rdflibgo.Subject) int {
			return strings.Compare(a.N3(), b.N3())
		})
	}
	sortSubjects(topSubjects)
	sortSubjects(otherSubjects)
	// BNodes sorted by ref count ascending, then by N3
	slices.SortFunc(bnodeSubjects, func(a, b rdflibgo.Subject) int {
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

// resolveSubject finds the original Subject term from an N3 key via the precomputed map.
func (ts *turtleState) resolveSubject(sk string) rdflibgo.Subject {
	return ts.subjectMap[sk]
}

// write outputs the Turtle document.
func (ts *turtleState) write(w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	w = bw

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

	// Subjects. A blank line separates consecutive statements; subjects already
	// emitted inline (as [ ... ] or as a collection) are skipped.
	wrote := false
	writeOne := func(subj rdflibgo.Subject) error {
		if wrote {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		wrote = true
		return ts.writeSubject(w, subj)
	}

	for _, subj := range ts.subjects {
		if ts.serialized[termKey(subj)] {
			continue
		}
		if err := writeOne(subj); err != nil {
			return err
		}
	}

	// Nodes that hit the nesting limit are emitted here. The queue may grow
	// while it is drained (a flattened node can itself defer another); it
	// terminates because every entry is a distinct key of spoMap and each pass
	// marks its node serialized.
	next := 0
	drainDeferred := func() error {
		for ; next < len(ts.deferred); next++ {
			sk := ts.deferred[next]
			if ts.serialized[sk] {
				continue
			}
			subj := ts.subjectMap[sk]
			if subj == nil {
				continue
			}
			if err := writeOne(subj); err != nil {
				return err
			}
		}
		return nil
	}
	if err := drainDeferred(); err != nil {
		return err
	}

	// Whatever is still unwritten is only reachable from inside itself: a
	// list cell whose single reference comes from within its own collection
	// (a cycle through rdf:first). Emit it as a labelled statement so no
	// triple is lost; that may defer further nodes, which are drained again.
	var rest []string
	for sk := range ts.spoMap {
		if !ts.serialized[sk] {
			rest = append(rest, sk)
		}
	}
	slices.Sort(rest)
	for _, sk := range rest {
		if ts.serialized[sk] {
			continue
		}
		if err := writeOne(ts.subjectMap[sk]); err != nil {
			return err
		}
		if err := drainDeferred(); err != nil {
			return err
		}
	}

	return bw.Flush()
}

// writeSubject writes a single subject block.
func (ts *turtleState) writeSubject(w io.Writer, subj rdflibgo.Subject) error {
	sk := termKey(subj)
	ts.serialized[sk] = true

	head := ts.label(subj)
	// A BNode nothing refers to needs no label of its own.
	if _, isBNode := subj.(rdflibgo.BNode); isBNode && ts.refs[sk] == 0 {
		head = "[]"
	}
	if _, err := io.WriteString(w, head); err != nil {
		return err
	}
	return ts.writePredicates(w, sk, 1, " .\n")
}

// writePredicates writes the predicate-object list for sk, followed by
// terminator. level is the nesting level the predicates are indented to (only
// used in the pretty layout).
func (ts *turtleState) writePredicates(w io.Writer, sk string, level int, terminator string) error {
	preds := ts.spoMap[sk]
	if len(preds) == 0 {
		_, err := io.WriteString(w, terminator)
		return err
	}

	// Sort predicates: rdf:type first, then rdfs:label, then alphabetically
	sortedPreds := ts.sortPredicates(preds)

	for i, pk := range sortedPreds {
		objs := preds[pk]

		if i > 0 {
			if _, err := io.WriteString(w, " ;"); err != nil {
				return err
			}
		}
		switch {
		case ts.pretty:
			if _, err := io.WriteString(w, "\n"+ts.indent(level)+ts.predLabel(pk)); err != nil {
				return err
			}
		case i == 0:
			if _, err := io.WriteString(w, " "+ts.predLabel(pk)); err != nil {
				return err
			}
		default:
			if _, err := io.WriteString(w, "\n"+ts.indent(1)+ts.predLabel(pk)); err != nil {
				return err
			}
		}

		// Sort objects
		slices.SortFunc(objs, func(a, b rdflibgo.Term) int {
			return rdflibgo.CompareTerm(a, b)
		})

		// In the pretty layout objects go on their own lines when there is more
		// than one, or when the single object expands into a block.
		ownLines := ts.pretty && (len(objs) > 1 || ts.expandsToBlock(objs[0]))

		for j, obj := range objs {
			if j > 0 {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			sep := " "
			if ownLines {
				sep = "\n" + ts.indent(level+1)
			}
			if _, err := io.WriteString(w, sep); err != nil {
				return err
			}
			if err := ts.writeObject(w, obj, level+1); err != nil {
				return err
			}
		}
	}

	_, err := io.WriteString(w, terminator)
	return err
}

// sortPredicates returns predicate keys with rdf:type first, then rdfs:label, then alphabetical.
func (ts *turtleState) sortPredicates(preds map[string][]rdflibgo.Term) []string {
	typeKey, labelKey := ts.typeKey, ts.labelKey

	ordered := make([]string, 0, len(preds))
	rest := make([]string, 0, len(preds))

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
func (ts *turtleState) label(t rdflibgo.Term) string {
	switch v := t.(type) {
	case rdflibgo.URIRef:
		return ts.qnameOrFull(v)
	case rdflibgo.BNode:
		return v.N3()
	default:
		return t.N3()
	}
}

// predLabel returns the Turtle representation of a predicate.
func (ts *turtleState) predLabel(pk string) string {
	if pk == ts.typeKey {
		return "a"
	}
	if s, ok := ts.predCache[pk]; ok {
		return s
	}
	// Resolve to a URIRef and get its qname. pk is N3 form like <http://...>.
	uri := strings.TrimPrefix(strings.TrimSuffix(pk, ">"), "<")
	s := ts.qnameOrFull(rdflibgo.NewURIRefUnsafe(uri))
	ts.predCache[pk] = s
	return s
}

// objectForm classifies how an object term is written in object position.
type objectForm int

const (
	formScalar objectForm = iota // written as a single token
	formBNode                    // expanded into [ ... ]
	formList                     // expanded into ( ... )
)

// objectForm decides whether t is expanded inline. The decision depends on
// mutable state (what has been serialized already, current nesting depth), so
// it must be taken immediately before the term is written.
func (ts *turtleState) objectForm(t rdflibgo.Term) objectForm {
	b, ok := t.(rdflibgo.BNode)
	if !ok {
		return formScalar
	}
	bk := termKey(b)
	if ts.serialized[bk] {
		return formScalar
	}
	// Inline nesting is recursive; stop before an adversarially deep graph
	// turns into deep recursion and a quadratic amount of indentation.
	if ts.depth >= ts.maxNestDepth {
		return formScalar
	}
	if ts.listCells[bk] && ts.canWriteList(bk) {
		return formList
	}
	// Inline a blank node only when nothing else refers to it.
	if ts.refs[bk] <= 1 && len(ts.spoMap[bk]) > 0 {
		return formBNode
	}
	return formScalar
}

// expandsToBlock reports whether t is written across several lines in the
// pretty layout. A blank node always is; a collection only when one of its
// items is itself a block, so that flat collections stay on one line.
func (ts *turtleState) expandsToBlock(t rdflibgo.Term) bool {
	switch ts.objectForm(t) {
	case formBNode:
		return true
	case formList:
		ts.depth++
		defer func() { ts.depth-- }()
		for _, item := range ts.listItems(t.(rdflibgo.BNode), false) {
			if ts.expandsToBlock(item) {
				return true
			}
		}
	}
	return false
}

// deferNode queues a blank node that was not inlined so that its own statement
// is emitted at the top level. Without this, flattening a node at the nesting
// limit would drop every triple it is the subject of.
func (ts *turtleState) deferNode(bk string) {
	if ts.serialized[bk] || ts.deferredSet[bk] || len(ts.spoMap[bk]) == 0 {
		return
	}
	ts.deferredSet[bk] = true
	ts.deferred = append(ts.deferred, bk)
}

// objectStr returns the Turtle representation of an object term on a single line.
func (ts *turtleState) objectStr(t rdflibgo.Term) (string, error) {
	switch v := t.(type) {
	case rdflibgo.URIRef:
		return ts.qnameOrFull(v), nil
	case rdflibgo.BNode:
		switch ts.objectForm(v) {
		case formList:
			return ts.listStr(v)
		case formBNode:
			return ts.inlineBNode(v)
		default:
			ts.deferNode(termKey(v))
			return v.N3(), nil
		}
	case rdflibgo.Literal:
		return ts.literalStr(v), nil
	case rdflibgo.TripleTerm:
		return ts.tripleTermStr(v)
	default:
		return t.N3(), nil
	}
}

// writeObject writes an object term at the given nesting level, expanding blank
// nodes and collections across lines in the pretty layout.
func (ts *turtleState) writeObject(w io.Writer, t rdflibgo.Term, level int) error {
	if b, ok := t.(rdflibgo.BNode); ok && ts.pretty {
		switch ts.objectForm(b) {
		case formBNode:
			return ts.writeBNodeBlock(w, b, level)
		case formList:
			return ts.writeListBlock(w, b, level)
		}
	}
	s, err := ts.objectStr(t)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s)
	return err
}

// writeBNodeBlock writes a blank node as an indented [ ... ] block. The opening
// bracket sits at the caller's cursor, the closing one at indent(level).
func (ts *turtleState) writeBNodeBlock(w io.Writer, b rdflibgo.BNode, level int) error {
	bk := termKey(b)
	ts.serialized[bk] = true
	ts.depth++
	defer func() { ts.depth-- }()

	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	return ts.writePredicates(w, bk, level+1, "\n"+ts.indent(level)+"]")
}

// writeListBlock writes a collection, keeping it on one line unless an item
// expands into a block of its own.
func (ts *turtleState) writeListBlock(w io.Writer, head rdflibgo.BNode, level int) error {
	ts.depth++
	defer func() { ts.depth-- }()

	items := ts.listItems(head, true)
	if len(items) == 0 {
		_, err := io.WriteString(w, "()")
		return err
	}

	multiline := false
	for _, it := range items {
		if ts.expandsToBlock(it) {
			multiline = true
			break
		}
	}

	open, sep, closing := "( ", " ", " )"
	if multiline {
		open = "(\n" + ts.indent(level+1)
		sep = "\n" + ts.indent(level+1)
		closing = "\n" + ts.indent(level) + ")"
	}
	if _, err := io.WriteString(w, open); err != nil {
		return err
	}
	for i, it := range items {
		if i > 0 {
			if _, err := io.WriteString(w, sep); err != nil {
				return err
			}
		}
		if err := ts.writeObject(w, it, level+1); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, closing)
	return err
}

// literalStr formats a literal for Turtle output.
func (ts *turtleState) literalStr(l rdflibgo.Literal) string {
	n3 := l.N3()
	// If N3 already uses shorthand (integer, boolean, decimal), use it
	if !strings.HasPrefix(n3, "\"") {
		return n3
	}
	// Try to use prefixed datatype
	if l.Language() == "" && l.Datatype() != (rdflibgo.URIRef{}) && l.Datatype() != rdflibgo.XSDString {
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
func (ts *turtleState) qnameOrFull(u rdflibgo.URIRef) string {
	uri := u.Value()
	if s, ok := ts.qnameCache[uri]; ok {
		return s
	}
	s := ts.computeQName(u)
	ts.qnameCache[uri] = s
	return s
}

func (ts *turtleState) computeQName(u rdflibgo.URIRef) string {
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

// isValidPrefixName checks if a string is a valid Turtle PN_PREFIX (prefix label).
// Empty string is valid (default prefix). Otherwise, must match PN_CHARS_BASE
// for the first character, and PN_CHARS (plus dot in middle) for the rest.
func isValidPrefixName(s string) bool {
	if s == "" {
		return true // default prefix
	}
	runes := []rune(s)
	if !isPNCharsBase(runes[0]) {
		return false
	}
	for i := 1; i < len(runes); i++ {
		c := runes[i]
		if isPNCharsBase(c) || c == '_' || c == '-' || c == '\u00B7' ||
			unicode.IsDigit(c) ||
			(c >= '\u0300' && c <= '\u036F') || (c >= '\u203F' && c <= '\u2040') ||
			c == '.' {
			continue
		}
		return false
	}
	// Last character must not be '.'
	if runes[len(runes)-1] == '.' {
		return false
	}
	return true
}

// isValidLocalName checks if a string is valid as a Turtle PN_LOCAL name.
// It uses a positive match aligned with the Turtle grammar specification.
func isValidLocalName(s string) bool {
	if s == "" {
		return false
	}
	runes := []rune(s)
	// First character: must be PN_CHARS_U, digit, ':', or PLX start
	if !isPNCharsU(runes[0]) && !unicode.IsDigit(runes[0]) && runes[0] != ':' {
		return false
	}
	// Middle and last characters
	for i := 1; i < len(runes); i++ {
		c := runes[i]
		if isPNCharsBase(c) || c == '_' || c == '-' || c == '\u00B7' ||
			(c >= '\u0300' && c <= '\u036F') || (c >= '\u203F' && c <= '\u2040') ||
			unicode.IsDigit(c) || c == ':' || c == '.' {
			continue
		}
		return false
	}
	// Last character must not be '.'
	if runes[len(runes)-1] == '.' {
		return false
	}
	return true
}

// tripleTermStr serializes a TripleTerm as <<( s p o )>>.
func (ts *turtleState) tripleTermStr(tt rdflibgo.TripleTerm) (string, error) {
	s := ts.label(tt.Subject())
	pred := ts.qnameOrFull(tt.Predicate())
	o, err := ts.objectStr(tt.Object())
	if err != nil {
		return "", err
	}
	return "<<( " + s + " " + pred + " " + o + " )>>", nil
}

// isPNCharsU returns true if the rune matches PN_CHARS_U (PN_CHARS_BASE | '_').
func isPNCharsU(r rune) bool {
	return r == '_' || isPNCharsBase(r)
}

// isPNCharsBase returns true if the rune matches PN_CHARS_BASE from the Turtle grammar.
func isPNCharsBase(r rune) bool {
	return (r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 0x00C0 && r <= 0x00D6) ||
		(r >= 0x00D8 && r <= 0x00F6) ||
		(r >= 0x00F8 && r <= 0x02FF) ||
		(r >= 0x0370 && r <= 0x037D) ||
		(r >= 0x037F && r <= 0x1FFF) ||
		(r >= 0x200C && r <= 0x200D) ||
		(r >= 0x2070 && r <= 0x218F) ||
		(r >= 0x2C00 && r <= 0x2FEF) ||
		(r >= 0x3001 && r <= 0xD7FF) ||
		(r >= 0xF900 && r <= 0xFDCF) ||
		(r >= 0xFDF0 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0xEFFFF)
}

// listItems walks a well-formed rdf:List and returns its items. With mark set,
// every node of the list is recorded as serialized — pass false to inspect a
// list that is not being written yet. The list is known to be acyclic because
// only cells accepted by detectLists reach here.
func (ts *turtleState) listItems(head rdflibgo.BNode, mark bool) []rdflibgo.Term {
	var items []rdflibgo.Term

	node := termKey(head)
	for node != ts.nilKey {
		if mark {
			ts.serialized[node] = true
		}
		if firsts := ts.spoMap[node][ts.firstKey]; len(firsts) > 0 {
			items = append(items, firsts[0])
		}
		rests := ts.spoMap[node][ts.restKey]
		if len(rests) == 0 {
			break
		}
		node = termKey(rests[0])
	}
	return items
}

// listStr serializes an rdf:List as Turtle collection syntax: ( item1 item2 ... )
func (ts *turtleState) listStr(head rdflibgo.BNode) (string, error) {
	ts.depth++
	defer func() { ts.depth-- }()

	items := ts.listItems(head, true)

	var b strings.Builder
	b.WriteString("( ")
	for _, item := range items {
		str, err := ts.objectStr(item)
		if err != nil {
			return "", err
		}
		b.WriteString(str)
		b.WriteByte(' ')
	}
	b.WriteByte(')')
	return b.String(), nil
}

// inlineBNode serializes a blank node inline: [ pred1 obj1 ; pred2 obj2 ]
func (ts *turtleState) inlineBNode(b rdflibgo.BNode) (string, error) {
	sk := termKey(b)
	ts.serialized[sk] = true
	ts.depth++
	defer func() { ts.depth-- }()

	preds := ts.spoMap[sk]

	var out strings.Builder
	out.WriteString("[ ")
	for i, pk := range ts.sortPredicates(preds) {
		if i > 0 {
			out.WriteString(" ; ")
		}
		out.WriteString(ts.predLabel(pk))

		objs := preds[pk]
		slices.SortFunc(objs, func(a, b rdflibgo.Term) int {
			return rdflibgo.CompareTerm(a, b)
		})
		for j, obj := range objs {
			if j > 0 {
				out.WriteByte(',')
			}
			str, err := ts.objectStr(obj)
			if err != nil {
				return "", err
			}
			out.WriteByte(' ')
			out.WriteString(str)
		}
	}
	out.WriteString(" ]")
	return out.String(), nil
}
