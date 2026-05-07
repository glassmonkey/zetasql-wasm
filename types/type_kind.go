package types

import (
	"strings"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

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
	Enum       TypeKind = TypeKind(generated.TypeKind_TYPE_ENUM)
	Proto      TypeKind = TypeKind(generated.TypeKind_TYPE_PROTO)
	Extended   TypeKind = TypeKind(generated.TypeKind_TYPE_EXTENDED)
)

func (k TypeKind) toProto() generated.TypeKind {
	return generated.TypeKind(k)
}

// String returns the SQL name of the kind (e.g. "INT64", "ARRAY"). The
// underlying proto enum name carries a "TYPE_" prefix that is a wire-format
// implementation detail; this method strips it so callers can use the result
// directly in SQL contexts (DDL column types, error messages) without having
// to know about the proto layer. Returns "UNKNOWN" for the zero value and
// the decimal string of the kind for values outside the known enum range.
func (k TypeKind) String() string {
	return strings.TrimPrefix(generated.TypeKind(k).String(), "TYPE_")
}

// IsSimple returns true for scalar types whose value can stand alone without
// referencing an external descriptor or element schema. Composite kinds
// (Array, Struct), reference kinds (Enum, Proto), and the open-ended Extended
// kind all return false.
func (k TypeKind) IsSimple() bool {
	switch k {
	case Array, Struct, Enum, Proto, Extended:
		return false
	default:
		return true
	}
}
