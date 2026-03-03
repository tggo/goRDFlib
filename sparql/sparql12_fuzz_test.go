package sparql_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tggo/goRDFlib/sparql"
)

func FuzzParse(f *testing.F) {
	// Seed corpus from W3C SPARQL 1.2 test queries.
	patterns := []string{
		"../testdata/w3c/rdf-tests/sparql/sparql12/*/*.rq",
		"../testdata/w3c/rdf-tests/sparql/sparql11/*/*.rq",
	}
	for _, pat := range patterns {
		files, _ := filepath.Glob(pat)
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err == nil && len(data) > 0 {
				f.Add(string(data))
			}
		}
	}
	f.Fuzz(func(t *testing.T, input string) {
		sparql.Parse(input) // must not panic
	})
}

func FuzzParseUpdate(f *testing.F) {
	// Seed corpus from W3C SPARQL 1.2 update files.
	patterns := []string{
		"../testdata/w3c/rdf-tests/sparql/sparql12/*/*.ru",
		"../testdata/w3c/rdf-tests/sparql/sparql11/*/*.ru",
	}
	for _, pat := range patterns {
		files, _ := filepath.Glob(pat)
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err == nil && len(data) > 0 {
				f.Add(string(data))
			}
		}
	}
	f.Fuzz(func(t *testing.T, input string) {
		sparql.ParseUpdate(input) // must not panic
	})
}
