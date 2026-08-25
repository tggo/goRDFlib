# rdflibgo — Project Rules

## Go Coding Standards

### Concurrency Safety
- NEVER use `math/rand` global functions — use `math/rand/v2` or a mutex-protected source
- Functions that must return the same value within a logical scope (like SPARQL `NOW()`) must receive the value from the caller, not compute it themselves
- Document thread-safety on every exported type: "safe for concurrent use" or "not safe for concurrent use"
- When shallow-copying structs that contain maps/slices, deep-copy the mutable fields to prevent shared-state mutation

### Performance
- Always pre-allocate maps with capacity when the size is known or estimable: `make(map[K]V, expectedSize)`
- Never concatenate strings with `+` or `+=` in loops — use `strings.Builder`
- Never call `regexp.Compile` in a hot path — cache compiled patterns (use `sync.Map` or package-level `var`)
- Avoid creating maps/slices inside functions called per-row (iterators, matchers). Use `sync.Pool` if reuse is possible

### Error Handling
- Use sentinel errors (`var ErrFoo = errors.New(...)`) and wrap with `fmt.Errorf("%w", err)` consistently
- Parser errors must include position information (offset, line, or context snippet)
- Numeric casts (`int64(floatVal)`) must check for overflow/NaN/Inf before casting
- Silent error suppression (returning a default instead of nil/error) must have a comment citing the spec section that justifies it

### Debuggability (don't forget)
- Errors must be **actionable**: never surface a raw stdlib/third-party error (e.g. `bufio.Scanner: token too long`) to callers. Wrap it in a package sentinel and state the remediation — name the option/limit that resolves it (e.g. `ErrLineTooLong` → "raise it with WithMaxLineLength or remove it with WithUnboundedLines").
- Re-export internal sentinels from the public package (`var ErrX = internalpkg.ErrX`) so downstream callers can `errors.Is` against them — `internal/` packages are not importable outside the module.
- When a parser/loader wraps a sub-parser, **forward its options** (`...Option`) end to end; a size/error knob that can't be reached is a debugging dead end (see jsonld→nq, shacl loaders, rdfloader).
- Prefer a verbose/trace escape hatch on parsing, loading, and batch ops: an error/event handler callback (`WithErrorHandler(func(lineNum int, line string, err error) ...)`) or a `WithVerbose`-style option, so users can trace progress and pinpoint the failing input rather than getting one terminal error.

### API Design
- Long-running or I/O functions must accept `context.Context` as the first parameter
- Follow Go naming: `MustX` for panic-on-error constructors (not `XUnsafe`)
- Validate inputs at public API boundaries (language tags per RFC 5646, IRI syntax, prefix names as NCName)
- Plugin/registry patterns must panic or return error on duplicate registration

### File Organization
- No single .go file should exceed 1000 lines. Split by concern (lexer, parser, evaluator, etc.)
- Keep one exported type per file where practical

### Format Detection (plugin/)
- Trim UTF-8 BOM and leading whitespace before content sniffing
- Document heuristic limitations in comments

### Dependencies
- Run `go mod tidy` after adding/removing imports
- Direct dependencies must not be marked `// indirect`

## Testing
- All W3C test suites must stay at 100% pass rate
- Add benchmarks (`_bench_test.go`) for performance-critical packages
- Isomorphism checks: avoid O(n²) algorithms — use reverse indexes for signature matching

## Key Packages

### Storage Backends
The `store.Store` interface (13 methods) has four implementations:

| Store | Package | Backend | Plugin name | Persistence |
|-------|---------|---------|-------------|-------------|
| MemoryStore | `store/` | In-memory maps | (default) | No |
| BadgerStore | `store/badgerstore/` | Badger v4 LSM-tree KV | `"badger"` | Yes |
| SQLiteStore | `store/sqlitestore/` | modernc.org/sqlite (pure Go) | `"sqlite"` | Yes |
| SPARQLStore | `store/sparqlstore/` | HTTP SPARQL Protocol | `"sparql"` | Remote |

- All stores use `term.TermKey()` for serialization; `term.TermFromKey()` for deserialization
- BadgerStore: 3 KV indexes (SPO/POS/OSP) via prefix scans, MVCC concurrency
- SQLiteStore: relational schema with 3 SQL indexes, WAL mode, inspectable with sqlite3 CLI
- SPARQLStore: translates Store methods to SPARQL queries/updates over HTTP
- BNode contexts treated as default graph in all persistent stores
- Write ops silently ignore errors (store.Store interface constraint)

### store/storetest (shared conformance suite)
- `storetest.Run(t, Config{New: ...})` is the executable half of the
  `store.Store` contract, run by all four in-repo backends. An out-of-tree
  backend (the MongoDB store lives in its own repo) proves conformance with it.
- Optional-interface sections are **detected, not declared**: implement
  `QueryableStore` or `ReachabilityStore` and the section turns itself on.
  Persistence is the exception — it cannot be detected, so it is `Config.Reopen`.
- `Config.Known` maps a subtest path to a **reason** and skips it. Use it only
  for what a backend genuinely cannot do (sparqlstore: named graphs on the test
  server, bnode labels over the protocol, triple terms). An empty reason is a
  hard error — an unexplained exemption is how a bug becomes a feature.
- Adding a case here changes the bar for every backend including the external
  ones, which is what the `satellite-stores` CI job exists to catch.

### store.Store context conventions (were undocumented, now pinned)
- **nil context means the default graph, not "all graphs".** The interface doc
  used to say `nil = all` for `Len` and "removes from all contexts" for
  `Remove`; all three persistent backends did the opposite. The doc was wrong,
  not the code.
- A **BNode context is the default graph** too — `Graph` passes its own
  identifier through, and an unnamed graph's identifier is a BNode.
- `Contexts` reports named graphs only.
- Iteration order is unspecified and need not be stable between calls.
  MemoryStore's genuinely is not (Go randomizes map iteration), so
  `TriplesWithLimit` cannot be required to page consistently across calls — only
  the window arithmetic is conformance-tested.
- `TriplesWithLimit` with `limit <= 0` means **no limit**. SQLite spells that as
  a negative LIMIT; passing 0 through returned nothing, which is the opposite.

### store.ReachabilityStore + paths pushdown
- `Reachable(q) ([]term.Term, error)` returns every node reachable in **>= 1**
  steps. Start is in the result only if a cycle leads back to it. Literals count
  as reachable nodes. Cycles must terminate.
- The result is **fully materialized on purpose**: a backend that cannot finish
  must return an error and no partial result, because `paths.MulPath` falls back
  to a local traversal and a partially yielded stream cannot be un-yielded. Any
  error means "compute this yourself"; `ErrReachabilityUnsupported` is the
  expected way to decline.
- `paths/reachability.go:flatten` decides what is pushable: `URIRefPath`,
  `InvPath`, `NegatedPath`, and an `AlternativePath` whose arms agree on
  direction and polarity. Sequences and nested repetitions stay local. `?` is
  never pushed — it is not transitive.
- Pushdown needs **one endpoint bound**. With both unbound it would be one round
  trip per node, so it stays local.
- MemoryStore implements `Reachable` even though it has no network to save. That
  is deliberate: it makes every property path in the W3C suite exercise the
  pushdown path, which is the only thing keeping it honest.

### bugs the conformance suite found on its first run (do not regress)
- `TermKey(TripleTerm)` emits `"T:"` + NUL-joined component keys, but
  `TermFromKey` parsed the `"T:"` body as N3. Every triple term therefore failed
  to decode, and stores skip rows they cannot decode — so RDF 1.2 triple terms
  vanished silently on read from sqlite and badger. `tripleTermFromKey` splits
  on NUL with `SplitN(…, 3)`: only the object may nest, and it is last.
- `sparql/parser_term.go:parseLiteralString` found the closing quote with
  `strings.Index`, ignoring escapes. `"he said \"hi\""` was truncated to
  `he said \`. Guard: `sparql/literal_escape_test.go`. No W3C test covers it.
- sqlitestore ran its pragmas with `db.Exec` after opening. `database/sql` pools
  connections and a PRAGMA applies only to the connection that ran it, so every
  connection after the first had no `busy_timeout` and concurrent writes died
  with SQLITE_BUSY — silently, since `Store` cannot report a write error. The
  pragmas now travel in the DSN (`dsnWithPragmas`).

### store/sparqlstore (details)
- `Server` is an httptest-based SPARQL endpoint for integration testing
- Files: `doc.go`, `store.go`, `http.go`, `server.go`, `register.go`
- Server queries `ds.Default` only; named graphs not queryable on test server
- Test coverage: 99.7% (71 tests)

### shacl/ SHACL-AF layer (af_*.go)
- SHACL-AF is a W3C **Note**, SHACL 1.2 is a **draft** respecifying the same
  ground. Both are supported and must stay **separate**: AF is opt-in via
  `WithAdvancedFeatures()`, keys off `sh:`-vocabulary; SHACL 1.2 node
  expressions key off `shnex:`. Never make AF vocabulary active by default —
  a 1.2 graph must not acquire AF semantics by accident.
- `sh:expression` is the one genuinely ambiguous property. `parseExpressionFor`
  decides: a blank node with any `shnex:` property is 1.2; otherwise AF is tried
  first with 1.2 as fallback.
- **Rules run in `sh:order`, once per focus node** — that is the spec's model,
  not a fixed point. `WithRuleIteration()` is the opt-in fixpoint. Don't make
  iteration the default: it changes results for rule sets that rely on ordering
  and can diverge.
- `Validate` must never mutate the caller's data graph. Rules infer into a copy
  (`prepareAdvanced`); `ApplyRules` is the explicit mutating entry point.
- `sh:condition` and `sh:filterShape` may be **anonymous** shapes, which are not
  in `shapesMap`. Always resolve through `resolveShape`, never a bare map lookup.
- SHACL functions are bound **per query** (`sparql.ParsedQuery.BindFunctions`),
  never via the global `sparql.RegisterFunction`: definitions come from the
  shapes graph, and two graphs may define the same IRI differently.
- Function parameter order: by `sh:order` when **all** parameters have one,
  otherwise alphabetically by the local name of `sh:path`. Argument values bind
  to initial bindings keyed by that local name.
- Recursion is bounded in three places (`maxFunctionDepth`, `afExprMaxDepth`,
  `ruleIterationLimit`) — all three are reachable with legal input.
- Reference suite: `testdata/dash-af/` (DASH), run by `shacl/af_dash_test.go`.
  It is the only cross-implementation check we have; keep it at 13/13.

### shacl/ invariants that bit us once
- `Graph.All` must handle the **fully bound** (s,p,o) pattern explicitly. It
  once fell through to the wildcard branch, so `Has(&s,&p,&o)` returned true for
  every triple in any non-empty graph — which silently made existence checks and
  two DASH tests pass for the wrong reason.
- `Graph` is safe for concurrent **reads** only. Indexes are built lazily, so
  construction is serialised and published atomically (`idx atomic.Pointer`).
  Reading while another goroutine calls `Add`/`Merge` is not safe.
- SPARQL queries from a shapes graph get that graph's own `@prefix` declarations
  as a fallback (`writeGraphPrefixes`), because shapes files routinely rely on
  them instead of `sh:declare`. An unresolved prefix does not error — it matches
  like a wildcard, which is why a missing declaration shows up as wildly too
  many results rather than as a failure.
- `parseShapes` reads Core targets only. Every AF pass must follow it with
  `addAFTargets` (`afContext.evalCtx` and `Validate` both do), or a shape whose
  only target is `sh:target` gets an empty focus node set. It once did that for
  rules only: the rule ran zero times, `applyRule` returned `(0, nil)`, and the
  same shape validated correctly — so the shape looked right and derived
  nothing. Guard: `shacl/af_target_test.go`.
- A target that cannot be run reports `ErrMalformedTarget` through
  `WithErrorHandler` rather than selecting nothing. Selecting nothing is a legal
  outcome, so a discarded error here is invisible by construction.

### sparql/ initial bindings
- `evalPatternPreBound` (used only for caller-supplied `initBindings`) pushes
  values down so `BIND`/`FILTER` expressions see them — they are constants
  substituted into the query, in scope everywhere.
- `evalPatternWithBindings` (used for join/optional inner sides) must **not**
  expose its bindings to expressions: SPARQL 1.1 §18.2.1 puts a variable bound
  in one group out of scope in a sibling group. The `bind10` W3C test is the
  guard; conflating the two functions breaks it.

### reasoning/ (RDFS + OWL 2 RL)
- Entry: `Expand(g, RDFS|OWLRL)` → `ExpandCheck` (also returns `[]Inconsistency`)
- **A class may be a blank node.** An anonymous class expression (`owl:Restriction`)
  has no IRI, so class-valued indexes (`domains`, `ranges`, `subClassOf`,
  `equivClass`) are typed `[]term.Subject`, never `[]term.URIRef`. Never
  type-assert a class to `term.URIRef` before an index lookup — key it with
  `term.TermKey()` directly. `term.Subject` admits `URIRef`/`BNode` and excludes
  `Literal`, which is what keeps a literal from being treated as a class.
- **Property**-valued indexes (`subPropOf`, `inverseOf`, `equivProp`) stay
  `term.URIRef` — RDFS and OWL 2 RL both require a property to be named.
- `ExpandCheck` must alternate RDFS and OWL RL closures **to a joint fixpoint**:
  each closure only reaches its own fixed point and does not see the other's
  output (an OWL rule derives an `rdf:type` triple that rdfs9 must then act on,
  and vice versa). Neither regime mints new terms, so the loop terminates.
- Each closure's dedup set is seeded from the graph, so re-running `Expand` on a
  closed graph adds 0 triples — that invariant is the fixpoint test.
- Regression tests for both of the above: `reasoning/bnodeclass_test.go`
