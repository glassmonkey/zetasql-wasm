package zetasql

import (
	"context"
	"errors"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/catalog"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
)

// TestAnalyzeStatement verifies that AnalyzeStatement produces a correct
// resolved AST for valid SQL statements and returns typed errors for invalid ones.
// Two literal values and two column queries triangulate that the analyzer
// resolves distinct inputs to distinct outputs.
func TestAnalyzeStatement(t *testing.T) {
	a := newTestAnalyzer(t)
	ctx := context.Background()

	t.Run("SELECT literal resolves to QueryStmt with int64 literal value", func(t *testing.T) {
		type payload struct {
			ColumnCount  int
			LiteralValue int64
		}
		tests := []struct {
			name string
			sql  string
			want payload
		}{
			{name: "literal 1", sql: "SELECT 1", want: payload{ColumnCount: 1, LiteralValue: 1}},
			{name: "literal 42", sql: "SELECT 42", want: payload{ColumnCount: 1, LiteralValue: 42}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				opts := NewAnalyzerOptions()
				output, err := a.AnalyzeStatement(ctx, tt.sql, nil, opts)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				stmt := output.ResolvedStatement().(*resolved_ast.QueryStmtNode)
				literal := findNode[*resolved_ast.LiteralNode](t, stmt)
				got := payload{
					ColumnCount:  len(stmt.OutputColumnList()),
					LiteralValue: literal.Value().GetValue().GetInt64Value(),
				}
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			})
		}
	})

	t.Run("SELECT columns resolves table scan with correct column names", func(t *testing.T) {
		type columnPayload struct {
			Name string
		}
		type payload struct {
			Columns   []columnPayload
			TableName string
		}
		tests := []struct {
			name string
			sql  string
			cat  *catalog.SimpleCatalog
			want payload
		}{
			{
				name: "two columns from users",
				sql:  "SELECT id, name FROM users",
				cat:  newUsersCatalog(),
				want: payload{
					Columns:   []columnPayload{{Name: "id"}, {Name: "name"}},
					TableName: "users",
				},
			},
			{
				name: "single column from orders",
				sql:  "SELECT order_id FROM orders",
				cat:  newUsersOrdersCatalog(),
				want: payload{
					Columns:   []columnPayload{{Name: "order_id"}},
					TableName: "orders",
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				opts := NewAnalyzerOptions()
				output, err := a.AnalyzeStatement(ctx, tt.sql, tt.cat, opts)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				stmt := output.ResolvedStatement().(*resolved_ast.QueryStmtNode)
				cols := stmt.OutputColumnList()
				var gotCols []columnPayload
				for _, c := range cols {
					gotCols = append(gotCols, columnPayload{Name: c.Name()})
				}
				tableScan := findNode[*resolved_ast.TableScanNode](t, stmt)
				got := payload{
					Columns:   gotCols,
					TableName: tableScan.Table().GetName(),
				}
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			})
		}
	})

	t.Run("Invalid SQL returns typed AnalyzeError", func(t *testing.T) {
		tests := []struct {
			name string
			sql  string
			cat  *catalog.SimpleCatalog
		}{
			{name: "incomplete SELECT", sql: "SELECT", cat: nil},
			{
				name: "table not found",
				sql:  "SELECT id FROM nonexistent",
				cat: func() *catalog.SimpleCatalog {
					c := catalog.NewSimpleCatalog("test")
					c.AddZetaSQLBuiltinFunctions(nil)
					return c
				}(),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				opts := NewAnalyzerOptions()
				_, err := a.AnalyzeStatement(ctx, tt.sql, tt.cat, opts)
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				var analyzeErr *AnalyzeError
				if !errors.As(err, &analyzeErr) {
					t.Fatalf("expected *AnalyzeError, got %T: %v", err, err)
				}
			})
		}
	})
}

// TestAnalyzeNextStatement verifies that multi-statement SQL strings are
// parsed incrementally: each call resolves exactly one statement with the
// correct literal value, BytePosition advances, and AtEnd reflects completion.
func TestAnalyzeNextStatement(t *testing.T) {
	a := newTestAnalyzer(t)
	ctx := context.Background()

	t.Run("each statement resolves to a distinct literal value", func(t *testing.T) {
		type stmtPayload struct {
			LiteralValue int64
			More         bool
		}
		tests := []struct {
			name string
			sql  string
			want []stmtPayload
		}{
			{
				name: "three literals",
				sql:  "SELECT 1; SELECT 2; SELECT 3",
				want: []stmtPayload{
					{LiteralValue: 1, More: true},
					{LiteralValue: 2, More: true},
					{LiteralValue: 3, More: false},
				},
			},
			{
				name: "two literals",
				sql:  "SELECT 100; SELECT 200",
				want: []stmtPayload{
					{LiteralValue: 100, More: true},
					{LiteralValue: 200, More: false},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				loc := NewParseResumeLocation(tt.sql)
				opts := NewAnalyzerOptions()

				var got []stmtPayload
				for i := 0; i < len(tt.want); i++ {
					output, more, err := a.AnalyzeNextStatement(ctx, loc, nil, opts)
					if err != nil {
						t.Fatalf("statement %d: %v", i, err)
					}
					stmt := output.ResolvedStatement().(*resolved_ast.QueryStmtNode)
					literal := findNode[*resolved_ast.LiteralNode](t, stmt)
					got = append(got, stmtPayload{
						LiteralValue: literal.Value().GetValue().GetInt64Value(),
						More:         more,
					})
				}
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
				if !loc.AtEnd() {
					t.Error("loc.AtEnd() = false, want true")
				}
			})
		}
	})

	t.Run("BytePosition advances monotonically between statements", func(t *testing.T) {
		sql := "SELECT 1; SELECT 2"
		loc := NewParseResumeLocation(sql)
		opts := NewAnalyzerOptions()

		type posPayload struct {
			AdvancedFromPrevious bool
		}

		prevPos := loc.BytePosition()
		var got []posPayload
		for i := 0; i < 2; i++ {
			_, _, err := a.AnalyzeNextStatement(ctx, loc, nil, opts)
			if err != nil {
				t.Fatalf("statement %d: %v", i, err)
			}
			curPos := loc.BytePosition()
			got = append(got, posPayload{AdvancedFromPrevious: curPos > prevPos})
			prevPos = curPos
		}
		want := []posPayload{
			{AdvancedFromPrevious: true},
			{AdvancedFromPrevious: true},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("(-want +got):\n%s", diff)
		}
		if !loc.AtEnd() {
			t.Error("loc.AtEnd() = false, want true")
		}
	})

	t.Run("mixed statement types resolve correctly with catalog", func(t *testing.T) {
		cat := newUsersCatalog()
		sql := "SELECT id FROM users; SELECT 99"
		loc := NewParseResumeLocation(sql)
		opts := NewAnalyzerOptions()

		// First: table query
		output1, more1, err := a.AnalyzeNextStatement(ctx, loc, cat, opts)
		if err != nil {
			t.Fatalf("first statement: %v", err)
		}
		type tableStmtPayload struct {
			More       bool
			TableName  string
			ColumnName string
		}
		tableScan := findNode[*resolved_ast.TableScanNode](t, output1.ResolvedStatement())
		cols := output1.ResolvedStatement().(*resolved_ast.QueryStmtNode).OutputColumnList()
		gotFirst := tableStmtPayload{
			More:       more1,
			TableName:  tableScan.Table().GetName(),
			ColumnName: cols[0].Name(),
		}
		wantFirst := tableStmtPayload{More: true, TableName: "users", ColumnName: "id"}
		if diff := cmp.Diff(wantFirst, gotFirst); diff != "" {
			t.Errorf("first statement (-want +got):\n%s", diff)
		}

		// Second: literal
		output2, more2, err := a.AnalyzeNextStatement(ctx, loc, cat, opts)
		if err != nil {
			t.Fatalf("second statement: %v", err)
		}
		type literalStmtPayload struct {
			More         bool
			LiteralValue int64
		}
		literal := findNode[*resolved_ast.LiteralNode](t, output2.ResolvedStatement())
		gotSecond := literalStmtPayload{
			More:         more2,
			LiteralValue: literal.Value().GetValue().GetInt64Value(),
		}
		wantSecond := literalStmtPayload{More: false, LiteralValue: 99}
		if diff := cmp.Diff(wantSecond, gotSecond); diff != "" {
			t.Errorf("second statement (-want +got):\n%s", diff)
		}
	})
}

// TestAnalyzeStatement_CustomFunction verifies that user-defined scalar
// functions registered in the catalog are resolved in the AST with the correct
// group:name and typed arguments. Two distinct functions triangulate that
// name/group resolution works generally, not just for one case.
func TestAnalyzeStatement_CustomFunction(t *testing.T) {
	a := newTestAnalyzer(t)
	ctx := context.Background()

	type payload struct {
		FuncName string
		ArgVals  []int64
	}

	tests := []struct {
		name     string
		funcName string
		group    string
		sql      string
		want     payload
	}{
		{
			name:     "two int64 arguments",
			funcName: "my_add",
			group:    "custom",
			sql:      "SELECT my_add(1, 2)",
			want:     payload{FuncName: "custom:my_add", ArgVals: []int64{1, 2}},
		},
		{
			name:     "single int64 argument with different group",
			funcName: "double",
			group:    "math",
			sql:      "SELECT double(7)",
			want:     payload{FuncName: "math:double", ArgVals: []int64{7}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat := catalog.NewSimpleCatalog("test")
			cat.AddZetaSQLBuiltinFunctions(nil)

			argTypes := make([]*types.FunctionArgumentType, len(tt.want.ArgVals))
			for i := range argTypes {
				argTypes[i] = types.NewFunctionArgumentType(types.Int64Type(), nil)
			}
			cat.AddFunction(types.NewFunction(
				[]string{tt.funcName},
				tt.group,
				types.ScalarMode,
				[]*types.FunctionSignature{
					types.NewFunctionSignature(
						types.NewFunctionArgumentType(types.Int64Type(), nil),
						argTypes,
					),
				},
			))

			opts := NewAnalyzerOptions()
			output, err := a.AnalyzeStatement(ctx, tt.sql, cat, opts)
			if err != nil {
				t.Fatalf("AnalyzeStatement failed: %v", err)
			}

			stmt := output.ResolvedStatement().(*resolved_ast.QueryStmtNode)
			funcCall := findNode[*resolved_ast.FunctionCallNode](t, stmt)

			var gotArgVals []int64
			for _, arg := range funcCall.ArgumentList() {
				lit := arg.(*resolved_ast.LiteralNode)
				gotArgVals = append(gotArgVals, lit.Value().GetValue().GetInt64Value())
			}
			got := payload{
				FuncName: funcCall.Function().GetName(),
				ArgVals:  gotArgVals,
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// TestAnalyzeStatement_TemplatedFunction verifies that templated functions
// (ARG_TYPE_ANY_1) are resolved with concrete argument types inferred from the
// call site, and function names include the group prefix. Two functions with
// different names and groups triangulate that resolution is general.
func TestAnalyzeStatement_TemplatedFunction(t *testing.T) {
	a := newTestAnalyzer(t)
	ctx := context.Background()

	type payload struct {
		FuncName     string
		ArgCount     int
		LiteralValue int64
	}

	tests := []struct {
		name     string
		funcName string
		group    string
		sql      string
		want     payload
	}{
		{
			name:     "identity in custom group",
			funcName: "identity",
			group:    "custom",
			sql:      "SELECT identity(42)",
			want:     payload{FuncName: "custom:identity", ArgCount: 1, LiteralValue: 42},
		},
		{
			name:     "negate in math group",
			funcName: "negate",
			group:    "math",
			sql:      "SELECT negate(7)",
			want:     payload{FuncName: "math:negate", ArgCount: 1, LiteralValue: 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat := catalog.NewSimpleCatalog("test")
			cat.AddZetaSQLBuiltinFunctions(nil)
			cat.AddFunction(types.NewFunction(
				[]string{tt.funcName},
				tt.group,
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
			output, err := a.AnalyzeStatement(ctx, tt.sql, cat, opts)
			if err != nil {
				t.Fatalf("AnalyzeStatement failed: %v", err)
			}

			stmt := output.ResolvedStatement().(*resolved_ast.QueryStmtNode)
			funcCall := findNode[*resolved_ast.FunctionCallNode](t, stmt)
			arg := funcCall.ArgumentList()[0].(*resolved_ast.LiteralNode)
			got := payload{
				FuncName:     funcCall.Function().GetName(),
				ArgCount:     len(funcCall.ArgumentList()),
				LiteralValue: arg.Value().GetValue().GetInt64Value(),
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
