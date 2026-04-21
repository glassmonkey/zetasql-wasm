package catalog

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
)

func TestSimpleColumn(t *testing.T) {
	col := NewSimpleColumn("users", "id", types.Int64Type())
	if col.Name() != "id" {
		t.Errorf("Name() = %q, want %q", col.Name(), "id")
	}
	if col.FullName() != "users.id" {
		t.Errorf("FullName() = %q, want %q", col.FullName(), "users.id")
	}
	if col.Type() != types.Int64Type() {
		t.Error("Type() should be Int64Type()")
	}
	if col.IsPseudoColumn() {
		t.Error("IsPseudoColumn() should be false by default")
	}
	if !col.IsWritable() {
		t.Error("IsWritable() should be true by default")
	}
}

func TestSimpleColumnToProto(t *testing.T) {
	col := NewSimpleColumn("t", "name", types.StringType())
	p := col.ToProto()
	if p.GetName() != "name" {
		t.Errorf("proto Name = %q, want %q", p.GetName(), "name")
	}
	if p.GetType().GetTypeKind().String() != "TYPE_STRING" {
		t.Errorf("proto TypeKind = %v, want TYPE_STRING", p.GetType().GetTypeKind())
	}
	if p.GetIsPseudoColumn() {
		t.Error("proto IsPseudoColumn should be false")
	}
	if !p.GetIsWritableColumn() {
		t.Error("proto IsWritableColumn should be true")
	}
}

func TestSimpleTable(t *testing.T) {
	col1 := NewSimpleColumn("users", "id", types.Int64Type())
	col2 := NewSimpleColumn("users", "name", types.StringType())
	table := NewSimpleTable("users", col1, col2)

	if table.Name() != "users" {
		t.Errorf("Name() = %q, want %q", table.Name(), "users")
	}
	if table.NumColumns() != 2 {
		t.Fatalf("NumColumns() = %d, want 2", table.NumColumns())
	}
	if table.Column(0).Name() != "id" {
		t.Error("Column(0) should be id")
	}
	if table.Column(1).Name() != "name" {
		t.Error("Column(1) should be name")
	}
}

func TestSimpleTableAddColumn(t *testing.T) {
	table := NewSimpleTable("t")
	if table.NumColumns() != 0 {
		t.Fatalf("NumColumns() = %d, want 0", table.NumColumns())
	}
	table.AddColumn(NewSimpleColumn("t", "x", types.BoolType()))
	if table.NumColumns() != 1 {
		t.Fatalf("NumColumns() = %d, want 1", table.NumColumns())
	}
}

func TestSimpleTableToProto(t *testing.T) {
	table := NewSimpleTable("orders",
		NewSimpleColumn("orders", "id", types.Int64Type()),
	)
	p := table.ToProto()
	if p.GetName() != "orders" {
		t.Errorf("proto Name = %q, want %q", p.GetName(), "orders")
	}
	if len(p.GetColumn()) != 1 {
		t.Fatalf("proto Column count = %d, want 1", len(p.GetColumn()))
	}
	if p.GetColumn()[0].GetName() != "id" {
		t.Errorf("proto Column[0].Name = %q, want %q", p.GetColumn()[0].GetName(), "id")
	}
}

func TestSimpleCatalog(t *testing.T) {
	cat := NewSimpleCatalog("test")
	if cat.Name() != "test" {
		t.Errorf("Name() = %q, want %q", cat.Name(), "test")
	}
}

func TestSimpleCatalogWithTablesAndBuiltins(t *testing.T) {
	cat := NewSimpleCatalog("main")
	cat.AddTable(NewSimpleTable("users",
		NewSimpleColumn("users", "id", types.Int64Type()),
		NewSimpleColumn("users", "name", types.StringType()),
		NewSimpleColumn("users", "active", types.BoolType()),
	))
	cat.AddZetaSQLBuiltinFunctions(nil)

	p := cat.ToProto()
	if p.GetName() != "main" {
		t.Errorf("proto Name = %q, want %q", p.GetName(), "main")
	}
	if len(p.GetTable()) != 1 {
		t.Fatalf("proto Table count = %d, want 1", len(p.GetTable()))
	}
	if p.GetTable()[0].GetName() != "users" {
		t.Errorf("proto Table[0].Name = %q, want %q", p.GetTable()[0].GetName(), "users")
	}
	if len(p.GetTable()[0].GetColumn()) != 3 {
		t.Fatalf("proto Table[0] Column count = %d, want 3", len(p.GetTable()[0].GetColumn()))
	}
	if p.GetBuiltinFunctionOptions() == nil {
		t.Error("proto BuiltinFunctionOptions should not be nil")
	}
}

func TestSimpleCatalogSubCatalogs(t *testing.T) {
	root := NewSimpleCatalog("root")
	sub := NewSimpleCatalog("schema1")
	sub.AddTable(NewSimpleTable("t1",
		NewSimpleColumn("t1", "col", types.DoubleType()),
	))
	root.AddSubCatalog(sub)

	p := root.ToProto()
	if len(p.GetCatalog()) != 1 {
		t.Fatalf("proto Catalog count = %d, want 1", len(p.GetCatalog()))
	}
	subProto := p.GetCatalog()[0]
	if subProto.GetName() != "schema1" {
		t.Errorf("sub catalog name = %q, want %q", subProto.GetName(), "schema1")
	}
	if len(subProto.GetTable()) != 1 {
		t.Fatalf("sub catalog table count = %d, want 1", len(subProto.GetTable()))
	}
}

func TestSimpleCatalogNoBuiltins(t *testing.T) {
	cat := NewSimpleCatalog("empty")
	p := cat.ToProto()
	if p.GetBuiltinFunctionOptions() != nil {
		t.Error("proto BuiltinFunctionOptions should be nil when not set")
	}
}
