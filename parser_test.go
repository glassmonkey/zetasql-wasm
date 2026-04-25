package zetasql

import (
	"context"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParser_ParseStatement_AST verifies AST shape via ast.DebugString
// (test helper that flattens the tree). Triangulated across multiple SQL shapes.
func TestParser_ParseStatement_AST(t *testing.T) {
	// Arrange (shared)
	ctx := context.Background()
	parser, err := NewParser(ctx)
	require.NoError(t, err)
	defer parser.Close(ctx)

	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "literal",
			sql:  "SELECT 1",
			want: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral
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
          KindIntLiteral
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			parsed, err := parser.ParseStatement(ctx, tt.sql)
			require.NoError(t, err)

			// Act
			got := ast.DebugString(parsed.Root)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParser_ParseStatement_PreservesSQL verifies that the SQL field on the
// returned Statement matches the input SQL string.
func TestParser_ParseStatement_PreservesSQL(t *testing.T) {
	// Arrange (shared)
	ctx := context.Background()
	parser, err := NewParser(ctx)
	require.NoError(t, err)
	defer parser.Close(ctx)

	tests := []struct {
		name string
		sql  string
		want string
	}{
		{name: "literal", sql: "SELECT 1", want: "SELECT 1"},
		{name: "star", sql: "SELECT * FROM users", want: "SELECT * FROM users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := parser

			// Act
			parsed, err := sut.ParseStatement(ctx, tt.sql)
			require.NoError(t, err)
			got := parsed.SQL

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParser_ParseStatement_Errors verifies that invalid SQL yields the
// expected error type. wantErr is a type witness compared via assert.IsType.
func TestParser_ParseStatement_Errors(t *testing.T) {
	// Arrange (shared)
	ctx := context.Background()
	parser, err := NewParser(ctx)
	require.NoError(t, err)
	defer parser.Close(ctx)

	tests := []struct {
		name    string
		sql     string
		wantErr error
	}{
		{name: "incomplete SELECT", sql: "SELECT", wantErr: &ParseError{}},
		{name: "missing select list", sql: "SELECT FROM users", wantErr: &ParseError{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := parser

			// Act
			_, got := sut.ParseStatement(ctx, tt.sql)

			// Assert
			assert.IsType(t, tt.wantErr, got)
		})
	}
}
