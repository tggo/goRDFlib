# Migrating between goRDFlib versions

goRDFlib is pre-1.0, and fixing a spec violation sometimes changes what correct code gets back. This guide lists, for each step, what you may have to change and how to find the places it affects.

The public Go API has barely broken. `apidiff` between every pair of tags from v0.1.3 to v0.5.3 reports one source-visible change (v0.1.x, below). Nearly everything here is **behaviour**: results that were wrong and are now right, and invalid input or output that is now an error.

Upgrade straight to the latest version and read every section after the version you are on. Each section says who is affected, so most readers can skip most of it.

| You are on | Sections to read |
|---|---|
| v0.1.0 – v0.1.15 | all (in [Within v0.1.x](#within-v01x), the items after your version) |
| v0.1.16 | from [v0.1.x → v0.2.0](#v01x--v020) |
| v0.2.0 | from [v0.2.0 → v0.4.0](#v020--v040) |
| v0.3.x | from [v0.2.0 → v0.4.0](#v020--v040) (the v0.4.0 part) |
| v0.4.0 | from [v0.4.0 → v0.5.0](#v040--v050) |
| v0.5.0 – v0.5.2 | [v0.5.x → v0.5.3](#v05x--v053) |

If you implement `store.Store` yourself, also read [Custom store implementations](#custom-store-implementations).

---

## Within v0.1.x

**API (the only source-visible change).** The `shacl.Load*` helpers gained variadic parser options in v0.1.11, and `shacl.Validate` gained `...Option` in v0.1.15:

```go
shacl.Validate(data, shapes)                 // still compiles
shacl.LoadTurtle(r, base)                    // still compiles
var f func(*shacl.Graph, *shacl.Graph) shacl.ValidationReport = shacl.Validate // no longer compiles
```

Calls are unaffected. Only code that stores these functions in a variable or struct field of the old function type has to change: add the `...Option` parameter to that type, or wrap the call in a closure.

**Behaviour**

- **v0.1.13, SHACL:** a `sh:sparql` constraint whose `$this` is a blank node is scoped to that node. It used to fire on unrelated nodes too, so reports with blank-node focus nodes can have fewer results.
- **v0.1.14, SPARQL:**
  - A variable bound to a term that cannot sit in a position (a literal as subject) makes the pattern unsatisfiable. Before, it acted as a wildcard, most visibly inside `FILTER NOT EXISTS`.
  - `<`, `>`, `<=`, `>=` on an unbound operand are an error, so FILTER drops the row. `?unbound < 5` used to be true.
- **v0.1.15:**
  - `shacl.Graph.All` / `Has` with a fully bound pattern answered "present" for any non-empty graph; it is now exact.
  - SPARQL `initBindings` reach `BIND` and `FILTER`.
  - `reasoning.ExpandCheck` alternates RDFS and OWL RL until neither adds a triple, so it can infer more.
  - Prefixes declared in a shapes file are available inside `sh:select`.

## v0.1.x → v0.2.0

**Affects:** custom `store.Store` implementations, SQLite users, and data that contains RDF 1.2 triple terms or escaped quotes.

- **`store.Store` contract.** The written contract was corrected to match the shipped stores: `Len(nil)` counts the default graph, not every graph, and `Remove(pattern, nil)` removes from the default graph. A custom store that followed the old comments now fails `store/storetest`. The rules changed again in v0.5.3 (see [Custom store implementations](#custom-store-implementations)).
- **SQLite `TriplesWithLimit(…, 0, …)`** returned no rows; `limit <= 0` now means no limit, as on every other backend.
- **Triple terms** written to SQLite or Badger could not be read back and were silently skipped. They now decode.
- **SPARQL string literals with escaped quotes** (`"he said \"hi\""`) were truncated at the first `\"`. `FILTER` results over such literals can change.

## v0.2.0 → v0.4.0

**v0.3.x adds only opt-in features** (`WithProvenance`, source lines in SHACL reports). One change is visible: `jsonld.Parse` used to panic on some malformed input, and now returns an error wrapping `jsonld.ErrProcessorPanic`.

**v0.4.0: blank node labels are scoped to one `Parse` call.** This is the one intentional breaking change before v0.5.3.

**Affects:** code that parses several documents into one graph and expects `_:b1` in one to be the same node as `_:b1` in another, or that looks nodes up by their source label.

RDF 1.1 §3.4 scopes blank node labels to the document, and rdflib, Jena and RDF4J do the same. To keep the old behaviour, opt out on the parser you use:

```go
turtle.Parse(g, r, turtle.WithPreserveBlankNodeIDs())
// also trig, nt, nq, rdfxml, jsonld, and rdfloader.WithPreserveBlankNodeIDs()
```

With JSON-LD, preserve mode keeps json-gold's labels, not the ones in your source.

## v0.4.0 → v0.5.0

**Affects:** SHACL users who read result messages, or who walk results.

- **Message templates.** In a SPARQL-based constraint, `{?var}` and `{$var}` in `sh:message` are filled in from the solution (SHACL §5.3.2). If you matched on the literal template text, match on the filled-in text instead.
- **Nested `sh:node` results.** A `sh:node` violation now carries the nested shape's results in `ValidationResult.Details`. Code that walks results recursively sees more of them.

## v0.5.0 → v0.5.2

**Affects:** code that relies on the order of SPARQL solutions without `ORDER BY`.

The engine now chooses the join order of triple patterns, so rows come out in the order of the plan rather than the written order. SPARQL leaves that order unspecified, and the set of solutions is unchanged. If the order matters, add `ORDER BY`.

v0.5.2 adds the optional `store.CardinalityStore`. You don't have to implement it; stores without it are still planned correctly, just with a little more work.

## v0.5.x → v0.5.3

This release fixed about 50 bugs, and many of the fixes change results. Check the parts you use.

### Named graphs and the default graph

**Affects:** anyone using datasets, TriG, N-Quads or `graph.Graph.Identifier()`, and every custom store.

| Before | Now |
|---|---|
| `graph.NewGraph().Identifier()` returns a new blank node | returns `store.DefaultGraph` (`<urn:x-rdflib:default>`) |
| a blank node context means the default graph | a blank node context is a **named graph** (TriG `_:g { }`) |
| `MemoryStore` ignores contexts, so `graph.NewDataset()` merges every named graph | `MemoryStore` keeps graphs apart |
| `ConjunctiveGraph` / `Dataset` `Triples`/`Len` give everything on MemoryStore and only the default graph on Badger/SQLite | the union of all graphs on every store; `Remove(…, nil)` clears every graph |

What to change:

- Test for the default graph with `store.IsDefaultGraph(ctx)` (or `rdflibgo.IsDefaultGraph`), never with `ctx == nil` or "is a blank node".
- If you built unnamed graphs by passing a blank node as their identifier and expected the default graph, pass `store.DefaultGraph` or nil instead.
- If you worked around MemoryStore's missing graph support with an in-memory Badger store, you can drop the workaround.
- Stored data needs no migration. Badger and SQLite always stored blank-node contexts under the default-graph key, so existing data stays in the default graph. Only new writes to a blank-node context land in a separate graph.

Find affected code:

```sh
grep -rn 'Identifier()' --include='*.go' .
grep -rn 'term.BNode' --include='*.go' . | grep -i 'context\|graph\|ctx'
grep -rn 'ctx == nil\|context == nil' --include='*.go' .
```

### SPARQL

**Affects:** queries over data with mixed or ill-typed literals, queries that call functions on possibly unbound variables, and code that reads variable lists.

- **Equality follows datatypes.** `"1" = 1` is false. Comparing literals of different or unknown datatypes, or ill-typed literals such as `"abc"^^xsd:integer`, is a type error (§17.4.1.7), so FILTER drops the row. Before, `"abc"^^xsd:integer` counted as 0.
- **Built-ins raise errors instead of inventing values.** With an unbound or wrongly typed argument, `MD5`, `STRLEN`, `UCASE`, `CONTAINS`, `REGEX`, `ABS` and the rest raise an error (§17.4.1.3). `BIND` then leaves the variable unbound and FILTER treats the row as false. Before, `MD5(?unbound)` was the hash of `""` and `STRLEN(<urn:x>)` was 5. `COALESCE` is the way to supply a fallback.
- **Numeric types.**
  - `xsd:short`, `xsd:byte`, `xsd:nonNegativeInteger` and the other derived integer types are numeric.
  - Integer division returns `xsd:decimal`, as does `AVG` over integers.
  - `ROUND(-2.5)` is `-2`.
  - The date functions (`YEAR` and the rest) need a typed `xsd:dateTime` / `xsd:date` / `xsd:time`.
- **Blank nodes in updates and CONSTRUCT are fresh.** `INSERT DATA { _:a … }` creates a new node on every request, and INSERT/CONSTRUCT templates create one per solution. Two separate requests no longer share `_:a`.
- **Unbound `GRAPH ?g` in a DELETE/INSERT template** skips that quad (Update §3.1.3). Before, it wrote to or deleted from the default graph.
- **`SELECT *`** lists every in-scope variable, including OPTIONAL variables that end up unbound and variables named `?_x`. Parser-generated variables start with `.`; filter those out if you print variable lists.
- **Strings and names.**
  - `\b`, `\f` and `\'` are decoded, and malformed escapes (`\q`, `\u00`) are syntax errors.
  - Prefixed names such as `ex:a-b`, `ex:a.b` and `ex:a\/b` work in expressions.
- `EXISTS` works in SELECT expressions, `HAVING` and `ORDER BY`.

### Parsers

**Affects:** anyone parsing untrusted or hand-written data.

| Input | Before | Now |
|---|---|---|
| Turtle/TriG `"x"@abcdefghi` (malformed language tag) | the tag was silently dropped | syntax error |
| Turtle/TriG `"a"^^rdf:langString` without a tag | accepted | syntax error |
| RDF 1.2 `<<( :s :p [ :r 1 ] )>>` | accepted | syntax error (ttObject allows no property list) |
| nesting deeper than 10000 levels | fatal stack overflow | `turtle.ErrNestingTooDeep` / `trig.ErrNestingTooDeep`; raise the limit with `WithMaxParseDepth` (also on `rdfloader`) |
| relative IRIs with a Unicode base or an opaque base (`urn:`) | resolved through net/url, percent-encoded or wrong | RFC 3986 §5.2 on the IRI string |
| RDF/XML `rdf:about="a%2Fb"` | decoded to `a/b` | kept as `a%2Fb` |
| JSON-LD statement with an ill-formed IRI | parse error, unless `WithSkipInvalidIRIs` | statement dropped (JSON-LD 1.1 API §8.1) |
| JSON-LD parse that fails part-way | graph left half-filled | graph unchanged |

JSON-LD options:

```go
jsonld.Parse(g, r)                          // drops ill-formed IRIs (new default)
jsonld.Parse(g, r, jsonld.WithStrictIRIs()) // old default: fail instead
jsonld.Parse(g, r, jsonld.WithSkipHandler(func(stmt string, err error) {
    log.Printf("dropped: %s: %v", stmt, err) // see what was dropped
}))
```

`jsonld.WithSkipInvalidIRIs()` still compiles, but it is deprecated and does nothing; remove it.

`term.NewURIRef` rejects characters `#x00`–`#x20`. Check them up front with `term.ValidIRI`, and language tags with `term.ValidLanguageTag`.

### Serializers

**Affects:** anyone who writes output and ignored the returned error, or who compares output byte for byte.

Serializers now return errors where they used to write invalid output or lose triples. **Check the error from `Serialize`.**

| Serializer | New errors | When |
|---|---|---|
| `rdfxml` | `ErrNoQName`, `ErrReservedPropertyName`, `ErrUnrepresentableChar` | a predicate IRI with no valid XML name, `rdf:about` etc. used as a predicate, characters XML 1.0 cannot carry |
| `nt`, `nq` | `ErrRelativeIRI`, `ErrInvalidUTF8`, `ErrInvalidIRI` | relative IRIs, invalid UTF-8, characters IRIREF forbids |
| `turtle`, `trig` | an error wrapping `rdflibgo.ErrInvalidIRI` | an IRI the grammar cannot express |

Output that changes on purpose:

- **RDF/XML:**
  - Unbound namespaces get generated prefixes (`ns1`, `ns2`…) instead of invalid element names.
  - Triple terms are written with `rdf:parseType="Triple"`, and base direction with `its:dir`.
  - A failed `Serialize` writes nothing.
- **Turtle/TriG:**
  - Lists that are not well-formed collections are written as plain triples instead of being dropped.
  - Blank nodes inside triple terms, and blank nodes shared between graphs, keep their labels.
  - Nesting deeper than 64 levels (TriG) or `WithMaxNestDepth` (Turtle, capped at 1024) is written as labelled nodes.
  - Multi-line literals ending in `"` are escaped correctly.
- **JSON-LD:** the context holds only prefixes that are valid and used, and output is always compacted.
- **Literals:** `NewLiteral(math.Inf(1))` is `INF`, not `+Inf`. An ill-typed `"1.5e3"^^xsd:decimal` is written quoted, not as the shorthand `1.5e3`.

### Terms

**Affects:** code using `Literal.ValueEqual` and code that stores literals.

- **`Literal.ValueEqual` is exact:**
  - decimals and big integers compare without float rounding;
  - dateTimes compare with the timezone normalised, and a dateTime with a timezone never equals one without;
  - an ill-typed literal equals only an identical literal;
  - `"a"@en` does not equal `"a"@fr`.
- **Store keys for ill-typed numerics.** `MemoryStore` kept only one of `"1.5e3"^^xsd:decimal` and `"1.5e3"^^xsd:double`, and a valid `"1.5e3"^^xsd:double` read back from Badger/SQLite/Mongo came out as `xsd:decimal`. Both are fixed. Keys change only for ill-typed decimals and doubles that used the shorthand, and for literals ending in a quote, which could not be decoded before.

### SHACL

**Affects:** code that asserts on report contents or order, and shapes that use `sh:datatype`, `sh:entailment` or recursion.

- **Stable order.** Results come out in the same order on every run, grouped by focus node. Tests that pinned the old, random order will need updating once, and then stay stable.
- **Stricter `sh:datatype`.** Ill-typed dateTime, date, time, dateTimeStamp, g* types, durations, hexBinary and base64Binary values are violations (§4.1.2). Data that used to conform can now fail.
- **Failures the report cannot show** go to `shacl.WithErrorHandler`:
  - `shacl.ErrUnsupportedEntailment`: `sh:entailment` with any regime other than `sh:Rules`, which needs `WithAdvancedFeatures`;
  - `shacl.ErrAdvancedFeatures`: the shapes graph declares `sh:SPARQLFunction` but advanced features are off.

  Install a handler if you don't have one; without it these conditions are invisible.
- **Recursive shapes over cyclic data** used to crash the process with a stack overflow. They now terminate: a (shape, node) pair already being validated on the current path is assumed to conform (§3.4.3).

---

## Custom store implementations

The current contract is checked by `store/storetest`. Run it against your store on every goRDFlib upgrade:

```go
func TestConformance(t *testing.T) {
    storetest.Run(t, storetest.Config{
        New: func(t *testing.T) store.Store { return mystore.New() },
    })
}
```

Rules that changed:

| Version | Rule |
|---|---|
| v0.2.0 | a nil context is the default graph, not every graph (`Len`, `Remove`, `Triples`); `TriplesWithLimit` with `limit <= 0` means no limit |
| v0.5.3 | the default graph is nil **or** `store.DefaultGraph`; route every context through `store.IsDefaultGraph`. Any other term, blank nodes included, names a graph |
| v0.5.3 | if you implement `store.CardinalityStore`, the count must be exact per pattern **and** context |

The v0.5.3 change in code, as made in [rdflibgo-mongostore](https://github.com/tggo/rdflibgo-mongostore/commit/01b53cb):

```go
// before
func graphKey(ctx term.Term) string {
    if ctx == nil {
        return defaultGraphKey
    }
    if _, isBNode := ctx.(term.BNode); isBNode {
        return defaultGraphKey
    }
    return term.TermKey(ctx)
}

// after
func graphKey(ctx term.Term) string {
    if store.IsDefaultGraph(ctx) {
        return defaultGraphKey
    }
    return term.TermKey(ctx)
}
```

A store that could not persist blank-node graph names before keeps its data as it was: those triples were written under the default-graph key, and they stay in the default graph.
