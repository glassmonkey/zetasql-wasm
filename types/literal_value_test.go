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

// accessorCase pins one typed accessor's contract on a single row.
// Each field corresponds to one axis the accessor's contract is
// checked on:
//
//	typ         The Type the accessor dispatches on (Type.Kind() must
//	            match for the accessor to return a real value).
//	happyValue  A Value of the Go type the accessor expects, paired
//	            with typ. Feeding (typ, happyValue) into a LiteralValue
//	            is the canonical happy input.
//	happyWant   The value the accessor must return for that input,
//	            with ok=true.
//	wrongTyp    Any Type whose Kind() differs from typ.Kind(). Used
//	            for the kind-mismatch axis: the surrounding Type lies,
//	            so the accessor must reject with (zero, false).
//	wrongValue  A Value whose Go type does not match what the accessor
//	            expects, kept under the correct typ. Used for the
//	            value-shape-mismatch axis (which subsumes SQL NULL,
//	            since Value=nil fails the type assertion the same way).
//	zero        The accessor's documented zero return — what every
//	            (zero, false) case must produce.
//	call        Invokes the accessor on the supplied LiteralValue and
//	            returns its (got, ok) pair widened to (any, bool) so
//	            every row shares one signature.
//
// Holding all of this in the row keeps the table the single source
// of truth: adding an accessor means adding one row that pins every
// axis at once, and HappyPath and ContractViolation will both pick
// it up automatically.
type accessorCase struct {
	name       string
	typ        Type
	wrongTyp   Type
	happyValue any
	happyWant  any
	wrongValue any
	zero       any
	call       func(*LiteralValue) (any, bool)
}

// allAccessors enumerates every typed accessor on LiteralValue. Read
// as a spec table: each row says "if typ matches and Value is
// happyValue, the accessor returns happyWant with ok=true; if any of
// the four (zero, false) conditions hold instead, it returns zero
// with ok=false."
func allAccessors() []accessorCase {
	timestampFixture := timestamppb.New(time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))
	return []accessorCase{
		{
			name: "AsInt32", typ: Int32Type(), wrongTyp: Int64Type(),
			happyValue: int32(7), happyWant: int32(7),
			wrongValue: "not-int32", zero: int32(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsInt32() },
		},
		{
			name: "AsInt64", typ: Int64Type(), wrongTyp: Int32Type(),
			happyValue: int64(42), happyWant: int64(42),
			wrongValue: "not-int64", zero: int64(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsInt64() },
		},
		{
			name: "AsUint32", typ: Uint32Type(), wrongTyp: Int64Type(),
			happyValue: uint32(7), happyWant: uint32(7),
			wrongValue: "not-uint32", zero: uint32(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsUint32() },
		},
		{
			name: "AsUint64", typ: Uint64Type(), wrongTyp: Int64Type(),
			happyValue: uint64(42), happyWant: uint64(42),
			wrongValue: "not-uint64", zero: uint64(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsUint64() },
		},
		{
			name: "AsBool", typ: BoolType(), wrongTyp: Int64Type(),
			happyValue: true, happyWant: true,
			wrongValue: int64(1), zero: false,
			call: func(v *LiteralValue) (any, bool) { return v.AsBool() },
		},
		{
			name: "AsFloat", typ: FloatType(), wrongTyp: Int64Type(),
			happyValue: float32(1.5), happyWant: float32(1.5),
			wrongValue: float64(1), zero: float32(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsFloat() },
		},
		{
			name: "AsDouble", typ: DoubleType(), wrongTyp: Int64Type(),
			happyValue: float64(2.5), happyWant: float64(2.5),
			wrongValue: float32(1), zero: float64(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsDouble() },
		},
		{
			name: "AsString", typ: StringType(), wrongTyp: Int64Type(),
			happyValue: "hello", happyWant: "hello",
			wrongValue: []byte("not-string"), zero: "",
			call: func(v *LiteralValue) (any, bool) { return v.AsString() },
		},
		{
			name: "AsBytes", typ: BytesType(), wrongTyp: Int64Type(),
			happyValue: []byte{0x01, 0x02}, happyWant: []byte{0x01, 0x02},
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsBytes() },
		},
		{
			name: "AsJson", typ: JsonType(), wrongTyp: Int64Type(),
			happyValue: `{"k":1}`, happyWant: `{"k":1}`,
			wrongValue: []byte("not-json"), zero: "",
			call: func(v *LiteralValue) (any, bool) { return v.AsJson() },
		},
		{
			name: "AsDateDays", typ: DateType(), wrongTyp: Int64Type(),
			happyValue: int32(20000), happyWant: int32(20000),
			wrongValue: int64(1), zero: int32(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsDateDays() },
		},
		{
			name: "AsTimeMicros", typ: TimeType(), wrongTyp: Int64Type(),
			happyValue: int64(123456789), happyWant: int64(123456789),
			wrongValue: int32(1), zero: int64(0),
			call: func(v *LiteralValue) (any, bool) { return v.AsTimeMicros() },
		},
		{
			name: "AsTimestamp", typ: TimestampType(), wrongTyp: Int64Type(),
			happyValue: timestampFixture,
			happyWant:  time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
			wrongValue: int64(1), zero: time.Time{},
			call: func(v *LiteralValue) (any, bool) { return v.AsTimestamp() },
		},
		{
			name:       "AsArray",
			typ:        &ArrayType{ElementType: Int64Type()},
			wrongTyp:   Int64Type(),
			happyValue: ArrayValue{{Type: Int64Type(), Value: int64(1)}},
			happyWant:  ArrayValue{{Type: Int64Type(), Value: int64(1)}},
			wrongValue: "not-array", zero: (ArrayValue)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsArray() },
		},
		{
			name:       "AsStruct",
			typ:        &StructType{Fields: []*StructField{{Name: "a", Type: Int64Type()}}},
			wrongTyp:   Int64Type(),
			happyValue: StructValue{{Type: Int64Type(), Value: int64(1)}},
			happyWant:  StructValue{{Type: Int64Type(), Value: int64(1)}},
			wrongValue: "not-struct", zero: (StructValue)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsStruct() },
		},
		{
			name: "AsNumeric", typ: NumericType(), wrongTyp: Int64Type(),
			happyValue: []byte{0xAA}, happyWant: []byte{0xAA},
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsNumeric() },
		},
		{
			name: "AsBigNumeric", typ: BigNumericType(), wrongTyp: Int64Type(),
			happyValue: []byte{0xBB}, happyWant: []byte{0xBB},
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsBigNumeric() },
		},
		{
			name: "AsInterval", typ: IntervalType(), wrongTyp: Int64Type(),
			happyValue: []byte{0xCC}, happyWant: []byte{0xCC},
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsInterval() },
		},
		{
			name: "AsGeography", typ: GeographyType(), wrongTyp: Int64Type(),
			happyValue: []byte{0xDD}, happyWant: []byte{0xDD},
			wrongValue: "not-bytes", zero: ([]byte)(nil),
			call: func(v *LiteralValue) (any, bool) { return v.AsGeography() },
		},
	}
}

// TestLiteralValue_TypedAccessors_HappyPath pins the positive half
// of every typed accessor's contract: when the surrounding
// LiteralValue's Type matches the accessor's kind and Value carries
// the documented Go type, the accessor returns that value with
// ok=true. Composite kinds use a one-element fixture so the
// observation is on this accessor's own contract, not on recursive
// wrapping (which TestWrapLiteralValue already covers).
func TestLiteralValue_TypedAccessors_HappyPath(t *testing.T) {
	for _, c := range allAccessors() {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			sut := &LiteralValue{Type: c.typ, Value: c.happyValue}

			// Act
			got, ok := c.call(sut)

			// Assert
			assert.True(t, ok)
			assert.Equal(t, c.happyWant, got)
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
// Sharing allAccessors() with the HappyPath test forces a newly
// added accessor to be exercised on both axes by a single new row.
func TestLiteralValue_TypedAccessors_ContractViolation(t *testing.T) {
	for _, c := range allAccessors() {
		t.Run(c.name, func(t *testing.T) {
			violations := []struct {
				name string
				in   *LiteralValue
			}{
				{"nil receiver", nil},
				{"nil Type", &LiteralValue{Type: nil, Value: c.wrongValue}},
				{"kind mismatch", &LiteralValue{Type: c.wrongTyp, Value: nil}},
				{"value type mismatch", &LiteralValue{Type: c.typ, Value: c.wrongValue}},
				{"NULL (Value=nil)", &LiteralValue{Type: c.typ, Value: nil}},
			}
			for _, v := range violations {
				t.Run(v.name, func(t *testing.T) {
					// Arrange
					sut := v.in

					// Act
					got, ok := c.call(sut)

					// Assert
					assert.False(t, ok)
					assert.Equal(t, c.zero, got)
				})
			}
		})
	}
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
