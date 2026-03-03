package trig

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// Serialize writes the graph in TriG format (single graph, no wrapping block).
func Serialize(g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	ds := graph.NewDataset()
	defG := ds.DefaultContext()
	g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
		defG.Add(t.Subject, t.Predicate, t.Object)
		return true
	})
	g.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
		ds.Bind(prefix, ns)
		return true
	})
	return SerializeDataset(ds, w, opts...)
}

// SerializeDataset writes the dataset in TriG format.
func SerializeDataset(ds *graph.Dataset, w io.Writer, opts ...Option) error {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	bw := bufio.NewWriter(w)
	defer bw.Flush()

	// Collect all used prefixes across all graphs
	usedNS := make(map[string]rdflibgo.URIRef)
	var allGraphs []*graph.Graph
	defaultCtx := ds.DefaultContext()

	for g := range ds.Graphs() {
		allGraphs = append(allGraphs, g)
		g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
			trackNSForTerm(t.Subject, ds, usedNS)
			trackNSForTerm(t.Predicate, ds, usedNS)
			trackNSForTerm(t.Object, ds, usedNS)
			return true
		})
	}

	// @base
	if cfg.base != "" {
		fmt.Fprintf(bw, "@base <%s> .\n", cfg.base)
	}

	// @prefix declarations
	var prefixes []string
	for p := range usedNS {
		prefixes = append(prefixes, p)
	}
	slices.Sort(prefixes)
	for _, p := range prefixes {
		fmt.Fprintf(bw, "@prefix %s: <%s> .\n", p, usedNS[p].Value())
	}
	if len(prefixes) > 0 || cfg.base != "" {
		fmt.Fprintln(bw)
	}

	// Sort graphs: default first, then named graphs sorted by identifier
	slices.SortFunc(allGraphs, func(a, b *graph.Graph) int {
		aID := a.Identifier()
		bID := b.Identifier()
		aIsDefault := aID == defaultCtx.Identifier()
		bIsDefault := bID == defaultCtx.Identifier()
		if aIsDefault && !bIsDefault {
			return -1
		}
		if !aIsDefault && bIsDefault {
			return 1
		}
		return strings.Compare(aID.N3(), bID.N3())
	})

	first := true
	for _, g := range allGraphs {
		if g.Len() == 0 {
			continue
		}

		isDefault := g.Identifier() == defaultCtx.Identifier()

		if !first {
			fmt.Fprintln(bw)
		}
		first = false

		ts := newTrigState(g, usedNS)
		ts.preprocess()
		ts.orderSubjects()

		if isDefault {
			// Default graph: emit triples in { } block
			fmt.Fprintln(bw, "{")
			ts.writeIndented(bw, "    ")
			fmt.Fprintln(bw, "}")
		} else {
			// Named graph
			label := trigLabel(g.Identifier(), usedNS)
			fmt.Fprintf(bw, "%s {\n", label)
			ts.writeIndented(bw, "    ")
			fmt.Fprintln(bw, "}")
		}
	}

	return nil
}

func trigLabel(t rdflibgo.Term, usedNS map[string]rdflibgo.URIRef) string {
	if u, ok := t.(rdflibgo.URIRef); ok {
		return qnameOrFull(u, usedNS)
	}
	return t.N3()
}

// trigState holds serialization state for a single graph.
type trigState struct {
	g      *graph.Graph
	usedNS map[string]rdflibgo.URIRef

	spoMap     map[string]map[string][]rdflibgo.Term
	subjects   []rdflibgo.Subject
	refs       map[string]int
	listHeads  map[string]bool
	listNodes  map[string]bool
	serialized map[string]bool
	subjectMap map[string]rdflibgo.Subject
}

func newTrigState(g *graph.Graph, usedNS map[string]rdflibgo.URIRef) *trigState {
	return &trigState{
		g:          g,
		usedNS:     usedNS,
		spoMap:     make(map[string]map[string][]rdflibgo.Term),
		refs:       make(map[string]int),
		listHeads:  make(map[string]bool),
		listNodes:  make(map[string]bool),
		serialized: make(map[string]bool),
		subjectMap: make(map[string]rdflibgo.Subject),
	}
}

func (ts *trigState) preprocess() {
	ts.g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
		sk := t.Subject.N3()
		pk := t.Predicate.N3()

		if ts.spoMap[sk] == nil {
			ts.spoMap[sk] = make(map[string][]rdflibgo.Term)
		}
		ts.subjectMap[sk] = t.Subject
		ts.spoMap[sk][pk] = append(ts.spoMap[sk][pk], t.Object)
		ts.refs[t.Object.N3()]++
		return true
	})
	ts.detectLists()
}

func (ts *trigState) detectLists() {
	firstKey := rdflibgo.RDF.First.N3()
	restKey := rdflibgo.RDF.Rest.N3()
	nilKey := rdflibgo.RDF.Nil.N3()

	for sk, preds := range ts.spoMap {
		if _, hasFirst := preds[firstKey]; !hasFirst {
			continue
		}
		if ts.isValidList(sk, firstKey, restKey, nilKey) {
			ts.listHeads[sk] = true
			ts.markListNodes(sk, restKey, nilKey)
		}
	}
}

func (ts *trigState) isValidList(sk, firstKey, restKey, nilKey string) bool {
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
		node = rests[0].N3()
	}
	return true
}

func (ts *trigState) markListNodes(sk, restKey, nilKey string) {
	node := sk
	for node != nilKey {
		ts.listNodes[node] = true
		rests := ts.spoMap[node][restKey]
		if len(rests) == 0 {
			break
		}
		node = rests[0].N3()
	}
}

func (ts *trigState) orderSubjects() {
	typeKey := rdflibgo.RDF.Type.N3()
	classKey := rdflibgo.RDFS.Class.N3()

	var topSubjects []rdflibgo.Subject
	var bnodeSubjects []rdflibgo.Subject
	var otherSubjects []rdflibgo.Subject

	for sk := range ts.spoMap {
		if ts.listNodes[sk] {
			continue
		}
		subj := ts.subjectMap[sk]
		if subj == nil {
			continue
		}
		if preds, ok := ts.spoMap[sk]; ok {
			if objs, ok := preds[typeKey]; ok {
				for _, o := range objs {
					if o.N3() == classKey {
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
	slices.SortFunc(bnodeSubjects, func(a, b rdflibgo.Subject) int {
		ra, rb := ts.refs[a.N3()], ts.refs[b.N3()]
		if ra != rb {
			return ra - rb
		}
		return strings.Compare(a.N3(), b.N3())
	})

	ts.subjects = append(ts.subjects, topSubjects...)
	ts.subjects = append(ts.subjects, otherSubjects...)
	ts.subjects = append(ts.subjects, bnodeSubjects...)
}

func (ts *trigState) writeIndented(w io.Writer, indent string) error {
	for i, subj := range ts.subjects {
		sk := subj.N3()
		if ts.serialized[sk] {
			continue
		}
		if err := ts.writeSubject(w, subj, indent); err != nil {
			return err
		}
		if i < len(ts.subjects)-1 {
			fmt.Fprintln(w)
		}
	}
	return nil
}

func (ts *trigState) writeSubject(w io.Writer, subj rdflibgo.Subject, indent string) error {
	sk := subj.N3()
	ts.serialized[sk] = true

	if _, isBNode := subj.(rdflibgo.BNode); isBNode && ts.refs[sk] == 0 && !ts.listHeads[sk] {
		fmt.Fprintf(w, "%s[]", indent)
		return ts.writePredicates(w, sk, indent)
	}

	label := ts.label(subj)
	fmt.Fprintf(w, "%s%s", indent, label)
	return ts.writePredicates(w, sk, indent)
}

func (ts *trigState) writePredicates(w io.Writer, sk string, indent string) error {
	preds := ts.spoMap[sk]
	if len(preds) == 0 {
		_, err := fmt.Fprintln(w, " .")
		return err
	}

	sortedPreds := ts.sortPredicates(preds)

	for i, pk := range sortedPreds {
		objs := preds[pk]
		predLabel := ts.predLabel(pk)

		if i == 0 {
			fmt.Fprintf(w, " %s", predLabel)
		} else {
			fmt.Fprintf(w, " ;\n%s    %s", indent, predLabel)
		}

		slices.SortFunc(objs, func(a, b rdflibgo.Term) int {
			return rdflibgo.CompareTerm(a, b)
		})

		for j, obj := range objs {
			if j > 0 {
				fmt.Fprintf(w, ",")
			}
			objStr, err := ts.objectStr(obj)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, " %s", objStr)
		}
	}

	_, err := fmt.Fprintln(w, " .")
	return err
}

func (ts *trigState) sortPredicates(preds map[string][]rdflibgo.Term) []string {
	typeKey := rdflibgo.RDF.Type.N3()
	labelKey := rdflibgo.RDFS.Label.N3()

	var ordered []string
	var rest []string

	for pk := range preds {
		switch pk {
		case typeKey, labelKey:
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

func (ts *trigState) label(t rdflibgo.Term) string {
	if u, ok := t.(rdflibgo.URIRef); ok {
		return qnameOrFull(u, ts.usedNS)
	}
	return t.N3()
}

func (ts *trigState) predLabel(pk string) string {
	if pk == rdflibgo.RDF.Type.N3() {
		return "a"
	}
	uri := strings.TrimPrefix(strings.TrimSuffix(pk, ">"), "<")
	u := rdflibgo.NewURIRefUnsafe(uri)
	return qnameOrFull(u, ts.usedNS)
}

func (ts *trigState) objectStr(t rdflibgo.Term) (string, error) {
	switch v := t.(type) {
	case rdflibgo.URIRef:
		return qnameOrFull(v, ts.usedNS), nil
	case rdflibgo.BNode:
		bk := v.N3()
		if ts.listHeads[bk] && !ts.serialized[bk] {
			return ts.listStr(v)
		}
		if ts.refs[bk] <= 1 && !ts.serialized[bk] {
			if preds := ts.spoMap[bk]; len(preds) > 0 {
				return ts.inlineBNode(v)
			}
		}
		return v.N3(), nil
	case rdflibgo.Literal:
		return ts.literalStr(v), nil
	case rdflibgo.TripleTerm:
		return ts.tripleTermStr(v)
	default:
		return t.N3(), nil
	}
}

func (ts *trigState) literalStr(l rdflibgo.Literal) string {
	n3 := l.N3()
	if !strings.HasPrefix(n3, "\"") {
		return n3
	}
	if l.Language() == "" && l.Datatype() != (rdflibgo.URIRef{}) && l.Datatype() != rdflibgo.XSDString {
		dtN3 := l.Datatype().N3()
		dtQName := qnameOrFull(l.Datatype(), ts.usedNS)
		if dtQName != dtN3 {
			return strings.Replace(n3, "^^"+dtN3, "^^"+dtQName, 1)
		}
	}
	return n3
}

func (ts *trigState) tripleTermStr(tt rdflibgo.TripleTerm) (string, error) {
	s := ts.label(tt.Subject())
	pred := qnameOrFull(tt.Predicate(), ts.usedNS)
	o, err := ts.objectStr(tt.Object())
	if err != nil {
		return "", err
	}
	return "<<( " + s + " " + pred + " " + o + " )>>", nil
}

func (ts *trigState) listStr(head rdflibgo.BNode) (string, error) {
	var items []string
	restKey := rdflibgo.RDF.Rest.N3()
	firstKey := rdflibgo.RDF.First.N3()
	nilKey := rdflibgo.RDF.Nil.N3()

	node := head.N3()
	for node != nilKey {
		ts.serialized[node] = true
		firsts := ts.spoMap[node][firstKey]
		if len(firsts) > 0 {
			str, err := ts.objectStr(firsts[0])
			if err != nil {
				return "", err
			}
			items = append(items, str)
		}
		rests := ts.spoMap[node][restKey]
		if len(rests) == 0 {
			break
		}
		node = rests[0].N3()
	}
	return "( " + strings.Join(items, " ") + " )", nil
}

func (ts *trigState) inlineBNode(b rdflibgo.BNode) (string, error) {
	sk := b.N3()
	ts.serialized[sk] = true
	preds := ts.spoMap[sk]

	sortedPreds := ts.sortPredicates(preds)
	var parts []string

	for _, pk := range sortedPreds {
		objs := preds[pk]
		predLabel := ts.predLabel(pk)

		slices.SortFunc(objs, func(a, b rdflibgo.Term) int {
			return rdflibgo.CompareTerm(a, b)
		})

		var objStrs []string
		for _, obj := range objs {
			str, err := ts.objectStr(obj)
			if err != nil {
				return "", err
			}
			objStrs = append(objStrs, str)
		}
		parts = append(parts, predLabel+" "+strings.Join(objStrs, ", "))
	}

	return "[ " + strings.Join(parts, " ; ") + " ]", nil
}

// qnameOrFull returns a prefixed name if possible, otherwise the full N3 form.
func qnameOrFull(u rdflibgo.URIRef, usedNS map[string]rdflibgo.URIRef) string {
	uri := u.Value()
	bestPrefix := ""
	bestNS := ""

	for p, ns := range usedNS {
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
		if isValidLocalName(local) {
			return bestPrefix + ":" + local
		}
	}
	return u.N3()
}

func trackNSForTerm(t rdflibgo.Term, ds *graph.Dataset, usedNS map[string]rdflibgo.URIRef) {
	if tt, ok := t.(rdflibgo.TripleTerm); ok {
		trackNSForTerm(tt.Subject(), ds, usedNS)
		trackNSForTerm(tt.Predicate(), ds, usedNS)
		trackNSForTerm(tt.Object(), ds, usedNS)
		return
	}
	u, ok := t.(rdflibgo.URIRef)
	if !ok {
		return
	}
	uri := u.Value()
	ds.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
		nsStr := ns.Value()
		if strings.HasPrefix(uri, nsStr) && len(uri) > len(nsStr) && isValidPrefixName(prefix) {
			usedNS[prefix] = ns
		}
		return true
	})
}

func isValidPrefixName(s string) bool {
	if s == "" {
		return true
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
	if runes[len(runes)-1] == '.' {
		return false
	}
	return true
}

func isValidLocalName(s string) bool {
	if s == "" {
		return false
	}
	runes := []rune(s)
	if !isPNCharsU(runes[0]) && !unicode.IsDigit(runes[0]) && runes[0] != ':' {
		return false
	}
	for i := 1; i < len(runes); i++ {
		c := runes[i]
		if isPNCharsBase(c) || c == '_' || c == '-' || c == '\u00B7' ||
			(c >= '\u0300' && c <= '\u036F') || (c >= '\u203F' && c <= '\u2040') ||
			unicode.IsDigit(c) || c == ':' || c == '.' {
			continue
		}
		return false
	}
	if runes[len(runes)-1] == '.' {
		return false
	}
	return true
}
