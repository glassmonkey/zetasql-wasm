package zetasql

import (
	"context"
	"errors"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/catalog"
	"github.com/glassmonkey/zetasql-wasm/types"
)

func TestAnalyzeStatement(t *testing.T) {
	ctx := context.Background()
	analyzer, err := NewAnalyzer(ctx)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}
	defer analyzer.Close(ctx)

	tests := []struct {
		name    string
		sql     string
		catalog *catalog.SimpleCatalog
		wantErr bool
	}{
		{
			name:    "SELECT literal without catalog",
			sql:     "SELECT 1",
			catalog: nil,
			wantErr: false,
		},
		{
			name: "SELECT columns from table",
			sql:  "SELECT id, name FROM users",
			catalog: func() *catalog.SimpleCatalog {
				cat := catalog.NewSimpleCatalog("test")
				cat.AddZetaSQLBuiltinFunctions(nil)
				table := catalog.NewSimpleTable("users",
					catalog.NewSimpleColumn("users", "id", types.Int64Type()),
					catalog.NewSimpleColumn("users", "name", types.StringType()),
				)
				cat.AddTable(table)
				return cat
			}(),
			wantErr: false,
		},
		{
			name: "SELECT star from table",
			sql:  "SELECT * FROM users",
			catalog: func() *catalog.SimpleCatalog {
				cat := catalog.NewSimpleCatalog("test")
				cat.AddZetaSQLBuiltinFunctions(nil)
				table := catalog.NewSimpleTable("users",
					catalog.NewSimpleColumn("users", "id", types.Int64Type()),
					catalog.NewSimpleColumn("users", "name", types.StringType()),
				)
				cat.AddTable(table)
				return cat
			}(),
			wantErr: false,
		},
		{
			name:    "Invalid SQL",
			sql:     "SELECT",
			catalog: nil,
			wantErr: true,
		},
		{
			name: "Table not found",
			sql:  "SELECT id FROM nonexistent",
			catalog: func() *catalog.SimpleCatalog {
				cat := catalog.NewSimpleCatalog("test")
				cat.AddZetaSQLBuiltinFunctions(nil)
				return cat
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewAnalyzerOptions()
			output, err := analyzer.AnalyzeStatement(ctx, tt.sql, tt.catalog, opts)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				var analyzeErr *AnalyzeError
				if !errors.As(err, &analyzeErr) {
					t.Fatalf("expected *AnalyzeError, got %T: %v", err, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output == nil {
				t.Fatal("expected non-nil output")
			}

			resolved := output.ResolvedStatement()
			if resolved == nil {
				t.Fatal("expected non-nil resolved statement")
			}
		})
	}
}
