package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestStructTypeProperties(t *testing.T) {
	st, err := NewStructType([]*StructField{
		NewStructField("x", Int64Type()),
		NewStructField("y", StringType()),
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Kind", st.Kind(), Struct},
		{"IsArray", st.IsArray(), false},
		{"IsStruct", st.IsStruct(), true},
		{"NumFields", st.NumFields(), 2},
		{"Field(0).Name", st.Field(0).Name(), "x"},
		{"Field(0).Type", st.Field(0).Type(), Int64Type()},
		{"Field(1).Name", st.Field(1).Name(), "y"},
		{"Field(1).Type", st.Field(1).Type(), StringType()},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
	if st.AsStruct() != st {
		t.Error("AsStruct() should return self")
	}
	if st.AsArray() != nil {
		t.Error("AsArray() should return nil")
	}
}

func TestStructTypeNilFields(t *testing.T) {
	st, err := NewStructType(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.NumFields(); got != 0 {
		t.Errorf("NumFields() = %d, want 0", got)
	}
}

func TestStructTypeToProto(t *testing.T) {
	st, _ := NewStructType([]*StructField{
		NewStructField("id", Int64Type()),
		NewStructField("name", StringType()),
	})
	got := st.ToProto()
	want := &generated.TypeProto{
		TypeKind: generated.TypeKind_TYPE_STRUCT.Enum(),
		StructType: &generated.StructTypeProto{
			Field: []*generated.StructFieldProto{
				{FieldName: ptr("id"), FieldType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_INT64.Enum()}},
				{FieldName: ptr("name"), FieldType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_STRING.Enum()}},
			},
		},
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
	}
}

func TestStructTypeRoundTrip(t *testing.T) {
	original, _ := NewStructType([]*StructField{
		NewStructField("id", Int64Type()),
		NewStructField("name", StringType()),
	})
	restored, err := TypeFromProto(original.ToProto())
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(original.ToProto(), restored.ToProto(), protocmp.Transform()); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func ptr(s string) *string { return &s }
