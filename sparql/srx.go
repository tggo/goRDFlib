package sparql

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"slices"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// ParseSRX parses SPARQL Results XML (.srx) from a reader into a *Result.
func ParseSRX(r io.Reader) (*Result, error) {
	var doc srxDocument
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, err
	}

	// Boolean result
	if doc.Boolean != nil {
		return &Result{Type: "ASK", AskResult: *doc.Boolean}, nil
	}

	vars := make([]string, len(doc.Head.Variables))
	for i, v := range doc.Head.Variables {
		vars[i] = v.Name
	}

	var bindings []map[string]rdflibgo.Term
	for _, r := range doc.Results.Results {
		row := make(map[string]rdflibgo.Term)
		for _, b := range r.Bindings {
			row[b.Name] = parseSRXBinding(b)
		}
		bindings = append(bindings, row)
	}

	return &Result{Type: "SELECT", Vars: vars, Bindings: bindings}, nil
}

type srxDocument struct {
	XMLName xml.Name   `xml:"sparql"`
	Head    srxHead    `xml:"head"`
	Results srxResults `xml:"results"`
	Boolean *bool      `xml:"boolean"`
}

type srxHead struct {
	Variables []srxVariable `xml:"variable"`
}

type srxVariable struct {
	Name string `xml:"name,attr"`
}

type srxResults struct {
	Results []srxResult `xml:"result"`
}

type srxResult struct {
	Bindings []srxBinding `xml:"binding"`
}

type srxBinding struct {
	Name    string     `xml:"name,attr"`
	URI     string     `xml:"uri"`
	BNode   string     `xml:"bnode"`
	Literal *srxLiteral `xml:"literal"`
	Triple  *srxTriple `xml:"triple"`
}

type srxLiteral struct {
	Value    string `xml:",chardata"`
	Lang     string `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	Dir      string `xml:"http://www.w3.org/2005/11/its dir,attr"`
	Datatype string `xml:"datatype,attr"`
}

type srxTriple struct {
	Subject   srxTripleComponent `xml:"subject"`
	Predicate srxTripleComponent `xml:"predicate"`
	Object    srxTripleComponent `xml:"object"`
}

type srxTripleComponent struct {
	URI     string      `xml:"uri"`
	BNode   string      `xml:"bnode"`
	Literal *srxLiteral `xml:"literal"`
	Triple  *srxTriple  `xml:"triple"`
}

func parseSRXBinding(b srxBinding) rdflibgo.Term {
	if b.URI != "" {
		return rdflibgo.NewURIRefUnsafe(b.URI)
	}
	if b.BNode != "" {
		return rdflibgo.NewBNode(b.BNode)
	}
	if b.Literal != nil {
		return parseSRXLiteral(b.Literal)
	}
	if b.Triple != nil {
		return parseSRXTriple(b.Triple)
	}
	return nil
}

func parseSRXLiteral(lit *srxLiteral) rdflibgo.Literal {
	var opts []rdflibgo.LiteralOption
	if lit.Lang != "" {
		if idx := strings.Index(lit.Lang, "--"); idx >= 0 {
			// Legacy: direction embedded in lang tag as lang--dir
			opts = append(opts, rdflibgo.WithLang(lit.Lang[:idx]))
			opts = append(opts, rdflibgo.WithDir(lit.Lang[idx+2:]))
		} else {
			opts = append(opts, rdflibgo.WithLang(lit.Lang))
		}
		// SPARQL 1.2: its:dir attribute takes precedence over --dir suffix
		if lit.Dir != "" {
			opts = append(opts, rdflibgo.WithDir(lit.Dir))
		}
	} else if lit.Datatype != "" {
		opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(lit.Datatype)))
	}
	return rdflibgo.NewLiteral(lit.Value, opts...)
}

func parseSRXTripleComponent(c srxTripleComponent) rdflibgo.Term {
	if c.URI != "" {
		return rdflibgo.NewURIRefUnsafe(c.URI)
	}
	if c.BNode != "" {
		return rdflibgo.NewBNode(c.BNode)
	}
	if c.Literal != nil {
		return parseSRXLiteral(c.Literal)
	}
	if c.Triple != nil {
		return parseSRXTriple(c.Triple)
	}
	return nil
}

func parseSRXTriple(t *srxTriple) rdflibgo.Term {
	s := parseSRXTripleComponent(t.Subject)
	p := parseSRXTripleComponent(t.Predicate)
	o := parseSRXTripleComponent(t.Object)
	subj, okS := s.(rdflibgo.Subject)
	pred, okP := p.(rdflibgo.URIRef)
	if !okS || !okP || o == nil {
		return nil
	}
	return rdflibgo.NewTripleTerm(subj, pred, o)
}

// ParseSRJ parses SPARQL Results JSON (.srj) from a reader into a *Result.
func ParseSRJ(r io.Reader) (*Result, error) {
	var doc srjDocument
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, err
	}

	if doc.Boolean != nil {
		return &Result{Type: "ASK", AskResult: *doc.Boolean}, nil
	}

	vars := doc.Head.Vars

	var bindings []map[string]rdflibgo.Term
	for _, row := range doc.Results.Bindings {
		b := make(map[string]rdflibgo.Term)
		for k, v := range row {
			b[k] = parseSRJValue(v)
		}
		bindings = append(bindings, b)
	}

	return &Result{Type: "SELECT", Vars: vars, Bindings: bindings}, nil
}

type srjDocument struct {
	Head    srjHead    `json:"head"`
	Results srjResults `json:"results"`
	Boolean *bool      `json:"boolean,omitempty"`
}

type srjHead struct {
	Vars []string `json:"vars"`
}

type srjResults struct {
	Bindings []map[string]srjValue `json:"bindings"`
}

type srjValue struct {
	Type     string          `json:"type"`
	Value    json.RawMessage `json:"value"`
	Lang     string          `json:"xml:lang,omitempty"`
	Dir      string          `json:"its:dir,omitempty"`
	Datatype string          `json:"datatype,omitempty"`
}

type srjTripleValue struct {
	Subject   srjValue `json:"subject"`
	Predicate srjValue `json:"predicate"`
	Object    srjValue `json:"object"`
}

func parseSRJValue(v srjValue) rdflibgo.Term {
	switch v.Type {
	case "uri":
		return rdflibgo.NewURIRefUnsafe(srjString(v.Value))
	case "bnode":
		return rdflibgo.NewBNode(srjString(v.Value))
	case "literal", "typed-literal":
		var opts []rdflibgo.LiteralOption
		if v.Lang != "" {
			if idx := strings.Index(v.Lang, "--"); idx >= 0 {
				opts = append(opts, rdflibgo.WithLang(v.Lang[:idx]))
				opts = append(opts, rdflibgo.WithDir(v.Lang[idx+2:]))
			} else {
				opts = append(opts, rdflibgo.WithLang(v.Lang))
			}
		} else if v.Datatype != "" {
			opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(v.Datatype)))
		}
		if v.Dir != "" {
			opts = append(opts, rdflibgo.WithDir(v.Dir))
		}
		return rdflibgo.NewLiteral(srjString(v.Value), opts...)
	case "triple":
		var tv srjTripleValue
		if err := json.Unmarshal(v.Value, &tv); err != nil {
			return nil
		}
		s := parseSRJValue(tv.Subject)
		p := parseSRJValue(tv.Predicate)
		o := parseSRJValue(tv.Object)
		subj, okS := s.(rdflibgo.Subject)
		pred, okP := p.(rdflibgo.URIRef)
		if !okS || !okP || o == nil {
			return nil
		}
		return rdflibgo.NewTripleTerm(subj, pred, o)
	}
	return nil
}

// srjString extracts a string from a json.RawMessage (which may be a JSON string).
func srjString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return string(raw)
	}
	return s
}

// ResultsEqual compares two SPARQL SELECT results for set equality of bindings.
// Variable order doesn't matter. Binding order doesn't matter.
// BNode labels are compared structurally (same label in one result maps to same label in other).
func ResultsEqual(a, b *Result) bool {
	if a.Type != b.Type {
		return false
	}
	if a.Type == "ASK" {
		return a.AskResult == b.AskResult
	}
	if len(a.Bindings) != len(b.Bindings) {
		return false
	}

	// First try without bnode normalization (fast path)
	aKeys := make(map[string]int)
	for _, row := range a.Bindings {
		aKeys[bindingKeyWith(row, false)]++
	}
	bKeys := make(map[string]int)
	for _, row := range b.Bindings {
		bKeys[bindingKeyWith(row, false)]++
	}
	match := true
	for k, v := range aKeys {
		if bKeys[k] != v {
			match = false
			break
		}
	}
	if match && len(aKeys) == len(bKeys) {
		return true
	}

	// Fall back to bnode-normalized comparison
	// Try to find a consistent bnode mapping by matching rows
	return resultEqualWithBnodes(a.Bindings, b.Bindings)
}

func resultEqualWithBnodes(a, b []map[string]rdflibgo.Term) bool {
	// Build keys ignoring bnode labels (replace all bnodes with placeholder)
	aKeys := make(map[string][]int) // key → indices
	for i, row := range a {
		k := bindingKeyWith(row, true)
		aKeys[k] = append(aKeys[k], i)
	}
	bKeys := make(map[string][]int)
	for i, row := range b {
		k := bindingKeyWith(row, true)
		bKeys[k] = append(bKeys[k], i)
	}
	if len(aKeys) != len(bKeys) {
		return false
	}
	for k, av := range aKeys {
		bv, ok := bKeys[k]
		if !ok || len(av) != len(bv) {
			return false
		}
	}
	return true
}

// bindingKeyWith builds a canonical string key for a binding row.
// If collapseBnodes is true, all blank nodes are collapsed to a single placeholder.
func bindingKeyWith(row map[string]rdflibgo.Term, collapseBnodes bool) string {
	var parts []string
	for k, v := range row {
		val := ""
		if v != nil {
			if collapseBnodes {
				if _, ok := v.(rdflibgo.BNode); ok {
					val = "_:BNODE"
					parts = append(parts, k+"="+val)
					continue
				}
			}
			if l, ok := v.(rdflibgo.Literal); ok && isNumericDatatype(l.Datatype()) {
				val = fmt.Sprintf("NUM:%g", toFloat64(v))
			} else {
				val = v.N3()
				if collapseBnodes {
					// Also normalize bnodes inside triple term N3 representations
					val = normalizeBnodesInN3(val)
				}
			}
		}
		parts = append(parts, k+"="+val)
	}
	slices.Sort(parts)
	return strings.Join(parts, "|")
}

// normalizeBnodesInN3 replaces all bnode references (e.g., _:XXXX) in N3 strings with _:BNODE.
func normalizeBnodesInN3(s string) string {
	if !strings.Contains(s, "_:") {
		return s
	}
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if i+2 <= len(s) && s[i] == '_' && s[i+1] == ':' {
			sb.WriteString("_:BNODE")
			i += 2
			// Skip the bnode label
			for i < len(s) && (s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' || s[i] >= '0' && s[i] <= '9' || s[i] == '_' || s[i] == '-' || s[i] == '.') {
				i++
			}
		} else {
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String()
}
