package rdfxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

func termKey(t rdflibgo.Term) string { return t.N3() }

// Serialize serializes a Graph to RDF/XML format.
// It groups triples by subject and emits typed node elements when rdf:type is present.
// If a subject has multiple rdf:type values, the first one (in sorted order) with a
// valid QName becomes the element name; the rest are emitted as rdf:type property elements.
//
// Every predicate needs an XML qualified name. Namespaces bound on the graph are
// used when they leave a valid local name; otherwise the IRI is split before its
// trailing NCName and the namespace gets a generated prefix (ns1, ns2, ...). The
// prefix rdf always names the RDF namespace, so a user binding of rdf to another
// namespace is renamed. A predicate with no QName makes Serialize fail with
// ErrNoQName, and an RDF/XML syntax name used as a predicate with
// ErrReservedPropertyName. Nothing is written to w when Serialize fails.
//
// Options: WithBase sets xml:base on the root element.
func Serialize(g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}

	// Group triples by subject
	subjects := make(map[string][]rdflibgo.Triple)
	var subjectOrder []string
	labels := make(map[string]bool)
	g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
		sk := termKey(t.Subject)
		if _, exists := subjects[sk]; !exists {
			subjectOrder = append(subjectOrder, sk)
		}
		subjects[sk] = append(subjects[sk], t)
		collectLabels(t.Subject, labels)
		collectLabels(t.Object, labels)
		return true
	})
	slices.Sort(subjectOrder)

	bindings := make(map[string]string)
	g.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
		bindings[prefix] = ns.Value()
		return true
	})
	sw := &xmlWriter{
		ns:        newNSTable(bindings, map[string]string{"rdf": rdfNS}),
		labels:    labels,
		relabeled: make(map[string]string),
	}

	// The body is rendered first: it decides which namespaces need a generated
	// prefix, and a failure leaves w untouched instead of half-written.
	for _, sk := range subjectOrder {
		if err := sw.writeSubject(subjects[sk]); err != nil {
			return err
		}
	}

	var out bytes.Buffer
	out.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<rdf:RDF")
	for _, ns := range sw.ns.declarations() {
		fmt.Fprintf(&out, "\n   xmlns:%s=%s", sw.ns.prefixOf[ns], xmlAttr(ns))
	}
	if cfg.base != "" {
		fmt.Fprintf(&out, "\n   xml:base=%s", xmlAttr(cfg.base))
	}
	out.WriteString(">\n")
	out.Write(sw.body.Bytes())
	out.WriteString("</rdf:RDF>\n")
	_, err := w.Write(out.Bytes())
	return err
}

// xmlWriter renders the RDF/XML body. It is not safe for concurrent use.
type xmlWriter struct {
	body      bytes.Buffer
	ns        *nsTable
	labels    map[string]bool   // every blank node label in the graph
	relabeled map[string]string // label -> NCName label used in rdf:nodeID
	nextLabel int
}

func (sw *xmlWriter) writeSubject(triples []rdflibgo.Triple) error {
	if len(triples) == 0 {
		return nil
	}
	subj := triples[0].Subject

	// Sort for determinism: the element name is the first usable rdf:type.
	slices.SortFunc(triples, func(a, b rdflibgo.Triple) int {
		return strings.Compare(a.Predicate.N3()+a.Object.N3(), b.Predicate.N3()+b.Object.N3())
	})

	elemName := "rdf:Description"
	remaining := make([]rdflibgo.Triple, 0, len(triples))
	for _, t := range triples {
		if elemName == "rdf:Description" && t.Predicate == rdflibgo.RDF.Type {
			if u, ok := t.Object.(rdflibgo.URIRef); ok && !forbiddenNodeElementNames[u.Value()] {
				if qn := sw.ns.qname(u.Value()); qn != "" {
					elemName = qn
					continue
				}
			}
		}
		remaining = append(remaining, t)
	}

	subjAttr, err := sw.nodeAttr(subj)
	if err != nil {
		return err
	}
	fmt.Fprintf(&sw.body, "  <%s %s>\n", elemName, subjAttr)
	for _, t := range remaining {
		if err := sw.writeProperty("    ", t.Predicate, t.Object); err != nil {
			return err
		}
	}
	fmt.Fprintf(&sw.body, "  </%s>\n", elemName)
	return nil
}

// nodeAttr returns the rdf:about or rdf:nodeID attribute naming a node.
func (sw *xmlWriter) nodeAttr(n rdflibgo.Term) (string, error) {
	switch v := n.(type) {
	case rdflibgo.URIRef:
		return "rdf:about=" + xmlAttr(v.Value()), nil
	case rdflibgo.BNode:
		return "rdf:nodeID=" + xmlAttr(sw.nodeID(v)), nil
	default:
		return "", fmt.Errorf("rdfxml: cannot write %T %s as a node element", n, n.N3())
	}
}

func (sw *xmlWriter) writeProperty(indent string, pred rdflibgo.URIRef, obj rdflibgo.Term) error {
	if forbiddenPropertyElementNames[pred.Value()] || pred.Value() == rdfNS+"li" {
		return fmt.Errorf("%w: <%s> cannot be written as a property element (rdf:li would be renumbered on reading); "+
			"serialize this graph as Turtle, N-Triples or JSON-LD instead", ErrReservedPropertyName, pred.Value())
	}
	predQN := sw.ns.qname(pred.Value())
	if predQN == "" {
		return noQNameError(pred.Value())
	}

	b := &sw.body
	switch o := obj.(type) {
	case rdflibgo.URIRef:
		fmt.Fprintf(b, "%s<%s rdf:resource=%s/>\n", indent, predQN, xmlAttr(o.Value()))
	case rdflibgo.BNode:
		fmt.Fprintf(b, "%s<%s rdf:nodeID=%s/>\n", indent, predQN, xmlAttr(sw.nodeID(o)))
	case rdflibgo.Literal:
		if o.Language() != "" {
			fmt.Fprintf(b, "%s<%s xml:lang=%s>%s</%s>\n", indent, predQN, xmlAttr(o.Language()), xmlEscape(o.Lexical()), predQN)
		} else if o.Datatype() != (rdflibgo.URIRef{}) && o.Datatype() != rdflibgo.XSDString {
			fmt.Fprintf(b, "%s<%s rdf:datatype=%s>%s</%s>\n", indent, predQN, xmlAttr(o.Datatype().Value()), xmlEscape(o.Lexical()), predQN)
		} else {
			fmt.Fprintf(b, "%s<%s>%s</%s>\n", indent, predQN, xmlEscape(o.Lexical()), predQN)
		}
	}
	return nil
}

// nodeID returns an rdf:nodeID value for b. RDF/XML requires an NCName, which
// labels from N-Triples or Turtle need not be (_:1a is legal there); those are
// replaced by generated labels that collide with no label in the graph.
func (sw *xmlWriter) nodeID(b rdflibgo.BNode) string {
	label := b.Value()
	if isValidNCName(label) {
		return label
	}
	if l, ok := sw.relabeled[label]; ok {
		return l
	}
	for {
		sw.nextLabel++
		l := "genid" + strconv.Itoa(sw.nextLabel)
		if !sw.labels[l] {
			sw.labels[l] = true
			sw.relabeled[label] = l
			return l
		}
	}
}

func collectLabels(t rdflibgo.Term, labels map[string]bool) {
	switch v := t.(type) {
	case rdflibgo.BNode:
		labels[v.Value()] = true
	case rdflibgo.TripleTerm:
		collectLabels(v.Subject(), labels)
		collectLabels(v.Object(), labels)
	}
}

// xmlAttr returns an XML-escaped, double-quoted attribute value.
func xmlAttr(s string) string {
	var b strings.Builder
	// xml.EscapeText writing to strings.Builder cannot fail, so we ignore the error.
	_ = xml.EscapeText(&b, []byte(s))
	return `"` + b.String() + `"`
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
