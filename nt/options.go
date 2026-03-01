package nt

// Option configures N-Triples parsing or serialization.
type Option func(*config)

type config struct {
	base string
}

// WithBase sets the base IRI for resolving relative IRIs.
func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}
