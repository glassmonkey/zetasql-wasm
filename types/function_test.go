package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
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

	if got, want := len(proto.GetNamePath()), 1; got != want {
		t.Fatalf("len(NamePath) = %d, want %d", got, want)
	}
	if got, want := proto.GetNamePath()[0], "my_func"; got != want {
		t.Errorf("NamePath[0] = %q, want %q", got, want)
	}
	if got, want := proto.GetGroup(), "custom"; got != want {
		t.Errorf("Group = %q, want %q", got, want)
	}
	if got, want := proto.GetMode(), generated.FunctionEnums_SCALAR; got != want {
		t.Errorf("Mode = %v, want %v", got, want)
	}

	sigs := proto.GetSignature()
	if got, want := len(sigs), 1; got != want {
		t.Fatalf("len(Signature) = %d, want %d", got, want)
	}

	ret := sigs[0].GetReturnType()
	if got, want := ret.GetKind(), generated.SignatureArgumentKind_ARG_TYPE_FIXED; got != want {
		t.Errorf("ReturnType.Kind = %v, want %v", got, want)
	}

	args := sigs[0].GetArgument()
	if got, want := len(args), 1; got != want {
		t.Fatalf("len(Argument) = %d, want %d", got, want)
	}
	if got, want := args[0].GetKind(), generated.SignatureArgumentKind_ARG_TYPE_FIXED; got != want {
		t.Errorf("Argument[0].Kind = %v, want %v", got, want)
	}
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

	if got, want := sig.GetReturnType().GetKind(), generated.SignatureArgumentKind_ARG_TYPE_ANY_1; got != want {
		t.Errorf("ReturnType.Kind = %v, want %v", got, want)
	}
	if got, want := sig.GetArgument()[0].GetKind(), generated.SignatureArgumentKind_ARG_TYPE_ANY_1; got != want {
		t.Errorf("Argument[0].Kind = %v, want %v", got, want)
	}
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

	proto := fn.ToProto()
	if got, want := len(proto.GetSignature()), 2; got != want {
		t.Errorf("len(Signature) = %d, want %d", got, want)
	}
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

	if got, want := fn.ToProto().GetMode(), generated.FunctionEnums_AGGREGATE; got != want {
		t.Errorf("Mode = %v, want %v", got, want)
	}
}
