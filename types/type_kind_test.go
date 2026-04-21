package types

import "testing"

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
