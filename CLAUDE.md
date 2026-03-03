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
