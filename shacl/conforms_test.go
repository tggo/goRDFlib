package shacl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

const conformsShapes = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <http://example.org/> .

ex:Named a sh:NodeShape ;
	sh:targetClass ex:NeverUsed ;
	sh:property [ sh:path ex:name ; sh:minCount 1 ; sh:datatype xsd:string ] .
ex:Adult a sh:NodeShape ;
	sh:property [ sh:path ex:age ; sh:minInclusive 18 ] .
ex:NamedAdult a sh:NodeShape ; sh:and ( ex:Named ex:Adult ) .
ex:NamedOrAdult a sh:NodeShape ; sh:or ( ex:Named ex:Adult ) .
ex:OnlyOne a sh:NodeShape ; sh:xone ( ex:Named ex:Adult ) .
ex:NotNamed a sh:NodeShape ; sh:not ex:Named .
ex:Team a sh:NodeShape ;
	sh:property [ sh:path ex:member ; sh:node ex:Named ] ;
	sh:property [ sh:path ex:member ; sh:qualifiedValueShape ex:Adult ; sh:qualifiedMinCount 1 ] .
ex:Chain a sh:NodeShape ;
	sh:property [ sh:path ex:next ; sh:node ex:Chain ] ;
	sh:property [ sh:path ex:name ; sh:minCount 1 ] .
ex:Off a sh:NodeShape ; sh:deactivated true ; sh:property [ sh:path ex:name ; sh:minCount 5 ] .
ex:NoTypeShape sh:property [ sh:path ex:name ; sh:maxCount 1 ] .
`

const conformsData = `
@prefix ex: <http://example.org/> .
ex:ann ex:name "Ann" ; ex:age 30 .
ex:bob ex:name "Bob" ; ex:age 12 .
ex:cid ex:age 40 .
ex:dan ex:name 7 .
ex:t1 ex:member ex:ann, ex:bob .
ex:t2 ex:member ex:bob .
ex:t3 ex:member ex:cid .
ex:c1 ex:name "1" ; ex:next ex:c2 .
ex:c2 ex:name "2" ; ex:next ex:c1 .
ex:c3 ex:name "3" ; ex:next ex:c4 .
ex:c4 ex:next ex:c3 .
`

var (
	conformsNodes  = []string{"ann", "bob", "cid", "dan", "t1", "t2", "t3", "c1", "c3", "c4", "nobody"}
	conformsShapeN = []string{"Named", "Adult", "NamedAdult", "NamedOrAdult", "OnlyOne", "NotNamed", "Team", "Chain", "Off", "NoTypeShape"}
)

func exT(local string) Term { return IRI("http://example.org/" + local) }

func prepareConforms(t *testing.T) *Prepared {
	t.Helper()
	shapes, err := LoadTurtleString(conformsShapes, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := LoadTurtleString(conformsData, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := PrepareContext(context.Background(), data, shapes)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Issue #44.
func TestConforms(t *testing.T) {
	p := prepareConforms(t)
	tests := []struct {
		node, shape string
		want        bool
	}{
		{"ann", "Named", true}, // targets (ex:NeverUsed) are ignored
		{"cid", "Named", false},
		{"dan", "Named", false},
		{"bob", "Adult", false},
		{"nobody", "Adult", true}, // no values, no violation
		{"ann", "NamedAdult", true},
		{"bob", "NamedAdult", false},
		{"cid", "NamedOrAdult", true},
		{"dan", "NamedOrAdult", true}, // no ex:age: Adult holds vacuously
		{"dan", "NotNamed", true},
		{"ann", "OnlyOne", false},
		{"bob", "OnlyOne", true},
		{"cid", "NotNamed", true},
		{"ann", "NotNamed", false},
		{"t1", "Team", true},
		{"t2", "Team", false}, // no adult member
		{"t3", "Team", false}, // member without a name
		{"c1", "Chain", true}, // cycle, all named
		{"c3", "Chain", false},
		{"c4", "Chain", false},
		{"bob", "Off", true},
		{"ann", "NoTypeShape", true},
	}
	for _, tt := range tests {
		t.Run(tt.node+"/"+tt.shape, func(t *testing.T) {
			got, err := p.Conforms(context.Background(), exT(tt.node), exT(tt.shape))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Conforms = %v, want %v", got, tt.want)
			}
		})
	}
}

// Conforms must agree with the workaround it replaces: a wrapper shape that
// targets only the node and requires sh:node of the shape, run through Validate.
func TestConformsAgreesWithValidate(t *testing.T) {
	p := prepareConforms(t)
	data, _ := LoadTurtleString(conformsData, "")
	for _, s := range conformsShapeN {
		for _, n := range conformsNodes {
			got, err := p.Conforms(context.Background(), exT(n), exT(s))
			if err != nil {
				t.Fatal(err)
			}
			shapes, err := LoadTurtleString(conformsShapes+fmt.Sprintf(`
ex:Wrapper sh:targetNode ex:%s ; sh:node ex:%s .`, n, s), "")
			if err != nil {
				t.Fatal(err)
			}
			want := true
			for _, r := range Validate(data, shapes).Results {
				if r.SourceShape.Equal(exT("Wrapper")) {
					want = false
				}
			}
			if got != want {
				t.Errorf("Conforms(%s, %s) = %v, Validate says %v", n, s, got, want)
			}
		}
	}
}

func TestConformsBlankNodeShape(t *testing.T) {
	p := prepareConforms(t)
	var prop Term
	for _, o := range p.ShapeObjects(exT("Adult"), IRI(SH+"property")) {
		prop = o
	}
	if !prop.IsBlank() {
		t.Fatalf("no blank property shape found")
	}
	// A property shape's focus node is the resource whose values it checks.
	for node, want := range map[string]bool{"ann": true, "bob": false} {
		got, err := p.Conforms(context.Background(), exT(node), prop)
		if err != nil || got != want {
			t.Errorf("Conforms(%s, _:prop) = %v, %v; want %v", node, got, err, want)
		}
	}
}

func TestConformsErrors(t *testing.T) {
	p := prepareConforms(t)
	if _, err := p.Conforms(context.Background(), exT("ann"), exT("NoSuchShape")); !errors.Is(err, ErrUnknownShape) {
		t.Errorf("unknown shape: err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Conforms(ctx, exT("ann"), exT("Named")); !errors.Is(err, ErrCancelled) || !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: err = %v", err)
	}
}

func TestConformsConcurrent(t *testing.T) {
	p := prepareConforms(t)
	want := make(map[[2]string]bool)
	for _, s := range conformsShapeN {
		for _, n := range conformsNodes {
			ok, err := p.Conforms(context.Background(), exT(n), exT(s))
			if err != nil {
				t.Fatal(err)
			}
			want[[2]string{n, s}] = ok
		}
	}
	shapesBefore := len(p.ctx.shapesMap)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for k, w := range want {
				if got, err := p.Conforms(context.Background(), exT(k[0]), exT(k[1])); err != nil || got != w {
					t.Errorf("Conforms(%s, %s) = %v, %v; want %v", k[0], k[1], got, err, w)
				}
			}
		})
	}
	wg.Wait()
	if len(p.ctx.shapesMap) != shapesBefore {
		t.Errorf("shape map grew from %d to %d", shapesBefore, len(p.ctx.shapesMap))
	}
}

func TestCompiledShapesConforms(t *testing.T) {
	shapes, _ := LoadTurtleString(conformsShapes, "")
	data, _ := LoadTurtleString(conformsData, "")
	c, err := CompileShapes(shapes)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Conforms(context.Background(), data, exT("t1"), exT("Team"))
	if err != nil || !got {
		t.Errorf("Conforms(t1, Team) = %v, %v; want true", got, err)
	}
	got, err = c.Conforms(context.Background(), data, exT("t2"), exT("Team"))
	if err != nil || got {
		t.Errorf("Conforms(t2, Team) = %v, %v; want false", got, err)
	}
}

// SHACL §2.1.5: every node conforms to a deactivated shape, also when the shape
// is reached through sh:node rather than through its own targets. Found while
// cross-checking Conforms against Validate.
func TestNodeToDeactivatedShapeConforms(t *testing.T) {
	shapes, _ := LoadTurtleString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:Off a sh:NodeShape ; sh:deactivated true ; sh:property [ sh:path ex:name ; sh:minCount 1 ] .
ex:S a sh:NodeShape ; sh:targetNode ex:a ;
	sh:node ex:Off ; sh:and ( ex:Off ) ; sh:or ( ex:Off ) ; sh:not [ sh:not ex:Off ] .
`, "")
	if r := Validate(NewGraph(), shapes); !r.Conforms {
		t.Errorf("got %d results, want a conforming report: %+v", len(r.Results), r.Results)
	}
}
