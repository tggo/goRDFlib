package jsonld

import (
	"bytes"
	"encoding/json"
	"sort"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/term"

	"github.com/piprate/json-gold/ld"
)

// ProvenanceHandler is called for each triple as it is added to the graph, with
// the 1-based line of the JSON node object the triple's subject was declared in.
//
// # Precision, and why it is the node and not the triple
//
// Every other parser here reports the line of the triple itself. JSON-LD cannot:
// the triples come out of an expansion performed by
// [github.com/piprate/json-gold], which resolves contexts, applies term
// definitions and produces a dataset with no memory of where in the source any
// of it came from. piprate/json-gold#96 asks for that and is still open.
//
// What can be recovered from the source without reimplementing JSON-LD is where
// each `@id` was written. So a triple is reported against the line its subject's
// node object begins on — the precision shacl.SourceLineFocusNode gives, and for
// JSON-LD arguably the natural unit, since a node object is a block a reader
// navigates to as a whole.
//
// # It never guesses
//
// Identifiers written in the source are expanded through the document's own
// `@context`, using the same processor that produced the triples, so a match is
// a match and not a resemblance. Anything that fails to expand, or expands to
// something no triple mentions, contributes nothing. The handler is therefore
// not called for:
//
//   - a node object with no `@id`, which becomes a blank node whose label the
//     expander invents — nothing in the source names it;
//   - an identifier introduced by a scoped or remote context, since only the
//     document's top-level `@context` is used to expand;
//   - a subject that never appears as the `@id` of its own node object.
//
// Silence is the right answer there: a wrong line is worse than no line. Use
// provenance.Index.Len to see how much was recovered.
//
// # Aliased @id
//
// JSON-LD lets a context alias `@id` to another term (`{"id": "@id"}`). Aliases
// are honoured: every `@context` in the document is scanned for terms defined as
// the plain string "@id", or as an object whose "@id" is "@id". Aliases that
// arrive through a remote context are not seen, because the scan reads only the
// bytes the caller supplied.
type ProvenanceHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, lineNum int)

// WithProvenance sets a callback invoked for each triple with the 1-based line
// of the JSON node object its subject was declared in. See ProvenanceHandler for
// what that does and does not cover. When unset there is zero overhead: the
// source is not read a second time and no scan runs.
func WithProvenance(h ProvenanceHandler) Option {
	return func(c *config) { c.provenance = h }
}

// subjectLines maps a subject, keyed by term.TermKey, to the line its node
// object begins on.
type subjectLines map[string]int

// buildSubjectLines finds where each subject was declared in the source.
//
// Two things have to come together: the position scan, which knows the `@id`
// strings exactly as written and where they were written, and the document's
// context, which knows what those strings expand to. Neither alone can match a
// triple.
func buildSubjectLines(src []byte, base string, loader ld.DocumentLoader, expandCtx any) subjectLines {
	raw := scanIDPositions(src, expandCtx)
	if len(raw) == 0 {
		return nil
	}

	expand := iriExpander(src, base, loader, expandCtx)
	out := make(subjectLines, len(raw))
	for written, line := range raw {
		// A blank node identifier is not an IRI and is never expanded. The
		// expander may relabel it, in which case nothing matches and the node
		// simply gets no line.
		if len(written) > 2 && written[0] == '_' && written[1] == ':' {
			out[term.TermKey(term.NewBNode(written[2:]))] = line
			continue
		}
		iri, ok := expand(written)
		if !ok || iri == "" {
			continue
		}
		key := term.TermKey(term.NewURIRefUnsafe(iri))
		if prev, seen := out[key]; !seen || line < prev {
			out[key] = line
		}
	}
	return out
}

// iriExpander returns a function that expands an identifier the way the JSON-LD
// processor does, using the caller's expand context (WithExpandContext) and then
// the document's own top-level context, in that order — the same order the
// processor applies them, so the document's definitions take precedence.
//
// Using the processor's own context rather than a hand-rolled prefix table is
// what makes this exact instead of approximate: a compact IRI, a relative IRI,
// a term with an @id mapping and an absolute IRI all go through the same code
// that produced the triples. When no context can be built, expansion falls back
// to resolving against the base, which still handles the plain absolute and
// relative cases.
func iriExpander(src []byte, base string, loader ld.DocumentLoader, expandCtx any) func(string) (string, bool) {
	opts := ld.NewJsonLdOptions(base)
	if loader != nil {
		opts.DocumentLoader = loader
	}

	active := ld.NewContext(nil, opts)
	if expandCtx = unwrapContext(expandCtx); expandCtx != nil {
		// A failed parse here also fails Parse itself, so what the fallback
		// yields is never reported: Parse returns before the scan is used.
		if parsed, err := active.Parse(expandCtx); err == nil {
			active = parsed
		}
	}
	if local := topLevelContext(src); local != nil {
		if parsed, err := active.Parse(local); err == nil {
			active = parsed
		}
	}

	return func(written string) (string, bool) {
		// relative=true, vocab=false: an @id is resolved against the base, not
		// against @vocab, which is what the expansion algorithm specifies for
		// the @id position.
		iri, err := active.ExpandIri(written, true, false, nil, nil)
		if err != nil {
			// Defensive: the processor reports an error for a handful of
			// keyword situations an @id cannot be in. Declining here means the
			// node gets no line, which is the safe direction.
			return "", false
		}
		return iri, true
	}
}

// unwrapContext accepts an expand context given either as a context value or as
// a whole document carrying a "@context" key, the way the processor does.
func unwrapContext(ctx any) any {
	if doc, ok := ctx.(map[string]any); ok {
		if inner, ok := doc["@context"]; ok {
			return inner
		}
	}
	return ctx
}

// topLevelContext returns the document's outermost `@context` value, or nil.
func topLevelContext(src []byte) any {
	var doc any
	if err := json.Unmarshal(src, &doc); err != nil {
		return nil
	}
	// A document may be an array of node objects; the context, if any, is then
	// on the first one that carries it.
	switch v := doc.(type) {
	case map[string]any:
		return v["@context"]
	case []any:
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if ctx, ok := obj["@context"]; ok {
					return ctx
				}
			}
		}
	}
	return nil
}

// scanIDPositions walks the raw JSON and records where each `@id` value was
// written, keyed by the value exactly as it appears in the source.
//
// It is a token scan rather than a parse into a tree, because encoding/json's
// Decoder reports an input offset per token and that offset is precisely the
// position a decoded value throws away.
//
// Two passes are needed: the first collects the terms that alias `@id`, because
// a context may be declared after the node objects that use it — key order in a
// JSON object carries no meaning. An alias may also come from the caller's
// expand context, which the document never mentions.
func scanIDPositions(src []byte, expandCtx any) map[string]int {
	aliases := scanIDAliases(src, expandCtx)
	positions := make(map[string]int)

	lines := newLineIndex(src)
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()

	// The line each currently-open object started on. An `@id` is reported
	// against the start of its node object rather than against the `@id` key,
	// so a reader is sent to the top of the block instead of into its middle.
	var objectStart []int
	var pendingKey string
	expectKey := true

	for {
		tok, err := dec.Token()
		if err != nil {
			// A truncated or malformed document simply yields fewer positions.
			// Parse reports the syntax error; this scan must not.
			return positions
		}
		// The offset has to be read *after* the token. Before it, the decoder
		// still sits where the previous token ended, which is on the far side
		// of the whitespace and so usually on the previous line.
		startLine := lines.at(dec.InputOffset())

		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				objectStart = append(objectStart, startLine)
				expectKey = true
			case '}':
				if len(objectStart) > 0 {
					objectStart = objectStart[:len(objectStart)-1]
				}
				expectKey = true
			case '[':
				expectKey = false
			case ']':
				expectKey = len(objectStart) > 0
			}
			pendingKey = ""

		case string:
			if expectKey && len(objectStart) > 0 && pendingKey == "" {
				pendingKey = t
				expectKey = false
				continue
			}
			if len(objectStart) > 0 && (pendingKey == "@id" || aliases[pendingKey]) {
				// Keep the first: a document may state the same node twice, and
				// the earlier block is the one a reader calls its definition.
				if _, seen := positions[t]; !seen {
					positions[t] = objectStart[len(objectStart)-1]
				}
			}
			pendingKey = ""
			expectKey = len(objectStart) > 0

		default:
			pendingKey = ""
			expectKey = len(objectStart) > 0
		}
	}
}

// scanIDAliases collects terms that a `@context` in this document, or the
// caller's inline expand context, defines as an alias for `@id`. An expand
// context given as an IRI is not fetched, so aliases it declares are not seen.
//
// It decodes into a generic tree rather than scanning tokens, because an alias
// can be nested arbitrarily deep and here the structure matters, not the
// position.
func scanIDAliases(src []byte, expandCtx any) map[string]bool {
	aliases := map[string]bool{}
	if ctx := unwrapContext(expandCtx); ctx != nil {
		collectAliasesFromContext(ctx, aliases)
	}
	var doc any
	if err := json.Unmarshal(src, &doc); err != nil {
		if len(aliases) == 0 {
			return nil
		}
		return aliases
	}
	collectIDAliases(doc, aliases)
	if len(aliases) == 0 {
		return nil
	}
	return aliases
}

func collectIDAliases(node any, out map[string]bool) {
	switch v := node.(type) {
	case map[string]any:
		if ctx, ok := v["@context"]; ok {
			collectAliasesFromContext(ctx, out)
		}
		for k, child := range v {
			if k == "@context" {
				continue
			}
			collectIDAliases(child, out)
		}
	case []any:
		for _, child := range v {
			collectIDAliases(child, out)
		}
	}
}

func collectAliasesFromContext(ctx any, out map[string]bool) {
	switch v := ctx.(type) {
	case map[string]any:
		for t, def := range v {
			switch d := def.(type) {
			case string:
				if d == "@id" {
					out[t] = true
				}
			case map[string]any:
				if id, ok := d["@id"].(string); ok && id == "@id" {
					out[t] = true
				}
			}
		}
	case []any:
		for _, item := range v {
			collectAliasesFromContext(item, out)
		}
	}
}

// lineIndex answers "which line is this byte offset on" in logarithmic time.
//
// The scan asks once per token, and counting newlines from the start each time
// would make it quadratic in the size of the document — which for JSON-LD, a
// format people generate by the megabyte, is not a theoretical concern.
type lineIndex struct {
	newlines []int64 // offsets of each '\n'
}

func newLineIndex(src []byte) *lineIndex {
	var nl []int64
	for i, b := range src {
		if b == '\n' {
			nl = append(nl, int64(i))
		}
	}
	return &lineIndex{newlines: nl}
}

// at returns the 1-based line containing offset.
func (l *lineIndex) at(offset int64) int {
	n := sort.Search(len(l.newlines), func(i int) bool { return l.newlines[i] >= offset })
	return n + 1
}
