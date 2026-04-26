package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSimpleTableToProto(t *testing.T) {
	tests := []struct {
		name  string
		table *SimpleTable
		want  *generated.SimpleTableProto
	}{
		{
			name: "multi-column table",
			table: NewSimpleTable("orders",
				NewSimpleColumn("orders", "id", Int64Type()),
				NewSimpleColumn("orders", "total", DoubleType()),
			),
			want: &generated.SimpleTableProto{
				Name:         ptr("orders"),
				IsValueTable: boolPtr(false),
				Column: []*generated.SimpleColumnProto{
					{
						Name:             ptr("id"),
						Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_INT64.Enum()},
						IsPseudoColumn:   boolPtr(false),
						IsWritableColumn: boolPtr(true),
					},
					{
						Name:             ptr("total"),
						Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_DOUBLE.Enum()},
						IsPseudoColumn:   boolPtr(false),
						IsWritableColumn: boolPtr(true),
					},
				},
			},
		},
		{
			name:  "empty table",
			table: NewSimpleTable("empty"),
			want: &generated.SimpleTableProto{
				Name:         ptr("empty"),
				IsValueTable: boolPtr(false),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Empty(t, cmp.Diff(tt.want, tt.table.ToProto(), protocmp.Transform()), "ToProto() mismatch")
		})
	}
}

func TestSimpleTableAddColumnToProto(t *testing.T) {
	tests := []struct {
		name string
		add  []*SimpleColumn
		want *generated.SimpleTableProto
	}{
		{
			name: "add one column",
			add:  []*SimpleColumn{NewSimpleColumn("t", "x", BoolType())},
			want: &generated.SimpleTableProto{
				Name:         ptr("t"),
				IsValueTable: boolPtr(false),
				Column: []*generated.SimpleColumnProto{
					{
						Name:             ptr("x"),
						Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_BOOL.Enum()},
						IsPseudoColumn:   boolPtr(false),
						IsWritableColumn: boolPtr(true),
					},
				},
			},
		},
		{
			name: "add two columns",
			add: []*SimpleColumn{
				NewSimpleColumn("t", "a", StringType()),
				NewSimpleColumn("t", "b", TimestampType()),
			},
			want: &generated.SimpleTableProto{
				Name:         ptr("t"),
				IsValueTable: boolPtr(false),
				Column: []*generated.SimpleColumnProto{
					{
						Name:             ptr("a"),
						Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_STRING.Enum()},
						IsPseudoColumn:   boolPtr(false),
						IsWritableColumn: boolPtr(true),
					},
					{
						Name:             ptr("b"),
						Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_TIMESTAMP.Enum()},
						IsPseudoColumn:   boolPtr(false),
						IsWritableColumn: boolPtr(true),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := NewSimpleTable("t")
			table.Columns = append(table.Columns, tt.add...)
			assert.Empty(t, cmp.Diff(tt.want, table.ToProto(), protocmp.Transform()), "ToProto() mismatch")
		})
	}
}
