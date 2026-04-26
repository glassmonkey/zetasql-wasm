package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		assert.Equal(t, tt.want, tt.kind.IsSimple(), "%v.IsSimple()", tt.kind)
	}
}
