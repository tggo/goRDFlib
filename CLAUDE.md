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
| MongoStore | **external**: `github.com/tggo/rdflibgo-mongostore` | MongoDB | `"mongo"` | Yes |

- All stores use `term.TermKey()` for serialization; `term.TermFromKey()` for deserialization
- BadgerStore: 3 KV indexes (SPO/POS/OSP) via prefix scans, MVCC concurrency
- SQLiteStore: relational schema with 3 SQL indexes, WAL mode, inspectable with sqlite3 CLI
- SPARQLStore: translates Store methods to SPARQL queries/updates over HTTP
- The default graph is `store.DefaultGraph`; a BNode context is a named graph in every store except sparqlstore (see the conventions below)
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
  ones, which is what the `satellite-stores` CI job exists to catch. The one
  satellite today is `github.com/tggo/rdflibgo-mongostore`; the job builds and
  runs its integration suite against every commit here, via a `replace`.

### store.Store context conventions (were undocumented, now pinned)
- **nil context means the default graph, not "all graphs".** The interface doc
  used to say `nil = all` for `Len` and "removes from all contexts" for
  `Remove`; all three persistent backends did the opposite. The doc was wrong,
  not the code.
- **The default graph has an identifier: `store.DefaultGraph`
  (`<urn:x-rdflib:default>`).** A nil context and `DefaultGraph` both address
  it; test with `store.IsDefaultGraph`, never `ctx == nil`. `graph.NewGraph()`
  and a Dataset's default context carry it, which is what keeps `sparql`/`paths`
  (they pass `g.Identifier()` to the store) on the default graph.
- **A BNode context is a named graph** (TriG `_:g { }`, rdflib #2445). It used
  to be folded into the default graph in every store, because unnamed graphs
  had BNode identifiers. Old Badger/SQLite data is unaffected (it was stored
  under the empty key). sparqlstore cannot express it (SPARQL `GRAPH` takes
  VarOrIri) and declares that in `Known`. The Mongo satellite had to switch its
  `graphKey` to `store.IsDefaultGraph`.
- **MemoryStore is context-aware:** one index set per graph; the default graph
  is a fixed field so single-graph reads don't pay a map lookup. `Triples` must
  not capture the context in the default-graph closure (keeps its allocation
  size class). Cardinality is exact per pattern AND context.
- **`ConjunctiveGraph` is a real union:** `Triples`/`Len` dedup across the
  default and named graphs. It only looked like one before because MemoryStore
  ignored contexts.
- `Contexts` reports named graphs only.
- Iteration order is unspecified and need not be stable between calls.
  MemoryStore's genuinely is not (Go randomizes map iteration), so
  `TriplesWithLimit` cannot be required to page consistently across calls — only
  the window arithmetic is conformance-tested.
- `TriplesWithLimit` with `limit <= 0` means **no limit**. SQLite spells that as
  a negative LIMIT; passing 0 through returned nothing, which is the opposite.

### context.Context reaching stores (issue #35, `store.ContextBinder`)
- `store.Store` takes no `context.Context`, and adding one would break every
  backend including the satellites. A store opts in with
  `BindContext(ctx) Store`, a **view** over the same data. Engines bind, they
  never change the Store interface.
- The view must keep the receiver's optional interfaces (Queryable, Snapshot,
  Cardinality, Reachability); engines type-assert on the store they get, so a
  view that drops one silently loses the pushdown. storetest checks it.
- Queries bind once at entry (`evalCtx.bindQuery`, before the snapshot
  redirect). Updates bind **where a graph is taken from the Dataset**
  (`ec.bind`, `getOrCreateGraph`), because operations create and drop entries
  in `Dataset.NamedGraphs`: a bound copy of the map would lose them, and a
  bound view stored in the caller's map would outlive the request (and break
  `endpoint`'s `g.Store() == st` commit check). Guard:
  `TestUpdateContextReachesStore`.
- A bound store that gives up on a read looks like "no match" (Store has no
  errors). So a query that used one polls once more before returning, SHACL
  checks after every focus node and at the end, and a stopped run returns an
  error, never a partial result. Prepared must not cache targets selected
  while the context was done.
- A query stopped by its context is never reported through `WithErrorHandler`
  (`config.report` filters it): it is not a malformed target or rule.
- SHACL carries the context on `recursionGuard.ctx`, because the guard is the
  one object every derived evalContext/nodeExprContext already shares. SHACL
  functions do not use the afContext's context: they are `ContextFunction`s
  and run with the context of the query that calls them.
- Extension functions: `Function` stays; `ContextFunction` sits beside it in
  one registry. `FuncExpr.Fn`/`FnCtx`: at most one is set. Do not wrap `Fn` in
  a closure per call (it allocates per row).
- Reasoning: an OWL RL rule pass can be long (prp-trp is quadratic in a chain),
  so `emit` ticks the stopper. A pass cut short adds nothing: the dedup set
  already counts its unemitted triples as known.

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
- `sparql/parser_path.go:parsePathEltOrInverse` returned from the `^` branch
  before looking for a modifier, so `^ex:p+` (legal SPARQL) failed to parse and
  only `(^ex:p)+` worked. The fix normalises `^p+` to `(^p)+` — inverting a
  closure and closing an inverse are the same relation — which also keeps the
  path in the flat shape `flatten` can push down. Guard:
  `sparql/inverse_path_test.go`. Found by the MongoDB backend's tests.
- sqlitestore ran its pragmas with `db.Exec` after opening. `database/sql` pools
  connections and a PRAGMA applies only to the connection that ran it, so every
  connection after the first had no `busy_timeout` and concurrent writes died
  with SQLITE_BUSY — silently, since `Store` cannot report a write error. The
  pragmas now travel in the DSN (`dsnWithPragmas`).

### store/sparqlstore (details)
- `Server` is deprecated and delegates to `endpoint`; it answers updates with 204
  and serves named graphs and triple terms (the conformance exemptions for both
  are gone).
- Files: `doc.go`, `store.go`, `http.go`, `server.go`, `register.go`

### store.SnapshotStore (Badger)
- A query opens one read snapshot per store (`sparql/snapshot.go`, from
  `EvalQueryContext`) instead of one Badger transaction per lookup. Opening and
  discarding a transaction goes through Badger's watermark channel, and under
  16 concurrent queries that bookkeeping was most of the CPU.
- Badger iterators run with `PrefetchValues = false` (`iteratePrefix`): the
  default prefetches values in goroutines, which costs more than short index
  lookups of small values save.
- Measured: `BenchmarkStoreBackendQuery/badger` 212 µs → 66 µs; sparql-server
  throughput on Badger ~4.8k → ~14.6k req/s (median of 6 alternating runs).
- Badger throughput runs right after a bulk load are bimodal (background
  compaction). Compare builds by alternating processes.
- SQLite is still ~250 req/s on the same test: it has no snapshot or
  cardinality support yet.

### endpoint/ (SPARQL 1.1 Protocol + Graph Store HTTP Protocol)
- `New(*sparql.Dataset)`, `NewForGraph`, `NewForStore(*graph.Dataset)` (copies
  graphs an update creates into the store, because `getOrCreateGraph` makes
  in-memory graphs). Auth, metrics, limits are options; CORS, gzip, TLS and the
  listener belong to the binary (tggo/sparql-server).
- W3C manifests `protocol` (34/34) and `http-rdf-update` (19/19) run from the
  manifests themselves; four GSP manifest defects are worked around and
  commented in `w3c_gsp_test.go`.
- Status decisions: 422 for `WithMaxResultRows` (fail, never truncate), 503
  without Retry-After for timeouts (incl. waiting for the lock), 501 DESCRIBE,
  499 in the hook on client disconnect.
- SELECT/ASK are evaluated fully, then `results.Write` pre-checks every value
  before the first byte; a failure after the header aborts the connection
  (`http.ErrAbortHandler`) rather than sending a complete-looking document.
- Lock: readers-writer, cancellable, writer-preferring. Queries share it while
  evaluating (not while writing the response); updates and GSP writes hold it
  exclusively. Multi-operation updates are not atomic.
- Security defaults: LOAD disabled unless `WithLoader` (rdfloader would read
  local files: SSRF); JSON-LD payloads never fetch remote contexts; body limits
  on every request; panics recovered.
- FROM/FROM NAMED come from `ParsedQuery.DatasetClause` (issue #37); the
  endpoint resolves a relative one against the service base and lets protocol
  parameters win (§2.1.4). `scan.go` is left with `scanForm`, the one thing
  that must happen before parsing: DESCRIBE and updates get their own status
  instead of a syntax error.

### sparql/ cancellation and results
- `sparql/cancel.go`: every evaluation loop calls `ec.stop()` (polls ctx every
  1024 calls, counts matched triples not rows) and passes `ec` down, never nil.
  An update builds its changes first and calls `ec.poll()` before the first
  write. New path types implement `eval(st, …)`; a recursive traversal must stop
  when a deeper level returns false.
- Cancellation test pattern: cancel after 20 ms, allow 250 ms, and prove the test
  fails with the check removed. A triple term with variables in the BGP stops the
  planner from reordering a deliberately slow query.
- `sparql/results`: `check.go` must reject exactly what the encoders reject
  (`FuzzWriters` enforces it). A Writer writes nothing before the first row, and
  blank nodes are relabelled per document. One-column CSV writes `""` for empty
  values because CSV readers skip blank lines. `TestW3CCSVTSV` is the
  conformance suite.

### provenance/ + shacl source lines
- `provenance.Index` collects the `WithProvenance` callback the nt/nq/turtle/trig
  parsers already emit; `idx.Triple` and `idx.Quad` are written to match those
  handler signatures exactly, so they are passed as method values.
- `shacl.WithSourceLines(idx)` fills `ValidationResult.SourceLine`. Annotation
  happens **once over the finished report** (`annotateSourceLines` at the end of
  `Validate`), never at the eight places a result is constructed — a new
  constraint gets line reporting for free and never learns provenance exists.
- Two precisions, and they must stay distinguishable via `SourceLineKind`:
  a **triple** line (focus + IRI path + value all bound → the exact triple) and
  a **focus node** line (everything else). A `sh:minCount` violation has no
  offending triple — the triple is what is missing — so reporting a triple line
  there would be a lie.
- `ResultPath` must be an **IRI** for the triple lookup. A sequence/alternative/
  inverse path is a blank node in the shapes graph and the value is several hops
  away, so no single triple is the offender.
- The key separator in `provenance` is **NUL** on purpose: a literal's lexical
  form goes into `term.TermKey` verbatim, so a printable separator would let a
  crafted literal forge another triple's key. Guard:
  `TestKeySeparatorCannotCollide`.
- All six parsers report provenance. Precision differs by format and that is
  not a bug: nt/nq/turtle/trig/rdfxml are **per triple**, JSON-LD is **per node
  object**.
- `rdfxml`: every triple goes through `p.add`, never `p.g.Add` — a new
  production that calls `g.Add` directly would silently skip provenance. Guard:
  `TestProvenanceEveryTripleIsReported` compares index size to graph size.
- `jsonld`: json-gold discards positions (piprate/json-gold#96), so the source
  is scanned separately for `@id` positions and those are expanded through the
  document's **own** context via `ld.Context.ExpandIri` — not a hand-rolled
  prefix table. That is what makes it exact: a failed expansion yields no line
  rather than a wrong one. Only the **top-level** `@context` is used; scoped and
  remote contexts are not seen. `@id` aliases are collected in a separate pass
  because a context may be written after the nodes that use it.
- `jsonld.WithExpandContext` (issue #29) sets json-gold's `ExpandContext`. The
  processor parses it into the active context **before** the document's own
  `@context`, so the document wins term by term — it is for defaults and
  pre-declared terms, not for overriding a document. `iriExpander` must apply
  the two in the same order or provenance would expand an `@id` differently
  from the triples. `scanIDAliases` also reads `@id` aliases from an inline
  expand context; an IRI-valued one is not fetched for that.
- The JSON-LD position scan reads `dec.InputOffset()` **after** `Token()`.
  Before it, the decoder still sits where the previous token ended — on the far
  side of the whitespace, so usually on the previous line. That was an
  off-by-one in every case with indentation.
- `shacl/rdf.go:toTerm` must be the **exact inverse** of `fromRDFLib`. It was
  not: `fromRDFLib` folds a base direction into the language as `lang--dir` and
  `toTerm` left it folded, producing a literal whose `TermKey` differed from the
  original — so every provenance lookup on a directional literal silently missed
  and degraded to the focus-node line. Anything keyed by `TermKey` across that
  boundary has the same exposure.
- Fuzzing: `make test-fuzz` (`FUZZTIME=5m` to hunt). Targets live in
  `provenance/fuzz_test.go`, `jsonld/fuzz_test.go`, `rdfxml/fuzz_test.go`,
  `shacl/sourceline_fuzz_test.go`. Seed corpora run as ordinary tests on every
  `go test`, which is where the regression value is.
- The JSON-LD fuzz target found that **json-gold panics** rather than erroring
  on some malformed input (`{"@id":"%"}` with a base: `url.Parse` fails,
  json-gold dereferences the nil result). `jsonld.guard` catches it and returns
  `ErrProcessorPanic` with the stack. A parser is routinely aimed at untrusted
  bytes; taking the process down is not an acceptable failure mode. Wrap any new
  json-gold entry point the same way.
- Two `if err != nil` branches in `jsonld/provenance.go` are unreachable with
  current json-gold and stay uncovered on purpose — they guard error returns
  that exist in the API.
- Context for why any of this exists: oxigraph/oxigraph#1526,
  RDFLib/pySHACL#321 (closed as impossible on RDFLib).

### blank node scoping (internal/bnodes, PR #28)
- Every parser gives blank node labels a **fresh scope per Parse call** (RDF 1.1
  §3.4: a label identifies a node within one document). `_:b1` in two documents
  parsed into one graph are two nodes. This is what rdflib, Jena and RDF4J do;
  `WithPreserveBlankNodeIDs` (named after RDF4J's `PRESERVE_BNODE_IDS`) is the
  opt-out for callers that manage identity themselves. Never make preserve the
  default again — merging unrelated documents through `_:b1` is silent data
  corruption.
- The scope **replaces** a label with a fresh `NewBNode()` via a per-parse map.
  It must not *prefix* it: the first version did (`N<uuid>_b1`), and every
  parse → serialize → parse hop then grew the label by another 33 bytes without
  bound. Guard: `nt/roundtrip_bnode_test.go`, `TestScopeLabelDoesNotGrow`.
- A `Scope` owns a map and is **not** safe for concurrent use; a parse runs on
  one goroutine. Anonymous nodes (`[]`, missing `@id`) never go through the
  scope and are always fresh.
- JSON-LD: json-gold relabels source blank nodes before the N-Quads stage, so
  even preserve mode keeps json-gold's labels, not the source ones, and
  provenance reports no line for an explicit `_:` `@id`.
- Splitting one document into several Parse calls (the SPARQL 1.2 test runner's
  TriG splitter does) needs preserve, or cross-fragment labels come apart.

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

### shacl/ SPARQL result messages (issue #31, `sparql_message.go`)
- Precedence per SHACL §5.3.2: `?message` binding → `sh:message` of the
  constraint / validator → the **shape's** `sh:message` (set by `makeResult`,
  copied verbatim, §2.1.5) → `sh:message` of the component. The component's
  message is shared by every shape using it, so it must not overwrite a
  shape's own; the first version did.
- `{?var}`/`{$var}` are expanded in **one left-to-right pass**. Sequential
  per-variable replacement (what pySHACL does) re-expands a data value that
  itself contains `{?other}`. Guard: `FuzzExpandTemplate`.
- The `?message` binding is used **verbatim**, never as a template — it comes
  from the data. Unbound placeholders stay as written so a typo is visible.
- `sh:detail`: `sh:node` keeps the nested results in `Details` (PR #30).

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

### sparql/ BGP join ordering (`bgp_order.go`)
- `evalBGP` plans, `evalBGPInOrder` runs the nested loops. Recursion must stay
  in `evalBGPInOrder`, or every partial solution re-plans the rest of the BGP.
- A `store.CardinalityStore` (MemoryStore) gives exact counts from index sizes
  and is never probed (`TestOrderBGP_CardinalityStoreIsNotProbed`). Its
  conformance section checks Cardinality against Triples after
  Remove/Set/duplicates; MemoryStore keeps a per-predicate counter for `(?, p, ?)`.
  Planner tests run on both paths via `forEachPlannerStore`.
- Other stores are probed, and probing IS the planner's cost: a counted match
  costs about what an evaluated one does. The probe limit starts at 64, shrinks to the smallest exact count,
  and only goes to 1000 when everything was capped.
  `TestOrderBGP_ProbesStopAtSmallestCount` guards the shrink.
- Capped counts are lower bounds: exact counts rank first, and capped counts
  are evened out to one limit, so probe order never decides the plan.
- Result order without ORDER BY now follows the plan, not the written order.
- Every rule has a test that fails when the rule is removed (checked by
  mutation). `TestOrderBGP_SameSolutionsForEveryPermutation` is the
  correctness guard.
- `benchmarks/bench_bgp_order_test.go` has written-well vs badly-written
  queries. A 4-pattern chain from a constant costs ~1µs more for planning; the
  badly-written queries went from seconds to µs.

### MemoryStore lookups
- `triplesLocked` must index straight on every bound key pair (`spo[s][p]`,
  `pos[p][o]`, `osp[o][s]`). It used to iterate the outer map and filter the
  second key, so `(?, p, o)` scanned every object of `p`. That cost 10µs on
  `StoreLookup_FilmsByDirector_100k`, down to 77ns after the fix.

### store stress numbers (README "Store Stress Test")
- Ingest and heap delta come from `TestStress3M` (one run). Reads come from
  `BenchmarkStress3M`, which loads each backend once per process and repeats
  every read. The old one-shot timings after an 8 GB ingest measured GC:
  `Len()` differed 47x between identical runs.
- Compare versions by alternating processes AND reversing the order. A version
  that always runs last after 8 GB processes looked 26% slower on
  SubjectLookup; with the order reversed there was no difference.
- modernc SQLite allocates outside the Go heap: heap delta cannot measure it.

### term/ literals and IRIs
- Numeric N3 shorthand (writer and `TermFromKey` decoder) follows the Turtle
  INTEGER/DECIMAL/DOUBLE productions in `term/turtle_numeric.go`. They are
  disjoint, which keeps `TermKey` injective — every store indexes on it. A
  looser check once merged "1.5e3"^^xsd:decimal with the double in the store.
- `testutil.AssertGraphEqual` compares term identity (lexical, datatype, lang,
  dir), never `N3()`. It is exponential on long blank-node chains (every link
  has the same signature): walk such chains link by link in tests.
- `Literal.ValueEqual` (`literal_value.go`) is exact: math/big for integers and
  decimals, XSD lexical spaces only, timezoned and untimezoned dateTimes never
  equal, NaN != NaN, an ill-typed literal equals only an identical one.
- `NewURIRef` rejects #x00-#x20; `ValidIRI`, `ValidLanguageTag` are exported
  (and re-exported from the root package).

### parsers and serializers
- Relative IRIs go through `internal/iri.Resolve` (RFC 3986 §5.2 on strings).
  Never use net/url for IRIs (it percent-encodes, breaks opaque bases, drops an
  empty `#`), and never unescape a resolved IRI.
- Turtle/TriG parsing is bounded by `WithMaxParseDepth` (default 10000, also on
  rdfloader); serializers flatten nesting beyond 64 (TriG) / `WithMaxNestDepth`
  capped at 1024 (Turtle). A Go stack overflow is fatal and cannot be recovered.
- With `WithBase`, the Turtle and TriG serializers write IRIs relative to the
  base (issue #34) through `internal/iri.Relativize`, which keeps a candidate
  only if `Resolve(base, candidate) == target`. Never shorten an IRI without
  that check: resolution normalises dot segments, so a hand-built relative
  form can name a different resource. A prefixed name still wins over a
  relative IRI. Guard: `FuzzRelativize`.
- A list is written as `( ... )` only under the rule in
  `turtle/serializer_lists.go`; TriG has a copy — keep them in sync. TriG counts
  blank-node references across the whole dataset (`bnodeUsage`); blank nodes
  inside triple terms are never inlined.
- Round-trip tests for multi-graph datasets: use a Badger store and flatten by
  putting the graph name into the predicate; encoding quads as triple terms
  makes the isomorphism check hang.
- RDF/XML serializer renders the body before writing anything (a failed
  Serialize writes nothing) and decides prefixes while rendering; unwritable
  names are `ErrNoQName` / `ErrReservedPropertyName` / `ErrUnrepresentableChar`.
- N-Triples/N-Quads serializers refuse any term their own parser would reject.
- JSON-LD parsing adds to the graph only on success and drops ill-formed IRIs by
  default (`WithStrictIRIs` to fail). `resolveEmptyFragmentVocab` works around
  json-gold dropping an empty `#` in `@vocab`; remove it once upstream fixes it.

### sparql/ expression semantics
- A nil term means "error" throughout the evaluator. Built-ins get arguments
  already checked for unbound; only FILTER turns an error into false.
- `=` and `<` use `rdfTermEqual`/`valueCompare`; ORDER BY, MIN, MAX use the
  total order in `compareTermValues`. Don't mix them.
- The W3C result comparison matches numbers by value only (`srx.go`), so the
  datatype and lexical form of numeric results need our own tests.
- An ill-typed xsd:boolean used as a boolean is an error (follows SPARQL 1.2
  test `expression/not-not`, which contradicts 1.1).
- Parser-generated variables start with `.` (`isInternalVar`); never hide user
  variables by a `_` prefix again.
- Template blank nodes need one `bnodes.New(false)` per instantiation; don't
  write generated nodes into solution maps, rows can share them.
- Known gaps: `BNODE("x")` is not fresh per solution; `_:b` in a WHERE pattern
  acts as an unnamed wildcard; aggregates inside function calls stay unbound.

### shacl/ recursion, datatypes, order
- Any new place that creates an `evalContext` or `nodeExprContext` must pass on
  `guard` (`sharedGuard()`); a fresh guard lets a cycle through that boundary
  overflow the stack. Recursive (shape, node) pairs are assumed to conform
  (§3.4.3); the guard is a per-path set, not a cache.
- Shape definitions in node expressions come from
  `nodeExprContext.shapeDefinitions()`, never `ctx.dataGraph` (the W3C tests
  keep shapes and data in one graph and cannot catch this).
- `Validate` sorts the report (`orderResults`); blank-node labels only break
  ties. `ResultMessages` is shared with the graph index — copy before sorting.
- `sh:datatype` lexical checks live in `xsd_lexical.go` (XSD 1.1 grammars, no
  whitespace trimming; "1."^^xsd:decimal is valid).
- Processor failures that don't change conforms go through `cfg.report` /
  `WithErrorHandler`: unsupported `sh:entailment` (`ErrUnsupportedEntailment`)
  and declared `sh:SPARQLFunction` with advanced features off.

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
