package ast

import (
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// parseLocationRangeOf walks the proto Parent chain starting at msg until
// it reaches ASTNodeProto and returns its parse_location_range. Concrete
// parser-AST proto messages embed their supertype through a `parent` field;
// following that chain ends at ASTNodeProto, where parse_location_range
// lives. Returns nil when msg is nil, when an intermediate parent is unset
// on the wire (chain broken before reaching ASTNodeProto), or when
// parse_location_range itself is unset.
func parseLocationRangeOf(msg proto.Message) *generated.ParseLocationRangeProto {
	if msg == nil {
		return nil
	}
	refl := msg.ProtoReflect()
	for {
		if refl == nil || !refl.IsValid() {
			return nil
		}
		if refl.Descriptor().Name() == "ASTNodeProto" {
			return readLocationField(refl)
		}
		next := stepToParent(refl)
		if next == nil {
			return nil
		}
		refl = next
	}
}

func readLocationField(refl protoreflect.Message) *generated.ParseLocationRangeProto {
	field := refl.Descriptor().Fields().ByName("parse_location_range")
	if field == nil || !refl.Has(field) {
		return nil
	}
	return refl.Get(field).Message().Interface().(*generated.ParseLocationRangeProto)
}

func stepToParent(refl protoreflect.Message) protoreflect.Message {
	field := refl.Descriptor().Fields().ByName("parent")
	if field == nil || !refl.Has(field) {
		return nil
	}
	return refl.Get(field).Message()
}
