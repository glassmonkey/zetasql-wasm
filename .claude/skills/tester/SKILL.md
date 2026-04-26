---
name: tester
description: >
  Authors and reviews Go test code in the zetasql-wasm repository against
  the project's test conventions: SUT/got/want pattern, testify as the
  assertion implementation, AAA structure, table-driven triangulation,
  payload structs defined on the production side, no getters in tests,
  wantErr type witness, minimal helpers. Use this skill whenever the user
  says "write a test", "add a test for X", "review my tests", "look at this
  test code", "do my tests follow the conventions?", "improve these tests",
  "refactor these tests", or opens a *_test.go file with intent to author
  or improve it. Production-code design concerns belong to the `developer`
  skill; the TDD cycle belongs to the `tdd` skill.
---

# Tester

Authors and reviews Go tests in zetasql-wasm. Tests follow a fixed shape (SUT/got/want, testify, AAA, table-driven, full-struct want, type-witness errors). Helpers stay minimal so they don't themselves need tests.

This skill covers two modes:

- **Author mode**: writing a new test from scratch (typically called from the `tdd` cycle).
- **Review mode**: reading an existing test against the rules and producing a structured report with concrete fixes.

The same R1–R11 rule set governs both.

## Underlying principle: testify is the test implementation

testify (`assert.Equal`, `require.NoError`, `assert.IsType`) **is** the assertion implementation. Tests are not composed from `if got != want { t.Errorf(...) }` primitives. The test author reaches for the testify API directly.

`cmp.Diff` is reserved for cases testify can't handle directly — namely proto comparisons under `protocmp.Transform()`. Using `cmp.Diff` against a plain Go struct is convention drift.

A new test starts at `assert.Equal(t, want, got)` and works backward to define `got` and `want`. It does not start at `if/t.Errorf` and work forward.

The other principle behind these rules — **separation of behavior and data** (DTOs are data, no getters, public fields) — lives in the `developer` skill. Several rules below (R6, R7, R11) are the test-side reflection of that production-side principle.

## Rules (R1–R11)

### R1: Use testify
- Don't use bare `t.Errorf` / `t.Fatalf` for assertions
- Use `assert.Equal` for equality
- Use `require` for setup-failure checks (`New*` constructors, etc.) where continuing is meaningless
- Use `assert` for the main test body checks

**Why**: Unifies diff display on failure and continue-vs-abort behavior.

### R2: SUT / got / want pattern
- **SUT** = the object/function under test
- **got** = the direct return value of `sut.method(args)` (or a single field/single accessor on it)
- **want** = the expected value
- Building a fresh `payload` struct in the test by extracting multiple fields from the SUT's return is **NOT allowed**
- `want` must be a complete struct so field add/remove surfaces in the diff

**Bad**:
```go
type payload struct { Kind Kind; Name string }
got := payload{Kind: stmt.Kind(), Name: stmt.Name()}
```

**Good**:
```go
got := sut.Method(args)
want := &Statement{SQL: "SELECT 1"}
assert.Equal(t, want, got)
```

### R3: One SUT call → one got → one assert.Equal
- One `assert.Equal` per test case, as a rule
- If multiple verification angles are needed, split into separate cases or separate test functions
- `require` calls for setup are not counted as assertions (they are guards that abort if violated)

**Why**: One pass/fail signal per test case. When something breaks, the failure points to a single behavior.

### R4: Explicit AAA comments
- Each test case carries `// Arrange`, `// Act`, `// Assert` markers
- Shared setup outside the loop is labeled `// Arrange (shared)`

**Why**: Makes the structural intent obvious. Reinforces TDD habits.

### R5: Triangulation (table-driven)
- At least two positive cases per behavior
- Same behavior verified across different inputs is what locks down the spec
- Use table-driven form: `tests := []struct{ name string; ...; want T; wantErr error }{...}`

**Why**: A single case can be passed by an implementation that hard-codes the magic values. Two+ cases force generalization.

### R6: Payload struct defined on the production side
- Don't define a "for-test-only comparison struct" in the test file
- Use the production code's struct directly to build `want`
- If a comparable shape is needed, add a DTO struct to the production code (consult `developer`) and have the SUT return it

**Why**: A test-only struct itself needs verification, doubling the surface area.

### R7: No getters; public fields
- Strip simple getters like `obj.GetX()` from production code (consult `developer`)
- Make struct fields public and access them directly in tests (`obj.X`)
- Derived computations (e.g., `AtEnd()` whose logic is non-trivial) are fine; pure getters are not

**Why**: A getter is equivalent to a public field but adds boilerplate. The DTO style depends on direct field access.

### R8: wantErr error as type witness
- Error cases also live in the table; carry a `wantErr error` field
- A typed witness like `wantErr: &ParseError{}` plus `assert.IsType(t, tt.wantErr, got)` verifies the type
- If the error message is readable/deterministic, compare the value with `assert.Equal`

**Why**: Typed error checking confirms the failure took the right path.

### R9: Test helpers stay trivially correct
This rule applies to **every test helper**, not just tree-comparison helpers. The intent: a helper that contains logic worth testing has crossed the line and now requires its own tests-of-tests, which compounds maintenance and dilutes the value of the test suite.

**Discouraged patterns (each is a smell that invites silent bugs):**

| Pattern | Smell | Why it invites bugs |
|---|---|---|
| Branching (`if`/`else`, conditional return) | logic in helper | each branch is an untested path |
| Multi-accessor compound (`a.B().C() + a.D().E()`) | composition logic | wrong field combinations are silent |
| Numeric type conversions (`string(rune(int))`, `strconv.*`) | semantic conversion | linters cannot catch intent errors here |
| "Default"/fallback returns (`return "[?]"`, `return ""`) | lossy fallback | masks real failures behind an opaque value |
| Comparing values inside the helper | hidden assertion | the assertion's failure is invisible |

**Tree-comparison helpers specifically** (e.g., flattening AST to `<Kind> <single-accessor>\n`):
- Each `case` in the type switch is a **single accessor call** — no compound formatting, no multi-field joins
- The walker itself uses only the public Node interface (`Kind`, `NumChildren`, `Child`)

**Good**:
```go
case *resolved_ast.TableScanNode:
    return " " + v.Table().GetName()  // single accessor
```

**Bad** (the canonical "you needed tests for this helper" example):
```go
func formatNode(n resolved_ast.Node) string {
    switch v := n.(type) {
    case *resolved_ast.OutputColumnNode:
        col := v.Column()
        if col.GetTableName() != "" {  // ← branch
            return "[" + col.GetTableName() + "." + col.GetName() + " as " + v.Name() + "]"
            // ← multi-accessor compound; if the join order is wrong the test still passes silently
        }
        return "[" + col.GetName() + " as " + v.Name() + "]"
    case *resolved_ast.LiteralNode:
        val := v.Value().GetValue()
        if val.GetInt64Value() != 0 {  // ← branch + value-check inside helper
            return "[int64:" + string(rune(val.GetInt64Value())) + "]"
            // ← `string(rune(int64))` is a semantic-conversion bug (linters do NOT flag it
            //    because the explicit rune() suppresses go vet's stringintconv warning).
            //    100 becomes "d" instead of "100". This is exactly the class of bug R9 prevents.
        }
        return "[unknown]"  // ← lossy fallback
    }
    return "[?]"  // ← another lossy fallback
}
```

**The review heuristic**: if removing or rewriting the helper would force someone to verify it still works (i.e., it begs for unit tests), the helper is too complex — flag it under R9 and recommend collapsing each case to a single accessor.

### R10: Test independence
- Tests don't depend on execution order
- No shared global state (use `t.Cleanup` for reliable teardown)
- Must be safe to run in parallel

### R11: No test-only production APIs
- Adding a field/method to `package zetasql` purely to make testing easier is **NOT allowed**
- If test access is needed, handle it via a test-side helper (in the `*_test.go` file)

**Why**: Production code shouldn't carry weight that only tests use. Consult `developer` if a real production-side change would clean things up instead.

## Author mode (writing a new test)

Use this when invoked from the `tdd` cycle (Step 2: red) or when the user asks "write a test for X".

### A1: Pick the SUT
- A function → the function itself
- A method → the receiver instance
- A constructor (`New*`) → frequently the SUT

### A2: Write a table-driven skeleton

```go
func TestSUT_Behavior(t *testing.T) {
    // Arrange (shared)
    // ... shared setup, if any ...

    tests := []struct {
        name    string
        // input fields
        input   InputType
        // expected
        want    OutputType
        wantErr error  // type witness for error cases
    }{
        {name: "positive case 1", input: ..., want: ...},
        {name: "positive case 2", input: ..., want: ...},  // triangulation
        {name: "error: invalid input", input: ..., wantErr: &SomeError{}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            sut := ... // subject under test

            // Act
            got, err := sut.Method(tt.input)

            // Assert
            if tt.wantErr != nil {
                assert.IsType(t, tt.wantErr, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### A3: `want` is a complete struct
For struct returns, build a fully populated `want` so any field add/remove appears in the diff. For scalar returns, the scalar type alone is fine.

### A4: Tree comparison uses the minimal helper
For AST-shaped output, use a helper that flattens to `<Kind> <single-accessor>\n`. Each switch `case` is a single accessor call. No compound formatting (R9).

### A5: Reuse, don't reinvent
Helpers like `newTestAnalyzer`, `newUsersCatalog`, `findNode`, `resolvedDebugString` already exist. Use them. If you need a helper that doesn't exist, prefer extending an existing one (keeping each case a single accessor) over creating a parallel one.

### A6: Aligning with existing tests

Existing tests are a **style reference**, not an oracle of correctness. Treat them as cues for the local idiom — but keep R1–R11 as the standard of truth. If you see a violation in existing code, do not propagate it.

What to copy:
- Reuse the helpers above
- Imports from `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require`
- Prefer adding to a related existing test file (e.g., `parser_test.go`, `analyzer_test.go`) over creating a new file

What **not** to copy:
- A leftover getter or setter — even if it's there, the new code uses public fields directly (R7)
- A multi-assertion test — even if a neighbor has three `assert.Equal` calls, your new test still has one per case (R3)
- A custom payload struct in the test — even if you find one nearby, don't add another (R2, R6); flag the existing one for a separate review pass

When in doubt: the rules win, the example loses.

## Review mode (reading an existing test)

Use this when the user says "review my tests" or similar.

### Step 1: Identify the target
Ask the user which file/directory to review. If unspecified:
- Check `git status` / `git log` for recently modified `*_test.go` files
- Confirm with the user before proceeding

### Step 2: Read every test file
Use the `Read` tool to load the target files completely. Include shared helper files within the same package.

### Step 3: Check rule by rule
Walk each test function through R1–R11. Record every violation.

### Step 4: Output the report

```
## Test review: <file>

### Finding 1: R<n> <rule name>
- **Location**: foo_test.go:42
- **Issue**: <what specifically is wrong>
- **Fix**:
```go
// before
<violating code>

// after
<corrected code>
```
- **Why this rule matters**: <one-line rationale>

### Finding 2: ...
```

If multiple files are reviewed, separate by file. If a file has no violations, state "✓ Conforms to conventions."

### Step 5: Suggest priority

When violations are numerous, indicate fix order:
1. **R3 (one assert per got)**: design-level, fix first
2. **R6 (test-side payload struct)**: structural
3. **R2 (SUT/got/want)**: clarifies structure
4. Format-level (R1, R4) last

### Review etiquette

- **Avoid heavy-handed MUSTs**: communicate the *why*, not blind directives
- **Concrete fixes only**: always show the corrected code; no vague "please fix"
- **Acknowledge trade-offs**: e.g., when constructing a `want` tree is impractical, R9's minimal helper is the legitimate substitute
- **Match existing patterns**: if neighboring tests in the same repo follow a particular style, anchor to it
- **Production-side smells**: when a test-side issue (e.g., R7 getter use, R11 test-only API) points at a production-code problem, refer the user to `developer` for the production fix

## Allowed exceptions

- **Constructing the full tree as `want` is impractical** → use R9's helper to compare via string
- **Error message is non-deterministic** → R8's `assert.IsType` only (skip value compare)
- **Proposing a new pattern** → discuss with the user before adopting

When applying an exception, leave a comment in the test explaining why.

## Anti-pattern catalog

### AP1: Multiple assertions in sequence
```go
// bad
assert.Equal(t, 1, len(cols))
assert.Equal(t, "id", cols[0].Name)
assert.Equal(t, "users", cols[0].TableName)
```
→ Violates R3. Either split into cases, or build a complete `want` struct comparison.

### AP2: Test-side payload construction
```go
// bad
type payload struct { Name string; Kind Kind }
got := payload{Name: stmt.Name(), Kind: stmt.Kind()}
```
→ Violates R2/R6. Use the production struct directly, or change the SUT to return a comparable shape (consult `developer`).

### AP3: Elaborate format helper
```go
// bad
func summary(n Node) string {
    if n.HasFoo() && n.Foo().Bar() != "" {
        return fmt.Sprintf("[%s/%s]", n.Foo().Bar(), n.Baz())
    }
    return n.Default()
}
```
→ Violates R9. Helpers must not carry logic worth testing. Branches, multi-accessor compounds, numeric conversions, and fallback returns each invite silent bugs — and linters (`go vet`, `staticcheck`, `gocritic`) cannot catch the semantic-intent bugs in these helpers, so the review must.

### AP4: Comparison via getter
```go
// bad
assert.Equal(t, sql, stmt.SQL())
```
→ Violates R7. Use direct field access (`stmt.SQL`) and remove the getter (a `developer` concern).

## How this skill interacts with others

- `tdd`: drives the cycle and calls into this skill at the red phase to write a test, and again at green/refactor time to verify the test still conforms.
- `developer`: handles the production-side fix when a test-side issue points at production code (e.g., a getter spotted under R7, a test-only field flagged under R11). When in doubt: tests are the consumer of the production design — if the test is awkward, the production design is probably awkward too.
