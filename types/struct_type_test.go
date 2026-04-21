package types

import "testing"

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
