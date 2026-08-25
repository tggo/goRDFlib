// Package main demonstrates SHACL validation results that point at the line of
// the data file they blame, rendered the way a compiler renders a diagnostic.
//
// SHACL reports are about a graph, and a graph has no text, so a plain report
// can only name a node. Parsing with a provenance index keeps the mapping from
// triple to source line, and shacl.WithSourceLines puts it back on the results.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/provenance"
	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/turtle"
)

const dataFile = "people.ttl"

var data = `@prefix ex:  <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:alice a ex:Person ;
    ex:name  "Alice" ;
    ex:email "alice@example.org" ;
    ex:age   34 .

ex:bob a ex:Person ;
    ex:name  "Bob" ;
    ex:email "bob-at-example-org" ;
    ex:age   "middle aged" .

ex:carol a ex:Person ;
    ex:email "carol@example.org" .
`

var shapes = `@prefix sh:  <http://www.w3.org/ns/shacl#> .
@prefix ex:  <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path     ex:name ;
        sh:minCount 1 ;
        sh:message  "a person needs a name" ;
    ] ;
    sh:property [
        sh:path     ex:email ;
        sh:pattern  "^[^@]+@[^@]+$" ;
        sh:message  "not an email address" ;
    ] ;
    sh:property [
        sh:path     ex:age ;
        sh:datatype xsd:integer ;
        sh:message  "age must be an integer" ;
    ] .
`

func main() {
	// Parse the data, recording where every triple came from. Without the
	// option the parse is identical and the index simply stays empty.
	dataGraph := graph.NewGraph()
	lines := provenance.NewIndex()
	if err := turtle.Parse(dataGraph, strings.NewReader(data), turtle.WithProvenance(lines.Triple)); err != nil {
		log.Fatal(err)
	}

	shapesGraph := graph.NewGraph()
	if err := turtle.Parse(shapesGraph, strings.NewReader(shapes)); err != nil {
		log.Fatal(err)
	}

	report := shacl.Validate(
		shacl.NewGraphFromRDF(dataGraph, ""),
		shacl.NewGraphFromRDF(shapesGraph, ""),
		shacl.WithSourceLines(lines),
	)

	fmt.Printf("conforms: %v\n", report.Conforms)
	fmt.Printf("%d violations, %d triples with a known source line\n\n",
		len(report.Results), lines.Len())

	source := strings.Split(data, "\n")

	results := append([]shacl.ValidationResult(nil), report.Results...)
	sort.Slice(results, func(i, j int) bool {
		if results[i].SourceLine != results[j].SourceLine {
			return results[i].SourceLine < results[j].SourceLine
		}
		return results[i].FocusNode.Value() < results[j].FocusNode.Value()
	})

	for _, r := range results {
		fmt.Printf("%s:%d: %s\n", filepath.Base(dataFile), r.SourceLine, message(r))

		// A line kind of "focus node" means the result blames something that is
		// absent, so the quoted line is where the resource is defined rather
		// than where the mistake is written.
		if r.SourceLine > 0 && r.SourceLine <= len(source) {
			fmt.Printf("     | %s\n", strings.TrimRight(source[r.SourceLine-1], " \t"))
		}
		fmt.Printf("     = focus node: %s\n", r.FocusNode)
		fmt.Printf("     = line points at: the %s\n\n", r.SourceLineKind)
	}

	if !report.Conforms {
		// A linter would exit non-zero here; the example keeps going so the
		// golden output stays stable.
		fmt.Fprintln(os.Stdout, "(a real linter would exit 1)")
	}
}

func message(r shacl.ValidationResult) string {
	if len(r.ResultMessages) > 0 {
		return r.ResultMessages[0].Value()
	}
	return r.SourceConstraintComponent.Value()
}
