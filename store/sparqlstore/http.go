package sparqlstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// execQuery sends a SPARQL query to the query endpoint and parses the result.
func (s *SPARQLStore) execQuery(ctx context.Context, query string) (*sparql.Result, error) {
	body := url.Values{"query": {query}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.queryURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sparqlstore: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/sparql-results+xml, application/sparql-results+json;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sparqlstore: query request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max error message
		return nil, fmt.Errorf("sparqlstore: query returned %d: %s", resp.StatusCode, string(msg))
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "json") {
		return sparql.ParseSRJ(resp.Body)
	}
	return sparql.ParseSRX(resp.Body)
}

// execUpdate sends a SPARQL update to the update endpoint.
func (s *SPARQLStore) execUpdate(ctx context.Context, update string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.updateURL, strings.NewReader(update))
	if err != nil {
		return fmt.Errorf("sparqlstore: create update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/sparql-update")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sparqlstore: update request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max error message
		return fmt.Errorf("sparqlstore: update returned %d: %s", resp.StatusCode, string(msg))
	}
	return nil
}

// termToSPARQL converts a Term to its SPARQL representation.
func termToSPARQL(t term.Term) string {
	return t.N3()
}

// tripleToSPARQL converts a Triple to SPARQL syntax: <s> <p> <o> .
func tripleToSPARQL(t term.Triple) string {
	return fmt.Sprintf("%s %s %s .", termToSPARQL(t.Subject), termToSPARQL(t.Predicate), termToSPARQL(t.Object))
}

// patternToSPARQL converts a TriplePattern to SPARQL, using variables for nil positions.
func patternToSPARQL(p term.TriplePattern) string {
	sv, pv, ov := "?s", "?p", "?o"
	if p.Subject != nil {
		sv = termToSPARQL(p.Subject)
	}
	if p.Predicate != nil {
		pv = termToSPARQL(*p.Predicate)
	}
	if p.Object != nil {
		ov = termToSPARQL(p.Object)
	}
	return fmt.Sprintf("%s %s %s .", sv, pv, ov)
}

// wrapGraph wraps SPARQL body in GRAPH <ctx> { ... } if context is a URIRef.
// BNode contexts are treated as the default graph (blank nodes cannot name
// SPARQL graphs, and Graph passes its BNode identifier as context).
func wrapGraph(ctx term.Term, body string) string {
	if store.IsDefaultGraph(ctx) {
		return body
	}
	// SPARQL cannot name a graph with a blank node (GRAPH takes VarOrIri, and a
	// blank node in INSERT DATA denotes a fresh node anyway), so a blank-node
	// context has no representation on the wire. It falls back to the default
	// graph; storetest records this as a declared limitation.
	if _, isBNode := ctx.(term.BNode); isBNode {
		return body
	}
	return fmt.Sprintf("GRAPH %s { %s }", termToSPARQL(ctx), body)
}
