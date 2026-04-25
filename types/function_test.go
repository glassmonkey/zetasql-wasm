package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunction_ToProto(t *testing.T) {
	fn := NewFunction(
		[]string{"my_func"},
		"custom",
		ScalarMode,
		[]*FunctionSignature{
			NewFunctionSignature(
				NewFunctionArgumentType(Int64Type(), nil),
				[]*FunctionArgumentType{
					NewFunctionArgumentType(StringType(), nil),
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
				NewTemplatedFunctionArgumentType(generated.SignatureArgumentKind_ARG_TYPE_ANY_1, nil),
				[]*FunctionArgumentType{
					NewTemplatedFunctionArgumentType(generated.SignatureArgumentKind_ARG_TYPE_ANY_1, nil),
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
				NewFunctionArgumentType(Int64Type(), nil),
				[]*FunctionArgumentType{NewFunctionArgumentType(Int64Type(), nil)},
			),
			NewFunctionSignature(
				NewFunctionArgumentType(StringType(), nil),
				[]*FunctionArgumentType{NewFunctionArgumentType(StringType(), nil)},
			),
		},
	)

	assert.Len(t, fn.ToProto().GetSignature(), 2)
}

func TestFunction_AggregateMode(t *testing.T) {
	fn := NewFunction(
		[]string{"my_sum"},
		"custom",
		AggregateMode,
		[]*FunctionSignature{
			NewFunctionSignature(
				NewFunctionArgumentType(Int64Type(), nil),
				[]*FunctionArgumentType{NewFunctionArgumentType(Int64Type(), nil)},
			),
		},
	)

	assert.Equal(t, generated.FunctionEnums_AGGREGATE, fn.ToProto().GetMode())
}
