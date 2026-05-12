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
// ARRAY recursion, STRUCT recursion, ENUM literals (proto enum number
// surfaces as int32), and the documented gap kinds (DATETIME has a
// Type but no wrappable Value yet) plus a defensive case for
// malformed STRUCT input.
//
// Triangulation across nil / NULL / two scalars / two compounds / an
// enum / one gap / one defensive case so a regression in any single
// dispatch branch shows up in the diff.
func TestWrapLiteralValue(t *testing.T) {
	arrayKind := generated.TypeKind_TYPE_ARRAY
	structKind := generated.TypeKind_TYPE_STRUCT
	enumKind := generated.TypeKind_TYPE_ENUM

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
			name: "ENUM DateTimestampPart=DAY (3) yields Value=int32(3)",
			in: &generated.ValueWithTypeProto{
				Type: &generated.TypeProto{
					TypeKind: &enumKind,
					EnumType: &generated.EnumTypeProto{
						EnumName: ptr("zetasql.functions.DateTimestampPart"),
					},
				},
				Value: &generated.ValueProto{
					Value: &generated.ValueProto_EnumValue{EnumValue: 3},
				},
			},
			want: &LiteralValue{
				Type:  &EnumType{Name: "zetasql.functions.DateTimestampPart"},
				Value: int32(3),
			},
		},
		{
			name: "DATETIME = 2026-05-10 12:34:56.123456789 (proto Datetime message wrapped)",
			in: &generated.ValueWithTypeProto{
				Type: typeOf(generated.TypeKind_TYPE_DATETIME),
				Value: &generated.ValueProto{
					Value: &generated.ValueProto_DatetimeValue{
						DatetimeValue: &generated.ValueProto_Datetime{
							// Packed64DatetimeSeconds for 2026-05-10 12:34:56.
							BitFieldDatetimeSeconds: ptr(int64(
								(2026 << 26) | (5 << 22) | (10 << 17) |
									(12 << 12) | (34 << 6) | 56,
							)),
							Nanos: ptr(int32(123456789)),
						},
					},
				},
			},
			want: &LiteralValue{
				Type: DatetimeType(),
				Value: &generated.ValueProto_Datetime{
					BitFieldDatetimeSeconds: ptr(int64(
						(2026 << 26) | (5 << 22) | (10 << 17) |
							(12 << 12) | (34 << 6) | 56,
					)),
					Nanos: ptr(int32(123456789)),
				},
			},
		},
		{
			name: "TIME = 12:34:56.123456789 (Packed64TimeNanos bit field wrapped)",
			in: &generated.ValueWithTypeProto{
				Type: typeOf(generated.TypeKind_TYPE_TIME),
				Value: &generated.ValueProto{
					Value: &generated.ValueProto_TimeValue{
						// Packed64TimeNanos: ((hour<<12)|(min<<6)|sec) << 30 | nano
						TimeValue: ((int64(12) << 12) | (int64(34) << 6) | 56) << 30 |
							123456789,
					},
				},
			},
			want: &LiteralValue{
				Type: TimeType(),
				Value: ((int64(12) << 12) | (int64(34) << 6) | 56) << 30 | 123456789,
			},
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

// TestLiteralValue_TypedAccessors_HappyPath reads as a spec table:
// each row says "for this Type and this Value, the accessor returns
// `want` with ok=true." Composite kinds use a one-element fixture
// so the observation is on this accessor's own contract, not on
// recursive wrapping (which TestWrapLiteralValue already covers).
func TestLiteralValue_TypedAccessors_HappyPath(t *testing.T) {
	timestampFixture := timestamppb.New(time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))

	tests := []struct {
		name string
		in   *LiteralValue
		want any
		call func(*LiteralValue) (any, bool)
	}{
		{
			name: "AsInt32",
			in:   &LiteralValue{Type: Int32Type(), Value: int32(7)},
			want: int32(7),
			call: func(v *LiteralValue) (any, bool) { return v.AsInt32() },
		},
		{
			name: "AsInt64",
			in:   &LiteralValue{Type: Int64Type(), Value: int64(42)},
			want: int64(42),
			call: func(v *LiteralValue) (any, bool) { return v.AsInt64() },
		},
		{
			name: "AsUint32",
			in:   &LiteralValue{Type: Uint32Type(), Value: uint32(7)},
			want: uint32(7),
			call: func(v *LiteralValue) (any, bool) { return v.AsUint32() },
		},
		{
			name: "AsUint64",
			in:   &LiteralValue{Type: Uint64Type(), Value: uint64(42)},
			want: uint64(42),
			call: func(v *LiteralValue) (any, bool) { return v.AsUint64() },
		},
		{
			name: "AsBool",
			in:   &LiteralValue{Type: BoolType(), Value: true},
			want: true,
			call: func(v *LiteralValue) (any, bool) { return v.AsBool() },
		},
		{
			name: "AsFloat",
			in:   &LiteralValue{Type: FloatType(), Value: float32(1.5)},
			want: float32(1.5),
			call: func(v *LiteralValue) (any, bool) { return v.AsFloat() },
		},
		{
			name: "AsDouble",
			in:   &LiteralValue{Type: DoubleType(), Value: float64(2.5)},
			want: float64(2.5),
			call: func(v *LiteralValue) (any, bool) { return v.AsDouble() },
		},
		{
			name: "AsString",
			in:   &LiteralValue{Type: StringType(), Value: "hello"},
			want: "hello",
			call: func(v *LiteralValue) (any, bool) { return v.AsString() },
		},
		{
			name: "AsBytes",
			in:   &LiteralValue{Type: BytesType(), Value: []byte{0x01, 0x02}},
			want: []byte{0x01, 0x02},
			call: func(v *LiteralValue) (any, bool) { return v.AsBytes() },
		},
		{
			name: "AsJson",
			in:   &LiteralValue{Type: JsonType(), Value: `{"k":1}`},
			want: `{"k":1}`,
			call: func(v *LiteralValue) (any, bool) { return v.AsJson() },
		},
		{
			name: "AsDateDays",
			in:   &LiteralValue{Type: DateType(), Value: int32(20000)},
			want: int32(20000),
			call: func(v *LiteralValue) (any, bool) { return v.AsDateDays() },
		},
		{
			name: "AsTimeMicros",
			in:   &LiteralValue{Type: TimeType(), Value: int64(123456789)},
			want: int64(123456789),
			call: func(v *LiteralValue) (any, bool) { return v.AsTimeMicros() },
		},
		{
			name: "AsTimeOfDay decodes Packed64TimeNanos 12:34:56.123456789",
			in: &LiteralValue{
				Type:  TimeType(),
				Value: ((int64(12) << 12) | (int64(34) << 6) | 56) << 30 | 123456789,
			},
			want: time.Date(1, time.January, 1, 12, 34, 56, 123456789, time.UTC),
			call: func(v *LiteralValue) (any, bool) { return v.AsTimeOfDay() },
		},
		{
			name: "AsDatetime decodes Packed64DatetimeSeconds + Nanos 2026-05-10 12:34:56.123456789",
			in: &LiteralValue{
				Type: DatetimeType(),
				Value: &generated.ValueProto_Datetime{
					BitFieldDatetimeSeconds: ptr(int64(
						(2026 << 26) | (5 << 22) | (10 << 17) | (12 << 12) | (34 << 6) | 56,
					)),
					Nanos: ptr(int32(123456789)),
				},
			},
			want: time.Date(2026, time.May, 10, 12, 34, 56, 123456789, time.UTC),
			call: func(v *LiteralValue) (any, bool) { return v.AsDatetime() },
		},
		{
			name: "AsTimestamp",
			in:   &LiteralValue{Type: TimestampType(), Value: timestampFixture},
			want: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
			call: func(v *LiteralValue) (any, bool) { return v.AsTimestamp() },
		},
		{
			name: "AsArray",
			in: &LiteralValue{
				Type:  &ArrayType{ElementType: Int64Type()},
				Value: ArrayValue{{Type: Int64Type(), Value: int64(1)}},
			},
			want: ArrayValue{{Type: Int64Type(), Value: int64(1)}},
			call: func(v *LiteralValue) (any, bool) { return v.AsArray() },
		},
		{
			name: "AsStruct",
			in: &LiteralValue{
				Type:  &StructType{Fields: []*StructField{{Name: "a", Type: Int64Type()}}},
				Value: StructValue{{Type: Int64Type(), Value: int64(1)}},
			},
			want: StructValue{{Type: Int64Type(), Value: int64(1)}},
			call: func(v *LiteralValue) (any, bool) { return v.AsStruct() },
		},
		{
			name: "AsNumeric",
			in:   &LiteralValue{Type: NumericType(), Value: []byte{0xAA}},
			want: []byte{0xAA},
			call: func(v *LiteralValue) (any, bool) { return v.AsNumeric() },
		},
		{
			name: "AsBigNumeric",
			in:   &LiteralValue{Type: BigNumericType(), Value: []byte{0xBB}},
			want: []byte{0xBB},
			call: func(v *LiteralValue) (any, bool) { return v.AsBigNumeric() },
		},
		{
			name: "AsInterval",
			in:   &LiteralValue{Type: IntervalType(), Value: []byte{0xCC}},
			want: []byte{0xCC},
			call: func(v *LiteralValue) (any, bool) { return v.AsInterval() },
		},
		{
			name: "AsGeography",
			in:   &LiteralValue{Type: GeographyType(), Value: []byte{0xDD}},
			want: []byte{0xDD},
			call: func(v *LiteralValue) (any, bool) { return v.AsGeography() },
		},
		{
			name: "AsEnumNumber",
			in: &LiteralValue{
				Type:  &EnumType{Name: "zetasql.functions.DateTimestampPart"},
				Value: int32(3),
			},
			want: int32(3),
			call: func(v *LiteralValue) (any, bool) { return v.AsEnumNumber() },
		},
		{
			name: "AsEnumName resolves DAY via protoregistry",
			in: &LiteralValue{
				Type:  &EnumType{Name: "zetasql.functions.DateTimestampPart"},
				Value: int32(3),
			},
			want: "DAY",
			call: func(v *LiteralValue) (any, bool) { return v.AsEnumName() },
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

// TestLiteralValue_TypedAccessors_ContractViolation pins the four
// "(zero, false)" conditions that hold for every typed accessor
// regardless of kind:
//
//   - nil receiver — calling on a nil *LiteralValue;
//   - nil Type — Type is missing entirely;
//   - kind mismatch — Type carries a different kind from the
//     accessor's;
//   - value type mismatch — Type matches but Value's Go type does
//     not (this also covers SQL NULL: Value=nil fails the type
//     assertion the same way).
//
// The table here only carries what the contract checks need (typ,
// wrongTyp, wrongValue, zero, call) — happy-path inputs live next
// to TestLiteralValue_TypedAccessors_HappyPath, so each test reads
// as one self-contained spec.
func TestLiteralValue_TypedAccessors_ContractViolation(t *testing.T) {
	accessors := []struct {
		name       string
		typ        Type
		wrongTyp   Type
		wrongValue any
		zero       any
		call       func(*LiteralValue) (any, bool)
	}{
		{
			name: "AsInt32", typ: Int32Type(), wrongTyp: Int64Type(),
			wrongValue: "not-int32", zero: int32(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsInt32() },
		},
		{
			name: "AsInt64", typ: Int64Type(), wrongTyp: Int32Type(),
			wrongValue: "not-int64", zero: int64(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsInt64() },
		},
		{
			name: "AsUint32", typ: Uint32Type(), wrongTyp: Int64Type(),
			wrongValue: "not-uint32", zero: uint32(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsUint32() },
		},
		{
			name: "AsUint64", typ: Uint64Type(), wrongTyp: Int64Type(),
			wrongValue: "not-uint64", zero: uint64(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsUint64() },
		},
		{
			name: "AsBool", typ: BoolType(), wrongTyp: Int64Type(),
			wrongValue: int64(1), zero: false,
			call: func(v *LiteralValue) (any, bool) { return v.AsBool() },
		},
		{
			name: "AsFloat", typ: FloatType(), wrongTyp: Int64Type(),
			wrongValue: float64(1), zero: float32(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsFloat() },
		},
		{
			name: "AsDouble", typ: DoubleType(), wrongTyp: Int64Type(),
			wrongValue: float32(1), zero: float64(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsDouble() },
		},
		{
			name: "AsString", typ: StringType(), wrongTyp: Int64Type(),
			wrongValue: []byte("not-string"), zero: "",
			call: func(v *LiteralValue) (any, bool) { return v.AsString() },
		},
		{
			name: "AsBytes", typ: BytesType(), wrongTyp: Int64Type(),
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsBytes() },
		},
		{
			name: "AsJson", typ: JsonType(), wrongTyp: Int64Type(),
			wrongValue: []byte("not-json"), zero: "",
			call: func(v *LiteralValue) (any, bool) { return v.AsJson() },
		},
		{
			name: "AsDateDays", typ: DateType(), wrongTyp: Int64Type(),
			wrongValue: int64(1), zero: int32(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsDateDays() },
		},
		{
			name: "AsTimeMicros", typ: TimeType(), wrongTyp: Int64Type(),
			wrongValue: int32(1), zero: int64(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsTimeMicros() },
		},
		{
			name: "AsTimeOfDay", typ: TimeType(), wrongTyp: Int64Type(),
			wrongValue: "not-int64", zero: time.Time{},
			call: func(v *LiteralValue) (any, bool) { return v.AsTimeOfDay() },
		},
		{
			name: "AsDatetime", typ: DatetimeType(), wrongTyp: Int64Type(),
			wrongValue: int64(1), zero: time.Time{},
			call: func(v *LiteralValue) (any, bool) { return v.AsDatetime() },
		},
		{
			name: "AsTimestamp", typ: TimestampType(), wrongTyp: Int64Type(),
			wrongValue: int64(1), zero: time.Time{},
			call: func(v *LiteralValue) (any, bool) { return v.AsTimestamp() },
		},
		{
			name: "AsArray", typ: &ArrayType{ElementType: Int64Type()}, wrongTyp: Int64Type(),
			wrongValue: "not-array", zero: (ArrayValue)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsArray() },
		},
		{
			name:       "AsStruct",
			typ:        &StructType{Fields: []*StructField{{Name: "a", Type: Int64Type()}}},
			wrongTyp:   Int64Type(),
			wrongValue: "not-struct", zero: (StructValue)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsStruct() },
		},
		{
			name: "AsNumeric", typ: NumericType(), wrongTyp: Int64Type(),
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsNumeric() },
		},
		{
			name: "AsBigNumeric", typ: BigNumericType(), wrongTyp: Int64Type(),
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsBigNumeric() },
		},
		{
			name: "AsInterval", typ: IntervalType(), wrongTyp: Int64Type(),
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsInterval() },
		},
		{
			name: "AsGeography", typ: GeographyType(), wrongTyp: Int64Type(),
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsGeography() },
		},
		{
			name:       "AsEnumNumber",
			typ:        &EnumType{Name: "zetasql.functions.DateTimestampPart"},
			wrongTyp:   Int64Type(),
			wrongValue: "not-int32",
			zero:       int32(0),
			call:       func(v *LiteralValue) (any, bool) { return v.AsEnumNumber() },
		},
		{
			name:       "AsEnumName",
			typ:        &EnumType{Name: "zetasql.functions.DateTimestampPart"},
			wrongTyp:   Int64Type(),
			wrongValue: "not-int32",
			zero:       "",
			call:       func(v *LiteralValue) (any, bool) { return v.AsEnumName() },
		},
	}

	for _, a := range accessors {
		t.Run(a.name, func(t *testing.T) {
			violations := []struct {
				name string
				in   *LiteralValue
			}{
				{"nil receiver", nil},
				{"nil Type", &LiteralValue{Type: nil, Value: a.wrongValue}},
				{"kind mismatch", &LiteralValue{Type: a.wrongTyp, Value: nil}},
				{"value type mismatch", &LiteralValue{Type: a.typ, Value: a.wrongValue}},
				{"NULL (Value=nil)", &LiteralValue{Type: a.typ, Value: nil}},
			}
			for _, v := range violations {
				t.Run(v.name, func(t *testing.T) {
					// Arrange
					sut := v.in

					// Act
					got, ok := a.call(sut)

					// Assert
					assert.False(t, ok)
					assert.Equal(t, a.zero, got)
				})
			}
		})
	}
}

// TestLiteralValue_TypedNilPointer pins the guard that accessors
// returning a time.Time apply on top of asScalar: a *typed* nil
// pointer (the Value's dynamic type matches the accessor's expected
// pointer type, but the pointer itself is nil) is reported as
// (zero, false). asScalar alone cannot reject this case because the
// type assertion succeeds on a typed nil, and proto-go's generated
// Get* methods are nil-safe and return zeroes — without the explicit
// `X == nil` guard, the accessor would silently return a normalized
// time.Date(0, 0, 0, ...) with ok=true instead of (zero, false).
//
// Every accessor that returns a time.Time over a proto pointer Value
// shares this guard; the table lists them so a new accessor of the
// same shape only needs one row, and a regression in any guard shows
// up as one specific row failing.
func TestLiteralValue_TypedNilPointer(t *testing.T) {
	tests := []struct {
		name string
		sut  *LiteralValue
		call func(*LiteralValue) (time.Time, bool)
	}{
		{
			name: "AsTimestamp on (*timestamppb.Timestamp)(nil)",
			sut:  &LiteralValue{Type: TimestampType(), Value: (*timestamppb.Timestamp)(nil)},
			call: func(v *LiteralValue) (time.Time, bool) { return v.AsTimestamp() },
		},
		{
			name: "AsDatetime on (*generated.ValueProto_Datetime)(nil)",
			sut:  &LiteralValue{Type: DatetimeType(), Value: (*generated.ValueProto_Datetime)(nil)},
			call: func(v *LiteralValue) (time.Time, bool) { return v.AsDatetime() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.sut

			// Act
			got, ok := tt.call(sut)

			// Assert
			assert.False(t, ok)
			assert.Equal(t, time.Time{}, got)
		})
	}
}

// TestLiteralValue_AsDatetime_DecodesPacked64DatetimeSeconds documents
// the proto encoding the accessor consumes:
//
//	BitFieldDatetimeSeconds (int64) holds a Packed64DatetimeSeconds
//	bit field (civil_time.h):
//	  bits 0-5    second (6 bits)
//	  bits 6-11   minute (6 bits)
//	  bits 12-16  hour   (5 bits)
//	  bits 17-21  day    (5 bits)
//	  bits 22-25  month  (4 bits)
//	  bits 26-39  year   (14 bits)
//	Nanos (int32) carries the nanosecond fraction in a separate field,
//	NOT packed into the int64.
//
// The cases triangulate this contract: every component decodes from
// the right bit window, Nanos rides as its own field (not multiplied
// into microseconds), and the high year / max-second boundaries do
// not lose bits. A regression in any shift / mask in the decoder
// shows up as one specific row failing.
func TestLiteralValue_AsDatetime_DecodesPacked64DatetimeSeconds(t *testing.T) {
	packSeconds := func(year, month, day, hour, minute, second int64) int64 {
		return (year << 26) | (month << 22) | (day << 17) |
			(hour << 12) | (minute << 6) | second
	}

	tests := []struct {
		name string
		dt   *generated.ValueProto_Datetime
		want time.Time
	}{
		{
			name: "midnight 0001-01-01 (Go zero-ish, all components at min) with Nanos=0",
			dt: &generated.ValueProto_Datetime{
				BitFieldDatetimeSeconds: ptr(packSeconds(1, 1, 1, 0, 0, 0)),
				Nanos:                   ptr(int32(0)),
			},
			want: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "2026-05-10 12:34:56 with Nanos=123456789 (Nanos rides as separate field)",
			dt: &generated.ValueProto_Datetime{
				BitFieldDatetimeSeconds: ptr(packSeconds(2026, 5, 10, 12, 34, 56)),
				Nanos:                   ptr(int32(123456789)),
			},
			want: time.Date(2026, time.May, 10, 12, 34, 56, 123456789, time.UTC),
		},
		{
			name: "max year 9999-12-31 23:59:59 with Nanos=999999999 (14-bit year, no truncation)",
			dt: &generated.ValueProto_Datetime{
				BitFieldDatetimeSeconds: ptr(packSeconds(9999, 12, 31, 23, 59, 59)),
				Nanos:                   ptr(int32(999999999)),
			},
			want: time.Date(9999, time.December, 31, 23, 59, 59, 999999999, time.UTC),
		},
		{
			name: "Nanos field absent (proto Get* returns 0) decodes to zero nanoseconds",
			dt: &generated.ValueProto_Datetime{
				BitFieldDatetimeSeconds: ptr(packSeconds(2026, 5, 10, 12, 34, 56)),
				Nanos:                   nil,
			},
			want: time.Date(2026, time.May, 10, 12, 34, 56, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := &LiteralValue{Type: DatetimeType(), Value: tt.dt}

			// Act
			got, ok := sut.AsDatetime()

			// Assert
			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestLiteralValue_AsTimeOfDay_DecodesPacked64TimeNanos documents the
// proto encoding the accessor consumes:
//
//	time_value (int64) holds a Packed64TimeNanos bit field
//	(civil_time.h):
//	  bits 0-29   nanosecond (30 bits)
//	  bits 30-35  second     (6 bits)
//	  bits 36-41  minute     (6 bits)
//	  bits 42-46  hour       (5 bits)
//	Unlike DATETIME the nanos are packed INTO the int64, not carried
//	as a separate field — so the decoder must use the 30-bit
//	NanosShift, not the 20-bit MicrosShift that some legacy decoders
//	use.
//
// The date portion of the returned time.Time is fixed to the Go zero
// date (year=1, month=January, day=1) in UTC; callers that only
// consume the time-of-day components (e.g. format with
// "15:04:05.999999999") get the right answer.
func TestLiteralValue_AsTimeOfDay_DecodesPacked64TimeNanos(t *testing.T) {
	packNanos := func(hour, minute, second, nano int64) int64 {
		return (((hour << 12) | (minute << 6) | second) << 30) | nano
	}

	tests := []struct {
		name string
		bits int64
		want time.Time
	}{
		{
			name: "00:00:00.000000000 (all zero bits)",
			bits: packNanos(0, 0, 0, 0),
			want: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "12:34:56.123456789 (mid-range with non-zero nanos)",
			bits: packNanos(12, 34, 56, 123456789),
			want: time.Date(1, time.January, 1, 12, 34, 56, 123456789, time.UTC),
		},
		{
			name: "23:59:59.999999999 (max time-of-day, 30-bit nanos at boundary)",
			bits: packNanos(23, 59, 59, 999999999),
			want: time.Date(1, time.January, 1, 23, 59, 59, 999999999, time.UTC),
		},
		{
			name: "00:00:00.000000001 (smallest nano, isolates nanos low bit)",
			bits: packNanos(0, 0, 0, 1),
			want: time.Date(1, time.January, 1, 0, 0, 0, 1, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := &LiteralValue{Type: TimeType(), Value: tt.bits}

			// Act
			got, ok := sut.AsTimeOfDay()

			// Assert
			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestLiteralValue_AsEnumName_NameOfFailure covers the AsEnumName
// failure modes that AsEnumNumber (which stops at asScalar) cannot
// reach: kind matches and Value is a valid int32, but the descriptor
// lookup or value-by-number lookup fails. The number/typed-accessor
// contract is already pinned by the shared ContractViolation table;
// this test pins the NameOf-specific fall-throughs.
func TestLiteralValue_AsEnumName_NameOfFailure(t *testing.T) {
	tests := []struct {
		name string
		in   *LiteralValue
	}{
		{
			name: "registered enum but undefined number",
			in: &LiteralValue{
				Type:  &EnumType{Name: "zetasql.functions.DateTimestampPart"},
				Value: int32(9999),
			},
		},
		{
			name: "unregistered enum name",
			in: &LiteralValue{
				Type:  &EnumType{Name: "no.such.Enum"},
				Value: int32(1),
			},
		},
		{
			name: "empty enum name",
			in: &LiteralValue{
				Type:  &EnumType{Name: ""},
				Value: int32(1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got, ok := sut.AsEnumName()

			// Assert
			assert.False(t, ok)
			assert.Equal(t, "", got)
		})
	}
}
