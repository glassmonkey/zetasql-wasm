package types

import (
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// Mode represents the function mode (scalar, aggregate, analytic).
type Mode int32

// Function mode constants.
const (
	ScalarMode    Mode = Mode(generated.FunctionEnums_SCALAR)
	AggregateMode Mode = Mode(generated.FunctionEnums_AGGREGATE)
	AnalyticMode  Mode = Mode(generated.FunctionEnums_ANALYTIC)
)

// String returns the canonical proto enum name (e.g. "SCALAR").
func (m Mode) String() string {
	return generated.FunctionEnums_Mode(m).String()
}

func (m Mode) toProto() generated.FunctionEnums_Mode {
	return generated.FunctionEnums_Mode(m)
}

// SignatureArgumentKind represents the kind of a function argument
// (fixed-type vs templated, with several templated variants).
type SignatureArgumentKind int32

// SignatureArgumentKind constants. ARG_TYPE_FIXED is the value used for
// concrete-type arguments; the ARG_*_ANY_N variants describe templated
// arguments whose concrete type is unified across multiple positions.
const (
	ArgTypeFixed     SignatureArgumentKind = SignatureArgumentKind(generated.SignatureArgumentKind_ARG_TYPE_FIXED)
	ArgTypeAny1      SignatureArgumentKind = SignatureArgumentKind(generated.SignatureArgumentKind_ARG_TYPE_ANY_1)
	ArgTypeAny2      SignatureArgumentKind = SignatureArgumentKind(generated.SignatureArgumentKind_ARG_TYPE_ANY_2)
	ArgArrayTypeAny1 SignatureArgumentKind = SignatureArgumentKind(generated.SignatureArgumentKind_ARG_ARRAY_TYPE_ANY_1)
	ArgArrayTypeAny2 SignatureArgumentKind = SignatureArgumentKind(generated.SignatureArgumentKind_ARG_ARRAY_TYPE_ANY_2)
)

// String returns the canonical proto enum name (e.g. "ARG_TYPE_FIXED").
func (k SignatureArgumentKind) String() string {
	return generated.SignatureArgumentKind(k).String()
}

func (k SignatureArgumentKind) toProto() generated.SignatureArgumentKind {
	return generated.SignatureArgumentKind(k)
}

// Cardinality describes whether a function argument is required, optional,
// or repeated (zero-or-more).
type Cardinality int32

// Cardinality constants.
const (
	RequiredCardinality Cardinality = Cardinality(generated.FunctionEnums_REQUIRED)
	OptionalCardinality Cardinality = Cardinality(generated.FunctionEnums_OPTIONAL)
	RepeatedCardinality Cardinality = Cardinality(generated.FunctionEnums_REPEATED)
)

// String returns the canonical proto enum name (e.g. "REQUIRED").
func (c Cardinality) String() string {
	return generated.FunctionEnums_ArgumentCardinality(c).String()
}

func (c Cardinality) toProto() generated.FunctionEnums_ArgumentCardinality {
	return generated.FunctionEnums_ArgumentCardinality(c)
}

// FunctionArgumentTypeOptions holds optional per-argument metadata such as
// cardinality (REQUIRED / REPEATED / OPTIONAL). Zero values map to the proto
// defaults and are omitted from the wire representation.
type FunctionArgumentTypeOptions struct {
	Cardinality Cardinality
}

// NewFunctionArgumentTypeOptions creates a FunctionArgumentTypeOptions with
// the given cardinality. Equivalent to a struct literal — kept for
// readability when only cardinality is set.
func NewFunctionArgumentTypeOptions(c Cardinality) *FunctionArgumentTypeOptions {
	return &FunctionArgumentTypeOptions{Cardinality: c}
}

func (o *FunctionArgumentTypeOptions) toProto() *generated.FunctionArgumentTypeOptionsProto {
	p := &generated.FunctionArgumentTypeOptionsProto{}
	if o.Cardinality != RequiredCardinality {
		c := o.Cardinality.toProto()
		p.Cardinality = &c
	}
	return p
}

// FunctionArgumentType represents a single function argument type.
// Type is nil for templated arguments.
type FunctionArgumentType struct {
	Kind    SignatureArgumentKind
	Type    Type
	Options *FunctionArgumentTypeOptions
}

// NewFunctionArgumentType creates a fixed-type function argument.
func NewFunctionArgumentType(typ Type) *FunctionArgumentType {
	return &FunctionArgumentType{
		Kind: ArgTypeFixed,
		Type: typ,
	}
}

// NewTemplatedFunctionArgumentType creates a templated function argument.
func NewTemplatedFunctionArgumentType(kind SignatureArgumentKind) *FunctionArgumentType {
	return &FunctionArgumentType{Kind: kind}
}

func (a *FunctionArgumentType) toProto() *generated.FunctionArgumentTypeProto {
	kind := a.Kind.toProto()
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
	mode := f.Mode.toProto()
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
