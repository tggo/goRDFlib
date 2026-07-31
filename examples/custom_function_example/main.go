// Package main demonstrates SPARQL extension functions (SPARQL 1.1 §17.6) —
// the Go counterpart of rdflib's register_custom_function.
//
// A function is bound to an IRI once, then any query can call it either by the
// full IRI or through a prefixed name that resolves to it.
package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	rdf "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/turtle"
)

const geoNS = "http://example.org/geo#"

// distance computes the plain Euclidean distance between two "x,y" points. A
// real geo extension would parse WKT here; the registration mechanics are the
// same.
func distance(args []rdf.Term) (rdf.Term, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("geo:distance: want 2 arguments, got %d", len(args))
	}
	x1, y1, err := point(args[0])
	if err != nil {
		return nil, err
	}
	x2, y2, err := point(args[1])
	if err != nil {
		return nil, err
	}
	return rdf.NewLiteral(math.Hypot(x2-x1, y2-y1)), nil
}

func point(t rdf.Term) (x, y float64, err error) {
	lit, ok := t.(rdf.Literal)
	if !ok {
		return 0, 0, fmt.Errorf("geo:distance: want a literal, got %T", t)
	}
	parts := strings.Split(lit.Lexical(), ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("geo:distance: %q is not an \"x,y\" point", lit.Lexical())
	}
	if x, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err != nil {
		return 0, 0, fmt.Errorf("geo:distance: %w", err)
	}
	if y, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err != nil {
		return 0, 0, fmt.Errorf("geo:distance: %w", err)
	}
	return x, y, nil
}

func main() {
	// Register once — typically from an init function in the extension package.
	sparql.MustRegisterFunction(geoNS+"distance", distance)

	g := rdf.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(`
		@prefix ex: <http://example.org/> .

		ex:office ex:at "0,0" .
		ex:cafe   ex:at "3,4" .
		ex:depot  ex:at "30,40" .
	`)); err != nil {
		panic(err)
	}

	// Call through a prefixed name, in both SELECT and FILTER position.
	res, err := sparql.Query(g, `
		PREFIX ex:  <http://example.org/>
		PREFIX geo: <http://example.org/geo#>

		SELECT ?place ?dist WHERE {
			ex:office ex:at ?from .
			?place ex:at ?to .
			FILTER(?place != ex:office)
			BIND(geo:distance(?from, ?to) AS ?dist)
			FILTER(?dist < 100)
		}
		ORDER BY ?dist
	`)
	if err != nil {
		panic(err)
	}

	fmt.Println("Places within 100 units of the office:")
	for _, b := range res.Bindings {
		fmt.Printf("  %s at distance %s\n", b["place"], b["dist"])
	}

	// The same function called by its full IRI.
	res, err = sparql.Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?d WHERE {
			ex:office ex:at ?a .
			ex:depot  ex:at ?b .
			BIND(<http://example.org/geo#distance>(?a, ?b) AS ?d)
		}
	`)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\nOffice to depot: %s\n", res.Bindings[0]["d"])

	// An extension function that returns an error produces a SPARQL expression
	// error: the value is unbound and the enclosing FILTER rejects the row.
	// The query itself still succeeds.
	res, err = sparql.Query(g, `
		PREFIX ex:  <http://example.org/>
		PREFIX geo: <http://example.org/geo#>
		SELECT ?place WHERE {
			?place ex:at ?to .
			FILTER(geo:distance("not a point", ?to) < 100)
		}
	`)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\nRows surviving a failing extension call: %d\n", len(res.Bindings))

	fmt.Printf("\nRegistered extension functions: %v\n", sparql.RegisteredFunctions())
}
