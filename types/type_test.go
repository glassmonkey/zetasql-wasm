package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestScalarTypeSingletons(t *testing.T) {
	accessors := []func() Type{
		Int32Type, Int64Type, Uint32Type, Uint64Type,
		BoolType, FloatType, DoubleType, StringType, BytesType,
		DateType, TimestampType, TimeType, DatetimeType,
		GeographyType, NumericType, BigNumericType, JsonType, IntervalType,
	}
	for _, fn := range accessors {
		if fn() != fn() {
			t.Errorf("singleton identity broken for Kind=%v", fn().Kind())
		}
	}
}

func TestTypeInterfaceDispatch(t *testing.T) {
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
			if got := tt.typ.Kind(); got != tt.wantKind {
				t.Errorf("Kind() = %v, want %v", got, tt.wantKind)
			}
			if got := tt.typ.IsArray(); got != tt.wantArray {
				t.Errorf("IsArray() = %v, want %v", got, tt.wantArray)
			}
			if got := tt.typ.IsStruct(); got != tt.wantStruct {
				t.Errorf("IsStruct() = %v, want %v", got, tt.wantStruct)
			}
		})
	}
}

func TestTypeFromKind(t *testing.T) {
	tests := []struct {
		kind    TypeKind
		want    Type
		wantErr bool
	}{
		{Int64, Int64Type(), false},
		{String, StringType(), false},
		{Bool, BoolType(), false},
		{Array, nil, true},
		{Struct, nil, true},
	}
	for _, tt := range tests {
		got, err := TypeFromKind(tt.kind)
		if (err != nil) != tt.wantErr {
			t.Errorf("TypeFromKind(%v) error = %v, wantErr %v", tt.kind, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("TypeFromKind(%v) = %v, want %v", tt.kind, got, tt.want)
		}
	}
}

func TestScalarTypeToProtoRoundTrip(t *testing.T) {
	for kind, typ := range scalarTypes {
		got := typ.ToProto()
		want := &generated.TypeProto{TypeKind: generated.TypeKind(kind).Enum()}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("ToProto() mismatch for %v (-want +got):\n%s", kind, diff)
		}
		restored, err := TypeFromProto(got)
		if err != nil {
			t.Fatalf("TypeFromProto failed for %v: %v", kind, err)
		}
		if restored != typ {
			t.Errorf("round-trip for %v did not return same singleton", kind)
		}
	}
}
