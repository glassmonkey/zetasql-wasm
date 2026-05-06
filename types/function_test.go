package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestFunction_ToProto(t *testing.T) {
	fn := NewFunction(
		[]string{"my_func"},
		"custom",
		ScalarMode,
		[]*FunctionSignature{
			NewFunctionSignature(
				NewFunctionArgumentType(Int64Type()),
				[]*FunctionArgumentType{
					NewFunctionArgumentType(StringType()),
				},
			),
		},
	)

	proto := fn.ToProto()

	require.Len(t, proto.GetNamePath(), 1)
	assert.Equal(t, "my_func", proto.GetNamePath()[0])
	assert.Equal(t, "custom", proto.GetGroup())
	assert.Equal(t, generated.FunctionEnums_SCALAR, proto.GetMode())

	sigs := proto.GetSignature()
	require.Len(t, sigs, 1)

	ret := sigs[0].GetReturnType()
	assert.Equal(t, generated.SignatureArgumentKind_ARG_TYPE_FIXED, ret.GetKind())

	args := sigs[0].GetArgument()
	require.Len(t, args, 1)
	assert.Equal(t, generated.SignatureArgumentKind_ARG_TYPE_FIXED, args[0].GetKind())
}

func TestFunction_TemplatedArgument(t *testing.T) {
	fn := NewFunction(
		[]string{"identity"},
		"custom",
		ScalarMode,
		[]*FunctionSignature{
			NewFunctionSignature(
				NewTemplatedFunctionArgumentType(ArgTypeAny1),
				[]*FunctionArgumentType{
					NewTemplatedFunctionArgumentType(ArgTypeAny1),
				},
			),
		},
	)

	proto := fn.ToProto()
	sig := proto.GetSignature()[0]

	assert.Equal(t, generated.SignatureArgumentKind_ARG_TYPE_ANY_1, sig.GetReturnType().GetKind())
	assert.Equal(t, generated.SignatureArgumentKind_ARG_TYPE_ANY_1, sig.GetArgument()[0].GetKind())
}

func TestFunction_MultipleSignatures(t *testing.T) {
	fn := NewFunction(
		[]string{"overloaded"},
		"custom",
		ScalarMode,
		[]*FunctionSignature{
			NewFunctionSignature(
				NewFunctionArgumentType(Int64Type()),
				[]*FunctionArgumentType{NewFunctionArgumentType(Int64Type())},
			),
			NewFunctionSignature(
				NewFunctionArgumentType(StringType()),
				[]*FunctionArgumentType{NewFunctionArgumentType(StringType())},
			),
		},
	)

	assert.Len(t, fn.ToProto().GetSignature(), 2)
}

// TestFunctionArgumentType_toProto_Options verifies how the Options
// sub-proto is serialized: nil Options yields no Options on the wire,
// an Options with REQUIRED (zero) cardinality yields an empty Options
// proto, and non-zero cardinalities propagate. Want is the full Options
// proto so a future Options field shows up in the diff.
func TestFunctionArgumentType_toProto_Options(t *testing.T) {
	repeated := generated.FunctionEnums_REPEATED
	optional := generated.FunctionEnums_OPTIONAL

	tests := []struct {
		name    string
		options *FunctionArgumentTypeOptions
		want    *generated.FunctionArgumentTypeOptionsProto
	}{
		{
			name:    "nil Options yields no Options proto",
			options: nil,
			want:    nil,
		},
		{
			name:    "REQUIRED (zero) yields empty Options proto",
			options: &FunctionArgumentTypeOptions{Cardinality: RequiredCardinality},
			want:    &generated.FunctionArgumentTypeOptionsProto{},
		},
		{
			name:    "REPEATED is propagated",
			options: &FunctionArgumentTypeOptions{Cardinality: RepeatedCardinality},
			want:    &generated.FunctionArgumentTypeOptionsProto{Cardinality: &repeated},
		},
		{
			name:    "OPTIONAL is propagated",
			options: &FunctionArgumentTypeOptions{Cardinality: OptionalCardinality},
			want:    &generated.FunctionArgumentTypeOptionsProto{Cardinality: &optional},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := &FunctionArgumentType{
				Kind:    ArgTypeFixed,
				Type:    Int64Type(),
				Options: tt.options,
			}

			// Act
			got := sut.toProto().GetOptions()

			// Assert
			assert.Empty(t, cmp.Diff(tt.want, got, protocmp.Transform()))
		})
	}
}

// TestWrapFunctionArgumentType pins the read-side wrap contract:
// nil-on-nil, Kind round-trips, and Options.Cardinality propagates only
// when the proto Options field is present. The Type field is asserted
// nil even when a proto Type is supplied, locking in the documented
// "WrapType not implemented yet" gap so regressions are loud.
func TestWrapFunctionArgumentType(t *testing.T) {
	fixedKind := generated.SignatureArgumentKind_ARG_TYPE_FIXED
	repeated := generated.FunctionEnums_REPEATED
	int64Kind := generated.TypeKind_TYPE_INT64
	int64Type := &generated.TypeProto{TypeKind: &int64Kind}

	tests := []struct {
		name string
		in   *generated.FunctionArgumentTypeProto
		want *FunctionArgumentType
	}{
		{
			name: "nil proto returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "kind only, no options",
			in:   &generated.FunctionArgumentTypeProto{Kind: &fixedKind},
			want: &FunctionArgumentType{Kind: ArgTypeFixed},
		},
		{
			name: "options.Cardinality propagates",
			in: &generated.FunctionArgumentTypeProto{
				Kind:    &fixedKind,
				Options: &generated.FunctionArgumentTypeOptionsProto{Cardinality: &repeated},
			},
			want: &FunctionArgumentType{
				Kind:    ArgTypeFixed,
				Options: &FunctionArgumentTypeOptions{Cardinality: RepeatedCardinality},
			},
		},
		{
			name: "proto Type is dropped (documented gap, no WrapType yet)",
			in: &generated.FunctionArgumentTypeProto{
				Kind: &fixedKind,
				Type: int64Type,
			},
			want: &FunctionArgumentType{Kind: ArgTypeFixed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got := WrapFunctionArgumentType(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestWrapFunctionSignature verifies that arguments are wrapped
// element-wise, ContextID propagates, and FunctionSignatureOptions is
// dropped on read (documented gap — input-side struct doesn't model it).
func TestWrapFunctionSignature(t *testing.T) {
	fixedKind := generated.SignatureArgumentKind_ARG_TYPE_FIXED

	tests := []struct {
		name string
		in   *generated.FunctionSignatureProto
		want *FunctionSignature
	}{
		{
			name: "nil proto returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty proto yields zero-valued signature with empty Arguments slice",
			in:   &generated.FunctionSignatureProto{},
			want: &FunctionSignature{Arguments: []*FunctionArgumentType{}},
		},
		{
			name: "return type, args, and ContextID round-trip",
			in: &generated.FunctionSignatureProto{
				ReturnType: &generated.FunctionArgumentTypeProto{Kind: &fixedKind},
				Argument: []*generated.FunctionArgumentTypeProto{
					{Kind: &fixedKind},
					{Kind: &fixedKind},
				},
				ContextId: proto.Int64(42),
			},
			want: &FunctionSignature{
				ReturnType: &FunctionArgumentType{Kind: ArgTypeFixed},
				Arguments: []*FunctionArgumentType{
					{Kind: ArgTypeFixed},
					{Kind: ArgTypeFixed},
				},
				ContextID: 42,
			},
		},
		{
			name: "Options is dropped (documented gap)",
			in: &generated.FunctionSignatureProto{
				ReturnType: &generated.FunctionArgumentTypeProto{Kind: &fixedKind},
				Options:    &generated.FunctionSignatureOptionsProto{},
			},
			want: &FunctionSignature{
				ReturnType: &FunctionArgumentType{Kind: ArgTypeFixed},
				Arguments:  []*FunctionArgumentType{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got := WrapFunctionSignature(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFunction_AggregateMode(t *testing.T) {
	fn := NewFunction(
		[]string{"my_sum"},
		"custom",
		AggregateMode,
		[]*FunctionSignature{
			NewFunctionSignature(
				NewFunctionArgumentType(Int64Type()),
				[]*FunctionArgumentType{NewFunctionArgumentType(Int64Type())},
			),
		},
	)

	assert.Equal(t, generated.FunctionEnums_AGGREGATE, fn.ToProto().GetMode())
}
