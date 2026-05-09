package types

import (
	"testing"
	"time"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestWrapLiteralValue covers the read-side contract end to end:
// nil-on-nil, NULL semantics (proto oneof unset → Value=nil), scalar
// dispatch round-tripping a representative set of primitive kinds,
// ARRAY recursion, STRUCT recursion, and the documented gap kinds
// (DATETIME has a Type but no wrappable Value yet) plus a defensive
// case for malformed STRUCT input.
//
// Triangulation across nil / NULL / two scalars / two compounds / one
// gap / one defensive case so a regression in any single dispatch
// branch shows up in the diff.
func TestWrapLiteralValue(t *testing.T) {
	arrayKind := generated.TypeKind_TYPE_ARRAY
	structKind := generated.TypeKind_TYPE_STRUCT

	// typeOf returns a fresh *generated.TypeProto for the given scalar
	// kind on each call. A factory (rather than function-scope shared
	// pointers) keeps per-case Arrange independent.
	typeOf := func(k generated.TypeKind) *generated.TypeProto {
		return &generated.TypeProto{TypeKind: &k}
	}
	int64Lit := func(n int64) *generated.ValueProto {
		return &generated.ValueProto{Value: &generated.ValueProto_Int64Value{Int64Value: n}}
	}
	stringLit := func(s string) *generated.ValueProto {
		return &generated.ValueProto{Value: &generated.ValueProto_StringValue{StringValue: s}}
	}

	tests := []struct {
		name string
		in   *generated.ValueWithTypeProto
		want *LiteralValue
	}{
		{
			name: "nil proto returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "NULL of INT64 (Value oneof unset) yields Value=nil",
			in: &generated.ValueWithTypeProto{
				Type:  typeOf(generated.TypeKind_TYPE_INT64),
				Value: &generated.ValueProto{},
			},
			want: &LiteralValue{Type: Int64Type(), Value: nil},
		},
		{
			name: "INT64 = 42",
			in: &generated.ValueWithTypeProto{
				Type:  typeOf(generated.TypeKind_TYPE_INT64),
				Value: int64Lit(42),
			},
			want: &LiteralValue{Type: Int64Type(), Value: int64(42)},
		},
		{
			name: "STRING = \"hello\"",
			in: &generated.ValueWithTypeProto{
				Type:  typeOf(generated.TypeKind_TYPE_STRING),
				Value: stringLit("hello"),
			},
			want: &LiteralValue{Type: StringType(), Value: "hello"},
		},
		{
			name: "ARRAY<INT64> = [1, 2]",
			in: &generated.ValueWithTypeProto{
				Type: &generated.TypeProto{
					TypeKind: &arrayKind,
					ArrayType: &generated.ArrayTypeProto{
						ElementType: typeOf(generated.TypeKind_TYPE_INT64),
					},
				},
				Value: &generated.ValueProto{
					Value: &generated.ValueProto_ArrayValue{
						ArrayValue: &generated.ValueProto_Array{
							Element: []*generated.ValueProto{int64Lit(1), int64Lit(2)},
						},
					},
				},
			},
			want: &LiteralValue{
				Type: &ArrayType{ElementType: Int64Type()},
				Value: ArrayValue{
					{Type: Int64Type(), Value: int64(1)},
					{Type: Int64Type(), Value: int64(2)},
				},
			},
		},
		{
			name: "STRUCT<a INT64, b STRING> = {42, \"x\"}",
			in: &generated.ValueWithTypeProto{
				Type: &generated.TypeProto{
					TypeKind: &structKind,
					StructType: &generated.StructTypeProto{
						Field: []*generated.StructFieldProto{
							{FieldName: ptr("a"), FieldType: typeOf(generated.TypeKind_TYPE_INT64)},
							{FieldName: ptr("b"), FieldType: typeOf(generated.TypeKind_TYPE_STRING)},
						},
					},
				},
				Value: &generated.ValueProto{
					Value: &generated.ValueProto_StructValue{
						StructValue: &generated.ValueProto_Struct{
							Field: []*generated.ValueProto{int64Lit(42), stringLit("x")},
						},
					},
				},
			},
			want: &LiteralValue{
				Type: &StructType{Fields: []*StructField{
					{Name: "a", Type: Int64Type()},
					{Name: "b", Type: StringType()},
				}},
				Value: StructValue{
					{Type: Int64Type(), Value: int64(42)},
					{Type: StringType(), Value: "x"},
				},
			},
		},
		{
			name: "DATETIME has a Type but Value is nil (nested-proto kind not yet wrapped)",
			in: &generated.ValueWithTypeProto{
				Type: typeOf(generated.TypeKind_TYPE_DATETIME),
				Value: &generated.ValueProto{
					Value: &generated.ValueProto_DatetimeValue{
						DatetimeValue: &generated.ValueProto_Datetime{},
					},
				},
			},
			want: &LiteralValue{Type: DatetimeType(), Value: nil},
		},
		{
			name: "STRUCT with mismatched field count yields Value=nil (defensive)",
			in: &generated.ValueWithTypeProto{
				Type: &generated.TypeProto{
					TypeKind: &structKind,
					StructType: &generated.StructTypeProto{
						Field: []*generated.StructFieldProto{
							{FieldName: ptr("a"), FieldType: typeOf(generated.TypeKind_TYPE_INT64)},
							{FieldName: ptr("b"), FieldType: typeOf(generated.TypeKind_TYPE_STRING)},
						},
					},
				},
				Value: &generated.ValueProto{
					Value: &generated.ValueProto_StructValue{
						StructValue: &generated.ValueProto_Struct{
							Field: []*generated.ValueProto{int64Lit(42)}, // only one
						},
					},
				},
			},
			want: &LiteralValue{
				Type: &StructType{Fields: []*StructField{
					{Name: "a", Type: Int64Type()},
					{Name: "b", Type: StringType()},
				}},
				Value: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got := WrapLiteralValue(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// accessorCase couples a typed accessor under test with the kind it
// dispatches on and a value of the wrong Go type for that kind. The
// "wrong value" is what lets the contract test confirm that a kind
// match alone is not enough — Value's Go type must also match.
type accessorCase struct {
	name        string
	kind        TypeKind
	wrongValue  any
	callBoxed   func(*LiteralValue) (any, bool)
}

// allAccessors enumerates every typed accessor on LiteralValue. The
// HappyPath and ContractViolation tests both iterate this list so a
// newly added accessor is forced into both axes by adding one row.
func allAccessors() []accessorCase {
	return []accessorCase{
		{
			name: "AsInt32", kind: Int32, wrongValue: "not-int32",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsInt32(); return g, ok },
		},
		{
			name: "AsInt64", kind: Int64, wrongValue: "not-int64",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsInt64(); return g, ok },
		},
		{
			name: "AsUint32", kind: Uint32, wrongValue: "not-uint32",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsUint32(); return g, ok },
		},
		{
			name: "AsUint64", kind: Uint64, wrongValue: "not-uint64",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsUint64(); return g, ok },
		},
		{
			name: "AsBool", kind: Bool, wrongValue: int64(1),
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsBool(); return g, ok },
		},
		{
			name: "AsFloat", kind: Float, wrongValue: float64(1),
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsFloat(); return g, ok },
		},
		{
			name: "AsDouble", kind: Double, wrongValue: float32(1),
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsDouble(); return g, ok },
		},
		{
			name: "AsString", kind: String, wrongValue: []byte("not-string"),
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsString(); return g, ok },
		},
		{
			name: "AsBytes", kind: Bytes, wrongValue: "not-bytes",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsBytes(); return g, ok },
		},
		{
			name: "AsJson", kind: Json, wrongValue: []byte("not-json"),
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsJson(); return g, ok },
		},
		{
			name: "AsDateDays", kind: Date, wrongValue: int64(1),
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsDateDays(); return g, ok },
		},
		{
			name: "AsTimeMicros", kind: Time, wrongValue: int32(1),
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsTimeMicros(); return g, ok },
		},
		{
			name: "AsTimestamp", kind: Timestamp, wrongValue: int64(1),
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsTimestamp(); return g, ok },
		},
		{
			name: "AsArray", kind: Array, wrongValue: "not-array",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsArray(); return g, ok },
		},
		{
			name: "AsStruct", kind: Struct, wrongValue: "not-struct",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsStruct(); return g, ok },
		},
		{
			name: "AsNumeric", kind: Numeric, wrongValue: "not-bytes",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsNumeric(); return g, ok },
		},
		{
			name: "AsBigNumeric", kind: BigNumeric, wrongValue: "not-bytes",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsBigNumeric(); return g, ok },
		},
		{
			name: "AsInterval", kind: Interval, wrongValue: "not-bytes",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsInterval(); return g, ok },
		},
		{
			name: "AsGeography", kind: Geography, wrongValue: "not-bytes",
			callBoxed: func(v *LiteralValue) (any, bool) { g, ok := v.AsGeography(); return g, ok },
		},
	}
}

// typeFor returns the canonical Type for kinds the accessor surface
// covers. Composite kinds (Array, Struct) get a minimal element/field
// schema so the kind dispatch can be exercised without coupling the
// test to deeper recursion (covered by TestWrapLiteralValue).
func typeFor(k TypeKind) Type {
	switch k {
	case Int32:
		return Int32Type()
	case Int64:
		return Int64Type()
	case Uint32:
		return Uint32Type()
	case Uint64:
		return Uint64Type()
	case Bool:
		return BoolType()
	case Float:
		return FloatType()
	case Double:
		return DoubleType()
	case String:
		return StringType()
	case Bytes:
		return BytesType()
	case Date:
		return DateType()
	case Time:
		return TimeType()
	case Timestamp:
		return TimestampType()
	case Geography:
		return GeographyType()
	case Numeric:
		return NumericType()
	case BigNumeric:
		return BigNumericType()
	case Json:
		return JsonType()
	case Interval:
		return IntervalType()
	case Array:
		return &ArrayType{ElementType: Int64Type()}
	case Struct:
		return &StructType{Fields: []*StructField{{Name: "a", Type: Int64Type()}}}
	}
	return nil
}

// TestLiteralValue_TypedAccessors_HappyPath checks that every accessor
// returns the documented Go value with ok=true when the LiteralValue
// is well-formed for that kind. Composite kinds use a one-element
// fixture so the assertion is on the accessor's own contract, not on
// recursive wrapping (which TestWrapLiteralValue already covers).
func TestLiteralValue_TypedAccessors_HappyPath(t *testing.T) {
	timestampFixture := timestamppb.New(time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))

	tests := []struct {
		name string
		in   *LiteralValue
		call func(*LiteralValue) (any, bool)
		want any
	}{
		{
			name: "AsInt32",
			in:   &LiteralValue{Type: Int32Type(), Value: int32(7)},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsInt32(); return g, ok },
			want: int32(7),
		},
		{
			name: "AsInt64",
			in:   &LiteralValue{Type: Int64Type(), Value: int64(42)},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsInt64(); return g, ok },
			want: int64(42),
		},
		{
			name: "AsUint32",
			in:   &LiteralValue{Type: Uint32Type(), Value: uint32(7)},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsUint32(); return g, ok },
			want: uint32(7),
		},
		{
			name: "AsUint64",
			in:   &LiteralValue{Type: Uint64Type(), Value: uint64(42)},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsUint64(); return g, ok },
			want: uint64(42),
		},
		{
			name: "AsBool",
			in:   &LiteralValue{Type: BoolType(), Value: true},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsBool(); return g, ok },
			want: true,
		},
		{
			name: "AsFloat",
			in:   &LiteralValue{Type: FloatType(), Value: float32(1.5)},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsFloat(); return g, ok },
			want: float32(1.5),
		},
		{
			name: "AsDouble",
			in:   &LiteralValue{Type: DoubleType(), Value: float64(2.5)},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsDouble(); return g, ok },
			want: float64(2.5),
		},
		{
			name: "AsString",
			in:   &LiteralValue{Type: StringType(), Value: "hello"},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsString(); return g, ok },
			want: "hello",
		},
		{
			name: "AsBytes",
			in:   &LiteralValue{Type: BytesType(), Value: []byte{0x01, 0x02}},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsBytes(); return g, ok },
			want: []byte{0x01, 0x02},
		},
		{
			name: "AsJson",
			in:   &LiteralValue{Type: JsonType(), Value: `{"k":1}`},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsJson(); return g, ok },
			want: `{"k":1}`,
		},
		{
			name: "AsDateDays",
			in:   &LiteralValue{Type: DateType(), Value: int32(20000)},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsDateDays(); return g, ok },
			want: int32(20000),
		},
		{
			name: "AsTimeMicros",
			in:   &LiteralValue{Type: TimeType(), Value: int64(123456789)},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsTimeMicros(); return g, ok },
			want: int64(123456789),
		},
		{
			name: "AsTimestamp",
			in:   &LiteralValue{Type: TimestampType(), Value: timestampFixture},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsTimestamp(); return g, ok },
			want: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
		},
		{
			name: "AsArray",
			in: &LiteralValue{
				Type:  &ArrayType{ElementType: Int64Type()},
				Value: ArrayValue{{Type: Int64Type(), Value: int64(1)}},
			},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsArray(); return g, ok },
			want: ArrayValue{{Type: Int64Type(), Value: int64(1)}},
		},
		{
			name: "AsStruct",
			in: &LiteralValue{
				Type:  &StructType{Fields: []*StructField{{Name: "a", Type: Int64Type()}}},
				Value: StructValue{{Type: Int64Type(), Value: int64(1)}},
			},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsStruct(); return g, ok },
			want: StructValue{{Type: Int64Type(), Value: int64(1)}},
		},
		{
			name: "AsNumeric",
			in:   &LiteralValue{Type: NumericType(), Value: []byte{0xAA}},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsNumeric(); return g, ok },
			want: []byte{0xAA},
		},
		{
			name: "AsBigNumeric",
			in:   &LiteralValue{Type: BigNumericType(), Value: []byte{0xBB}},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsBigNumeric(); return g, ok },
			want: []byte{0xBB},
		},
		{
			name: "AsInterval",
			in:   &LiteralValue{Type: IntervalType(), Value: []byte{0xCC}},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsInterval(); return g, ok },
			want: []byte{0xCC},
		},
		{
			name: "AsGeography",
			in:   &LiteralValue{Type: GeographyType(), Value: []byte{0xDD}},
			call: func(v *LiteralValue) (any, bool) { g, ok := v.AsGeography(); return g, ok },
			want: []byte{0xDD},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got, ok := tt.call(sut)

			// Assert
			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestLiteralValue_TypedAccessors_ContractViolation enumerates the
// four "(zero, false)" conditions documented on the typed-accessor
// block — nil receiver, nil Type, kind mismatch, and value-shape
// mismatch (which subsumes SQL NULL, since Value=nil fails the type
// assertion) — and asserts every accessor honours each one. The list
// of accessors is shared with the HappyPath test so a new accessor
// must satisfy both axes.
func TestLiteralValue_TypedAccessors_ContractViolation(t *testing.T) {
	// wrongKind picks a kind that is guaranteed to differ from the
	// accessor's own kind. Using Int64 as the default and Int32 only
	// when the accessor itself is Int64 keeps the rule a one-liner.
	wrongKind := func(self TypeKind) Type {
		if self == Int64 {
			return Int32Type()
		}
		return Int64Type()
	}

	for _, a := range allAccessors() {
		t.Run(a.name, func(t *testing.T) {
			t.Run("nil receiver", func(t *testing.T) {
				// Arrange
				var sut *LiteralValue

				// Act
				got, ok := a.callBoxed(sut)

				// Assert
				assert.False(t, ok)
				assert.Equal(t, zeroFor(a.name), got)
			})
			t.Run("nil Type", func(t *testing.T) {
				// Arrange
				sut := &LiteralValue{Type: nil, Value: a.wrongValue}

				// Act
				got, ok := a.callBoxed(sut)

				// Assert
				assert.False(t, ok)
				assert.Equal(t, zeroFor(a.name), got)
			})
			t.Run("kind mismatch", func(t *testing.T) {
				// Arrange: Type carries a kind the accessor does not
				// dispatch on. Value is left nil because we want the
				// kind check to be the one that rejects the call.
				sut := &LiteralValue{Type: wrongKind(a.kind), Value: nil}

				// Act
				got, ok := a.callBoxed(sut)

				// Assert
				assert.False(t, ok)
				assert.Equal(t, zeroFor(a.name), got)
			})
			t.Run("value type mismatch", func(t *testing.T) {
				// Arrange: kind is correct, Value's Go type is not.
				sut := &LiteralValue{Type: typeFor(a.kind), Value: a.wrongValue}

				// Act
				got, ok := a.callBoxed(sut)

				// Assert
				assert.False(t, ok)
				assert.Equal(t, zeroFor(a.name), got)
			})
			t.Run("NULL (Value=nil)", func(t *testing.T) {
				// Arrange: kind matches but Value is nil, the proto
				// "empty oneof" convention for SQL NULL.
				sut := &LiteralValue{Type: typeFor(a.kind), Value: nil}

				// Act
				got, ok := a.callBoxed(sut)

				// Assert
				assert.False(t, ok)
				assert.Equal(t, zeroFor(a.name), got)
			})
		})
	}
}

// zeroFor returns the zero value an accessor must hand back when its
// contract is violated. Listing it once, keyed by accessor name,
// keeps every contract subtest's `want` honest without each subtest
// re-deriving the zero from the accessor's signature.
func zeroFor(accessor string) any {
	switch accessor {
	case "AsInt32", "AsDateDays":
		return int32(0)
	case "AsInt64", "AsTimeMicros":
		return int64(0)
	case "AsUint32":
		return uint32(0)
	case "AsUint64":
		return uint64(0)
	case "AsBool":
		return false
	case "AsFloat":
		return float32(0)
	case "AsDouble":
		return float64(0)
	case "AsString", "AsJson":
		return ""
	case "AsBytes", "AsNumeric", "AsBigNumeric", "AsInterval", "AsGeography":
		return ([]byte)(nil)
	case "AsTimestamp":
		return time.Time{}
	case "AsArray":
		return (ArrayValue)(nil)
	case "AsStruct":
		return (StructValue)(nil)
	}
	return nil
}

// TestLiteralValue_AsTimestamp_TypedNilPointer covers a case the
// generic accessor contract cannot express: even when Value is a
// genuine *timestamppb.Timestamp (so the type assertion succeeds),
// a nil pointer must still be reported as (zero, false) because
// AsTime on a nil receiver would otherwise panic. Kept as a separate
// test so the AsTimestamp wrapping path is the only thing under
// observation.
func TestLiteralValue_AsTimestamp_TypedNilPointer(t *testing.T) {
	// Arrange
	sut := &LiteralValue{Type: TimestampType(), Value: (*timestamppb.Timestamp)(nil)}

	// Act
	got, ok := sut.AsTimestamp()

	// Assert
	assert.False(t, ok)
	assert.Equal(t, time.Time{}, got)
}
