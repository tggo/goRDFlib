# goRDFlib

[![CI](https://github.com/tggo/goRDFlib/actions/workflows/ci.yml/badge.svg)](https://github.com/tggo/goRDFlib/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tggo/goRDFlib.svg)](https://pkg.go.dev/github.com/tggo/goRDFlib)
[![Go Report Card](https://goreportcard.com/badge/github.com/tggo/goRDFlib)](https://goreportcard.com/report/github.com/tggo/goRDFlib)
![Coverage](https://img.shields.io/badge/Coverage-93.7%25-brightgreen)
![W3C Tests](https://img.shields.io/badge/W3C_Tests-2439%2F2439-brightgreen)
![W3C SPARQL Query](https://img.shields.io/badge/W3C_SPARQL_1.1_Query-329%2F329-brightgreen)
![W3C SPARQL Update](https://img.shields.io/badge/W3C_SPARQL_1.1_Update-158%2F158-brightgreen)
![W3C SPARQL 1.2](https://img.shields.io/badge/W3C_SPARQL_1.2-235%2F235-brightgreen)
![W3C Turtle](https://img.shields.io/badge/W3C_Turtle-314%2F314-brightgreen)
![W3C Turtle 1.2](https://img.shields.io/badge/W3C_Turtle_1.2-98%2F98-brightgreen)
![W3C N-Triples](https://img.shields.io/badge/W3C_N--Triples-71%2F71-brightgreen)
![W3C N-Triples 1.2](https://img.shields.io/badge/W3C_N--Triples_1.2-30%2F30-brightgreen)
![W3C N-Quads](https://img.shields.io/badge/W3C_N--Quads-88%2F88-brightgreen)
![W3C N-Quads 1.2](https://img.shields.io/badge/W3C_N--Quads_1.2-28%2F28-brightgreen)
![W3C RDF/XML](https://img.shields.io/badge/W3C_RDF%2FXML-167%2F167-brightgreen)
![W3C RDF/XML 1.2](https://img.shields.io/badge/W3C_RDF%2FXML_1.2-31%2F31-brightgreen)
![W3C TriG](https://img.shields.io/badge/W3C_TriG-357%2F357-brightgreen)
![W3C TriG 1.2](https://img.shields.io/badge/W3C_TriG_1.2-61%2F61-brightgreen)
![W3C SHACL](https://img.shields.io/badge/W3C_SHACL-461%2F461-brightgreen)
![SPARQL Protocol](https://img.shields.io/badge/SPARQL_Protocol-99.7%25_coverage-brightgreen)
![Badger Store](https://img.shields.io/badge/Badger_Store-persistent_KV-blue)
![SQLite Store](https://img.shields.io/badge/SQLite_Store-persistent_SQL-blue)

A Go port of the Python [RDFLib](https://github.com/RDFLib/rdflib) library for working with RDF (Resource Description Framework) data.

> **Note:** This project is in active development. The core API is stabilizing but may still change. All W3C conformance suites pass at 100% (2439/2439 tests). Fuzz-tested for robustness.

## About

goRDFlib is a Go implementation of the core RDFLib functionality, ported from the [Python RDFLib](https://github.com/RDFLib/rdflib) library (BSD 3-Clause License). The architecture, algorithms, and test cases are derived from the Python original, adapted to idiomatic Go patterns.

## Features

### RDF Data Model

- **URIRef** -- IRI references with RFC 3987 validation, base URI resolution, fragment extraction
- **BNode** -- Blank nodes with auto-generated UUIDs or explicit IDs
- **Literal** -- Typed literals with language tags, XSD datatype support, and type-aware comparison
  - Native Go type mapping: string, int, int64, float32, float64, bool
  - XSD datatypes: string, integer, int, long, float, double, decimal, boolean, dateTime, date, time, anyURI
  - Directional language tags (`@en--ltr`) per RDF 1.2
- **TripleTerm** -- RDF 1.2 triple terms (triple-as-term) for reification
- **Variable** -- SPARQL query variables
- **Triple / Quad** -- Triple and quad representations with pattern matching

### Graph

- **Graph** -- Single RDF graph with full CRUD operations
- **ConjunctiveGraph** -- Multiple named graphs over a shared store
- **Dataset** -- Named graph management with concurrent access control
- **Resource** -- Node-centric API for convenient traversal (`resource.Objects()`, `resource.Value()`)
- **Collection** -- RDF List (rdf:List) support with cycle detection
- **Set operations** -- Union, intersection, difference between graphs
- **Pattern matching** -- `Triples()`, `Subjects()`, `Predicates()`, `Objects()`, and pair iterators

### In-Memory Store

- Thread-safe triple store with RWMutex
- Three-way indexing (SPO, POS, OSP) for efficient pattern matching
- Namespace binding storage

### Persistent Badger Store

- **BadgerStore** -- `store.Store` implementation backed by [Badger](https://github.com/dgraph-io/badger) LSM-tree KV engine (from the creators of Dgraph)
- Three KV indexes (SPO, POS, OSP) for efficient triple pattern matching via prefix scans
- Named graph support with graph key embedded in every index entry
- ACID transactions, MVCC concurrency (safe for parallel reads/writes)
- Pure Go -- zero CGo dependencies, cross-compile friendly
- Options: `WithDir()`, `WithInMemory()`, `WithReadOnly()`, `WithLogger()`
- Registered as `"badger"` store type via the plugin system

```go
import "github.com/tggo/goRDFlib/store/badgerstore"

// Persistent store
s, _ := badgerstore.New(badgerstore.WithDir("/path/to/data"))
defer s.Close()

// Use with Graph
g := graph.NewGraph(graph.WithStore(s))
g.Add(alice, name, rdf.NewLiteral("Alice"))
// Data survives process restart
```

### Persistent SQLite Store

- **SQLiteStore** -- `store.Store` implementation backed by [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, zero CGo)
- Standard relational schema with three indexes (SPO, POS, OSP)
- WAL mode with busy_timeout for concurrent access
- ACID transactions, human-readable database (inspectable with `sqlite3` CLI)
- Options: `WithFile()`, `WithInMemory()`
- Registered as `"sqlite"` store type via the plugin system

### Remote SPARQL Store

- **SPARQLStore** -- `store.Store` implementation backed by a remote SPARQL endpoint
- Full W3C SPARQL 1.1 Protocol support (query via GET/POST, update via POST)
- Content negotiation: `application/sparql-results+xml`, `application/sparql-results+json`
- Named graph support via `GRAPH` clause wrapping
- Options: `WithUpdate()`, `WithHTTPClient()`, `WithTimeout()`
- Built-in test server (`sparqlstore.Server`) for integration testing
- Registered as `"sparql"` store type via the plugin system

### Serialization Formats

All formats include both parser and serializer:

| Format | Parser | Serializer | W3C Tests |
|--------|:------:|:----------:|-----------|
| Turtle 1.2 | yes | yes | 313/313 + 97/97 RDF 1.2 (100%) |
| TriG 1.2 | yes | yes | 356/356 + 60/60 RDF 1.2 (100%) |
| N-Triples 1.2 | yes | yes | 70/70 + 29/29 RDF 1.2 (100%) |
| N-Quads 1.2 | yes | yes | 87/87 + 28/28 RDF 1.2 (100%) |
| RDF/XML 1.2 | yes | yes | 166/166 + 32/32 RDF 1.2 (100%) |
| JSON-LD | yes | yes | via [piprate/json-gold](https://github.com/piprate/json-gold) |

All parsers support RDF 1.2 features: triple terms (`<<( s p o )>>`), reified triples, annotations (`{| p o |}`), directional language tags, and `rdf:parseType="Triple"` (RDF/XML).

### Namespace System

Built-in vocabularies with pre-defined terms:

| Vocabulary | Prefix | Description |
|------------|--------|-------------|
| RDF | `rdf:` | RDF core vocabulary |
| RDFS | `rdfs:` | RDF Schema |
| OWL | `owl:` | Web Ontology Language (50+ terms) |
| XSD | `xsd:` | XML Schema Datatypes |
| SHACL | `sh:` | Shapes Constraint Language |
| FOAF | `foaf:` | Friend of a Friend |
| DC | `dc:` | Dublin Core Elements 1.1 |
| DCTERMS | `dcterms:` | Dublin Core Terms |
| SKOS | `skos:` | Simple Knowledge Organization System |
| PROV | `prov:` | Provenance Ontology |
| SOSA/SSN | `sosa:`/`ssn:` | Sensor Network Ontologies |
| DCAT | `dcat:` | Data Catalog Vocabulary |
| VOID | `void:` | Vocabulary of Interlinked Datasets |

Custom namespaces with open and closed (restricted) modes.

### SPARQL 1.1 Query Engine

**Query forms:** SELECT, ASK, CONSTRUCT (including CONSTRUCT WHERE shorthand)

**Clauses and patterns:**
- Basic Graph Patterns (BGP)
- FILTER, OPTIONAL, UNION, MINUS
- BIND, VALUES (inline data)
- Sub-SELECT (subqueries)
- ORDER BY (ASC/DESC), DISTINCT, LIMIT/OFFSET
- GROUP BY with HAVING

**Aggregates (7):** COUNT (with DISTINCT), SUM, AVG, MIN, MAX, GROUP_CONCAT (with SEPARATOR), SAMPLE

**Built-in functions (40+):**

| Category | Functions |
|----------|-----------|
| Term tests | BOUND, ISIRI, ISBLANK, ISLITERAL, ISNUMERIC |
| String | STR, STRLEN, SUBSTR, UCASE, LCASE, STRSTARTS, STRENDS, CONTAINS, CONCAT, REGEX, REPLACE, STRBEFORE, STRAFTER, ENCODE_FOR_URI |
| Constructors | STRLANG, STRDT, IRI/URI, BNODE |
| Accessors | LANG, DATATYPE |
| Numeric | ABS, ROUND, CEIL, FLOOR |
| Hash | MD5, SHA1, SHA256, SHA384, SHA512 |
| Date/Time | NOW, YEAR, MONTH, DAY, HOURS, MINUTES, SECONDS, TIMEZONE, TZ |
| Conditional | IF, COALESCE |
| Comparison | LANGMATCHES, SAMETERM |
| Random/UUID | RAND, UUID, STRUUID |
| XSD casts | xsd:boolean, xsd:integer, xsd:float, xsd:double, xsd:decimal, xsd:string |

### SPARQL 1.2 Extensions

Full SPARQL 1.2 support -- **234/234 W3C tests pass (100%)**:

- **Triple term patterns** -- `<<( ?s ?p ?o )>>` in graph patterns
- **Reified triples** -- `<< ?s ?p ?o >>` with `rdf:reifies` desugaring
- **Annotation syntax** -- `?s ?p ?o {| :source :web |}`
- **VERSION directive** -- `VERSION "1.2"`
- **Codepoint escapes** -- `\uHHHH` / `\UHHHHHHHH` in prefixed names
- **Directional language tags** -- `"text"@en--ltr`
- **New functions** -- `TRIPLE()`, `SUBJECT()`, `PREDICATE()`, `OBJECT()`, `isTriple()`, `LANGDIR()`, `hasLANG()`, `hasLANGDIR()`, `STRLANGDIR()`

### SPARQL 1.1 Update Engine

**Data operations:**
- INSERT DATA, DELETE DATA
- DELETE/INSERT WHERE (with template instantiation)
- DELETE WHERE (shorthand)

**Modify operations:**
- WITH clause (scoped graph target)
- USING / USING NAMED (query dataset override)
- Multiple operations separated by `;`

**Graph management:**
- CLEAR, DROP (DEFAULT / NAMED / ALL / specific graph)
- CREATE GRAPH
- ADD, MOVE, COPY (between graphs)
- LOAD (with `rdfloader.DefaultLoader` for http(s):// and file:// URIs, auto-format detection)

**Dataset:** `sparql.Dataset` holds default graph + named graphs map for multi-graph update operations.

### Property Paths

Full SPARQL 1.1 property path support:

- **Inverse** -- `^p`
- **Sequence** -- `p1/p2`
- **Alternative** -- `p1|p2`
- **Repetition** -- `p*` (zero or more), `p+` (one or more), `p?` (zero or one)
- **Negated** -- `!p` (negated property set)
- Cycle detection for transitive closure operations

### SHACL Validator

Full W3C SHACL Core validation engine -- **98/98 W3C tests pass (100%)**.

**Targets:**
- `sh:targetNode`, `sh:targetClass` (with `rdfs:subClassOf` inference)
- `sh:targetSubjectsOf`, `sh:targetObjectsOf`
- Implicit class targets (NodeShape that is also `rdfs:Class`)

**Constraint components (28):**

| Category | Constraints |
|----------|-------------|
| Cardinality | sh:minCount, sh:maxCount |
| Value type | sh:class, sh:datatype, sh:nodeKind |
| Value range | sh:minExclusive, sh:minInclusive, sh:maxExclusive, sh:maxInclusive |
| String | sh:minLength, sh:maxLength, sh:pattern (with flags), sh:languageIn, sh:uniqueLang |
| Property pair | sh:equals, sh:disjoint, sh:lessThan, sh:lessThanOrEquals |
| Logical | sh:and, sh:or, sh:not, sh:xone |
| Shape-based | sh:node, sh:property, sh:qualifiedValueShape (with min/max count) |
| Other | sh:hasValue, sh:in, sh:closed |

**Additional features:**
- Property shapes with `sh:path` (including property paths)
- Node shapes
- Severity levels: `sh:Violation`, `sh:Warning`, `sh:Info`
- Custom messages via `sh:message`
- Shape deactivation via `sh:deactivated`
- Recursive shape validation

### Plugin System

- Format auto-detection by filename extension (`.ttl`, `.trig`, `.nt`, `.nq`, `.rdf`, `.owl`, `.jsonld`)
- MIME type detection (`text/turtle`, `application/rdf+xml`, etc.)
- Content sniffing (XML headers, JSON, `@prefix`, etc.)
- Extensible parser, serializer, and store registries

## Installation

```bash
go get github.com/tggo/goRDFlib
```

## Quick Start

```go
package main

import (
    "fmt"
    "os"
    "strings"

    rdf "github.com/tggo/goRDFlib"
    "github.com/tggo/goRDFlib/sparql"
    "github.com/tggo/goRDFlib/turtle"
)

func main() {
    g := rdf.NewGraph()
    g.Bind("ex", rdf.NewURIRefUnsafe("http://example.org/"))

    alice, _ := rdf.NewURIRef("http://example.org/Alice")
    name, _ := rdf.NewURIRef("http://example.org/name")
    age, _ := rdf.NewURIRef("http://example.org/age")

    g.Add(alice, rdf.RDF.Type, rdf.NewURIRefUnsafe("http://example.org/Person"))
    g.Add(alice, name, rdf.NewLiteral("Alice"))
    g.Add(alice, age, rdf.NewLiteral(30))

    // Serialize to Turtle
    turtle.Serialize(g, os.Stdout)

    // Parse Turtle
    g2 := rdf.NewGraph()
    turtle.Parse(g2, strings.NewReader(`
        @prefix ex: <http://example.org/> .
        ex:Bob a ex:Person ; ex:name "Bob" .
    `))

    // SPARQL query
    result, _ := sparql.Query(g, `
        PREFIX ex: <http://example.org/>
        SELECT ?name WHERE { ?s ex:name ?name }
    `)
    for _, row := range result.Bindings {
        fmt.Println(row["name"])
    }
}
```

### SPARQL Update Example

```go
ds := &sparql.Dataset{Default: g}

// DELETE/INSERT WHERE: bulk status change with pattern matching
err := sparql.Update(ds, `
    PREFIX ex: <http://example.org/>
    DELETE { ?s ex:status "draft" }
    INSERT { ?s ex:status "published" }
    WHERE  { ?s ex:status "draft" }
`)

// Graph management: copy default graph to a named graph
err = sparql.Update(ds, `COPY DEFAULT TO <http://example.org/archive>`)
```

### SHACL Validation Example

```go
package main

import (
    "fmt"
    "log"

    "github.com/tggo/goRDFlib/shacl"
)

func main() {
    data, err := shacl.LoadTurtleString(`
        @prefix ex: <http://example.org/> .
        @prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
        ex:Alice a ex:Person ; ex:name "Alice" ; ex:age 30 .
        ex:Bob   a ex:Person ; ex:age "not a number" .
    `, "")
    if err != nil {
        log.Fatal(err)
    }

    shapes, err := shacl.LoadTurtleString(`
        @prefix sh: <http://www.w3.org/ns/shacl#> .
        @prefix ex: <http://example.org/> .
        @prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

        ex:PersonShape a sh:NodeShape ;
            sh:targetClass ex:Person ;
            sh:property [
                sh:path ex:name ;
                sh:minCount 1 ;
                sh:datatype xsd:string ;
            ] ;
            sh:property [
                sh:path ex:age ;
                sh:datatype xsd:integer ;
            ] .
    `, "")
    if err != nil {
        log.Fatal(err)
    }

    report := shacl.Validate(data, shapes)
    fmt.Printf("Conforms: %v\n", report.Conforms)
    for _, r := range report.Results {
        fmt.Printf("  Focus: %s, Path: %s, Constraint: %s\n",
            r.FocusNode, r.ResultPath, r.SourceConstraintComponent)
    }
}
```

## W3C Conformance

All parsers, SPARQL engine, and SHACL validator are validated against official W3C test suites:

| Component | Tests | Pass | Status |
|-----------|-------|------|--------|
| SPARQL 1.1 Query | 329 | 329 | 100% |
| SPARQL 1.1 Update | 158 | 158 | 100% |
| SPARQL 1.2 | 234 | 234 | 100% |
| Turtle 1.1 | 313 | 313 | 100% |
| Turtle 1.2 | 97 | 97 | 100% |
| N-Triples 1.2 | 29 | 29 | 100% |
| N-Quads 1.2 | 28 | 28 | 100% |
| N-Triples | 70 | 70 | 100% |
| N-Quads | 87 | 87 | 100% |
| RDF/XML | 166 | 166 | 100% |
| RDF/XML 1.2 | 32 | 32 | 100% |
| TriG 1.1 | 356 | 356 | 100% |
| TriG 1.2 | 60 | 60 | 100% |
| SHACL Core | 98 | 98 | 100% |
| **Total** | **1644** | **1644** | **100%** |

```bash
make test          # all tests
make test-sparql   # W3C SPARQL 1.1 conformance
```

### Fuzz Testing

The SPARQL parser is fuzz-tested with `go test -fuzz` to detect panics and infinite loops on malformed input. Seed corpus includes all W3C SPARQL 1.1 and 1.2 test files.

## Performance

Benchmarked against Python rdflib 7.6.0 + pyshacl 0.31.0 on Apple M4 Max:

| Benchmark | Go | Python | Speedup |
|-----------|---:|-------:|--------:|
| NewURIRef | 40 ns | 332 ns | **8x** |
| NewBNode | 251 ns | 2,655 ns | **11x** |
| NewLiteral (string) | 16 ns | 1,310 ns | **82x** |
| NewLiteral (int) | 16 ns | 1,976 ns | **124x** |
| URIRef.N3() | 18 ns | 280 ns | **16x** |
| Literal.N3() | 30 ns | 393 ns | **13x** |
| Literal.Eq() | 20 ns | 339 ns | **17x** |
| Store Add 10k | 12.4 ms | 89.4 ms | **7x** |
| Store Lookup 1k | 7 us | 911 us | **130x** |
| Parse Turtle | 5.4 us | 265 us | **49x** |
| Serialize Turtle | 5.7 us | 96 us | **17x** |
| SPARQL SELECT | 56 us | 1,820 us | **33x** |
| SHACL Validate (10 nodes) | 21 us | 1,168 us | **56x** |
| SHACL Validate (100 nodes) | 105 us | 8,021 us | **76x** |
| SHACL Validate (complex) | 70 us | 8,111 us | **116x** |

```bash
go test ./benchmarks/ -bench=. -benchmem
python3 benchmarks/bench_python.py
```

### Store Stress Test (3M triples, Apple M4 Max)

| Metric | Memory | Badger | Badger Disk | SQLite | SQLite Disk |
|--------|-------:|-------:|------------:|-------:|------------:|
| **Ingest time** | 5.5s | 7.7s | 9.0s | 21.5s | 28.8s |
| **Ingest rate** | 544K/s | 389K/s | 335K/s | 139K/s | 104K/s |
| **RAM delta** | 8.1 GB | 489 MB | — | ~0 MB | — |
| **Len()** | 4ms | 22ms | — | 229ms | — |
| **Full scan** | 835ms | 1.75s | — | 1.6s | — |
| **Subject lookup** | 144 ns | 3.5 us | 3.1 us | 5.5 us | 7.4 us |
| **Predicate scan** | 2.6ms | 8.4ms | 8.8ms | 345ms | 1.2s |

```bash
go test ./store/ -run TestStress -v -count=1
```

## Examples

The [examples/](examples/) directory contains runnable programs:

| Example | Description |
|---------|-------------|
| `simple_example` | Basic graph operations and triple manipulation |
| `format_examples` | Parsing and serializing all supported formats |
| `sparql_query_example` | SPARQL SELECT, ASK, and CONSTRUCT queries |
| `property_paths_example` | Property path traversal |
| `resource_example` | Node-centric Resource API |
| `shacl_example` | Basic SHACL validation |
| `shacl_constraints_example` | SHACL constraint types |
| `transitive_example` | Transitive closure with property paths |

## Package Structure

```
goRDFlib/
  term/         Core RDF terms (URIRef, BNode, Literal, Variable)
  store/        Store interface + thread-safe in-memory triple store
  graph/        Graph, ConjunctiveGraph, Dataset, Resource, Collection
  namespace/    Built-in vocabularies and namespace management
  turtle/       Turtle parser and serializer
  nt/           N-Triples parser and serializer
  nq/           N-Quads parser and serializer
  rdfxml/       RDF/XML parser and serializer
  jsonld/       JSON-LD parser and serializer
  trig/         TriG parser and serializer
  sparql/       SPARQL 1.1/1.2 query and update engine
  store/badgerstore/  Persistent Badger KV store (SPO/POS/OSP indexes)
  store/sqlitestore/  Persistent SQLite store (pure Go)
  store/sparqlstore/  Remote SPARQL Protocol store + test server
  paths/        Property path evaluation
  shacl/        SHACL Core validator
  rdfloader/    HTTP/file URI loader for SPARQL LOAD
  plugin/       Format registry and auto-detection
  examples/     Runnable example programs
  benchmarks/   Performance benchmarks
```

## Based On

This project is a Go port of [RDFLib](https://github.com/RDFLib/rdflib) (v7.x), a Python library for working with RDF. The original RDFLib is maintained by the RDFLib Team and licensed under the BSD 3-Clause License.

## Known Limitations

- JSON-LD processing delegates to [piprate/json-gold](https://github.com/piprate/json-gold) which may attempt remote context fetches
- SPARQL UPDATE `LOAD` requires the `SILENT` flag when no loader is configured (default: `rdfloader.DefaultLoader` handles `http://`, `https://`, and `file://` URIs with auto-format detection)

## License

BSD 3-Clause License. See [LICENSE](LICENSE).

The original Python RDFLib is Copyright (c) 2002-2025, RDFLib Team.
