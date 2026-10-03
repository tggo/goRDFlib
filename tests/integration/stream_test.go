package integration_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/trig"
	"github.com/tggo/goRDFlib/turtle"
)

// streamFormat runs one format both ways: into a graph, and through its
// streaming entry point. Both return statements as keys with blank nodes
// masked, sorted, so the two can be compared without an isomorphism check.
type streamFormat struct {
	name   string
	ext    []string
	parse  func(r io.Reader) ([]string, error)
	stream func(ctx context.Context, r io.Reader, emit func(string) error) error
}

func stmtKey(g Term, s Subject, p URIRef, o Term) string {
	mask := func(t Term) string {
		if t == nil {
			return "DEFAULT"
		}
		if _, ok := t.(BNode); ok {
			return "_"
		}
		if tt, ok := t.(TripleTerm); ok {
			return "<<" + stmtKey(nil, tt.Subject(), tt.Predicate(), tt.Object()) + ">>"
		}
		return term.TermKey(t)
	}
	return mask(g) + " " + mask(s) + " " + mask(p) + " " + mask(o)
}

func graphKeys(g *Graph) []string {
	var keys []string
	g.Triples(nil, nil, nil)(func(t Triple) bool {
		keys = append(keys, stmtKey(nil, t.Subject, t.Predicate, t.Object))
		return true
	})
	slices.Sort(keys)
	return keys
}

func datasetKeys(ds *graph.Dataset) []string {
	var keys []string
	for g := range ds.Graphs() {
		var id Term = g.Identifier()
		if g == ds.DefaultContext() {
			id = nil
		}
		g.Triples(nil, nil, nil)(func(t Triple) bool {
			keys = append(keys, stmtKey(id, t.Subject, t.Predicate, t.Object))
			return true
		})
	}
	slices.Sort(keys)
	return keys
}

var streamFormats = []streamFormat{
	{
		name: "nt", ext: []string{".nt"},
		parse: func(r io.Reader) ([]string, error) {
			g := NewGraph()
			err := nt.Parse(g, r)
			return graphKeys(g), err
		},
		stream: func(ctx context.Context, r io.Reader, emit func(string) error) error {
			return nt.ParseStreamContext(ctx, r, func(s Subject, p URIRef, o Term) error {
				return emit(stmtKey(nil, s, p, o))
			})
		},
	},
	{
		name: "nq", ext: []string{".nq"},
		parse: func(r io.Reader) ([]string, error) {
			g := NewGraph()
			err := nq.Parse(g, r)
			return graphKeys(g), err
		},
		stream: func(ctx context.Context, r io.Reader, emit func(string) error) error {
			return nq.ParseStreamContext(ctx, r, func(s Subject, p URIRef, o Term, _ Term) error {
				return emit(stmtKey(nil, s, p, o)) // Parse drops the graph too
			})
		},
	},
	{
		name: "turtle", ext: []string{".ttl"},
		parse: func(r io.Reader) ([]string, error) {
			g := NewGraph()
			err := turtle.Parse(g, r, turtle.WithBase("http://example/base/"))
			return graphKeys(g), err
		},
		stream: func(ctx context.Context, r io.Reader, emit func(string) error) error {
			return turtle.ParseStreamContext(ctx, r, func(s Subject, p URIRef, o Term) error {
				return emit(stmtKey(nil, s, p, o))
			}, turtle.WithBase("http://example/base/"))
		},
	},
	{
		name: "trig", ext: []string{".trig"},
		parse: func(r io.Reader) ([]string, error) {
			ds := graph.NewDataset()
			err := trig.ParseDataset(ds, r, trig.WithBase("http://example/base/"))
			return datasetKeys(ds), err
		},
		stream: func(ctx context.Context, r io.Reader, emit func(string) error) error {
			return trig.ParseStreamContext(ctx, r, func(s Subject, p URIRef, o Term, g Term) error {
				return emit(stmtKey(g, s, p, o))
			}, trig.WithBase("http://example/base/"))
		},
	},
	{
		name: "rdfxml", ext: []string{".rdf"},
		parse: func(r io.Reader) ([]string, error) {
			g := NewGraph()
			err := rdfxml.Parse(g, r, rdfxml.WithBase("http://example/base/"))
			return graphKeys(g), err
		},
		stream: func(ctx context.Context, r io.Reader, emit func(string) error) error {
			return rdfxml.ParseStreamContext(ctx, r, func(s Subject, p URIRef, o Term) error {
				return emit(stmtKey(nil, s, p, o))
			}, rdfxml.WithBase("http://example/base/"))
		},
	},
}

// collect streams data and returns the keys the way a graph would hold them:
// deduplicated (blank nodes masked, so duplicates by mask collapse too — the
// same masking is applied to the graph side).
func collect(f streamFormat, data []byte) ([]string, error) {
	var keys []string
	err := f.stream(context.Background(), bytes.NewReader(data), func(k string) error {
		keys = append(keys, k)
		return nil
	})
	slices.Sort(keys)
	return keys, err
}

// Every W3C test file of the five formats: the stream must deliver exactly
// what Parse puts in the graph, and fail exactly when Parse fails.
func TestStreamMatchesParseOnW3CCorpus(t *testing.T) {
	root := "../../testdata/w3c/rdf-tests/rdf"
	for _, f := range streamFormats {
		t.Run(f.name, func(t *testing.T) {
			n := 0
			filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !slices.Contains(f.ext, filepath.Ext(path)) {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				n++
				want, wantErr := f.parse(bytes.NewReader(data))
				got, gotErr := collect(f, data)
				if (wantErr == nil) != (gotErr == nil) {
					t.Errorf("%s: Parse err = %v, stream err = %v", path, wantErr, gotErr)
					return nil
				}
				if wantErr != nil {
					return nil
				}
				// A graph is a set; a stream may repeat a statement.
				got = slices.Compact(got)
				want = slices.Compact(want)
				if !slices.Equal(got, want) {
					t.Errorf("%s: stream gave %d statements, Parse %d", path, len(got), len(want))
				}
				return nil
			})
			if n == 0 {
				t.Skip("no W3C files (submodule not checked out?)")
			}
		})
	}
}

func streamInput(name string, n int) []byte {
	var b strings.Builder
	switch name {
	case "rdfxml":
		b.WriteString(`<?xml version="1.0"?><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/">`)
		for i := range n {
			fmt.Fprintf(&b, `<rdf:Description rdf:about="http://example.org/s%d"><ex:p>v%d</ex:p></rdf:Description>`+"\n", i, i)
		}
		b.WriteString(`</rdf:RDF>`)
	case "trig":
		b.WriteString("<http://example.org/g> {\n")
		for i := range n {
			fmt.Fprintf(&b, "<http://example.org/s%d> <http://example.org/p> \"v%d\" .\n", i, i)
		}
		b.WriteString("}\n")
	case "nq":
		for i := range n {
			fmt.Fprintf(&b, "<http://example.org/s%d> <http://example.org/p> \"v%d\" <http://example.org/g> .\n", i, i)
		}
	default:
		for i := range n {
			fmt.Fprintf(&b, "<http://example.org/s%d> <http://example.org/p> \"v%d\" .\n", i, i)
		}
	}
	return []byte(b.String())
}

var errStopHere = errors.New("stop here")

func TestStreamHandlerErrorStops(t *testing.T) {
	for _, f := range streamFormats {
		t.Run(f.name, func(t *testing.T) {
			calls := 0
			err := f.stream(context.Background(), bytes.NewReader(streamInput(f.name, 5000)), func(string) error {
				calls++
				if calls == 10 {
					return errStopHere
				}
				return nil
			})
			if !errors.Is(err, errStopHere) {
				t.Fatalf("err = %v, want errStopHere", err)
			}
			if calls != 10 {
				t.Errorf("handler called %d times, want 10: no call after the error", calls)
			}
		})
	}
}

func TestStreamContextCancel(t *testing.T) {
	for _, f := range streamFormats {
		t.Run(f.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			calls := 0
			err := f.stream(ctx, bytes.NewReader(streamInput(f.name, 20000)), func(string) error {
				calls++
				if calls == 100 {
					cancel()
				}
				return nil
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
			if calls >= 20000 {
				t.Errorf("parse ran to the end (%d calls) after cancel", calls)
			}
			// Already cancelled: nothing is delivered.
			calls = 0
			err = f.stream(ctx, bytes.NewReader(streamInput(f.name, 10)), func(string) error { calls++; return nil })
			if !errors.Is(err, context.Canceled) || calls != 0 {
				t.Errorf("pre-cancelled: err = %v, calls = %d", err, calls)
			}
		})
	}
}

func TestStreamNilHandler(t *testing.T) {
	r := strings.NewReader("")
	for name, err := range map[string]error{
		"nt":     nt.ParseStream(r, nil),
		"nq":     nq.ParseStream(r, nil),
		"turtle": turtle.ParseStream(r, nil),
		"trig":   trig.ParseStream(r, nil),
		"rdfxml": rdfxml.ParseStream(r, nil),
	} {
		if err == nil {
			t.Errorf("%s: nil handler accepted", name)
		}
	}
}

// Prefix bindings still reach the graph through Parse after the sink refactor.
func TestParseStillBindsPrefixes(t *testing.T) {
	g := NewGraph()
	if err := turtle.Parse(g, strings.NewReader("@prefix zz: <http://zz.example/> .\nzz:a zz:b zz:c .")); err != nil {
		t.Fatal(err)
	}
	ds := graph.NewDataset()
	if err := trig.ParseDataset(ds, strings.NewReader("@prefix yy: <http://yy.example/> .\nyy:g { yy:a yy:b yy:c }\nyy:empty { }")); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	g.Namespaces()(func(p string, _ URIRef) bool { found["g:"+p] = true; return true })
	ds.Namespaces()(func(p string, _ URIRef) bool { found["ds:"+p] = true; return true })
	if !found["g:zz"] || !found["ds:yy"] {
		t.Errorf("prefixes not bound: %v", found)
	}
	named := 0
	for g := range ds.Graphs() {
		if g != ds.DefaultContext() {
			named++
		}
	}
	if named != 2 {
		t.Errorf("empty GRAPH block no longer creates its graph: %d named graphs", named)
	}
}
