package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
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
	int64Kind := generated.TypeKind_TYPE_INT64
	stringKind := generated.TypeKind_TYPE_STRING
	arrayKind := generated.TypeKind_TYPE_ARRAY
	structKind := generated.TypeKind_TYPE_STRUCT
	datetimeKind := generated.TypeKind_TYPE_DATETIME

	int64Type := &generated.TypeProto{TypeKind: &int64Kind}
	stringType := &generated.TypeProto{TypeKind: &stringKind}

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
				Type:  int64Type,
				Value: &generated.ValueProto{},
			},
			want: &LiteralValue{Type: Int64Type(), Value: nil},
		},
		{
			name: "INT64 = 42",
			in: &generated.ValueWithTypeProto{
				Type:  int64Type,
				Value: int64Lit(42),
			},
			want: &LiteralValue{Type: Int64Type(), Value: int64(42)},
		},
		{
			name: "STRING = \"hello\"",
			in: &generated.ValueWithTypeProto{
				Type:  stringType,
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
						ElementType: int64Type,
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
							{FieldName: ptr("a"), FieldType: int64Type},
							{FieldName: ptr("b"), FieldType: stringType},
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
				Type: &generated.TypeProto{TypeKind: &datetimeKind},
				Value: &generated.ValueProto{
					Value: &generated.ValueProto_DatetimeValue{
						DatetimeValue: &generated.ValueProto_Datetime{},
					},
				},
			},
			want: &LiteralValue{Type: TypeFromKind(Datetime), Value: nil},
		},
		{
			name: "STRUCT with mismatched field count yields Value=nil (defensive)",
			in: &generated.ValueWithTypeProto{
				Type: &generated.TypeProto{
					TypeKind: &structKind,
					StructType: &generated.StructTypeProto{
						Field: []*generated.StructFieldProto{
							{FieldName: ptr("a"), FieldType: int64Type},
							{FieldName: ptr("b"), FieldType: stringType},
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
