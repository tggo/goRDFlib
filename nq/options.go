package nq

type config struct {
	base string
}

type Option func(*config)

func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}
