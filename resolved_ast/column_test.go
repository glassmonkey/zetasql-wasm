package resolved_ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// TestWrapColumn covers the contract of wrapColumn / WrapColumn:
// scalar fields are populated from the proto getters (which themselves
// are nil-safe), Type and AnnotationMap pointer fields are forwarded
// without copying, and a nil input maps to a nil output. Triangulated
// across nil, an unset proto, and a populated proto so that a future
// regression in any field's wiring shows up in the diff.
func TestWrapColumn(t *testing.T) {
	typ := &generated.TypeProto{TypeKind: ptr(generated.TypeKind_TYPE_INT64)}
	annot := &generated.AnnotationMapProto{}

	tests := []struct {
		name string
		in   *generated.ResolvedColumnProto
		want *Column
	}{
		{
			name: "nil proto returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty proto yields zero-valued Column",
			in:   &generated.ResolvedColumnProto{},
			want: &Column{},
		},
		{
			name: "populated proto fields are propagated",
			in: &generated.ResolvedColumnProto{
				ColumnId:      proto.Int64(7),
				TableName:     proto.String("orders"),
				Name:          proto.String("total"),
				Type:          typ,
				AnnotationMap: annot,
			},
			want: &Column{
				ID:            7,
				TableName:     "orders",
				Name:          "total",
				Type:          typ,
				AnnotationMap: annot,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got := WrapColumn(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestWrapColumnSlice verifies that nil/empty/populated input slices
// map to nil/empty/populated output respectively and that nil elements
// inside a non-empty slice survive as nil entries.
func TestWrapColumnSlice(t *testing.T) {
	a := &generated.ResolvedColumnProto{ColumnId: proto.Int64(1), Name: proto.String("a")}
	b := &generated.ResolvedColumnProto{ColumnId: proto.Int64(2), Name: proto.String("b")}

	tests := []struct {
		name string
		in   []*generated.ResolvedColumnProto
		want []*Column
	}{
		{
			name: "nil input returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty input returns empty slice",
			in:   []*generated.ResolvedColumnProto{},
			want: []*Column{},
		},
		{
			name: "populated and nil entries are wrapped element-wise",
			in:   []*generated.ResolvedColumnProto{a, nil, b},
			want: []*Column{
				{ID: 1, Name: "a"},
				nil,
				{ID: 2, Name: "b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got := wrapColumnSlice(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

