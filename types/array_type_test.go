package types

import "testing"

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
