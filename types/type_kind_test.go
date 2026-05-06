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
		{Enum, false},
		{Proto, false},
		{Extended, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.kind.IsSimple(), "%v.IsSimple()", tt.kind)
	}
}

func TestTypeKindString(t *testing.T) {
	tests := []struct {
		kind TypeKind
		want string
	}{
		{Int64, "TYPE_INT64"},
		{String, "TYPE_STRING"},
		{Array, "TYPE_ARRAY"},
		{Struct, "TYPE_STRUCT"},
		{Enum, "TYPE_ENUM"},
		{Extended, "TYPE_EXTENDED"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.kind.String(), "TypeKind(%d).String()", tt.kind)
	}
}
