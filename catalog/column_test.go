package catalog

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSimpleColumnProperties(t *testing.T) {
	col := NewSimpleColumn("users", "id", types.Int64Type())
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Name", col.Name(), "id"},
		{"FullName", col.FullName(), "users.id"},
		{"Type", col.Type(), types.Int64Type()},
		{"IsPseudoColumn", col.IsPseudoColumn(), false},
		{"IsWritable", col.IsWritable(), true},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

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

func ptr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }
