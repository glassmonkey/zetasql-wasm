package types

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// StructField represents a single field in a ZetaSQL STRUCT type.
type StructField struct {
	name string
	typ  Type
}

// NewStructField creates a StructField with the given name and type.
func NewStructField(name string, typ Type) *StructField {
	return &StructField{name: name, typ: typ}
}

func (f *StructField) Name() string { return f.name }
func (f *StructField) Type() Type   { return f.typ }

// StructType represents a ZetaSQL STRUCT type.
type StructType struct {
	fields []*StructField
}

// NewStructType creates a StructType with the given fields.
func NewStructType(fields []*StructField) (*StructType, error) {
	if fields == nil {
		fields = []*StructField{}
	}
	return &StructType{fields: fields}, nil
}

func (t *StructType) Kind() TypeKind       { return Struct }
func (t *StructType) IsArray() bool        { return false }
func (t *StructType) IsStruct() bool       { return true }
func (t *StructType) AsArray() *ArrayType   { return nil }
func (t *StructType) AsStruct() *StructType { return t }
func (t *StructType) NumFields() int        { return len(t.fields) }
func (t *StructType) Field(i int) *StructField { return t.fields[i] }

func (t *StructType) ToProto() *generated.TypeProto {
	k := Struct.toProto()
	protoFields := make([]*generated.StructFieldProto, len(t.fields))
	for i, f := range t.fields {
		name := f.name
		protoFields[i] = &generated.StructFieldProto{
			FieldName: &name,
			FieldType: f.typ.ToProto(),
		}
	}
	return &generated.TypeProto{
		TypeKind:   &k,
		StructType: &generated.StructTypeProto{Field: protoFields},
	}
}
