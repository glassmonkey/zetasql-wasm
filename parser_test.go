package zetasql

import (
	"context"
	"testing"
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
		wantAST string // Expected AST output from ZetaSQL (exact match)
	}{
		{
			name:    "Simple SELECT",
			sql:     "SELECT * FROM users",
			wantErr: false,
			wantAST: `QueryStatement [0-19]
  Query [0-19]
    Select [0-19]
      SelectList [7-8]
        SelectColumn [7-8]
          Star(*) [7-8]
      FromClause [9-19]
        TablePathExpression [14-19]
          PathExpression [14-19]
            Identifier(users) [14-19]
`,
		},
		{
			name:    "SELECT with WHERE",
			sql:     "SELECT * FROM users WHERE age > 20",
			wantErr: false,
			wantAST: `QueryStatement [0-34]
  Query [0-34]
    Select [0-34]
      SelectList [7-8]
        SelectColumn [7-8]
          Star(*) [7-8]
      FromClause [9-19]
        TablePathExpression [14-19]
          PathExpression [14-19]
            Identifier(users) [14-19]
      WhereClause [20-34]
        BinaryExpression(>) [26-34]
          PathExpression [26-29]
            Identifier(age) [26-29]
          IntLiteral(20) [32-34]
`,
		},
		{
			name:    "SELECT with JOIN",
			sql:     "SELECT u.name, o.total FROM users u JOIN orders o ON u.id = o.user_id",
			wantErr: false,
			wantAST: `QueryStatement [0-69]
  Query [0-69]
    Select [0-69]
      SelectList [7-22]
        SelectColumn [7-13]
          PathExpression [7-13]
            Identifier(u) [7-8]
            Identifier(name) [9-13]
        SelectColumn [15-22]
          PathExpression [15-22]
            Identifier(o) [15-16]
            Identifier(total) [17-22]
      FromClause [23-69]
        Join [28-69]
          TablePathExpression [28-35]
            PathExpression [28-33]
              Identifier(users) [28-33]
            Alias [34-35]
              Identifier(u) [34-35]
          Location [36-40]
          TablePathExpression [41-49]
            PathExpression [41-47]
              Identifier(orders) [41-47]
            Alias [48-49]
              Identifier(o) [48-49]
          OnClause [50-69]
            BinaryExpression(=) [53-69]
              PathExpression [53-57]
                Identifier(u) [53-54]
                Identifier(id) [55-57]
              PathExpression [60-69]
                Identifier(o) [60-61]
                Identifier(user_id) [62-69]
`,
		},
		{
			name:    "SELECT specific columns",
			sql:     "SELECT name, email, age FROM customers",
			wantErr: false,
			wantAST: `QueryStatement [0-38]
  Query [0-38]
    Select [0-38]
      SelectList [7-23]
        SelectColumn [7-11]
          PathExpression [7-11]
            Identifier(name) [7-11]
        SelectColumn [13-18]
          PathExpression [13-18]
            Identifier(email) [13-18]
        SelectColumn [20-23]
          PathExpression [20-23]
            Identifier(age) [20-23]
      FromClause [24-38]
        TablePathExpression [29-38]
          PathExpression [29-38]
            Identifier(customers) [29-38]
`,
		},
		{
			name:    "Invalid SQL - missing FROM",
			sql:     "SELECT",
			wantErr: true,
			wantAST: "",
		},
		{
			name:    "UTF-8 Japanese",
			sql:     "SELECT '日本語テスト'",
			wantErr: false,
			wantAST: `QueryStatement [0-27]
  Query [0-27]
    Select [0-27]
      SelectList [7-27]
        SelectColumn [7-27]
          StringLiteral [7-27]
            StringLiteralComponent('日本語テスト') [7-27]
`,
		},
		{
			name:    "UTF-8 Emoji",
			sql:     "SELECT '🔥 test'",
			wantErr: false,
			wantAST: `QueryStatement [0-18]
  Query [0-18]
    Select [0-18]
      SelectList [7-18]
        SelectColumn [7-18]
          StringLiteral [7-18]
            StringLiteralComponent('🔥 test') [7-18]
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := parser.ParseStatement(ctx, tt.sql)
			if err != nil {
				t.Fatalf("ParseStatement returned error: %v", err)
			}

			if stmt == nil {
				t.Fatal("ParseStatement returned nil statement")
			}

			if stmt.SQL != tt.sql {
				t.Errorf("Statement.SQL = %q, want %q", stmt.SQL, tt.sql)
			}

			if tt.wantErr {
				if stmt.Parsed {
					t.Errorf("Expected parsing to fail for SQL: %s", tt.sql)
				}
				if stmt.Error == "" {
					t.Error("Expected error message but got empty string")
				}
				t.Logf("Got expected error: %s", stmt.Error)
			} else {
				if !stmt.Parsed {
					t.Errorf("Expected parsing to succeed for SQL: %s, got error: %s", tt.sql, stmt.Error)
				}
				if stmt.AST == "" {
					t.Error("Expected non-empty AST")
				}

				// Exact match check for AST
				if stmt.AST != tt.wantAST {
					t.Errorf("AST mismatch for SQL: %s\nGot:\n%s\nWant:\n%s", tt.sql, stmt.AST, tt.wantAST)
				}
			}
		})
	}
}

func TestParserClose(t *testing.T) {
	ctx := context.Background()
	parser, err := NewParser(ctx)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Parse a simple statement
	stmt, err := parser.ParseStatement(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("ParseStatement failed: %v", err)
	}
	if !stmt.Parsed {
		t.Errorf("Expected parsing to succeed, got error: %s", stmt.Error)
	}

	// Close the parser
	if err := parser.Close(ctx); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Trying to parse after close should fail
	_, err = parser.ParseStatement(ctx, "SELECT 1")
	if err == nil {
		t.Error("Expected error when parsing after close, got nil")
	}
}
