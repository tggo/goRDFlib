package w3c

import (
	"context"
	"os"
	"path/filepath"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

const (
	mf   = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#"
	RDFT = "http://www.w3.org/ns/rdftest#"
	qt   = "http://www.w3.org/2001/sw/DataAccess/tests/test-query#"
)

// Manifest holds parsed W3C test manifest data.
type Manifest struct {
	Entries         []TestEntry
	AssumedTestBase string // mf:assumedTestBase (empty if not set)
	ManifestDir     string // absolute path of the manifest file's directory
}

// TestEntry represents a single W3C conformance test.
type TestEntry struct {
	Name      string   // mf:name
	Type      string   // full IRI, e.g. "http://www.w3.org/ns/rdftest#TestTurtleEval"
	Action    string   // absolute file path from mf:action (for simple tests)
	Result    string   // absolute file path from mf:result (empty for syntax tests)
	Query     string   // absolute file path from qt:query (SPARQL tests)
	Data      string   // absolute file path from qt:data (SPARQL tests)
	GraphData []string // absolute file paths from qt:graphData (named graphs)

	// Update test fields (ut: namespace)
	Request         string          // ut:request - update file path
	ActionData      string          // ut:data in action (pre-data for default graph)
	ActionGraphData []NamedGraphRef // ut:graphData in action (pre-data for named graphs)
	ResultData      string          // ut:data in result (post-data for default graph)
	ResultGraphData []NamedGraphRef // ut:graphData in result (post-data for named graphs)

	// Entailment test fields
	EntailmentRegime      string   // mf:entailmentRegime ("simple"/"RDF"/"RDFS")
	RecognizedDatatypes   []string // mf:recognizedDatatypes (IRI list)
	UnrecognizedDatatypes []string // mf:unrecognizedDatatypes (IRI list)
	ResultFalse           bool     // true when mf:result is literal `false`
}

// NamedGraphRef is a named graph reference with a file path and label (graph IRI).
type NamedGraphRef struct {
	Graph string // absolute file path (from ut:graph)
	Label string // graph IRI (from rdfs:label)
}

// ParseManifest reads a W3C manifest.ttl and returns the manifest with all test entries.
func ParseManifest(manifestPath string) (*Manifest, error) {
	absPath, err := filepath.Abs(manifestPath)
	if err != nil {
		return nil, err
	}
	base := "file://" + absPath

	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	g := graph.NewGraph(graph.WithBase(base))
	if err := turtle.Parse(context.Background(), g, f, turtle.WithBase(base)); err != nil {
		return nil, err
	}

	m := &Manifest{ManifestDir: filepath.Dir(absPath)}

	// Extract mf:assumedTestBase if present.
	assumedBasePred := term.NewURIRefUnsafe(mf + "assumedTestBase")
	for tr := range g.Triples(context.Background(), nil, &assumedBasePred, nil) {
		m.AssumedTestBase = tr.Object.(term.URIRef).Value()
		break
	}

	// Find the list head from the mf:entries triple.
	entriesPred := term.NewURIRefUnsafe(mf + "entries")
	var listSubj term.Subject
	for tr := range g.Triples(context.Background(), nil, &entriesPred, nil) {
		if s, ok := tr.Object.(term.Subject); ok {
			listSubj = s
		}
		break
	}
	if listSubj == nil {
		return m, nil
	}

	// Walk the RDF list to get ordered test IRIs.
	coll := graph.NewCollection(g, listSubj)
	typePred := namespace.RDF.Type
	namePred := term.NewURIRefUnsafe(mf + "name")
	actionPred := term.NewURIRefUnsafe(mf + "action")
	resultPred := term.NewURIRefUnsafe(mf + "result")
	queryPred := term.NewURIRefUnsafe(qt + "query")
	dataPred := term.NewURIRefUnsafe(qt + "data")
	graphDataPred := term.NewURIRefUnsafe(qt + "graphData")

	// Update test predicates (ut: namespace)
	const ut = "http://www.w3.org/2009/sparql/tests/test-update#"
	utRequest := term.NewURIRefUnsafe(ut + "request")
	utData := term.NewURIRefUnsafe(ut + "data")
	utGraphData := term.NewURIRefUnsafe(ut + "graphData")
	utGraph := term.NewURIRefUnsafe(ut + "graph")
	rdfsLabel := term.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#label")

	// Entailment test predicates
	entailmentRegimePred := term.NewURIRefUnsafe(mf + "entailmentRegime")
	recognizedDTPred := term.NewURIRefUnsafe(mf + "recognizedDatatypes")
	unrecognizedDTPred := term.NewURIRefUnsafe(mf + "unrecognizedDatatypes")

	coll.Iter(context.Background())(func(item term.Term, _ error) bool {
		subj, ok := item.(term.Subject)
		if !ok {
			return true
		}

		var e TestEntry

		if v, ok, _ := g.Value(context.Background(), subj, &namePred, nil); ok {
			e.Name = v.(term.Literal).Lexical()
		}
		if v, ok, _ := g.Value(context.Background(), subj, &typePred, nil); ok {
			e.Type = v.(term.URIRef).Value()
		}

		// Check if mf:action is a URI or a blank node (SPARQL tests use blank node)
		if v, ok, _ := g.Value(context.Background(), subj, &actionPred, nil); ok {
			switch av := v.(type) {
			case term.URIRef:
				e.Action = toFilePath(av.Value())
			case term.Subject:
				// SPARQL query manifest: qt:query, qt:data, qt:graphData
				if qv, ok, _ := g.Value(context.Background(), av, &queryPred, nil); ok {
					e.Query = toFilePath(qv.(term.URIRef).Value())
				}
				if dv, ok, _ := g.Value(context.Background(), av, &dataPred, nil); ok {
					e.Data = toFilePath(dv.(term.URIRef).Value())
				}
				for gd := range g.Triples(context.Background(), av, &graphDataPred, nil) {
					if u, ok := gd.Object.(term.URIRef); ok {
						e.GraphData = append(e.GraphData, toFilePath(u.Value()))
					}
				}
				// SPARQL update manifest: ut:request, ut:data, ut:graphData
				if rv, ok, _ := g.Value(context.Background(), av, &utRequest, nil); ok {
					e.Request = toFilePath(rv.(term.URIRef).Value())
				}
				if dv, ok, _ := g.Value(context.Background(), av, &utData, nil); ok {
					e.ActionData = toFilePath(dv.(term.URIRef).Value())
				}
				for gd := range g.Triples(context.Background(), av, &utGraphData, nil) {
					gdNode, ok := gd.Object.(term.Subject)
					if !ok {
						continue
					}
					var ref NamedGraphRef
					if gv, ok, _ := g.Value(context.Background(), gdNode, &utGraph, nil); ok {
						ref.Graph = toFilePath(gv.(term.URIRef).Value())
					}
					if lv, ok, _ := g.Value(context.Background(), gdNode, &rdfsLabel, nil); ok {
						ref.Label = lv.(term.Literal).Lexical()
					}
					e.ActionGraphData = append(e.ActionGraphData, ref)
				}
			}
		}

		// Parse mf:result for update tests (blank node with ut:data, ut:graphData)
		if v, ok, _ := g.Value(context.Background(), subj, &resultPred, nil); ok {
			switch rv := v.(type) {
			case term.URIRef:
				e.Result = toFilePath(rv.Value())
			case term.Literal:
				// mf:result false (boolean literal indicating inconsistency)
				if rv.Lexical() == "false" {
					e.ResultFalse = true
				}
			case term.Subject:
				if dv, ok, _ := g.Value(context.Background(), rv, &utData, nil); ok {
					e.ResultData = toFilePath(dv.(term.URIRef).Value())
				}
				for gd := range g.Triples(context.Background(), rv, &utGraphData, nil) {
					gdNode, ok := gd.Object.(term.Subject)
					if !ok {
						continue
					}
					var ref NamedGraphRef
					if gv, ok, _ := g.Value(context.Background(), gdNode, &utGraph, nil); ok {
						ref.Graph = toFilePath(gv.(term.URIRef).Value())
					}
					if lv, ok, _ := g.Value(context.Background(), gdNode, &rdfsLabel, nil); ok {
						ref.Label = lv.(term.Literal).Lexical()
					}
					e.ResultGraphData = append(e.ResultGraphData, ref)
				}
			}
		}

		// Parse entailment test fields
		if v, ok, _ := g.Value(context.Background(), subj, &entailmentRegimePred, nil); ok {
			if lit, ok := v.(term.Literal); ok {
				e.EntailmentRegime = lit.Lexical()
			}
		}
		if v, ok, _ := g.Value(context.Background(), subj, &recognizedDTPred, nil); ok {
			if listHead, ok := v.(term.Subject); ok {
				c := graph.NewCollection(g, listHead)
				c.Iter(context.Background())(func(item term.Term, _ error) bool {
					if u, ok := item.(term.URIRef); ok {
						e.RecognizedDatatypes = append(e.RecognizedDatatypes, u.Value())
					}
					return true
				})
			}
		}
		if v, ok, _ := g.Value(context.Background(), subj, &unrecognizedDTPred, nil); ok {
			if listHead, ok := v.(term.Subject); ok {
				c := graph.NewCollection(g, listHead)
				c.Iter(context.Background())(func(item term.Term, _ error) bool {
					if u, ok := item.(term.URIRef); ok {
						e.UnrecognizedDatatypes = append(e.UnrecognizedDatatypes, u.Value())
					}
					return true
				})
			}
		}

		m.Entries = append(m.Entries, e)
		return true
	})

	return m, nil
}

// ParseIncludeManifest parses a top-level manifest that uses mf:include to reference sub-manifests.
// Returns all entries from all sub-manifests.
func ParseIncludeManifest(manifestPath string) (*Manifest, error) {
	absPath, err := filepath.Abs(manifestPath)
	if err != nil {
		return nil, err
	}
	base := "file://" + absPath

	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	g := graph.NewGraph(graph.WithBase(base))
	if err := turtle.Parse(context.Background(), g, f, turtle.WithBase(base)); err != nil {
		return nil, err
	}

	includePred := term.NewURIRefUnsafe(mf + "include")
	var listHead term.Subject
	for tr := range g.Triples(context.Background(), nil, &includePred, nil) {
		if s, ok := tr.Object.(term.Subject); ok {
			listHead = s
		}
		break
	}
	if listHead == nil {
		return &Manifest{ManifestDir: filepath.Dir(absPath)}, nil
	}

	result := &Manifest{ManifestDir: filepath.Dir(absPath)}
	coll := graph.NewCollection(g, listHead)
	coll.Iter(context.Background())(func(item term.Term, _ error) bool {
		u, ok := item.(term.URIRef)
		if !ok {
			return true
		}
		subPath := toFilePath(u.Value())
		subManifest, err := ParseManifest(subPath)
		if err != nil {
			return true
		}
		result.Entries = append(result.Entries, subManifest.Entries...)
		return true
	})

	return result, nil
}

// toFilePath converts a file:// URI to an absolute filesystem path.
func toFilePath(uri string) string {
	if len(uri) > 7 && uri[:7] == "file://" {
		return uri[7:]
	}
	return uri
}

// BaseURI returns the test base URI for a given action file path.
// If the manifest has an assumedTestBase, the base is that + relative path from manifest dir.
// Otherwise, the base is the file:// URI of the action path.
func (m *Manifest) BaseURI(actionPath string) string {
	if m.AssumedTestBase != "" && m.ManifestDir != "" {
		rel, err := filepath.Rel(m.ManifestDir, actionPath)
		if err == nil {
			return m.AssumedTestBase + filepath.ToSlash(rel)
		}
	}
	return "file://" + actionPath
}
