package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestNestedArrayOfStructToProto(t *testing.T) {
	st, _ := NewStructType([]*StructField{
		NewStructField("a", Int64Type()),
	})
	arr, err := NewArrayType(st)
	if err != nil {
		t.Fatal(err)
	}
	got := arr.ToProto()
	want := &generated.TypeProto{
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
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
	}
}

func TestNestedArrayOfStructRoundTrip(t *testing.T) {
	st, _ := NewStructType([]*StructField{
		NewStructField("a", Int64Type()),
	})
	original, _ := NewArrayType(st)
	restored, err := TypeFromProto(original.ToProto())
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(original.ToProto(), restored.ToProto(), protocmp.Transform()); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
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
