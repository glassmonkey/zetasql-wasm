package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

func TestScalarTypeSingletons(t *testing.T) {
	// Verify that accessor functions return the same instance.
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

func TestTypeKindIsSimple(t *testing.T) {
	if !Int64.IsSimple() {
		t.Error("Int64 should be simple")
	}
	if Array.IsSimple() {
		t.Error("Array should not be simple")
	}
	if Struct.IsSimple() {
		t.Error("Struct should not be simple")
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

func TestArrayType(t *testing.T) {
	arr, err := NewArrayType(Int64Type())
	if err != nil {
		t.Fatal(err)
	}
	if arr.Kind() != Array {
		t.Errorf("Kind() = %v, want Array", arr.Kind())
	}
	if !arr.IsArray() {
		t.Error("IsArray() should be true")
	}
	if arr.AsArray() != arr {
		t.Error("AsArray() should return self")
	}
	if arr.ElementType() != Int64Type() {
		t.Error("ElementType() should be Int64Type()")
	}
}

func TestArrayTypeNilElement(t *testing.T) {
	_, err := NewArrayType(nil)
	if err == nil {
		t.Error("NewArrayType(nil) should return error")
	}
}

func TestArrayOfArrayRejected(t *testing.T) {
	inner, _ := NewArrayType(StringType())
	_, err := NewArrayType(inner)
	if err == nil {
		t.Error("array of array should be rejected")
	}
}

func TestArrayTypeToProtoRoundTrip(t *testing.T) {
	arr, _ := NewArrayType(StringType())
	proto := arr.ToProto()

	restored, err := TypeFromProto(proto)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Kind() != Array {
		t.Errorf("restored Kind() = %v, want Array", restored.Kind())
	}
	restoredArr := restored.AsArray()
	if restoredArr == nil {
		t.Fatal("restored AsArray() is nil")
	}
	if restoredArr.ElementType().Kind() != String {
		t.Errorf("restored element Kind() = %v, want String", restoredArr.ElementType().Kind())
	}
}

func TestStructType(t *testing.T) {
	st, err := NewStructType([]*StructField{
		NewStructField("x", Int64Type()),
		NewStructField("y", StringType()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind() != Struct {
		t.Errorf("Kind() = %v, want Struct", st.Kind())
	}
	if !st.IsStruct() {
		t.Error("IsStruct() should be true")
	}
	if st.AsStruct() != st {
		t.Error("AsStruct() should return self")
	}
	if st.NumFields() != 2 {
		t.Fatalf("NumFields() = %d, want 2", st.NumFields())
	}
	if st.Field(0).Name() != "x" || st.Field(0).Type() != Int64Type() {
		t.Error("Field(0) mismatch")
	}
	if st.Field(1).Name() != "y" || st.Field(1).Type() != StringType() {
		t.Error("Field(1) mismatch")
	}
}

func TestStructTypeNilFields(t *testing.T) {
	st, err := NewStructType(nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.NumFields() != 0 {
		t.Errorf("NumFields() = %d, want 0", st.NumFields())
	}
}

func TestStructTypeToProtoRoundTrip(t *testing.T) {
	st, _ := NewStructType([]*StructField{
		NewStructField("id", Int64Type()),
		NewStructField("name", StringType()),
	})
	proto := st.ToProto()

	restored, err := TypeFromProto(proto)
	if err != nil {
		t.Fatal(err)
	}
	restoredSt := restored.AsStruct()
	if restoredSt == nil {
		t.Fatal("restored AsStruct() is nil")
	}
	if restoredSt.NumFields() != 2 {
		t.Fatalf("restored NumFields() = %d, want 2", restoredSt.NumFields())
	}
	if restoredSt.Field(0).Name() != "id" {
		t.Errorf("restored Field(0).Name() = %q, want %q", restoredSt.Field(0).Name(), "id")
	}
	if restoredSt.Field(1).Type().Kind() != String {
		t.Errorf("restored Field(1).Type().Kind() = %v, want String", restoredSt.Field(1).Type().Kind())
	}
}

func TestNestedArrayOfStruct(t *testing.T) {
	st, _ := NewStructType([]*StructField{
		NewStructField("a", Int64Type()),
	})
	arr, err := NewArrayType(st)
	if err != nil {
		t.Fatal(err)
	}
	proto := arr.ToProto()
	restored, err := TypeFromProto(proto)
	if err != nil {
		t.Fatal(err)
	}
	elem := restored.AsArray().ElementType()
	if elem.Kind() != Struct {
		t.Errorf("element Kind() = %v, want Struct", elem.Kind())
	}
	if elem.AsStruct().NumFields() != 1 {
		t.Errorf("element NumFields() = %d, want 1", elem.AsStruct().NumFields())
	}
}

func TestTypeFromProtoNil(t *testing.T) {
	_, err := TypeFromProto(nil)
	if err == nil {
		t.Error("TypeFromProto(nil) should return error")
	}
}
