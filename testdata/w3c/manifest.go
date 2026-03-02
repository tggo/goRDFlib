package w3c

import (
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
	Name   string // mf:name
	Type   string // full IRI, e.g. "http://www.w3.org/ns/rdftest#TestTurtleEval"
	Action string // absolute file path from mf:action (for simple tests)
	Result string // absolute file path from mf:result (empty for syntax tests)
	Query  string // absolute file path from qt:query (SPARQL tests)
	Data   string // absolute file path from qt:data (SPARQL tests)
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
	if err := turtle.Parse(g, f, turtle.WithBase(base)); err != nil {
		return nil, err
	}

	m := &Manifest{ManifestDir: filepath.Dir(absPath)}

	// Extract mf:assumedTestBase if present.
	assumedBasePred := term.NewURIRefUnsafe(mf + "assumedTestBase")
	for tr := range g.Triples(nil, &assumedBasePred, nil) {
		m.AssumedTestBase = tr.Object.(term.URIRef).Value()
		break
	}

	// Find the list head from the mf:entries triple.
	entriesPred := term.NewURIRefUnsafe(mf + "entries")
	var listSubj term.Subject
	for tr := range g.Triples(nil, &entriesPred, nil) {
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

	coll.Iter()(func(item term.Term) bool {
		subj, ok := item.(term.Subject)
		if !ok {
			return true
		}

		var e TestEntry

		if v, ok := g.Value(subj, &namePred, nil); ok {
			e.Name = v.(term.Literal).Lexical()
		}
		if v, ok := g.Value(subj, &typePred, nil); ok {
			e.Type = v.(term.URIRef).Value()
		}
		if v, ok := g.Value(subj, &resultPred, nil); ok {
			e.Result = toFilePath(v.(term.URIRef).Value())
		}

		// Check if mf:action is a URI or a blank node (SPARQL tests use blank node)
		if v, ok := g.Value(subj, &actionPred, nil); ok {
			switch av := v.(type) {
			case term.URIRef:
				e.Action = toFilePath(av.Value())
			case term.Subject:
				// SPARQL manifest: action is a blank node with qt:query and qt:data
				if qv, ok := g.Value(av, &queryPred, nil); ok {
					e.Query = toFilePath(qv.(term.URIRef).Value())
				}
				if dv, ok := g.Value(av, &dataPred, nil); ok {
					e.Data = toFilePath(dv.(term.URIRef).Value())
				}
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
	if err := turtle.Parse(g, f, turtle.WithBase(base)); err != nil {
		return nil, err
	}

	includePred := term.NewURIRefUnsafe(mf + "include")
	var listHead term.Subject
	for tr := range g.Triples(nil, &includePred, nil) {
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
	coll.Iter()(func(item term.Term) bool {
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
