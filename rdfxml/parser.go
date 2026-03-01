package rdfxml

import (
	"encoding/xml"
	"io"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

const rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
const xmlNS = "http://www.w3.org/XML/1998/namespace"

// Parse parses RDF/XML format into the given graph.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	p := &rdfxmlParser{
		g:        g,
		base:     cfg.base,
		bnodeMap: make(map[string]rdflibgo.BNode),
	}
	return p.parse(r)
}

type rdfxmlParser struct {
	g        *rdflibgo.Graph
	base     string
	lang     string
	bnodeMap map[string]rdflibgo.BNode
}

func (p *rdfxmlParser) parse(r io.Reader) error {
	decoder := xml.NewDecoder(r)
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
			return p.parseNodeElement(decoder, se, "")
		}
	}
}

func (p *rdfxmlParser) parseRDFRoot(decoder *xml.Decoder, root xml.StartElement) error {
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
			return nil
		}
	}
}

func (p *rdfxmlParser) parseNodeElement(decoder *xml.Decoder, el xml.StartElement, parentLang string, preSubj ...rdflibgo.Subject) error {
	elemURI := el.Name.Space + el.Name.Local

	var subj rdflibgo.Subject
	if len(preSubj) > 0 && preSubj[0] != nil {
		subj = preSubj[0]
	}
	lang := parentLang

	savedBase := p.base

	for _, attr := range el.Attr {
		switch {
		case isXMLAttr(attr, "lang"):
			lang = attr.Value
		case isXMLAttr(attr, "base"):
			p.base = attr.Value
		case subj == nil && (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "about":
			subj = rdflibgo.NewURIRefUnsafe(p.resolve(attr.Value))
		case subj == nil && (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "ID":
			subj = rdflibgo.NewURIRefUnsafe(p.resolve("#" + attr.Value))
		case subj == nil && (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "nodeID":
			subj = p.getBNode(attr.Value)
		}
	}

	if subj == nil {
		subj = rdflibgo.NewBNode()
	}

	defer func() { p.base = savedBase }()

	if elemURI != rdfNS+"Description" {
		p.g.Add(subj, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe(elemURI))
	}

	for _, attr := range el.Attr {
		if attr.Name.Space == "xml" || attr.Name.Space == "xmlns" {
			continue
		}
		attrURI := attr.Name.Space + attr.Name.Local
		if attrURI == rdfNS+"about" || attrURI == rdfNS+"ID" || attrURI == rdfNS+"nodeID" || attrURI == rdfNS+"type" {
			if attrURI == rdfNS+"type" {
				p.g.Add(subj, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe(p.resolve(attr.Value)))
			}
			continue
		}
		if attr.Name.Space == "" {
			continue
		}
		var opts []rdflibgo.LiteralOption
		if lang != "" {
			opts = append(opts, rdflibgo.WithLang(lang))
		}
		p.g.Add(subj, rdflibgo.NewURIRefUnsafe(attrURI), rdflibgo.NewLiteral(attr.Value, opts...))
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
			if err := p.parsePropertyElement(decoder, t, subj, lang); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

func (p *rdfxmlParser) parsePropertyElement(decoder *xml.Decoder, el xml.StartElement, subj rdflibgo.Subject, parentLang string) error {
	predURI := el.Name.Space + el.Name.Local

	pred := rdflibgo.NewURIRefUnsafe(predURI)
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

	if resource != "" {
		p.g.Add(subj, pred, rdflibgo.NewURIRefUnsafe(p.resolve(resource)))
		decoder.Skip()
		return nil
	}

	if nodeID != "" {
		p.g.Add(subj, pred, p.getBNode(nodeID))
		decoder.Skip()
		return nil
	}

	if parseType == "Resource" {
		bnode := rdflibgo.NewBNode()
		p.g.Add(subj, pred, bnode)
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

	if parseType == "Collection" {
		return p.parseCollection(decoder, subj, pred, lang)
	}

	if parseType == "Literal" {
		content, err := readInnerXML(decoder)
		if err != nil {
			return err
		}
		lit := rdflibgo.NewLiteral(content, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(rdfNS+"XMLLiteral")))
		p.g.Add(subj, pred, lit)
		return nil
	}

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
			childSubj := p.extractSubject(t)
			if err := p.parseNodeElement(decoder, t, lang, childSubj); err != nil {
				return err
			}
			p.g.Add(subj, pred, childSubj)
			skipToEnd(decoder)
			return nil
		case xml.EndElement:
			if hasChild {
				return nil
			}
			text := textContent.String()
			var opts []rdflibgo.LiteralOption
			if lang != "" {
				opts = append(opts, rdflibgo.WithLang(lang))
			}
			if datatype != "" {
				opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(p.resolve(datatype))))
			}
			p.g.Add(subj, pred, rdflibgo.NewLiteral(text, opts...))
			return nil
		}
	}
}

func (p *rdfxmlParser) parseCollection(decoder *xml.Decoder, subj rdflibgo.Subject, pred rdflibgo.URIRef, lang string) error {
	var items []rdflibgo.Subject
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
			if len(items) == 0 {
				p.g.Add(subj, pred, rdflibgo.RDF.Nil)
				return nil
			}
			head := rdflibgo.NewBNode()
			p.g.Add(subj, pred, head)
			current := head
			for i, item := range items {
				p.g.Add(current, rdflibgo.RDF.First, item)
				if i < len(items)-1 {
					next := rdflibgo.NewBNode()
					p.g.Add(current, rdflibgo.RDF.Rest, next)
					current = next
				} else {
					p.g.Add(current, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
				}
			}
			return nil
		}
	}
}

func (p *rdfxmlParser) extractSubject(el xml.StartElement) rdflibgo.Subject {
	for _, attr := range el.Attr {
		if (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "about" {
			return rdflibgo.NewURIRefUnsafe(p.resolve(attr.Value))
		}
		if (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "ID" {
			return rdflibgo.NewURIRefUnsafe(p.resolve("#" + attr.Value))
		}
		if (attr.Name.Space == rdfNS || attr.Name.Space == "") && attr.Name.Local == "nodeID" {
			return p.getBNode(attr.Value)
		}
	}
	return rdflibgo.NewBNode()
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

func (p *rdfxmlParser) getBNode(id string) rdflibgo.BNode {
	if b, ok := p.bnodeMap[id]; ok {
		return b
	}
	b := rdflibgo.NewBNode(id)
	p.bnodeMap[id] = b
	return b
}

func isAbsoluteIRI(s string) bool {
	colon := strings.Index(s, ":")
	if colon <= 0 {
		return false
	}
	for i := 0; i < colon; i++ {
		ch := s[i]
		if i == 0 {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
				return false
			}
		} else {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '+' || ch == '-' || ch == '.') {
				return false
			}
		}
	}
	return true
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
