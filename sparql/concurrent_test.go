package sparql_test

import (
	"fmt"
	"sync"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

func buildTestGraph(n int) *rdflibgo.Graph {
	g := rdflibgo.NewGraph()
	for i := 0; i < n; i++ {
		s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		p := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/p%d", i%10))
		o := term.NewLiteral(fmt.Sprintf("value_%d", i))
		g.Add(s, p, o)
	}
	return g
}

// queriesForConcurrent returns a slice of 20 different SPARQL query strings
// exercising SELECT, ASK, CONSTRUCT, GROUP BY, ORDER BY, FILTER, OPTIONAL,
// UNION, and LIMIT.
func queriesForConcurrent() []string {
	return []string{
		// 0: basic SELECT
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p0 ?o }`,
		// 1: SELECT with LIMIT
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ?p ?o } LIMIT 10`,
		// 2: SELECT with FILTER
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p1 ?o . FILTER(str(?o) > "value_50") }`,
		// 3: SELECT with ORDER BY
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p2 ?o } ORDER BY ?o LIMIT 20`,
		// 4: SELECT DISTINCT
		`PREFIX ex: <http://example.org/>
SELECT DISTINCT ?p WHERE { ?s ?p ?o }`,
		// 5: ASK true
		`PREFIX ex: <http://example.org/>
ASK { ?s ex:p0 ?o }`,
		// 6: ASK false
		`PREFIX ex: <http://example.org/>
ASK { ex:nonexistent ex:p0 ?o }`,
		// 7: CONSTRUCT
		`PREFIX ex: <http://example.org/>
CONSTRUCT { ?s ex:has ?o } WHERE { ?s ex:p3 ?o } LIMIT 5`,
		// 8: OPTIONAL
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o1 ?o2 WHERE {
  ?s ex:p0 ?o1 .
  OPTIONAL { ?s ex:p1 ?o2 }
} LIMIT 10`,
		// 9: UNION
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE {
  { ?s ex:p0 ?o } UNION { ?s ex:p1 ?o }
} LIMIT 15`,
		// 10: GROUP BY with COUNT
		`PREFIX ex: <http://example.org/>
SELECT ?p (COUNT(?s) AS ?cnt) WHERE {
  ?s ?p ?o
} GROUP BY ?p`,
		// 11: GROUP BY with ORDER BY
		`PREFIX ex: <http://example.org/>
SELECT ?p (COUNT(?s) AS ?cnt) WHERE {
  ?s ?p ?o
} GROUP BY ?p ORDER BY DESC(?cnt)`,
		// 12: SELECT with OFFSET and LIMIT
		`PREFIX ex: <http://example.org/>
SELECT ?s WHERE { ?s ?p ?o } ORDER BY ?s LIMIT 10 OFFSET 5`,
		// 13: FILTER with regex
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p4 ?o . FILTER(regex(str(?o), "^value_[0-9]+$")) } LIMIT 10`,
		// 14: SELECT with bound check
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE {
  ?s ex:p5 ?o .
  FILTER(bound(?o))
} LIMIT 10`,
		// 15: CONSTRUCT with UNION
		`PREFIX ex: <http://example.org/>
CONSTRUCT { ?s ex:relatedTo ?o } WHERE {
  { ?s ex:p6 ?o } UNION { ?s ex:p7 ?o }
} LIMIT 5`,
		// 16: SELECT with VALUES
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE {
  VALUES ?s { ex:s0 ex:s1 ex:s2 }
  ?s ?p ?o
}`,
		// 17: SELECT with FILTER NOT EXISTS
		`PREFIX ex: <http://example.org/>
SELECT ?s WHERE {
  ?s ex:p0 ?o .
  FILTER NOT EXISTS { ?s ex:p1 ?o2 }
} LIMIT 10`,
		// 18: SELECT with BIND
		`PREFIX ex: <http://example.org/>
SELECT ?s ?label WHERE {
  ?s ex:p8 ?o .
  BIND(str(?o) AS ?label)
} LIMIT 10`,
		// 19: SELECT with string function
		`PREFIX ex: <http://example.org/>
SELECT ?s (strlen(str(?o)) AS ?len) WHERE {
  ?s ex:p9 ?o
} ORDER BY ?len LIMIT 10`,
	}
}

// TestConcurrentSPARQLQueries builds a graph with 1000 triples, then launches
// 50 goroutines each running 20 different SPARQL queries. Verifies no panics
// and that results are valid (non-nil, correct type). Run with -race.
func TestConcurrentSPARQLQueries(t *testing.T) {
	g := buildTestGraph(1000)
	queries := queriesForConcurrent()

	const goroutines = 50
	const runsEach = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for run := 0; run < runsEach; run++ {
				for qi, q := range queries {
					res, err := sparql.Query(g, q)
					if err != nil {
						// Use t.Errorf (not t.Fatal) to avoid calling Fatal from goroutine.
						t.Errorf("goroutine query[%d] error: %v", qi, err)
						continue
					}
					if res == nil {
						t.Errorf("goroutine query[%d]: got nil result", qi)
						continue
					}
					switch res.Type {
					case "SELECT":
						if res.Bindings == nil {
							t.Errorf("goroutine query[%d] SELECT: nil Bindings", qi)
						}
					case "ASK":
						// AskResult is a bool; no nil check needed
					case "CONSTRUCT":
						if res.Graph == nil {
							t.Errorf("goroutine query[%d] CONSTRUCT: nil Graph", qi)
						}
					default:
						t.Errorf("goroutine query[%d]: unexpected result type %q", qi, res.Type)
					}
				}
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentReadWrite starts with 100 triples, launches 5 writer goroutines
// each adding 100 triples, and 20 reader goroutines each running 50 SPARQL
// queries. Verifies no panics or data corruption.
func TestConcurrentReadWrite(t *testing.T) {
	g := buildTestGraph(100)

	const writers = 5
	const writesEach = 100
	const readers = 20
	const readsEach = 50

	readQuery := `PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ?p ?o } LIMIT 10`

	var wg sync.WaitGroup
	wg.Add(writers + readers)

	// Writer goroutines: add triples
	for w := 0; w < writers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < writesEach; i++ {
				// Use a high base to avoid collision with initial triples
				base := 10000 + w*writesEach + i
				s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/ws%d", base))
				p := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/p%d", base%10))
				o := term.NewLiteral(fmt.Sprintf("write_value_%d", base))
				g.Add(s, p, o)
			}
		}()
	}

	// Reader goroutines: run SPARQL queries
	for r := 0; r < readers; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < readsEach; i++ {
				res, err := sparql.Query(g, readQuery)
				if err != nil {
					t.Errorf("reader query error: %v", err)
					continue
				}
				if res == nil {
					t.Errorf("reader: got nil result")
					continue
				}
				if res.Type != "SELECT" {
					t.Errorf("reader: expected SELECT, got %q", res.Type)
					continue
				}
				if res.Bindings == nil {
					t.Errorf("reader: nil Bindings")
				}
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentQueryCache enables the query cache, runs 50 goroutines each
// executing the same 10 queries 20 times, and verifies all cached results are
// identical to sequential execution. Disables cache after test.
func TestConcurrentQueryCache(t *testing.T) {
	sparql.EnableQueryCache(256)
	defer sparql.DisableQueryCache()

	g := buildTestGraph(200)

	cacheQueries := []string{
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p0 ?o }`,
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p1 ?o }`,
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p2 ?o }`,
		`PREFIX ex: <http://example.org/>
ASK { ?s ex:p3 ?o }`,
		`PREFIX ex: <http://example.org/>
ASK { ?s ex:p4 ?o }`,
		`PREFIX ex: <http://example.org/>
SELECT (COUNT(?s) AS ?cnt) WHERE { ?s ?p ?o }`,
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p5 ?o } ORDER BY ?o`,
		`PREFIX ex: <http://example.org/>
CONSTRUCT { ?s ex:has ?o } WHERE { ?s ex:p6 ?o }`,
		`PREFIX ex: <http://example.org/>
SELECT DISTINCT ?p WHERE { ?s ?p ?o }`,
		`PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p7 ?o } LIMIT 5`,
	}

	// Run sequentially first to get reference results.
	type refResult struct {
		resultType string
		bindingLen int
		askResult  bool
		graphSize  int
	}
	ref := make([]refResult, len(cacheQueries))
	for i, q := range cacheQueries {
		res, err := sparql.Query(g, q)
		if err != nil {
			t.Fatalf("sequential query[%d] error: %v", i, err)
		}
		rr := refResult{resultType: res.Type, askResult: res.AskResult}
		switch res.Type {
		case "SELECT":
			rr.bindingLen = len(res.Bindings)
		case "CONSTRUCT":
			rr.graphSize = res.Graph.Len()
		}
		ref[i] = rr
	}

	const goroutines = 50
	const runsEach = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for run := 0; run < runsEach; run++ {
				for qi, q := range cacheQueries {
					res, err := sparql.Query(g, q)
					if err != nil {
						t.Errorf("cached goroutine query[%d] run %d error: %v", qi, run, err)
						continue
					}
					if res == nil {
						t.Errorf("cached goroutine query[%d] run %d: nil result", qi, run)
						continue
					}
					rr := ref[qi]
					if res.Type != rr.resultType {
						t.Errorf("cached goroutine query[%d]: type mismatch: got %q, want %q",
							qi, res.Type, rr.resultType)
						continue
					}
					switch res.Type {
					case "SELECT":
						if len(res.Bindings) != rr.bindingLen {
							t.Errorf("cached goroutine query[%d]: binding count mismatch: got %d, want %d",
								qi, len(res.Bindings), rr.bindingLen)
						}
					case "ASK":
						if res.AskResult != rr.askResult {
							t.Errorf("cached goroutine query[%d]: ASK mismatch: got %v, want %v",
								qi, res.AskResult, rr.askResult)
						}
					case "CONSTRUCT":
						if res.Graph == nil {
							t.Errorf("cached goroutine query[%d]: nil CONSTRUCT graph", qi)
						} else if res.Graph.Len() != rr.graphSize {
							t.Errorf("cached goroutine query[%d]: CONSTRUCT graph size mismatch: got %d, want %d",
								qi, res.Graph.Len(), rr.graphSize)
						}
					}
				}
			}
		}()
	}

	wg.Wait()
}
