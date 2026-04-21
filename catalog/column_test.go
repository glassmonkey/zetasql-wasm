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
