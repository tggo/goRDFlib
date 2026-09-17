package shaclwalk_test

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/shacl/shaclwalk"
)

const prefixes = `
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`

func graph(t testing.TB, text string) *shacl.Graph {
	t.Helper()
	g, err := shacl.LoadTurtleString(prefixes+text, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func iri(name string) shacl.Term { return shacl.IRI("http://example.org/" + name) }

func TestWalkUsesSingleDerivationAndPreservesNestedTargets(t *testing.T) {
	data := graph(t, `ex:product ex:language [ ex:en "name" ] . ex:independentMap ex:en "other" .`)
	shapes := graph(t, `
ex:Trigger a sh:NodeShape ; sh:targetNode ex:product ;
    sh:rule [ a sh:SPARQLRule ; sh:construct """
        PREFIX ex: <http://example.org/>
        CONSTRUCT { $this a ex:Selected ; ex:step ?next }
        WHERE { OPTIONAL { $this ex:step ?step } BIND(COALESCE(?step, 0) + 1 AS ?next) }
    """ ] ; sh:property ex:Step .
ex:Step sh:path ex:step ; sh:minCount 1 ; sh:maxCount 1 ; sh:maxInclusive 1 .
ex:Root a sh:NodeShape ; sh:targetClass ex:Selected ; sh:property ex:Map .
ex:Map sh:path ex:language ; sh:node ex:Fields .
ex:Fields sh:targetNode ex:independentMap ; sh:property ex:Name .
ex:Name sh:path ex:nl ; sh:minCount 1 .
`)
	beforeSize := data.Len()
	prepared := shacl.Prepare(data, shapes, shacl.WithAdvancedFeatures())
	before := prepared.Validate()
	var applications []shaclwalk.Event
	for range 2 {
		applications = nil
		shaclwalk.Walk(prepared, func(event shaclwalk.Event) {
			if event.Phase == shaclwalk.Enter && event.Shape == iri("Name") {
				applications = append(applications, event)
			}
		})
		if len(applications) != 2 {
			t.Fatalf("applications = %+v", applications)
		}
		for _, event := range applications {
			if event.SelectedShape == iri("Root") {
				if event.SelectedFocus != iri("product") || !event.Focus.IsBlank() {
					t.Fatalf("nested shape replaced its enclosing context: %+v", event)
				}
			} else if event.SelectedShape != iri("Fields") || event.SelectedFocus != iri("independentMap") || event.Focus != iri("independentMap") {
				t.Fatalf("unexpected independent application: %+v", event)
			}
		}
	}
	steps := prepared.ValueNodes(iri("Step"), iri("product"))
	if len(steps) != 1 || steps[0].Value() != "1" || data.Len() != beforeSize {
		t.Fatalf("walking reran rules or mutated source data: steps = %v, data = %v", steps, data.Triples())
	}
	if after := prepared.Validate(); !reflect.DeepEqual(before, after) {
		t.Fatalf("report changed: before = %+v, after = %+v", before, after)
	}
}

func TestWalkEmptyScopesAndNilVisitor(t *testing.T) {
	shaclwalk.Walk(nil, nil)
	data := graph(t, "")
	shapes := graph(t, `
ex:Root a sh:NodeShape ; sh:targetNode ex:product ; sh:property ex:Map .
ex:Map sh:path ex:missing ; sh:node [ sh:property ex:Child ] .
ex:Child sh:path ex:child .
`)
	prepared := shacl.Prepare(data, shapes)
	var fields []shacl.Term
	shaclwalk.Walk(prepared, func(event shaclwalk.Event) {
		info, _ := prepared.Shape(event.Shape)
		if event.Phase == shaclwalk.Enter && info.IsProperty {
			fields = append(fields, info.Path)
		}
	})
	if !reflect.DeepEqual(fields, []shacl.Term{iri("missing")}) {
		t.Fatalf("empty scope created child applications: %v", fields)
	}
}

func TestWalkIndependentRunsShareImmutableSources(t *testing.T) {
	data := graph(t, `ex:product ex:child [ ex:name "value" ] .`)
	shapes := graph(t, `
ex:Root a sh:NodeShape ; sh:targetNode ex:product ;
    sh:property [ sh:path ex:child ; sh:node [ sh:property ex:Name ] ] .
ex:Name sh:path ex:name ; sh:minCount 1 .
`)
	for i := range 8 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			prepared := shacl.Prepare(data, shapes)
			count := 0
			shaclwalk.Walk(prepared, func(event shaclwalk.Event) {
				if event.Phase == shaclwalk.Enter && event.Shape == iri("Name") {
					count++
				}
			})
			if count != 1 || !prepared.Validate().Conforms {
				t.Fatalf("independent run has %d name applications or failed validation", count)
			}
		})
	}
}

func TestWalkAllBooleanBranchesAndRepeatedUses(t *testing.T) {
	data := graph(t, "")
	shapes := graph(t, `
ex:Root a sh:NodeShape ; sh:targetNode ex:product ;
    sh:property ex:Shared ;
    sh:and (ex:Fail ex:AfterAnd) ;
    sh:or (ex:Pass ex:AfterOr [ sh:property ex:Shared ]) ;
    sh:xone (ex:Pass ex:AfterXone) ;
    sh:not [ sh:not ex:AfterNot ] .
ex:Fail sh:property [ sh:path ex:missing ; sh:minCount 1 ] .
ex:Pass a sh:NodeShape .
ex:AfterAnd sh:property [ sh:path ex:a ] .
ex:AfterOr sh:property [ sh:path ex:b ; sh:minCount 1 ] .
ex:AfterXone sh:property [ sh:path ex:c ; sh:minCount 1 ] .
ex:AfterNot sh:property [ sh:path ex:d ; sh:minCount 1 ] .
ex:Shared sh:path ex:shared ; sh:minCount 1 .
`)
	prepared := shacl.Prepare(data, shapes)
	before := prepared.Validate()
	paths := make(map[shacl.Term][][]shacl.Term)
	var context []shacl.Term
	shaclwalk.Walk(prepared, func(event shaclwalk.Event) {
		switch event.Phase {
		case shaclwalk.Enter:
			context = append(context, event.Via)
			info, _ := prepared.Shape(event.Shape)
			if info.IsProperty {
				paths[info.Path] = append(paths[info.Path], slices.Clone(context))
			}
		case shaclwalk.Leave:
			context = context[:len(context)-1]
		default:
			t.Fatalf("unexpected event: %+v", event)
		}
	})
	for field, via := range map[string]string{"a": "and", "b": "or", "c": "xone", "d": "not"} {
		contexts := paths[iri(field)]
		if len(contexts) != 1 || !slices.Contains(contexts[0], shacl.IRI(shacl.SH+via)) {
			t.Fatalf("field %s lost its branch context: %v", field, contexts)
		}
	}
	notCount := 0
	for _, via := range paths[iri("d")][0] {
		if via == shacl.IRI(shacl.SH+"not") {
			notCount++
		}
	}
	if notCount != 2 {
		t.Fatalf("nested negation context has %d not links", notCount)
	}
	shared := paths[iri("shared")]
	if len(shared) != 2 || slices.Contains(shared[0], shacl.IRI(shacl.SH+"or")) || !slices.Contains(shared[1], shacl.IRI(shacl.SH+"or")) {
		t.Fatalf("repeated property applications lost their contexts: %v", shared)
	}
	if after := prepared.Validate(); !reflect.DeepEqual(after, before) {
		t.Fatalf("walking changed logical validation: before = %+v, after = %+v", before, after)
	}
}

func TestWalkSkipsInactiveAndNonStructuralShapes(t *testing.T) {
	data := graph(t, `ex:product a ex:Product .`)
	shapes := graph(t, `
ex:Root a sh:NodeShape ; sh:targetNode ex:product ;
    sh:property ex:Visible, ex:InactiveProperty ; sh:node ex:InactiveNode ;
    sh:rule [ a sh:TripleRule ; sh:subject sh:this ; sh:predicate ex:marker ; sh:object true ; sh:condition ex:Condition ] .
ex:Visible sh:path ex:visible .
ex:InactiveProperty sh:path ex:hidden ; sh:deactivated true .
ex:InactiveNode sh:deactivated true ; sh:property [ sh:path ex:hiddenChild ] .
ex:InactiveRoot a sh:NodeShape ; sh:targetNode ex:product ; sh:deactivated true ; sh:property [ sh:path ex:hiddenRoot ] .
ex:Condition sh:property [ sh:path ex:conditionOnly ; sh:minCount 1 ] .
ex:Opaque a sh:NodeShape ; sh:targetNode ex:product ; sh:sparql [ sh:select "broken query" ] .
`)
	var diagnostics []error
	prepared := shacl.Prepare(data, shapes, shacl.WithAdvancedFeatures(), shacl.WithErrorHandler(func(err error) {
		diagnostics = append(diagnostics, err)
	}))
	var fields []shacl.Term
	shaclwalk.Walk(prepared, func(event shaclwalk.Event) {
		if event.Phase == shaclwalk.Enter {
			info, _ := prepared.Shape(event.Shape)
			if info.Deactivated {
				t.Fatalf("visited an inactive shape: %+v", event)
			}
			if info.IsProperty {
				fields = append(fields, info.Path)
			}
		}
	})
	if !reflect.DeepEqual(fields, []shacl.Term{iri("visible")}) || len(diagnostics) != 0 {
		t.Fatalf("fields = %v, diagnostics = %v", fields, diagnostics)
	}
}

func TestWalkPropertyScopesAndComputedValues(t *testing.T) {
	for _, scope := range []string{
		`sh:path ex:child`,
		`sh:path ex:absent ; sh:values [ sh:select "SELECT ?value WHERE { $this <http://example.org/child> ?value }" ]`,
	} {
		t.Run(scope, func(t *testing.T) {
			data := graph(t, `ex:product ex:child ex:map .`)
			shapes := graph(t, `
ex:Scope a sh:PropertyShape ; sh:targetNode ex:product ; `+scope+` ;
    sh:property ex:Direct ; sh:node [ sh:property ex:Nested ] .
ex:Direct sh:path ex:direct .
ex:Nested sh:path ex:nested .
`)
			prepared := shacl.Prepare(data, shapes)
			fields := make(map[shacl.Term]shacl.Term)
			shaclwalk.Walk(prepared, func(event shaclwalk.Event) {
				if event.Phase == shaclwalk.Enter && (event.Shape == iri("Direct") || event.Shape == iri("Nested")) {
					fields[event.Shape] = event.Focus
				}
			})
			want := map[shacl.Term]shacl.Term{iri("Direct"): iri("map"), iri("Nested"): iri("map")}
			if !reflect.DeepEqual(fields, want) {
				t.Fatalf("property scopes = %v, want %v", fields, want)
			}
		})
	}
}

func TestWalkReportsCyclesWithoutLosingOtherOrigins(t *testing.T) {
	data := graph(t, `ex:a ex:next ex:b . ex:b ex:next ex:a .`)
	shapes := graph(t, `
ex:Root a sh:NodeShape ; sh:targetNode ex:a, ex:b ; sh:node ex:Recursive .
ex:Recursive sh:property ex:Name, ex:Next .
ex:Name sh:path ex:name .
ex:Next sh:path ex:next ; sh:node ex:Recursive .
`)
	prepared := shacl.Prepare(data, shapes)
	cycles := make(map[shacl.Term]int)
	owners := make(map[shacl.Term]int)
	balance := 0
	shaclwalk.Walk(prepared, func(event shaclwalk.Event) {
		switch event.Phase {
		case shaclwalk.Enter:
			balance++
			if event.Shape == iri("Name") {
				owners[event.Focus]++
			}
		case shaclwalk.Leave:
			balance--
		case shaclwalk.Cycle:
			if event.Shape != iri("Recursive") || event.Focus != event.SelectedFocus {
				t.Fatalf("unexpected cycle: %+v", event)
			}
			cycles[event.SelectedFocus]++
		}
	})
	if !reflect.DeepEqual(cycles, map[shacl.Term]int{iri("a"): 1, iri("b"): 1}) {
		t.Fatalf("cycles = %v", cycles)
	}
	if !reflect.DeepEqual(owners, map[shacl.Term]int{iri("a"): 2, iri("b"): 2}) || balance != 0 {
		t.Fatalf("owners = %v, balance = %d", owners, balance)
	}
}

func TestWalkNestedApplications(t *testing.T) {
	for _, filled := range []bool{false, true} {
		name, extra := "missing values", ""
		if filled {
			name, extra = "valid values", `_:name ex:nl "product" . ex:categoryName ex:nl "category" .`
		}
		t.Run(name, func(t *testing.T) {
			data := graph(t, `
ex:product a ex:ProductSoldInNL ; ex:language _:name ; ex:reference ex:category .
_:name ex:en "product" .
ex:category ex:language ex:categoryName ; ex:reference ex:category .
ex:categoryName ex:en "category" .
ex:unrelated ex:language ex:unselected .
`+extra)
			shapes := graph(t, `
ex:DutchLocaleRequired a sh:NodeShape ; sh:targetClass ex:ProductSoldInNL ;
    sh:property [ sh:path ([ sh:zeroOrMorePath ex:reference ] ex:language) ; sh:node ex:Localized ] .
ex:Localized sh:property ex:DutchKey .
ex:DutchKey sh:path ex:nl ; sh:minCount 1 .
`)
			prepared := shacl.Prepare(data, shapes)
			before := prepared.Validate()
			if before.Conforms != filled {
				t.Fatalf("unexpected validation outcome: %+v", before)
			}
			owners := make(map[shacl.Term]int)
			var stack []shaclwalk.Event
			shaclwalk.Walk(prepared, func(event shaclwalk.Event) {
				if event.Phase == shaclwalk.Enter {
					stack = append(stack, event)
					if event.Shape == iri("DutchKey") {
						if event.SelectedShape != iri("DutchLocaleRequired") || event.SelectedFocus != iri("product") || event.Via != shacl.IRI(shacl.SH+"property") {
							t.Fatalf("lost source context: %+v", event)
						}
						owners[event.Focus]++
					}
				} else if event.Phase == shaclwalk.Leave {
					if len(stack) == 0 {
						t.Fatal("leave without enter")
					}
					enter := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					enter.Phase = shaclwalk.Leave
					if enter != event {
						t.Fatalf("unbalanced events: enter = %+v, leave = %+v", enter, event)
					}
				}
			})
			productMap := data.Objects(iri("product"), iri("language"))[0]
			want := map[shacl.Term]int{productMap: 1, iri("categoryName"): 1}
			if !reflect.DeepEqual(owners, want) || len(stack) != 0 {
				t.Fatalf("owners = %v, want %v; remaining events = %v", owners, want, stack)
			}
			if after := prepared.Validate(); !reflect.DeepEqual(before, after) {
				t.Fatalf("walking changed the report: before = %+v, after = %+v", before, after)
			}
		})
	}
}
