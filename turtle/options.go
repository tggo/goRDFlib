package turtle

import rdflibgo "github.com/tggo/goRDFlib"

const (
	// defaultIndentWidth is the indentation step used by WithPretty.
	defaultIndentWidth = 4
	// maxIndentWidth caps WithIndent so a nested document cannot be blown up
	// by an absurd indentation step.
	maxIndentWidth = 16
	// defaultMaxNestDepth bounds how deeply blank nodes and collections are
	// nested inline. See WithMaxNestDepth.
	defaultMaxNestDepth = 64
)

type config struct {
	base                 string
	provenance           ProvenanceHandler
	preserveBlankNodeIDs bool

	pretty       bool
	indentWidth  int // 0 means "unset"; resolved by indentUnit
	maxNestDepth int // 0 means "unset"; resolved by nestDepth
}

// indentUnit returns the resolved indentation step for one nesting level.
func (c *config) indentUnit() string {
	w := c.indentWidth
	if w == 0 {
		w = defaultIndentWidth
	}
	if w < 0 {
		w = 0
	}
	if w > maxIndentWidth {
		w = maxIndentWidth
	}
	return spaces(w)
}

// nestDepth returns the resolved inline nesting limit.
func (c *config) nestDepth() int {
	if c.maxNestDepth <= 0 {
		return defaultMaxNestDepth
	}
	return c.maxNestDepth
}

func spaces(n int) string {
	const pad = "                " // maxIndentWidth
	return pad[:n]
}

// Option configures turtle parsing or serialization.
type Option func(*config)

// ProvenanceHandler is called for each triple as it is added to the graph, with
// the 1-based source line number the triple was emitted from. Use it to map
// triples back to their origin in the input (e.g. so a SHACL report can cite a
// line number). The handler must not be nil when passed to WithProvenance.
//
// Unlike line-oriented N-Triples, a Turtle statement can span multiple lines and
// a single line can expand to many triples (collections, blank-node property
// lists). The reported line is the parser's position at the point the triple is
// emitted, which is the best available approximation of its origin.
type ProvenanceHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, lineNum int)

// WithPreserveBlankNodeIDs keeps labelled blank node IDs exactly as written in
// the input. By default, each Parse call gives labels a fresh document scope.
// Use this option only when IDs must be stable: separate documents with the
// same label can then merge nodes. Anonymous blank nodes remain fresh.
func WithPreserveBlankNodeIDs() Option {
	return func(c *config) { c.preserveBlankNodeIDs = true }
}

// WithBase sets the base IRI for resolving relative IRIs.
func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}

// WithProvenance sets a callback invoked for each triple with its 1-based source
// line number. See ProvenanceHandler for semantics. When unset there is zero
// per-triple overhead.
func WithProvenance(h ProvenanceHandler) Option {
	return func(c *config) { c.provenance = h }
}

// WithPretty makes Serialize lay the document out over multiple lines: every
// subject starts a line of its own, each predicate is indented one level below
// its subject, and blank-node property lists are expanded into indented blocks.
// The default (without this option) is the compact single-statement-per-subject
// layout.
//
// Both layouts denote the same graph — whitespace is not significant in Turtle —
// so a pretty-printed document round-trips through Parse unchanged.
//
// The indentation step is 4 spaces; use WithIndent to change it.
func WithPretty() Option {
	return func(c *config) { c.pretty = true }
}

// WithIndent enables pretty printing (see WithPretty) with width spaces per
// nesting level. Negative widths are treated as 0 and widths above 16 are
// clamped to 16, so a deeply nested document cannot be inflated without bound.
func WithIndent(width int) Option {
	return func(c *config) {
		c.pretty = true
		if width == 0 {
			// 0 is the "unset" sentinel; represent "no indentation" as -1 so it
			// survives resolution in indentUnit.
			width = -1
		}
		c.indentWidth = width
	}
}

// WithMaxNestDepth bounds how deeply blank nodes and collections are nested
// inline, in both the compact and the pretty layout. The default is 64.
//
// The limit exists because inline nesting is recursive: a graph built from a
// long chain of blank nodes (which an untrusted input can produce) would
// otherwise recurse once per link and grow the indentation of every following
// line without bound. Beyond the limit a blank node is written as a bare label
// (_:b) and its own statement is emitted at the top level instead, so no triple
// is ever lost — only the nesting is flattened. Each level costs two
// indentation steps, so the widest line is bounded by (2*depth+1) steps.
//
// Values <= 0 select the default. Raise it if your data genuinely nests deeper
// and you want it inlined.
func WithMaxNestDepth(depth int) Option {
	return func(c *config) { c.maxNestDepth = depth }
}
