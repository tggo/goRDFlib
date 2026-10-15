package jsonld

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/piprate/json-gold/ld"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/testutil"
)

// The W3C JSON-LD 1.1 API test suite, toRdf manifest
// (testdata/w3c/json-ld-api, a submodule of github.com/w3c/json-ld-api).
const (
	w3cDir  = "../testdata/w3c/json-ld-api/tests"
	w3cBase = "https://w3c.github.io/json-ld-api/tests/"
)

// w3cKnown maps a toRdf test id to the reason it is skipped. Only what this
// package genuinely cannot do belongs here; an empty reason is an error.
//
// Every entry is a json-gold (piprate/json-gold v0.9.1) limitation; none is in
// this package's own conversion. A known test that starts passing fails the
// run, so the list cannot outlive the fix.
var w3cKnown = map[string]string{}

func init() {
	known := func(reason string, ids ...string) {
		for _, id := range ids {
			w3cKnown["#"+id] = reason
		}
	}
	known("json-gold resolves IRIs with net/url: it normalises dot segments in a fragment- or query-only reference "+
		"(RFC 3986 §5.2.2 keeps the base path as is), mangles non-hierarchical bases (tag:, ex:) into tag:///…, "+
		"and percent-encodes characters an IRI may not hold instead of rejecting them",
		"t0122", "t0123", "t0124", "t0125", "t0128", "t0130", "t0131", "t0132", "tli11", "tli12", "te111", "te112")
	known("json-gold's JSON canonicaliser accepts only objects; for other values it stored its error text as the "+
		"literal (Parse now fails with ErrUnsupportedJSONLiteral), and it writes doubles in Go's E-notation",
		"tjs03", "tjs07", "tjs08", "tjs14", "tjs17", "tjs18", "tjs22", "tjs23")
	known("json-gold does not implement the rdfDirection option (i18n-datatype, compound-literal)",
		"tdi09", "tdi10", "tdi11", "tdi12")
	known("json-gold does not report this context error",
		"tc033", "te123", "tec02", "ter52", "ter54", "ter56", "tpr28")
	known("json-gold resolves a relative remote context against @base instead of the document URL", "tc031")
	known("json-gold does not apply a property-scoped context through an @nest alias", "tc037")
	known("json-gold gives a type-scoped context the wrong propagation", "tc038")
	known("json-gold rejects a term whose @reverse value has the form of a keyword instead of ignoring it", "tpr38", "tpr39")
	known("json-gold keeps term protection through a null context in an @graph container", "tpr43")
}

type w3cTest struct {
	ID     string   `json:"@id"`
	Type   []string `json:"@type"`
	Name   string   `json:"name"`
	Input  string   `json:"input"`
	Expect string   `json:"expect"`
	Error  string   `json:"expectErrorCode"`
	Option struct {
		SpecVersion           string `json:"specVersion"`
		ProcessingMode        string `json:"processingMode"`
		Base                  string `json:"base"`
		ExpandContext         string `json:"expandContext"`
		RDFDirection          string `json:"rdfDirection"`
		ProduceGeneralizedRdf bool   `json:"produceGeneralizedRdf"`
	} `json:"option"`
}

func (t w3cTest) is(typ string) bool {
	for _, x := range t.Type {
		if x == typ {
			return true
		}
	}
	return false
}

// localLoader serves the suite's own URLs from the checkout and refuses
// everything else, so the run never touches the network.
type localLoader struct{}

func (localLoader) LoadDocument(u string) (*ld.RemoteDocument, error) {
	rel, ok := strings.CutPrefix(u, w3cBase)
	if !ok {
		return nil, ld.NewJsonLdError(ld.LoadingDocumentFailed, "not in the test suite: "+u)
	}
	if i := strings.IndexAny(rel, "?#"); i >= 0 {
		rel = rel[:i]
	}
	f, err := os.Open(filepath.Join(w3cDir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, ld.NewJsonLdError(ld.LoadingDocumentFailed, err)
	}
	defer f.Close()
	doc, err := ld.DocumentFromReader(f)
	if err != nil {
		return nil, ld.NewJsonLdError(ld.LoadingDocumentFailed, err)
	}
	return &ld.RemoteDocument{DocumentURL: u, Document: doc}, nil
}

func TestW3CToRDF(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(w3cDir, "toRdf-manifest.jsonld"))
	if err != nil {
		t.Skipf("JSON-LD test suite not checked out (git submodule update --init): %v", err)
	}
	var m struct {
		Sequence []w3cTest `json:"sequence"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for id, reason := range w3cKnown {
		if reason == "" {
			t.Fatalf("w3cKnown[%s] has no reason", id)
		}
	}

	var pass, fail, skip int
	for _, tc := range m.Sequence {
		t.Run(strings.TrimPrefix(tc.ID, "#"), func(t *testing.T) {
			if tc.Option.SpecVersion == "json-ld-1.0" || tc.Option.ProcessingMode == "json-ld-1.0" {
				skip++
				t.Skip("JSON-LD 1.0 only; this is a 1.1 processor")
			}
			if reason, ok := w3cKnown[tc.ID]; ok {
				// A pass means json-gold fixed it and the entry must go.
				if runW3CToRDF(t, tc, false) == nil {
					t.Errorf("passes now; remove it from w3cKnown (%s)", reason)
					return
				}
				skip++
				t.Skip(reason)
			}
			err := runW3CToRDF(t, tc, true)
			if err != nil {
				fail++
				t.Errorf("%s: %v", tc.Name, err)
				return
			}
			pass++
		})
	}
	t.Logf("toRdf: %d passed, %d failed, %d skipped of %d", pass, fail, skip, len(m.Sequence))
}

// runW3CToRDF runs one test; with report, a graph mismatch is printed.
func runW3CToRDF(t *testing.T, tc w3cTest, report bool) error {
	ctx := t.Context()
	base := w3cBase + tc.Input
	if tc.Option.Base != "" {
		base = tc.Option.Base
	}
	opts := []Option{WithBase(base), WithDocumentLoader(localLoader{})}
	if tc.Option.ExpandContext != "" {
		rd, err := localLoader{}.LoadDocument(w3cBase + tc.Option.ExpandContext)
		if err != nil {
			return err
		}
		opts = append(opts, WithExpandContext(rd.Document))
	}
	in, err := os.Open(filepath.Join(w3cDir, filepath.FromSlash(tc.Input)))
	if err != nil {
		return err
	}
	defer in.Close()
	got := rdflibgo.NewGraph()
	perr := Parse(ctx, got, in, opts...)

	switch {
	case tc.is("jld:NegativeEvaluationTest"):
		if perr == nil {
			return fmt.Errorf("parsed without error, want %q", tc.Error)
		}
		return nil
	case perr != nil:
		return fmt.Errorf("unexpected error: %w", perr)
	case tc.is("jld:PositiveSyntaxTest"):
		return nil
	}

	// Parse flattens named graphs into one, so the expected quads are
	// flattened the same way before comparing.
	want := rdflibgo.NewGraph()
	exp, err := os.ReadFile(filepath.Join(w3cDir, filepath.FromSlash(tc.Expect)))
	if err != nil {
		return err
	}
	err = nq.ParseStream(strings.NewReader(string(exp)), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, _ rdflibgo.Term) error {
		return want.Add(ctx, s, p, o)
	})
	if err != nil {
		return fmt.Errorf("expected output does not parse: %w", err)
	}
	same, err := testutil.Isomorphic(ctx, want, got)
	if err != nil {
		return err
	}
	if !same {
		if report {
			testutil.AssertGraphEqual(t, want, got) // prints the difference
		}
		return errors.New("graph differs from the expected output")
	}
	return nil
}
