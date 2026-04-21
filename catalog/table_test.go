package catalog

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

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

func TestSimpleTableAddColumn(t *testing.T) {
	table := NewSimpleTable("t")
	table.AddColumn(NewSimpleColumn("t", "x", types.BoolType()))
	got := table.ToProto()
	want := &generated.SimpleTableProto{
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
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() after AddColumn mismatch (-want +got):\n%s", diff)
	}
}
