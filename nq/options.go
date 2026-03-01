package nq

// Option configures N-Quads parsing or serialization.
type Option func(*config)

type config struct {
	base        string
	quadHandler QuadHandler
}

// WithBase sets the base IRI for resolving relative IRIs.
func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}

// WithQuadHandler sets a callback that receives the graph context for each parsed quad.
// The graph term is nil for triples without an explicit graph context.
func WithQuadHandler(h QuadHandler) Option {
	return func(c *config) { c.quadHandler = h }
}
