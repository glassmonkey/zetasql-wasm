package zetasql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParser_ParseStatement_AST verifies AST shape via the canonical
// String() representation defined in package ast. Triangulated across
// multiple SQL shapes.
func TestParser_ParseStatement_AST(t *testing.T) {
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
			ctx := t.Context()
			sut := newTestParser(t)

			// Act
			parsed, err := sut.ParseStatement(ctx, tt.sql)
			require.NoError(t, err)
			got := parsed.Root.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParser_ParseStatement_Errors verifies that invalid SQL yields the
// expected error type. wantErr is a type witness compared via assert.IsType.
func TestParser_ParseStatement_Errors(t *testing.T) {
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
			ctx := t.Context()
			sut := newTestParser(t)

			// Act
			_, got := sut.ParseStatement(ctx, tt.sql)

			// Assert
			assert.IsType(t, tt.wantErr, got)
		})
	}
}
