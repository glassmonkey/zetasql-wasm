package zetasql

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/catalog"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestAnalyzer(t)

			// Act
			out, err := sut.AnalyzeStatement(ctx, tt.sql, tt.cat, NewAnalyzerOptions())
			require.NoError(t, err)
			got := out.Statement.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
