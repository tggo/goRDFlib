package nt

import rdflibgo "github.com/tggo/goRDFlib"

// Option configures N-Triples parsing or serialization.
type Option func(*config)

// ErrorHandler is called when a line fails to parse.
// It receives the 1-based line number, the raw line text, and the parse error.
// If retry is true, fixedLine is parsed instead (exactly one retry attempt).
// If retry is false, the line is skipped and parsing continues.
// To preserve the default fail-fast behavior, do not set an error handler.
type ErrorHandler func(lineNum int, line string, err error) (fixedLine string, retry bool)

// ProvenanceHandler is called for each successfully parsed triple with the
// 1-based source line number it came from. Use it to map triples back to their
// origin in the input (e.g. so a SHACL report can cite a line number). It is
// invoked after the triple is added to the graph / dispatched to the stream
// handler. The handler must not be nil when passed to WithProvenance.
//
// A single source line yields exactly one triple in N-Triples, so the mapping is
// one-to-one. Note that the graph deduplicates triples, so if the same triple
// appears on multiple lines the handler is still called once per occurrence —
// the caller decides how to reconcile duplicates.
type ProvenanceHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, lineNum int)

type config struct {
	base         string
	errorHandler ErrorHandler
	provenance   ProvenanceHandler
	maxLineLen   int
	unbounded    bool
}

// WithBase sets the base IRI for resolving relative IRIs.
func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}

// WithErrorHandler sets a callback invoked when a line fails to parse.
// See ErrorHandler for semantics.
func WithErrorHandler(h ErrorHandler) Option {
	return func(c *config) { c.errorHandler = h }
}

// WithProvenance sets a callback invoked for each successfully parsed triple
// with its 1-based source line number. See ProvenanceHandler for semantics.
// When unset there is zero per-triple overhead.
func WithProvenance(h ProvenanceHandler) Option {
	return func(c *config) { c.provenance = h }
}

// WithMaxLineLength raises the maximum byte length of a single N-Triples line.
// The default (bufio.MaxScanTokenSize, 64KB) is too small for inputs that pack
// large literals — e.g. WKT polygons — onto one line. Pass a value larger than
// the longest expected line in bytes.
//
// Use this when you can bound the longest line; it keeps a fixed memory ceiling.
// When you cannot bound it, prefer WithUnboundedLines.
func WithMaxLineLength(n int) Option {
	return func(c *config) { c.maxLineLen = n }
}

// WithUnboundedLines parses lines of arbitrary length, growing the read buffer
// as needed instead of enforcing a fixed maximum. Use this when the longest line
// cannot be bounded ahead of time — e.g. N-Triples dumps mixing tiny literals
// with multi-megabyte WKT geometries — so neither the 64KB default nor a guessed
// WithMaxLineLength value fits.
//
// The trade-off is memory: the buffer may grow to hold the single longest line,
// and a malformed input with no newlines can force the whole stream into memory.
// When the maximum line size is known, WithMaxLineLength is the safer, bounded
// choice. WithUnboundedLines takes precedence over WithMaxLineLength if both are set.
func WithUnboundedLines() Option {
	return func(c *config) { c.unbounded = true }
}
