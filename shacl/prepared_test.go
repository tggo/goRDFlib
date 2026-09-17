package shacl_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tggo/goRDFlib/shacl"
)

func preparedGraph(t *testing.T, text string) *shacl.Graph {
	t.Helper()
	g, err := shacl.LoadTurtleString(`
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`+text, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestPreparedShapeDescriptionsDoNotChangeAfterValidation(t *testing.T) {
	data := preparedGraph(t, "")
	shapes := preparedGraph(t, `
ex:Root a sh:NodeShape ; sh:targetNode ex:product ;
    sh:expression [ sh:filterShape [ sh:nodeKind sh:IRI ] ; sh:nodes sh:this ] .
`)
	prepared := shacl.Prepare(data, shapes, shacl.WithAdvancedFeatures())
	var before []shacl.ShapeInfo
	for info := range prepared.Shapes() {
		before = append(before, info)
	}
	prepared.Validate()
	var after []shacl.ShapeInfo
	for info := range prepared.Shapes() {
		after = append(after, info)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("validation changed shape descriptions: before = %+v, after = %+v", before, after)
	}
}

func TestPreparedInspectionUsesDerivedGraph(t *testing.T) {
	data := preparedGraph(t, `ex:product a ex:Product ; ex:child [ ex:name "value" ] .`)
	shapes := preparedGraph(t, `
ex:Rule a sh:NodeShape ; sh:targetClass ex:Product ;
    sh:rule [ a sh:TripleRule ; sh:subject sh:this ; sh:predicate rdf:type ; sh:object ex:Selected ] .
ex:SelectedShape a sh:NodeShape ; sh:targetClass ex:Selected ; sh:property ex:Child .
ex:Child sh:path (ex:child ex:name) ; sh:minCount 1 .
`)
	prepared := shacl.Prepare(data, shapes, shacl.WithAdvancedFeatures())
	selected := shacl.IRI("http://example.org/SelectedShape")
	child := shacl.IRI("http://example.org/Child")
	product := shacl.IRI("http://example.org/product")
	var shapeIDs []shacl.Term
	for info := range prepared.Shapes() {
		shapeIDs = append(shapeIDs, info.ID)
	}
	if len(shapeIDs) != 3 {
		t.Fatalf("shape IDs = %v, want three parsed shapes", shapeIDs)
	}
	info, ok := prepared.Shape(child)
	if !ok || !info.IsProperty || !info.Path.IsBlank() || info.Deactivated {
		t.Fatalf("property shape = %+v, found = %v", info, ok)
	}
	var children []shacl.Term
	for via, id := range prepared.ShapeLinks(selected) {
		if via != shacl.IRI(shacl.SH+"property") {
			t.Fatalf("unexpected structural link %v", via)
		}
		children = append(children, id)
	}
	if !reflect.DeepEqual(children, []shacl.Term{child}) {
		t.Fatalf("children = %v", children)
	}
	targets := prepared.Targets(selected)
	if !reflect.DeepEqual(targets, []shacl.Term{product}) {
		t.Fatalf("derived targets = %v", targets)
	}
	targets[0] = shacl.IRI("urn:changed")
	if got := prepared.Targets(selected); !reflect.DeepEqual(got, []shacl.Term{product}) {
		t.Fatalf("target results alias prepared state: %v", got)
	}
	wantValues := []shacl.Term{shacl.Literal("value", shacl.XSD+"string", "")}
	for _, values := range [][]shacl.Term{prepared.ValueNodes(child, product), prepared.PathValues(info.Path, product)} {
		if !reflect.DeepEqual(values, wantValues) {
			t.Fatalf("path values = %v, want %v", values, wantValues)
		}
		values[0] = shacl.IRI("urn:changed")
	}
	if got := prepared.ValueNodes(child, product); !reflect.DeepEqual(got, wantValues) {
		t.Fatalf("value results alias prepared state: %v", got)
	}
	minimum := shacl.IRI(shacl.SH + "minCount")
	parameters := prepared.ShapeObjects(child, minimum)
	if len(parameters) != 1 || parameters[0].Value() != "1" {
		t.Fatalf("minimum = %v", parameters)
	}
	parameters[0] = shacl.IRI("urn:changed")
	if prepared.ShapeObjects(child, minimum)[0].Value() != "1" {
		t.Fatal("shape parameters alias the shapes graph")
	}
	if report := prepared.Validate(); !report.Conforms {
		t.Fatalf("inspection changed validation: %+v", report)
	}
}

func TestPreparedPreservesErrorReporting(t *testing.T) {
	data := preparedGraph(t, "")
	shapes := preparedGraph(t, `
ex:Root a sh:NodeShape ; sh:targetNode ex:product ; sh:rule [ a sh:TripleRule ] .
`)
	var diagnostics []error
	prepared := shacl.Prepare(data, shapes, shacl.WithAdvancedFeatures(), shacl.WithErrorHandler(func(err error) {
		diagnostics = append(diagnostics, err)
	}))
	if len(diagnostics) != 1 || !errors.Is(diagnostics[0], shacl.ErrMalformedRule) {
		t.Fatalf("preparation diagnostics = %v", diagnostics)
	}
	prepared.Validate()
	if len(diagnostics) != 1 {
		t.Fatalf("validation repeated rule preparation: %v", diagnostics)
	}
}

func TestPreparedSharesTargetSelection(t *testing.T) {
	for _, inspectFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "validate first", true: "inspect first"}[inspectFirst], func(t *testing.T) {
			data := preparedGraph(t, "")
			shapes := preparedGraph(t, `
ex:Selected a sh:NodeShape ; sh:nodeKind sh:IRI ;
    sh:target [ a sh:SPARQLTarget ; sh:select "SELECT (BNODE() AS ?this) WHERE {}" ] .
`)
			prepared := shacl.Prepare(data, shapes, shacl.WithAdvancedFeatures())
			shape := shacl.IRI("http://example.org/Selected")
			var targets []shacl.Term
			if inspectFirst {
				targets = prepared.Targets(shape)
			}
			report := prepared.Validate()
			if !inspectFirst {
				targets = prepared.Targets(shape)
			}
			if len(targets) != 1 || len(report.Results) != 1 || targets[0] != report.Results[0].FocusNode {
				t.Fatalf("selection was not shared: targets = %v, report = %+v", targets, report)
			}
			if again := prepared.Validate(); !reflect.DeepEqual(again, report) {
				t.Fatalf("targets changed on repeated validation: %+v", again)
			}
		})
	}
}

func TestPreparedValueNodesUseComputedValues(t *testing.T) {
	data := preparedGraph(t, `ex:product ex:actual [ ex:name "name" ] .`)
	shapes := preparedGraph(t, `
ex:Computed a sh:PropertyShape ; sh:targetNode ex:product ;
    sh:path ex:absent ; sh:minCount 1 ;
    sh:values [ sh:select "SELECT ?value WHERE { $this <http://example.org/actual> ?value }" ] .
`)
	prepared := shacl.Prepare(data, shapes)
	product := shacl.IRI("http://example.org/product")
	shape := shacl.IRI("http://example.org/Computed")
	values := prepared.ValueNodes(shape, product)
	if len(values) != 1 || !values[0].IsBlank() {
		t.Fatalf("computed values = %v", values)
	}
	if got := prepared.PathValues(shacl.IRI("http://example.org/absent"), product); len(got) != 0 {
		t.Fatalf("ordinary path unexpectedly returned %v", got)
	}
	if report := prepared.Validate(); !report.Conforms {
		t.Fatalf("validation did not use computed values: %+v", report)
	}
}

func TestPreparedValidationDerivesOnce(t *testing.T) {
	data := preparedGraph(t, `ex:product a ex:Product .`)
	shapes := preparedGraph(t, `
ex:Counter a sh:NodeShape ;
    sh:targetClass ex:Product ;
    sh:rule [ a sh:SPARQLRule ;
        sh:construct """
            PREFIX ex: <http://example.org/>
            CONSTRUCT { $this ex:step ?next }
            WHERE {
                OPTIONAL { $this ex:step ?step }
                BIND(COALESCE(?step, 0) + 1 AS ?next)
            }
        """ ] ;
    sh:property [ sh:path ex:step ; sh:minCount 1 ; sh:maxCount 1 ; sh:maxInclusive 1 ] .
`)
	prepared := shacl.Prepare(data, shapes, shacl.WithAdvancedFeatures())
	for range 2 {
		if report := prepared.Validate(); !report.Conforms || len(report.Results) != 0 {
			t.Fatalf("prepared validation reran or missed derivation: %+v", report)
		}
	}
	legacy := shacl.Validate(data, shapes, shacl.WithAdvancedFeatures())
	if got := prepared.Validate(); !reflect.DeepEqual(got, legacy) {
		t.Fatalf("prepared report = %+v, legacy report = %+v", got, legacy)
	}
	if data.Len() != 1 {
		t.Fatalf("preparation modified caller data: %v", data.Triples())
	}
}
