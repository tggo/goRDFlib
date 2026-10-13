package benchmarks_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// Film-like dataset generator inspired by Freebase/dgraph-benchmarks schema.
// Generates directors, films, actors, genres, countries with realistic relationships.

var (
	fbFilm, _     = term.NewURIRef("http://freebase.com/film.film")
	fbDirector, _ = term.NewURIRef("http://freebase.com/film.director")
	fbActor, _    = term.NewURIRef("http://freebase.com/film.actor")
	fbGenre, _    = term.NewURIRef("http://freebase.com/film.genre")
	fbCountry, _  = term.NewURIRef("http://freebase.com/film.country")

	fbDirectedBy, _    = term.NewURIRef("http://freebase.com/film.film.directed_by")
	fbStarring, _      = term.NewURIRef("http://freebase.com/film.film.starring")
	fbPerformance, _   = term.NewURIRef("http://freebase.com/film.performance")
	fbPerfActor, _     = term.NewURIRef("http://freebase.com/film.performance.actor")
	fbPerfCharacter, _ = term.NewURIRef("http://freebase.com/film.performance.character")
	fbPerfFilm, _      = term.NewURIRef("http://freebase.com/film.performance.film")
	fbHasGenre, _      = term.NewURIRef("http://freebase.com/film.film.genre")
	fbInCountry, _     = term.NewURIRef("http://freebase.com/film.film.country")
	fbReleaseDate, _   = term.NewURIRef("http://freebase.com/film.film.initial_release_date")
	fbRating, _        = term.NewURIRef("http://freebase.com/film.film.rating")

	rdfType, _  = term.NewURIRef("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")
	rdfsLabel   = term.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#label")
	xsdDate, _  = term.NewURIRef("http://www.w3.org/2001/XMLSchema#date")
	xsdFloat, _ = term.NewURIRef("http://www.w3.org/2001/XMLSchema#float")
)

var genreNames = []string{
	"Action", "Comedy", "Drama", "Horror", "Sci-Fi", "Thriller",
	"Romance", "Documentary", "Animation", "Fantasy", "Mystery",
	"Adventure", "Crime", "Musical", "Western", "War", "Biography",
	"Family", "Sport", "History",
}

var countryNames = []string{
	"United States", "United Kingdom", "France", "Germany", "Japan",
	"India", "South Korea", "Italy", "Spain", "Canada", "Australia",
	"Brazil", "Mexico", "China", "Russia", "Sweden", "Denmark",
	"Norway", "Argentina", "New Zealand",
}

// generateFilmDataset creates a film-like RDF graph with approximate triple count.
// The dataset mirrors Freebase film structure used by dgraph-benchmarks.
func generateFilmDataset(targetTriples int) *graph.Graph {
	g := graph.NewGraph()
	rng := rand.New(rand.NewPCG(42, 0))

	// Ratios derived from dgraph 1M dataset structure:
	// ~10% directors, ~30% films, ~40% actors, ~20% genres/countries/metadata
	numDirectors := targetTriples / 50
	numFilms := targetTriples / 15
	numActors := targetTriples / 12

	if numDirectors < 10 {
		numDirectors = 10
	}
	if numFilms < 20 {
		numFilms = 20
	}
	if numActors < 30 {
		numActors = 30
	}

	// Create genres
	genres := make([]term.URIRef, len(genreNames))
	for i, name := range genreNames {
		u := term.NewURIRefUnsafe(fmt.Sprintf("http://freebase.com/genre/%d", i))
		genres[i] = u
		mustAdd(g, u, rdfType, fbGenre)
		mustAdd(g, u, rdfsLabel, term.NewLiteral(name, term.WithLang("en")))
	}

	// Create countries
	countries := make([]term.URIRef, len(countryNames))
	for i, name := range countryNames {
		u := term.NewURIRefUnsafe(fmt.Sprintf("http://freebase.com/country/%d", i))
		countries[i] = u
		mustAdd(g, u, rdfType, fbCountry)
		mustAdd(g, u, rdfsLabel, term.NewLiteral(name, term.WithLang("en")))
	}

	// Create directors
	directors := make([]term.URIRef, numDirectors)
	for i := range numDirectors {
		u := term.NewURIRefUnsafe(fmt.Sprintf("http://freebase.com/director/%d", i))
		directors[i] = u
		mustAdd(g, u, rdfType, fbDirector)
		mustAdd(g, u, rdfsLabel, term.NewLiteral(fmt.Sprintf("Director %d", i), term.WithLang("en")))
	}

	// Create actors
	actors := make([]term.URIRef, numActors)
	for i := range numActors {
		u := term.NewURIRefUnsafe(fmt.Sprintf("http://freebase.com/actor/%d", i))
		actors[i] = u
		mustAdd(g, u, rdfType, fbActor)
		mustAdd(g, u, rdfsLabel, term.NewLiteral(fmt.Sprintf("Actor %d", i), term.WithLang("en")))
	}

	// Create films with relationships
	perfID := 0
	for i := range numFilms {
		film := term.NewURIRefUnsafe(fmt.Sprintf("http://freebase.com/film/%d", i))
		mustAdd(g, film, rdfType, fbFilm)
		mustAdd(g, film, rdfsLabel, term.NewLiteral(fmt.Sprintf("Film %d", i), term.WithLang("en")))

		// Director
		mustAdd(g, film, fbDirectedBy, directors[rng.IntN(numDirectors)])

		// Release date
		year := 1950 + rng.IntN(75)
		month := 1 + rng.IntN(12)
		day := 1 + rng.IntN(28)
		mustAdd(g, film, fbReleaseDate, term.NewLiteral(
			fmt.Sprintf("%04d-%02d-%02d", year, month, day),
			term.WithDatatype(xsdDate),
		))

		// Rating
		rating := 1.0 + rng.Float64()*9.0
		mustAdd(g, film, fbRating, term.NewLiteral(
			fmt.Sprintf("%.1f", rating),
			term.WithDatatype(xsdFloat),
		))

		// 1-3 genres
		numGenres := 1 + rng.IntN(3)
		used := make(map[int]bool, numGenres)
		for range numGenres {
			idx := rng.IntN(len(genres))
			if !used[idx] {
				mustAdd(g, film, fbHasGenre, genres[idx])
				used[idx] = true
			}
		}

		// 1-2 countries
		numCountries := 1 + rng.IntN(2)
		for j := range numCountries {
			mustAdd(g, film, fbInCountry, countries[(i+j)%len(countries)])
		}

		// 2-5 actors (performances)
		numCast := 2 + rng.IntN(4)
		for j := range numCast {
			perf := term.NewURIRefUnsafe(fmt.Sprintf("http://freebase.com/performance/%d", perfID))
			perfID++
			mustAdd(g, film, fbStarring, perf)
			mustAdd(g, perf, rdfType, fbPerformance)
			mustAdd(g, perf, fbPerfActor, actors[rng.IntN(numActors)])
			mustAdd(g, perf, fbPerfFilm, film)
			mustAdd(g, perf, fbPerfCharacter, term.NewLiteral(fmt.Sprintf("Character %d", j), term.WithLang("en")))
		}
	}

	return g
}

// generateFilmTurtle returns the dataset as Turtle string (for parser benchmarks).
func generateFilmTurtle(targetTriples int) string {
	g := generateFilmDataset(targetTriples)
	var sb strings.Builder
	sb.WriteString("@prefix fb: <http://freebase.com/> .\n")
	sb.WriteString("@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n")
	sb.WriteString("@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .\n")
	sb.WriteString("@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n\n")

	for triple, err := range g.Triples(context.Background(), nil, nil, nil) {
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(&sb, "%s %s %s .\n", triple.Subject.N3(), triple.Predicate.N3(), triple.Object.N3())
	}
	return sb.String()
}

// generateFilmNTriples returns N-Triples format string.
func generateFilmNTriples(targetTriples int) string {
	g := generateFilmDataset(targetTriples)
	var sb strings.Builder
	for triple, err := range g.Triples(context.Background(), nil, nil, nil) {
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(&sb, "%s %s %s .\n", triple.Subject.N3(), triple.Predicate.N3(), triple.Object.N3())
	}
	return sb.String()
}

// mustAdd adds a triple to a MemoryStore-backed graph, which cannot fail
// unless the context is done; the generators run before any timing starts.
func mustAdd(g *graph.Graph, s term.Subject, p term.URIRef, o term.Term) {
	if err := g.Add(context.Background(), s, p, o); err != nil {
		panic(err)
	}
}
