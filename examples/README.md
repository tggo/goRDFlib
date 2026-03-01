# Examples

Runnable examples demonstrating goRDFlib features. Each example has a `main.go` and an `output.golden` file for regression testing.

## Examples

| Directory | Description |
|-----------|-------------|
| `simple_example/` | Basic graph operations: creating terms, adding triples, pattern matching, serialization |
| `sparql_query_example/` | SPARQL queries: SELECT, ASK, CONSTRUCT, FILTER, OPTIONAL, ORDER BY |
| `format_examples/` | Parsing and serializing in Turtle, N-Triples, N-Quads, RDF/XML, JSON-LD |
| `property_paths_example/` | Property path queries: inverse, sequence, alternative, transitive closure |
| `resource_example/` | Resource API for navigating RDF graphs with a fluent interface |
| `transitive_example/` | Transitive closure queries over hierarchical data |

## Running examples

```bash
# Run a single example
go run ./examples/simple_example/

# Run all examples
for d in examples/*/; do echo "=== $d ===" && go run "./$d"; done
```

## Golden tests

Each example has an `output.golden` file containing expected output. The `TestExamples` test in the root package verifies that each example produces the expected output:

```bash
go test -run TestExamples ./...
```

To update golden files after intentional output changes:

```bash
go run ./examples/simple_example/ > examples/simple_example/output.golden
```
