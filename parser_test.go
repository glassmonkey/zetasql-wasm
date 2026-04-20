package zetasql

import (
	"context"
	"errors"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/ast"
)

func TestParseStatement(t *testing.T) {
	ctx := context.Background()
	parser, err := NewParser(ctx)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	defer parser.Close(ctx)

	tests := []struct {
		name    string
		sql     string
		wantErr bool
		wantAST string
	}{
		{
			name: "Simple SELECT literal",
			sql:  "SELECT 1",
			wantAST: `KindQueryStatement
  KindQuery
    KindSelect
      KindSelectList
        KindSelectColumn
          KindIntLiteral
`,
		},
		{
			name: "SELECT star FROM table",
			sql:  "SELECT * FROM users",
			wantAST: `KindQueryStatement
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
			name: "SELECT with WHERE",
			sql:  "SELECT * FROM users WHERE age > 20",
			wantAST: `KindQueryStatement
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
			name: "SELECT columns from table",
			sql:  "SELECT name, email FROM customers",
			wantAST: `KindQueryStatement
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
			name:    "Invalid SQL",
			sql:     "SELECT",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := parser.ParseStatement(ctx, tt.sql)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				var parseErr *ParseError
				if !errors.As(err, &parseErr) {
					t.Fatalf("expected *ParseError, got %T: %v", err, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stmt.SQL() != tt.sql {
				t.Errorf("SQL() = %q, want %q", stmt.SQL(), tt.sql)
			}

			got := ast.DebugString(stmt.RootNode())
			if got != tt.wantAST {
				t.Errorf("AST mismatch for %q\ngot:\n%s\nwant:\n%s", tt.sql, got, tt.wantAST)
			}
		})
	}
}


