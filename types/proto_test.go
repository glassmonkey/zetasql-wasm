package types

import "testing"

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
