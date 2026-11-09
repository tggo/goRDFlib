package shacl

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
)

// A constraint's query text is prepared once per $PATH binding and reused for
// every focus node (preparedQuery). The reports must hold the expected results
// and equal the first (cold) run's on every later run and from 8 goroutines
// (run with -race), for a node shape (no $PATH), two property shapes that
// reference one sh:sparql node with different paths, a query in $var syntax,
// and a SELECT with no variable left once $PATH is removed from it. The
// prepared text must equal the text built per focus node before, with one
// entry per $PATH binding on one constraint.
func TestPreparedSPARQLConstraintText(t *testing.T) {
	shapes := `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
ex:N a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT ?this ?value WHERE { ?this ex:size ?value FILTER (?value > 5) }""" ] .
ex:big sh:select """PREFIX ex: <http://example.org/>
    SELECT $this ?value WHERE { $this $PATH ?value FILTER (?value > 10) }""" .
ex:P1 a sh:NodeShape ; sh:targetClass ex:Item ; sh:property [ sh:path ex:size ; sh:sparql ex:big ] .
ex:P2 a sh:NodeShape ; sh:targetClass ex:Item ; sh:property [ sh:path ex:weight ; sh:sparql ex:big ] .
ex:E a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:property [ sh:path ex:weight ; sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $PATH WHERE { $this $PATH ?w FILTER (?w = 3) }""" ] ] .`
	var data strings.Builder
	data.WriteString("@prefix ex: <http://example.org/> .\n")
	for i := range 30 {
		fmt.Fprintf(&data, "ex:i%d a ex:Item ; ex:size %d ; ex:weight %d .\n", i, i, i%7)
	}
	dg, err := LoadTurtleString(data.String(), "")
	if err != nil {
		t.Fatal(err)
	}
	report := func(r ValidationReport) []string {
		var out []string
		for _, x := range r.Results {
			shape := x.SourceShape.String()
			if x.SourceShape.IsBlank() { // property shapes: labels differ between loads
				shape = "_"
			}
			out = append(out, x.FocusNode.String()+" "+x.Value.String()+" "+shape+" "+x.ResultPath.String())
		}
		sort.Strings(out)
		return out
	}
	fresh := func() []string {
		sg, err := LoadTurtleString(shapes, "http://example.org/shapes")
		if err != nil {
			t.Fatal(err)
		}
		return report(Validate(dg, sg))
	}
	want := fresh()
	// ex:N: sizes 6..29 (24); ex:P1: sizes 11..29 (19); ex:P2: no weight above
	// 10 (0); ex:E: weight 3 on i3, i10, i17, i24 (4)
	if len(want) != 24+19+0+4 {
		t.Fatalf("got %d results, want %d: %v", len(want), 24+19+4, want)
	}
	byShape := map[string]int{}
	for _, r := range want {
		f := strings.Fields(r)
		byShape[f[2]+" "+f[len(f)-1]]++
	}
	if byShape["_ <http://example.org/size>"] != 19 || byShape["_ <http://example.org/weight>"] != 4 {
		t.Fatalf("results by shape and path: %v", byShape)
	}
	sg, err := LoadTurtleString(shapes, "http://example.org/shapes")
	if err != nil {
		t.Fatal(err)
	}
	cs, err := CompileShapes(sg)
	if err != nil {
		t.Fatal(err)
	}
	check := func(got []string) {
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("report with prepared texts differs:\n got %v\nwant %v", got, want)
		}
	}
	for range 3 {
		check(report(cs.Validate(dg)))
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 5 {
				check(report(cs.Validate(dg)))
			}
		})
	}
	wg.Wait()
	// the prepared text is the text the constraint built per focus node before,
	// one entry per $PATH binding
	c := &SPARQLConstraint{Prefixes: "PREFIX ex: <http://example.org/>\n", Select: "SELECT $this ?v WHERE { $this $PATH ?v }"}
	for _, path := range []string{"<http://example.org/size>", "<http://example.org/weight>"} {
		for range 2 {
			if got, w := c.preparedQuery(path), preBindQuery(normalizeDollarVars(c.Prefixes+c.Select), map[string]string{"PATH": path}); got != w {
				t.Errorf("prepared text for %s:\n got %q\nwant %q", path, got, w)
			}
		}
	}
	if got, w := c.preparedQuery(""), preBindQuery(normalizeDollarVars(c.Prefixes+c.Select), map[string]string{}); got != w {
		t.Errorf("prepared text without $PATH:\n got %q\nwant %q", got, w)
	}
	n := 0
	c.prepared.Range(func(_, _ any) bool { n++; return true })
	if n != 3 {
		t.Errorf("%d cached texts, want 3 (two paths and none)", n)
	}
}
