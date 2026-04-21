package catalog

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSimpleTableProperties(t *testing.T) {
	table := NewSimpleTable("users",
		NewSimpleColumn("users", "id", types.Int64Type()),
		NewSimpleColumn("users", "name", types.StringType()),
	)
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Name", table.Name(), "users"},
		{"NumColumns", table.NumColumns(), 2},
		{"Column(0).Name", table.Column(0).Name(), "id"},
		{"Column(1).Name", table.Column(1).Name(), "name"},
		{"IsValueTable", table.IsValueTable(), false},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestSimpleTableAddColumn(t *testing.T) {
	table := NewSimpleTable("t")
	table.AddColumn(NewSimpleColumn("t", "x", types.BoolType()))
	if got := table.NumColumns(); got != 1 {
		t.Errorf("NumColumns() = %d, want 1", got)
	}
}

func TestSimpleTableToProto(t *testing.T) {
	table := NewSimpleTable("orders",
		NewSimpleColumn("orders", "id", types.Int64Type()),
		NewSimpleColumn("orders", "total", types.DoubleType()),
	)
	got := table.ToProto()
	want := &generated.SimpleTableProto{
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
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
	}
}
