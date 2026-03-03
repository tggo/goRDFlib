# goRDFlib

[![CI](https://github.com/tggo/goRDFlib/actions/workflows/ci.yml/badge.svg)](https://github.com/tggo/goRDFlib/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tggo/goRDFlib.svg)](https://pkg.go.dev/github.com/tggo/goRDFlib)
[![Go Report Card](https://goreportcard.com/badge/github.com/tggo/goRDFlib)](https://goreportcard.com/report/github.com/tggo/goRDFlib)
![W3C SPARQL](https://img.shields.io/badge/W3C_SPARQL_1.1-328%2F328-brightgreen)
![W3C Turtle](https://img.shields.io/badge/W3C_Turtle-313%2F313-brightgreen)
![W3C N-Triples](https://img.shields.io/badge/W3C_N--Triples-70%2F70-brightgreen)
![W3C N-Quads](https://img.shields.io/badge/W3C_N--Quads-87%2F87-brightgreen)
![W3C RDF/XML](https://img.shields.io/badge/W3C_RDF%2FXML-166%2F166-brightgreen)
![W3C SHACL](https://img.shields.io/badge/W3C_SHACL-98%2F98-brightgreen)

A Go port of the Python [RDFLib](https://github.com/RDFLib/rdflib) library for working with RDF (Resource Description Framework) data.

> **Warning:** This project is in early development (v0.0.x). The API is not stable and may contain bugs. Do not use in production without thorough testing.

## About

goRDFlib is a Go implementation of the core RDFLib functionality, ported from the [Python RDFLib](https://github.com/RDFLib/rdflib) library (BSD 3-Clause License). The architecture, algorithms, and test cases are derived from the Python original, adapted to idiomatic Go patterns.

## Features

### RDF Data Model

- **URIRef** -- IRI references with RFC 3987 validation, base URI resolution, fragment extraction
- **BNode** -- Blank nodes with auto-generated UUIDs or explicit IDs
- **Literal** -- Typed literals with language tags, XSD datatype support, and type-aware comparison
  - Native Go type mapping: string, int, int64, float32, float64, bool
  - XSD datatypes: string, integer, int, long, float, double, decimal, boolean, dateTime, date, time, anyURI
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

### Serialization Formats

All formats include both parser and serializer:

| Format | Parser | Serializer | W3C Tests |
|--------|:------:|:----------:|-----------|
| Turtle | yes | yes | 313/313 (100%) |
| N-Triples | yes | yes | 70/70 (100%) |
| N-Quads | yes | yes | 87/87 (100%) |
| RDF/XML | yes | yes | 166/166 (100%) |
| JSON-LD | yes | yes | via [piprate/json-gold](https://github.com/piprate/json-gold) |

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

### SPARQL Query Engine

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

- Format auto-detection by filename extension (`.ttl`, `.nt`, `.nq`, `.rdf`, `.owl`, `.jsonld`)
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
| SPARQL 1.1 Query | 328 | 328 | 100% |
| Turtle | 313 | 313 | 100% |
| N-Triples | 70 | 70 | 100% |
| N-Quads | 87 | 87 | 100% |
| RDF/XML | 166 | 166 | 100% |
| SHACL Core | 98 | 98 | 100% |

```bash
make test          # all 1928 tests
make test-sparql   # W3C SPARQL 1.1 conformance
```

## Performance

Benchmarked against Python rdflib 7.6.0 on Apple M4 Max:

| Benchmark | Go | Python | Speedup |
|-----------|---:|-------:|--------:|
| NewURIRef | 25 ns | 306 ns | **12x** |
| NewBNode | 217 ns | 2,458 ns | **11x** |
| NewLiteral (string) | 14 ns | 1,239 ns | **89x** |
| NewLiteral (int) | 15 ns | 1,813 ns | **121x** |
| URIRef.N3() | 16 ns | 272 ns | **17x** |
| Literal.N3() | 27 ns | 369 ns | **14x** |
| Literal.Eq() | 16 ns | 317 ns | **20x** |
| Store Add 10k | 10.5 ms | 80.9 ms | **8x** |
| Store Lookup 1k | 6 us | 819 us | **131x** |
| Parse Turtle | 4.5 us | 256 us | **57x** |
| Serialize Turtle | 5.1 us | 91 us | **18x** |
| SPARQL SELECT | 52 us | 1,674 us | **32x** |

```bash
go test ./benchmarks/ -bench=. -benchmem
python3 benchmarks/bench_python.py
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
  store/        Thread-safe in-memory triple store
  graph/        Graph, ConjunctiveGraph, Dataset, Resource, Collection
  namespace/    Built-in vocabularies and namespace management
  turtle/       Turtle parser and serializer
  nt/           N-Triples parser and serializer
  nq/           N-Quads parser and serializer
  rdfxml/       RDF/XML parser and serializer
  jsonld/       JSON-LD parser and serializer
  sparql/       SPARQL query parser and engine
  paths/        Property path evaluation
  shacl/        SHACL Core validator
  plugin/       Format registry and auto-detection
  examples/     Runnable example programs
  benchmarks/   Performance benchmarks
```

## Based On

This project is a Go port of [RDFLib](https://github.com/RDFLib/rdflib) (v7.x), a Python library for working with RDF. The original RDFLib is maintained by the RDFLib Team and licensed under the BSD 3-Clause License.

## Known Limitations

- No SPARQL UPDATE support
- No TriG format support yet
- JSON-LD processing delegates to [piprate/json-gold](https://github.com/piprate/json-gold) which may attempt remote context fetches

## License

BSD 3-Clause License. See [LICENSE](LICENSE).

The original Python RDFLib is Copyright (c) 2002-2025, RDFLib Team.
