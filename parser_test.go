package zetasql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParser_ParseStatement covers both the happy-path AST shape and the
// invalid-SQL error type for ParseStatement. Cases share a single fixture
// (newTestParser); errors are flagged in the table via wantErr (type
// witness) — happy cases set want only, error cases set wantErr only.
func TestParser_ParseStatement(t *testing.T) {
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
			sut := newTestParser(t)

			// Act
			parsed, err := sut.ParseStatement(ctx, tt.sql)

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
