package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
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
			col:  NewSimpleColumn("t", "name", StringType()),
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
				c := NewSimpleColumn("t", "_partition", Int64Type())
				c.IsPseudoColumn = true
				c.IsWritable = false
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
			assert.Empty(t, cmp.Diff(tt.want, tt.col.ToProto(), protocmp.Transform()), "ToProto() mismatch")
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
		c := NewSimpleColumn(tt.table, tt.col, Int64Type())
		assert.Equal(t, tt.want, c.FullName())
	}
}

func boolPtr(b bool) *bool { return &b }
