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
| v0.5.3 – v0.8.x | [v0.8.x → v0.9.0](#v08x--v090) |
| v0.9.x | [v0.9.x → next release](#v09x--next-release) |

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

## v0.8.x → v0.9.0

**API, breaking, everyone.** v0.9 makes errors and `context.Context` part of every API that can do I/O
(issue #52). Before it, a persistent store that failed a read or a write had
no way to say so: `Len` returned 0, `Triples` yielded nothing, `Add` returned
nothing, and the caller saw an empty graph. The in-memory store never fails,
so the change is mechanical for code that only uses it: add a `ctx`, handle
an `err`.

### The rule

Every method that reaches a store takes `ctx context.Context` as its first
parameter and returns an `error`. Iterators become `iter.Seq2[T, error]`:
an iteration that fails yields one `(zero, err)` pair and stops. Callers stop
at the first non-nil error:

```go
for t, err := range g.Triples(ctx, nil, nil, nil) {
	if err != nil {
		return err
	}
	use(t)
}
```

Methods that never touch storage (`Identifier`, `Base`, `Store`,
`ContextAware`, `TransactionAware`, term constructors) are unchanged.

### store.Store

| v0.8 | v0.9 |
|---|---|
| `Add(t, g)` | `Add(ctx, t, g) error` |
| `AddN(quads)` | `AddN(ctx, quads) error` |
| `Remove(p, g)` | `Remove(ctx, p, g) error` |
| `Set(t, g)` | `Set(ctx, t, g) error` |
| `Triples(p, g) iter.Seq[Triple]` | `Triples(ctx, p, g) iter.Seq2[Triple, error]` |
| `Len(g) int` | `Len(ctx, g) (int, error)` |
| `Contexts(t) iter.Seq[Term]` | `Contexts(ctx, t) iter.Seq2[Term, error]` |
| `Bind(prefix, ns)` | `Bind(ctx, prefix, ns) error` |
| `Namespace(prefix) (URIRef, bool)` | `Namespace(ctx, prefix) (URIRef, bool, error)` |
| `Prefix(ns) (string, bool)` | `Prefix(ctx, ns) (string, bool, error)` |
| `Namespaces() iter.Seq2[string, URIRef]` | `Namespaces(ctx) iter.Seq2[NamespaceBinding, error]` |

The type aliases keep their names and change their shape:
`TripleIterator = iter.Seq2[term.Triple, error]`,
`TermIterator = iter.Seq2[term.Term, error]`,
`TermPairIterator = iter.Seq2[TermPair, error]` (`TermPair{First, Second}`),
`NamespaceIterator = iter.Seq2[NamespaceBinding, error]`
(`NamespaceBinding{Prefix, Namespace}`).

Optional interfaces:

| v0.8 | v0.9 |
|---|---|
| `QueryableStore.TriplesWithLimit(p, g, limit, offset)` | `(ctx, p, g, limit, offset) TripleIterator` |
| `QueryableStore.Count(p, g) int` | `(ctx, p, g) (int, error)` |
| `QueryableStore.Exists(p, g) bool` | `(ctx, p, g) (bool, error)` |
| `CardinalityStore.Cardinality(p, g) int` | `(ctx, p, g) (int, error)` |
| `SnapshotStore.ReadSnapshot() (Store, func())` | `ReadSnapshot(ctx) (Store, func(), error)` |
| `ReachabilityStore.Reachable(q)` | `Reachable(ctx, q)` |
| `ContextBinder.BindContext(ctx) Store`, `store.BindContext` | **removed**: the context is a parameter of every call |

A store must honour `ctx`: a read or write started with a done context
returns `ctx.Err()` (wrapped is fine) and does nothing.

### graph.Graph

| v0.8 | v0.9 |
|---|---|
| `Add(s, p, o) *Graph` | `Add(ctx, s, p, o) error` (no chaining) |
| `Remove(s, p, o) *Graph` | `Remove(ctx, s, p, o) error` |
| `Set(s, p, o) *Graph` | `Set(ctx, s, p, o) error` |
| `Len() int` | `Len(ctx) (int, error)` |
| `Contains(s, p, o) bool` | `Contains(ctx, s, p, o) (bool, error)` |
| `Triples(s, p, o)` | `Triples(ctx, s, p, o) TripleIterator` |
| `Subjects/Predicates/Objects(…)` | `(ctx, …) TermIterator` |
| `SubjectPredicates/SubjectObjects/PredicateObjects(…)` | `(ctx, …) TermPairIterator` |
| `Value(s, p, o) (Term, bool)` | `Value(ctx, s, p, o) (Term, bool, error)` |
| `Bind(prefix, ns)` | `Bind(ctx, prefix, ns) error` |
| `Namespaces()` | `Namespaces(ctx) NamespaceIterator` |
| `QName(uri) string` | `QName(ctx, uri) (string, error)` |
| `Union/Intersection/Difference(other) *Graph` | `(ctx, other) (*Graph, error)` |
| `AllNodes() []Term` | `AllNodes(ctx) ([]Term, error)` |
| `Connected() bool` | `Connected(ctx) (bool, error)` |
| `Skolemize/DeSkolemize(authority, …) *Graph` | `(ctx, authority, …) (*Graph, error)` |
| `BindContext(ctx) *Graph` | **removed** |

`ConjunctiveGraph`, `Dataset`, `Collection` and `Resource` follow the same
rule: every method that reads or writes takes `ctx` and returns an error.

### Parsers and serializers

`plugin.Parser.Parse(ctx, g, r) error` and
`plugin.Serializer.Serialize(ctx, g, w) error`. The format packages follow:
`turtle.Parse(ctx, g, r, opts...)`, `turtle.Serialize(ctx, g, w, opts...)`,
and the same for nt, nq, trig, rdfxml and jsonld. A store error while
serializing fails `Serialize`; it never produces a truncated document.
`ParseStream`/`ParseStreamContext` do not touch a store and are unchanged.

### Engines

`sparql`, `shacl` and `reasoning` keep both forms (`Query` /
`QueryContext`, `Validate` / `ValidateContext`, `Expand` / `ExpandContext`):
the form without a context uses `context.Background()`. What changes is what
a store error does: it used to look like "no match"; now it is the error of
the query, update, validation or expansion, and no partial result is
returned. `paths.Path.Eval` takes `ctx` and returns
`iter.Seq2[store.TermPair, error]`; `paths.EvalContext` is gone.

### Other API changes in v0.9

- `NewGraph` binds rdf/rdfs/xsd/owl only over a MemoryStore; for any other
  store call `g.BindDefaults(ctx)`.
- `shacl.NewGraphFromRDF(ctx, g, base) (*Graph, error)`; `shacl.Graph.Add`
  and `Merge` take `ctx` and return an error. New sentinel
  `shacl.ErrQueryFailed` wraps an engine error of a shape's SPARQL query.
  `CompileShapesContext` and `EvalRuleSetContext` are new. The entry points
  without an error result (`Validate`, `Prepared.Validate`,
  `CompiledShapes.Validate`) report a store failure in the new
  `ValidationReport.Err` (with `Conforms` false and no results) and through
  `WithErrorHandler`; check `Err` or use the `*Context` variants.
  `(*shacl.Graph).Err()` reports a failed lazy index build.
- `sparql.ErrUpdateRejected` marks an update refused for what it asks (LOAD
  without a Loader, MOVE from a missing graph); a store error does not match it.
  `endpoint` answers the first with 400 and a store failure with 500.
- `sparqlstore`: a write without an update URL returns `ErrNoUpdateEndpoint`
  (it was a silent no-op); `WriteNTriples` takes `ctx`.
- `nq.Writer.WriteGraph(ctx, g)` takes `ctx`; it writes as it reads, so discard
  the output after an error.
- `reasoning.RDFSClosure`, `OWLRLClosure`, `OWLRLClosureCheck` take `ctx` and
  return an error.

### Writing a store

Run `storetest.Run` against it. It checks the signatures above, that a done
context is refused, and that an error from the store reaches the caller of
`graph.Graph` instead of looking like an empty result.

---

## v0.9.x → next release

No Go API changed. Two things can change what a program sees: queries that used to parse and no longer do, and a few answers that were wrong.

### SPARQL text that no longer parses

The parser used to accept text that is not SPARQL, and sometimes read it as something else. Each of these is now a parse error with the position, and each has a one-line fix. They do not occur in valid queries; they show up in hand-written or generated query text, and in SHACL `sh:select` text.

| Text that used to parse | Why it is not SPARQL | Write instead |
|---|---|---|
| `?` or `$` with no name (`SELECT ?` , `FILTER (? = 1)`) | it read as a variable named `""` (VARNAME needs a character, §19.8) | `?x` |
| `SELECT WHERE { … }`, `SELECT { … }` (nothing selected) | a SELECT needs `*`, a variable or `(expr AS ?v)` | `SELECT *` or name the variables |
| `VALUES d { … }`, `VALUES !d { … }` (a variable without `?`/`$`) | the first character was skipped without being looked at, so `!d` and `d` read as `?d` | `VALUES ?d { … }` |
| a bare word as a term: `?s ex:p word`, `A ex:p ?o`, `?s A ?o` | an IRI is `<…>`, a prefixed name has a colon, the type shorthand is the lower-case `a` | `<urn:x>`, `ex:word`, `a` |
| `FILTER ?x`, `FILTER 1`, `FILTER ex:f` (no call), `FILTER word` | a constraint is a bracketed expression, a built-in call or a function call (grammar [68]) | `FILTER (?x)`, `FILTER (1)`, `FILTER ex:f(?x)` |
| `FILTER !bound(?x)`, `FILTER !isLiteral(?o)`, `FILTER !EXISTS { … }` | `!` starts a UnaryExpression, which only a bracketed expression may hold ([68]); `FILTER NOT EXISTS { … }` is fine | `FILTER (!bound(?x))`, `FILTER NOT EXISTS { … }` |
| `HAVING COUNT(?o) > 1`, `HAVING ?s`, `HAVING !bound(?s)` | HAVING takes the same Constraint ([22]) | `HAVING (COUNT(?o) > 1)` |
| `GROUP_CONCAT(?o; SEPARATOR=a)`, `SEPARATOR=<urn:x>` | the separator is a string literal (§18.2.4.4); this used to panic | `SEPARATOR=", "` |
| one blank-node label in two basic graph patterns: `_:a ex:p ?v . OPTIONAL { _:a ex:q ?w }`, `{ _:a ex:p ?v } _:a ex:q ?w` | a label may not be used in two BGPs (§4.1.4); OPTIONAL, UNION, MINUS, GRAPH, BIND, VALUES, EXISTS, a subquery or a nested group starts a new one, a FILTER does not | a variable: `?a ex:p ?v . OPTIONAL { ?a ex:q ?w }` |
| two triples blocks with no `.` between them: `?x ex:q ?x ?z ex:p ?x`, `:s1 :p1 :o1 :s2 :p2 :o2` | the second subject read as another object; TriplesBlock needs the dot (grammar [55]); both Oxigraph and rdflib reject it | `?x ex:q ?x . ?z ex:p ?x` |
| a blank node, `[]`, a literal or a number as a predicate (`:x [] :q`, `:x _:a :q`) or as a graph name (`GRAPH [] { }`, `GRAPH _:g { }`) | a predicate and a graph name are a variable, an IRI or a prefixed name (VarOrIri, [106]) | a variable: `:x ?p :q`, `GRAPH ?g { }` |
| `FILTER (?x<?a&&?b>?y)` | `<?a&&?b>` is one IRIREF token, tokens are matched greedily | spaces around the operator: `?x < ?a && ?b > ?y` |

`FILTER !bound(?x)` is the one most likely to be in existing queries. It is accepted by several engines, but it is not in the SPARQL 1.1 grammar, and goRDFlib now follows the grammar. The bracketed form means the same.

To find them: parse every query you store or generate with `sparql.Parse` (or `ParseUpdate`) in a test and look at the errors. In a SHACL shapes graph, `shacl.WithErrorHandler` reports a `sh:select` that does not parse as `ErrMalformedConstraint`, and the constraint yields a violation per focus node, so a shape that suddenly reports violations everywhere has one of these in its query. A search for `FILTER\s+[^(\s]` over your query text finds the unbracketed constraints.

### SPARQL text that now parses

SPARQL 1.0 and 1.1 text the parser used to reject, found by running the DAWG SPARQL 1.0 suites (`TestW3CSPARQL10`):

- a `.` after OPTIONAL, MINUS, a group, BIND, VALUES or GRAPH (`OPTIONAL { … } .`), and a trailing `;` in a property list (`:s :p :o ; FILTER (…)`, `:s :p :o ; OPTIONAL { … }`)
- unary `+` (`FILTER (+?x > 1)`, `+1 + +2`)
- non-ASCII characters in prefixes and local names (`食:ほげ`), checked against PN_CHARS_BASE / PN_CHARS
- a prefix called `a` or beginning with `a` (`a:`, `a-b:x`); `a` alone is still `rdf:type`
- `DESCRIBE` parses (`ParsedQuery.Describe` holds its targets) and stays unevaluated: `EvalQuery` returns an error, the endpoint answers 501 as before

### Answers that were wrong

- **`=` and `<` on `xsd:date`, `xsd:time` and `xsd:dateTime` with and without a timezone.** `"2006-08-23"^^xsd:date = "2006-08-23Z"^^xsd:date` was true (the untimezoned reading was taken as UTC). The order of the two is indeterminate unless they are more than 14 hours apart (XSD 1.1 §3.3.7.2), so the comparison is now a type error, like the one `ValueEqual` already treated as unequal for `xsd:dateTime`. `"…T24:00:00"` is now the first instant of the next day instead of an unparsable value.
- **A relative namespace in `PREFIX`** (`BASE <http://example.org/x/> PREFIX : <>`) is resolved against the BASE written before it; `:x` used to stay the relative IRI `x`.
- **`xsd:dateTime(…)` as a cast** was an unbound error for every argument; it now accepts an `xsd:dateTime` or a string with a valid lexical form.
- **A variable repeated in one triple pattern.** `?x ex:p ?x` matched every triple of `ex:p` and kept the subject; it now matches only triples whose subject and object are the same term. Same for `?s ?s ?o` and a path `?x ex:p+ ?x`.
- **A zero-length path step on a term the graph does not hold.** `{ VALUES ?d { "x" } ?s ex:p* ?d }` joins the zero-length solutions of the graph's nodes with the VALUES row (§18.5); "x" is not a node, so there is no row, and it was one when `?d` came from VALUES, BIND or an earlier pattern. A term written in the pattern (`?s ex:p* "x"`) and an initial binding still match, as the W3C tests require.
- **`(^p)+` and `^p+` walked backward through a literal.** On a store without `Reachable` (Badger, SQLite; the in-memory store uses the pushdown and was right) the walk reached unrelated nodes after meeting a literal child.
- **A blank-node label in a query pattern.** `_:b ex:p ?o . _:b ex:q ?z` treated each `_:b` as an independent wildcard, a cross product; the label is a variable (§4.1.4) and the two patterns now join on it. `COUNT(*)` and `ASK` over one pattern with a repeated variable (`?x ex:p ?x`) counted or found every triple of `ex:p`; `SELECT DISTINCT *` kept rows that differed only in a hidden `[]` or label.
- **Initial bindings and MINUS.** `{ MINUS { ?this ex:p ?o } }` with `?this` pre-bound removed the solution; with the value written into the query it does not, and now neither does the pre-bound form.
- **Initial bindings and a right-hand pattern.** Queries run with initial bindings (SHACL pre-binds `$this`) now substitute the row of a join into a right-hand BGP, UNION or join of BGPs instead of evaluating it for every row. The answers are the same; the queries are much faster. Anything with an expression on the right (FILTER, BIND, OPTIONAL, subquery) still does not see the left row, as before (§18.2.1).
- **Codepoint escapes (`\uXXXX`, `\UXXXXXXXX`) are recognised only inside `<IRI>` and string literals** (SPARQL 1.2 §19.2). They used to be decoded across the whole query text first, so `ns:a\u0062c` was `ns:abc`, a whole query could be written in escapes, and `"a\u0022b"` ended the string at the decoded quote. A decoded character is now data: `"\u005Cn"` is a backslash and an `n`, not a newline. Escapes elsewhere (names, variables, keywords) and surrogates or characters an IRI may not hold (`<a\u0020b>`) are parse errors.
- **A SELECT expression may reuse a pattern variable that GROUP BY does not expose.** `SELECT (1 AS ?z) { ?s :p ?z } GROUP BY ?s` parses now (after grouping only the keys are in scope); `GROUP BY ?z` and `GROUP BY (1 AS ?z)` with the same alias are errors even when the pattern never binds `?z`.
- **A trailing VALUES clause after GROUP BY is joined with the grouped solutions**, so `SELECT ?s ?v { ?s :p ?o } GROUP BY ?s VALUES ?v { 1 }` projects `?v`; it used to be joined into the pattern before grouping. A subquery's own trailing VALUES is joined before its OFFSET and LIMIT apply (it used to be joined after).
- **`TRIPLE(s, p, o)` with a triple term as `s`** is an error (unbound), as RDF 1.2 allows only an IRI or blank node subject; it built an ill-formed triple term.

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
| v0.9.0 | every method takes `ctx` and returns an error; iterators are `iter.Seq2[T, error]`; `ContextBinder` is gone. See [v0.8.x → v0.9.0](#v08x--v090) |

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
