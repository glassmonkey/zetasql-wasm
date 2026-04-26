package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSimpleCatalog_FullName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "main", want: "main"},
		{name: "analytics_db", want: "analytics_db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := NewSimpleCatalog(tt.name)

			// Act
			got := sut.FullName()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// newFindTestCatalog builds a fixed two-level catalog used by both FindTable
// and FindFunction tests:
//
//	root
//	├── users (table)
//	├── orders (table)
//	├── my_func (function with NamePath ["my_func"])
//	└── schema1 (sub-catalog)
//	    ├── inner_table (table)
//	    └── inner_func (function with NamePath ["inner_func"])
func newFindTestCatalog() *SimpleCatalog {
	cat := NewSimpleCatalog("root")
	cat.Tables = append(cat.Tables,
		NewSimpleTable("users"),
		NewSimpleTable("orders"),
	)
	cat.Functions = append(cat.Functions, NewFunction(
		[]string{"my_func"}, "custom", ScalarMode,
		[]*FunctionSignature{NewFunctionSignature(
			NewFunctionArgumentType(Int64Type()),
			[]*FunctionArgumentType{NewFunctionArgumentType(Int64Type())},
		)},
	))
	sub := NewSimpleCatalog("schema1")
	sub.Tables = append(sub.Tables, NewSimpleTable("inner_table"))
	sub.Functions = append(sub.Functions, NewFunction(
		[]string{"inner_func"}, "custom", ScalarMode,
		[]*FunctionSignature{NewFunctionSignature(
			NewFunctionArgumentType(StringType()),
			[]*FunctionArgumentType{NewFunctionArgumentType(StringType())},
		)},
	))
	cat.SubCatalogs = append(cat.SubCatalogs, sub)
	return cat
}

func TestSimpleCatalog_FindTable_Found(t *testing.T) {
	tests := []struct {
		name     string
		namePath []string
		want     string
	}{
		{name: "single segment", namePath: []string{"users"}, want: "users"},
		{name: "case-insensitive single segment", namePath: []string{"USERS"}, want: "users"},
		{name: "two segments via sub-catalog", namePath: []string{"schema1", "inner_table"}, want: "inner_table"},
		{name: "case-insensitive sub-catalog and table", namePath: []string{"SCHEMA1", "INNER_TABLE"}, want: "inner_table"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := newFindTestCatalog()

			// Act
			got, err := sut.FindTable(tt.namePath)

			// Assert
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.Name)
		})
	}
}

func TestSimpleCatalog_FindTable_NotFound(t *testing.T) {
	tests := []struct {
		name     string
		namePath []string
	}{
		{name: "single segment miss", namePath: []string{"missing"}},
		{name: "two segments miss in sub-catalog", namePath: []string{"schema1", "missing"}},
		{name: "two segments miss on sub-catalog", namePath: []string{"missing_schema", "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := newFindTestCatalog()

			// Act
			got, err := sut.FindTable(tt.namePath)

			// Assert
			assert.ErrorIs(t, err, ErrNotFound)
			assert.Nil(t, got)
		})
	}
}

func TestSimpleCatalog_FindFunction_Found(t *testing.T) {
	tests := []struct {
		name     string
		namePath []string
		want     string
	}{
		{name: "root function", namePath: []string{"my_func"}, want: "my_func"},
		{name: "case-insensitive root function", namePath: []string{"MY_FUNC"}, want: "my_func"},
		{name: "function in sub-catalog (recursive search)", namePath: []string{"inner_func"}, want: "inner_func"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := newFindTestCatalog()

			// Act
			got, err := sut.FindFunction(tt.namePath)

			// Assert
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got.NamePath, 1)
			assert.Equal(t, tt.want, got.NamePath[0])
		})
	}
}

func TestSimpleCatalog_FindFunction_NotFound(t *testing.T) {
	tests := []struct {
		name     string
		namePath []string
	}{
		{name: "missing single segment", namePath: []string{"missing_func"}},
		{name: "wrong path length", namePath: []string{"my_func", "extra"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := newFindTestCatalog()

			// Act
			got, err := sut.FindFunction(tt.namePath)

			// Assert
			assert.ErrorIs(t, err, ErrNotFound)
			assert.Nil(t, got)
		})
	}
}
