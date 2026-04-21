package catalog

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
)

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
