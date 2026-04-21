package catalog

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSimpleColumnToProto(t *testing.T) {
	col := NewSimpleColumn("t", "name", types.StringType())
	got := col.ToProto()
	want := &generated.SimpleColumnProto{
		Name:             ptr("name"),
		Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_STRING.Enum()},
		IsPseudoColumn:   boolPtr(false),
		IsWritableColumn: boolPtr(true),
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
	}
}

func TestSimpleColumnFullName(t *testing.T) {
	col := NewSimpleColumn("users", "id", types.Int64Type())
	if got, want := col.FullName(), "users.id"; got != want {
		t.Errorf("FullName() = %q, want %q", got, want)
	}
}

func ptr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }
