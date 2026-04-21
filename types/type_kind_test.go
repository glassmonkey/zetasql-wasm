package types

import "testing"

func TestTypeKindIsSimple(t *testing.T) {
	tests := []struct {
		kind TypeKind
		want bool
	}{
		{Int32, true},
		{Int64, true},
		{String, true},
		{Bool, true},
		{Double, true},
		{Array, false},
		{Struct, false},
	}
	for _, tt := range tests {
		if got := tt.kind.IsSimple(); got != tt.want {
			t.Errorf("%v.IsSimple() = %v, want %v", tt.kind, got, tt.want)
		}
	}
}
