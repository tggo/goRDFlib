package rdfxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/bnodes"
)

const (
	rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	xmlNS = "http://www.w3.org/XML/1998/namespace"
	itsNS = "http://www.w3.org/2005/11/its"
)

// Parse parses RDF/XML format into the given graph. Labelled blank nodes share
// one scope per call, unless WithPreserveBlankNodeIDs is set. Anonymous nodes
// always receive fresh identifiers.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	p := &rdfxmlParser{
		g:          g,
		base:       cfg.base,
		provenance: cfg.provenance,
		bnodes:     bnodes.New(cfg.preserveBlankNodeIDs),
		usedIDs:    make(map[string]bool),
		nsPrefixes: make(map[string]string),
	}
	return p.parse(r)
}

type rdfxmlParser struct {
	g             *rdflibgo.Graph
	base          string
	provenance    ProvenanceHandler
	dec           *xml.Decoder // the decoder currently being read, for source positions
	bnodes        bnodes.Scope
	usedIDs       map[string]bool   // track rdf:ID values for uniqueness
	nsPrefixes    map[string]string // prefix → namespace URI (in-scope)
	nsPrefixOrder []string          // insertion order of prefixes
	rdfVersion    string            // "1.2" if rdf:version="1.2" on root (RDF 1.2)
}

// itsContext holds the ITS (Internationalization Tag Set) directional context.
type itsContext struct {
	version string // its:version ("2.0" to enable)
	dir     string // "ltr" | "rtl" | ""
}

// rdfNames that are not allowed as node element names.
var forbiddenNodeElementNames = map[string]bool{
	rdfNS + "RDF":             true,
	rdfNS + "Description":     false, // allowed
	rdfNS + "ID":              true,
	rdfNS + "about":           true,
	rdfNS + "parseType":       true,
	rdfNS + "resource":        true,
	rdfNS + "nodeID":          true,
	rdfNS + "datatype":        true,
	rdfNS + "li":              true,
	rdfNS + "aboutEach":       true,
	rdfNS + "aboutEachPrefix": true,
	rdfNS + "bagID":           true,
}

// rdfNames that are not allowed as property element names.
var forbiddenPropertyElementNames = map[string]bool{
	rdfNS + "RDF":             true,
	rdfNS + "Description":     true,
	rdfNS + "ID":              true,
	rdfNS + "about":           true,
	rdfNS + "parseType":       true,
	rdfNS + "resource":        true,
	rdfNS + "nodeID":          true,
	rdfNS + "datatype":        true,
	rdfNS + "aboutEach":       true,
	rdfNS + "aboutEachPrefix": true,
	rdfNS + "bagID":           true,
}

// rdfNames that are not allowed as property attribute URIs.
var forbiddenPropertyAttributeNames = map[string]bool{
	rdfNS + "RDF":             true,
	rdfNS + "Description":     true,
	rdfNS + "li":              true,
	rdfNS + "aboutEach":       true,
	rdfNS + "aboutEachPrefix": true,
	rdfNS + "bagID":           true,
}

// coreRDFAttrs are rdf attributes handled specially, not as property attributes.
var coreRDFAttrs = map[string]bool{
	"about":            true,
	"ID":               true,
	"nodeID":           true,
	"resource":         true,
	"parseType":        true,
	"datatype":         true,
	"type":             false, // handled specially but IS a property attribute in some contexts
	"annotation":       true,
	"annotationNodeID": true,
	"version":          true,
}

// add records a triple in the graph and, when provenance is enabled, reports
// the line the parser has reached.
//
// Every triple this parser produces goes through here rather than calling
// g.Add directly, so that a new production cannot quietly skip provenance.
func (p *rdfxmlParser) add(s rdflibgo.Subject, pred rdflibgo.URIRef, o rdflibgo.Term) {
	p.g.Add(s, pred, o)
	if p.provenance == nil || p.dec == nil {
		return
	}
	// InputPos reports where the decoder is after the token that completed the
	// triple, which for an element-form triple is its end tag.
	line, _ := p.dec.InputPos()
	p.provenance(s, pred, o, line)
}

func (p *rdfxmlParser) parse(r io.Reader) error {
	decoder := xml.NewDecoder(r)
	p.dec = decoder
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
			_, err := p.parseNodeElement(decoder, se, "", itsContext{})
			return err
		}
	}
}

func (p *rdfxmlParser) parseRDFRoot(decoder *xml.Decoder, root xml.StartElement) error {
	p.collectNamespaces(root)
	rootITS := itsContext{}
	rootLang := ""
	for _, attr := range root.Attr {
		if isXMLAttr(attr, "base") {
			p.base = attr.Value
		}
		if isXMLAttr(attr, "lang") {
			rootLang = attr.Value
		}
		if isRDFAttr(attr) && attr.Name.Local == "version" {
			p.rdfVersion = attr.Value
		}
		if attr.Name.Space == itsNS && attr.Name.Local == "version" {
			rootITS.version = attr.Value
		}
		if attr.Name.Space == itsNS && attr.Name.Local == "dir" {
			rootITS.dir = attr.Value
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
			if _, err := p.parseNodeElement(decoder, t, rootLang, rootITS); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// parseNodeElement handles both typed and untyped node elements.
// Returns the subject used for this node element.
func (p *rdfxmlParser) parseNodeElement(decoder *xml.Decoder, el xml.StartElement, parentLang string, its itsContext) (rdflibgo.Subject, error) {
	elemURI := el.Name.Space + el.Name.Local

	p.collectNamespaces(el)

	// Validate node element name.
	if forbidden, ok := forbiddenNodeElementNames[elemURI]; ok && forbidden {
		return nil, fmt.Errorf("rdf/xml: %s not allowed as node element name", elemURI)
	}

	lang := parentLang
	savedBase := p.base
	defer func() { p.base = savedBase }()

	// Extract xml:lang, xml:base, and ITS attributes.
	for _, attr := range el.Attr {
		if isXMLAttr(attr, "lang") {
			lang = attr.Value
		} else if isXMLAttr(attr, "base") {
			p.base = attr.Value
		}
		if attr.Name.Space == itsNS && attr.Name.Local == "version" {
			its.version = attr.Value
		}
		if attr.Name.Space == itsNS && attr.Name.Local == "dir" {
			its.dir = attr.Value
		}
	}

	// Determine subject — check for conflicting attributes.
	var subj rdflibgo.Subject
	var hasAbout, hasID, hasNodeID bool
	for _, attr := range el.Attr {
		if !isRDFAttr(attr) {
			continue
		}
		switch attr.Name.Local {
		case "about":
			hasAbout = true
			subj = rdflibgo.NewURIRefUnsafe(p.resolve(attr.Value))
		case "ID":
			hasID = true
			if err := p.checkID(attr.Value); err != nil {
				return nil, err
			}
			subj = rdflibgo.NewURIRefUnsafe(p.resolve("#" + attr.Value))
		case "nodeID":
			hasNodeID = true
			if !isValidNCName(attr.Value) {
				return nil, fmt.Errorf("rdf/xml: invalid rdf:nodeID %q", attr.Value)
			}
			subj = p.getBNode(attr.Value)
		}
	}
	// Validate: at most one of about, ID, nodeID.
	if (hasAbout && hasID) || (hasAbout && hasNodeID) || (hasID && hasNodeID) {
		return nil, fmt.Errorf("rdf/xml: conflicting subject attributes (about/ID/nodeID)")
	}
	if subj == nil {
		subj = rdflibgo.NewBNode()
	}

	// Emit rdf:type for typed nodes.
	if elemURI != rdfNS+"Description" {
		p.add(subj, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe(elemURI))
	}

	// Process property attributes on node element.
	for _, attr := range el.Attr {
		if isXMLNSAttr(attr) || isAnyXMLAttr(attr) {
			continue
		}
		if isRDFAttr(attr) {
			switch attr.Name.Local {
			case "about", "ID", "nodeID":
				continue // already handled
			case "type":
				p.add(subj, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe(p.resolve(attr.Value)))
				continue
			default:
				attrURI := rdfNS + attr.Name.Local
				if forbiddenPropertyAttributeNames[attrURI] {
					return nil, fmt.Errorf("rdf/xml: %s not allowed as property attribute", attrURI)
				}
			}
		}
		attrURI := attr.Name.Space + attr.Name.Local
		if attr.Name.Space == "" {
			continue // unqualified attributes on node elements are ignored
		}
		if forbiddenPropertyAttributeNames[attrURI] {
			return nil, fmt.Errorf("rdf/xml: %s not allowed as property attribute", attrURI)
		}
		litOpts := p.langOpts(lang, its)
		p.add(subj, rdflibgo.NewURIRefUnsafe(attrURI), rdflibgo.NewLiteral(attr.Value, litOpts...))
	}

	// Parse child property elements.
	liCounter := 1
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return subj, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := p.parsePropertyElement(decoder, t, subj, lang, &liCounter, its); err != nil {
				return nil, err
			}
		case xml.EndElement:
			return subj, nil
		}
	}
}

func (p *rdfxmlParser) parsePropertyElement(decoder *xml.Decoder, el xml.StartElement, subj rdflibgo.Subject, parentLang string, liCounter *int, its itsContext) error {
	predURI := el.Name.Space + el.Name.Local

	// Handle rdf:li → rdf:_N
	if predURI == rdfNS+"li" {
		predURI = fmt.Sprintf("%s_%d", rdfNS, *liCounter)
		*liCounter++
	}

	// Validate property element name.
	if forbiddenPropertyElementNames[predURI] {
		return fmt.Errorf("rdf/xml: %s not allowed as property element name", predURI)
	}

	pred := rdflibgo.NewURIRefUnsafe(predURI)
	lang := parentLang

	savedBase := p.base
	defer func() { p.base = savedBase }()

	// Extract xml:lang, xml:base, and ITS attrs.
	for _, attr := range el.Attr {
		if isXMLAttr(attr, "lang") {
			lang = attr.Value
		} else if isXMLAttr(attr, "base") {
			p.base = attr.Value
		}
		if attr.Name.Space == itsNS && attr.Name.Local == "version" {
			its.version = attr.Value
		}
		if attr.Name.Space == itsNS && attr.Name.Local == "dir" {
			its.dir = attr.Value
		}
	}

	var resource, nodeID, parseType, datatype, reifyID string
	var annotationIRI, annotationNodeID string
	var hasResource bool
	var propAttrs []xml.Attr

	for _, attr := range el.Attr {
		if isXMLNSAttr(attr) || isAnyXMLAttr(attr) {
			continue
		}
		// Skip ITS namespace attributes (handled above).
		if attr.Name.Space == itsNS {
			continue
		}
		if isRDFAttr(attr) {
			switch attr.Name.Local {
			case "resource":
				resource = attr.Value
				hasResource = true
			case "nodeID":
				nodeID = attr.Value
			case "parseType":
				parseType = attr.Value
			case "datatype":
				datatype = attr.Value
			case "ID":
				reifyID = attr.Value
			case "annotation":
				annotationIRI = attr.Value
			case "annotationNodeID":
				annotationNodeID = attr.Value
			case "version":
				// rdf:version on property element enables RDF 1.2 features
				if attr.Value == "1.2" {
					p.rdfVersion = "1.2"
				}
				continue
			case "type":
				propAttrs = append(propAttrs, attr)
			default:
				attrURI := rdfNS + attr.Name.Local
				if forbiddenPropertyAttributeNames[attrURI] {
					return fmt.Errorf("rdf/xml: %s not allowed as property attribute", attrURI)
				}
				propAttrs = append(propAttrs, attr)
			}
			continue
		}
		if attr.Name.Space == "" {
			continue
		}
		attrURI := attr.Name.Space + attr.Name.Local
		if forbiddenPropertyAttributeNames[attrURI] {
			return fmt.Errorf("rdf/xml: %s not allowed as property attribute", attrURI)
		}
		propAttrs = append(propAttrs, attr)
	}

	// Validate incompatible combinations.
	if parseType == "Literal" && resource != "" {
		return fmt.Errorf("rdf/xml: rdf:parseType='Literal' and rdf:resource cannot be combined")
	}
	if parseType == "Literal" && nodeID != "" {
		return fmt.Errorf("rdf/xml: rdf:parseType='Literal' and rdf:nodeID cannot be combined")
	}
	if hasResource && nodeID != "" {
		return fmt.Errorf("rdf/xml: rdf:resource and rdf:nodeID cannot be combined")
	}

	if reifyID != "" {
		if err := p.checkID(reifyID); err != nil {
			return err
		}
	}

	// Case 1: rdf:resource or rdf:nodeID → resource property element.
	if hasResource || nodeID != "" || len(propAttrs) > 0 {
		// This is a resource-valued property or has property attributes.
		if resource == "" && nodeID == "" && len(propAttrs) > 0 {
			// Empty property element with property attributes → create blank node.
			obj := rdflibgo.NewBNode()
			p.add(subj, pred, obj)
			p.emitPropertyAttrs(obj, propAttrs, lang, its)
			if reifyID != "" {
				p.emitReification(reifyID, subj, pred, obj)
			}
			p.emitAnnotation(annotationIRI, annotationNodeID, subj, pred, obj)
			decoder.Skip()
			return nil
		}
		var obj rdflibgo.Term
		if hasResource {
			obj = rdflibgo.NewURIRefUnsafe(p.resolve(resource))
		} else {
			if !isValidNCName(nodeID) {
				return fmt.Errorf("rdf/xml: invalid rdf:nodeID %q", nodeID)
			}
			obj = p.getBNode(nodeID)
		}
		p.add(subj, pred, obj)
		if len(propAttrs) > 0 {
			if objSubj, ok := obj.(rdflibgo.Subject); ok {
				p.emitPropertyAttrs(objSubj, propAttrs, lang, its)
			}
		}
		if reifyID != "" {
			p.emitReification(reifyID, subj, pred, obj)
		}
		p.emitAnnotation(annotationIRI, annotationNodeID, subj, pred, obj)
		decoder.Skip()
		return nil
	}

	// Case 2: parseType="Resource"
	if parseType == "Resource" {
		bnode := rdflibgo.NewBNode()
		p.add(subj, pred, bnode)
		if reifyID != "" {
			p.emitReification(reifyID, subj, pred, bnode)
		}
		p.emitAnnotation(annotationIRI, annotationNodeID, subj, pred, bnode)
		liCounter := 1
		for {
			tok, err := decoder.Token()
			if err != nil {
				return err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if err := p.parsePropertyElement(decoder, t, bnode, lang, &liCounter, its); err != nil {
					return err
				}
			case xml.EndElement:
				return nil
			}
		}
	}

	// Case 3: parseType="Collection"
	if parseType == "Collection" {
		return p.parseCollection(decoder, subj, pred, lang, reifyID, annotationIRI, annotationNodeID, its)
	}

	// Case 3b: parseType="Triple" (RDF 1.2)
	if parseType == "Triple" {
		return p.parseTripleParseType(decoder, subj, pred, reifyID, annotationIRI, annotationNodeID, lang, its)
	}

	// Case 4: parseType="Literal" → XML literal
	if parseType == "Literal" {
		content, err := readInnerXML(decoder, p.nsPrefixes, p.nsPrefixOrder)
		if err != nil {
			return err
		}
		lit := rdflibgo.NewLiteral(content, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(rdfNS+"XMLLiteral")))
		p.add(subj, pred, lit)
		if reifyID != "" {
			p.emitReification(reifyID, subj, pred, lit)
		}
		p.emitAnnotation(annotationIRI, annotationNodeID, subj, pred, lit)
		return nil
	}

	// Case 5: Default — literal or nested node element.
	var textContent strings.Builder

	for {
		tok, err := decoder.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			textContent.Write(t)
		case xml.StartElement:
			// Child node element.
			childSubj, err := p.parseNodeElement(decoder, t, lang, its)
			if err != nil {
				return err
			}
			p.add(subj, pred, childSubj)
			if reifyID != "" {
				p.emitReification(reifyID, subj, pred, childSubj)
			}
			p.emitAnnotation(annotationIRI, annotationNodeID, subj, pred, childSubj)
			if err := skipToEnd(decoder); err != nil {
				return err
			}
			return nil
		case xml.EndElement:
			text := textContent.String()
			var opts []rdflibgo.LiteralOption
			if datatype != "" {
				opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(p.resolve(datatype))))
			} else {
				opts = p.langOpts(lang, its)
			}
			lit := rdflibgo.NewLiteral(text, opts...)
			p.add(subj, pred, lit)
			if reifyID != "" {
				p.emitReification(reifyID, subj, pred, lit)
			}
			p.emitAnnotation(annotationIRI, annotationNodeID, subj, pred, lit)
			return nil
		}
	}
}

func (p *rdfxmlParser) parseCollection(decoder *xml.Decoder, subj rdflibgo.Subject, pred rdflibgo.URIRef, lang string, reifyID string, annotIRI, annotNodeID string, its itsContext) error {
	var items []rdflibgo.Subject
	for {
		tok, err := decoder.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			childSubj, err := p.parseNodeElement(decoder, t, lang, its)
			if err != nil {
				return err
			}
			items = append(items, childSubj)
		case xml.EndElement:
			if len(items) == 0 {
				p.add(subj, pred, rdflibgo.RDF.Nil)
				if reifyID != "" {
					p.emitReification(reifyID, subj, pred, rdflibgo.RDF.Nil)
				}
				p.emitAnnotation(annotIRI, annotNodeID, subj, pred, rdflibgo.RDF.Nil)
				return nil
			}
			head := rdflibgo.NewBNode()
			p.add(subj, pred, head)
			if reifyID != "" {
				p.emitReification(reifyID, subj, pred, head)
			}
			p.emitAnnotation(annotIRI, annotNodeID, subj, pred, head)
			current := head
			for i, item := range items {
				p.add(current, rdflibgo.RDF.First, item)
				if i < len(items)-1 {
					next := rdflibgo.NewBNode()
					p.add(current, rdflibgo.RDF.Rest, next)
					current = next
				} else {
					p.add(current, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
				}
			}
			return nil
		}
	}
}

func (p *rdfxmlParser) emitPropertyAttrs(subj rdflibgo.Subject, attrs []xml.Attr, lang string, its itsContext) {
	for _, attr := range attrs {
		attrURI := attr.Name.Space + attr.Name.Local
		if isRDFAttr(attr) && attr.Name.Local == "type" {
			p.add(subj, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe(p.resolve(attr.Value)))
			continue
		}
		litOpts := p.langOpts(lang, its)
		p.add(subj, rdflibgo.NewURIRefUnsafe(attrURI), rdflibgo.NewLiteral(attr.Value, litOpts...))
	}
}

// parseTripleParseType handles rdf:parseType="Triple" (RDF 1.2).
// Parses exactly one child node element with exactly one property to form a triple term.
func (p *rdfxmlParser) parseTripleParseType(decoder *xml.Decoder, subj rdflibgo.Subject, pred rdflibgo.URIRef, reifyID, annotIRI, annotNodeID, lang string, its itsContext) error {
	if p.rdfVersion != "1.2" {
		// Per RDF 1.2 XML Syntax §3, parseType="Triple" is only recognized when
		// rdf:version="1.2" is declared. Without it, the element is silently skipped
		// (W3C test rdf12-xml-tt-01 confirms empty output).
		decoder.Skip()
		return nil
	}

	// Use a temporary graph to capture the inner triple.
	tempG := rdflibgo.NewGraph()
	savedG, savedProvenance := p.g, p.provenance
	p.g = tempG
	// Captured triples are components of a triple term, not assertions in the
	// caller's graph. Only the outer assertion has output provenance.
	p.provenance = nil
	defer func() {
		p.g, p.provenance = savedG, savedProvenance
	}()

	var found bool
	for {
		tok, err := decoder.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if found {
				return fmt.Errorf("rdf/xml: parseType='Triple' must contain exactly one node element")
			}
			_, innerErr := p.parseNodeElement(decoder, t, lang, its)
			if innerErr != nil {
				return innerErr
			}
			found = true
		case xml.EndElement:
			if !found {
				return fmt.Errorf("rdf/xml: parseType='Triple' must contain exactly one node element")
			}
			// Extract the single triple from tempG
			var triples []rdflibgo.Triple
			tempG.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
				triples = append(triples, t)
				return true
			})
			if len(triples) != 1 {
				return fmt.Errorf("rdf/xml: parseType='Triple' node must produce exactly 1 triple, got %d", len(triples))
			}
			innerT := triples[0]
			tt := rdflibgo.NewTripleTerm(innerT.Subject, innerT.Predicate, innerT.Object)
			p.g = savedG // restore before adding to real graph; defer is a safety net
			p.provenance = savedProvenance
			p.add(subj, pred, tt)
			if reifyID != "" {
				p.emitReification(reifyID, subj, pred, tt)
			}
			p.emitAnnotation(annotIRI, annotNodeID, subj, pred, tt)
			return nil
		}
	}
}

// emitAnnotation emits an rdf:reifies triple for RDF 1.2 annotations.
func (p *rdfxmlParser) emitAnnotation(annotIRI, annotNodeID string, subj rdflibgo.Subject, pred rdflibgo.URIRef, obj rdflibgo.Term) {
	if annotIRI == "" && annotNodeID == "" {
		return
	}
	var reifier rdflibgo.Subject
	if annotIRI != "" {
		reifier = rdflibgo.NewURIRefUnsafe(p.resolve(annotIRI))
	} else {
		reifier = p.getBNode(annotNodeID)
	}
	tt := rdflibgo.NewTripleTerm(subj, pred, obj)
	p.add(reifier, rdflibgo.RDFReifies, tt)
}

// langOpts returns literal options for lang + directional context.
func (p *rdfxmlParser) langOpts(lang string, its itsContext) []rdflibgo.LiteralOption {
	if lang == "" {
		return nil
	}
	opts := []rdflibgo.LiteralOption{rdflibgo.WithLang(lang)}
	if p.rdfVersion == "1.2" && its.dir != "" {
		opts = append(opts, rdflibgo.WithDir(its.dir))
	}
	return opts
}

func (p *rdfxmlParser) emitReification(id string, subj rdflibgo.Subject, pred rdflibgo.URIRef, obj rdflibgo.Term) {
	stmt := rdflibgo.NewURIRefUnsafe(p.resolve("#" + id))
	p.add(stmt, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe(rdfNS+"Statement"))
	p.add(stmt, rdflibgo.NewURIRefUnsafe(rdfNS+"subject"), subj)
	p.add(stmt, rdflibgo.NewURIRefUnsafe(rdfNS+"predicate"), pred)
	p.add(stmt, rdflibgo.NewURIRefUnsafe(rdfNS+"object"), obj)
}

func (p *rdfxmlParser) checkID(id string) error {
	if !isValidNCName(id) {
		return fmt.Errorf("rdf/xml: invalid rdf:ID %q", id)
	}
	resolved := p.resolve("#" + id)
	if p.usedIDs[resolved] {
		return fmt.Errorf("rdf/xml: duplicate rdf:ID %q", id)
	}
	p.usedIDs[resolved] = true
	return nil
}

func (p *rdfxmlParser) resolve(uri string) string {
	if p.base == "" || isAbsoluteIRI(uri) {
		return uri
	}
	if uri == "" {
		// Empty URI resolves to the base without fragment.
		if idx := strings.Index(p.base, "#"); idx >= 0 {
			return p.base[:idx]
		}
		return p.base
	}
	baseURL, err := url.Parse(p.base)
	if err != nil {
		return uri
	}
	ref, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	resolved := baseURL.ResolveReference(ref).String()
	if strings.Contains(uri, "#") && !strings.Contains(resolved, "#") {
		resolved += "#"
	}
	// Go's url package percent-encodes non-ASCII characters, but RDF uses IRIs
	// which allow Unicode directly. Unescape percent-encoded Unicode.
	if unescaped, err := url.PathUnescape(resolved); err == nil {
		resolved = unescaped
	}
	return resolved
}

// getBNode resolves every labelled node through the parse's shared scope so
// node declarations, property references, and annotation reifiers agree.
func (p *rdfxmlParser) getBNode(id string) rdflibgo.BNode {
	return p.bnodes.Label(id)
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

func isValidNCName(s string) bool {
	if s == "" {
		return false
	}
	for i, ch := range s {
		if i == 0 {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_') {
				return false
			}
		} else {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.' || ch == 0xB7 ||
				(ch >= 0x00C0 && ch <= 0x00D6) || (ch >= 0x00D8 && ch <= 0x00F6) ||
				(ch >= 0x00F8 && ch <= 0x02FF) || (ch >= 0x0300 && ch <= 0x036F) ||
				(ch >= 0x0370 && ch <= 0x037D) || (ch >= 0x037F && ch <= 0x1FFF) ||
				(ch >= 0x200C && ch <= 0x200D) || (ch >= 0x203F && ch <= 0x2040) ||
				(ch >= 0x2070 && ch <= 0x218F) || (ch >= 0x2C00 && ch <= 0x2FEF) ||
				(ch >= 0x3001 && ch <= 0xD7FF) || (ch >= 0xF900 && ch <= 0xFDCF) ||
				(ch >= 0xFDF0 && ch <= 0xFFFD) || (ch >= 0x10000 && ch <= 0xEFFFF)) {
				return false
			}
		}
	}
	return true
}

func (p *rdfxmlParser) collectNamespaces(el xml.StartElement) {
	for _, attr := range el.Attr {
		if attr.Name.Space == "xmlns" {
			if _, exists := p.nsPrefixes[attr.Name.Local]; !exists {
				p.nsPrefixOrder = append(p.nsPrefixOrder, attr.Name.Local)
			}
			p.nsPrefixes[attr.Name.Local] = attr.Value
		}
	}
}

func isRDFAttr(attr xml.Attr) bool {
	return attr.Name.Space == rdfNS || (attr.Name.Space == "" && coreRDFAttrs[attr.Name.Local])
}

func isXMLNSAttr(attr xml.Attr) bool {
	return attr.Name.Space == "xmlns" || (attr.Name.Space == "" && attr.Name.Local == "xmlns")
}

func isXMLAttr(attr xml.Attr, local string) bool {
	return attr.Name.Local == local && (attr.Name.Space == "xml" || attr.Name.Space == xmlNS)
}

func isAnyXMLAttr(attr xml.Attr) bool {
	return attr.Name.Space == "xml" || attr.Name.Space == xmlNS
}

func readInnerXML(decoder *xml.Decoder, nsMap map[string]string, nsOrder []string) (string, error) {
	// Build reverse map: namespace URI → prefix.
	nsToPrefix := make(map[string]string, len(nsMap))
	for prefix, ns := range nsMap {
		nsToPrefix[ns] = prefix
	}

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
			sb.WriteString("<")
			sb.WriteString(t.Name.Local)

			// Add in-scope namespace declarations.
			for _, prefix := range nsOrder {
				ns := nsMap[prefix]
				sb.WriteString(fmt.Sprintf(` xmlns:%s="%s"`, prefix, ns))
			}

			for _, attr := range t.Attr {
				if attr.Name.Space == "xmlns" {
					continue // skip xmlns declarations (we add our own)
				}
				sb.WriteString(" ")
				if attr.Name.Space != "" {
					if prefix, ok := nsToPrefix[attr.Name.Space]; ok {
						sb.WriteString(prefix)
						sb.WriteString(":")
					}
				}
				sb.WriteString(attr.Name.Local)
				sb.WriteString(`="`)
				xmlEscapeToBuilder(&sb, attr.Value)
				sb.WriteString(`"`)
			}
			sb.WriteString(">")
		case xml.EndElement:
			depth--
			if depth > 0 {
				sb.WriteString("</")
				sb.WriteString(t.Name.Local)
				sb.WriteString(">")
			}
		case xml.CharData:
			xmlEscapeToBuilder(&sb, string(t))
		}
	}
	return sb.String(), nil
}

func xmlEscapeToBuilder(sb *strings.Builder, s string) {
	for _, r := range s {
		switch r {
		case '<':
			sb.WriteString("&lt;")
		case '>':
			sb.WriteString("&gt;")
		case '&':
			sb.WriteString("&amp;")
		case '"':
			sb.WriteString("&quot;")
		default:
			sb.WriteRune(r)
		}
	}
}

func skipToEnd(decoder *xml.Decoder) error {
	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("skipToEnd: %w", err)
		}
		if tok == nil {
			return nil
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}
