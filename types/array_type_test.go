package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

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
	tests := []struct {
		name string
		elem Type
		want *generated.TypeProto
	}{
		{
			name: "ARRAY<STRING>",
			elem: StringType(),
			want: &generated.TypeProto{
				TypeKind: generated.TypeKind_TYPE_ARRAY.Enum(),
				ArrayType: &generated.ArrayTypeProto{
					ElementType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_STRING.Enum()},
				},
			},
		},
		{
			name: "ARRAY<INT64>",
			elem: Int64Type(),
			want: &generated.TypeProto{
				TypeKind: generated.TypeKind_TYPE_ARRAY.Enum(),
				ArrayType: &generated.ArrayTypeProto{
					ElementType: &generated.TypeProto{TypeKind: generated.TypeKind_TYPE_INT64.Enum()},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arr, _ := NewArrayType(tt.elem)
			if diff := cmp.Diff(tt.want, arr.ToProto(), protocmp.Transform()); diff != "" {
				t.Errorf("ToProto() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestArrayTypeRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		elem Type
	}{
		{"ARRAY<STRING>", StringType()},
		{"ARRAY<BOOL>", BoolType()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original, _ := NewArrayType(tt.elem)
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

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
