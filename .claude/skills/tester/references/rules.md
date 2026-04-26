# Rules R1–R12 (detailed)

The full rule set governing how Go tests are written and reviewed in this repository. R0 (the canonical `(SUT × behavior)` shape) lives inline in `SKILL.md` because it sets up everything else; R1–R12 are constraints on that shape and are loaded here on demand.

When `SKILL.md` cites a rule by number, this file is the source of truth for the wording, examples, and the *why*. Every rule has a one-liner in `SKILL.md`'s rules table — open this file to see the full text.

## R1: Use testify
- Don't use bare `t.Errorf` / `t.Fatalf` for assertions
- Use `assert.Equal` for equality
- Use `require` for setup-failure checks (`New*` constructors, etc.) where continuing is meaningless
- Use `assert` for the main test body checks

**Why**: Unifies diff display on failure and continue-vs-abort behavior.

## R2: SUT / got / want pattern
- **SUT** = the object/function under test (per R0)
- **got** = the value returned by *invoking* the behavior on the SUT — `sut.Method(args)` or `sut(args)`. Field access on the return value (`sut.Method(args).Field`) is fine *as long as* the method itself has logic worth probing. Bare `sut.Field` (no method invocation) is not a test of the SUT.
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

## R3: One SUT call → one got → one assert.Equal
- One `assert.Equal` per test case, as a rule
- If multiple verification angles are needed, split into separate cases or separate test functions
- `require` calls for setup are not counted as assertions (they are guards that abort if violated)

**Why**: One pass/fail signal per test case. When something breaks, the failure points to a single behavior.

## R4: Explicit AAA comments
- Each test case carries `// Arrange`, `// Act`, `// Assert` markers
- Shared setup outside the loop is labeled `// Arrange (shared)`

**Why**: Makes the structural intent obvious. Reinforces TDD habits.

## R5: Triangulation (table-driven)
- At least two positive cases per behavior
- Same behavior verified across different inputs is what locks down the spec
- Use table-driven form: `tests := []struct{ name string; ...; want T; wantErr error }{...}`

**Why**: A single case can be passed by an implementation that hard-codes the magic values. Two+ cases force generalization.

## R6: Payload struct defined on the production side
- Don't define a "for-test-only comparison struct" in the test file
- Use the production code's struct directly to build `want`
- If a comparable shape is needed, add a DTO struct to the production code (consult `developer`) and have the SUT return it

**Why**: A test-only struct itself needs verification, doubling the surface area.

## R7: No getters; public fields
- Strip simple getters like `obj.GetX()` from production code (consult `developer`)
- Make struct fields public and access them directly in tests (`obj.X`)
- Derived computations (e.g., `AtEnd()` whose logic is non-trivial) are fine; pure getters are not

**Why**: A getter is equivalent to a public field but adds boilerplate. The DTO style depends on direct field access.

## R8: wantErr error as type witness
- Error cases also live in the table; carry a `wantErr error` field
- A typed witness like `wantErr: &ParseError{}` plus `assert.IsType(t, tt.wantErr, got)` verifies the type
- If the error message is readable/deterministic, compare the value with `assert.Equal`

**Why**: Typed error checking confirms the failure took the right path.

## R9: Test helpers stay trivially correct
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

## R10: Test independence
- Tests don't depend on execution order
- No shared global state (use `t.Cleanup` for reliable teardown)
- Must be safe to run in parallel

## R11: No test-only production APIs
- Adding a field/method to `package zetasql` purely to make testing easier is **NOT allowed**
- If test access is needed, handle it via a test-side helper (in the `*_test.go` file)

**Why**: Production code shouldn't carry weight that only tests use. Consult `developer` if a real production-side change would clean things up instead.

## R12: Test behavior, not internal state or construction

A test must assert on something **observable from outside the SUT** that the SUT is contractually responsible for producing. Tests that only assert on internal state with no behavioral consequence — or on the result of a constructor that merely assigns fields — are not earning their keep.

**Reference**: Meszaros, *xUnit Test Patterns*, specifically:
- *State Verification* vs *Behavior Verification* — both are valid, but each requires the asserted property to be **observable** (a return value, an effect on a collaborator, a serialized output). Asserting on a private/public field that the SUT just stored verbatim is neither.
- *Sensitive Equality* (smell) — an assertion that compares more detail than the test cares about. Includes "comparing the entire internal state of the SUT" when only a subset is part of the contract.
- *Fragile Test* (smell, caused by *Overspecified Software*) — a test that breaks on harmless refactors because it locked in an implementation detail.
- *Goals of Test Automation* — tests must reduce risk and survive refactors. A test that breaks on every internal rename without flagging a real regression has negative value.

**What is *not* worth testing on its own**:

| Pattern | Why it's not behavior |
|---|---|
| `TestNewX` that asserts `&X{Field: v}` was set up | tests Go's struct-literal assignment, not the SUT |
| `TestSetX` / `TestEnableX` for a method whose body is `o.Field = v` (or `m[k] = true`) | tests Go's `=` / map insertion |
| A test that the package's internal lookup table contains entry "foo" | the higher-level methods that *use* the entry already exercise it; missing entries make those tests fail |
| `assert.Equal(sql, stmt.SQL)` when `ParseStatement` does `&Statement{SQL: sql, Root: root}` | pure passthrough — no transformation, no validation, no logic |

**What *is* behavior worth testing** (each has a logic surface that can break independently of "Go assigned a value"):

- A method whose result depends on input transformation, filtering, validation, or aggregation
- An invariant maintained across multiple fields (e.g., `SetSupportsAllStatementKinds` must clear `StatementKinds`)
- ToProto / FromProto serialization (boundary cases: nil, empty, deeply nested)
- Validation that returns an error for malformed input
- Method side effects on collaborators (analyzer + options + catalog → analysis result)

**Heuristic** (the "given/when/then" test):

Phrase the test as "given <input>, when <SUT call>, then <observable result>". If `<observable result>` reduces to "the field I just set has the value I set" or "the lookup table I built has the entry I added", the test is verifying Go itself — delete it. The behavior, if any, is exercised by the next-level test that *uses* the field/entry.

**Why**: Without this filter, the test suite grows to mirror the implementation rather than the contract. Every refactor (renaming a field, swapping `Features` from `map` to `[]LanguageFeature`, dropping a redundant export check) breaks tests that aren't flagging real regressions, which conditions everyone to either skip the failing tests or skip the refactor. Both outcomes are worse than not having the test.
