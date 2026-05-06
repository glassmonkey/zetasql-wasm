package types

import (
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// Type is the interface for all ZetaSQL types.
type Type interface {
	Kind() TypeKind
	IsArray() bool
	IsStruct() bool
	AsArray() *ArrayType
	AsStruct() *StructType
	ToProto() *generated.TypeProto
}

// scalarType represents a simple (non-compound) ZetaSQL type.
type scalarType struct {
	kind TypeKind
}

func (t *scalarType) Kind() TypeKind        { return t.kind }
func (t *scalarType) IsArray() bool         { return false }
func (t *scalarType) IsStruct() bool        { return false }
func (t *scalarType) AsArray() *ArrayType   { return nil }
func (t *scalarType) AsStruct() *StructType { return nil }
func (t *scalarType) ToProto() *generated.TypeProto {
	k := t.kind.toProto()
	return &generated.TypeProto{TypeKind: &k}
}

// Singletons for all scalar types.
var (
	int32Type      = &scalarType{kind: Int32}
	int64Type      = &scalarType{kind: Int64}
	uint32Type     = &scalarType{kind: Uint32}
	uint64Type     = &scalarType{kind: Uint64}
	boolType       = &scalarType{kind: Bool}
	floatType      = &scalarType{kind: Float}
	doubleType     = &scalarType{kind: Double}
	stringType     = &scalarType{kind: String}
	bytesType      = &scalarType{kind: Bytes}
	dateType       = &scalarType{kind: Date}
	timestampType  = &scalarType{kind: Timestamp}
	timeType       = &scalarType{kind: Time}
	datetimeType   = &scalarType{kind: Datetime}
	geographyType  = &scalarType{kind: Geography}
	numericType    = &scalarType{kind: Numeric}
	bigNumericType = &scalarType{kind: BigNumeric}
	jsonType       = &scalarType{kind: Json}
	intervalType   = &scalarType{kind: Interval}
)

func Int32Type() Type      { return int32Type }
func Int64Type() Type      { return int64Type }
func Uint32Type() Type     { return uint32Type }
func Uint64Type() Type     { return uint64Type }
func BoolType() Type       { return boolType }
func FloatType() Type      { return floatType }
func DoubleType() Type     { return doubleType }
func StringType() Type     { return stringType }
func BytesType() Type      { return bytesType }
func DateType() Type       { return dateType }
func TimestampType() Type  { return timestampType }
func TimeType() Type       { return timeType }
func DatetimeType() Type   { return datetimeType }
func GeographyType() Type  { return geographyType }
func NumericType() Type    { return numericType }
func BigNumericType() Type { return bigNumericType }
func JsonType() Type       { return jsonType }
func IntervalType() Type   { return intervalType }

// scalarTypes maps TypeKind to the singleton scalar Type.
var scalarTypes = map[TypeKind]Type{
	Int32:      int32Type,
	Int64:      int64Type,
	Uint32:     uint32Type,
	Uint64:     uint64Type,
	Bool:       boolType,
	Float:      floatType,
	Double:     doubleType,
	String:     stringType,
	Bytes:      bytesType,
	Date:       dateType,
	Timestamp:  timestampType,
	Time:       timeType,
	Datetime:   datetimeType,
	Geography:  geographyType,
	Numeric:    numericType,
	BigNumeric: bigNumericType,
	Json:       jsonType,
	Interval:   intervalType,
}

// TypeFromKind returns the singleton Type for a scalar kind. Composite kinds
// (Array, Struct) and reference kinds (Enum, Proto, Extended) return nil
// since they require element / field information that the kind alone
// cannot supply. Callers can distinguish "scalar with this kind" from
// "compound or unknown" via a nil-check.
func TypeFromKind(k TypeKind) Type {
	return scalarTypes[k]
}
