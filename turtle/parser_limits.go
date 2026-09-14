package turtle

import "errors"

// DefaultMaxParseDepth is how deeply the parser lets blank node property lists
// ([ ... ]), collections (( ... )), triple terms and reified triples (<< ... >>)
// and annotation blocks ({| ... |}) nest inside one another. See
// WithMaxParseDepth.
const DefaultMaxParseDepth = 10000

// ErrNestingTooDeep is reported (wrapped, with the line number) when the input
// nests deeper than the parser's depth limit. Detect it with errors.Is and raise
// the limit with WithMaxParseDepth if the document is trusted and genuinely
// nests that deeply.
var ErrNestingTooDeep = errors.New("turtle: nesting too deep")

// WithMaxParseDepth bounds how deeply nested constructs ([ ], ( ), << >>,
// {| |}) may nest while parsing. The default is DefaultMaxParseDepth; values
// <= 0 select it.
//
// The parser is recursive, so without a bound an input such as "[ :p [ :p ..."
// nested a few million levels deep — a few megabytes of untrusted bytes —
// exhausts the goroutine stack, and Go reports a stack overflow as a fatal
// error that takes the whole process down rather than as a recoverable panic.
// Past the limit the parse fails with ErrNestingTooDeep instead.
func WithMaxParseDepth(depth int) Option {
	return func(c *config) { c.maxParseDepth = depth }
}

// maxDepth returns the resolved nesting limit.
func (c *config) maxDepth() int {
	if c.maxParseDepth <= 0 {
		return DefaultMaxParseDepth
	}
	return c.maxParseDepth
}

// enter records one more level of nesting and fails once the limit is passed.
// Every call that returns nil must be paired with leave.
func (p *turtleParser) enter() error {
	limit := p.maxDepth
	if limit <= 0 {
		limit = DefaultMaxParseDepth
	}
	if p.depth >= limit {
		return p.errorf("%w: more than %d nested levels; raise the limit with WithMaxParseDepth", ErrNestingTooDeep, limit)
	}
	p.depth++
	return nil
}

// leave undoes enter.
func (p *turtleParser) leave() { p.depth-- }
