package rdfxml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrNoQName is returned by Serialize when a predicate IRI cannot be written as
// an RDF/XML property element. RDF/XML (§2.2 of the RDF 1.1 XML Syntax spec)
// writes every predicate as an XML qualified name, namespace plus local name,
// and the local name must be an NCName (Namespaces in XML 1.0 [4]). An IRI
// whose trailing characters cannot form an NCName, such as one ending in a
// digit run or a '/', has no such spelling.
var ErrNoQName = errors.New("rdfxml: predicate IRI has no XML qualified name")

// ErrReservedPropertyName is returned by Serialize when a predicate is one of
// the RDF syntax names that RDF/XML forbids as a property element
// (rdf:Description, rdf:about, rdf:li, ...).
var ErrReservedPropertyName = errors.New("rdfxml: predicate is an RDF/XML syntax name")

// nsTable assigns an XML namespace prefix to every namespace the output uses.
// It is not safe for concurrent use; one is built per Serialize call.
type nsTable struct {
	prefixOf map[string]string // namespace IRI -> prefix
	nsOf     map[string]string // prefix -> namespace IRI
	next     int               // counter for generated ns<N> prefixes
	qnames   map[string]string // IRI -> qname, "" when the IRI has none
}

func newNSTable(bindings map[string]string, reserved map[string]string) *nsTable {
	t := &nsTable{
		prefixOf: make(map[string]string, len(bindings)+len(reserved)),
		nsOf:     make(map[string]string, len(bindings)+len(reserved)),
		qnames:   make(map[string]string),
	}
	for prefix, ns := range reserved {
		t.prefixOf[ns] = prefix
		t.nsOf[prefix] = ns
	}
	// Sorted so that two prefixes bound to one namespace resolve the same way
	// on every run.
	prefixes := make([]string, 0, len(bindings))
	for prefix := range bindings {
		prefixes = append(prefixes, prefix)
	}
	slices.Sort(prefixes)
	for _, prefix := range prefixes {
		ns := bindings[prefix]
		if ns == "" || !isUsablePrefix(prefix) {
			// The empty prefix and non-NCName prefixes cannot be declared as
			// xmlns:<prefix>; the namespace gets a generated prefix if used.
			continue
		}
		if _, taken := t.nsOf[prefix]; taken {
			// A reserved prefix (rdf, its) bound to another namespace: that
			// namespace is renamed rather than declared twice.
			continue
		}
		if _, has := t.prefixOf[ns]; has {
			continue
		}
		t.prefixOf[ns] = prefix
		t.nsOf[prefix] = ns
	}
	return t
}

// isUsablePrefix reports whether p can be declared as xmlns:p. Namespaces in
// XML 1.0 §3 reserves prefixes beginning with "xml" in any case.
func isUsablePrefix(p string) bool {
	return isXMLNCName(p) && !strings.HasPrefix(strings.ToLower(p), "xml")
}

// qname returns the qualified name for iri, allocating a prefix for its
// namespace when none is bound. It returns "" when iri has no QName.
func (t *nsTable) qname(iri string) string {
	if q, ok := t.qnames[iri]; ok {
		return q
	}
	q := t.lookupBound(iri)
	if q == "" {
		if ns, local := splitIRI(iri); local != "" {
			q = t.prefixFor(ns) + ":" + local
		}
	}
	t.qnames[iri] = q
	return q
}

// lookupBound finds the longest declared namespace that leaves a valid local
// name.
func (t *nsTable) lookupBound(iri string) string {
	bestNS := ""
	for ns := range t.prefixOf {
		if len(ns) <= len(bestNS) || !strings.HasPrefix(iri, ns) {
			continue
		}
		if isXMLNCName(iri[len(ns):]) {
			bestNS = ns
		}
	}
	if bestNS == "" {
		return ""
	}
	return t.prefixOf[bestNS] + ":" + iri[len(bestNS):]
}

func (t *nsTable) prefixFor(ns string) string {
	if p, ok := t.prefixOf[ns]; ok {
		return p
	}
	for {
		t.next++
		p := "ns" + strconv.Itoa(t.next)
		if _, taken := t.nsOf[p]; !taken {
			t.prefixOf[ns] = p
			t.nsOf[p] = ns
			return p
		}
	}
}

// declarations returns the namespace declarations sorted by namespace IRI.
func (t *nsTable) declarations() []string {
	nss := make([]string, 0, len(t.prefixOf))
	for ns := range t.prefixOf {
		nss = append(nss, ns)
	}
	slices.Sort(nss)
	return nss
}

// splitIRI splits iri into a namespace and the longest trailing NCName, as
// rdflib's split_uri does. local is "" when no split exists: the IRI ends in a
// character that cannot be part of an XML name, the trailing name characters
// contain no valid start character, or nothing is left for the namespace
// (an empty namespace name cannot be bound to a prefix, Namespaces in XML 1.0
// §3).
func splitIRI(iri string) (ns, local string) {
	i := len(iri)
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(iri[:i])
		if r == utf8.RuneError || !isNameChar(r) {
			break
		}
		i -= size
	}
	for i < len(iri) {
		r, size := utf8.DecodeRuneInString(iri[i:])
		if isNameStartChar(r) {
			break
		}
		i += size
	}
	if i == 0 || i == len(iri) {
		return "", ""
	}
	if !decoderAcceptsName(iri[i:]) {
		return "", ""
	}
	return iri[:i], iri[i:]
}

// isXMLNCName reports whether s is an NCName (Namespaces in XML 1.0 [4]) that
// encoding/xml also reads back as one name.
func isXMLNCName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == utf8.RuneError {
			return false
		}
		if i == 0 && !isNameStartChar(r) || !isNameChar(r) {
			return false
		}
	}
	return decoderAcceptsName(s)
}

// isNameStartChar is XML 1.0 (5th edition) [4] NameStartChar without ':'.
func isNameStartChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		return true
	case r < 0xC0:
		return false
	}
	return r <= 0xD6 || r >= 0xD8 && r <= 0xF6 || r >= 0xF8 && r <= 0x2FF ||
		r >= 0x370 && r <= 0x37D || r >= 0x37F && r <= 0x1FFF ||
		r >= 0x200C && r <= 0x200D || r >= 0x2070 && r <= 0x218F ||
		r >= 0x2C00 && r <= 0x2FEF || r >= 0x3001 && r <= 0xD7FF ||
		r >= 0xF900 && r <= 0xFDCF || r >= 0xFDF0 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0xEFFFF
}

// isNameChar is XML 1.0 (5th edition) [4a] NameChar without ':'.
func isNameChar(r rune) bool {
	return isNameStartChar(r) || r == '-' || r == '.' || r >= '0' && r <= '9' ||
		r == 0xB7 || r >= 0x300 && r <= 0x36F || r >= 0x203F && r <= 0x2040
}

// decoderAcceptsName guards the non-ASCII case. encoding/xml checks names
// against the older XML 1.0 (4th edition) character tables, which are narrower
// than the 5th edition ranges above, and a name it rejects would make our own
// parser fail on our own output. ASCII names are identical under both.
func decoderAcceptsName(s string) bool {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return true
	}
	d := xml.NewDecoder(strings.NewReader("<" + s + "/>"))
	tok, err := d.Token()
	if err != nil {
		return false
	}
	se, ok := tok.(xml.StartElement)
	return ok && se.Name.Local == s && se.Name.Space == ""
}

func noQNameError(iri string) error {
	return fmt.Errorf("%w: <%s> does not end in an NCName, so RDF/XML cannot write it as a property element; "+
		"serialize this graph as Turtle, N-Triples or JSON-LD, or rename the predicate so it ends in a letter or '_' followed by name characters", ErrNoQName, iri)
}
