package endpoint

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/internal/iri"
	"github.com/tggo/goRDFlib/store"
)

const gspAllow = "GET, HEAD, PUT, POST, DELETE, OPTIONS"

// gspTarget is the graph a Graph Store request addresses.
type gspTarget struct {
	isDefault bool
	// iri is the graph IRI; empty for the default graph and for the graph
	// store itself.
	iri string
	// isStore marks a request to the graph store as a whole (direct
	// identification of the mount point).
	isStore bool
}

func (t gspTarget) String() string {
	switch {
	case t.isDefault:
		return "the default graph"
	case t.isStore:
		return "the graph store"
	}
	return "<" + t.iri + ">"
}

// graphStore serves the Graph Store HTTP Protocol. direct enables direct
// graph identification.
func (x *exchange) graphStore(direct bool) error {
	target, err := x.gspTarget(direct)
	if err != nil {
		return err
	}
	switch x.r.Method {
	case http.MethodGet, http.MethodHead:
		if err := x.authorize(OpGraphRead); err != nil {
			return err
		}
		return x.gspGet(target)
	case http.MethodPut, http.MethodPost, http.MethodDelete:
		if err := x.authorize(OpGraphWrite); err != nil {
			return err
		}
		if x.h.cfg.readOnly {
			return httpErr(http.StatusForbidden, ErrReadOnly)
		}
		switch x.r.Method {
		case http.MethodPut:
			return x.gspPut(target)
		case http.MethodPost:
			return x.gspPost(target)
		default:
			return x.gspDelete(target)
		}
	case http.MethodOptions:
		x.w.Header().Set("Allow", gspAllow)
		x.w.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return methodNotAllowed(x.r.Method, gspAllow, "the Graph Store Protocol uses GET, HEAD, PUT, POST and DELETE")
	}
}

func (x *exchange) gspTarget(direct bool) (gspTarget, error) {
	params := x.r.URL.Query()
	_, isDefault := params["default"]
	graphs, isGraph := params["graph"]
	switch {
	case isDefault && isGraph:
		return gspTarget{}, httpErrf(http.StatusBadRequest, "use either ?default or ?graph=, not both")
	case isDefault:
		if v := params["default"]; len(v) != 1 || v[0] != "" {
			return gspTarget{}, httpErrf(http.StatusBadRequest, "?default takes no value")
		}
		return gspTarget{isDefault: true}, nil
	case isGraph:
		if len(graphs) != 1 {
			return gspTarget{}, httpErrf(http.StatusBadRequest, "exactly one graph= parameter is allowed, got %d", len(graphs))
		}
		g := graphs[0]
		// GSP §4.2: the embedded IRI must be absolute, or 400.
		if !iri.IsAbsolute(g) || !validIRIChars(g) {
			return gspTarget{}, httpErrf(http.StatusBadRequest,
				"graph=%q is not an absolute IRI (blank node graph names cannot be addressed)", g)
		}
		if g == store.DefaultGraphIRI {
			return gspTarget{isDefault: true}, nil
		}
		return gspTarget{iri: g}, nil
	}
	if !direct {
		return gspTarget{}, httpErrf(http.StatusBadRequest, "a Graph Store request needs ?default or ?graph=<iri>")
	}
	if p := x.r.URL.Path; p == "" || p == "/" {
		return gspTarget{isStore: true}, nil
	}
	return gspTarget{iri: x.h.requestIRI(x.r)}, nil
}

// lookup returns the addressed graph, or nil when it does not exist. The
// default graph always exists.
func (x *exchange) lookup(ctx context.Context, t gspTarget) (*graph.Graph, error) {
	if t.isDefault {
		def, _, err := x.h.backend.view(ctx)
		return def, err
	}
	g, ok, err := x.h.backend.graph(ctx, t.iri)
	if err != nil || !ok {
		return nil, err
	}
	return g, nil
}

func notFound(t gspTarget) error {
	return httpErrf(http.StatusNotFound, "graph %s does not exist", t)
}

func (x *exchange) gspGet(t gspTarget) error {
	if t.isStore {
		return httpErrf(http.StatusBadRequest, "GET on the graph store itself needs ?default or ?graph=<iri>")
	}
	x.w.Header().Add("Vary", "Accept")
	accept := x.r.Header.Get("Accept")
	f := negotiateRDF(accept)
	if f == nil {
		return notAcceptable(accept, rdfMediaTypes())
	}

	ctx, cancel := withTimeout(x.r.Context(), x.h.cfg.queryTimeout)
	defer cancel()
	var buf bytes.Buffer
	err := x.readLocked(ctx, func() error {
		g, err := x.lookup(ctx, t)
		if err != nil {
			return err
		}
		if g == nil {
			return notFound(t)
		}
		n, err := g.Len(ctx)
		if err != nil {
			return fmt.Errorf("graph %s cannot be read: %w", t, err)
		}
		x.rows = n
		if max := x.h.cfg.maxResultRows; max > 0 && n > max {
			return tooManyRows(max, "triples")
		}
		// Serialized into memory under the lock, sent after it: a slow client
		// must not hold back updates. A store failure fails the request here,
		// before the first byte, so a truncated graph is never sent.
		if err := f.serialize(ctx, g, &buf); err != nil {
			if cerr := ctx.Err(); cerr != nil {
				return err
			}
			return httpErrf(http.StatusInternalServerError,
				"graph %s cannot be written as %s: %v (if the format is the cause, request another with the Accept header)", t, f.mediaType, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	hdr := x.w.Header()
	hdr.Set("Content-Type", f.contentType)
	hdr.Set("Content-Length", strconv.Itoa(buf.Len()))
	x.w.WriteHeader(http.StatusOK)
	if x.r.Method == http.MethodHead {
		return nil
	}
	_, err = x.w.Write(buf.Bytes())
	return err
}

func (x *exchange) gspPut(t gspTarget) error {
	if t.isStore {
		return httpErrf(http.StatusBadRequest, "PUT on the graph store itself needs ?default or ?graph=<iri>")
	}
	payload, err := x.readPayload(t)
	if err != nil {
		return err
	}
	var created bool
	err = x.writeLocked(func(ctx context.Context) (err error) {
		created, err = x.replace(ctx, t, payload)
		return err
	})
	if err != nil {
		return err
	}
	x.w.Header().Set("Cache-Control", "no-store")
	if created {
		x.w.WriteHeader(http.StatusCreated)
	} else {
		x.w.WriteHeader(http.StatusNoContent)
	}
	return nil
}

// replace makes the target graph hold exactly the payload, and reports
// whether it did not exist (for the default graph: was empty) before.
func (x *exchange) replace(ctx context.Context, t gspTarget, payload *graph.Graph) (bool, error) {
	b := x.h.backend
	var g *graph.Graph
	var created bool
	if t.isDefault {
		var err error
		if g, _, err = b.view(ctx); err != nil {
			return false, err
		}
		n, err := g.Len(ctx)
		if err != nil {
			return false, fmt.Errorf("graph %s cannot be read: %w", t, err)
		}
		created = n == 0
	} else {
		_, exists, err := b.graph(ctx, t.iri)
		if err != nil {
			return false, err
		}
		created = !exists
		g = b.ensureGraph(t.iri)
	}
	// Not atomic: a store that fails between the two steps leaves the graph
	// emptied or partly filled, and the request is answered with an error.
	if err := g.Remove(ctx, nil, nil, nil); err != nil {
		return false, fmt.Errorf("graph %s cannot be cleared: %w", t, err)
	}
	if err := copyTriples(ctx, g, payload); err != nil {
		return false, fmt.Errorf("graph %s cannot be written: %w", t, err)
	}
	return created, nil
}

func (x *exchange) gspPost(t gspTarget) error {
	if t.isStore {
		iri, err := x.newGraphIRI()
		if err != nil {
			return err
		}
		t = gspTarget{iri: iri}
		payload, err := x.readPayload(t)
		if err != nil {
			return err
		}
		if err := x.writeLocked(func(ctx context.Context) error {
			if err := copyTriples(ctx, x.h.backend.ensureGraph(iri), payload); err != nil {
				return fmt.Errorf("graph %s cannot be written: %w", t, err)
			}
			return nil
		}); err != nil {
			return err
		}
		x.w.Header().Set("Location", iri)
		x.w.Header().Set("Cache-Control", "no-store")
		x.w.WriteHeader(http.StatusCreated)
		return nil
	}
	payload, err := x.readPayload(t)
	if err != nil {
		return err
	}
	created := false
	err = x.writeLocked(func(ctx context.Context) error {
		var g *graph.Graph
		if t.isDefault {
			var err error
			if g, _, err = x.h.backend.view(ctx); err != nil {
				return err
			}
		} else {
			_, exists, err := x.h.backend.graph(ctx, t.iri)
			if err != nil {
				return err
			}
			created = !exists
			g = x.h.backend.ensureGraph(t.iri)
		}
		// The parser gave the payload's blank nodes fresh identities, so adding
		// its triples is an RDF merge, not a union (GSP §5.5).
		if err := copyTriples(ctx, g, payload); err != nil {
			return fmt.Errorf("graph %s cannot be written: %w", t, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	x.w.Header().Set("Cache-Control", "no-store")
	if created {
		x.w.WriteHeader(http.StatusCreated)
	} else {
		// 200 as in the W3C manifest's "POST - existing graph"; §5.5 allows
		// 200 or 204.
		x.w.WriteHeader(http.StatusOK)
	}
	return nil
}

func (x *exchange) gspDelete(t gspTarget) error {
	if t.isStore {
		return httpErrf(http.StatusBadRequest, "DELETE on the graph store itself needs ?default or ?graph=<iri>")
	}
	err := x.writeLocked(func(ctx context.Context) error {
		if t.isDefault {
			def, _, err := x.h.backend.view(ctx)
			if err != nil {
				return err
			}
			if err := def.Remove(ctx, nil, nil, nil); err != nil {
				return fmt.Errorf("graph %s cannot be cleared: %w", t, err)
			}
			return nil
		}
		_, ok, err := x.h.backend.graph(ctx, t.iri)
		if err != nil {
			return err
		}
		if !ok {
			return notFound(t)
		}
		return x.h.backend.dropGraph(ctx, t.iri)
	})
	if err != nil {
		return err
	}
	x.w.Header().Set("Cache-Control", "no-store")
	// 200 as in the W3C manifest's "DELETE - existing graph".
	x.w.WriteHeader(http.StatusOK)
	return nil
}

func (x *exchange) readLocked(ctx context.Context, fn func() error) error {
	if err := x.h.lock.rlock(ctx); err != nil {
		return evalError(ctx, err)
	}
	defer x.h.lock.runlock()
	return evalError(ctx, fn())
}

func (x *exchange) writeLocked(fn func(ctx context.Context) error) error {
	ctx, cancel := withTimeout(x.r.Context(), x.h.cfg.updateTimeout)
	defer cancel()
	if err := x.h.lock.lock(ctx); err != nil {
		return evalError(ctx, err)
	}
	defer x.h.lock.unlock()
	return evalError(ctx, fn(ctx))
}

// copyTriples adds every triple of src to dst, stopping at the first store
// error.
func copyTriples(ctx context.Context, dst, src *graph.Graph) error {
	for t, err := range src.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return err
		}
		if err := dst.Add(ctx, t.Subject, t.Predicate, t.Object); err != nil {
			return err
		}
	}
	return nil
}

// newGraphIRI mints the IRI of a graph created by POST to the graph store:
// the store's IRI followed by a random segment.
func (x *exchange) newGraphIRI() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("minting a graph IRI: %w", err)
	}
	return strings.TrimSuffix(x.h.requestIRI(x.r), "/") + "/" + hex.EncodeToString(b[:]), nil
}

// readPayload parses a PUT or POST body into a new in-memory graph, before any
// lock is taken, so a malformed body changes nothing.
func (x *exchange) readPayload(t gspTarget) (*graph.Graph, error) {
	base := x.h.cfg.baseIRI
	if base == "" {
		if t.iri != "" {
			base = t.iri
		} else {
			base = x.h.requestIRI(x.r)
		}
	}
	g := graph.NewGraph()
	header := x.r.Header.Get("Content-Type")
	mt, charset := mediaType(header)
	if mt == "multipart/form-data" {
		return g, x.readMultipart(g, header, base)
	}
	f := rdfFormatFor(mt)
	if f == nil {
		return nil, unsupportedRDF(mt)
	}
	if !utf8Charset(charset) {
		return nil, httpErrf(http.StatusUnsupportedMediaType, "charset %q is not supported: RDF payloads must be UTF-8", charset)
	}
	body, err := x.readBody(x.h.cfg.maxGraphBytes)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return g, nil
	}
	if err := parsePayload(x.r.Context(), f, g, bytes.NewReader(body), base); err != nil {
		return nil, err
	}
	if x.rows, err = g.Len(x.r.Context()); err != nil {
		return nil, err
	}
	return g, nil
}

func (x *exchange) readMultipart(g *graph.Graph, header, base string) error {
	_, params, err := mime.ParseMediaType(header)
	if err != nil || params["boundary"] == "" {
		return httpErrf(http.StatusBadRequest, "multipart/form-data without a boundary")
	}
	mr := multipart.NewReader(x.bodyReader(x.h.cfg.maxGraphBytes), params["boundary"])
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return bodyError(err, x.h.cfg.maxGraphBytes)
		}
		mt, charset := mediaType(part.Header.Get("Content-Type"))
		f := rdfFormatFor(mt)
		if part.Header.Get("Content-Type") == "" {
			f = formatFromFilename(part.FileName())
		}
		if f == nil {
			return unsupportedRDF(mt)
		}
		if !utf8Charset(charset) {
			return httpErrf(http.StatusUnsupportedMediaType, "charset %q is not supported: RDF payloads must be UTF-8", charset)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return bodyError(err, x.h.cfg.maxGraphBytes)
		}
		if len(bytes.TrimSpace(data)) == 0 {
			continue
		}
		if err := parsePayload(x.r.Context(), f, g, bytes.NewReader(data), base); err != nil {
			return fmt.Errorf("part %q: %w", part.FileName(), err)
		}
	}
	x.rows, err = g.Len(x.r.Context())
	return err
}

// formatFromFilename guesses a part's format from its extension, as GSP §5.5
// allows for multipart bodies without a per-part Content-Type.
func formatFromFilename(name string) *rdfFormat {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return rdfFormatFor("")
	}
	switch strings.ToLower(name[i:]) {
	case ".ttl", ".turtle":
		return rdfFormatFor("text/turtle")
	case ".nt":
		return rdfFormatFor("application/n-triples")
	case ".jsonld", ".json":
		return rdfFormatFor("application/ld+json")
	}
	return rdfFormatFor("")
}

func parsePayload(ctx context.Context, f *rdfFormat, g *graph.Graph, r io.Reader, base string) (err error) {
	defer func() {
		// A parser is aimed at untrusted bytes; a panic in one is a bad
		// request, not a crashed handler.
		if v := recover(); v != nil {
			err = httpErrf(http.StatusBadRequest, "malformed %s payload: parser failed: %v", f.mediaType, v)
		}
	}()
	if perr := f.parse(ctx, g, r, base); perr != nil {
		return httpErrf(http.StatusBadRequest, "malformed %s payload: %v", f.mediaType, perr)
	}
	return nil
}

func unsupportedRDF(mt string) error {
	var names []string
	for _, f := range rdfFormats {
		if f.parse != nil {
			names = append(names, f.mediaType)
		}
	}
	return httpErrf(http.StatusUnsupportedMediaType, "unsupported Content-Type %q for a graph payload; use one of %s",
		mt, strings.Join(names, ", "))
}
