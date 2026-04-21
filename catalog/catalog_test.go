package catalog

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSimpleCatalogEmpty(t *testing.T) {
	got := NewSimpleCatalog("test").ToProto()
	want := &generated.SimpleCatalogProto{
		Name: ptr("test"),
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
	}
}

func TestSimpleCatalogWithTablesAndBuiltins(t *testing.T) {
	cat := NewSimpleCatalog("main")
	cat.AddTable(NewSimpleTable("users",
		NewSimpleColumn("users", "id", types.Int64Type()),
		NewSimpleColumn("users", "name", types.StringType()),
	))
	cat.AddZetaSQLBuiltinFunctions(nil)

	got := cat.ToProto()
	want := &generated.SimpleCatalogProto{
		Name: ptr("main"),
		Table: []*generated.SimpleTableProto{
			{
				Name:         ptr("users"),
				IsValueTable: boolPtr(false),
				Column: []*generated.SimpleColumnProto{
					{
						Name:             ptr("id"),
						Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_INT64.Enum()},
						IsPseudoColumn:   boolPtr(false),
						IsWritableColumn: boolPtr(true),
					},
					{
						Name:             ptr("name"),
						Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_STRING.Enum()},
						IsPseudoColumn:   boolPtr(false),
						IsWritableColumn: boolPtr(true),
					},
				},
			},
		},
		BuiltinFunctionOptions: &generated.ZetaSQLBuiltinFunctionOptionsProto{},
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
	}
}

func TestSimpleCatalogSubCatalogs(t *testing.T) {
	root := NewSimpleCatalog("root")
	sub := NewSimpleCatalog("schema1")
	sub.AddTable(NewSimpleTable("t1",
		NewSimpleColumn("t1", "col", types.DoubleType()),
	))
	root.AddSubCatalog(sub)

	got := root.ToProto()
	want := &generated.SimpleCatalogProto{
		Name: ptr("root"),
		Catalog: []*generated.SimpleCatalogProto{
			{
				Name: ptr("schema1"),
				Table: []*generated.SimpleTableProto{
					{
						Name:         ptr("t1"),
						IsValueTable: boolPtr(false),
						Column: []*generated.SimpleColumnProto{
							{
								Name:             ptr("col"),
								Type:             &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_DOUBLE.Enum()},
								IsPseudoColumn:   boolPtr(false),
								IsWritableColumn: boolPtr(true),
							},
						},
					},
				},
			},
		},
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
	}
}
