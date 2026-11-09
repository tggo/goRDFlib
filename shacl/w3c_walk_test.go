package shacl

import (
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
)

// collectShaclTests walks a manifest and every manifest it mf:includes, to any
// depth, and returns the files that hold sht:Validate entries. A test file
// that is itself a manifest (scope-interactions/*/manifest.ttl lists its own
// entry) is a leaf. Nothing is skipped silently: an include that cannot be
// resolved, loaded, or that holds neither tests nor includes is an error.
func collectShaclTests(t *testing.T, manifestPath string) []string {
	t.Helper()
	var out []string
	seen := map[string]bool{}
	var walk func(path string)
	walk = func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		g, err := LoadTurtleFile(path)
		if err != nil {
			t.Errorf("load manifest %s: %v", path, err)
			return
		}
		typePred, validate := IRI(RDFType), IRI(SHT+"Validate")
		if len(g.All(nil, &typePred, &validate)) > 0 {
			out = append(out, path)
			return
		}
		includePred := IRI(MF + "include")
		includes := g.All(nil, &includePred, nil)
		if len(includes) == 0 {
			t.Errorf("%s has neither sht:Validate entries nor mf:include", path)
			return
		}
		for _, inc := range includes {
			p := uriToPath(resolveURI(g.BaseURI(), inc.Object.Value()))
			if p == "" || !strings.HasSuffix(p, ".ttl") {
				t.Errorf("%s: cannot resolve include %s", path, inc.Object.Value())
				continue
			}
			walk(p)
		}
	}
	walk(manifestPath)
	sort.Strings(out)
	return out
}

// runShaclManifestTree runs every Validate test reachable from root/manifest.ttl.
func runShaclManifestTree(t *testing.T, suiteBase, root string) {
	t.Helper()
	baseDir := filepath.Join(suiteBase, root)
	files := collectShaclTests(t, filepath.Join(baseDir, "manifest.ttl"))
	if len(files) == 0 {
		t.Fatalf("no tests found under %s", baseDir)
	}
	var passed int64
	for _, f := range files {
		rel, _ := filepath.Rel(suiteBase, f)
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			if runSingleTest(t, f) {
				atomic.AddInt64(&passed, 1)
			}
		})
	}
	t.Logf("%s: Total: %d, Passed: %d", root, len(files), atomic.LoadInt64(&passed))
}
