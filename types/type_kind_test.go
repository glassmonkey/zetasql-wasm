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
		{Int64, "INT64"},
		{String, "STRING"},
		{Array, "ARRAY"},
		{Struct, "STRUCT"},
		{Enum, "ENUM"},
		{Extended, "EXTENDED"},
		{TypeKind(0), "UNKNOWN"},
		// Proto values not yet exposed as named constants. Locked in to guard
		// the strip logic against future regressions and to document SQL
		// names for kinds that callers may still encounter from the wire.
		{TypeKind(28), "TOKENLIST"},
		{TypeKind(29), "RANGE"},
		{TypeKind(30), "GRAPH_ELEMENT"},
		{TypeKind(31), "MAP"},
		{TypeKind(32), "UUID"},
		{TypeKind(33), "GRAPH_PATH"},
		{TypeKind(34), "MEASURE"},
		{TypeKind(36), "ROW"},
		// Outside the proto-known range: proto returns the decimal string and
		// strip leaves it unchanged.
		{TypeKind(99), "99"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.kind.String(), "TypeKind(%d).String()", tt.kind)
	}
}
