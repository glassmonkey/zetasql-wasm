package zetasql

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/catalog"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----- Test helpers (shared with analyzer_test.go) -----

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

func newUsersCatalog() *catalog.SimpleCatalog {
	cat := catalog.NewSimpleCatalog("test")
	cat.AddZetaSQLBuiltinFunctions(nil)
	cat.Tables = append(cat.Tables, catalog.NewSimpleTable("users",
		catalog.NewSimpleColumn("users", "id", types.Int64Type()),
		catalog.NewSimpleColumn("users", "name", types.StringType()),
	))
	return cat
}

func newUsersOrdersCatalog() *catalog.SimpleCatalog {
	cat := newUsersCatalog()
	cat.Tables = append(cat.Tables, catalog.NewSimpleTable("orders",
		catalog.NewSimpleColumn("orders", "order_id", types.Int64Type()),
		catalog.NewSimpleColumn("orders", "user_id", types.Int64Type()),
		catalog.NewSimpleColumn("orders", "amount", types.Int64Type()),
	))
	return cat
}

// newBuiltinsCatalog returns a catalog with only the ZetaSQL builtin
// functions registered (no tables). Use this for SELECT queries that do not
// reference any table but do use builtin functions like CAST, COUNT, etc.
func newBuiltinsCatalog() *catalog.SimpleCatalog {
	cat := catalog.NewSimpleCatalog("test")
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
		cat  *catalog.SimpleCatalog
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
