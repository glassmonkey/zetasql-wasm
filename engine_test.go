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

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	ctx := t.Context()
	e, err := New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = e.Close(ctx) })
	return e
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
	lang.EnableLanguageFeature(FeatureAnalyticFunctions)
	return &AnalyzerOptions{Language: lang}
}

// newQueryStmtAnalyzerOptions builds AnalyzerOptions configured to
// accept top-level QUERY statements — the only statement kind the
// parameter-flow integration tests need. Returns a fresh instance per
// call so per-case Arrange stays independent.
func newQueryStmtAnalyzerOptions() *AnalyzerOptions {
	lang := NewLanguageOptions()
	lang.SetSupportedStatementKinds([]StatementKind{StatementKindQuery})
	return &AnalyzerOptions{Language: lang}
}

// observedParam is the projection of *resolved_ast.ParameterNode the
// parameter-flow integration tests assert on — the resolved type kind
// and whether the analyzer marked the parameter untyped. Combining
// both into one tuple keeps each happy case to a single assert.Equal.
type observedParam struct {
	TypeKind  generated.TypeKind
	IsUntyped bool
}

func observeParam(p *resolved_ast.ParameterNode) observedParam {
	return observedParam{
		TypeKind:  p.Type().GetTypeKind(),
		IsUntyped: p.IsUntyped(),
	}
}

// findNode walks the tree and returns the first node matching the type T.
func findNode[T resolved_ast.Node](t *testing.T, root resolved_ast.Node) T {
	t.Helper()
	var result T
	var found bool
	_ = resolved_ast.Walk(root, func(n resolved_ast.Node) error {
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

// TestEngine_Analyze covers both the happy-path resolved-AST shape and
// invalid-SQL error type for Engine.Analyze. Cases share a single fixture
// (newTestEngine); errors are flagged in the table via wantErr (type
// witness) — happy cases set want only, error cases set wantErr only. The
// error path also asserts that the AnalyzeOutput is nil so a regression
// that returned a partial output alongside an error would surface
// (R13 Width).
func TestEngine_Analyze(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		cat     *types.SimpleCatalog
		opts    *AnalyzerOptions // nil → NewAnalyzerOptions()
		want    string           // happy-path expected AST string; empty when wantErr is set
		wantErr error            // type witness for error cases; nil for happy path
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
		{
			name: "named query parameter in WHERE",
			sql:  "SELECT id FROM users WHERE id = @id",
			cat:  newUsersCatalog(),
			opts: &AnalyzerOptions{
				ParameterMode:   ParameterNamed,
				QueryParameters: map[string]types.Type{"id": types.Int64Type()},
			},
			want: `KindQueryStmt
  KindOutputColumn id
  KindProjectScan
    KindFilterScan
      KindTableScan users
      KindFunctionCall ZetaSQL:$equal
        KindColumnRef id
        KindParameter
`,
		},
		{
			name: "named query parameters in BETWEEN",
			sql:  "SELECT id FROM users WHERE id BETWEEN @lo AND @hi",
			cat:  newUsersCatalog(),
			opts: &AnalyzerOptions{
				ParameterMode: ParameterNamed,
				QueryParameters: map[string]types.Type{
					"lo": types.Int64Type(),
					"hi": types.Int64Type(),
				},
			},
			want: `KindQueryStmt
  KindOutputColumn id
  KindProjectScan
    KindFilterScan
      KindTableScan users
      KindFunctionCall ZetaSQL:$between
        KindColumnRef id
        KindParameter
        KindParameter
`,
		},
		// IN UNNEST(@ids) is the literal SQL shape that triggered the
		// v0.5.0 emulator boot bug — bigquery-emulator's
		// internal/metadata/repository.go runs `WHERE id IN UNNEST(@ids)`
		// during server startup. Pinning it here exercises the actual
		// front door: an ARRAY<INT64> parameter resolved into a
		// FunctionCall ZetaSQL:$in_array against a TableScan.
		{
			name: "named array parameter in IN UNNEST (emulator boot pattern)",
			sql:  "SELECT id FROM users WHERE id IN UNNEST(@ids)",
			cat:  newUsersCatalog(),
			opts: &AnalyzerOptions{
				ParameterMode:   ParameterNamed,
				QueryParameters: map[string]types.Type{
					"ids": &types.ArrayType{ElementType: types.Int64Type()},
				},
			},
			want: `KindQueryStmt
  KindOutputColumn id
  KindProjectScan
    KindFilterScan
      KindTableScan users
      KindFunctionCall ZetaSQL:$in_array
        KindColumnRef id
        KindParameter
`,
		},
		// Error cases — SQL the analyzer must reject. The contract is
		// (output == nil, err is *AnalyzeError); both halves are
		// asserted in the loop body.
		{name: "syntax error: incomplete SELECT", sql: "SELECT", wantErr: &AnalyzeError{}},
		{name: "table not found", sql: "SELECT id FROM nonexistent", cat: newBuiltinsCatalog(), wantErr: &AnalyzeError{}},
		{name: "column not found in known table", sql: "SELECT nonexistent FROM users", cat: newUsersCatalog(), wantErr: &AnalyzeError{}},
		{name: "function not found", sql: "SELECT my_undefined_fn(id) FROM users", cat: newUsersCatalog(), wantErr: &AnalyzeError{}},
		{name: "type mismatch in arithmetic", sql: "SELECT id + name FROM users", cat: newUsersCatalog(), wantErr: &AnalyzeError{}},
		{name: "wrong argument count", sql: "SELECT LENGTH() FROM users", cat: newUsersCatalog(), wantErr: &AnalyzeError{}},
		{name: "ungrouped column with aggregate", sql: "SELECT id, COUNT(*) FROM users", cat: newUsersCatalog(), wantErr: &AnalyzeError{}},
		{name: "syntax error reaching analyzer", sql: "SELECT * FROM", wantErr: &AnalyzeError{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestEngine(t)
			opts := tt.opts
			if opts == nil {
				opts = NewAnalyzerOptions()
			}

			// Act
			out, err := sut.Analyze(ctx, tt.sql, tt.cat, opts)

			// Assert
			if tt.wantErr != nil {
				assert.IsType(t, tt.wantErr, err)
				assert.Nil(t, out)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, out.Statement.String())
		})
	}
}

// TestEngine_Analyze_ParsedAST verifies that AnalyzeStatement
// surfaces the parser AST alongside the resolved Statement. The expected
// strings mirror the shape TestEngine_Parse asserts for the same
// SQL — the contract being locked is "AnalyzeOutput.Parsed is the parser
// AST for the input, not nil and not an unrelated tree". Triangulated
// across SQL families (literal, table scan, function call) so a regression
// that nils Parsed for one family but not another surfaces.
func TestEngine_Analyze_ParsedAST(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		cat  *types.SimpleCatalog
		want string
	}{
		{
			name: "literal",
			sql:  "SELECT 1",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral [1]
`,
		},
		{
			name: "table scan with column reference",
			sql:  "SELECT id FROM users",
			cat:  newUsersCatalog(),
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [id]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
		},
		{
			name: "function call",
			sql:  "SELECT UPPER('x')",
			cat:  newBuiltinsCatalog(),
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindFunctionCall
            KindPathExpression
              KindIdentifier [UPPER]
            KindStringLiteral [x]
              KindStringLiteralComponent
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestEngine(t)

			// Act
			out, err := sut.Analyze(ctx, tt.sql, tt.cat, NewAnalyzerOptions())
			require.NoError(t, err)
			require.NotNil(t, out.Parsed, "AnalyzeOutput.Parsed must be populated")
			got := out.Parsed.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEngine_AnalyzeNext_ParsedAST locks the same parser-AST
// contract for the multi-statement bridge path. A regression that nils
// Parsed only on AnalyzeNextStatement (because the path uses a separate
// ParseNextStatement + AnalyzeStatementFromParserOutputUnowned wiring)
// would slip past the single-statement test above without this case.
func TestEngine_AnalyzeNext_ParsedAST(t *testing.T) {
	// Arrange
	ctx := t.Context()
	sut := newTestEngine(t)
	loc := NewParseResumeLocation("SELECT 1; SELECT 2")
	opts := NewAnalyzerOptions()

	// Act
	var got []string
	for {
		out, more, err := sut.AnalyzeNext(ctx, loc, nil, opts)
		require.NoError(t, err)
		require.NotNil(t, out.Parsed, "Parsed must be populated on each AnalyzeNextStatement call")
		got = append(got, out.Parsed.String())
		if !more {
			break
		}
	}

	// Assert
	want := []string{
		`KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral [1]
`,
		`KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral [2]
`,
	}
	assert.Equal(t, want, got)
}

// TestEngine_AnalyzeNext_AST verifies that AnalyzeNextStatement
// resolves each statement in a multi-statement SQL string. The got is a
// slice of AST debug strings, one per statement.
func TestEngine_AnalyzeNext_AST(t *testing.T) {
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
			sut := newTestEngine(t)
			loc := NewParseResumeLocation(tt.sql)
			opts := NewAnalyzerOptions()

			// Act
			var got []string
			for {
				out, more, err := sut.AnalyzeNext(ctx, loc, nil, opts)
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

// TestEngine_AnalyzeNext_AdvancesLocation verifies that consuming
// every statement leaves the ParseResumeLocation at the end of input.
func TestEngine_AnalyzeNext_AdvancesLocation(t *testing.T) {
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
			sut := newTestEngine(t)
			got := NewParseResumeLocation(tt.sql)
			opts := NewAnalyzerOptions()

			// Act
			for more := true; more; {
				_, m, err := sut.AnalyzeNext(ctx, got, nil, opts)
				require.NoError(t, err)
				more = m
			}

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEngine_Analyze_CustomFunction verifies that a user-defined
// scalar function registered in the catalog is resolved to the expected
// FunctionCall in the AST. Triangulated across function name and group.
func TestEngine_Analyze_CustomFunction(t *testing.T) {
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
			sut := newTestEngine(t)

			// Act
			out, err := sut.Analyze(ctx, tt.sql, cat, NewAnalyzerOptions())
			require.NoError(t, err)
			got := out.Statement.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEngine_Analyze_TemplatedFunction verifies that a templated
// (ARG_TYPE_ANY_1) function is resolved with concrete argument types
// inferred from the call site.
func TestEngine_Analyze_TemplatedFunction(t *testing.T) {
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
						types.NewTemplatedFunctionArgumentType(types.ArgTypeAny1),
						[]*types.FunctionArgumentType{
							types.NewTemplatedFunctionArgumentType(types.ArgTypeAny1),
						},
					),
				},
			))
			sut := newTestEngine(t)

			// Act
			out, err := sut.Analyze(ctx, tt.sql, cat, NewAnalyzerOptions())
			require.NoError(t, err)
			got := out.Statement.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEngine_Analyze_PositionalParameter covers the
// positional-parameter analyzer path end to end: the wire fields
// PositionalQueryParameters / ParameterMode round-trip through the
// WASM bridge and the declared count is consulted by the analyzer.
//
// SQL inputs follow real-world shapes (`WHERE id = ?`,
// `WHERE name = ? AND id = ?`) rather than synthetic `SELECT ?` so a
// regression that only affects nested-position `?` usage surfaces.
//
// Triangulated across:
//   - declared 1 INT64 / 2 (STRING, INT64) resolve cleanly,
//   - count mismatches (no positionals declared / one declared but
//     two `?` used) reject.
//
// (got, err) is checked in both halves: an error case carries a nil
// output, a happy case carries a non-nil output whose first
// ParameterNode payload (TypeKind + IsUntyped) is observed via
// observeParam.
func TestEngine_Analyze_PositionalParameter(t *testing.T) {
	tests := []struct {
		name      string
		params    []types.Type
		sql       string
		wantParam observedParam
		wantErr   error
	}{
		{
			name:      "single INT64 positional in WHERE",
			params:    []types.Type{types.Int64Type()},
			sql:       "SELECT id FROM users WHERE id = ?",
			wantParam: observedParam{TypeKind: generated.TypeKind_TYPE_INT64},
		},
		{
			name:      "two positionals (STRING, INT64) in WHERE — first resolves to STRING",
			params:    []types.Type{types.StringType(), types.Int64Type()},
			sql:       "SELECT id FROM users WHERE name = ? AND id = ?",
			wantParam: observedParam{TypeKind: generated.TypeKind_TYPE_STRING},
		},
		{
			name:    "no positionals declared but SQL uses one",
			sql:     "SELECT id FROM users WHERE id = ?",
			wantErr: &AnalyzeError{},
		},
		{
			name:    "one positional declared but SQL uses two",
			params:  []types.Type{types.Int64Type()},
			sql:     "SELECT id FROM users WHERE name = ? AND id = ?",
			wantErr: &AnalyzeError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			a := newTestEngine(t)
			cat := newUsersCatalog()
			opts := newQueryStmtAnalyzerOptions()
			opts.ParameterMode = ParameterPositional
			opts.PositionalQueryParameters = tt.params

			// Act
			got, err := a.Analyze(ctx, tt.sql, cat, opts)

			// Assert
			if tt.wantErr != nil {
				assert.IsType(t, tt.wantErr, err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			stmt, ok := got.Statement.(*resolved_ast.QueryStmtNode)
			require.True(t, ok, "Statement is %T, want *resolved_ast.QueryStmtNode", got.Statement)
			param := findNode[*resolved_ast.ParameterNode](t, stmt)
			assert.Equal(t, tt.wantParam, observeParam(param))
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

// TestEngine_Parse covers both the happy-path AST shape and the
// invalid-SQL error type for Engine.Parse. Cases share a single fixture
// (newTestEngine); errors are flagged in the table via wantErr (type
// witness) — happy cases set want only, error cases set wantErr only.
func TestEngine_Parse(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		want    string  // happy-path expected AST string; empty when wantErr is set
		wantErr error   // type witness for error cases; nil for happy path
	}{
		{
			name: "literal",
			sql:  "SELECT 1",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral [1]
`,
		},
		{
			name: "star from table",
			sql:  "SELECT * FROM users",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
		},
		{
			name: "where clause",
			sql:  "SELECT * FROM users WHERE age > 20",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
      KindWhereClause
        KindBinaryExpression
          KindPathExpression
            KindIdentifier [age]
          KindIntLiteral [20]
`,
		},
		{
			name: "multiple columns",
			sql:  "SELECT name, email FROM customers",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [name]
        KindSelectColumn
          KindPathExpression
            KindIdentifier [email]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [customers]
`,
		},
		{
			name: "aggregate function call",
			sql:  "SELECT COUNT(*) FROM users",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindFunctionCall
            KindPathExpression
              KindIdentifier [COUNT]
            KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
		},
		{
			name: "order by descending",
			sql:  "SELECT * FROM users ORDER BY id DESC",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
    KindOrderBy
      KindOrderingExpression [DESC]
        KindPathExpression
          KindIdentifier [id]
`,
		},
		{
			name: "limit and offset",
			sql:  "SELECT * FROM users LIMIT 10 OFFSET 5",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
    KindLimitOffset
      KindLimit
        KindIntLiteral [10]
      KindIntLiteral [5]
`,
		},
		{
			name: "inner join with table aliases",
			sql:  "SELECT u.id FROM users AS u JOIN orders AS o ON u.id = o.user_id",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [u]
            KindIdentifier [id]
      KindFromClause
        KindJoin
          KindTablePathExpression
            KindPathExpression
              KindIdentifier [users]
            KindAlias
              KindIdentifier [u]
          KindTablePathExpression
            KindPathExpression
              KindIdentifier [orders]
            KindAlias
              KindIdentifier [o]
          KindOnClause
            KindBinaryExpression
              KindPathExpression
                KindIdentifier [u]
                KindIdentifier [id]
              KindPathExpression
                KindIdentifier [o]
                KindIdentifier [user_id]
          KindLocation
`,
		},
		{
			name: "group by with having",
			sql:  "SELECT id, COUNT(*) FROM users GROUP BY id HAVING COUNT(*) > 1",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [id]
        KindSelectColumn
          KindFunctionCall
            KindPathExpression
              KindIdentifier [COUNT]
            KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
      KindGroupBy
        KindGroupingItem
          KindPathExpression
            KindIdentifier [id]
      KindHaving
        KindBinaryExpression
          KindFunctionCall
            KindPathExpression
              KindIdentifier [COUNT]
            KindStar
          KindIntLiteral [1]
`,
		},
		{
			name: "column alias with AS",
			sql:  "SELECT name AS n FROM users",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [name]
          KindAlias
            KindIdentifier [n]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
		},
		{
			name: "IN operator",
			sql:  "SELECT * FROM users WHERE id IN (1, 2, 3)",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
      KindWhereClause
        KindInExpression
          KindPathExpression
            KindIdentifier [id]
          KindInList
            KindIntLiteral [1]
            KindIntLiteral [2]
            KindIntLiteral [3]
          KindLocation
`,
		},
		{
			name: "CASE expression",
			sql:  "SELECT CASE WHEN id > 0 THEN 'a' ELSE 'b' END FROM users",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindCaseNoValueExpression
            KindBinaryExpression
              KindPathExpression
                KindIdentifier [id]
              KindIntLiteral [0]
            KindStringLiteral [a]
              KindStringLiteralComponent
            KindStringLiteral [b]
              KindStringLiteralComponent
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
		},
		{
			name: "subquery in FROM clause",
			sql:  "SELECT * FROM (SELECT id FROM users) AS sub",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTableSubquery
          KindQuery
            KindSelect
              KindSelectList
                KindSelectColumn
                  KindPathExpression
                    KindIdentifier [id]
              KindFromClause
                KindTablePathExpression
                  KindPathExpression
                    KindIdentifier [users]
          KindAlias
            KindIdentifier [sub]
`,
		},
		{
			name: "UNION ALL of two SELECTs",
			sql:  "SELECT id FROM users UNION ALL SELECT id FROM orders",
			want: `KindQueryStatement
  KindQuery
    KindSetOperation
      KindSetOperationMetadataList
        KindSetOperationMetadata
          KindSetOperationType [UNION]
          KindSetOperationAllOrDistinct [ALL]
      KindSelect
        KindSelectList
          KindSelectColumn
            KindPathExpression
              KindIdentifier [id]
        KindFromClause
          KindTablePathExpression
            KindPathExpression
              KindIdentifier [users]
      KindSelect
        KindSelectList
          KindSelectColumn
            KindPathExpression
              KindIdentifier [id]
        KindFromClause
          KindTablePathExpression
            KindPathExpression
              KindIdentifier [orders]
`,
		},
		{
			name: "IS NULL predicate",
			sql:  "SELECT * FROM users WHERE name IS NULL",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
      KindWhereClause
        KindBinaryExpression
          KindPathExpression
            KindIdentifier [name]
          KindNullLiteral [NULL]
`,
		},
		{
			name: "DISTINCT modifier",
			sql:  "SELECT DISTINCT name FROM users",
			want: `KindQueryStatement
  KindQuery
    KindSelect [DISTINCT]
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [name]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
		},
		{
			// Smoke test only: the AST wrapper does not yet expose children
			// of CreateTableStatement, so String() returns just the kind. This
			// confirms the parser routes the DDL to the right node kind but
			// does not validate the column list. Tighten when the wrapper
			// gains child accessors for this kind.
			name: "CREATE TABLE DDL",
			sql:  "CREATE TABLE t1 (id INT64, name STRING)",
			want: `KindCreateTableStatement
`,
		},
		{
			name: "CTE with single binding",
			sql:  "WITH x AS (SELECT 1 AS a) SELECT * FROM x",
			want: `KindQueryStatement
  KindQuery
    KindWithClause
      KindWithClauseEntry
        KindAliasedQuery
          KindIdentifier [x]
          KindQuery
            KindSelect
              KindSelectList
                KindSelectColumn
                  KindIntLiteral [1]
                  KindAlias
                    KindIdentifier [a]
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [x]
`,
		},
		{
			name: "CTE chain referencing previous binding",
			sql:  "WITH a AS (SELECT 1 AS v), b AS (SELECT v * 2 AS v FROM a) SELECT * FROM b",
			want: `KindQueryStatement
  KindQuery
    KindWithClause
      KindWithClauseEntry
        KindAliasedQuery
          KindIdentifier [a]
          KindQuery
            KindSelect
              KindSelectList
                KindSelectColumn
                  KindIntLiteral [1]
                  KindAlias
                    KindIdentifier [v]
      KindWithClauseEntry
        KindAliasedQuery
          KindIdentifier [b]
          KindQuery
            KindSelect
              KindSelectList
                KindSelectColumn
                  KindBinaryExpression
                    KindPathExpression
                      KindIdentifier [v]
                    KindIntLiteral [2]
                  KindAlias
                    KindIdentifier [v]
              KindFromClause
                KindTablePathExpression
                  KindPathExpression
                    KindIdentifier [a]
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [b]
`,
		},
		{
			name: "ARRAY literal",
			sql:  "SELECT [1, 2, 3]",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindArrayConstructor
            KindIntLiteral [1]
            KindIntLiteral [2]
            KindIntLiteral [3]
`,
		},
		{
			name: "ARRAY OFFSET access",
			sql:  "SELECT [10, 20, 30][OFFSET(1)]",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindArrayElement
            KindArrayConstructor
              KindIntLiteral [10]
              KindIntLiteral [20]
              KindIntLiteral [30]
            KindFunctionCall
              KindPathExpression
                KindIdentifier [OFFSET]
              KindIntLiteral [1]
            KindLocation
`,
		},
		{
			name: "STRUCT with named fields",
			sql:  "SELECT STRUCT(1 AS a, 'x' AS b)",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStructConstructorWithKeyword
            KindStructConstructorArg
              KindIntLiteral [1]
              KindAlias
                KindIdentifier [a]
            KindStructConstructorArg
              KindStringLiteral [x]
                KindStringLiteralComponent
              KindAlias
                KindIdentifier [b]
`,
		},
		{
			name: "STRUCT field access via path",
			sql:  "SELECT s.a FROM (SELECT STRUCT(1 AS a) AS s) AS t",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [s]
            KindIdentifier [a]
      KindFromClause
        KindTableSubquery
          KindQuery
            KindSelect
              KindSelectList
                KindSelectColumn
                  KindStructConstructorWithKeyword
                    KindStructConstructorArg
                      KindIntLiteral [1]
                      KindAlias
                        KindIdentifier [a]
                  KindAlias
                    KindIdentifier [s]
          KindAlias
            KindIdentifier [t]
`,
		},
		{
			name: "EXISTS subquery",
			sql:  "SELECT 1 WHERE EXISTS (SELECT 1)",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral [1]
      KindWhereClause
        KindExpressionSubquery
          KindQuery
            KindSelect
              KindSelectList
                KindSelectColumn
                  KindIntLiteral [1]
`,
		},
		{
			name: "UNNEST as table source",
			sql:  "SELECT v FROM UNNEST([1, 2, 3]) AS v",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [v]
      KindFromClause
        KindTablePathExpression
          KindUnnestExpression
            KindExpressionWithOptAlias
              KindArrayConstructor
                KindIntLiteral [1]
                KindIntLiteral [2]
                KindIntLiteral [3]
          KindAlias
            KindIdentifier [v]
`,
		},
		{
			name: "CAST expression",
			sql:  "SELECT CAST('1' AS INT64)",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindCastExpression
            KindStringLiteral [1]
              KindStringLiteralComponent
            KindSimpleType
              KindPathExpression
                KindIdentifier [INT64]
`,
		},
		{
			name: "NOT BETWEEN precedence",
			sql:  "SELECT * FROM users WHERE NOT id BETWEEN 1 AND 10",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
      KindWhereClause
        KindUnaryExpression
          KindBetweenExpression
            KindPathExpression
              KindIdentifier [id]
            KindIntLiteral [1]
            KindIntLiteral [10]
            KindLocation
`,
		},
		{
			name: "window function with ORDER BY",
			sql:  "SELECT ROW_NUMBER() OVER (ORDER BY id) FROM users",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindAnalyticFunctionCall
            KindFunctionCall
              KindPathExpression
                KindIdentifier [ROW_NUMBER]
            KindWindowSpecification
              KindOrderBy
                KindOrderingExpression [UNSPECIFIED]
                  KindPathExpression
                    KindIdentifier [id]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
		},
		{
			name: "window function with PARTITION BY",
			sql:  "SELECT SUM(x) OVER (PARTITION BY id) FROM users",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindAnalyticFunctionCall
            KindFunctionCall
              KindPathExpression
                KindIdentifier [SUM]
              KindPathExpression
                KindIdentifier [x]
            KindWindowSpecification
              KindPartitionBy
                KindPathExpression
                  KindIdentifier [id]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
		},
		{name: "incomplete SELECT", sql: "SELECT", wantErr: &ParseError{}},
		{name: "missing select list", sql: "SELECT FROM users", wantErr: &ParseError{}},
		{name: "unmatched right paren", sql: "SELECT 1) FROM users", wantErr: &ParseError{}},
		{name: "unmatched left paren", sql: "SELECT (1 FROM users", wantErr: &ParseError{}},
		{name: "unclosed string literal", sql: "SELECT 'unclosed FROM users", wantErr: &ParseError{}},
		{name: "garbage non-keyword input", sql: "NOTSQL nonsense", wantErr: &ParseError{}},
		{name: "WHERE without expression", sql: "SELECT * FROM users WHERE", wantErr: &ParseError{}},
		{name: "ORDER BY without expression", sql: "SELECT * FROM users ORDER BY", wantErr: &ParseError{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestEngine(t)

			// Act
			parsed, err := sut.Parse(ctx, tt.sql)

			// Assert
			if tt.wantErr != nil {
				assert.IsType(t, tt.wantErr, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, parsed.Root.String())
		})
	}
}
