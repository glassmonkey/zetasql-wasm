package types

import (
	"time"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LiteralValue is the typed Go view of ValueWithTypeProto: a SQL
// literal value paired with its declared type.
//
// Value carries the resolved Go value, dispatched on Type.Kind():
//
//	INT32 / INT64 / UINT32 / UINT64       int32 / int64 / uint32 / uint64
//	BOOL                                  bool
//	FLOAT / DOUBLE                        float32 / float64
//	STRING / JSON                         string
//	BYTES / GEOGRAPHY / NUMERIC /
//	BIGNUMERIC / INTERVAL                 []byte (proto-encoded payload)
//	DATE                                  int32 (days since 1970-01-01)
//	TIME                                  int64 (encoded time-of-day)
//	TIMESTAMP                             *timestamppb.Timestamp
//	ARRAY                                 ArrayValue (recursive)
//	STRUCT                                StructValue (recursive)
//	NULL (proto: empty oneof)             nil
//
// Kinds whose proto representation is a nested proto message that is
// not yet wrapped (DATETIME, TIMESTAMP_PICOS, RANGE, MAP, TOKENLIST,
// UUID), and the kinds WrapType itself does not yet model (ENUM,
// PROTO, EXTENDED), currently come back with Value=nil. The wrap
// surface will grow as actual callers ask for them.
type LiteralValue struct {
	Type  Type
	Value any
}

// ArrayValue is the Value form for ARRAY literals: the element values
// in proto order. Each element carries the array's (uniform) element
// type via its own LiteralValue.Type.
type ArrayValue []*LiteralValue

// StructValue is the Value form for STRUCT literals: the field values
// in proto order. Field names live on the surrounding Type at
// LiteralValue.Type.AsStruct().Fields[i].Name.
type StructValue []*LiteralValue

// WrapLiteralValue lifts a *generated.ValueWithTypeProto into the
// typed LiteralValue DTO. Returns nil for nil input.
//
// A non-nil result with Value == nil represents a SQL NULL of the
// carried Type (the proto's "empty oneof" convention) or a Type whose
// kind is not yet wrappable.
func WrapLiteralValue(p *generated.ValueWithTypeProto) *LiteralValue {
	if p == nil {
		return nil
	}
	typ := WrapType(p.GetType())
	return &LiteralValue{
		Type:  typ,
		Value: convertValue(typ, p.GetValue()),
	}
}

// convertValue is the recursive worker that pulls a Go value out of a
// ValueProto using the surrounding Type for dispatch (the proto value
// alone does not carry its own type). Returns nil for NULL literals
// (proto oneof unset) and for kinds that are not yet wrappable.
func convertValue(typ Type, v *generated.ValueProto) any {
	if typ == nil || v == nil || v.GetValue() == nil {
		return nil
	}
	switch typ.Kind() {
	case Int32:
		return v.GetInt32Value()
	case Int64:
		return v.GetInt64Value()
	case Uint32:
		return v.GetUint32Value()
	case Uint64:
		return v.GetUint64Value()
	case Bool:
		return v.GetBoolValue()
	case Float:
		return v.GetFloatValue()
	case Double:
		return v.GetDoubleValue()
	case String:
		return v.GetStringValue()
	case Bytes:
		return v.GetBytesValue()
	case Date:
		return v.GetDateValue()
	case Time:
		return v.GetTimeValue()
	case Timestamp:
		return v.GetTimestampValue()
	case Geography:
		return v.GetGeographyValue()
	case Numeric:
		return v.GetNumericValue()
	case BigNumeric:
		return v.GetBignumericValue()
	case Json:
		return v.GetJsonValue()
	case Interval:
		return v.GetIntervalValue()
	case Array:
		elemType := typ.AsArray().ElementType
		ps := v.GetArrayValue().GetElement()
		out := make(ArrayValue, len(ps))
		for i, e := range ps {
			out[i] = &LiteralValue{Type: elemType, Value: convertValue(elemType, e)}
		}
		return out
	case Struct:
		fields := typ.AsStruct().Fields
		ps := v.GetStructValue().GetField()
		// Defensive: a malformed proto whose Value field count differs
		// from the Type's field count is not safely indexable. Surface
		// it as nil rather than risk an out-of-range read.
		if len(ps) != len(fields) {
			return nil
		}
		out := make(StructValue, len(ps))
		for i, fv := range ps {
			ft := fields[i].Type
			out[i] = &LiteralValue{Type: ft, Value: convertValue(ft, fv)}
		}
		return out
	}
	return nil
}

// Typed accessors below let callers pull a Go value out of a
// LiteralValue without restating the kind→type mapping documented on
// LiteralValue itself. Each accessor returns (zero, false) on any of
// these conditions, so callers never have to nil-check or kind-check
// before dispatching:
//
//   - the receiver is nil
//   - Type is nil
//   - Type.Kind() does not match the accessor's kind
//   - Value does not hold the documented Go type for that kind
//     (which includes the SQL NULL case, where Value is nil)
//
// The (T, bool) shape mirrors Go map / type-assertion idioms: the
// second return is the witness that the first is meaningful.

// asScalar is the shared dispatcher for scalar accessors: it folds the
// four (zero, false) conditions into one place so each typed accessor
// stays a one-liner and the contract cannot drift between them.
func asScalar[T any](v *LiteralValue, k TypeKind) (T, bool) {
	var zero T
	if v == nil || v.Type == nil || v.Type.Kind() != k {
		return zero, false
	}
	t, ok := v.Value.(T)
	if !ok {
		return zero, false
	}
	return t, true
}

// AsInt32 returns the INT32 value.
func (v *LiteralValue) AsInt32() (int32, bool) { return asScalar[int32](v, Int32) }

// AsInt64 returns the INT64 value.
func (v *LiteralValue) AsInt64() (int64, bool) { return asScalar[int64](v, Int64) }

// AsUint32 returns the UINT32 value.
func (v *LiteralValue) AsUint32() (uint32, bool) { return asScalar[uint32](v, Uint32) }

// AsUint64 returns the UINT64 value.
func (v *LiteralValue) AsUint64() (uint64, bool) { return asScalar[uint64](v, Uint64) }

// AsBool returns the BOOL value. The second return distinguishes a
// genuine false from a missing/mismatched value.
func (v *LiteralValue) AsBool() (bool, bool) { return asScalar[bool](v, Bool) }

// AsFloat returns the FLOAT (float32) value.
func (v *LiteralValue) AsFloat() (float32, bool) { return asScalar[float32](v, Float) }

// AsDouble returns the DOUBLE (float64) value.
func (v *LiteralValue) AsDouble() (float64, bool) { return asScalar[float64](v, Double) }

// AsString returns the STRING value.
func (v *LiteralValue) AsString() (string, bool) { return asScalar[string](v, String) }

// AsBytes returns the BYTES value.
func (v *LiteralValue) AsBytes() ([]byte, bool) { return asScalar[[]byte](v, Bytes) }

// AsJson returns the JSON value as its serialized string form.
func (v *LiteralValue) AsJson() (string, bool) { return asScalar[string](v, Json) }

// AsDateDays returns the DATE value as days since 1970-01-01.
func (v *LiteralValue) AsDateDays() (int32, bool) { return asScalar[int32](v, Date) }

// AsTimeMicros returns the TIME value in ZetaSQL's encoded
// time-of-day form (the same int64 shape the proto carries).
func (v *LiteralValue) AsTimeMicros() (int64, bool) { return asScalar[int64](v, Time) }

// AsTimestamp returns the TIMESTAMP value as a time.Time. Callers do
// not need to import google.golang.org/protobuf/types/known/timestamppb
// to consume timestamp literals.
func (v *LiteralValue) AsTimestamp() (time.Time, bool) {
	ts, ok := asScalar[*timestamppb.Timestamp](v, Timestamp)
	if !ok || ts == nil {
		return time.Time{}, false
	}
	return ts.AsTime(), true
}

// AsArray returns the ARRAY value's elements in proto order.
func (v *LiteralValue) AsArray() (ArrayValue, bool) { return asScalar[ArrayValue](v, Array) }

// AsStruct returns the STRUCT value's fields in proto order. Field
// names live on the surrounding Type at Type.AsStruct().Fields[i].Name.
func (v *LiteralValue) AsStruct() (StructValue, bool) { return asScalar[StructValue](v, Struct) }

// AsNumeric returns the NUMERIC value's proto-encoded payload.
func (v *LiteralValue) AsNumeric() ([]byte, bool) { return asScalar[[]byte](v, Numeric) }

// AsBigNumeric returns the BIGNUMERIC value's proto-encoded payload.
func (v *LiteralValue) AsBigNumeric() ([]byte, bool) { return asScalar[[]byte](v, BigNumeric) }

// AsInterval returns the INTERVAL value's proto-encoded payload.
func (v *LiteralValue) AsInterval() ([]byte, bool) { return asScalar[[]byte](v, Interval) }

// AsGeography returns the GEOGRAPHY value's proto-encoded payload.
func (v *LiteralValue) AsGeography() ([]byte, bool) { return asScalar[[]byte](v, Geography) }
