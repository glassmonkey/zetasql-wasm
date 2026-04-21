package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestArrayTypeProperties(t *testing.T) {
	arr, err := NewArrayType(Int64Type())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Kind", arr.Kind(), Array},
		{"IsArray", arr.IsArray(), true},
		{"IsStruct", arr.IsStruct(), false},
		{"ElementType", arr.ElementType(), Int64Type()},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
	if arr.AsArray() != arr {
		t.Error("AsArray() should return self")
	}
	if arr.AsStruct() != nil {
		t.Error("AsStruct() should return nil")
	}
}

func TestArrayTypeErrors(t *testing.T) {
	tests := []struct {
		name    string
		elem    Type
		wantErr bool
	}{
		{"nil element", nil, true},
		{"array of array", must(NewArrayType(StringType())), true},
		{"valid", Int64Type(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewArrayType(tt.elem)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewArrayType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestArrayTypeToProto(t *testing.T) {
	arr, _ := NewArrayType(StringType())
	got := arr.ToProto()
	want := &generated.TypeProto{
		TypeKind: generated.TypeKind_TYPE_ARRAY.Enum(),
		ArrayType: &generated.ArrayTypeProto{
			ElementType: &generated.TypeProto{
				TypeKind: generated.TypeKind_TYPE_STRING.Enum(),
			},
		},
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
	}
}

func TestArrayTypeRoundTrip(t *testing.T) {
	original, _ := NewArrayType(StringType())
	restored, err := TypeFromProto(original.ToProto())
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(original.ToProto(), restored.ToProto(), protocmp.Transform()); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
