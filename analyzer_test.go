package zetasql

import (
	"strconv"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----- Test helpers (shared across the package's _test.go files) -----

func newTestAnalyzer(t *testing.T) *Analyzer {
	t.Helper()
	ctx := t.Context()
	a, err := NewAnalyzer(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { a.Close(ctx) })
	return a
}

func newTestParser(t *testing.T) *Parser {
	t.Helper()
	ctx := t.Context()
	p, err := NewParser(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { p.Close(ctx) })
	return p
}

func newUsersCatalog() *types.SimpleCatalog {
	cat := types.NewSimpleCatalog("test")
	cat.AddZetaSQLBuiltinFunctions(nil)
	cat.Tables = append(cat.Tables, types.NewSimpleTable("users",
		types.NewSimpleColumn("users", "id", types.Int64Type()),
		types.NewSimpleColumn("users", "name", types.StringType()),
	))
	return cat
}

func newUsersOrdersCatalog() *types.SimpleCatalog {
	cat := newUsersCatalog()
	cat.Tables = append(cat.Tables, types.NewSimpleTable("orders",
		types.NewSimpleColumn("orders", "order_id", types.Int64Type()),
		types.NewSimpleColumn("orders", "user_id", types.Int64Type()),
		types.NewSimpleColumn("orders", "amount", types.Int64Type()),
	))
	return cat
}

// newBuiltinsCatalog returns a catalog with only the ZetaSQL builtin
// functions registered (no tables). Use this for SELECT queries that do not
// reference any table but do use builtin functions like CAST, COUNT, etc.
func newBuiltinsCatalog() *types.SimpleCatalog {
	cat := types.NewSimpleCatalog("test")
	cat.AddZetaSQLBuiltinFunctions(nil)
	return cat
}

// newAnalyticOpts returns AnalyzerOptions with the FEATURE_ANALYTIC_FUNCTIONS
// language feature enabled. Required for OVER (...) window expressions.
func newAnalyticOpts() *AnalyzerOptions {
	lang := NewLanguageOptions()
	lang.EnableLanguageFeature(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS)
	return &AnalyzerOptions{Language: lang}
}

// findNode walks the tree and returns the first node matching the type T.
func findNode[T resolved_ast.Node](t *testing.T, root resolved_ast.Node) T {
	t.Helper()
	var result T
	var found bool
	resolved_ast.Walk(root, func(n resolved_ast.Node) error {
		if v, ok := n.(T); ok && !found {
			result = v
			found = true
		}
		return nil
	})
	require.True(t, found, "node of type %T not found", result)
	return result
}

// ----- Tests -----

// TestAnalyzer_AnalyzeStatement_AST verifies the resolved-AST shape produced
// by the analyzer for a variety of SQL inputs. Triangulated across multiple
// query patterns (literal, table scan, join, filter, project, function call).
func TestAnalyzer_AnalyzeStatement_AST(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		cat  *types.SimpleCatalog
		opts *AnalyzerOptions // nil → NewAnalyzerOptions()
		want string
	}{
		{
			name: "literal",
			sql:  "SELECT 1",
			cat:  nil,
			want: `KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindLiteral 1
    KindSingleRowScan
`,
		},
		{
			name: "select columns from users",
			sql:  "SELECT id, name FROM users",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn id
  KindOutputColumn name
  KindProjectScan
    KindTableScan users
`,
		},
		{
			name: "filter scan",
			sql:  "SELECT id FROM users WHERE id > 1",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn id
  KindProjectScan
    KindFilterScan
      KindTableScan users
      KindFunctionCall ZetaSQL:$greater
        KindColumnRef id
        KindLiteral 1
`,
		},
		{
			name: "project with addition",
			sql:  "SELECT id + 1 AS inc FROM users",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn inc
  KindProjectScan
    KindComputedColumn
      KindFunctionCall ZetaSQL:$add
        KindColumnRef id
        KindLiteral 1
    KindTableScan users
`,
		},
		{
			name: "function call with column arg",
			sql:  "SELECT UPPER(name) FROM users",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindFunctionCall ZetaSQL:upper
        KindColumnRef name
    KindTableScan users
`,
		},
		{
			name: "inner join",
			sql:  "SELECT u.id, o.amount FROM users u JOIN orders o ON u.id = o.user_id",
			cat:  newUsersOrdersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn id
  KindOutputColumn amount
  KindProjectScan
    KindJoinScan INNER
      KindTableScan users
      KindTableScan orders
      KindFunctionCall ZetaSQL:$equal
        KindColumnRef id
        KindColumnRef user_id
`,
		},
		{
			name: "count aggregation",
			sql:  "SELECT COUNT(*) FROM users",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindAggregateScan
      KindTableScan users
      KindComputedColumn
        KindAggregateFunctionCall
`,
		},
		{
			name: "order by descending",
			sql:  "SELECT id FROM users ORDER BY id DESC",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn id
  KindOrderByScan
    KindTableScan users
`,
		},
		{
			name: "limit and offset",
			sql:  "SELECT id FROM users LIMIT 10 OFFSET 5",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn id
  KindLimitOffsetScan
    KindProjectScan
      KindTableScan users
    KindLiteral 10
    KindLiteral 5
`,
		},
		{
			name: "group by",
			sql:  "SELECT id, COUNT(*) FROM users GROUP BY id",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn id
  KindOutputColumn $col2
  KindProjectScan
    KindAggregateScan
      KindTableScan users
      KindComputedColumn
        KindAggregateFunctionCall
`,
		},
		{
			name: "distinct",
			sql:  "SELECT DISTINCT name FROM users",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn name
  KindAggregateScan
    KindTableScan users
`,
		},
		{
			name: "union all",
			sql:  "SELECT id FROM users UNION ALL SELECT order_id FROM orders",
			cat:  newUsersOrdersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn id
  KindSetOperationScan
    KindSetOperationItem
      KindProjectScan
        KindTableScan users
    KindSetOperationItem
      KindProjectScan
        KindTableScan orders
`,
		},
		{
			name: "CTE single binding",
			sql:  "WITH x AS (SELECT 1 AS a) SELECT * FROM x",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStmt
  KindOutputColumn a
  KindWithScan
    KindProjectScan
      KindWithRefScan
`,
		},
		{
			name: "CTE chain referencing previous binding",
			sql:  "WITH a AS (SELECT 1 AS v), b AS (SELECT v * 2 AS v FROM a) SELECT * FROM b",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStmt
  KindOutputColumn v
  KindWithScan
    KindProjectScan
      KindWithRefScan
`,
		},
		{
			name: "ARRAY literal",
			sql:  "SELECT [1, 2, 3] AS arr",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStmt
  KindOutputColumn arr
  KindProjectScan
    KindComputedColumn
      KindLiteral <ARRAY>
        KindLiteral 1
        KindLiteral 2
        KindLiteral 3
    KindSingleRowScan
`,
		},
		{
			name: "ARRAY OFFSET access",
			sql:  "SELECT [10, 20, 30][OFFSET(1)] AS x",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStmt
  KindOutputColumn x
  KindProjectScan
    KindComputedColumn
      KindFunctionCall ZetaSQL:$array_at_offset
        KindLiteral <ARRAY>
          KindLiteral 10
          KindLiteral 20
          KindLiteral 30
        KindLiteral 1
    KindSingleRowScan
`,
		},
		{
			name: "STRUCT named fields",
			sql:  "SELECT STRUCT(1 AS a, 'x' AS b) AS s",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStmt
  KindOutputColumn s
  KindProjectScan
    KindComputedColumn
      KindLiteral <STRUCT>
        KindLiteral 1
        KindLiteral "x"
    KindSingleRowScan
`,
		},
		{
			name: "STRUCT field access",
			sql:  "SELECT s.a FROM (SELECT STRUCT(1 AS a) AS s) AS t",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStmt
  KindOutputColumn a
  KindProjectScan
    KindComputedColumn
      KindGetStructField
        KindColumnRef s
    KindProjectScan
      KindComputedColumn
        KindLiteral <STRUCT>
          KindLiteral 1
      KindSingleRowScan
`,
		},
		{
			name: "UNNEST as table source",
			sql:  "SELECT v FROM UNNEST([1, 2, 3]) AS v",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStmt
  KindOutputColumn v
  KindProjectScan
    KindArrayScan
      KindLiteral <ARRAY>
        KindLiteral 1
        KindLiteral 2
        KindLiteral 3
`,
		},
		{
			name: "CAST",
			sql:  "SELECT CAST('42' AS INT64) AS x",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStmt
  KindOutputColumn x
  KindProjectScan
    KindComputedColumn
      KindLiteral 42
    KindSingleRowScan
`,
		},
		{
			name: "NOT BETWEEN",
			sql:  "SELECT id FROM users WHERE NOT id BETWEEN 1 AND 10",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn id
  KindProjectScan
    KindFilterScan
      KindTableScan users
      KindFunctionCall ZetaSQL:$not
        KindFunctionCall ZetaSQL:$between
          KindColumnRef id
          KindLiteral 1
          KindLiteral 10
`,
		},
		{
			name: "IS NULL predicate",
			sql:  "SELECT id FROM users WHERE name IS NULL",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn id
  KindProjectScan
    KindFilterScan
      KindTableScan users
      KindFunctionCall ZetaSQL:$is_null
        KindColumnRef name
`,
		},
		{
			name: "CASE expression",
			sql:  "SELECT CASE WHEN id > 0 THEN 'a' ELSE 'b' END AS lbl FROM users",
			cat:  newUsersCatalog(),
			want: `KindQueryStmt
  KindOutputColumn lbl
  KindProjectScan
    KindComputedColumn
      KindFunctionCall ZetaSQL:$case_no_value
        KindFunctionCall ZetaSQL:$greater
          KindColumnRef id
          KindLiteral 0
        KindLiteral "a"
        KindLiteral "b"
    KindTableScan users
`,
		},
		{
			name: "window function with PARTITION BY",
			sql:  "SELECT SUM(id) OVER (PARTITION BY name) AS s FROM users",
			cat:  newUsersCatalog(),
			opts: newAnalyticOpts(),
			want: `KindQueryStmt
  KindOutputColumn s
  KindProjectScan
    KindAnalyticScan
      KindTableScan users
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestAnalyzer(t)
			opts := tt.opts
			if opts == nil {
				opts = NewAnalyzerOptions()
			}

			// Act
			out, err := sut.AnalyzeStatement(ctx, tt.sql, tt.cat, opts)
			require.NoError(t, err)
			got := out.Statement.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAnalyzer_AnalyzeStatement_Errors verifies that the analyzer returns
// a typed *AnalyzeError for invalid SQL. wantErr is a type witness checked
// via assert.IsType.
func TestAnalyzer_AnalyzeStatement_Errors(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		cat     *types.SimpleCatalog
		wantErr error
	}{
		{
			name:    "syntax error: incomplete SELECT",
			sql:     "SELECT",
			cat:     nil,
			wantErr: &AnalyzeError{},
		},
		{
			name:    "table not found",
			sql:     "SELECT id FROM nonexistent",
			cat:     newBuiltinsCatalog(),
			wantErr: &AnalyzeError{},
		},
		{
			name:    "column not found in known table",
			sql:     "SELECT nonexistent FROM users",
			cat:     newUsersCatalog(),
			wantErr: &AnalyzeError{},
		},
		{
			name:    "function not found",
			sql:     "SELECT my_undefined_fn(id) FROM users",
			cat:     newUsersCatalog(),
			wantErr: &AnalyzeError{},
		},
		{
			name:    "type mismatch in arithmetic",
			sql:     "SELECT id + name FROM users",
			cat:     newUsersCatalog(),
			wantErr: &AnalyzeError{},
		},
		{
			name:    "wrong argument count",
			sql:     "SELECT LENGTH() FROM users",
			cat:     newUsersCatalog(),
			wantErr: &AnalyzeError{},
		},
		{
			name:    "ungrouped column with aggregate",
			sql:     "SELECT id, COUNT(*) FROM users",
			cat:     newUsersCatalog(),
			wantErr: &AnalyzeError{},
		},
		{
			name:    "syntax error reaching analyzer",
			sql:     "SELECT * FROM",
			cat:     nil,
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
			cat := types.NewSimpleCatalog("test")
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
			cat := types.NewSimpleCatalog("test")
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
