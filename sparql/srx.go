package sparql

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"fmt"
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
			if b.URI != "" {
				row[b.Name] = rdflibgo.NewURIRefUnsafe(b.URI)
			} else if b.BNode != "" {
				row[b.Name] = rdflibgo.NewBNode(b.BNode)
			} else if b.Literal != nil {
				var opts []rdflibgo.LiteralOption
				if b.Literal.Lang != "" {
					opts = append(opts, rdflibgo.WithLang(b.Literal.Lang))
				} else if b.Literal.Datatype != "" {
					opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(b.Literal.Datatype)))
				}
				row[b.Name] = rdflibgo.NewLiteral(b.Literal.Value, opts...)
			}
		}
		bindings = append(bindings, row)
	}

	return &Result{Type: "SELECT", Vars: vars, Bindings: bindings}, nil
}

type srxDocument struct {
	XMLName xml.Name    `xml:"sparql"`
	Head    srxHead     `xml:"head"`
	Results srxResults  `xml:"results"`
	Boolean *bool       `xml:"boolean"`
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
	Name    string      `xml:"name,attr"`
	URI     string      `xml:"uri"`
	BNode   string      `xml:"bnode"`
	Literal *srxLiteral `xml:"literal"`
}

type srxLiteral struct {
	Value    string `xml:",chardata"`
	Lang     string `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	Datatype string `xml:"datatype,attr"`
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
			switch v.Type {
			case "uri":
				b[k] = rdflibgo.NewURIRefUnsafe(v.Value)
			case "bnode":
				b[k] = rdflibgo.NewBNode(v.Value)
			case "literal", "typed-literal":
				var opts []rdflibgo.LiteralOption
				if v.Lang != "" {
					opts = append(opts, rdflibgo.WithLang(v.Lang))
				} else if v.Datatype != "" {
					opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(v.Datatype)))
				}
				b[k] = rdflibgo.NewLiteral(v.Value, opts...)
			}
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
	Type     string `json:"type"`
	Value    string `json:"value"`
	Lang     string `json:"xml:lang,omitempty"`
	Datatype string `json:"datatype,omitempty"`
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
		aKeys[bindingKeySimple(row)]++
	}
	bKeys := make(map[string]int)
	for _, row := range b.Bindings {
		bKeys[bindingKeySimple(row)]++
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
		k := bindingKeyNoBnodes(row)
		aKeys[k] = append(aKeys[k], i)
	}
	bKeys := make(map[string][]int)
	for i, row := range b {
		k := bindingKeyNoBnodes(row)
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

func bindingKeySimple(row map[string]rdflibgo.Term) string {
	var parts []string
	for k, v := range row {
		val := ""
		if v != nil {
			if l, ok := v.(rdflibgo.Literal); ok && isNumericDatatype(l.Datatype()) {
				val = fmt.Sprintf("NUM:%g", toFloat64(v))
			} else {
				val = v.N3()
			}
		}
		parts = append(parts, k+"="+val)
	}
	sortStrings(parts)
	return strings.Join(parts, "|")
}

func bindingKeyNoBnodes(row map[string]rdflibgo.Term) string {
	var parts []string
	for k, v := range row {
		val := ""
		if v != nil {
			if _, ok := v.(rdflibgo.BNode); ok {
				val = "_:BNODE" // all bnodes collapse to same placeholder
			} else if l, ok := v.(rdflibgo.Literal); ok && isNumericDatatype(l.Datatype()) {
				val = fmt.Sprintf("NUM:%g", toFloat64(v))
			} else {
				val = v.N3()
			}
		}
		parts = append(parts, k+"="+val)
	}
	sortStrings(parts)
	return strings.Join(parts, "|")
}

func bindingKeyNorm(row map[string]rdflibgo.Term, bnodeMap map[string]string) string {
	var parts []string
	for k, v := range row {
		val := ""
		if v != nil {
			if b, ok := v.(rdflibgo.BNode); ok {
				label := b.N3()
				if norm, exists := bnodeMap[label]; exists {
					val = norm
				} else {
					norm := fmt.Sprintf("_:b%d", len(bnodeMap))
					bnodeMap[label] = norm
					val = norm
				}
			} else if l, ok := v.(rdflibgo.Literal); ok && isNumericDatatype(l.Datatype()) {
				f := toFloat64(v)
				val = fmt.Sprintf("NUM:%g", f)
			} else {
				val = v.N3()
			}
		}
		parts = append(parts, k+"="+val)
	}
	sortStrings(parts)
	return strings.Join(parts, "|")
}

func bindingKey(row map[string]rdflibgo.Term) string {
	var parts []string
	for k, v := range row {
		val := ""
		if v != nil {
			// Normalize numeric values for comparison
			if l, ok := v.(rdflibgo.Literal); ok && isNumericDatatype(l.Datatype()) {
				f := toFloat64(v)
				// Use a canonical numeric form regardless of datatype
				val = fmt.Sprintf("NUM:%g", f)
			} else {
				val = v.N3()
			}
		}
		parts = append(parts, k+"="+val)
	}
	sortStrings(parts)
	return strings.Join(parts, "|")
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
