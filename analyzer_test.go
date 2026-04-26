package zetasql

import (
	"strconv"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/catalog"
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalyzer_AnalyzeStatement_Errors verifies that the analyzer returns
// a typed *AnalyzeError for invalid SQL. wantErr is a type witness checked
// via assert.IsType.
func TestAnalyzer_AnalyzeStatement_Errors(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		cat     *catalog.SimpleCatalog
		wantErr error
	}{
		{
			name:    "incomplete SELECT",
			sql:     "SELECT",
			cat:     nil,
			wantErr: &AnalyzeError{},
		},
		{
			name: "table not found",
			sql:  "SELECT id FROM nonexistent",
			cat: func() *catalog.SimpleCatalog {
				c := catalog.NewSimpleCatalog("test")
				c.AddZetaSQLBuiltinFunctions(nil)
				return c
			}(),
			wantErr: &AnalyzeError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestAnalyzer(t)

			// Act
			_, got := sut.AnalyzeStatement(ctx, tt.sql, tt.cat, NewAnalyzerOptions())

			// Assert
			assert.IsType(t, tt.wantErr, got)
		})
	}
}

// TestAnalyzer_AnalyzeNextStatement_AST verifies that AnalyzeNextStatement
// resolves each statement in a multi-statement SQL string. The got is a
// slice of AST debug strings, one per statement.
func TestAnalyzer_AnalyzeNextStatement_AST(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "two literals",
			sql:  "SELECT 100; SELECT 200",
			want: []string{
				literalQueryAST(100),
				literalQueryAST(200),
			},
		},
		{
			name: "three literals",
			sql:  "SELECT 1; SELECT 2; SELECT 3",
			want: []string{
				literalQueryAST(1),
				literalQueryAST(2),
				literalQueryAST(3),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestAnalyzer(t)
			loc := NewParseResumeLocation(tt.sql)
			opts := NewAnalyzerOptions()

			// Act
			var got []string
			for {
				out, more, err := sut.AnalyzeNextStatement(ctx, loc, nil, opts)
				require.NoError(t, err)
				got = append(got, out.Statement.String())
				if !more {
					break
				}
			}

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAnalyzer_AnalyzeNextStatement_AdvancesLocation verifies that consuming
// every statement leaves the ParseResumeLocation at the end of input.
func TestAnalyzer_AnalyzeNextStatement_AdvancesLocation(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want *ParseResumeLocation
	}{
		{
			name: "two statements",
			sql:  "SELECT 1; SELECT 2",
			want: &ParseResumeLocation{Input: "SELECT 1; SELECT 2", BytePosition: int32(len("SELECT 1; SELECT 2"))},
		},
		{
			name: "three statements",
			sql:  "SELECT 1; SELECT 2; SELECT 3",
			want: &ParseResumeLocation{Input: "SELECT 1; SELECT 2; SELECT 3", BytePosition: int32(len("SELECT 1; SELECT 2; SELECT 3"))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestAnalyzer(t)
			got := NewParseResumeLocation(tt.sql)
			opts := NewAnalyzerOptions()

			// Act
			for more := true; more; {
				_, m, err := sut.AnalyzeNextStatement(ctx, got, nil, opts)
				require.NoError(t, err)
				more = m
			}

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAnalyzer_AnalyzeStatement_CustomFunction verifies that a user-defined
// scalar function registered in the catalog is resolved to the expected
// FunctionCall in the AST. Triangulated across function name and group.
func TestAnalyzer_AnalyzeStatement_CustomFunction(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		group    string
		argCount int
		sql      string
		want     string
	}{
		{
			name:     "two int64 args in custom group",
			funcName: "my_add",
			group:    "custom",
			argCount: 2,
			sql:      "SELECT my_add(1, 2)",
			want: `KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindFunctionCall custom:my_add
        KindLiteral 1
        KindLiteral 2
    KindSingleRowScan
`,
		},
		{
			name:     "single int64 arg in math group",
			funcName: "double",
			group:    "math",
			argCount: 1,
			sql:      "SELECT double(7)",
			want: `KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindFunctionCall math:double
        KindLiteral 7
    KindSingleRowScan
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			cat := catalog.NewSimpleCatalog("test")
			cat.AddZetaSQLBuiltinFunctions(nil)
			argTypes := make([]*types.FunctionArgumentType, tt.argCount)
			for i := range argTypes {
				argTypes[i] = types.NewFunctionArgumentType(types.Int64Type())
			}
			cat.Functions = append(cat.Functions, types.NewFunction(
				[]string{tt.funcName},
				tt.group,
				types.ScalarMode,
				[]*types.FunctionSignature{
					types.NewFunctionSignature(
						types.NewFunctionArgumentType(types.Int64Type()),
						argTypes,
					),
				},
			))
			sut := newTestAnalyzer(t)

			// Act
			out, err := sut.AnalyzeStatement(ctx, tt.sql, cat, NewAnalyzerOptions())
			require.NoError(t, err)
			got := out.Statement.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAnalyzer_AnalyzeStatement_TemplatedFunction verifies that a templated
// (ARG_TYPE_ANY_1) function is resolved with concrete argument types
// inferred from the call site.
func TestAnalyzer_AnalyzeStatement_TemplatedFunction(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		group    string
		sql      string
		want     string
	}{
		{
			name:     "identity in custom group",
			funcName: "identity",
			group:    "custom",
			sql:      "SELECT identity(42)",
			want: `KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindFunctionCall custom:identity
        KindLiteral 42
    KindSingleRowScan
`,
		},
		{
			name:     "negate in math group",
			funcName: "negate",
			group:    "math",
			sql:      "SELECT negate(7)",
			want: `KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindFunctionCall math:negate
        KindLiteral 7
    KindSingleRowScan
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			cat := catalog.NewSimpleCatalog("test")
			cat.AddZetaSQLBuiltinFunctions(nil)
			cat.Functions = append(cat.Functions, types.NewFunction(
				[]string{tt.funcName},
				tt.group,
				types.ScalarMode,
				[]*types.FunctionSignature{
					types.NewFunctionSignature(
						types.NewTemplatedFunctionArgumentType(generated.SignatureArgumentKind_ARG_TYPE_ANY_1),
						[]*types.FunctionArgumentType{
							types.NewTemplatedFunctionArgumentType(generated.SignatureArgumentKind_ARG_TYPE_ANY_1),
						},
					),
				},
			))
			sut := newTestAnalyzer(t)

			// Act
			out, err := sut.AnalyzeStatement(ctx, tt.sql, cat, NewAnalyzerOptions())
			require.NoError(t, err)
			got := out.Statement.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// literalQueryAST returns the expected AST debug string for "SELECT <n>".
func literalQueryAST(n int64) string {
	return "KindQueryStmt\n" +
		"  KindOutputColumn $col1\n" +
		"  KindProjectScan\n" +
		"    KindComputedColumn\n" +
		"      KindLiteral " + strconv.FormatInt(n, 10) + "\n" +
		"    KindSingleRowScan\n"
}
