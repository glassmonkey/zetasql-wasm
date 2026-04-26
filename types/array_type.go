package types

import (
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// ArrayType represents a ZetaSQL ARRAY type.
type ArrayType struct {
	ElementType Type
}

// NewArrayType creates an ArrayType with the given element type.
// ZetaSQL does not support arrays of arrays.
func NewArrayType(elementType Type) (*ArrayType, error) {
	if elementType == nil {
		return nil, fmt.Errorf("array element type must not be nil")
	}
	if elementType.Kind() == Array {
		return nil, fmt.Errorf("array of array is not supported in ZetaSQL")
	}
	return &ArrayType{ElementType: elementType}, nil
}

func (t *ArrayType) Kind() TypeKind        { return Array }
func (t *ArrayType) IsArray() bool         { return true }
func (t *ArrayType) IsStruct() bool        { return false }
func (t *ArrayType) AsArray() *ArrayType   { return t }
func (t *ArrayType) AsStruct() *StructType { return nil }

func (t *ArrayType) ToProto() *generated.TypeProto {
	k := Array.toProto()
	return &generated.TypeProto{
		TypeKind: &k,
		ArrayType: &generated.ArrayTypeProto{
			ElementType: t.ElementType.ToProto(),
		},
	}
}
