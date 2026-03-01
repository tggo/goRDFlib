package rdflibgo

import (
	"encoding/xml"
	"fmt"
	"io"
	"slices"
	"strings"
)

// RDFXMLSerializer serializes a Graph to RDF/XML format.
// Ported from: rdflib.plugins.serializers.rdfxml.XMLSerializer
type RDFXMLSerializer struct{}

func init() {
	RegisterSerializer("xml", func() Serializer { return &RDFXMLSerializer{} })
	RegisterSerializer("rdf/xml", func() Serializer { return &RDFXMLSerializer{} })
	RegisterSerializer("application/rdf+xml", func() Serializer { return &RDFXMLSerializer{} })
}

func (s *RDFXMLSerializer) Serialize(g *Graph, w io.Writer, base string) error {
	// Collect namespace prefixes
	nsMap := make(map[string]string) // namespace → prefix
	nsMap[rdfNS] = "rdf"
	g.Namespaces()(func(prefix string, ns URIRef) bool {
		nsMap[ns.Value()] = prefix
		return true
	})

	// Group triples by subject
	subjects := make(map[string][]Triple)
	var subjectOrder []string
	g.Triples(nil, nil, nil)(func(t Triple) bool {
		sk := termKey(t.Subject)
		if _, exists := subjects[sk]; !exists {
			subjectOrder = append(subjectOrder, sk)
		}
		subjects[sk] = append(subjects[sk], t)
		return true
	})
	slices.Sort(subjectOrder)

	// Write XML header
	if _, err := fmt.Fprintln(w, `<?xml version="1.0" encoding="utf-8"?>`); err != nil {
		return err
	}

	// rdf:RDF opening with namespace declarations
	if _, err := fmt.Fprintf(w, "<rdf:RDF"); err != nil {
		return err
	}
	// Sort namespaces for determinism
	var nsList []string
	for ns := range nsMap {
		nsList = append(nsList, ns)
	}
	slices.Sort(nsList)
	for _, ns := range nsList {
		prefix := nsMap[ns]
		if _, err := fmt.Fprintf(w, "\n   xmlns:%s=%s", prefix, xmlAttr(ns)); err != nil {
			return err
		}
	}
	if base != "" {
		if _, err := fmt.Fprintf(w, "\n   xml:base=%s", xmlAttr(base)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, ">"); err != nil {
		return err
	}

	// Write each subject as rdf:Description
	for _, sk := range subjectOrder {
		triples := subjects[sk]
		if len(triples) == 0 {
			continue
		}
		subj := triples[0].Subject

		// Determine element type: if there's rdf:type, use typed node
		elemName := "rdf:Description"
		var remaining []Triple
		for _, t := range triples {
			if t.Predicate == RDF.Type {
				if u, ok := t.Object.(URIRef); ok {
					qn := xmlQName(u.Value(), nsMap)
					if qn != "" {
						elemName = qn
						continue
					}
				}
			}
			remaining = append(remaining, t)
		}

		// Sort remaining triples for determinism
		slices.SortFunc(remaining, func(a, b Triple) int {
			return strings.Compare(a.Predicate.N3()+a.Object.N3(), b.Predicate.N3()+b.Object.N3())
		})

		// Opening tag
		switch v := subj.(type) {
		case URIRef:
			if _, err := fmt.Fprintf(w, "  <%s rdf:about=%s>\n", elemName, xmlAttr(v.Value())); err != nil {
				return err
			}
		case BNode:
			if _, err := fmt.Fprintf(w, "  <%s rdf:nodeID=%s>\n", elemName, xmlAttr(v.Value())); err != nil {
				return err
			}
		}

		// Property elements
		for _, t := range remaining {
			predQN := xmlQName(t.Predicate.Value(), nsMap)
			if predQN == "" {
				predQN = t.Predicate.Value()
			}

			var err error
			switch obj := t.Object.(type) {
			case URIRef:
				_, err = fmt.Fprintf(w, "    <%s rdf:resource=%s/>\n", predQN, xmlAttr(obj.Value()))
			case BNode:
				_, err = fmt.Fprintf(w, "    <%s rdf:nodeID=%s/>\n", predQN, xmlAttr(obj.Value()))
			case Literal:
				if obj.Language() != "" {
					_, err = fmt.Fprintf(w, "    <%s xml:lang=%s>%s</%s>\n", predQN, xmlAttr(obj.Language()), xmlEscape(obj.Lexical()), predQN)
				} else if obj.Datatype() != (URIRef{}) && obj.Datatype() != XSDString {
					_, err = fmt.Fprintf(w, "    <%s rdf:datatype=%s>%s</%s>\n", predQN, xmlAttr(obj.Datatype().Value()), xmlEscape(obj.Lexical()), predQN)
				} else {
					_, err = fmt.Fprintf(w, "    <%s>%s</%s>\n", predQN, xmlEscape(obj.Lexical()), predQN)
				}
			}
			if err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(w, "  </%s>\n", elemName); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(w, "</rdf:RDF>")
	return err
}

// xmlQName converts a full URI to a prefixed XML name using known namespaces.
func xmlQName(uri string, nsMap map[string]string) string {
	bestNS := ""
	bestPrefix := ""
	for ns, prefix := range nsMap {
		if strings.HasPrefix(uri, ns) && len(ns) > len(bestNS) {
			local := uri[len(ns):]
			if local != "" && !strings.ContainsAny(local, "/#") {
				bestNS = ns
				bestPrefix = prefix
			}
		}
	}
	if bestNS != "" {
		return bestPrefix + ":" + uri[len(bestNS):]
	}
	return ""
}

func xmlAttr(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return `"` + s + `"`
	}
	return `"` + b.String() + `"`
}

func xmlEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
