package catalog

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
)

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
