package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestStructTypeNilFields(t *testing.T) {
	st, err := NewStructType(nil)
	if err != nil {
		t.Fatal(err)
	}
	got := st.ToProto()
	want := &generated.TypeProto{
		TypeKind:   generated.TypeKind_TYPE_STRUCT.Enum(),
		StructType: &generated.StructTypeProto{},
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
	}
}

func TestStructTypeToProto(t *testing.T) {
	tests := []struct {
		name   string
		fields []*StructField
		want   *generated.TypeProto
	}{
		{
			name: "two fields",
			fields: []*StructField{
				NewStructField("id", Int64Type()),
				NewStructField("name", StringType()),
			},
			want: &generated.TypeProto{
				TypeKind: generated.TypeKind_TYPE_STRUCT.Enum(),
				StructType: &generated.StructTypeProto{
					Field: []*generated.StructFieldProto{
						{FieldName: ptr("id"), FieldType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_INT64.Enum()}},
						{FieldName: ptr("name"), FieldType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_STRING.Enum()}},
					},
				},
			},
		},
		{
			name: "single field",
			fields: []*StructField{
				NewStructField("value", DoubleType()),
			},
			want: &generated.TypeProto{
				TypeKind: generated.TypeKind_TYPE_STRUCT.Enum(),
				StructType: &generated.StructTypeProto{
					Field: []*generated.StructFieldProto{
						{FieldName: ptr("value"), FieldType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_DOUBLE.Enum()}},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, _ := NewStructType(tt.fields)
			if diff := cmp.Diff(tt.want, st.ToProto(), protocmp.Transform()); diff != "" {
				t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestStructTypeRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		fields []*StructField
	}{
		{"two fields", []*StructField{NewStructField("id", Int64Type()), NewStructField("name", StringType())}},
		{"single field", []*StructField{NewStructField("x", BoolType())}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original, _ := NewStructType(tt.fields)
			restored, err := TypeFromProto(original.ToProto())
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(original.ToProto(), restored.ToProto(), protocmp.Transform()); diff != "" {
				t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func ptr(s string) *string { return &s }
