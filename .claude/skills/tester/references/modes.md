# Author and Review modes (detailed)

The two operational modes for working with tests in zetasql-wasm. `SKILL.md` shows the mode skeletons (just the step names); this file is the detailed text — read it when you've actually entered one of the modes and need the full procedure.

## Author mode (writing a new test)

Use this when invoked from the `tdd` cycle (Step 2: red) or when the user asks "write a test for X".

### A1: Pick the SUT
- A function → the function itself
- A method → the receiver instance
- A constructor (`New*`) → frequently the SUT

### A2: Write a table-driven skeleton

```go
func TestSUT_Behavior(t *testing.T) {
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
            // Arrange — per case; no Arrange at function scope.
            ctx := t.Context()
            sut := ... // construct via Setup function (e.g., newTestAnalyzer(t))

            // Act
            got, err := sut.Method(ctx, tt.input)

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

The parameter table sits at function scope because it is read-only data, not Arrange. Anything mutable (the SUT, contexts, options, catalogs) goes inside the case. See `fixture-management.md` for the full rule and rationale.

### A3: `want` is a complete struct
For struct returns, build a fully populated `want` so any field add/remove appears in the diff. For scalar returns, the scalar type alone is fine.

### A4: Tree comparison uses the minimal helper
For AST-shaped output, use a helper that flattens to `<Kind> <single-accessor>\n`. Each switch `case` is a single accessor call. No compound formatting (R9).

### A5: Reuse, don't reinvent
Helpers like `newTestAnalyzer`, `newUsersCatalog`, `findNode` already exist. Use them. If you need a helper that doesn't exist, prefer extending an existing one (keeping each case a single accessor) over creating a parallel one.

### A6: Aligning with existing tests

Existing tests are a **style reference**, not an oracle of correctness. Treat them as cues for the local idiom — but keep R1–R12 as the standard of truth. If you see a violation in existing code, do not propagate it.

What to copy:
- Reuse the helpers above
- Imports from `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require`
- Prefer adding to a related existing test file (e.g., `parser_test.go`, `analyzer_test.go`) over creating a new file

What **not** to copy:
- A leftover getter or setter — even if it's there, the new code uses public fields directly (R7)
- A multi-assertion test — even if a neighbor has three `assert.Equal` calls, your new test still has one per case (R3)
- A custom payload struct in the test — even if you find one nearby, don't add another (R2, R6); flag the existing one for a separate review pass

When in doubt: the rules win, the example loses.

### A7: Run the pre-completion sniff scan

Before declaring the new test "done" (or before signalling that the TDD red phase is satisfied), execute the pre-completion sniff scan defined in `SKILL.md`. This is not optional — its purpose is to catch issues on the first pass instead of after user feedback. If the scan flags something, fix or delete it now; don't push it to a "follow-up".

## Review mode (reading an existing test)

Use this when the user says "review my tests" or similar.

### Step 1: Identify the target
Ask the user which file/directory to review. If unspecified:
- Check `git status` / `git log` for recently modified `*_test.go` files
- Confirm with the user before proceeding

### Step 2: Read every test file
Use the `Read` tool to load the target files completely. Include shared helper files within the same package.

### Step 3: Check rule by rule
Walk each test function through R1–R12 (see `rules.md` for the full text). Record every violation.

### Step 3b: Run the red-flag scan
After the rule pass, sweep the same files against the "Code-level red flags" table in `SKILL.md`. The table targets surface patterns that the rule pass can miss when reading large files quickly — dead-code tests, internal-state assertions, conditional test logic, and the like. Add any new findings to the report.

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
1. **R12 (behavior, not internal state)**: deletes whole tests that should not exist — fix first so subsequent rules apply to the right surface
2. **R3 (one assert per got)**: design-level
3. **R6 (test-side payload struct)**: structural
4. **R2 (SUT/got/want)**: clarifies structure
5. Format-level (R1, R4) last

### Review etiquette

- **Avoid heavy-handed MUSTs**: communicate the *why*, not blind directives
- **Concrete fixes only**: always show the corrected code; no vague "please fix"
- **Acknowledge trade-offs**: e.g., when constructing a `want` tree is impractical, R9's minimal helper is the legitimate substitute
- **Match existing patterns**: if neighboring tests in the same repo follow a particular style, anchor to it
- **Production-side smells**: when a test-side issue (e.g., R7 getter use, R11 test-only API) points at a production-code problem, refer the user to `developer` for the production fix
