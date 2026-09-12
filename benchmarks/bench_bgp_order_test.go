package benchmarks_test

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/sparql"
)

// BGP join-order benchmarks.
//
// Every query here is written so that the order the author typed is either the
// good one (WrittenWell) or a bad one (the rest). A planner that orders
// triple patterns by selectivity should make the bad spellings cost about the
// same as the good one, and must not slow the good one down. PerRowOptional
// exercises the per-left-row BGP evaluation behind OPTIONAL, where any
// ordering overhead is paid once per row rather than once per query.
var bgpOrderQueries = []struct {
	name, query string
}{
	{"ActorsByDirector/WrittenWell", `SELECT DISTINCT ?actor ?actorName WHERE {
		?film <http://freebase.com/film.film.directed_by> <http://freebase.com/director/0> .
		?film <http://freebase.com/film.film.starring> ?perf .
		?perf <http://freebase.com/film.performance.actor> ?actor .
		?actor <http://www.w3.org/2000/01/rdf-schema#label> ?actorName .
	}`},
	{"ActorsByDirector/Reversed", `SELECT DISTINCT ?actor ?actorName WHERE {
		?actor <http://www.w3.org/2000/01/rdf-schema#label> ?actorName .
		?perf <http://freebase.com/film.performance.actor> ?actor .
		?film <http://freebase.com/film.film.starring> ?perf .
		?film <http://freebase.com/film.film.directed_by> <http://freebase.com/director/0> .
	}`},
	{"FilmStar/TypeFirst", `SELECT ?film ?title WHERE {
		?film a <http://freebase.com/film.film> .
		?film <http://www.w3.org/2000/01/rdf-schema#label> ?title .
		?film <http://freebase.com/film.film.genre> <http://freebase.com/genre/3> .
		?film <http://freebase.com/film.film.directed_by> <http://freebase.com/director/0> .
	}`},
	{"PerRowOptional", `SELECT ?film ?actor WHERE {
		?film a <http://freebase.com/film.film> .
		OPTIONAL {
			?perf <http://freebase.com/film.performance.actor> ?actor .
			?film <http://freebase.com/film.film.starring> ?perf .
		}
	}`},
}

func BenchmarkBGPOrder(b *testing.B) {
	for _, size := range []int{10_000, 100_000} {
		g := generateFilmDataset(size)
		for _, q := range bgpOrderQueries {
			b.Run(fmt.Sprintf("%s/%dk", q.name, size/1000), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := sparql.Query(g, q.query); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
