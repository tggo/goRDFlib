// Package rdfloader provides a Loader that fetches RDF data from URIs
// and parses it into graphs. Supports http(s):// and file:// schemes.
// Format is auto-detected via Content-Type, file extension, or content sniffing.
//
// Not safe for concurrent use.
package rdfloader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/plugin"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/trig"
	"github.com/tggo/goRDFlib/turtle"
)

// Option configures a defaultLoader.
type Option func(*defaultLoader)

// WithHTTPClient sets a custom HTTP client for http(s):// requests.
func WithHTTPClient(c *http.Client) Option {
	return func(l *defaultLoader) { l.client = c }
}

// WithTimeout sets a timeout for the default HTTP client.
// Ignored if WithHTTPClient is also provided.
func WithTimeout(d time.Duration) Option {
	return func(l *defaultLoader) { l.timeout = d }
}

// WithMaxLineLength raises the per-line byte cap of the line-based parsers
// (N-Triples, N-Quads) for inputs that pack large literals onto one line.
// It has no effect on Turtle, TriG, RDF/XML or JSON-LD, which are not line-based.
func WithMaxLineLength(n int) Option {
	return func(l *defaultLoader) { l.maxLineLen = n }
}

// WithUnboundedLines removes the per-line byte cap of the line-based parsers
// (N-Triples, N-Quads). Use it when the longest line cannot be bounded ahead of
// time; see nq.WithUnboundedLines for the memory trade-off. No effect on
// Turtle, TriG, RDF/XML or JSON-LD.
func WithUnboundedLines() Option {
	return func(l *defaultLoader) { l.unbounded = true }
}

// WithMaxParseDepth sets how deeply Turtle and TriG documents may nest blank
// node property lists, collections and triple terms before the parse fails
// with turtle.ErrNestingTooDeep / trig.ErrNestingTooDeep. The default is
// turtle.DefaultMaxParseDepth. No effect on the other formats.
func WithMaxParseDepth(depth int) Option {
	return func(l *defaultLoader) { l.maxParseDepth = depth }
}

// WithPreserveBlankNodeIDs disables the fresh blank-node scope for each load.
// It forwards the compatibility option to all six parsers. Separate documents
// can then share blank-node IDs; callers must manage that identity themselves.
// For JSON-LD, this preserves json-gold expansion IDs, not source labels.
func WithPreserveBlankNodeIDs() Option {
	return func(l *defaultLoader) { l.preserveBlankNodeIDs = true }
}

type defaultLoader struct {
	client               *http.Client
	timeout              time.Duration
	maxLineLen           int
	unbounded            bool
	preserveBlankNodeIDs bool
	maxParseDepth        int
}

// DefaultLoader returns a Loader that handles file:// and http(s):// URIs.
// Format is auto-detected from Content-Type header, file extension, or content sniffing.
// Each load uses a fresh blank-node scope unless WithPreserveBlankNodeIDs is set.
func DefaultLoader(opts ...Option) *defaultLoader {
	l := &defaultLoader{timeout: 30 * time.Second}
	for _, o := range opts {
		o(l)
	}
	return l
}

func (l *defaultLoader) httpClient() *http.Client {
	if l.client != nil {
		return l.client
	}
	return &http.Client{Timeout: l.timeout}
}

// Load fetches RDF data from uri and parses it into g.
func (l *defaultLoader) Load(ctx context.Context, g *graph.Graph, uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("rdfloader: invalid URI %q: %w", uri, err)
	}

	switch u.Scheme {
	case "file", "":
		return l.loadFile(g, u.Path)
	case "http", "https":
		return l.loadHTTP(ctx, g, uri)
	default:
		return fmt.Errorf("rdfloader: unsupported URI scheme %q", u.Scheme)
	}
}

func (l *defaultLoader) loadFile(g *graph.Graph, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("rdfloader: %w", err)
	}
	defer f.Close()

	format, ok := plugin.FormatFromFilename(path)
	if !ok {
		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		if n == 0 {
			return fmt.Errorf("rdfloader: empty file %q", path)
		}
		format, ok = plugin.FormatFromContent(buf[:n])
		if !ok {
			return fmt.Errorf("rdfloader: unable to detect format for %q", path)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("rdfloader: %w", err)
		}
	}

	return l.parseFormat(g, f, format)
}

func (l *defaultLoader) loadHTTP(ctx context.Context, g *graph.Graph, uri string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return fmt.Errorf("rdfloader: %w", err)
	}
	req.Header.Set("Accept", "text/turtle, application/rdf+xml, application/n-triples, application/n-quads, application/trig, application/ld+json, */*;q=0.1")

	resp, err := l.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("rdfloader: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rdfloader: HTTP %d for %q", resp.StatusCode, uri)
	}

	// Try Content-Type header
	ct := resp.Header.Get("Content-Type")
	if format, ok := plugin.FormatFromMIME(ct); ok {
		return l.parseFormat(g, resp.Body, format)
	}

	// Try URL path extension
	u, _ := url.Parse(uri)
	if format, ok := plugin.FormatFromFilename(u.Path); ok {
		return l.parseFormat(g, resp.Body, format)
	}

	// Content sniffing: buffer prefix, detect, then replay with remaining body
	buf := make([]byte, 512)
	n, _ := io.ReadAtLeast(resp.Body, buf, 1)
	if n == 0 {
		return fmt.Errorf("rdfloader: empty response from %q", uri)
	}
	format, ok := plugin.FormatFromContent(buf[:n])
	if !ok {
		return fmt.Errorf("rdfloader: unable to detect format from %q", uri)
	}

	combined := io.MultiReader(bytes.NewReader(buf[:n]), resp.Body)
	return l.parseFormat(g, combined, format)
}

// parseFormat dispatches to the appropriate parser by format name, forwarding
// the line-length options to the line-based parsers (N-Triples, N-Quads) and
// the nesting limit to Turtle and TriG.
// The blank-node identity option is forwarded to every format.
func (l *defaultLoader) parseFormat(g *graph.Graph, r io.Reader, format string) error {
	switch format {
	case "turtle":
		var opts []turtle.Option
		if l.preserveBlankNodeIDs {
			opts = append(opts, turtle.WithPreserveBlankNodeIDs())
		}
		if l.maxParseDepth > 0 {
			opts = append(opts, turtle.WithMaxParseDepth(l.maxParseDepth))
		}
		return turtle.Parse(g, r, opts...)
	case "nt":
		return nt.Parse(g, r, l.ntOpts()...)
	case "nquads":
		return nq.Parse(g, r, l.nqOpts()...)
	case "trig":
		var opts []trig.Option
		if l.preserveBlankNodeIDs {
			opts = append(opts, trig.WithPreserveBlankNodeIDs())
		}
		if l.maxParseDepth > 0 {
			opts = append(opts, trig.WithMaxParseDepth(l.maxParseDepth))
		}
		return trig.Parse(g, r, opts...)
	case "xml":
		var opts []rdfxml.Option
		if l.preserveBlankNodeIDs {
			opts = append(opts, rdfxml.WithPreserveBlankNodeIDs())
		}
		return rdfxml.Parse(g, r, opts...)
	case "json-ld":
		var opts []jsonld.Option
		if l.preserveBlankNodeIDs {
			opts = append(opts, jsonld.WithPreserveBlankNodeIDs())
		}
		return jsonld.Parse(g, r, opts...)
	default:
		return fmt.Errorf("rdfloader: unsupported format %q", format)
	}
}

// ntOpts and nqOpts translate the loader's line-length and blank-node settings
// into parser options. They return nil when nothing is configured, preserving
// the parser defaults.
func (l *defaultLoader) ntOpts() []nt.Option {
	var opts []nt.Option
	if l.preserveBlankNodeIDs {
		opts = append(opts, nt.WithPreserveBlankNodeIDs())
	}
	if l.unbounded {
		opts = append(opts, nt.WithUnboundedLines())
	} else if l.maxLineLen > 0 {
		opts = append(opts, nt.WithMaxLineLength(l.maxLineLen))
	}
	return opts
}

func (l *defaultLoader) nqOpts() []nq.Option {
	var opts []nq.Option
	if l.preserveBlankNodeIDs {
		opts = append(opts, nq.WithPreserveBlankNodeIDs())
	}
	if l.unbounded {
		opts = append(opts, nq.WithUnboundedLines())
	} else if l.maxLineLen > 0 {
		opts = append(opts, nq.WithMaxLineLength(l.maxLineLen))
	}
	return opts
}
