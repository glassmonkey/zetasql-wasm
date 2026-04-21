package types

import (
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// TypeFromProto deserializes a TypeProto into a Type.
func TypeFromProto(p *generated.TypeProto) (Type, error) {
	if p == nil {
		return nil, fmt.Errorf("nil TypeProto")
	}
	kind := TypeKind(p.GetTypeKind())
	switch kind {
	case Array:
		at := p.GetArrayType()
		if at == nil {
			return nil, fmt.Errorf("TypeProto has ARRAY kind but no ArrayType field")
		}
		elemType, err := TypeFromProto(at.GetElementType())
		if err != nil {
			return nil, fmt.Errorf("array element type: %w", err)
		}
		return NewArrayType(elemType)
	case Struct:
		st := p.GetStructType()
		if st == nil {
			return nil, fmt.Errorf("TypeProto has STRUCT kind but no StructType field")
		}
		fields := make([]*StructField, len(st.GetField()))
		for i, pf := range st.GetField() {
			ft, err := TypeFromProto(pf.GetFieldType())
			if err != nil {
				return nil, fmt.Errorf("struct field %d type: %w", i, err)
			}
			fields[i] = NewStructField(pf.GetFieldName(), ft)
		}
		return NewStructType(fields)
	default:
		return TypeFromKind(kind)
	}
}
