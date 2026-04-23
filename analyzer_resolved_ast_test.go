package zetasql

import (
	"context"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/catalog"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

func newTestAnalyzer(t *testing.T) *Analyzer {
	t.Helper()
	ctx := context.Background()
	a, err := NewAnalyzer(ctx)
	if err != nil {
		t.Fatalf("NewAnalyzer: %v", err)
	}
	t.Cleanup(func() { a.Close(ctx) })
	return a
}

func newUsersCatalog() *catalog.SimpleCatalog {
	cat := catalog.NewSimpleCatalog("test")
	cat.AddZetaSQLBuiltinFunctions(nil)
	cat.AddTable(catalog.NewSimpleTable("users",
		catalog.NewSimpleColumn("users", "id", types.Int64Type()),
		catalog.NewSimpleColumn("users", "name", types.StringType()),
	))
	return cat
}

func newUsersOrdersCatalog() *catalog.SimpleCatalog {
	cat := catalog.NewSimpleCatalog("test")
	cat.AddZetaSQLBuiltinFunctions(nil)
	cat.AddTable(catalog.NewSimpleTable("users",
		catalog.NewSimpleColumn("users", "id", types.Int64Type()),
		catalog.NewSimpleColumn("users", "name", types.StringType()),
	))
	cat.AddTable(catalog.NewSimpleTable("orders",
		catalog.NewSimpleColumn("orders", "order_id", types.Int64Type()),
		catalog.NewSimpleColumn("orders", "user_id", types.Int64Type()),
		catalog.NewSimpleColumn("orders", "amount", types.Int64Type()),
	))
	return cat
}

func analyze(t *testing.T, a *Analyzer, sql string, cat *catalog.SimpleCatalog) *AnalyzeOutput {
	t.Helper()
	ctx := context.Background()
	opts := NewAnalyzerOptions()
	out, err := a.AnalyzeStatement(ctx, sql, cat, opts)
	if err != nil {
		t.Fatalf("AnalyzeStatement(%q): %v", sql, err)
	}
	return out
}

// findNode walks the tree and returns the first node matching the predicate.
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
	if !found {
		t.Fatalf("node of type %T not found in resolved AST", result)
	}
	return result
}

// collectNodes walks the tree and returns all nodes matching the type.
func collectNodes[T resolved_ast.Node](root resolved_ast.Node) []T {
	var result []T
	resolved_ast.Walk(root, func(n resolved_ast.Node) error {
		if v, ok := n.(T); ok {
			result = append(result, v)
		}
		return nil
	})
	return result
}

func TestResolvedAST_QueryStmt_SelectColumns(t *testing.T) {
	a := newTestAnalyzer(t)
	out := analyze(t, a, "SELECT id, name FROM users", newUsersCatalog())
	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	if got, want := stmt.Kind(), resolved_ast.KindQueryStmt; got != want {
		t.Errorf("Kind() = %v, want %v", got, want)
	}
	if got, want := stmt.IsValueTable(), false; got != want {
		t.Errorf("IsValueTable() = %v, want %v", got, want)
	}

	cols := stmt.OutputColumnList()
	if got, want := len(cols), 2; got != want {
		t.Fatalf("len(OutputColumnList()) = %d, want %d", got, want)
	}

	if got, want := cols[0].Name(), "id"; got != want {
		t.Errorf("cols[0].Name() = %q, want %q", got, want)
	}
	if got, want := cols[0].Column().GetTableName(), "users"; got != want {
		t.Errorf("cols[0].Column().TableName = %q, want %q", got, want)
	}
	if got, want := cols[0].Column().GetName(), "id"; got != want {
		t.Errorf("cols[0].Column().Name = %q, want %q", got, want)
	}

	if got, want := cols[1].Name(), "name"; got != want {
		t.Errorf("cols[1].Name() = %q, want %q", got, want)
	}
	if got, want := cols[1].Column().GetTableName(), "users"; got != want {
		t.Errorf("cols[1].Column().TableName = %q, want %q", got, want)
	}
	if got, want := cols[1].Column().GetName(), "name"; got != want {
		t.Errorf("cols[1].Column().Name = %q, want %q", got, want)
	}
}

func TestResolvedAST_TableScan(t *testing.T) {
	a := newTestAnalyzer(t)
	out := analyze(t, a, "SELECT id FROM users", newUsersCatalog())
	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	tableScan := findNode[*resolved_ast.TableScanNode](t, stmt)

	if got, want := tableScan.Kind(), resolved_ast.KindTableScan; got != want {
		t.Errorf("Kind() = %v, want %v", got, want)
	}
	if got, want := tableScan.Table().GetName(), "users"; got != want {
		t.Errorf("Table().Name = %q, want %q", got, want)
	}
	// SELECT id FROM users scans both columns (id, name) at the table level,
	// even though only id is projected. ZetaSQL includes all referenced columns.
	if got, want := len(tableScan.ColumnIndexList()), 2; got != want {
		t.Errorf("len(ColumnIndexList()) = %d, want %d", got, want)
	}
	if got, want := tableScan.ColumnIndexList()[0], int64(0); got != want {
		t.Errorf("ColumnIndexList()[0] = %d, want %d", got, want)
	}
	if got, want := tableScan.ColumnIndexList()[1], int64(1); got != want {
		t.Errorf("ColumnIndexList()[1] = %d, want %d", got, want)
	}
}

func TestResolvedAST_JoinScan(t *testing.T) {
	a := newTestAnalyzer(t)
	out := analyze(t, a, "SELECT u.id, o.amount FROM users u JOIN orders o ON u.id = o.user_id", newUsersOrdersCatalog())
	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	joinScan := findNode[*resolved_ast.JoinScanNode](t, stmt)

	if got, want := joinScan.Kind(), resolved_ast.KindJoinScan; got != want {
		t.Errorf("Kind() = %v, want %v", got, want)
	}
	if got, want := joinScan.JoinType(), generated.ResolvedJoinScanEnums_INNER; got != want {
		t.Errorf("JoinType() = %v, want %v (INNER)", got, want)
	}

	leftTable := joinScan.LeftScan().(*resolved_ast.TableScanNode)
	if got, want := leftTable.Table().GetName(), "users"; got != want {
		t.Errorf("LeftScan().Table().Name = %q, want %q", got, want)
	}

	rightTable := joinScan.RightScan().(*resolved_ast.TableScanNode)
	if got, want := rightTable.Table().GetName(), "orders"; got != want {
		t.Errorf("RightScan().Table().Name = %q, want %q", got, want)
	}

	// ON clause should be a FunctionCall ($equal)
	joinExpr := joinScan.JoinExpr().(*resolved_ast.FunctionCallNode)
	if got, want := joinExpr.Kind(), resolved_ast.KindFunctionCall; got != want {
		t.Errorf("JoinExpr().Kind() = %v, want %v", got, want)
	}
	if got, want := joinExpr.Function().GetName(), "ZetaSQL:$equal"; got != want {
		t.Errorf("JoinExpr().Function().Name = %q, want %q", got, want)
	}
	if got, want := len(joinExpr.ArgumentList()), 2; got != want {
		t.Errorf("len(JoinExpr().ArgumentList()) = %d, want %d", got, want)
	}
}

func TestResolvedAST_FilterScan(t *testing.T) {
	a := newTestAnalyzer(t)
	out := analyze(t, a, "SELECT id FROM users WHERE id > 1", newUsersCatalog())
	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	filterScan := findNode[*resolved_ast.FilterScanNode](t, stmt)

	if got, want := filterScan.Kind(), resolved_ast.KindFilterScan; got != want {
		t.Errorf("Kind() = %v, want %v", got, want)
	}

	// FilterExpr should be $greater_than function
	filterFunc := filterScan.FilterExpr().(*resolved_ast.FunctionCallNode)
	if got, want := filterFunc.Function().GetName(), "ZetaSQL:$greater"; got != want {
		t.Errorf("FilterExpr().Function().Name = %q, want %q", got, want)
	}
	if got, want := len(filterFunc.ArgumentList()), 2; got != want {
		t.Errorf("len(FilterExpr().ArgumentList()) = %d, want %d", got, want)
	}

	// InputScan should be a TableScan for "users"
	inputTableScan := filterScan.InputScan().(*resolved_ast.TableScanNode)
	if got, want := inputTableScan.Table().GetName(), "users"; got != want {
		t.Errorf("InputScan().Table().Name = %q, want %q", got, want)
	}
}

func TestResolvedAST_ProjectScan(t *testing.T) {
	a := newTestAnalyzer(t)
	out := analyze(t, a, "SELECT id + 1 AS inc FROM users", newUsersCatalog())
	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	projectScan := findNode[*resolved_ast.ProjectScanNode](t, stmt)

	if got, want := projectScan.Kind(), resolved_ast.KindProjectScan; got != want {
		t.Errorf("Kind() = %v, want %v", got, want)
	}

	exprList := projectScan.ExprList()
	if got, want := len(exprList), 1; got != want {
		t.Fatalf("len(ExprList()) = %d, want %d", got, want)
	}

	// The computed column's expression should be an $add function
	addFunc := exprList[0].Expr().(*resolved_ast.FunctionCallNode)
	if got, want := addFunc.Function().GetName(), "ZetaSQL:$add"; got != want {
		t.Errorf("ExprList()[0].Expr().Function().Name = %q, want %q", got, want)
	}
	if got, want := len(addFunc.ArgumentList()), 2; got != want {
		t.Errorf("len(ExprList()[0].Expr().ArgumentList()) = %d, want %d", got, want)
	}
}

func TestResolvedAST_FunctionCall_WithArguments(t *testing.T) {
	a := newTestAnalyzer(t)
	out := analyze(t, a, "SELECT UPPER(name) FROM users", newUsersCatalog())
	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	funcCall := findNode[*resolved_ast.FunctionCallNode](t, stmt)

	if got, want := funcCall.Kind(), resolved_ast.KindFunctionCall; got != want {
		t.Errorf("Kind() = %v, want %v", got, want)
	}
	if got, want := funcCall.Function().GetName(), "ZetaSQL:upper"; got != want {
		t.Errorf("Function().Name = %q, want %q", got, want)
	}

	args := funcCall.ArgumentList()
	if got, want := len(args), 1; got != want {
		t.Fatalf("len(ArgumentList()) = %d, want %d", got, want)
	}

	colRef := args[0].(*resolved_ast.ColumnRefNode)
	if got, want := colRef.Column().GetName(), "name"; got != want {
		t.Errorf("ArgumentList()[0].Column().Name = %q, want %q", got, want)
	}
	if got, want := colRef.Column().GetTableName(), "users"; got != want {
		t.Errorf("ArgumentList()[0].Column().TableName = %q, want %q", got, want)
	}
}

func TestResolvedAST_Walk_NodeKinds(t *testing.T) {
	a := newTestAnalyzer(t)
	out := analyze(t, a, "SELECT id, name FROM users WHERE id > 1", newUsersCatalog())

	var kinds []resolved_ast.Kind
	resolved_ast.Walk(out.ResolvedStatement(), func(n resolved_ast.Node) error {
		kinds = append(kinds, n.Kind())
		return nil
	})

	wantKinds := []resolved_ast.Kind{
		resolved_ast.KindQueryStmt,
		resolved_ast.KindOutputColumn,
		resolved_ast.KindOutputColumn,
		resolved_ast.KindProjectScan,
		resolved_ast.KindFilterScan,
		resolved_ast.KindTableScan,
		resolved_ast.KindFunctionCall,
		resolved_ast.KindColumnRef,
		resolved_ast.KindLiteral,
	}
	if got, want := len(kinds), len(wantKinds); got != want {
		t.Fatalf("Walk visited %d nodes, want %d; got kinds: %v", got, want, kinds)
	}
	for i := range kinds {
		if got, want := kinds[i], wantKinds[i]; got != want {
			t.Errorf("kinds[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestResolvedAST_ColumnRef_WalkTraversal(t *testing.T) {
	a := newTestAnalyzer(t)
	out := analyze(t, a, "SELECT UPPER(name) FROM users", newUsersCatalog())

	colRefs := collectNodes[*resolved_ast.ColumnRefNode](out.ResolvedStatement())

	if got, want := len(colRefs), 1; got != want {
		t.Fatalf("len(ColumnRefNodes) = %d, want %d", got, want)
	}

	if got, want := colRefs[0].Column().GetName(), "name"; got != want {
		t.Errorf("ColumnRef.Column().Name = %q, want %q", got, want)
	}
	if got, want := colRefs[0].Column().GetTableName(), "users"; got != want {
		t.Errorf("ColumnRef.Column().TableName = %q, want %q", got, want)
	}
	if got, want := colRefs[0].IsCorrelated(), false; got != want {
		t.Errorf("ColumnRef.IsCorrelated() = %v, want %v", got, want)
	}
}
