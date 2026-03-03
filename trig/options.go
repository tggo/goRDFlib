package trig

type config struct {
	base string
}

// Option configures TriG parsing or serialization.
type Option func(*config)

// WithBase sets the base IRI for resolving relative IRIs.
func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}
