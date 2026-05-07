package types

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

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
