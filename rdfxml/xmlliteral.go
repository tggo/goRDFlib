package rdfxml

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

// nsDecl is one xmlns declaration: prefix "" is the default namespace.
type nsDecl struct{ prefix, uri string }

// readXMLLiteral reads the content of an rdf:parseType="Literal" property
// element up to its end tag and returns it in Exclusive XML Canonicalization
// form without comments (https://www.w3.org/TR/xml-exc-c14n/), as RDF 1.1
// Concepts §3.3 requires for the lexical form of an rdf:XMLLiteral.
//
// outer holds the declarations in scope at the property element.
//
// Limitation: encoding/xml resolves prefixes to namespace URIs and drops the
// prefix. The prefix written for a name is the innermost in-scope prefix bound
// to its namespace; a document that binds one namespace to several prefixes
// at once may get a different but equivalent prefix back. Comments and
// processing instructions are dropped (exc-c14n "without comments").
func readXMLLiteral(decoder *xml.Decoder, outer []nsDecl) (string, error) {
	scope := append([]nsDecl(nil), outer...)
	// rendered[i] is what the nearest output ancestor at depth i has declared.
	rendered := []map[string]string{{}}
	var marks []int
	var sb strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("rdf/xml: reading rdf:parseType=\"Literal\" content: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			marks = append(marks, len(scope))
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" {
					scope = append(scope, nsDecl{a.Name.Local, a.Value})
				} else if a.Name.Space == "" && a.Name.Local == "xmlns" {
					scope = append(scope, nsDecl{"", a.Value})
				}
			}
			parent := rendered[len(rendered)-1]
			cur := make(map[string]string, len(parent))
			for k, v := range parent {
				cur[k] = v
			}

			qname := func(space, local string, attr bool) string {
				if space == "" {
					return local
				}
				if space == xmlNS || space == "xml" {
					return "xml:" + local
				}
				if pfx, ok := prefixFor(scope, space, attr); ok && pfx != "" {
					return pfx + ":" + local
				}
				return local
			}
			// Visibly utilised prefixes.
			used := map[string]string{}
			if t.Name.Space != "" {
				if pfx, ok := prefixFor(scope, t.Name.Space, false); ok {
					used[pfx] = t.Name.Space
				} else {
					used[""] = ""
				}
			} else {
				used[""] = ""
			}
			type attrOut struct{ space, local, name, val string }
			var attrs []attrOut
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				if a.Name.Space != "" && a.Name.Space != xmlNS && a.Name.Space != "xml" {
					if pfx, ok := prefixFor(scope, a.Name.Space, true); ok {
						used[pfx] = a.Name.Space
					}
				}
				attrs = append(attrs, attrOut{a.Name.Space, a.Name.Local, qname(a.Name.Space, a.Name.Local, true), a.Value})
			}

			var decls []nsDecl
			for pfx, uri := range used {
				old, had := cur[pfx]
				if had && old == uri {
					continue
				}
				if !had && pfx == "" && uri == "" {
					continue // xmlns="" only undoes a default that was written
				}
				decls = append(decls, nsDecl{pfx, uri})
				cur[pfx] = uri
			}
			sort.Slice(decls, func(i, j int) bool { return decls[i].prefix < decls[j].prefix })
			sort.Slice(attrs, func(i, j int) bool {
				if attrs[i].space != attrs[j].space {
					return attrs[i].space < attrs[j].space
				}
				return attrs[i].local < attrs[j].local
			})

			sb.WriteByte('<')
			sb.WriteString(qname(t.Name.Space, t.Name.Local, false))
			for _, d := range decls {
				if d.prefix == "" {
					sb.WriteString(` xmlns="`)
				} else {
					sb.WriteString(" xmlns:" + d.prefix + `="`)
				}
				c14nEscape(&sb, d.uri, true)
				sb.WriteByte('"')
			}
			for _, a := range attrs {
				sb.WriteString(" " + a.name + `="`)
				c14nEscape(&sb, a.val, true)
				sb.WriteByte('"')
			}
			sb.WriteByte('>')
			rendered = append(rendered, cur)
		case xml.EndElement:
			depth--
			if depth > 0 {
				sb.WriteString("</")
				sb.WriteString(endName(scope, t.Name))
				sb.WriteString(">")
				scope = scope[:marks[len(marks)-1]]
				marks = marks[:len(marks)-1]
				rendered = rendered[:len(rendered)-1]
			}
		case xml.CharData:
			c14nEscape(&sb, string(t), false)
		}
	}
	return sb.String(), nil
}

func endName(scope []nsDecl, n xml.Name) string {
	if n.Space != "" {
		if pfx, ok := prefixFor(scope, n.Space, false); ok && pfx != "" {
			return pfx + ":" + n.Local
		}
	}
	return n.Local
}

// prefixFor finds the innermost in-scope prefix bound to uri. Unprefixed
// attributes are in no namespace, so for attributes the default namespace is
// never an answer.
func prefixFor(scope []nsDecl, uri string, attr bool) (string, bool) {
	for i := len(scope) - 1; i >= 0; i-- {
		d := scope[i]
		if d.uri != uri || (attr && d.prefix == "") {
			continue
		}
		// Skip a binding that a nearer declaration of the same prefix hides.
		hidden := false
		for j := len(scope) - 1; j > i; j-- {
			if scope[j].prefix == d.prefix {
				hidden = true
				break
			}
		}
		if !hidden {
			return d.prefix, true
		}
	}
	return "", false
}

// c14nEscape writes s as canonical XML character data (attr false) or
// attribute value (attr true), per Canonical XML 1.0 §2.3.
func c14nEscape(sb *strings.Builder, s string, attr bool) {
	for _, r := range s {
		switch r {
		case '&':
			sb.WriteString("&amp;")
		case '<':
			sb.WriteString("&lt;")
		case '\r':
			sb.WriteString("&#xD;")
		case '>':
			if attr {
				sb.WriteRune(r)
			} else {
				sb.WriteString("&gt;")
			}
		case '"':
			if attr {
				sb.WriteString("&quot;")
			} else {
				sb.WriteRune(r)
			}
		case '\t':
			if attr {
				sb.WriteString("&#x9;")
			} else {
				sb.WriteRune(r)
			}
		case '\n':
			if attr {
				sb.WriteString("&#xA;")
			} else {
				sb.WriteRune(r)
			}
		default:
			sb.WriteRune(r)
		}
	}
}
