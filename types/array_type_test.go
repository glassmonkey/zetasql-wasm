package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
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
			assert.Empty(t, cmp.Diff(tt.want, arr.ToProto(), protocmp.Transform()), "ToProto() mismatch")
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
			restored, err := typeFromProto(original.ToProto())
			require.NoError(t, err)
			assert.Empty(t, cmp.Diff(original.ToProto(), restored.ToProto(), protocmp.Transform()), "round-trip mismatch")
		})
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
