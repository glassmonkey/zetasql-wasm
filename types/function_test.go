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

func TestFunctionArgumentType_OptionsCardinality(t *testing.T) {
	repeated := generated.FunctionEnums_REPEATED
	optional := generated.FunctionEnums_OPTIONAL

	tests := []struct {
		name        string
		cardinality generated.FunctionEnums_ArgumentCardinality
		want        *generated.FunctionEnums_ArgumentCardinality
	}{
		{
			name:        "REQUIRED (zero) is omitted",
			cardinality: generated.FunctionEnums_REQUIRED,
			want:        nil,
		},
		{
			name:        "REPEATED is propagated",
			cardinality: generated.FunctionEnums_REPEATED,
			want:        &repeated,
		},
		{
			name:        "OPTIONAL is propagated",
			cardinality: generated.FunctionEnums_OPTIONAL,
			want:        &optional,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := &FunctionArgumentType{
				Kind:    generated.SignatureArgumentKind_ARG_TYPE_FIXED,
				Type:    Int64Type(),
				Options: &FunctionArgumentTypeOptions{Cardinality: tt.cardinality},
			}

			// Act
			got := sut.toProto().GetOptions().Cardinality

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFunctionArgumentType_NilOptionsOmitsProtoOptions(t *testing.T) {
	// Arrange
	sut := NewFunctionArgumentType(Int64Type())

	// Act
	got := sut.toProto().GetOptions()

	// Assert
	assert.Nil(t, got)
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
