package types

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// EnumType represents a ZetaSQL ENUM type. Name is the fully-qualified
// proto enum name (e.g. "zetasql.functions.DateTimestampPart"). It is
// public so callers can construct an EnumType with a struct literal;
// an EnumType with an empty Name still behaves correctly (NameOf
// returns ("", false)), so there is no constructor-level invariant
// to enforce.
//
// EnumTypeProto on the wire only carries the proto name and an index
// into the surrounding TypeProto's FileDescriptorSet, not the
// descriptor itself. EnumType resolves names by consulting
// protoregistry.GlobalTypes — every enum compiled into this Go binary
// (which includes the ZetaSQL builtin enums like DateTimestampPart and
// RoundingMode) registers there at init time, so NameOf works for them
// without the caller wiring up a descriptor pool. Enums whose
// descriptor is not linked in resolve to ("", false).
type EnumType struct {
	Name string
}

func (t *EnumType) Kind() TypeKind        { return Enum }
func (t *EnumType) IsArray() bool         { return false }
func (t *EnumType) IsStruct() bool        { return false }
func (t *EnumType) IsEnum() bool          { return true }
func (t *EnumType) AsArray() *ArrayType   { return nil }
func (t *EnumType) AsStruct() *StructType { return nil }
func (t *EnumType) AsEnum() *EnumType     { return t }

func (t *EnumType) ToProto() *generated.TypeProto {
	k := Enum.toProto()
	name := t.Name
	return &generated.TypeProto{
		TypeKind: &k,
		EnumType: &generated.EnumTypeProto{EnumName: &name},
	}
}

// NameOf returns the enum value's declared name for the given proto
// enum number, or ("", false) if the enum descriptor is not registered
// in protoregistry.GlobalTypes or the number is undefined for this
// enum. A nil receiver is treated as an unregistered enum.
func (t *EnumType) NameOf(number int32) (string, bool) {
	if t == nil || t.Name == "" {
		return "", false
	}
	et, err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(t.Name))
	if err != nil {
		return "", false
	}
	v := et.Descriptor().Values().ByNumber(protoreflect.EnumNumber(number))
	if v == nil {
		return "", false
	}
	return string(v.Name()), true
}
