package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestNestedTypeToProto(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want *generated.TypeProto
	}{
		{
			name: "ARRAY<STRUCT<a INT64>>",
			typ: must(NewArrayType(must(NewStructType([]*StructField{
				NewStructField("a", Int64Type()),
			})))),
			want: &generated.TypeProto{
				TypeKind: generated.TypeKind_TYPE_ARRAY.Enum(),
				ArrayType: &generated.ArrayTypeProto{
					ElementType: &generated.TypeProto{
						TypeKind: generated.TypeKind_TYPE_STRUCT.Enum(),
						StructType: &generated.StructTypeProto{
							Field: []*generated.StructFieldProto{
								{FieldName: ptr("a"), FieldType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_INT64.Enum()}},
							},
						},
					},
				},
			},
		},
		{
			name: "STRUCT<tags ARRAY<STRING>, count INT64>",
			typ: must(NewStructType([]*StructField{
				NewStructField("tags", must(NewArrayType(StringType()))),
				NewStructField("count", Int64Type()),
			})),
			want: &generated.TypeProto{
				TypeKind: generated.TypeKind_TYPE_STRUCT.Enum(),
				StructType: &generated.StructTypeProto{
					Field: []*generated.StructFieldProto{
						{
							FieldName: ptr("tags"),
							FieldType: &generated.TypeProto{
								TypeKind: generated.TypeKind_TYPE_ARRAY.Enum(),
								ArrayType: &generated.ArrayTypeProto{
									ElementType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_STRING.Enum()},
								},
							},
						},
						{FieldName: ptr("count"), FieldType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_INT64.Enum()}},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Empty(t, cmp.Diff(tt.want, tt.typ.ToProto(), protocmp.Transform()), "ToProto() mismatch")
		})
	}
}

func TestNestedTypeRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
	}{
		{"ARRAY<STRUCT<a INT64>>", must(NewArrayType(must(NewStructType([]*StructField{NewStructField("a", Int64Type())}))))},
		{"STRUCT<tags ARRAY<STRING>>", must(NewStructType([]*StructField{NewStructField("tags", must(NewArrayType(StringType())))}))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restored, err := TypeFromProto(tt.typ.ToProto())
			require.NoError(t, err)
			assert.Empty(t, cmp.Diff(tt.typ.ToProto(), restored.ToProto(), protocmp.Transform()), "round-trip mismatch")
		})
	}
}

