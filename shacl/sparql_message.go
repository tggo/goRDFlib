package shacl

import (
	"strings"
	"unicode"
)

// resultMessages computes sh:resultMessage for one result of a SPARQL-based
// constraint or constraint component, per SHACL §5.3.2 (Mapping of Solution
// Bindings to Result Properties). The rules run top to bottom and the first
// that yields a value wins:
//
//  1. the binding of ?message in the solution, used as is;
//  2. the first non-empty list in candidates, with {?varName} and {$varName}
//     replaced from vars(). vars is called only when a template has a
//     placeholder, so results whose messages are plain text build no map.
//
// When neither yields a value it returns nil and the caller keeps whatever
// makeResult already set from the shape's own sh:message.
//
// The ?message binding is deliberately not treated as a template: it comes
// from the data, and the spec only calls sh:message literals templates.
func resultMessages(row map[string]Term, vars func() map[string]Term, candidates ...[]Term) []Term {
	if m, ok := row["message"]; ok && !m.IsNone() {
		return []Term{m}
	}
	for _, msgs := range candidates {
		if len(msgs) == 0 {
			continue
		}
		var bound map[string]Term
		out := make([]Term, len(msgs))
		for i, m := range msgs {
			if !isTemplate(m) {
				out[i] = m
				continue
			}
			if bound == nil {
				bound = vars()
			}
			out[i] = expandMessage(m, bound)
		}
		return out
	}
	return nil
}

// messageVars collects the variables a message template may reference: the
// pre-bound this/value/path/currentShape, the constraint component's
// parameters by local name, and then the solution's own bindings, which win
// over the pre-bound ones.
func messageVars(focusNode, value, path, currentShape Term, params map[string]Term, row map[string]Term) map[string]Term {
	vars := make(map[string]Term, 4+len(params)+len(row))
	setBound(vars, "this", focusNode)
	setBound(vars, "value", value)
	setBound(vars, "path", path)
	setBound(vars, "currentShape", currentShape)
	for name, t := range params {
		vars[name] = t
	}
	for name, t := range row {
		setBound(vars, name, t)
	}
	return vars
}

func setBound(vars map[string]Term, name string, t Term) {
	if !t.IsNone() {
		vars[name] = t
	}
}

// isTemplate reports whether a message literal may contain a placeholder.
func isTemplate(msg Term) bool {
	return msg.IsLiteral() && strings.Contains(msg.value, "{")
}

// expandMessage replaces {?varName} and {$varName} in a message literal,
// keeping its language tag and datatype. A non-literal message is returned
// unchanged.
func expandMessage(msg Term, vars map[string]Term) Term {
	if !isTemplate(msg) {
		return msg
	}
	expanded := msg
	expanded.value = expandTemplate(msg.value, vars)
	return expanded
}

// expandTemplate substitutes in a single left-to-right pass, so a value that
// itself contains "{?x}" is emitted literally rather than expanded again — a
// data value must not be able to pull other bindings into the message. A
// placeholder whose variable is unbound is left as written, which makes a
// misspelt name visible instead of silently turning it into an empty string.
func expandTemplate(tmpl string, vars map[string]Term) string {
	var b strings.Builder
	b.Grow(len(tmpl))
	for i := 0; i < len(tmpl); {
		if name, end, ok := placeholderAt(tmpl, i); ok {
			if t, bound := vars[name]; bound {
				b.WriteString(messageString(t))
				i = end
				continue
			}
		}
		b.WriteByte(tmpl[i])
		i++
	}
	return b.String()
}

// placeholderAt reports whether a {?name} or {$name} placeholder starts at
// s[i], returning the variable name and the index just past the closing brace.
// Names follow SPARQL's VARNAME: letters, digits, underscore, combining marks,
// U+00B7 and U+203F–U+2040.
func placeholderAt(s string, i int) (name string, end int, ok bool) {
	if i+1 >= len(s) || s[i] != '{' || (s[i+1] != '?' && s[i+1] != '$') {
		return "", 0, false
	}
	start := i + 2
	for j, r := range s[start:] {
		if r == '}' {
			if j == 0 {
				return "", 0, false
			}
			return s[start : start+j], start + j + 1, true
		}
		if !isVarNameRune(r) {
			return "", 0, false
		}
	}
	return "", 0, false
}

func isVarNameRune(r rune) bool {
	return r == '_' || r == '\u00B7' || r == '\u203F' || r == '\u2040' ||
		unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

// messageString is the "suitable string representation" §5.3.2 asks for: the
// IRI itself, a literal's lexical form, a blank node as _:label. It matches
// what pySHACL substitutes.
func messageString(t Term) string {
	if t.IsBlank() {
		return "_:" + t.value
	}
	return t.value
}
