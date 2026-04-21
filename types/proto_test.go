package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
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
			if diff := cmp.Diff(tt.want, tt.typ.ToProto(), protocmp.Transform()); diff != "" {
				t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
			}
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
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.typ.ToProto(), restored.ToProto(), protocmp.Transform()); diff != "" {
				t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTypeFromProtoErrors(t *testing.T) {
	tests := []struct {
		name  string
		proto *generated.TypeProto
	}{
		{"nil", nil},
		{"array without ArrayType", &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_ARRAY.Enum()}},
		{"struct without StructType", &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_STRUCT.Enum()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := TypeFromProto(tt.proto); err == nil {
				t.Error("TypeFromProto() should return error")
			}
		})
	}
}
