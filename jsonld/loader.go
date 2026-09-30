package jsonld

import (
	"net/http"
	"sync"
	"time"

	"github.com/piprate/json-gold/ld"
)

// DefaultLoaderTimeout bounds each HTTP request made by the loader that
// NewCachingDocumentLoader builds when it is given no loader of its own.
const DefaultLoaderTimeout = 30 * time.Second

// CachingDocumentLoader is an ld.DocumentLoader that fetches each remote
// document (typically a remote @context such as "https://schema.org/") once
// and serves it from memory afterwards (issue #39).
//
// Without WithDocumentLoader, every Parse call builds json-gold's default
// loader, which fetches every remote @context over HTTP again, with no
// timeout. json-gold's own ld.CachingDocumentLoader fixes the repetition but
// keeps its cache in an unguarded map, so it cannot be shared by goroutines.
//
// CachingDocumentLoader is safe for concurrent use and meant to be shared by
// every Parse call of a process:
//
//	loader := jsonld.NewCachingDocumentLoader(nil)
//	err := jsonld.Parse(g, r, jsonld.WithDocumentLoader(loader))
//
// Concurrent requests for a document that is not cached yet wait for a single
// fetch. A failed fetch is not cached, so the next request tries again.
// Cached documents never expire; build a new loader to refresh them. Documents
// are shared between callers and must not be modified.
type CachingDocumentLoader struct {
	next ld.DocumentLoader

	mu      sync.Mutex
	entries map[string]*loaderEntry
}

// loaderEntry is one document, fetched or being fetched. done is closed once
// doc and err are set.
type loaderEntry struct {
	done chan struct{}
	doc  *ld.RemoteDocument
	err  error
}

// NewCachingDocumentLoader returns a loader that caches what next loads. With
// a nil next it uses json-gold's default loader over an HTTP client whose
// requests time out after DefaultLoaderTimeout. Like json-gold's default
// loader, that one also reads IRIs that are not http(s) as local file paths;
// pass your own next loader when documents come from untrusted sources.
func NewCachingDocumentLoader(next ld.DocumentLoader) *CachingDocumentLoader {
	if next == nil {
		next = ld.NewDefaultDocumentLoader(&http.Client{Timeout: DefaultLoaderTimeout})
	}
	return &CachingDocumentLoader{next: next, entries: make(map[string]*loaderEntry)}
}

// AddDocument caches doc under u, replacing what was cached for it. It is how
// a context is bundled with a program instead of fetched.
func (l *CachingDocumentLoader) AddDocument(u string, doc any) {
	e := &loaderEntry{done: make(chan struct{}), doc: &ld.RemoteDocument{DocumentURL: u, Document: doc}}
	close(e.done)
	l.mu.Lock()
	l.entries[u] = e
	l.mu.Unlock()
}

// Preload caches doc under every URL in urls. A bundled context usually has to
// answer to several spellings of its address (for schema.org: with and
// without the trailing slash, http and https, the jsonldcontext.jsonld file),
// and each spelling is a separate cache key.
func (l *CachingDocumentLoader) Preload(doc any, urls ...string) {
	for _, u := range urls {
		l.AddDocument(u, doc)
	}
}

// LoadDocument implements ld.DocumentLoader.
func (l *CachingDocumentLoader) LoadDocument(u string) (*ld.RemoteDocument, error) {
	l.mu.Lock()
	e, ok := l.entries[u]
	if !ok {
		e = &loaderEntry{done: make(chan struct{})}
		l.entries[u] = e
	}
	l.mu.Unlock()
	if ok {
		<-e.done
		if e.err == nil {
			return e.doc, nil
		}
		// The fetch this call waited for failed and has been forgotten; one
		// retry, so a caller is not failed by an error it did not observe.
		return l.LoadDocument(u)
	}

	e.doc, e.err = l.next.LoadDocument(u)
	if e.err != nil {
		l.mu.Lock()
		if l.entries[u] == e {
			delete(l.entries, u)
		}
		l.mu.Unlock()
	}
	close(e.done)
	return e.doc, e.err
}
