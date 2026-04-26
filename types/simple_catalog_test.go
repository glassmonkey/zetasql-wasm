package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSimpleCatalogToProto(t *testing.T) {
	tests := []struct {
		name    string
		catalog *SimpleCatalog
		want    *generated.SimpleCatalogProto
	}{
		{
			name:    "empty catalog",
			catalog: NewSimpleCatalog("test"),
			want: &generated.SimpleCatalogProto{
				Name: ptr("test"),
			},
		},
		{
			name: "catalog with table and builtins",
			catalog: func() *SimpleCatalog {
				cat := NewSimpleCatalog("main")
				cat.Tables = append(cat.Tables, NewSimpleTable("users",
					NewSimpleColumn("users", "id", Int64Type()),
					NewSimpleColumn("users", "name", StringType()),
				))
				cat.AddZetaSQLBuiltinFunctions(nil)
				return cat
			}(),
			want: &generated.SimpleCatalogProto{
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
			},
		},
		{
			name: "catalog with sub-catalog",
			catalog: func() *SimpleCatalog {
				root := NewSimpleCatalog("root")
				sub := NewSimpleCatalog("schema1")
				sub.Tables = append(sub.Tables, NewSimpleTable("t1",
					NewSimpleColumn("t1", "col", DoubleType()),
				))
				root.SubCatalogs = append(root.SubCatalogs, sub)
				return root
			}(),
			want: &generated.SimpleCatalogProto{
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Empty(t, cmp.Diff(tt.want, tt.catalog.ToProto(), protocmp.Transform()), "ToProto() mismatch")
		})
	}
}
