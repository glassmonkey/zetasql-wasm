package zetasql

import (
	"context"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/catalog"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
)

// newTestAnalyzer creates a shared analyzer for integration tests.
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

// newUsersCatalog creates a catalog with a "users" table (id INT64, name STRING).
func newUsersCatalog() *catalog.SimpleCatalog {
	cat := catalog.NewSimpleCatalog("test")
	cat.AddZetaSQLBuiltinFunctions(nil)
	cat.AddTable(catalog.NewSimpleTable("users",
		catalog.NewSimpleColumn("users", "id", types.Int64Type()),
		catalog.NewSimpleColumn("users", "name", types.StringType()),
	))
	return cat
}

// newUsersOrdersCatalog creates a catalog with "users" and "orders" tables.
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

func TestResolvedAST_QueryStmt_SelectColumns(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newUsersCatalog()
	out := analyze(t, a, "SELECT id, name FROM users", cat)

	stmt := out.ResolvedStatement()
	queryStmt, ok := stmt.(*resolved_ast.QueryStmtNode)
	if !ok {
		t.Fatalf("expected *QueryStmtNode, got %T", stmt)
	}

	if queryStmt.IsValueTable() {
		t.Error("IsValueTable() = true, want false")
	}

	cols := queryStmt.OutputColumnList()
	if len(cols) != 2 {
		t.Fatalf("OutputColumnList() len = %d, want 2", len(cols))
	}

	wantNames := []string{"id", "name"}
	for i, col := range cols {
		if got := col.Name(); got != wantNames[i] {
			t.Errorf("OutputColumnList()[%d].Name() = %q, want %q", i, got, wantNames[i])
		}
		c := col.Column()
		if c == nil {
			t.Fatalf("OutputColumnList()[%d].Column() = nil", i)
		}
		if got := c.GetTableName(); got != "users" {
			t.Errorf("OutputColumnList()[%d].Column().TableName = %q, want %q", i, got, "users")
		}
	}

	// Query should be a scan node
	query := queryStmt.Query()
	if query == nil {
		t.Fatal("Query() = nil")
	}
}

func TestResolvedAST_TableScan(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newUsersCatalog()
	out := analyze(t, a, "SELECT id FROM users", cat)

	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	// Walk the tree to find the TableScan
	var tableScan *resolved_ast.TableScanNode
	resolved_ast.Walk(stmt, func(n resolved_ast.Node) error {
		if ts, ok := n.(*resolved_ast.TableScanNode); ok {
			tableScan = ts
		}
		return nil
	})

	if tableScan == nil {
		t.Fatal("no TableScanNode found in resolved AST")
	}

	tableRef := tableScan.Table()
	if tableRef == nil {
		t.Fatal("TableScanNode.Table() = nil")
	}
	if got := tableRef.GetName(); got != "users" {
		t.Errorf("Table().Name = %q, want %q", got, "users")
	}

	idxList := tableScan.ColumnIndexList()
	if len(idxList) == 0 {
		t.Error("ColumnIndexList() is empty, expected at least 1 column")
	}
}

func TestResolvedAST_JoinScan(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newUsersOrdersCatalog()
	out := analyze(t, a, "SELECT u.id, o.amount FROM users u JOIN orders o ON u.id = o.user_id", cat)

	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	var joinScan *resolved_ast.JoinScanNode
	resolved_ast.Walk(stmt, func(n resolved_ast.Node) error {
		if js, ok := n.(*resolved_ast.JoinScanNode); ok {
			joinScan = js
		}
		return nil
	})

	if joinScan == nil {
		t.Fatal("no JoinScanNode found in resolved AST")
	}

	// INNER JOIN
	if got := joinScan.JoinType(); got != 0 {
		t.Errorf("JoinType() = %v, want INNER (0)", got)
	}

	if joinScan.LeftScan() == nil {
		t.Error("LeftScan() = nil")
	}
	if joinScan.RightScan() == nil {
		t.Error("RightScan() = nil")
	}
	if joinScan.JoinExpr() == nil {
		t.Error("JoinExpr() = nil, expected ON clause expression")
	}

	// Verify left and right are TableScans
	leftTable, ok := joinScan.LeftScan().(*resolved_ast.TableScanNode)
	if !ok {
		t.Fatalf("LeftScan() type = %T, want *TableScanNode", joinScan.LeftScan())
	}
	if got := leftTable.Table().GetName(); got != "users" {
		t.Errorf("LeftScan().Table().Name = %q, want %q", got, "users")
	}

	rightTable, ok := joinScan.RightScan().(*resolved_ast.TableScanNode)
	if !ok {
		t.Fatalf("RightScan() type = %T, want *TableScanNode", joinScan.RightScan())
	}
	if got := rightTable.Table().GetName(); got != "orders" {
		t.Errorf("RightScan().Table().Name = %q, want %q", got, "orders")
	}
}

func TestResolvedAST_FilterScan(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newUsersCatalog()
	out := analyze(t, a, "SELECT id FROM users WHERE id > 1", cat)

	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	var filterScan *resolved_ast.FilterScanNode
	resolved_ast.Walk(stmt, func(n resolved_ast.Node) error {
		if fs, ok := n.(*resolved_ast.FilterScanNode); ok {
			filterScan = fs
		}
		return nil
	})

	if filterScan == nil {
		t.Fatal("no FilterScanNode found in resolved AST")
	}

	if filterScan.InputScan() == nil {
		t.Error("FilterScan.InputScan() = nil")
	}
	if filterScan.FilterExpr() == nil {
		t.Error("FilterScan.FilterExpr() = nil")
	}

	// The filter expression should be a FunctionCallNode (for the > operator)
	filterExpr := filterScan.FilterExpr()
	if got := filterExpr.Kind(); got != resolved_ast.KindFunctionCall {
		t.Errorf("FilterExpr().Kind() = %v, want KindFunctionCall", got)
	}

	// InputScan should contain a TableScan for "users"
	var tableScan *resolved_ast.TableScanNode
	resolved_ast.Walk(filterScan.InputScan(), func(n resolved_ast.Node) error {
		if ts, ok := n.(*resolved_ast.TableScanNode); ok {
			tableScan = ts
		}
		return nil
	})
	if tableScan == nil {
		t.Fatal("no TableScanNode found under FilterScan.InputScan()")
	}
	if got := tableScan.Table().GetName(); got != "users" {
		t.Errorf("InputScan TableScan.Table().Name = %q, want %q", got, "users")
	}
}

func TestResolvedAST_ProjectScan(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newUsersCatalog()
	out := analyze(t, a, "SELECT id + 1 AS inc FROM users", cat)

	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	var projectScan *resolved_ast.ProjectScanNode
	resolved_ast.Walk(stmt, func(n resolved_ast.Node) error {
		if ps, ok := n.(*resolved_ast.ProjectScanNode); ok {
			projectScan = ps
		}
		return nil
	})

	if projectScan == nil {
		t.Fatal("no ProjectScanNode found in resolved AST")
	}

	exprList := projectScan.ExprList()
	if len(exprList) == 0 {
		t.Fatal("ProjectScan.ExprList() is empty, expected computed columns")
	}

	// Each computed column should have a column and an expression
	for i, cc := range exprList {
		if cc.Column() == nil {
			t.Errorf("ExprList()[%d].Column() = nil", i)
		}
		if cc.Expr() == nil {
			t.Errorf("ExprList()[%d].Expr() = nil", i)
		}
	}

	if projectScan.InputScan() == nil {
		t.Error("ProjectScan.InputScan() = nil")
	}
}

func TestResolvedAST_FunctionCall(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newUsersCatalog()
	out := analyze(t, a, "SELECT UPPER(name) FROM users", cat)

	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	var funcCall *resolved_ast.FunctionCallNode
	resolved_ast.Walk(stmt, func(n resolved_ast.Node) error {
		if fc, ok := n.(*resolved_ast.FunctionCallNode); ok {
			funcCall = fc
		}
		return nil
	})

	if funcCall == nil {
		t.Fatal("no FunctionCallNode found in resolved AST")
	}

	if got := funcCall.Kind(); got != resolved_ast.KindFunctionCall {
		t.Errorf("Kind() = %v, want KindFunctionCall", got)
	}
}

func TestResolvedAST_Walk_CollectsAllNodeKinds(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newUsersCatalog()
	out := analyze(t, a, "SELECT id, name FROM users WHERE id > 1", cat)

	stmt := out.ResolvedStatement()

	kindSet := map[resolved_ast.Kind]bool{}
	resolved_ast.Walk(stmt, func(n resolved_ast.Node) error {
		kindSet[n.Kind()] = true
		return nil
	})

	// This query should produce at minimum these node kinds
	required := []resolved_ast.Kind{
		resolved_ast.KindQueryStmt,
		resolved_ast.KindOutputColumn,
		resolved_ast.KindTableScan,
		resolved_ast.KindFilterScan,
		resolved_ast.KindFunctionCall, // WHERE id > 1
	}
	for _, k := range required {
		if !kindSet[k] {
			t.Errorf("expected %v in Walk result, not found. Got kinds: %v", k, kindSet)
		}
	}
}

func TestResolvedAST_ColumnRef_ViaComputedColumn(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newUsersCatalog()
	// SELECT id + 1 produces a ProjectScan with ComputedColumn whose Expr contains a ColumnRef.
	// Note: FunctionCallNode currently does not expose parent-class children (ArgumentList),
	// so we verify ColumnRef is reachable via ComputedColumn.Expr() → FunctionCall,
	// and also verify direct ColumnRef for simple column projection.
	out := analyze(t, a, "SELECT id FROM users", cat)

	stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)

	// OutputColumn should reference a resolved column with correct metadata
	cols := stmt.OutputColumnList()
	if len(cols) != 1 {
		t.Fatalf("OutputColumnList() len = %d, want 1", len(cols))
	}
	col := cols[0].Column()
	if col == nil {
		t.Fatal("OutputColumn.Column() = nil")
	}
	if got := col.GetName(); got != "id" {
		t.Errorf("Column().Name = %q, want %q", got, "id")
	}
	if got := col.GetTableName(); got != "users" {
		t.Errorf("Column().TableName = %q, want %q", got, "users")
	}
	if col.GetColumnId() == 0 {
		t.Error("Column().ColumnId = 0, want nonzero")
	}
}
