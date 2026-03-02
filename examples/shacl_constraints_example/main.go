// Package main demonstrates various SHACL constraint types.
// Inspired by: pySHACL/examples/two_file_example.py
package main

import (
	"fmt"
	"sort"

	"github.com/tggo/goRDFlib/shacl"
)

var shapesData = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:ProductShape
    a sh:NodeShape ;
    sh:targetClass ex:Product ;
    sh:property [
        sh:path ex:name ;
        sh:datatype xsd:string ;
        sh:minCount 1 ;
        sh:maxCount 1 ;
    ] ;
    sh:property [
        sh:path ex:price ;
        sh:datatype xsd:decimal ;
        sh:minInclusive 0 ;
    ] ;
    sh:property [
        sh:path ex:category ;
        sh:minCount 1 ;
        sh:nodeKind sh:IRI ;
    ] ;
    sh:property [
        sh:path ex:status ;
        sh:in ( "active" "discontinued" "draft" ) ;
    ] .

ex:AddressShape
    a sh:NodeShape ;
    sh:targetClass ex:Address ;
    sh:closed true ;
    sh:ignoredProperties ( <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> ) ;
    sh:property [
        sh:path ex:street ;
        sh:datatype xsd:string ;
        sh:minCount 1 ;
    ] ;
    sh:property [
        sh:path ex:city ;
        sh:datatype xsd:string ;
        sh:minCount 1 ;
    ] .
`

var dataGraph = `
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:Widget
    a ex:Product ;
    ex:name "Super Widget" ;
    ex:price "29.99"^^xsd:decimal ;
    ex:category ex:Electronics ;
    ex:status "active" .

ex:Gadget
    a ex:Product ;
    ex:price "-5.00"^^xsd:decimal ;
    ex:status "unknown" .

ex:Office
    a ex:Address ;
    ex:street "123 Main St" ;
    ex:city "Springfield" ;
    ex:country "US" .
`

func main() {
	shapes, err := shacl.LoadTurtleString(shapesData, "")
	if err != nil {
		panic(err)
	}
	data, err := shacl.LoadTurtleString(dataGraph, "")
	if err != nil {
		panic(err)
	}

	report := shacl.Validate(data, shapes)
	fmt.Printf("Conforms: %v\n", report.Conforms)
	fmt.Printf("Total violations: %d\n", len(report.Results))

	// Sort for deterministic output.
	sort.Slice(report.Results, func(i, j int) bool {
		a, b := report.Results[i], report.Results[j]
		if a.FocusNode.String() != b.FocusNode.String() {
			return a.FocusNode.String() < b.FocusNode.String()
		}
		return a.ResultPath.String() < b.ResultPath.String()
	})

	for i, r := range report.Results {
		fmt.Printf("\n[%d] %s\n", i+1, r.FocusNode)
		if r.ResultPath.String() != "" {
			fmt.Printf("    Path: %s\n", r.ResultPath)
		}
		if r.Value.String() != "" {
			fmt.Printf("    Value: %s\n", r.Value)
		}
		fmt.Printf("    Constraint: %s\n", r.SourceConstraintComponent)
	}
}
