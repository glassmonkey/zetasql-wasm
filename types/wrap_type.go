package types

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// WrapType lifts a *generated.TypeProto into the typed Go Type tree.
// Returns nil for nil input.
//
// Coverage:
//   - Scalar kinds — returns the singleton from scalarTypes (same value
//     Int64Type, StringType, etc. return), zero allocations.
//   - ARRAY — recurses on the element type. If the element type is not
//     wrappable (e.g. an array of ENUM today), the whole array maps to
//     nil because an array's identity is its element type.
//   - STRUCT — recurses field-by-field. A field whose Type is not
//     wrappable becomes a *StructField with Type=nil; the field name is
//     still preserved so callers iterating field names get full coverage.
//
// ENUM, PROTO, and EXTENDED return nil today: those kinds reference an
// external descriptor or extension type that types.Type does not model
// on the read side yet. Callers needing them inspect the proto directly
// until the wrap surface grows.
func WrapType(p *generated.TypeProto) Type {
	if p == nil {
		return nil
	}
	kind := TypeKind(p.GetTypeKind())
	if t := TypeFromKind(kind); t != nil {
		return t
	}
	switch kind {
	case Array:
		elem := WrapType(p.GetArrayType().GetElementType())
		if elem == nil {
			return nil
		}
		return &ArrayType{ElementType: elem}
	case Struct:
		protoFields := p.GetStructType().GetField()
		fields := make([]*StructField, len(protoFields))
		for i, f := range protoFields {
			fields[i] = &StructField{
				Name: f.GetFieldName(),
				Type: WrapType(f.GetFieldType()),
			}
		}
		return &StructType{Fields: fields}
	}
	return nil
}
