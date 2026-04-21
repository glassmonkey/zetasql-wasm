package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

func TestScalarTypeSingletons(t *testing.T) {
	if Int64Type() != Int64Type() {
		t.Error("Int64Type() should return the same instance")
	}
	if StringType() != StringType() {
		t.Error("StringType() should return the same instance")
	}
}

func TestScalarTypeKind(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		kind TypeKind
	}{
		{"Int32", Int32Type(), Int32},
		{"Int64", Int64Type(), Int64},
		{"Uint32", Uint32Type(), Uint32},
		{"Uint64", Uint64Type(), Uint64},
		{"Bool", BoolType(), Bool},
		{"Float", FloatType(), Float},
		{"Double", DoubleType(), Double},
		{"String", StringType(), String},
		{"Bytes", BytesType(), Bytes},
		{"Date", DateType(), Date},
		{"Timestamp", TimestampType(), Timestamp},
		{"Time", TimeType(), Time},
		{"Datetime", DatetimeType(), Datetime},
		{"Geography", GeographyType(), Geography},
		{"Numeric", NumericType(), Numeric},
		{"BigNumeric", BigNumericType(), BigNumeric},
		{"Json", JsonType(), Json},
		{"Interval", IntervalType(), Interval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.typ.Kind() != tt.kind {
				t.Errorf("Kind() = %v, want %v", tt.typ.Kind(), tt.kind)
			}
			if tt.typ.IsArray() {
				t.Error("scalar type should not be array")
			}
			if tt.typ.IsStruct() {
				t.Error("scalar type should not be struct")
			}
			if tt.typ.AsArray() != nil {
				t.Error("AsArray() should return nil for scalar")
			}
			if tt.typ.AsStruct() != nil {
				t.Error("AsStruct() should return nil for scalar")
			}
		})
	}
}

func TestTypeFromKind(t *testing.T) {
	typ, err := TypeFromKind(Int64)
	if err != nil {
		t.Fatal(err)
	}
	if typ != Int64Type() {
		t.Error("TypeFromKind(Int64) should return Int64Type() singleton")
	}
}

func TestTypeFromKindError(t *testing.T) {
	_, err := TypeFromKind(Array)
	if err == nil {
		t.Error("TypeFromKind(Array) should return error")
	}
	_, err = TypeFromKind(Struct)
	if err == nil {
		t.Error("TypeFromKind(Struct) should return error")
	}
}

func TestScalarTypeToProtoRoundTrip(t *testing.T) {
	for kind, typ := range scalarTypes {
		proto := typ.ToProto()
		if proto.GetTypeKind() != generated.TypeKind(kind) {
			t.Errorf("ToProto().TypeKind = %v, want %v", proto.GetTypeKind(), kind)
		}
		restored, err := TypeFromProto(proto)
		if err != nil {
			t.Fatalf("TypeFromProto failed for %v: %v", kind, err)
		}
		if restored != typ {
			t.Errorf("round-trip for %v did not return same singleton", kind)
		}
	}
}
