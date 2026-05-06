package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
)

// TestWrapType covers the read-side wrap contract: scalar kinds return
// the package singleton, ARRAY / STRUCT recurse, ENUM / PROTO / EXTENDED
// surface as nil (documented gap), and an array of an unsupported kind
// collapses the whole array to nil while a struct preserves field names
// even when individual field types are unsupported. Triangulated across
// nil input, two scalar kinds, two compound shapes, an unsupported kind,
// and the asymmetric array-vs-struct fallback so a future regression in
// any one branch shows up in the diff.
func TestWrapType(t *testing.T) {
	int64Kind := generated.TypeKind_TYPE_INT64
	stringKind := generated.TypeKind_TYPE_STRING
	arrayKind := generated.TypeKind_TYPE_ARRAY
	structKind := generated.TypeKind_TYPE_STRUCT
	enumKind := generated.TypeKind_TYPE_ENUM

	tests := []struct {
		name string
		in   *generated.TypeProto
		want Type
	}{
		{
			name: "nil proto returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "TYPE_UNKNOWN (zero) returns nil",
			in:   &generated.TypeProto{},
			want: nil,
		},
		{
			name: "scalar INT64 returns the singleton",
			in:   &generated.TypeProto{TypeKind: &int64Kind},
			want: Int64Type(),
		},
		{
			name: "scalar STRING returns the singleton",
			in:   &generated.TypeProto{TypeKind: &stringKind},
			want: StringType(),
		},
		{
			name: "ARRAY of INT64 wraps recursively",
			in: &generated.TypeProto{
				TypeKind: &arrayKind,
				ArrayType: &generated.ArrayTypeProto{
					ElementType: &generated.TypeProto{TypeKind: &int64Kind},
				},
			},
			want: &ArrayType{ElementType: Int64Type()},
		},
		{
			name: "STRUCT of {a INT64, b STRING} wraps each field",
			in: &generated.TypeProto{
				TypeKind: &structKind,
				StructType: &generated.StructTypeProto{
					Field: []*generated.StructFieldProto{
						{FieldName: ptr("a"), FieldType: &generated.TypeProto{TypeKind: &int64Kind}},
						{FieldName: ptr("b"), FieldType: &generated.TypeProto{TypeKind: &stringKind}},
					},
				},
			},
			want: &StructType{Fields: []*StructField{
				{Name: "a", Type: Int64Type()},
				{Name: "b", Type: StringType()},
			}},
		},
		{
			name: "ENUM returns nil (documented gap)",
			in:   &generated.TypeProto{TypeKind: &enumKind},
			want: nil,
		},
		{
			name: "ARRAY of ENUM collapses to nil (element type unsupported)",
			in: &generated.TypeProto{
				TypeKind: &arrayKind,
				ArrayType: &generated.ArrayTypeProto{
					ElementType: &generated.TypeProto{TypeKind: &enumKind},
				},
			},
			want: nil,
		},
		{
			name: "STRUCT preserves field name when a field's Type is unsupported",
			in: &generated.TypeProto{
				TypeKind: &structKind,
				StructType: &generated.StructTypeProto{
					Field: []*generated.StructFieldProto{
						{FieldName: ptr("ok"), FieldType: &generated.TypeProto{TypeKind: &int64Kind}},
						{FieldName: ptr("dropped"), FieldType: &generated.TypeProto{TypeKind: &enumKind}},
					},
				},
			},
			want: &StructType{Fields: []*StructField{
				{Name: "ok", Type: Int64Type()},
				{Name: "dropped", Type: nil},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			in := tt.in

			// Act
			got := WrapType(in)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
