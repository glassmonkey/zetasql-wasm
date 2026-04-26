package types

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// StructField represents a single field in a ZetaSQL STRUCT type.
type StructField struct {
	Name string
	Type Type
}

// NewStructField creates a StructField with the given name and type.
func NewStructField(name string, typ Type) *StructField {
	return &StructField{Name: name, Type: typ}
}

// StructType represents a ZetaSQL STRUCT type.
type StructType struct {
	Fields []*StructField
}

// NewStructType creates a StructType with the given fields. The error return
// is preserved for symmetry with NewArrayType, but currently always returns nil.
func NewStructType(fields []*StructField) (*StructType, error) {
	if fields == nil {
		fields = []*StructField{}
	}
	return &StructType{Fields: fields}, nil
}

func (t *StructType) Kind() TypeKind        { return Struct }
func (t *StructType) IsArray() bool         { return false }
func (t *StructType) IsStruct() bool        { return true }
func (t *StructType) AsArray() *ArrayType   { return nil }
func (t *StructType) AsStruct() *StructType { return t }

func (t *StructType) ToProto() *generated.TypeProto {
	k := Struct.toProto()
	protoFields := make([]*generated.StructFieldProto, len(t.Fields))
	for i, f := range t.Fields {
		name := f.Name
		protoFields[i] = &generated.StructFieldProto{
			FieldName: &name,
			FieldType: f.Type.ToProto(),
		}
	}
	return &generated.TypeProto{
		TypeKind:   &k,
		StructType: &generated.StructTypeProto{Field: protoFields},
	}
}
