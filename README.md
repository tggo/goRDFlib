# goRDFlib

A Go port of the Python [RDFLib](https://github.com/RDFLib/rdflib) library for working with RDF (Resource Description Framework) data.

> **Warning:** This project is in early development (v0.0.x). The API is not stable and may contain bugs. Do not use in production without thorough testing.

## About

goRDFlib is a Go implementation of the core RDFLib functionality, ported from the [Python RDFLib](https://github.com/RDFLib/rdflib) library (BSD 3-Clause License). The architecture, algorithms, and test cases are derived from the Python original, adapted to idiomatic Go patterns.

## Features

- **RDF Terms** -- URIRef, BNode, Literal, Variable with N3 serialization
- **In-Memory Store** -- Thread-safe triple store with SPO/POS/OSP indices
- **Graph** -- Triple operations, pattern matching, set operations (union, intersection, difference)
- **Namespace System** -- Built-in RDF, RDFS, OWL, XSD + common vocabularies (FOAF, DC, SKOS, PROV, etc.)
- **Formats:**
  - Turtle (parser + serializer)
  - N-Triples (parser + serializer)
  - N-Quads (parser + serializer)
  - RDF/XML (parser + serializer)
  - JSON-LD (parser + serializer, via [piprate/json-gold](https://github.com/piprate/json-gold))
- **SPARQL** -- Query engine with SELECT, ASK, CONSTRUCT, FILTER, OPTIONAL, UNION, BIND, ORDER BY, LIMIT/OFFSET, DISTINCT, and 30+ built-in functions
- **Property Paths** -- Inverse, sequence, alternative, repetition (*/+/?), negated paths
- **Advanced Types** -- ConjunctiveGraph, Dataset, Collection (rdf:List), Resource
- **Plugin System** -- Format auto-detection by filename, MIME type, or content sniffing

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
    g.Serialize(os.Stdout, rdf.WithSerializeFormat("turtle"))

    // Parse Turtle
    g2 := rdf.NewGraph()
    g2.Parse(strings.NewReader(`
        @prefix ex: <http://example.org/> .
        ex:Bob a ex:Person ; ex:name "Bob" .
    `), rdf.WithFormat("turtle"))

    // SPARQL query
    result, _ := g.Query(`
        PREFIX ex: <http://example.org/>
        SELECT ?name WHERE { ?s ex:name ?name }
    `)
    for _, row := range result.Bindings {
        fmt.Println(row["name"])
    }
}
```

## Test Coverage

```
485 tests, 90% statement coverage, 0 race conditions
```

```bash
go test ./...
go test -race ./...
go test -cover ./...
```

## Based On

This project is a Go port of [RDFLib](https://github.com/RDFLib/rdflib) (v7.x), a Python library for working with RDF. The original RDFLib is maintained by the RDFLib Team and licensed under the BSD 3-Clause License.

The porting process followed a 14-phase plan covering core data model, store, graph, namespaces, parsers/serializers for 5 formats, property paths, SPARQL engine, and plugin system.

## Known Limitations

- SPARQL engine covers core features but not the full SPARQL 1.1 specification (no UPDATE, no aggregates beyond basic, no sub-queries)
- JSON-LD processing delegates to [piprate/json-gold](https://github.com/piprate/json-gold) which may attempt remote context fetches
- RDF/XML parser does not handle all edge cases (e.g., `rdf:li`, full XML Literal namespace preservation)
- N-Quads parser currently discards the graph context (4th element)
- No TriG format support yet
- Graph isomorphism (blank-node-aware comparison) not yet implemented

## License

BSD 3-Clause License. See [LICENSE](LICENSE).

The original Python RDFLib is Copyright (c) 2002-2025, RDFLib Team.
