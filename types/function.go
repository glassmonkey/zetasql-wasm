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

// FunctionArgumentTypeOptions holds optional per-argument metadata such as
// cardinality (REQUIRED / REPEATED / OPTIONAL). Zero values map to the proto
// defaults and are omitted from the wire representation.
type FunctionArgumentTypeOptions struct {
	Cardinality generated.FunctionEnums_ArgumentCardinality
}

func (o *FunctionArgumentTypeOptions) toProto() *generated.FunctionArgumentTypeOptionsProto {
	p := &generated.FunctionArgumentTypeOptionsProto{}
	if o.Cardinality != generated.FunctionEnums_REQUIRED {
		c := o.Cardinality
		p.Cardinality = &c
	}
	return p
}

// FunctionArgumentType represents a single function argument type.
// Type is nil for templated arguments.
type FunctionArgumentType struct {
	Kind    generated.SignatureArgumentKind
	Type    Type
	Options *FunctionArgumentTypeOptions
}

// NewFunctionArgumentType creates a fixed-type function argument.
func NewFunctionArgumentType(typ Type) *FunctionArgumentType {
	return &FunctionArgumentType{
		Kind: generated.SignatureArgumentKind_ARG_TYPE_FIXED,
		Type: typ,
	}
}

// NewTemplatedFunctionArgumentType creates a templated function argument.
func NewTemplatedFunctionArgumentType(kind generated.SignatureArgumentKind) *FunctionArgumentType {
	return &FunctionArgumentType{Kind: kind}
}

func (a *FunctionArgumentType) toProto() *generated.FunctionArgumentTypeProto {
	kind := a.Kind
	p := &generated.FunctionArgumentTypeProto{
		Kind: &kind,
	}
	if a.Type != nil {
		p.Type = a.Type.ToProto()
	}
	if a.Options != nil {
		p.Options = a.Options.toProto()
	}
	return p
}

// FunctionSignature represents a function signature (return type + arguments).
type FunctionSignature struct {
	ReturnType *FunctionArgumentType
	Arguments  []*FunctionArgumentType
	ContextID  int64
}

// NewFunctionSignature creates a new function signature.
func NewFunctionSignature(ret *FunctionArgumentType, args []*FunctionArgumentType) *FunctionSignature {
	return &FunctionSignature{
		ReturnType: ret,
		Arguments:  args,
	}
}

func (s *FunctionSignature) toProto() *generated.FunctionSignatureProto {
	p := &generated.FunctionSignatureProto{}
	if s.ReturnType != nil {
		p.ReturnType = s.ReturnType.toProto()
	}
	for _, arg := range s.Arguments {
		p.Argument = append(p.Argument, arg.toProto())
	}
	if s.ContextID != 0 {
		ctxID := s.ContextID
		p.ContextId = &ctxID
	}
	return p
}

// Function represents a ZetaSQL function with one or more signatures.
type Function struct {
	NamePath   []string
	Group      string
	Mode       Mode
	Signatures []*FunctionSignature
}

// NewFunction creates a new function.
func NewFunction(namePath []string, group string, mode Mode, sigs []*FunctionSignature) *Function {
	return &Function{
		NamePath:   namePath,
		Group:      group,
		Mode:       mode,
		Signatures: sigs,
	}
}

// ToProto converts the function to its protobuf representation.
func (f *Function) ToProto() *generated.FunctionProto {
	mode := f.Mode
	p := &generated.FunctionProto{
		NamePath: f.NamePath,
		Mode:     &mode,
	}
	if f.Group != "" {
		group := f.Group
		p.Group = &group
	}
	for _, sig := range f.Signatures {
		p.Signature = append(p.Signature, sig.toProto())
	}
	return p
}
