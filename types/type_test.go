package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestTypeInterfaceDispatch(t *testing.T) {
	t.Parallel()
	arr, _ := NewArrayType(Int64Type())
	st, _ := NewStructType([]*StructField{NewStructField("x", Int64Type())})

	tests := []struct {
		name       string
		typ        Type
		wantKind   TypeKind
		wantArray  bool
		wantStruct bool
	}{
		{"scalar", Int64Type(), Int64, false, false},
		{"array", arr, Array, true, false},
		{"struct", st, Struct, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantKind, tt.typ.Kind())
			assert.Equal(t, tt.wantArray, tt.typ.IsArray())
			assert.Equal(t, tt.wantStruct, tt.typ.IsStruct())
		})
	}
}

func TestScalarTypeToProtoRoundTrip(t *testing.T) {
	t.Parallel()
	for kind, typ := range scalarTypes {
		got := typ.ToProto()
		want := &generated.TypeProto{TypeKind: generated.TypeKind(kind).Enum()}
		assert.Empty(t, cmp.Diff(want, got, protocmp.Transform()), "ToProto() mismatch for %v", kind)
		restored, err := typeFromProto(got)
		require.NoError(t, err, "typeFromProto failed for %v", kind)
		assert.Equal(t, typ, restored, "round-trip for %v did not return same singleton", kind)
	}
}
