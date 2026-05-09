package zetasql

import (
	"strings"
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

// newBuiltinsCatalog returns an empty catalog (no tables, no user functions).
// Use this for SELECT queries that do not reference any table but do use
// builtin functions like CAST, COUNT, etc. — Engine.Analyze loads ZetaSQL
// builtins automatically, so an empty catalog is sufficient.
func newBuiltinsCatalog() *types.SimpleCatalog {
	return types.NewSimpleCatalog("test")
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
// and whether the analyzer marked the parameter untyped. The full
// *ParameterNode carries internal location/parent information that
// would make a direct equality assertion sensitive to fields the test
// does not actually care about; projecting to this two-field tuple
// keeps the assertion focused on the parameter-resolution contract.
type observedParam struct {
	TypeKind  generated.TypeKind
	IsUntyped bool
}

// observeParams walks the resolved tree and returns one observedParam
// per ParameterNode in tree-walk order. Returning every node (rather
// than only the first match) lets each test case assert on the full
// sequence as a single slice equality: a regression that mistypes the
// second positional parameter shows up as a slice diff instead of
// slipping past a first-match probe. Having a single observe helper
// (rather than singular/all variants) removes the discretionary call
// at each test site that would otherwise be a recurring source of
// false-negative tests.
func observeParams(root resolved_ast.Node) []observedParam {
	var got []observedParam
	_ = resolved_ast.Walk(root, func(n resolved_ast.Node) error {
		if p, ok := n.(*resolved_ast.ParameterNode); ok {
			got = append(got, observedParam{
				TypeKind:  p.Type().GetTypeKind(),
				IsUntyped: p.IsUntyped(),
			})
		}
		return nil
	})
	return got
}

// ----- Tests -----

// TestEngine_Analyze locks the full Engine.Analyze contract for a
// single statement: each happy-path case spells out both the parser
// AST (out.Parsed) and the resolved AST (out.Resolved); each error
// case names the expected error type and asserts the AnalyzeOutput
// is nil. Cases share a single fixture (newTestEngine); errors are
// flagged via wantErr (type witness) — happy cases set wantParsed +
// wantResolved only, error cases set wantErr only. Both observation
// axes (Parsed and Resolved) live on the same case so a regression
// that nils Parsed for one SQL family but not another surfaces in
// the same run that exercises Resolved-tree shape regressions.
func TestEngine_Analyze(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		cat          *types.SimpleCatalog
		opts         *AnalyzerOptions // nil → NewAnalyzerOptions()
		wantParsed   string           // parser AST string; empty when wantErr is set
		wantResolved string           // resolved AST string; empty when wantErr is set
		wantErr      error            // type witness for error cases; nil for happy path
	}{
		{
			name: "literal",
			sql:  "SELECT 1",
			cat:  nil,
			wantParsed: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral [1]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [id]
        KindSelectColumn
          KindPathExpression
            KindIdentifier [name]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
      KindWhereClause
        KindBinaryExpression
          KindPathExpression
            KindIdentifier [id]
          KindIntLiteral [1]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindBinaryExpression
            KindPathExpression
              KindIdentifier [id]
            KindIntLiteral [1]
          KindAlias
            KindIdentifier [inc]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindFunctionCall
            KindPathExpression
              KindIdentifier [UPPER]
            KindPathExpression
              KindIdentifier [name]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindPathExpression
            KindIdentifier [u]
            KindIdentifier [id]
        KindSelectColumn
          KindPathExpression
            KindIdentifier [o]
            KindIdentifier [amount]
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
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
    KindOrderBy
      KindOrderingExpression [DESC]
        KindPathExpression
          KindIdentifier [id]
`,
			wantResolved: `KindQueryStmt
  KindOutputColumn id
  KindOrderByScan
    KindTableScan users
`,
		},
		{
			name: "limit and offset",
			sql:  "SELECT id FROM users LIMIT 10 OFFSET 5",
			cat:  newUsersCatalog(),
			wantParsed: `KindQueryStatement
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
    KindLimitOffset
      KindLimit
        KindIntLiteral [10]
      KindIntLiteral [5]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
			wantResolved: `KindQueryStmt
  KindOutputColumn name
  KindAggregateScan
    KindTableScan users
`,
		},
		{
			name: "union all",
			sql:  "SELECT id FROM users UNION ALL SELECT order_id FROM orders",
			cat:  newUsersOrdersCatalog(),
			wantParsed: `KindQueryStatement
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
              KindIdentifier [order_id]
        KindFromClause
          KindTablePathExpression
            KindPathExpression
              KindIdentifier [orders]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindArrayConstructor
            KindIntLiteral [1]
            KindIntLiteral [2]
            KindIntLiteral [3]
          KindAlias
            KindIdentifier [arr]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
          KindAlias
            KindIdentifier [x]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
          KindAlias
            KindIdentifier [s]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindCastExpression
            KindStringLiteral [42]
              KindStringLiteralComponent
            KindSimpleType
              KindPathExpression
                KindIdentifier [INT64]
          KindAlias
            KindIdentifier [x]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
      KindWhereClause
        KindUnaryExpression
          KindBetweenExpression
            KindPathExpression
              KindIdentifier [id]
            KindIntLiteral [1]
            KindIntLiteral [10]
            KindLocation
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
      KindWhereClause
        KindBinaryExpression
          KindPathExpression
            KindIdentifier [name]
          KindNullLiteral [NULL]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
          KindAlias
            KindIdentifier [lbl]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindAnalyticFunctionCall
            KindFunctionCall
              KindPathExpression
                KindIdentifier [SUM]
              KindPathExpression
                KindIdentifier [id]
            KindWindowSpecification
              KindPartitionBy
                KindPathExpression
                  KindIdentifier [name]
          KindAlias
            KindIdentifier [s]
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [users]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
      KindWhereClause
        KindBinaryExpression
          KindPathExpression
            KindIdentifier [id]
          KindParameterExpr
            KindIdentifier [id]
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
      KindWhereClause
        KindBetweenExpression
          KindPathExpression
            KindIdentifier [id]
          KindParameterExpr
            KindIdentifier [lo]
          KindParameterExpr
            KindIdentifier [hi]
          KindLocation
`,
			wantResolved: `KindQueryStmt
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
			wantParsed: `KindQueryStatement
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
      KindWhereClause
        KindInExpression
          KindPathExpression
            KindIdentifier [id]
          KindUnnestExpression
            KindExpressionWithOptAlias
              KindParameterExpr
                KindIdentifier [ids]
          KindLocation
`,
			wantResolved: `KindQueryStmt
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
		// asserted in the loop body. wantParsed/wantResolved are left
		// empty because Analyze returns nil on error.
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
			require.NotNil(t, out.Parsed, "AnalyzeOutput.Parsed must be populated")
			assert.Equal(t, tt.wantParsed, out.Parsed.String())
			assert.Equal(t, tt.wantResolved, out.Resolved.String())
		})
	}
}

// TestEngine_AnalyzeNext locks the contract Engine.AnalyzeNext provides
// when consuming a multi-statement SQL string: each iteration returns a
// populated Parsed AST and Resolved AST, and the ParseResumeLocation
// has advanced to the end of the input by the time the loop exits. All
// three observations share the same fixture (engine, default
// AnalyzerOptions, joined ParseResumeLocation) so they live in a single
// test function — splitting them per observation axis would create the
// "where do I add the next case" decision the fixture-axis split rule
// is designed to prevent.
func TestEngine_AnalyzeNext(t *testing.T) {
	tests := []struct {
		name         string
		statements   []string
		cat          *types.SimpleCatalog // nil → analyzer uses no catalog
		wantParsed   []string
		wantResolved []string
		wantLoc      ParseResumeLocation
	}{
		{
			name:       "two literals",
			statements: []string{"SELECT 1", "SELECT 2"},
			wantParsed: []string{
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
			},
			wantResolved: []string{
				`KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindLiteral 1
    KindSingleRowScan
`,
				`KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindLiteral 2
    KindSingleRowScan
`,
			},
			wantLoc: ParseResumeLocation{
				Input:        "SELECT 1; SELECT 2",
				BytePosition: int32(len("SELECT 1; SELECT 2")),
			},
		},
		{
			name:       "three literals",
			statements: []string{"SELECT 1", "SELECT 2", "SELECT 3"},
			wantParsed: []string{
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
				`KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral [3]
`,
			},
			wantResolved: []string{
				`KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindLiteral 1
    KindSingleRowScan
`,
				`KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindLiteral 2
    KindSingleRowScan
`,
				`KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindLiteral 3
    KindSingleRowScan
`,
			},
			wantLoc: ParseResumeLocation{
				Input:        "SELECT 1; SELECT 2; SELECT 3",
				BytePosition: int32(len("SELECT 1; SELECT 2; SELECT 3")),
			},
		},
		{
			// Statements of different lengths and SQL families exercise
			// the multi-statement bridge's `;` boundary detection and
			// cumulative BytePosition arithmetic on input shapes that
			// the literal-only cases above do not reach.
			name:       "function call then arithmetic",
			statements: []string{"SELECT UPPER('hi')", "SELECT 1 + 2"},
			cat:        newBuiltinsCatalog(),
			wantParsed: []string{
				`KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindFunctionCall
            KindPathExpression
              KindIdentifier [UPPER]
            KindStringLiteral [hi]
              KindStringLiteralComponent
`,
				`KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindBinaryExpression
            KindIntLiteral [1]
            KindIntLiteral [2]
`,
			},
			wantResolved: []string{
				`KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindFunctionCall ZetaSQL:upper
        KindLiteral "hi"
    KindSingleRowScan
`,
				`KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindFunctionCall ZetaSQL:$add
        KindLiteral 1
        KindLiteral 2
    KindSingleRowScan
`,
			},
			wantLoc: ParseResumeLocation{
				Input:        "SELECT UPPER('hi'); SELECT 1 + 2",
				BytePosition: int32(len("SELECT UPPER('hi'); SELECT 1 + 2")),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestEngine(t)
			opts := NewAnalyzerOptions()
			loc := NewParseResumeLocation(strings.Join(tt.statements, "; "))

			// Act
			var gotParsed, gotResolved []string
			for {
				out, more, err := sut.AnalyzeNext(ctx, loc, tt.cat, opts)
				require.NoError(t, err)
				require.NotNil(t, out.Parsed, "Parsed must be populated on each AnalyzeNext call")
				gotParsed = append(gotParsed, out.Parsed.String())
				gotResolved = append(gotResolved, out.Resolved.String())
				if !more {
					break
				}
			}

			// Assert
			assert.Equal(t, tt.wantParsed, gotParsed)
			assert.Equal(t, tt.wantResolved, gotResolved)
			assert.Equal(t, tt.wantLoc, *loc)
		})
	}
}

// TestEngine_ParseNext locks the contract Engine.ParseNext provides
// when consuming a multi-statement SQL string: each iteration returns
// a populated parser AST, and the ParseResumeLocation has advanced to
// the end of the input by the time the loop exits. Both observations
// share the same fixture (engine + ParseResumeLocation) so they live
// in a single test function — the same fixture-axis split rule that
// keeps TestEngine_AnalyzeNext unified. Expected AST strings are
// hardcoded per statement (rather than derived from sut.Parse on each
// substring) so each case reads as a specification of what each
// parser AST payload must look like; deriving from another SUT call
// has a vacuous-green failure mode if both endpoints degenerate to
// the same empty/identical output.
//
// Triangulated across statement count (2/3), the realistic
// upstream-failure script (DDL+DML+DQL with trailing semicolon — the
// exact shape that surfaces as "Expected end of input but got keyword
// CREATE/INSERT" when fed to single-statement Engine.Parse), and
// trailing-semicolon presence.
func TestEngine_ParseNext(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantParsed []string
		wantLoc    ParseResumeLocation
	}{
		{
			name: "two literals",
			sql:  "SELECT 1; SELECT 2",
			wantParsed: []string{
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
			},
			wantLoc: ParseResumeLocation{
				Input:        "SELECT 1; SELECT 2",
				BytePosition: int32(len("SELECT 1; SELECT 2")),
			},
		},
		{
			name: "three literals",
			sql:  "SELECT 1; SELECT 2; SELECT 3",
			wantParsed: []string{
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
				`KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral [3]
`,
			},
			wantLoc: ParseResumeLocation{
				Input:        "SELECT 1; SELECT 2; SELECT 3",
				BytePosition: int32(len("SELECT 1; SELECT 2; SELECT 3")),
			},
		},
		{
			name: "ddl dml dql with trailing semicolon",
			sql:  "CREATE TABLE t1 (id INT64); INSERT INTO t1 VALUES (1); SELECT * FROM t1;",
			wantParsed: []string{
				`KindCreateTableStatement
  KindPathExpression
    KindIdentifier [t1]
  KindTableElementList
    KindColumnDefinition
      KindIdentifier [id]
      KindSimpleColumnSchema
        KindPathExpression
          KindIdentifier [INT64]
`,
				`KindInsertStatement
  KindPathExpression
    KindIdentifier [t1]
  KindInsertValuesRowList
    KindInsertValuesRow
      KindIntLiteral [1]
`,
				`KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindStar
      KindFromClause
        KindTablePathExpression
          KindPathExpression
            KindIdentifier [t1]
`,
			},
			wantLoc: ParseResumeLocation{
				Input:        "CREATE TABLE t1 (id INT64); INSERT INTO t1 VALUES (1); SELECT * FROM t1;",
				BytePosition: int32(len("CREATE TABLE t1 (id INT64); INSERT INTO t1 VALUES (1); SELECT * FROM t1;")),
			},
		},
		{
			name: "two selects with trailing semicolon",
			sql:  "SELECT 1; SELECT 2;",
			wantParsed: []string{
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
			},
			wantLoc: ParseResumeLocation{
				Input:        "SELECT 1; SELECT 2;",
				BytePosition: int32(len("SELECT 1; SELECT 2;")),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestEngine(t)
			loc := NewParseResumeLocation(tt.sql)

			// Act
			var gotParsed []string
			for {
				stmt, more, err := sut.ParseNext(ctx, loc)
				require.NoError(t, err)
				require.NotNil(t, stmt.Root, "Root must be populated on each ParseNext call")
				gotParsed = append(gotParsed, stmt.Root.String())
				if !more {
					break
				}
			}

			// Assert
			assert.Equal(t, tt.wantParsed, gotParsed)
			assert.Equal(t, tt.wantLoc, *loc)
		})
	}
}

// TestEngine_Analyze_CustomFunctions verifies that a user-defined
// scalar function registered in the catalog is resolved to the
// expected FunctionCall in the AST, across both fixed-signature and
// templated (ARG_TYPE_ANY_1) function shapes. Both branches share
// the same fixture (newTestEngine + a per-case catalog with the
// declared function appended after the ZetaSQL builtins) and the
// same observation (the resolved AST string), differing only in how
// the FunctionArgumentType return/arg shape is built — so per the
// fixture-axis split rule the two shapes live in one table. The
// returnType + argTypes fields carry the raw FunctionArgumentType
// values for each case, letting templated and concrete signatures
// sit alongside each other without per-branch construction logic in
// the test body.
func TestEngine_Analyze_CustomFunctions(t *testing.T) {
	int64Arg := types.NewFunctionArgumentType(types.Int64Type())
	any1Arg := types.NewTemplatedFunctionArgumentType(types.ArgTypeAny1)

	tests := []struct {
		name       string
		funcName   string
		group      string
		returnType *types.FunctionArgumentType
		argTypes   []*types.FunctionArgumentType
		sql        string
		want       string
	}{
		{
			name:       "two int64 args in custom group",
			funcName:   "my_add",
			group:      "custom",
			returnType: int64Arg,
			argTypes:   []*types.FunctionArgumentType{int64Arg, int64Arg},
			sql:        "SELECT my_add(1, 2)",
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
			name:       "single int64 arg in math group",
			funcName:   "double",
			group:      "math",
			returnType: int64Arg,
			argTypes:   []*types.FunctionArgumentType{int64Arg},
			sql:        "SELECT double(7)",
			want: `KindQueryStmt
  KindOutputColumn $col1
  KindProjectScan
    KindComputedColumn
      KindFunctionCall math:double
        KindLiteral 7
    KindSingleRowScan
`,
		},
		{
			name:       "Templated identity in custom group",
			funcName:   "identity",
			group:      "custom",
			returnType: any1Arg,
			argTypes:   []*types.FunctionArgumentType{any1Arg},
			sql:        "SELECT identity(42)",
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
			name:       "Templated negate in math group",
			funcName:   "negate",
			group:      "math",
			returnType: any1Arg,
			argTypes:   []*types.FunctionArgumentType{any1Arg},
			sql:        "SELECT negate(7)",
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
			cat.Functions = append(cat.Functions, types.NewFunction(
				[]string{tt.funcName},
				tt.group,
				types.ScalarMode,
				[]*types.FunctionSignature{
					types.NewFunctionSignature(tt.returnType, tt.argTypes),
				},
			))
			sut := newTestEngine(t)

			// Act
			out, err := sut.Analyze(ctx, tt.sql, cat, NewAnalyzerOptions())
			require.NoError(t, err)

			// Assert
			assert.Equal(t, tt.want, out.Resolved.String())
		})
	}
}

// TestEngine_Analyze_Parameters covers Engine.Analyze parameter
// resolution end to end across both ParameterMode values. Positional
// and named parameter cases share the same fixture (newTestEngine +
// newQueryStmtAnalyzerOptions) and the same observation (the
// ParameterNode sequence in the resolved tree, projected via
// observeParams) — the only differences are which mode-specific
// fields on AnalyzerOptions get populated and which catalog the SQL
// references. Combining them in one table preserves the fixture-axis
// split rule: same SUT method on the same fixture lives in one test.
//
// Triangulated across:
//   - Positional: declared 1 INT64 / 2 (STRING, INT64) resolve
//     cleanly; count mismatches (none declared but SQL uses one /
//     one declared but SQL uses two) reject.
//   - Named: declared @id (INT64) / @label (STRING) resolve cleanly;
//     undeclared @nope rejects under the strict default; same @nope
//     and @label are accepted as untyped INT64 when
//     AllowUndeclaredParameters is true.
//
// SQL inputs follow real-world shapes (`WHERE id = ?`, `SELECT @id`)
// so a regression that only affects nested-position `?` usage or
// drops named parameter wiring surfaces.
//
// allowUndeclared is meaningful only when paramMode is
// ParameterNamed; positional cases leave it false. The named branch
// guards against the v0.5.0 emulator boot bug where the Go wrap had
// no QueryParameters field and the WASM bridge dropped the proto
// fields even when populated.
func TestEngine_Analyze_Parameters(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		cat              *types.SimpleCatalog
		paramMode        ParameterMode
		namedParams      map[string]types.Type // populated when paramMode is ParameterNamed
		positionalParams []types.Type          // populated when paramMode is ParameterPositional
		allowUndeclared  bool                  // meaningful only for ParameterNamed
		wantParams       []observedParam
		wantErr          error
	}{
		// Positional cases.
		{
			name:             "Positional: single INT64 in WHERE",
			sql:              "SELECT id FROM users WHERE id = ?",
			cat:              newUsersCatalog(),
			paramMode:        ParameterPositional,
			positionalParams: []types.Type{types.Int64Type()},
			wantParams:       []observedParam{{TypeKind: generated.TypeKind_TYPE_INT64}},
		},
		{
			name:             "Positional: two (STRING, INT64) in WHERE",
			sql:              "SELECT id FROM users WHERE name = ? AND id = ?",
			cat:              newUsersCatalog(),
			paramMode:        ParameterPositional,
			positionalParams: []types.Type{types.StringType(), types.Int64Type()},
			wantParams: []observedParam{
				{TypeKind: generated.TypeKind_TYPE_STRING},
				{TypeKind: generated.TypeKind_TYPE_INT64},
			},
		},
		{
			name:      "Positional: none declared but SQL uses one",
			sql:       "SELECT id FROM users WHERE id = ?",
			cat:       newUsersCatalog(),
			paramMode: ParameterPositional,
			wantErr:   &AnalyzeError{},
		},
		{
			name:             "Positional: one declared but SQL uses two",
			sql:              "SELECT id FROM users WHERE name = ? AND id = ?",
			cat:              newUsersCatalog(),
			paramMode:        ParameterPositional,
			positionalParams: []types.Type{types.Int64Type()},
			wantErr:          &AnalyzeError{},
		},
		// Named cases.
		{
			name:        "Named: @id resolves to declared INT64",
			sql:         "SELECT @id",
			paramMode:   ParameterNamed,
			namedParams: map[string]types.Type{"id": types.Int64Type()},
			wantParams:  []observedParam{{TypeKind: generated.TypeKind_TYPE_INT64}},
		},
		{
			name:        "Named: @label resolves to declared STRING",
			sql:         "SELECT @label",
			paramMode:   ParameterNamed,
			namedParams: map[string]types.Type{"label": types.StringType()},
			wantParams:  []observedParam{{TypeKind: generated.TypeKind_TYPE_STRING}},
		},
		{
			name:      "Named: undeclared @nope rejected under strict mode",
			sql:       "SELECT @nope",
			paramMode: ParameterNamed,
			wantErr:   &AnalyzeError{},
		},
		{
			name:        "Named: @id declared but @other referenced",
			sql:         "SELECT @other",
			paramMode:   ParameterNamed,
			namedParams: map[string]types.Type{"id": types.Int64Type()},
			wantErr:     &AnalyzeError{},
		},
		// Analyzer assigns the default INT64 to undeclared params in
		// permissive mode but flags IsUntyped so callers can tell it
		// apart from a declared INT64.
		{
			name:            "Named: AllowUndeclared resolves @nope as untyped INT64",
			sql:             "SELECT @nope",
			paramMode:       ParameterNamed,
			allowUndeclared: true,
			wantParams:      []observedParam{{TypeKind: generated.TypeKind_TYPE_INT64, IsUntyped: true}},
		},
		{
			name:            "Named: AllowUndeclared resolves @label as untyped INT64",
			sql:             "SELECT @label",
			paramMode:       ParameterNamed,
			allowUndeclared: true,
			wantParams:      []observedParam{{TypeKind: generated.TypeKind_TYPE_INT64, IsUntyped: true}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			sut := newTestEngine(t)
			opts := newQueryStmtAnalyzerOptions()
			opts.ParameterMode = tt.paramMode
			opts.QueryParameters = tt.namedParams
			opts.PositionalQueryParameters = tt.positionalParams
			opts.AllowUndeclaredParameters = tt.allowUndeclared

			// Act
			got, err := sut.Analyze(ctx, tt.sql, tt.cat, opts)

			// Assert
			if tt.wantErr != nil {
				assert.IsType(t, tt.wantErr, err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantParams, observeParams(got.Resolved))
		})
	}
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
			name: "CREATE TABLE DDL",
			sql:  "CREATE TABLE t1 (id INT64, name STRING)",
			want: `KindCreateTableStatement
  KindPathExpression
    KindIdentifier [t1]
  KindTableElementList
    KindColumnDefinition
      KindIdentifier [id]
      KindSimpleColumnSchema
        KindPathExpression
          KindIdentifier [INT64]
    KindColumnDefinition
      KindIdentifier [name]
      KindSimpleColumnSchema
        KindPathExpression
          KindIdentifier [STRING]
`,
		},
		{
			// Triangulates the AST wrapper's parent-chain inheritance:
			// OptionsList lives on ASTCreateTableStmtBaseProto (the
			// abstract parent), distinct from Name and TableElementList
			// asserted in the case above. A regression that wired the
			// parent walk for one inherited field but not another
			// surfaces here.
			name: "CREATE TABLE DDL with OPTIONS",
			sql:  "CREATE TABLE t1 (id INT64) OPTIONS (description = 'foo')",
			want: `KindCreateTableStatement
  KindPathExpression
    KindIdentifier [t1]
  KindTableElementList
    KindColumnDefinition
      KindIdentifier [id]
      KindSimpleColumnSchema
        KindPathExpression
          KindIdentifier [INT64]
  KindOptionsList
    KindOptionsEntry
      KindIdentifier [description]
      KindStringLiteral [foo]
        KindStringLiteralComponent
`,
		},
		// CREATE TABLE flag triangulation. IsOrReplace, IsIfNotExists,
		// and Scope live on the depth-2 ancestor (CreateStatement) of
		// CreateTableStatement; nodeScalar surfaces them as bracketed
		// annotations on the KindCreateTableStatement line so the
		// inherited-field contract is observable through the same tree
		// String() path the rest of the suite uses.
		{
			name: "CREATE OR REPLACE TABLE",
			sql:  "CREATE OR REPLACE TABLE t1 (id INT64)",
			want: `KindCreateTableStatement [OR REPLACE]
  KindPathExpression
    KindIdentifier [t1]
  KindTableElementList
    KindColumnDefinition
      KindIdentifier [id]
      KindSimpleColumnSchema
        KindPathExpression
          KindIdentifier [INT64]
`,
		},
		{
			name: "CREATE TABLE IF NOT EXISTS",
			sql:  "CREATE TABLE IF NOT EXISTS t1 (id INT64)",
			want: `KindCreateTableStatement [IF NOT EXISTS]
  KindPathExpression
    KindIdentifier [t1]
  KindTableElementList
    KindColumnDefinition
      KindIdentifier [id]
      KindSimpleColumnSchema
        KindPathExpression
          KindIdentifier [INT64]
`,
		},
		{
			name: "CREATE TEMPORARY TABLE",
			sql:  "CREATE TEMPORARY TABLE t1 (id INT64)",
			want: `KindCreateTableStatement [TEMPORARY]
  KindPathExpression
    KindIdentifier [t1]
  KindTableElementList
    KindColumnDefinition
      KindIdentifier [id]
      KindSimpleColumnSchema
        KindPathExpression
          KindIdentifier [INT64]
`,
		},
		{
			name: "CREATE OR REPLACE TEMPORARY TABLE (combined flags)",
			sql:  "CREATE OR REPLACE TEMPORARY TABLE t1 (id INT64)",
			want: `KindCreateTableStatement [TEMPORARY, OR REPLACE]
  KindPathExpression
    KindIdentifier [t1]
  KindTableElementList
    KindColumnDefinition
      KindIdentifier [id]
      KindSimpleColumnSchema
        KindPathExpression
          KindIdentifier [INT64]
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

