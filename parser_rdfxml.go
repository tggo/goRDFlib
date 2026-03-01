package rdflibgo

import (
	"encoding/xml"
	"io"
	"strings"
)

// RDFXMLParser parses RDF/XML format.
// Ported from: rdflib.plugins.parsers.rdfxml.RDFXMLParser
type RDFXMLParser struct{}

func init() {
	RegisterParser("xml", func() Parser { return &RDFXMLParser{} })
	RegisterParser("rdf/xml", func() Parser { return &RDFXMLParser{} })
	RegisterParser("application/rdf+xml", func() Parser { return &RDFXMLParser{} })
}

const rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"

func (p *RDFXMLParser) Parse(g *Graph, r io.Reader, base string) error {
	parser := &rdfxmlParser{
		g:    g,
		base: base,
		bnodeMap: make(map[string]BNode),
	}
	return parser.parse(r)
}

type rdfxmlParser struct {
	g        *Graph
	base     string
	lang     string // inherited xml:lang
	bnodeMap map[string]BNode
}

func (p *rdfxmlParser) parse(r io.Reader) error {
	decoder := xml.NewDecoder(r)
	// Find rdf:RDF root (or process document element directly)
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if se, ok := tok.(xml.StartElement); ok {
			name := se.Name.Space + se.Name.Local
			if name == rdfNS+"RDF" {
				return p.parseRDFRoot(decoder, se)
			}
			// Document element is the subject itself (no rdf:RDF wrapper)
			return p.parseNodeElement(decoder, se, "")
		}
	}
}

func (p *rdfxmlParser) parseRDFRoot(decoder *xml.Decoder, root xml.StartElement) error {
	// Check for xml:base
	for _, attr := range root.Attr {
		if isXMLAttr(attr, "base") {
			p.base = attr.Value
		}
	}

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := p.parseNodeElement(decoder, t, ""); err != nil {
				return err
			}
		case xml.EndElement:
			return nil // end of rdf:RDF
		}
	}
}

// parseNodeElement handles rdf:Description or typed node elements.
// If preSubj is non-nil, it is used as the subject instead of deriving one from attributes.
// Ported from: rdflib.plugins.parsers.rdfxml — node element processing
func (p *rdfxmlParser) parseNodeElement(decoder *xml.Decoder, el xml.StartElement, parentLang string, preSubj ...Subject) error {
	elemURI := el.Name.Space + el.Name.Local

	var subj Subject
	if len(preSubj) > 0 && preSubj[0] != nil {
		subj = preSubj[0]
	}
	lang := parentLang

	// Save base for scoped xml:base — restore on exit
	savedBase := p.base

	for _, attr := range el.Attr {
		switch {
		case isXMLAttr(attr, "lang"):
			lang = attr.Value
		case isXMLAttr(attr, "base"):
			p.base = attr.Value // scoped — restored via defer below
		case subj == nil && (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "about":
			subj = NewURIRefUnsafe(p.resolve(attr.Value))
		case subj == nil && (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "ID":
			subj = NewURIRefUnsafe(p.resolve("#" + attr.Value))
		case subj == nil && (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "nodeID":
			subj = p.getBNode(attr.Value)
		}
	}

	if subj == nil {
		subj = NewBNode()
	}

	// Restore xml:base on exit (scoped per element)
	defer func() { p.base = savedBase }()

	// Typed node (not rdf:Description)
	if elemURI != rdfNS+"Description" {
		p.g.Add(subj, RDF.Type, NewURIRefUnsafe(elemURI))
	}

	// Process property attributes
	for _, attr := range el.Attr {
		if attr.Name.Space == "xml" || attr.Name.Space == "xmlns" {
			continue
		}
		attrURI := attr.Name.Space + attr.Name.Local
		if attrURI == rdfNS+"about" || attrURI == rdfNS+"ID" || attrURI == rdfNS+"nodeID" || attrURI == rdfNS+"type" {
			if attrURI == rdfNS+"type" {
				p.g.Add(subj, RDF.Type, NewURIRefUnsafe(p.resolve(attr.Value)))
			}
			continue
		}
		if attr.Name.Space == "" {
			continue // skip unprefixed attributes
		}
		// Property attribute → literal value
		var opts []LiteralOption
		if lang != "" {
			opts = append(opts, WithLang(lang))
		}
		p.g.Add(subj, NewURIRefUnsafe(attrURI), NewLiteral(attr.Value, opts...))
	}

	// Process child elements (property elements)
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := p.parsePropertyElement(decoder, t, subj, lang); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// parsePropertyElement handles predicate elements inside a node element.
// Ported from: rdflib.plugins.parsers.rdfxml — property element processing
func (p *rdfxmlParser) parsePropertyElement(decoder *xml.Decoder, el xml.StartElement, subj Subject, parentLang string) error {
	predURI := el.Name.Space + el.Name.Local

	// Handle rdf:li → rdf:_1, _2, etc. (skipped for simplicity)

	pred := NewURIRefUnsafe(predURI)
	lang := parentLang

	var resource, nodeID, parseType, datatype string

	for _, attr := range el.Attr {
		switch {
		case isXMLAttr(attr, "lang"):
			lang = attr.Value
		case (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "resource":
			resource = attr.Value
		case (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "nodeID":
			nodeID = attr.Value
		case (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "parseType":
			parseType = attr.Value
		case (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "datatype":
			datatype = attr.Value
		}
	}

	// rdf:resource attribute → object is a URI
	if resource != "" {
		p.g.Add(subj, pred, NewURIRefUnsafe(p.resolve(resource)))
		decoder.Skip()
		return nil
	}

	// rdf:nodeID → object is a blank node
	if nodeID != "" {
		p.g.Add(subj, pred, p.getBNode(nodeID))
		decoder.Skip()
		return nil
	}

	// rdf:parseType="Resource" → inline blank node
	if parseType == "Resource" {
		bnode := NewBNode()
		p.g.Add(subj, pred, bnode)
		// Parse child elements as properties of the blank node
		for {
			tok, err := decoder.Token()
			if err != nil {
				return err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if err := p.parsePropertyElement(decoder, t, bnode, lang); err != nil {
					return err
				}
			case xml.EndElement:
				return nil
			}
		}
	}

	// rdf:parseType="Collection" → rdf:List
	if parseType == "Collection" {
		return p.parseCollection(decoder, subj, pred, lang)
	}

	// rdf:parseType="Literal" → XML literal
	if parseType == "Literal" {
		content, err := readInnerXML(decoder)
		if err != nil {
			return err
		}
		lit := NewLiteral(content, WithDatatype(NewURIRefUnsafe(rdfNS+"XMLLiteral")))
		p.g.Add(subj, pred, lit)
		return nil
	}

	// Try reading content: either text or nested node element
	var textContent strings.Builder
	hasChild := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			textContent.Write(t)
		case xml.StartElement:
			hasChild = true
			// Extract subject before parsing so both use the same node
			childSubj := p.extractSubject(t)
			if err := p.parseNodeElement(decoder, t, lang, childSubj); err != nil {
				return err
			}
			p.g.Add(subj, pred, childSubj)
			// Skip to closing tag of property element
			skipToEnd(decoder)
			return nil
		case xml.EndElement:
			if hasChild {
				return nil
			}
			// Text content → literal
			text := textContent.String()
			var opts []LiteralOption
			if lang != "" {
				opts = append(opts, WithLang(lang))
			}
			if datatype != "" {
				opts = append(opts, WithDatatype(NewURIRefUnsafe(p.resolve(datatype))))
			}
			p.g.Add(subj, pred, NewLiteral(text, opts...))
			return nil
		}
	}
}

func (p *rdfxmlParser) parseCollection(decoder *xml.Decoder, subj Subject, pred URIRef, lang string) error {
	var items []Subject
	for {
		tok, err := decoder.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			childSubj := p.extractSubject(t)
			if err := p.parseNodeElement(decoder, t, lang, childSubj); err != nil {
				return err
			}
			items = append(items, childSubj)
		case xml.EndElement:
			// Build rdf:List
			if len(items) == 0 {
				p.g.Add(subj, pred, RDF.Nil)
				return nil
			}
			head := NewBNode()
			p.g.Add(subj, pred, head)
			current := head
			for i, item := range items {
				p.g.Add(current, RDF.First, item)
				if i < len(items)-1 {
					next := NewBNode()
					p.g.Add(current, RDF.Rest, next)
					current = next
				} else {
					p.g.Add(current, RDF.Rest, RDF.Nil)
				}
			}
			return nil
		}
	}
}

func (p *rdfxmlParser) extractSubject(el xml.StartElement) Subject {
	for _, attr := range el.Attr {
		if (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "about" {
			return NewURIRefUnsafe(p.resolve(attr.Value))
		}
		if (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "ID" {
			return NewURIRefUnsafe(p.resolve("#" + attr.Value))
		}
		if (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "nodeID" {
			return p.getBNode(attr.Value)
		}
	}
	return NewBNode()
}

func (p *rdfxmlParser) resolve(uri string) string {
	if p.base == "" || isAbsoluteIRI(uri) {
		return uri
	}
	if uri == "" {
		return p.base
	}
	if strings.HasPrefix(uri, "#") {
		return p.base + uri
	}
	lastSlash := strings.LastIndex(p.base, "/")
	if lastSlash >= 0 {
		return p.base[:lastSlash+1] + uri
	}
	return p.base + "/" + uri
}

func (p *rdfxmlParser) getBNode(id string) BNode {
	if b, ok := p.bnodeMap[id]; ok {
		return b
	}
	b := NewBNode(id)
	p.bnodeMap[id] = b
	return b
}

func readInnerXML(decoder *xml.Decoder) (string, error) {
	var sb strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			sb.WriteString("<" + t.Name.Local + ">")
		case xml.EndElement:
			depth--
			if depth > 0 {
				sb.WriteString("</" + t.Name.Local + ">")
			}
		case xml.CharData:
			sb.Write(t)
		}
	}
	return sb.String(), nil
}

const xmlNS = "http://www.w3.org/XML/1998/namespace"

func isXMLAttr(attr xml.Attr, local string) bool {
	return attr.Name.Local == local && (attr.Name.Space == "xml" || attr.Name.Space == xmlNS)
}

func skipToEnd(decoder *xml.Decoder) {
	depth := 1
	for depth > 0 {
		tok, _ := decoder.Token()
		if tok == nil {
			return
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
}
