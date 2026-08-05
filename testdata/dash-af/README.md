# DASH test suite — SHACL Advanced Features

Conformance tests for [SHACL-AF](https://www.w3.org/TR/shacl-af/), run by
`shacl/af_dash_test.go`.

The W3C published SHACL-AF as a Working Group Note without a test suite. DASH is
what the established implementations are tested against instead — TopQuadrant's
Java API and pySHACL both use these files — which makes it the reference for
"does this behave like the other engines".

## Provenance

| Directory | Source |
|---|---|
| `expression/`, `function/simpleSPARQLFunction`, `rules/`, `target/sparqlTarget-001` | [TopQuadrant/shacl](https://github.com/TopQuadrant/shacl), `src/test/resources/sh/tests/` |
| `function/callSPARQLFunction`, `target/sparqlTargetType-001` | [RDFLib/pySHACL](https://github.com/RDFLib/pySHACL), `test/resources/dash_tests/` |

Both projects are Apache-2.0 licensed. The files are unmodified.

## Format

Each file is self-contained: one graph holds the data, the shapes and the
expected result. Three test-case types appear:

- `dash:InferencingTestCase` — the rules must infer the reified triples in
  `dash:expectedResult`
- `dash:GraphValidationTestCase` — validation must produce the `sh:ValidationReport`
  in `dash:expectedResult`
- `dash:FunctionTestCase` — the SPARQL expression in `dash:expression` must
  evaluate to `dash:expectedResult`

`rules/triple/person.ttl` declares no test case; it is imported by
`person2schema` and `schema2person` and is skipped on its own.
