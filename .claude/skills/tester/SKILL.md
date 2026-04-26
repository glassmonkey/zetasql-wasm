---
name: tester
description: >
  Authors and reviews Go test code in the zetasql-wasm repository against
  the project's test conventions: SUT/got/want pattern, testify as the
  assertion implementation, AAA structure, table-driven triangulation,
  payload structs defined on the production side, no getters in tests,
  wantErr type witness, minimal helpers, behavior over internal state
  (per xUnit Test Patterns). Includes a code-level red-flag table and a
  mandatory pre-completion sniff scan so test smells (Mystery Guest,
  Buggy Tests, Sensitive Equality, etc.) get caught proactively on the
  first pass instead of after user pushback. Use this skill whenever
  the user says "write a test", "add a test for X", "review my tests",
  "look at this test code", "do my tests follow the conventions?",
  "improve these tests", "refactor these tests", or opens a *_test.go
  file with intent to author or improve it — and apply the sniff scan
  to your own test work even if no review was requested. Production-code
  design concerns belong to the `developer` skill; the TDD cycle belongs
  to the `tdd` skill.
---

# Tester

Authors and reviews Go tests in zetasql-wasm. Tests follow a fixed shape (SUT/got/want, testify, AAA, table-driven, full-struct want, type-witness errors). Helpers stay minimal so they don't themselves need tests.

This skill covers two modes:

- **Author mode**: writing a new test from scratch (typically called from the `tdd` cycle). Skeleton in this file; full procedure in [`references/modes.md`](references/modes.md).
- **Review mode**: reading an existing test against the rules and producing a structured report with concrete fixes. Skeleton in this file; full procedure in [`references/modes.md`](references/modes.md).

The same R0–R12 rule set governs both. R0 is inline below (it sets the canonical shape that every other rule constrains). R1–R12 details are in [`references/rules.md`](references/rules.md). Fixture construction policy (per-case Arrange, banned shared state) is in [`references/fixture-management.md`](references/fixture-management.md). When a review names an anti-pattern (AP1–AP6), the code examples and rationale live in [`references/anti-patterns.md`](references/anti-patterns.md). The xUnit Test Patterns vocabulary (Smell / Pattern / Refactoring catalog from Meszaros) lives in [`references/xunit-patterns.md`](references/xunit-patterns.md) — open it when you need a definition or want to verify you're using a smell name precisely.

## Underlying principle: testify is the test implementation

testify (`assert.Equal`, `require.NoError`, `assert.IsType`) **is** the assertion implementation. Tests are not composed from `if got != want { t.Errorf(...) }` primitives. The test author reaches for the testify API directly.

`cmp.Diff` is reserved for cases testify can't handle directly — namely proto comparisons under `protocmp.Transform()`. Using `cmp.Diff` against a plain Go struct is convention drift.

A new test starts at `assert.Equal(t, want, got)` and works backward to define `got` and `want`. It does not start at `if/t.Errorf` and work forward.

The other principle behind these rules — **separation of behavior and data** (DTOs are data, no getters, public fields) — lives in the `developer` skill. Several rules below (R6, R7, R11) are the test-side reflection of that production-side principle.

## xUnit Test Patterns vocabulary (Meszaros)

Most rules below are concrete instances of named patterns and smells from Gerard Meszaros, *xUnit Test Patterns: Refactoring Test Code* (2007). The mapping below is the shared vocabulary for diagnosing test problems on this project.

| Rule | xUnit pattern / smell | What the term means |
|---|---|---|
| R1 (testify) | Custom Assertion | The assertion library *is* the assertion implementation; tests don't reinvent `if/t.Errorf`. |
| R2 (SUT/got/want) | Tests as Documentation | Each test reads as a sentence; arrangement makes the contract obvious. |
| R3 (one assert per case) | Assertion Roulette · Eager Test | Multiple unrelated assertions in one case make the failure ambiguous and slow to localize. |
| R4 (AAA comments) | Four-Phase Test (Setup / Exercise / Verify / Teardown) | The four phases are the canonical structure; AAA is the conventional Go labelling of the first three. |
| R5 (triangulation) | Hard-Coded Test Data | A single positive case can be passed by an implementation that hard-codes the magic value; multiple inputs force the real generalization. |
| R6 (production-side payload) | Production Logic in Test | A test-only struct duplicates the production schema, then drifts; the test ends up describing what the test *thinks* the SUT does, not what it does. |
| R7 (no getters) | Test Method Acne | Trivial accessor tests proliferate without adding signal — every getter brings its own one-line "assertion". |
| R8 (typed wantErr) | Specific Assertion | Asserting on the error *type* (or value) verifies the failure mode, not just that "something failed." |
| R9 (helpers stay trivially correct) | Test Logic in Test (Conditional Test Logic) · Untested Test Code | Logic inside helpers becomes second-tier production code — untested, and silent when it goes wrong. |
| R10 (test independence) | Mystery Guest · Erratic Test (Test Run War, Resource Optimism, Resource Leakage) | A test that depends on hidden external state (file system, env, run order, leaked handles) flakes for reasons unrelated to the SUT. |
| R11 (no test-only production APIs) | Test Logic in Production · Test Hook | Production code that exists only for testing carries weight no caller benefits from. |
| R12 (behavior, not state) | Sensitive Equality · Fragile Test (Overspecified Software) · State vs Behavior Verification | Asserting on internal storage that the SUT holds verbatim, or on construction that has no logic, locks the test to the implementation rather than the contract. |

**Test Doubles taxonomy** (Chapter 11): when this project introduces a test double, name it precisely — *Dummy* (passed but not used), *Stub* (canned answers), *Spy* (records calls), *Mock* (verifies expectations), *Fake* (working but not production-grade implementation). Most tests here run the real WASM module rather than a double, so the taxonomy rarely comes up — but when it does, the imprecise word "mock" should not stand in for any of the five.

**Goals of Test Automation** (Chapter 3): a test must (a) help us understand the SUT, (b) reduce risk of defects, (c) survive refactoring, (d) be easy to write/maintain, (e) run fast enough to be run often. Every rule above serves one or more of these goals; if a proposed test or convention serves none, push back on it.

**Other smells worth naming when reviewing** (full definitions in `references/xunit-patterns.md`):

- *Obscure Test* (with sub-smells: Eager Test, General Fixture, Irrelevant Information, Hard-Coded Test Data, Mystery Guest, Verbose Test) — the reader cannot follow the test at a glance.
- *Erratic Test* (Resource Leakage, Resource Optimism, Test Run War, Unrepeatable Test, Test Dependency, Interacting Tests, Non-Deterministic Test) — passes sometimes, fails sometimes; the suite loses signal value.
- *Fragile Test* (Behavior Sensitivity, Interface Sensitivity, Data Sensitivity, Context Sensitivity, Overspecified Software) — breaks on harmless refactors instead of real regressions.
- *Buggy Tests* — the test itself has a bug and hides production bugs (e.g., `assert.Equal(x, x)`).
- *Slow Test* — slow enough that the suite stops getting run frequently.
- *Test Code Duplication* — the same arrangement/assertion repeated across many tests; cure with Test Utility Method, Creation Method, Parameterized Test.

When pointing out a problem in review, name the smell. "This is a *Sensitive Equality* on the internal map" gives the author a vocabulary to fix it; "this test feels off" does not.

## R0: Every test is `(SUT × behavior)`

Every test in this repo follows the same canonical shape:

```go
sut := <test target>           // a value/function/instance to exercise
got := sut.<Method>(<args>)    // invoke the behavior under test
assert.Equal(t, want, got)
```

- **SUT** = a meaningful object, function, or value under test (a `*Parser`, an `*AnalyzerOptions`, the function `TypeFromKind`, etc.).
- **Behavior** = the *invocation* on the SUT — `sut.SomeMethod(args)` for a method, `sut(args)` for a function-as-SUT. The invocation is the probe; the test asserts on what the probe returns.

If a test does not have this shape, **something is off**. The two recurring degenerate forms are:

1. **No method is invoked** — `got := someStruct.SomeField` after a constructor call. Without a behavior probe, the test asserts only that Go assigned a value (R12 / *Sensitive Equality*).
2. **The "method" is a trivial accessor** — `got := sut.GetX()` where `GetX` returns a single field (R7 / *Test Method Acne*).

R1–R12 are constraints on this shape. Open `references/rules.md` for the full text; the table below is the in-context summary.

## Rules R1–R12 (one-liners)

| # | Name | One-liner |
|---|---|---|
| R1 | Use testify | `assert.*` / `require.*`, never bare `t.Errorf` |
| R2 | SUT / got / want | `got := sut.Method(args)`; `want` is a complete struct |
| R3 | One assert per case | One pass/fail signal per case (avoid Assertion Roulette) |
| R4 | Explicit AAA | `// Arrange / // Act / // Assert` markers |
| R5 | Triangulation | At least two positive cases per behavior |
| R6 | Production-side payload | No test-only payload structs |
| R7 | No getters | Public fields, direct access |
| R8 | wantErr type witness | `wantErr error` carrying a typed `*FooError{}` |
| R9 | Helpers trivially correct | No branching, no multi-accessor compounds, no fallback returns |
| R10 | Test independence | Order-independent, parallel-safe, `t.Cleanup` for teardown |
| R11 | No test-only production APIs | If only tests use a function, delete the function (or move to test code) |
| R12 | Behavior, not state | The asserted outcome must be observable from outside the SUT |

When citing a rule (e.g., "R12 violation"), open `references/rules.md` for the example code and full *why*.

## Code-level red flags (proactive scan)

The rules above are diagnostic — they tell you *what's wrong* once you suspect a problem. This section is the inverse: **concrete code patterns that should immediately raise suspicion**, mapped to the rule/smell to invoke.

The point is to catch issues on the first read, before any human pushback. Scan any test file (yours or someone else's) against this table. Each row is grep-able.

| Pattern visible in the test code | Suspect | First action |
|---|---|---|
| `got := sut` with no method invocation between Arrange and Assert | R0 / R12 / AP6 | The test isn't probing any behavior — likely a constructor-only test. Delete it, or invoke a real method as Act. |
| `got := sut.Field` after `sut := &X{...}` (no method called) | R12 / Sensitive Equality | Either the assertion goes through a method that has logic, or the test is verifying Go's `=`. Delete or refactor. |
| `assert.True(t, sut.Map[k])` directly after `sut.SetX(k)` (or equivalent) | AP5 / R12 | Verify through an observable path (e.g., `sut.toProto()`) — or, if the mutator has no logic, delete the test entirely. |
| `if tt.wantErr { assert.Error } else { assert.NoError }` inside a case loop | R8 / Conditional Test Logic | Replace `wantErr bool` with `wantErr error` (typed witness) and use `assert.IsType` / `assert.ErrorIs`. |
| Helper body contains `if`/`switch`/`for` with a fallback `return ""` | R9 / Test Logic in Test | Inline the helper into the test, or split each branch into its own one-line case. |
| Test name `TestNewXxx` whose body just compares `NewXxx(...)` to `&Xxx{...}` | AP6 / Buggy Tests candidate | Delete unless `NewXxx` has real defaults / validation / allocation. |
| Test asserts that a particular entry exists in a generated/internal lookup (e.g., `compiledModule.ExportedFunctions()["foo"]`) | AP5 | A higher-level behavior test already exercises that entry. Confirm and delete the lookup test. |
| Same Arrange block (more than ~5 lines) duplicated across 3+ tests | Test Code Duplication | Extract a Creation Method (`newXxxCatalog` etc.). The helper must stay R9-trivial. |
| `t.Errorf` / `t.Fatalf` used as the assertion (not as a guard) | R1 | Migrate to `assert.*` / `require.*`. |
| `cmp.Diff` used against a plain Go struct (no `protocmp.Transform()`) | R1 / convention drift | `assert.Equal` is sufficient and gives clearer failure output. |
| Test depends on file paths, env vars, current time, or test execution order | R10 / Mystery Guest / Context Sensitivity | Inline the data, stub the dependency, or use a Virtual Clock. |
| Test verifies a function that has zero production callers (only tests use it) | R11 | The function is dead code held alive by its test. Delete both unless it's about to gain a real caller. |
| `t.Run(tt.name, func(t *testing.T) { ... })` body has 2+ unrelated `assert.*` calls | R3 / Assertion Roulette | Split into separate cases or build a complete `want` struct. |
| Mutable fixture (`*Analyzer`, `*FilterOptions`, etc.) constructed at the test function scope and reused across `t.Run` cases — often labeled `// Arrange (shared)` | R10 / Fixture management / Erratic Test | Move the construction inside each `t.Run` body. Use `t.Context()` instead of a shared `context.Background()`. |

When in doubt about the smell name, look it up in `references/xunit-patterns.md`. Naming the smell precisely makes the fix obvious.

## Fixture management

**Each `t.Run` body owns its entire Arrange.** No mutable state lives at the test function scope.

```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // Arrange — per case
        ctx := t.Context()
        a := newTestAnalyzer(t)
        cat := newUsersCatalog()

        // Act
        got, err := a.AnalyzeStatement(ctx, tt.sql, cat, &AnalyzerOptions{})
        // ...
    })
}
```

- `ctx := t.Context()` (Go 1.24+) — never `context.Background()` at function scope. `t.Context()` is canceled when the case ends and propagates to goroutines correctly.
- Fixture instances (`*Analyzer`, `*Parser`, options structs, catalogs) are constructed inside each case via Setup functions like `newTestAnalyzer(t)`. The Setup function returns a fresh instance per call — never a cached singleton.
- The test parameter table (`tests := []struct{...}{...}`) stays at function scope because it is read-only data, not Arrange.

**`// Arrange (shared)` is forbidden.** Sharing a mutable fixture instance across `t.Run` cases violates R10 (test independence) — even when the instance "looks read-only", helper methods can mutate hidden state, which is the *Erratic Test* failure mode (Resource Leakage / Test Run War / Interacting Tests). The cost of constructing the fixture N times is the price of independence; it is not negotiable at the test level.

If WASM construction wall-clock becomes a real problem, we revisit at the bridge level (e.g., a `sync.Pool` of pre-warmed instances with documented invariants), not by relaxing this rule.

When to extract a Setup function: same construction sequence appears in 3+ cases or 2+ test files. Below that, inline construction is fine. See [`references/fixture-management.md`](references/fixture-management.md) for the full forbidden-vs-required examples and a migration recipe for legacy tests.

## Pre-completion sniff scan (mandatory)

Before declaring any test work "done" — whether you authored it (Author mode) or reviewed it (Review mode) — run this scan over the affected files. Skipping this step is how the rules end up enforced only after user pushback, and that's exactly what this skill exists to prevent.

The scan, in order:

1. **R0 shape check**. For every `func Test...` you touched, confirm the body has both `sut := <target>` and `sut.<Method>(args)` (or the function-as-SUT equivalent `got := <fn>(args)`). If a test only constructs and asserts without invoking any method/function, suspect R12 / AP6 — investigate before proceeding.

2. **Red-flag scan**. Walk the "Code-level red flags" table above against the diff. For each row that hits, take the prescribed first action (fix or delete). Do not defer to "follow-up" — the whole point is to handle these on the same pass.

3. **Production-side dead code**. If you deleted a test, search for other callers of the production function that test exercised. If the test was the only consumer, the production function is now dead code held alive by nothing — delete it (R11). This is how exported `TypeFromProto` and `NodeFromBytes` should have been caught the first time.

4. **Smell-naming pass**. For each issue found, name the Meszaros smell (Mystery Guest, Buggy Tests, Sensitive Equality, etc.) using `references/xunit-patterns.md`. If a fix is going into a commit, put the smell name in the commit message — future readers and skill iterations both benefit.

5. **Borderline cases**. If a test feels off but you cannot point to a specific rule/smell, surface it to the user with the candidate diagnosis ("might be Fragile Test because Y"). Do not silently leave it. Borderline calls are the user's decision; smells you can name are yours.

The bar: by the time you say "done", a reviewer reading the resulting test file should not need to point out any of the patterns above. If they do, it means this scan was skipped.

## Author mode (skeleton)

Use this when invoked from the `tdd` cycle (Step 2: red) or when the user asks "write a test for X". Full text in [`references/modes.md`](references/modes.md#author-mode-writing-a-new-test).

1. **A1 — Pick the SUT**: function / method receiver / constructor.
2. **A2 — Write a table-driven skeleton**: `tests := []struct{name, input, want, wantErr}{...}` at function scope (read-only data); each `t.Run` body owns its own Arrange (`ctx := t.Context()`, Setup-function calls), follows the R0 shape, and labels the AAA phases. No `// Arrange (shared)` block.
3. **A3 — `want` is a complete struct**: full population so field add/remove surfaces in the diff.
4. **A4 — Tree comparison uses the minimal helper**: per R9 — single-accessor cases, no branches.
5. **A5 — Reuse, don't reinvent**: `newTestAnalyzer`, `newUsersCatalog`, `findNode` already exist.
6. **A6 — Aligning with existing tests**: existing tests are a style reference, not an oracle. The rules win, the example loses.
7. **A7 — Run the pre-completion sniff scan**: before declaring done. Mandatory.

## Review mode (skeleton)

Use this when the user says "review my tests" or similar. Full text in [`references/modes.md`](references/modes.md#review-mode-reading-an-existing-test).

1. **Step 1 — Identify the target**: which file/directory? If unspecified, check `git status` / `git log` and confirm.
2. **Step 2 — Read every test file**: load completely, including shared helpers in the same package.
3. **Step 3 — Check rule by rule**: walk each function through R1–R12.
4. **Step 3b — Run the red-flag scan**: sweep the table above for surface patterns the rule pass can miss.
5. **Step 4 — Output the report**: structured findings with location / rule / before / after / why.
6. **Step 5 — Suggest priority**: R12 first (deletes whole tests), then design (R3), then structure (R6, R2), then format (R1, R4).

## Allowed exceptions

- **Constructing the full tree as `want` is impractical** → use R9's helper to compare via string
- **Error message is non-deterministic** → R8's `assert.IsType` only (skip value compare)
- **Proposing a new pattern** → discuss with the user before adopting

When applying an exception, leave a comment in the test explaining why.

## Anti-patterns (index)

The full code samples and rationale are in [`references/anti-patterns.md`](references/anti-patterns.md). One-line index:

- **AP1 — Multiple assertions in sequence** (violates R3)
- **AP2 — Test-side payload construction** (violates R2/R6)
- **AP3 — Elaborate format helper** (violates R9)
- **AP4 — Comparison via getter** (violates R7)
- **AP5 — Asserting on internal state with no behavioral consequence** (violates R12)
- **AP6 — Testing a constructor directly** (violates R12)

## How this skill interacts with others

- `tdd`: drives the cycle and calls into this skill at the red phase to write a test, and again at green/refactor time to verify the test still conforms.
- `developer`: handles the production-side fix when a test-side issue points at production code (e.g., a getter spotted under R7, a test-only field flagged under R11). When in doubt: tests are the consumer of the production design — if the test is awkward, the production design is probably awkward too.
