package catalog

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSimpleColumnToProto(t *testing.T) {
	tests := []struct {
		name string
		col  *SimpleColumn
		want *generated.SimpleColumnProto
	}{
		{
			name: "writable string column",
			col:  NewSimpleColumn("t", "name", types.StringType()),
			want: &generated.SimpleColumnProto{
				Name:             ptr("name"),
				Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_STRING.Enum()},
				IsPseudoColumn:   boolPtr(false),
				IsWritableColumn: boolPtr(true),
			},
		},
		{
			name: "pseudo column",
			col: func() *SimpleColumn {
				c := NewSimpleColumn("t", "_partition", types.Int64Type())
				c.SetIsPseudoColumn(true)
				c.SetIsWritable(false)
				return c
			}(),
			want: &generated.SimpleColumnProto{
				Name:             ptr("_partition"),
				Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_INT64.Enum()},
				IsPseudoColumn:   boolPtr(true),
				IsWritableColumn: boolPtr(false),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, tt.col.ToProto(), protocmp.Transform()); diff != "" {
				t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSimpleColumnFullName(t *testing.T) {
	tests := []struct {
		table, col, want string
	}{
		{"users", "id", "users.id"},
		{"orders", "total", "orders.total"},
	}
	for _, tt := range tests {
		c := NewSimpleColumn(tt.table, tt.col, types.Int64Type())
		if got := c.FullName(); got != tt.want {
			t.Errorf("FullName() = %q, want %q", got, tt.want)
		}
	}
}

func ptr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }
