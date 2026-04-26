package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				NewTemplatedFunctionArgumentType(generated.SignatureArgumentKind_ARG_TYPE_ANY_1),
				[]*FunctionArgumentType{
					NewTemplatedFunctionArgumentType(generated.SignatureArgumentKind_ARG_TYPE_ANY_1),
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
			options: &FunctionArgumentTypeOptions{Cardinality: generated.FunctionEnums_REQUIRED},
			want:    &generated.FunctionArgumentTypeOptionsProto{},
		},
		{
			name:    "REPEATED is propagated",
			options: &FunctionArgumentTypeOptions{Cardinality: generated.FunctionEnums_REPEATED},
			want:    &generated.FunctionArgumentTypeOptionsProto{Cardinality: &repeated},
		},
		{
			name:    "OPTIONAL is propagated",
			options: &FunctionArgumentTypeOptions{Cardinality: generated.FunctionEnums_OPTIONAL},
			want:    &generated.FunctionArgumentTypeOptionsProto{Cardinality: &optional},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := &FunctionArgumentType{
				Kind:    generated.SignatureArgumentKind_ARG_TYPE_FIXED,
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
