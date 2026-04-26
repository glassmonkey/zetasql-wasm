# Fixture management

How to choose and lay out test fixtures in zetasql-wasm.

`SKILL.md` shows the rule and one inline example. This file expands the *why*, the **forbidden** patterns, and the migration cookbook.

## The rule

> **Each `t.Run` body owns its entire Arrange.** No mutable state lives at the test function scope.

Concretely:

- `ctx := t.Context()` is obtained inside each case, not shared at function level.
- Fixture instances (`*Analyzer`, `*Parser`, `*FilterOptions`, `*SimpleCatalog`, etc.) are constructed inside each case via Setup functions like `newTestAnalyzer(t)`.
- Test parameter tables (`tests := []struct{...}{...}`) stay at function level — they are read-only data, not Arrange.

## Forbidden: `// Arrange (shared)` block

```go
// FORBIDDEN — mutable shared state
func TestFoo(t *testing.T) {
    // Arrange (shared)
    a := newTestAnalyzer(t)            // ← shared *Analyzer mutated by cases
    ctx := context.Background()         // ← shared context, no per-test cancellation

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            out, err := a.AnalyzeStatement(ctx, tt.sql, tt.cat, opts)
            // ...
        })
    }
}
```

The `// Arrange (shared)` block was a previous convention; it is now banned because:

1. **Test interference**: even when an instance "looks read-only", helper methods can mutate hidden state (caches, counters, registered handlers). The check "can this be safely shared?" is high cognitive load and easy to get wrong silently — exactly the conditions for *Erratic Test* (Resource Leakage / Test Run War / Interacting Tests).
2. **Parallelism unsafety**: `t.Parallel()` becomes unsafe whenever any case writes to a shared instance.
3. **Cancellation semantics**: `context.Background()` shared at function scope outlives every case. `t.Context()` is canceled when its case ends, which propagates correctly to goroutines spawned by the test.

The discipline trade-off — paying construction cost N times instead of once — is accepted as the price of test independence (R10).

## Required: per-case Arrange

```go
// REQUIRED — each case owns its Arrange
func TestFoo(t *testing.T) {
    tests := []struct {
        name string
        sql  string
        want string
    }{
        {name: "...", sql: "...", want: "..."},
        {name: "...", sql: "...", want: "..."},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            ctx := t.Context()
            a := newTestAnalyzer(t)
            cat := newUsersCatalog()

            // Act
            got, err := a.AnalyzeStatement(ctx, tt.sql, cat, &AnalyzerOptions{})
            require.NoError(t, err)

            // Assert
            assert.Equal(t, tt.want, got.Statement.String())
        })
    }
}
```

## Setup functions (Delegated Setup / Creation Method)

When fixture construction is verbose or repeated, extract a Setup function. The Setup function returns a fresh instance every call; it is *not* a singleton.

```go
// Setup function — returns a fresh *Analyzer per call,
// registers cleanup with the testing.T.
func newTestAnalyzer(t *testing.T) *Analyzer {
    t.Helper()
    a, err := NewAnalyzer(t.Context())
    require.NoError(t, err)
    t.Cleanup(func() { a.Close(t.Context()) })
    return a
}

// Creation Method — returns a fresh *SimpleCatalog per call,
// hiding the table-construction boilerplate.
func newUsersCatalog() *catalog.SimpleCatalog {
    cat := catalog.NewSimpleCatalog("test")
    cat.AddZetaSQLBuiltinFunctions(nil)
    cat.Tables = append(cat.Tables, catalog.NewSimpleTable("users",
        catalog.NewSimpleColumn("users", "id", types.Int64Type()),
        catalog.NewSimpleColumn("users", "name", types.StringType()),
    ))
    return cat
}
```

**When to extract**: when the same construction sequence appears in three or more cases (or two test files). Below that threshold, an inline construction is fine and arguably easier to read.

**Helper discipline**: Setup functions are still subject to R9. They must not contain branches, switches with fallback returns, or numeric conversions. If you find yourself adding `if cfg.X { ... }` inside a Setup function, take that flag as a parameter and have the test choose, or split into two Setup functions.

## Things that are not "fixture"

These can stay at function scope without violating the rule, because they are not mutable state owned by the SUT:

- The test parameter table (`tests := []struct{...}{...}`) — read-only data
- Compile-time constants and pointer-to-pseudo-constants (e.g., `featureID := generated.LanguageFeature_FEATURE_TABLESAMPLE`)
- Test-local types declared via `type ...` (used to express the table's shape)

If you label any of these with `// Arrange (shared)`, remove the comment — it implies mutability that isn't there.

## Migration recipe (for legacy tests)

When fixing a test file written under the old convention:

1. Move every `a := newTestAnalyzer(t)` (or similar) from the function body into each `t.Run` body.
2. Replace `ctx := context.Background()` with `ctx := t.Context()` and move it into each `t.Run` body.
3. Delete every `// Arrange (shared)` comment.
4. Verify `go test ./...` still passes. The test count should be unchanged; wall-clock time may increase if WASM construction dominates.

## Why not optimize away the per-case cost?

Caching with `sync.Once`, package-level singletons, or `TestMain` are all tempting when WASM construction is ~100ms × N cases. They are all rejected:

- **`sync.Once` cache**: still a shared mutable instance, just hidden. Same interference risk, harder to debug.
- **`TestMain` setup**: enforces a singleton across the entire package. One test corrupting it poisons every later test.
- **Package-level globals**: same problem, and now visible to other packages too.

If wall-clock time becomes a real problem, we will revisit at the WASM-bridge level (e.g., a per-case `sync.Pool` of pre-warmed analyzer instances, with documented invariants), not by softening this rule.
