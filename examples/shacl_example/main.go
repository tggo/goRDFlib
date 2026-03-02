// Package main demonstrates SHACL validation with shapes and data graphs.
// Inspired by: pySHACL/examples/two_file_example.py
package main

import (
	"fmt"
	"sort"

	"github.com/tggo/goRDFlib/shacl"
)

var shapesData = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix schema: <http://schema.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

schema:PersonShape
    a sh:NodeShape ;
    sh:targetClass schema:Person ;
    sh:property [
        sh:path schema:givenName ;
        sh:datatype xsd:string ;
        sh:minCount 1 ;
        sh:name "given name" ;
    ] ;
    sh:property [
        sh:path schema:birthDate ;
        sh:maxCount 1 ;
        sh:datatype xsd:date ;
    ] ;
    sh:property [
        sh:path schema:gender ;
        sh:in ( "female" "male" ) ;
    ] .
`

var validData = `
@prefix schema: <http://schema.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

<http://example.org/Alice>
    a schema:Person ;
    schema:givenName "Alice" ;
    schema:birthDate "1990-01-01"^^xsd:date ;
    schema:gender "female" .
`

var invalidData = `
@prefix schema: <http://schema.org/> .

<http://example.org/Bob>
    a schema:Person ;
    schema:gender "unknown" .
`

func main() {
	shapesGraph, err := shacl.LoadTurtleString(shapesData, "")
	if err != nil {
		panic(err)
	}

	// Validate conforming data.
	fmt.Println("=== Valid data ===")
	dataGraph, err := shacl.LoadTurtleString(validData, "")
	if err != nil {
		panic(err)
	}
	report := shacl.Validate(dataGraph, shapesGraph)
	fmt.Printf("Conforms: %v\n", report.Conforms)
	fmt.Printf("Violations: %d\n", len(report.Results))

	// Validate non-conforming data.
	fmt.Println("\n=== Invalid data ===")
	dataGraph2, err := shacl.LoadTurtleString(invalidData, "")
	if err != nil {
		panic(err)
	}
	report2 := shacl.Validate(dataGraph2, shapesGraph)
	fmt.Printf("Conforms: %v\n", report2.Conforms)
	fmt.Printf("Violations: %d\n", len(report2.Results))
	sort.Slice(report2.Results, func(i, j int) bool {
		return report2.Results[i].ResultPath.String() < report2.Results[j].ResultPath.String()
	})
	for i, r := range report2.Results {
		fmt.Printf("\nViolation %d:\n", i+1)
		fmt.Printf("  Focus node: %s\n", r.FocusNode)
		if r.ResultPath.String() != "" {
			fmt.Printf("  Path: %s\n", r.ResultPath)
		}
		if r.Value.String() != "" {
			fmt.Printf("  Value: %s\n", r.Value)
		}
		if r.SourceConstraintComponent.String() != "" {
			fmt.Printf("  Constraint: %s\n", r.SourceConstraintComponent)
		}
	}
}
