package jsonld

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
)

// resolveEmptyFragmentVocab works around a json-gold v0.8.0 bug (still present
// on its master branch as of 2026-09-14; no upstream issue yet) with a relative
// @vocab that ends in an empty fragment, rdflib #2167.
//
// JSON-LD 1.1 (§4.1.4, and Context Processing step 5.9.3 of the API) resolves a
// relative @vocab against the base IRI, so "@vocab": "#" with the base
// http://example/document is http://example/document#, and the term name
// expands to http://example/document#name. json-gold resolves with net/url,
// whose URL.String drops an empty fragment, so the vocabulary mapping becomes
// http://example/document and the term expands to
// http://example/documentname — a different IRI, with no error.
//
// Before the document reaches json-gold, every context in it is walked with the
// base json-gold would use at that point (WithBase, then each @base in order,
// scoped to the node object that declares it), and a relative @vocab ending in
// "#" is replaced by its resolved absolute form. Everything else is left for
// json-gold: a @vocab that is absolute, a compact IRI, a blank node, a term
// defined in the same context, or relative without a base. Remote contexts are
// not fetched, so a @vocab they carry is not corrected.
//
// doc is modified in place; it is the caller's freshly decoded copy.
func resolveEmptyFragmentVocab(doc any, base string) {
	walkVocab(doc, base)
}

// fixExpandContextVocab applies the same correction to a WithExpandContext
// value and returns the base in effect after it, which is where the document's
// own contexts start. The caller's value is never modified: when it holds a
// @vocab it is copied through JSON first.
func fixExpandContextVocab(ec any, base string) (any, string) {
	if ec == nil {
		return nil, base
	}
	raw, err := json.Marshal(ec)
	if err != nil {
		return ec, base
	}
	var cp any
	if err := json.Unmarshal(raw, &cp); err != nil {
		return ec, base
	}
	ctx := cp
	if m, ok := cp.(map[string]any); ok {
		if inner, ok := m["@context"]; ok {
			ctx = inner // a whole document is unwrapped, as WithExpandContext does
		}
	}
	// The copy is walked either way, because an @base in the expand context
	// moves the base the document starts from; it replaces the caller's value
	// only when there is a @vocab it may have corrected.
	base = fixContext(ctx, base)
	if !bytes.Contains(raw, []byte(`"@vocab"`)) {
		return ec, base
	}
	return cp, base
}

func walkVocab(v any, base string) {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			walkVocab(item, base)
		}
	case map[string]any:
		if ctx, ok := t["@context"]; ok {
			base = fixContext(ctx, base)
		}
		for k, item := range t {
			if k != "@context" {
				walkVocab(item, base)
			}
		}
	}
}

// fixContext rewrites the @vocab values of a context (a map, an array of maps
// and IRIs, or an IRI) and returns the base in effect after it.
func fixContext(ctx any, base string) string {
	switch c := ctx.(type) {
	case []any:
		for _, item := range c {
			base = fixContext(item, base)
		}
	case map[string]any:
		if b, ok := c["@base"]; ok {
			switch bs := b.(type) {
			case nil:
				base = ""
			case string:
				base = resolveIRI(base, bs)
			}
		}
		if vs, ok := c["@vocab"].(string); ok && base != "" && strings.HasSuffix(vs, "#") && isRelativeRef(vs) {
			if _, isTerm := c[vs]; !isTerm {
				c["@vocab"] = resolveIRI(base, vs)
			}
		}
		// Scoped contexts inside term definitions see the same base.
		for k, def := range c {
			if m, ok := def.(map[string]any); ok && !strings.HasPrefix(k, "@") {
				if scoped, ok := m["@context"]; ok {
					fixContext(scoped, base)
				}
			}
		}
	}
	return base
}

// isRelativeRef reports whether s is a relative IRI reference, as opposed to
// an absolute IRI, a compact IRI or a blank node identifier.
func isRelativeRef(s string) bool {
	if strings.HasPrefix(s, "_:") || strings.Contains(s, ":") {
		return false
	}
	return true
}

// resolveIRI resolves ref against base per RFC 3986 and, unlike
// url.URL.String, keeps an empty fragment. It returns ref unchanged when
// either does not parse or base is not absolute (json-gold then reports it).
func resolveIRI(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil || !b.IsAbs() {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	out := b.ResolveReference(r).String()
	if strings.HasSuffix(ref, "#") && !strings.HasSuffix(out, "#") {
		out += "#"
	}
	return out
}
