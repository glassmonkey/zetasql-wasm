package types

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// TypeKind identifies the kind of a ZetaSQL type.
type TypeKind int32

const (
	Int32      TypeKind = TypeKind(generated.TypeKind_TYPE_INT32)
	Int64      TypeKind = TypeKind(generated.TypeKind_TYPE_INT64)
	Uint32     TypeKind = TypeKind(generated.TypeKind_TYPE_UINT32)
	Uint64     TypeKind = TypeKind(generated.TypeKind_TYPE_UINT64)
	Bool       TypeKind = TypeKind(generated.TypeKind_TYPE_BOOL)
	Float      TypeKind = TypeKind(generated.TypeKind_TYPE_FLOAT)
	Double     TypeKind = TypeKind(generated.TypeKind_TYPE_DOUBLE)
	String     TypeKind = TypeKind(generated.TypeKind_TYPE_STRING)
	Bytes      TypeKind = TypeKind(generated.TypeKind_TYPE_BYTES)
	Date       TypeKind = TypeKind(generated.TypeKind_TYPE_DATE)
	Timestamp  TypeKind = TypeKind(generated.TypeKind_TYPE_TIMESTAMP)
	Time       TypeKind = TypeKind(generated.TypeKind_TYPE_TIME)
	Datetime   TypeKind = TypeKind(generated.TypeKind_TYPE_DATETIME)
	Geography  TypeKind = TypeKind(generated.TypeKind_TYPE_GEOGRAPHY)
	Numeric    TypeKind = TypeKind(generated.TypeKind_TYPE_NUMERIC)
	BigNumeric TypeKind = TypeKind(generated.TypeKind_TYPE_BIGNUMERIC)
	Json       TypeKind = TypeKind(generated.TypeKind_TYPE_JSON)
	Interval   TypeKind = TypeKind(generated.TypeKind_TYPE_INTERVAL)
	Array      TypeKind = TypeKind(generated.TypeKind_TYPE_ARRAY)
	Struct     TypeKind = TypeKind(generated.TypeKind_TYPE_STRUCT)
)

func (k TypeKind) toProto() generated.TypeKind {
	return generated.TypeKind(k)
}

// IsSimple returns true for scalar types (not Array, Struct, etc.).
func (k TypeKind) IsSimple() bool {
	switch k {
	case Array, Struct:
		return false
	default:
		return true
	}
}
