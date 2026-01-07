# ZetaSQL WASM SDK Test Plan

## Purpose

Verify that the ZetaSQL WASM build's Go SDK functions correctly.

## Background

### WASM Build Characteristics

- ICU tools (pkgdata, icupkg, etc.) are disabled in cross-compilation environment (`--disable-tools`)
- ICU data is an empty stubdata (~600B), containing no locale data
- Some ICU-dependent features may have limitations

### Target SDK

- **Platform**: Go SDK (using wazero runtime)
- **Planned Publication**: github.com/glassmonkey/zetasql-wasm
- **Provided Features**: ZetaSQL parser, analyzer, various SQL functions

## Target Features

The following categories of ZetaSQL WASM SDK features will be verified.

### 1. SQL Parsing (Parser)

- **Target API**: `ParseStatement()`, `ParseExpression()`, `ParseType()`
- **Implementation File**: `zetasql/parser/parser.cc`
- **ICU Dependency**: None
- **Expected Behavior**: Fully functional

### 2. String Functions

- **Target Functions**: `UPPER()`, `LOWER()`, `LENGTH()`, `CONCAT()`, `SUBSTRING()`, `TRIM()`, `NORMALIZE()`
- **Implementation Files**: `zetasql/public/functions/string.cc`, `zetasql/common/unicode_utils.cc`
- **ICU Dependency**:
  - `UPPER()`, `LOWER()`: Use `icu::CaseMap` for locale-specific conversion
  - `NORMALIZE()`: Use `icu::Normalizer2` for Unicode normalization
- **Expected Behavior**:
  - ASCII characters: Fully functional
  - Non-ASCII characters: Works with default rules, or errors

### 3. Collation

- **Target Feature**: String comparison using `COLLATE` clause
- **Implementation File**: `zetasql/public/collator.cc`
- **ICU Dependency**: Uses `icu::Collator`, `icu::RuleBasedCollator` for locale-specific collation rules
- **Expected Behavior**: Error, or fallback to default collation order

### 4. Regexp Functions

- **Target Functions**: `REGEXP_CONTAINS()`, `REGEXP_EXTRACT()`, `REGEXP_REPLACE()`, `REGEXP_MATCH()`
- **Implementation File**: `zetasql/public/functions/regexp.cc`
- **ICU Dependency**: Uses character property tables for Unicode properties (`\p{L}`, `\p{Nd}`, etc.)
- **Expected Behavior**:
  - ASCII range patterns: Fully functional
  - Unicode properties: Error, or possible incorrect matches

### 5. UTF-8 Processing

- **Target Features**: Basic string operations, character counting
- **Implementation File**: `zetasql/common/utf_util.cc`
- **ICU Dependency**: `U8_*` macros (header-only, no data required)
- **Expected Behavior**: Fully functional

### 6. Other Features

- **Date/Time Functions**: `DATE()`, `TIMESTAMP()`, `FORMAT_DATE()`, etc.
- **Numeric Functions**: `ABS()`, `ROUND()`, `MOD()`, etc.
- **Aggregate Functions**: `SUM()`, `AVG()`, `COUNT()`, etc.
- **ICU Dependency**: Generally none
- **Expected Behavior**: Fully functional

## Test Cases

### Environment Setup

```go
package zetasql_test

import (
    "context"
    "testing"

    "github.com/glassmonkey/zetasql-wasm"
)

func setupParser(t *testing.T) (*zetasql.Parser, context.Context) {
    ctx := context.Background()
    parser, err := zetasql.NewParser(ctx)
    if err != nil {
        t.Fatalf("Failed to create parser: %v", err)
    }
    t.Cleanup(func() {
        parser.Close(ctx)
    })
    return parser, ctx
}
```

### 1. SQL Parsing Tests

#### 1.1 Basic Query Parsing

```go
func TestBasicParsing(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name  string
        query string
    }{
        {"simple select", "SELECT 1"},
        {"where clause", "SELECT id, name FROM users WHERE age > 20"},
        {"join", "SELECT * FROM a JOIN b ON a.id = b.a_id"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stmt, err := parser.ParseStatement(ctx, tt.query)
            if err != nil {
                t.Errorf("ParseStatement failed: %v", err)
            }
            if stmt == nil {
                t.Error("Expected non-nil statement")
            }
        })
    }
}
```

**Expected Result:** ✅ All succeed

**ICU Dependency:** None (parser doesn't depend on ICU)

#### 1.2 UTF-8 String Handling

```go
func TestUTF8StringHandling(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name  string
        query string
    }{
        {"japanese", "SELECT '日本語テスト'"},
        {"emoji", "SELECT '🔥 test'"},
        {"length", "SELECT LENGTH('こんにちは')"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stmt, err := parser.ParseStatement(ctx, tt.query)
            if err != nil {
                t.Errorf("ParseStatement failed: %v", err)
            }
            if stmt == nil {
                t.Error("Expected non-nil statement")
            }
        })
    }
}
```

**Expected Result:** ✅ All succeed

**ICU Dependency:** None (UTF-8 processing only uses `U8_*` header macros)

### 2. String Function Tests

#### 2.1 UPPER/LOWER Functions

```go
func TestCaseConversion(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name        string
        query       string
        expectError bool
        note        string
    }{
        {
            name:        "ASCII upper",
            query:       "SELECT UPPER('hello')",
            expectError: false,
            note:        "ASCII doesn't require locale data",
        },
        {
            name:        "German eszett",
            query:       "SELECT UPPER('straße')", // ß → SS
            expectError: false,
            note:        "Without ICU data, may produce incorrect result",
        },
        {
            name:        "Turkish I",
            query:       "SELECT LOWER('İstanbul')", // İ → i (Turkish)
            expectError: false,
            note:        "Without ICU data, may produce incorrect result",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stmt, err := parser.ParseStatement(ctx, tt.query)

            if tt.expectError && err == nil {
                t.Error("Expected error but got nil")
            } else if !tt.expectError && err != nil {
                t.Errorf("Unexpected error: %v", err)
            } else {
                t.Logf("Result: stmt=%v, error=%v, note=%s", stmt, err, tt.note)
            }
        })
    }
}
```

**Expected Result:**
- ✅ ASCII characters: Works normally
- ⚠️ Non-ASCII characters: Converts with default rules (may differ from expectations)

**ICU Dependency:** Yes (uses `icu::CaseMap` for locale-specific conversion)

**Investigation Items:**
- [ ] German ß (eszett) conversion result
- [ ] Turkish İ (dotted I) conversion result
- [ ] Details of default fallback behavior

#### 2.2 NORMALIZE Function

```go
func TestNormalization(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name        string
        query       string
        expectError bool
    }{
        {
            name:        "NFC normalization",
            query:       "SELECT NORMALIZE('é', NFC)", // U+00E9
            expectError: true, // Requires normalization table
        },
        {
            name:        "NFD normalization",
            query:       "SELECT NORMALIZE('é', NFD)", // U+0065 U+0301
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := parser.ParseStatement(ctx, tt.query)

            if tt.expectError {
                if err == nil {
                    t.Error("Expected error but got nil")
                } else {
                    t.Logf("Got expected error: %v", err)
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
            }
        })
    }
}
```

**Expected Result:**
- ❌ Error: "Normalizer not available" or similar message
- ⚠️ Or returns input unchanged (doesn't normalize)

**ICU Dependency:** Yes (uses `icu::Normalizer2` for Unicode normalization)

### 3. Collation Tests

#### 3.1 COLLATE Clause

```go
func TestCollation(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name        string
        query       string
        expectError bool
        errorMsg    string
    }{
        {
            name:        "en_US collation",
            query:       "SELECT 'a' < 'B' COLLATE 'en_US'",
            expectError: true, // Error expected due to missing ICU data
            errorMsg:    "Collation 'en_US' not found",
        },
        {
            name:        "de_DE collation",
            query:       "SELECT 'ä' = 'a' COLLATE 'de_DE'",
            expectError: true,
            errorMsg:    "Collation 'de_DE' not found",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := parser.ParseStatement(ctx, tt.query)

            if tt.expectError {
                if err == nil {
                    t.Error("Expected error but got nil")
                } else {
                    t.Logf("Got expected error: %v", err)
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
            }
        })
    }
}
```

**Expected Result:**
- ❌ Error: "Collation 'xx_XX' not found" or similar message
- ⚠️ Or fallback: Works with default collation order

**ICU Dependency:** Yes (uses `icu::Collator`, `icu::RuleBasedCollator` for locale-specific collation rules)

**Investigation Items:**
- [ ] Error message content
- [ ] Presence of fallback behavior
- [ ] Performance impact

### 4. Regexp Function Tests

#### 4.1 Unicode Properties

```go
func TestRegexpUnicodeProperties(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name        string
        query       string
        expectError bool
        note        string
    }{
        {
            name:        "ASCII regex",
            query:       `SELECT REGEXP_CONTAINS('test123', r'[a-z]+')`,
            expectError: false,
            note:        "ASCII range only, no ICU data required",
        },
        {
            name:        "Unicode letter property",
            query:       `SELECT REGEXP_CONTAINS('café', r'\p{L}+')`,
            expectError: false, // Parse itself may succeed
            note:        "\\p{L} (Unicode Letter) requires ICU data",
        },
        {
            name:        "Unicode digit property",
            query:       `SELECT REGEXP_CONTAINS('test123', r'\p{Nd}+')`,
            expectError: false,
            note:        "\\p{Nd} (Unicode Decimal Number) requires ICU data",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stmt, err := parser.ParseStatement(ctx, tt.query)

            if tt.expectError && err == nil {
                t.Error("Expected error but got nil")
            } else if !tt.expectError && err != nil {
                t.Errorf("Unexpected error: %v", err)
            } else {
                t.Logf("Result: stmt=%v, error=%v, note=%s", stmt, err, tt.note)
            }
        })
    }
}
```

**Expected Result:**
- ✅ ASCII range patterns: Works normally
- ❓ Unicode properties (`\p{}`): Parse succeeds, but may error or mismatch at runtime

**ICU Dependency:** Yes (uses character property tables for Unicode properties)

### 5. Edge Case Tests

#### 5.1 Empty Strings

```go
func TestEmptyStrings(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []string{
        "SELECT UPPER('')",
        "SELECT LOWER('')",
        // Add tests for COLLATE, NORMALIZE depending on implementation
    }

    for _, query := range tests {
        t.Run(query, func(t *testing.T) {
            _, err := parser.ParseStatement(ctx, query)
            if err != nil {
                t.Errorf("Should handle empty string without error: %v", err)
            }
        })
    }
}
```

**Expected Result:** ✅ No error, returns empty string

**ICU Dependency:** Varies by function

## Test Execution

### Prerequisites
- WASM build is complete (`wasm/zetasql.wasm` exists)
- Go 1.21 or later is installed

### Execution Steps

```bash
# 1. Navigate to repository root directory
cd /path/to/zetasql-wasm

# 2. Run tests
go test -v ./... -run TestICU

# 3. Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 4. Run benchmarks
go test -bench=. -benchmem ./...
```

### Log Collection

```go
// Add logging output in test file
func TestWithLogging(t *testing.T) {
    // Enable detailed logging
    t.Logf("Testing ICU-dependent feature...")

    // Record results for each test
    results := make(map[string]interface{})

    // Output results as JSON on test completion
    t.Cleanup(func() {
        if data, err := json.MarshalIndent(results, "", "  "); err == nil {
            t.Logf("Test results: %s", string(data))
        }
    })
}
```

## Success Criteria

### Phase 1: Basic Feature Validation
- [ ] SQL parsing functions work correctly
- [ ] ASCII-range string processing works correctly
- [ ] UTF-8 encoded strings are processed correctly
- [ ] ICU-dependent features handle errors appropriately (don't crash)
- [ ] Error messages are clear and understandable

### Phase 2: Documentation and Optimization
- [ ] Document operational status of each feature in README.md
- [ ] Specify ICU-dependent feature limitations and alternatives
- [ ] Publish performance benchmark results
- [ ] Establish ICU data provisioning method if necessary

## Feature-Specific Operational Status

### WASM Environment Operational Predictions

| Category | Feature | Operational Status | Limitations |
|---------|------|---------|---------|
| SQL Parsing | Parser (general) | ✅ Fully functional | None |
| UTF-8 Processing | Basic string operations | ✅ Fully functional | None |
| String Functions | UPPER/LOWER (ASCII) | ✅ Fully functional | None |
| String Functions | UPPER/LOWER (non-ASCII) | ⚠️ Partially functional | Locale-specific conversion unavailable, works with default rules |
| String Functions | NORMALIZE() | ❌ Limited | No ICU data, error or unsupported |
| Collation | COLLATE clause | ❌ Limited | Locale-specific collation unavailable, binary comparison alternative |
| Regexp | ASCII range | ✅ Fully functional | None |
| Regexp | Unicode properties | ⚠️ Partially functional | `\p{}` patterns may mismatch |

### WASM Build Technical Constraints

1. **ICU stubdata**
   - WASM build uses empty ICU stubdata (~600B)
   - Native build uses full ICU data (~30MB)
   - Does not contain locale information or normalization tables

2. **Cross-compilation Constraints**
   - `--disable-tools` prevents building ICU tools (pkgdata, icupkg, etc.)
   - Without tools, ICU data cannot be packaged into a static library

## Future Options

### Option 1: Maintain Current State (stubdata)
**Adoption Conditions:** When ZetaSQL usage is limited to Parser/Analyzer and doesn't use ICU-dependent features

- ✅ Advantages: Simple, small WASM size
- ❌ Disadvantages: Feature limitations

**Actions:**
- Document limitations in README.md
- Enhance error handling for ICU-dependent features

### Option 2: Provide ICU Data Separately
**Adoption Conditions:** When ICU-dependent features are required

- ✅ Advantages: Full functionality
- ❌ Disadvantages: WASM size increases by ~30MB

**Implementation:**
```go
// Bundle ICU data separately
//go:embed icudt75l.dat
var icuData []byte

// Load at runtime
func init() {
    // Write ICU data to wazero memory
    // Call udata_setCommonData()
}
```

### Option 3: Reduce ICU Dependencies
**Adoption Conditions:** Long-term solution

- ✅ Advantages: Fundamental solution
- ❌ Disadvantages: Requires ZetaSQL fork

**Considerations:**
- Identify ZetaSQL's ICU usage locations
- Investigate alternative implementation possibilities
- Propose to upstream

## Test Results Recording

After test execution, record results in the following format:

```markdown
## ZetaSQL WASM SDK Test Results

**Execution Date:** YYYY-MM-DD

**Environment:**
- OS: macOS/Linux/Windows
- Go version: 1.21.x
- WASM size: XX MB
- ICU stubdata size: 600B

**Results Summary:**
- ✅ Success: XX cases
- ⚠️ Partial: XX cases
- ❌ Failed: XX cases
- ⏭️ Skipped: XX cases

**Feature-Specific Results:**

### SQL Parsing
- ✅ Basic query parsing: Works normally
- ✅ UTF-8 string parsing: Works normally

### String Functions
- ✅ UPPER/LOWER (ASCII): Works normally
- ⚠️ UPPER/LOWER (non-ASCII): Works with default rules
- ❌ NORMALIZE(): Error "Normalizer not available"

### Collation
- ❌ COLLATE clause: Error "Collation 'en_US' not found"

### Regexp Functions
- ✅ ASCII range: Works normally
- ⚠️ Unicode properties: Parse succeeds, runtime behavior requires verification

**Notes:**
- ICU-dependent feature limitations are documented
- Alternative suggestions: ...
```

## References

### Documentation
- [patches.md](./patches.md) - Detailed explanation of WASM build patches
- [architecture.md](./architecture.md) - Overall project architecture design

### Implementation Details
- ICU stubdata implementation - ICU stubdata source code
- ZetaSQL Collator - ZetaSQL collation implementation
- ZetaSQL String Functions - String function implementation
- ZetaSQL Regexp Functions - Regexp function implementation

### External Resources
- [wazero documentation](https://wazero.io/) - WASM runtime for Go
- [ZetaSQL Documentation](https://github.com/google/zetasql) - ZetaSQL official documentation
- [ICU Data Management](https://unicode-org.github.io/icu/userguide/icu_data/) - ICU data management guide

---

**Last Updated**: 2026-01-08
**Version**: 1.0.0
**Status**: Draft
