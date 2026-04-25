package types

import (
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// Mode represents the function mode (scalar, aggregate, analytic).
type Mode = generated.FunctionEnums_Mode

// Function mode constants.
const (
	ScalarMode    Mode = generated.FunctionEnums_SCALAR
	AggregateMode Mode = generated.FunctionEnums_AGGREGATE
	AnalyticMode  Mode = generated.FunctionEnums_ANALYTIC
)

// SignatureArgumentKind represents the kind of a function argument.
type SignatureArgumentKind = generated.SignatureArgumentKind

// FunctionArgumentType represents a single function argument type.
type FunctionArgumentType struct {
	kind generated.SignatureArgumentKind
	typ  Type // nil for templated types
}

// NewFunctionArgumentType creates a fixed-type function argument.
func NewFunctionArgumentType(typ Type) *FunctionArgumentType {
	return &FunctionArgumentType{
		kind: generated.SignatureArgumentKind_ARG_TYPE_FIXED,
		typ:  typ,
	}
}

// NewTemplatedFunctionArgumentType creates a templated function argument.
func NewTemplatedFunctionArgumentType(kind generated.SignatureArgumentKind) *FunctionArgumentType {
	return &FunctionArgumentType{kind: kind}
}

func (a *FunctionArgumentType) toProto() *generated.FunctionArgumentTypeProto {
	p := &generated.FunctionArgumentTypeProto{
		Kind: &a.kind,
	}
	if a.typ != nil {
		p.Type = a.typ.ToProto()
	}
	return p
}

// FunctionSignature represents a function signature (return type + arguments).
type FunctionSignature struct {
	returnType *FunctionArgumentType
	arguments  []*FunctionArgumentType
	contextID  int64
}

// NewFunctionSignature creates a new function signature.
func NewFunctionSignature(ret *FunctionArgumentType, args []*FunctionArgumentType) *FunctionSignature {
	return &FunctionSignature{
		returnType: ret,
		arguments:  args,
	}
}

// SetContextID sets the context ID for this signature.
func (s *FunctionSignature) SetContextID(id int64) {
	s.contextID = id
}

func (s *FunctionSignature) toProto() *generated.FunctionSignatureProto {
	p := &generated.FunctionSignatureProto{}
	if s.returnType != nil {
		p.ReturnType = s.returnType.toProto()
	}
	for _, arg := range s.arguments {
		p.Argument = append(p.Argument, arg.toProto())
	}
	if s.contextID != 0 {
		p.ContextId = &s.contextID
	}
	return p
}

// Function represents a ZetaSQL function with one or more signatures.
type Function struct {
	namePath   []string
	group      string
	mode       Mode
	signatures []*FunctionSignature
}

// NewFunction creates a new function.
func NewFunction(namePath []string, group string, mode Mode, sigs []*FunctionSignature) *Function {
	return &Function{
		namePath:   namePath,
		group:      group,
		mode:       mode,
		signatures: sigs,
	}
}

// ToProto converts the function to its protobuf representation.
func (f *Function) ToProto() *generated.FunctionProto {
	p := &generated.FunctionProto{
		NamePath: f.namePath,
		Mode:     &f.mode,
	}
	if f.group != "" {
		p.Group = &f.group
	}
	for _, sig := range f.signatures {
		p.Signature = append(p.Signature, sig.toProto())
	}
	return p
}
