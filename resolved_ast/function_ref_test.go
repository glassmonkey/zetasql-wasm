package resolved_ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// TestWrapFunctionRef verifies the contract of wrapFunctionRef /
// WrapFunctionRef: nil-on-nil, zero-value on empty proto, and Name
// propagation when set.
func TestWrapFunctionRef(t *testing.T) {
	tests := []struct {
		name string
		in   *generated.FunctionRefProto
		want *FunctionRef
	}{
		{
			name: "nil proto returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty proto yields zero-valued FunctionRef",
			in:   &generated.FunctionRefProto{},
			want: &FunctionRef{},
		},
		{
			name: "populated proto propagates Name",
			in:   &generated.FunctionRefProto{Name: proto.String("ZetaSQL:add")},
			want: &FunctionRef{Name: "ZetaSQL:add"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got := WrapFunctionRef(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
