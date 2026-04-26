# Anti-pattern catalog (AP1–AP6)

Concrete examples of test code that violates the rules, paired with the rule(s) the example breaks. `SKILL.md` lists the AP names with one-liners; this file holds the code samples and the rationale, loaded on demand when a fix needs to be illustrated.

## AP1: Multiple assertions in sequence
```go
// bad
assert.Equal(t, 1, len(cols))
assert.Equal(t, "id", cols[0].Name)
assert.Equal(t, "users", cols[0].TableName)
```
→ Violates R3. Either split into cases, or build a complete `want` struct comparison.

## AP2: Test-side payload construction
```go
// bad
type payload struct { Name string; Kind Kind }
got := payload{Name: stmt.Name(), Kind: stmt.Kind()}
```
→ Violates R2/R6. Use the production struct directly, or change the SUT to return a comparable shape (consult `developer`).

## AP3: Elaborate format helper
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

## AP4: Comparison via getter
```go
// bad
assert.Equal(t, sql, stmt.SQL())
```
→ Violates R7. Use direct field access (`stmt.SQL`) and remove the getter (a `developer` concern).

## AP5: Asserting on internal state with no behavioral consequence
```go
// bad — testing Go's map assignment
sut := NewLanguageOptions()
sut.EnableLanguageFeature(generated.LanguageFeature_FEATURE_TABLESAMPLE)
assert.True(t, sut.Features[generated.LanguageFeature_FEATURE_TABLESAMPLE])

// bad — testing that an export exists, when behavior tests already exercise it
exports := compiledModule.ExportedFunctions()
_, ok := exports["parse_statement_proto"]
assert.True(t, ok)

// bad — testing that &X{Field: v} assigned the field
got := NewParseResumeLocation("SELECT 1")
assert.Equal(t, &ParseResumeLocation{Input: "SELECT 1"}, got)
```
→ Violates R12. Each of these is *Sensitive Equality* + *Fragile Test* (Meszaros): the assertion locks in an implementation detail (storage layout, export-list shape, struct-literal mechanics) without verifying any contract that callers depend on. The behavior path — features take effect through the analyzer, exports are reachable through `Parser.ParseStatement`, the resume location is consumed by `AnalyzeNextStatement` — is what the test should target. Delete and rely on the integration/behavior test that exercises the same code path with an observable result.

## AP6: Testing a constructor directly
```go
// bad
func TestNewAnalyzerOptions(t *testing.T) {
    got := NewAnalyzerOptions()
    assert.Equal(t, &AnalyzerOptions{}, got)
}
```
→ Violates R12. A constructor whose body is `return &X{...}` has no behavior beyond Go's struct literal. The constructor stays in production as ergonomic call-site sugar (`developer` AP2 distinguishes "useful constructor" from "noise constructor"), but the constructor itself is not the SUT — the SUT is whatever method consumes the constructed value. Test that method.
