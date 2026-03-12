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

### store/sparqlstore (details)
- `Server` is an httptest-based SPARQL endpoint for integration testing
- Files: `doc.go`, `store.go`, `http.go`, `server.go`, `register.go`
- Server queries `ds.Default` only; named graphs not queryable on test server
- Test coverage: 99.7% (71 tests)
