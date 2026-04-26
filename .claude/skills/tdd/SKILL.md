---
name: tdd
description: >
  Drives the red → green → refactor development cycle in the zetasql-wasm
  Go repository. Confirms specs, reads the current code, writes a failing
  test first, makes it pass with minimal implementation, then refactors.
  Catches process-level mistakes (skipped red phase, over-implementation,
  spec drift) before they ship. Use whenever the user says "implement with
  TDD", "test first", "red green refactor", "write tests then implement",
  "add a feature test-first", "write a failing test first", or otherwise
  asks for TDD-style development on this repository. The test-writing
  details belong to the `tester` skill; the production-code design details
  belong to the `developer` skill — this skill orchestrates the cycle and
  delegates.
---

# TDD

Drives Go feature development in zetasql-wasm using **red → green → refactor**. Tests are written first, the simplest implementation makes them pass, then refactors clean things up. This skill owns the *cycle*; test-writing details live in `tester` and production-code design details live in `developer`.

## Why TDD here

- **red**: confirming the test actually fails proves the test verifies *something* (prevents false positives)
- **green**: minimal implementation prevents over-engineering — code beyond what tests demand isn't built
- **refactor**: the safety net of passing tests lets refactoring happen without regression risk

When a feature request comes in, **never jump straight to implementation**. Always start with a test. Only after `go test` confirms the test fails (or fails to compile in the right way) does implementation begin.

## Cycle

### Step 0: Read the current code for grounding (mandatory)

Before clarifying the spec, before writing the test, before any planning — **read the actual code as it exists right now**. This step is not optional. Without it, the dominant failure mode is to write tests against an imagined API (private fields, getters, setters) that doesn't actually exist.

**Important framing**: this step is for **grounding the test against reality**, not for blindly imitating the existing code. The existing code might itself violate the conventions enforced by `tester` and `developer` (e.g., a leftover getter, a getter chain inside a helper). Reading is for accuracy. Convention adherence is decided by those skills' rule sets, not by whatever happens to be in the repo right now.

#### What to read, in order

1. **The production file you will modify.** Use the `Read` tool. Capture only the *facts*:
   - What is the exact name of the struct/type? Field names, exported or not?
   - What constructors / methods / setters / getters exist? With exact signatures?
   - What's used by callers (search briefly with `Grep` if non-obvious)?

   Do **not** decide style here — only inventory the API surface. If the file has an exported getter, that doesn't mean you should use one in your new code; it just means a getter currently exists.

2. **At least one existing test file in the same package.** Pull style cues from it (testify, AAA, table-driven, `wantErr` shape). If the existing test is itself non-conforming, **do not propagate the violation**. Note it as a separate finding for a follow-up review with `tester`; don't silently spread bad style to the new test.

3. **Reusable helpers** (e.g. `newTestAnalyzer`, `newUsersCatalog`, `findNode`, `resolvedDebugString`). Reuse them — don't invent parallel helpers.

### Step 1: Clarify the spec

Until these are concrete, do not write code:

- **API to add**: which package, which function/method/type?
- **Inputs and outputs**: argument types, return types, error types
- **Positive cases**: at least two concrete (input, expected output) pairs (triangulation)
- **Error cases**: which inputs should error, and which error type?

If the spec is fuzzy, confirm with the user before proceeding.

### Step 2: Test first (red)

Hand the test-writing details to the `tester` skill: SUT/got/want shape, table-driven layout with triangulation, AAA comments, testify, `wantErr` as a type witness, tree comparison via the minimal helper. This skill's job here is to make sure a test is written *first* and is *seen failing*.

Run `go test ./...`. The test should either fail to compile (missing type/function — types align, logic missing) or fail an assertion. That's the expected red state. Capture a snippet of the failure output for the report.

### Step 3: Make it green

#### 3.1 Naive is fine

Implement only what the tests demand. With triangulation in place, the general implementation tends to fall out naturally.

#### 3.2 Defer to `developer` for design choices

If the green-phase implementation calls for a non-trivial design choice (new type, new package boundary, new method shape, layering question), pause and consult `developer`. This skill's "minimal implementation" rule still holds — but "minimal" is judged through the `developer` principles (separation of behavior/data, deep modules, layering, etc.), not against an arbitrary baseline. Public fields over getters, errors defined out where possible, no speculative parameters.

#### 3.3 Confirm the tests pass

`go test ./...` must be all green.

### Step 4: Refactor

With tests green, clean up safely:
- Remove duplication
- Improve names
- Adjust abstraction levels (consult `developer` for principles)

After every refactor edit, rerun `go test ./...` to catch regressions. **The test spec (`want`) must not change** — only the implementation.

## Failure-mode catalog

Bad TDD outcomes break into five categories. Each entry has: what goes wrong, why it's bad, how to detect it before commit. Categories C and B link out to `tester` rules; D and E are process concerns this skill owns.

### Category A — Reality-mismatch (test refers to APIs that don't exist)

Step 0 catches these by forcing you to read the file before writing.

| # | Mode | What it looks like | Why bad | Detect by |
|---|---|---|---|---|
| F1 | Phantom getter | `loc.BytePosition()` when field is public `BytePosition` | imagined API; build fails | open production file, search for the symbol |
| F2 | Phantom setter | `opts.SetLanguageOptions(lang)` when setters were removed | imagined API; build fails | search for `SetX` in the file before using |
| F2b | Phantom field name | `opts.language` when current field is `Language` | private/public mismatch | look at the struct definition once |

### Category B — Convention drift (test conflicts with `tester` rules)

The test compiles but doesn't match the project's standards. **Existing code is not a license to ignore the rules** — if the existing code itself violates a rule, surface it as a follow-up, don't propagate it. See `tester` skill for the rule definitions.

| # | Mode | What it looks like | Detect by | tester rule |
|---|---|---|---|---|
| F3 | Private field + getter on new struct | `field foo string; func Foo() string { return f.foo }` | grep new code for single-return getters | R7 |
| F4 | `t.Errorf` instead of testify | `if got != want { t.Errorf(...) }` | grep new test for `t.Errorf` | R1 |
| F7 | `cmp.Diff` for plain Go struct | `cmp.Diff` used where `assert.Equal` would do | grep new test for `cmp.Diff` without `protocmp` | R1 |
| F8 | Missing AAA markers | code blocks blend Arrange/Act/Assert | grep new test for `// Arrange` / `// Act` / `// Assert` | R4 |
| F9 | Single-case table | `tests := []struct{...}{ {...} }` with one entry | count rows in the table | R5 |

### Category C — Test structure flaws (the test is internally misshapen)

| # | Mode | What it looks like | Detect by | tester rule |
|---|---|---|---|---|
| F5 | Multiple assertions per case | 3 `assert.Equal` calls under one `t.Run` | count `assert.*` calls per case | R3 |
| F6 | Test-side payload struct | `type payload struct{...}` defined inside test file | grep new test for `type .* struct` | R2, R6 |
| F6b | Multiple `cmp.Diff` per case | two diffs in one `t.Run` body | count `cmp.Diff` per case | R3 |

### Category D — Process flaws (TDD cycle was skipped or short-circuited)

These mean you shipped a "TDD" change without the safety the cycle is supposed to provide.

| # | Mode | What it looks like | Why bad | Detect by |
|---|---|---|---|---|
| F10 | Skipped red phase | implementation written without ever seeing `go test` fail | the test could be passing for the wrong reason (false positive) | confirm a `go test` output snippet showing the red state, before green |
| F11 | Adjusted want to fit impl | rewrote `want` after seeing actual output rather than fixing impl | the test is no longer specifying the spec; it's documenting whatever happened | git-diff the test: did `want` literals change after the impl was written? |
| F12 | Skipped refactor phase | left obvious duplication or unclear naming because tests pass | green-phase code accumulates rot | scan the diff for duplication, magic constants, unclear names |

### Category E — Spec flaws

| # | Mode | What it looks like | Why bad | Detect by |
|---|---|---|---|---|
| F13 | Implicit spec | started writing without confirming inputs/outputs/error types with the user | what gets built may not match the actual ask | re-read Step 1; if you can't list 2+ positive cases and the error types, the spec isn't ready |
| F14 | Over-implementation | added validation/logging/caching the test didn't require | code carries weight tests don't justify; hides bugs in untested paths | diff the impl against what the test exercises — anything not covered by an assertion is suspect |
| F15 | Implementation-detail leak | test depends on internal field name/order rather than observable behavior | tests break on harmless refactors | check whether want is described in terms of public observable state |

### Using this catalog during the cycle

- **Before red**: F13 (spec) and F1, F2, F2b (reality)
- **After red**: F10 (saw it fail?)
- **After green**: F4–F9 (conventions, via `tester`), F5/F6 (structure, via `tester`), F11 (didn't move the goalposts?), F14 (only what tests asked for?)
- **Before declaring done**: F3, F7, F8, F12, F15

If you catch yourself about to commit any F#, stop and address it.

## Conformance checklist (each cycle)

Verify before considering the cycle done.

**Reality**
- [ ] Step 0 done: production file + one existing test in the same package were read; new test references no phantom field/method/setter (F1, F2, F2b)

**Process**
- [ ] Spec confirmed before writing (F13)
- [ ] `go test` output captured showing the red state (F10)
- [ ] `want` literals were not edited to make a failing test pass (F11)
- [ ] Refactor pass run after green; nothing obviously duplicated/messy (F12)
- [ ] Implementation does only what the tests demand (F14)
- [ ] `want` describes observable state, not internal layout (F15)

**Convention** — defer to the `tester` checklist (R1–R11) and the `developer` checklist (P1–P13). Both must pass.

If any item fails, rewrite.

## Worked example: add `Reset` to `ParseResumeLocation`

### Step 0
Read `parse_resume_location.go`. Note: `ParseResumeLocation { Input string; BytePosition int32 }` with public fields, no getters. Existing test `parse_resume_location_test.go` uses testify, AAA, table-driven.

### Step 1: spec
- API: `func (l *ParseResumeLocation) Reset()` — sets `BytePosition` back to 0
- Input: receiver only; Output: none; Errors: none
- Positive cases: reset from `BytePosition=N`; reset already at 0

### Step 2: red

Following `tester`'s skeleton:

```go
func TestParseResumeLocation_Reset(t *testing.T) {
    tests := []struct {
        name    string
        initial *ParseResumeLocation
        want    *ParseResumeLocation
    }{
        {
            name:    "Reset from non-zero position",
            initial: &ParseResumeLocation{Input: "SELECT 1", BytePosition: 5},
            want:    &ParseResumeLocation{Input: "SELECT 1", BytePosition: 0},
        },
        {
            name:    "Reset already at zero",
            initial: &ParseResumeLocation{Input: "SELECT 1", BytePosition: 0},
            want:    &ParseResumeLocation{Input: "SELECT 1", BytePosition: 0},
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            sut := tt.initial
            // Act
            sut.Reset()
            got := sut
            // Assert
            assert.Equal(t, tt.want, got)
        })
    }
}
```

`go test -run TestParseResumeLocation_Reset` → compile error (`Reset` undefined). Expected red state.

### Step 3: green

Per `developer` (P1: behavior on the data, public field):

```go
func (l *ParseResumeLocation) Reset() {
    l.BytePosition = 0
}
```

`go test ./...` → all green.

### Step 4: refactor
Nothing to clean up at this size. Done.

## Anti-patterns

### AP1: Implementing before testing
**Symptom**: User says "add X" → you start writing `func X` immediately.
**Fix**: Write the test first. Confirm it fails. Then implement.

### AP2: Skipping the spec confirmation
**Symptom**: Inputs/outputs/errors aren't crisp; you start implementing on assumptions, then redo.
**Fix**: Pin down the spec in Step 1 before any code.

### AP3: Skipping triangulation
**Symptom**: One test case; "it works" is declared.
**Fix**: At least two positive cases. One case can be passed by hard-coding the value.

### AP4: Over-implementation (F14)
**Symptom**: Adding validation, logging, caching, or other concerns the tests don't ask for.
**Fix**: Implement only what tests demand. New requirements come with new tests.

### AP5: Skipping red verification (F10)
**Symptom**: "I'm sure it would have failed" without actually running `go test`.
**Fix**: Always run the test before the implementation. The cost is seconds; the safety is real.

### AP6: Adjusting want to match buggy output (F11)
**Symptom**: Test fails because impl is wrong, so the test's `want` gets edited to match the wrong output.
**Fix**: The `want` is the spec. Fix the impl, not the spec.

## Reporting back

After each cycle, summarize:

```
## TDD cycle complete

### red
- Test added: <file>:<TestFunc>
- Failure observed: <go test output snippet>

### green
- Implementation added: <file>:<func/type>
- All tests pass: <go test ./... summary>

### refactor
- <changes made, or "none">

### Conformance
- Process: all six items checked
- Test conventions (tester): all R1–R11 / list of deviations
- Production design (developer): all P1–P13 / list of deviations
```

This way the user can see at a glance whether the cycle ran cleanly.

## How this skill interacts with others

- `tester`: the source of truth for *how a test is written or reviewed*. When this skill says "write the test", the *shape* of that test (R1–R11, helpers, AAA) is `tester`'s domain.
- `developer`: the source of truth for *how production code is designed or reviewed*. When this skill says "implement", the *shape* of that implementation (P1–P13, DTO, layering, comments) is `developer`'s domain.

When in doubt: the cycle (this skill) ensures the change is built test-first; the other two skills ensure both halves of the change are well-shaped.
