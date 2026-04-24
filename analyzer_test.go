package zetasql

import (
	"context"
	"errors"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/catalog"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
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

func TestAnalyzeNextStatement(t *testing.T) {
	ctx := context.Background()
	analyzer, err := NewAnalyzer(ctx)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}
	defer analyzer.Close(ctx)

	t.Run("three statements", func(t *testing.T) {
		sql := "SELECT 1; SELECT 2; SELECT 3"
		loc := NewParseResumeLocation(sql)
		opts := NewAnalyzerOptions()

		var outputs []*AnalyzeOutput
		for {
			output, more, err := analyzer.AnalyzeNextStatement(ctx, loc, nil, opts)
			if err != nil {
				t.Fatalf("AnalyzeNextStatement failed at position %d: %v", loc.BytePosition(), err)
			}
			outputs = append(outputs, output)
			if !more {
				break
			}
		}

		if got, want := len(outputs), 3; got != want {
			t.Fatalf("len(outputs) = %d, want %d", got, want)
		}

		// Each output should have a QueryStmt
		for i, output := range outputs {
			stmt := output.ResolvedStatement()
			if stmt == nil {
				t.Fatalf("outputs[%d].ResolvedStatement() = nil", i)
			}
			if got, want := stmt.Kind(), resolved_ast.KindQueryStmt; got != want {
				t.Errorf("outputs[%d].Kind() = %v, want %v", i, got, want)
			}
		}
	})

	t.Run("single statement", func(t *testing.T) {
		sql := "SELECT 42"
		loc := NewParseResumeLocation(sql)
		opts := NewAnalyzerOptions()

		output, more, err := analyzer.AnalyzeNextStatement(ctx, loc, nil, opts)
		if err != nil {
			t.Fatalf("AnalyzeNextStatement failed: %v", err)
		}
		if got, want := more, false; got != want {
			t.Errorf("more = %v, want %v", got, want)
		}
		if output.ResolvedStatement() == nil {
			t.Fatal("ResolvedStatement() = nil")
		}
		if got, want := loc.AtEnd(), true; got != want {
			t.Errorf("loc.AtEnd() = %v, want %v", got, want)
		}
	})

	t.Run("with table catalog", func(t *testing.T) {
		cat := catalog.NewSimpleCatalog("test")
		cat.AddZetaSQLBuiltinFunctions(nil)
		table := catalog.NewSimpleTable("users",
			catalog.NewSimpleColumn("users", "id", types.Int64Type()),
		)
		cat.AddTable(table)

		sql := "SELECT id FROM users; SELECT 1"
		loc := NewParseResumeLocation(sql)
		opts := NewAnalyzerOptions()

		output1, more1, err := analyzer.AnalyzeNextStatement(ctx, loc, cat, opts)
		if err != nil {
			t.Fatalf("first statement failed: %v", err)
		}
		if got, want := more1, true; got != want {
			t.Errorf("more after first = %v, want %v", got, want)
		}
		if output1.ResolvedStatement() == nil {
			t.Fatal("first ResolvedStatement() = nil")
		}

		output2, more2, err := analyzer.AnalyzeNextStatement(ctx, loc, cat, opts)
		if err != nil {
			t.Fatalf("second statement failed: %v", err)
		}
		if got, want := more2, false; got != want {
			t.Errorf("more after second = %v, want %v", got, want)
		}
		if output2.ResolvedStatement() == nil {
			t.Fatal("second ResolvedStatement() = nil")
		}
	})
}

func TestAnalyzeStatement_CustomFunction(t *testing.T) {
	ctx := context.Background()
	analyzer, err := NewAnalyzer(ctx)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}
	defer analyzer.Close(ctx)

	cat := catalog.NewSimpleCatalog("test")
	cat.AddZetaSQLBuiltinFunctions(nil)
	cat.AddFunction(types.NewFunction(
		[]string{"my_add"},
		"custom",
		types.ScalarMode,
		[]*types.FunctionSignature{
			types.NewFunctionSignature(
				types.NewFunctionArgumentType(types.Int64Type(), nil),
				[]*types.FunctionArgumentType{
					types.NewFunctionArgumentType(types.Int64Type(), nil),
					types.NewFunctionArgumentType(types.Int64Type(), nil),
				},
			),
		},
	))

	opts := NewAnalyzerOptions()
	output, err := analyzer.AnalyzeStatement(ctx, "SELECT my_add(1, 2)", cat, opts)
	if err != nil {
		t.Fatalf("AnalyzeStatement failed: %v", err)
	}

	stmt := output.ResolvedStatement()
	if got, want := stmt.Kind(), resolved_ast.KindQueryStmt; got != want {
		t.Errorf("Kind() = %v, want %v", got, want)
	}
}

func TestAnalyzeStatement_TemplatedFunction(t *testing.T) {
	ctx := context.Background()
	analyzer, err := NewAnalyzer(ctx)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}
	defer analyzer.Close(ctx)

	cat := catalog.NewSimpleCatalog("test")
	cat.AddZetaSQLBuiltinFunctions(nil)
	cat.AddFunction(types.NewFunction(
		[]string{"identity"},
		"custom",
		types.ScalarMode,
		[]*types.FunctionSignature{
			types.NewFunctionSignature(
				types.NewTemplatedFunctionArgumentType(generated.SignatureArgumentKind_ARG_TYPE_ANY_1, nil),
				[]*types.FunctionArgumentType{
					types.NewTemplatedFunctionArgumentType(generated.SignatureArgumentKind_ARG_TYPE_ANY_1, nil),
				},
			),
		},
	))

	opts := NewAnalyzerOptions()
	output, err := analyzer.AnalyzeStatement(ctx, "SELECT identity(42)", cat, opts)
	if err != nil {
		t.Fatalf("AnalyzeStatement failed: %v", err)
	}

	stmt := output.ResolvedStatement()
	if got, want := stmt.Kind(), resolved_ast.KindQueryStmt; got != want {
		t.Errorf("Kind() = %v, want %v", got, want)
	}
}
