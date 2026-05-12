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

// ParseLocation is the byte range [Start, End) of the source SQL fragment
// from which a parser AST node was produced. Both ends are byte offsets
// into the original SQL string, suitable for slicing — e.g. for
// re-analysing a sub-statement of a script block.
type ParseLocation struct {
	Start int32
	End   int32
}

// parseLocator is the structural shape every auto-generated AST node
// implements (each concrete *XxxNode has a ParseLocationRange method
// that delegates to parseLocationRangeOf). Keeping the assertion
// unexported lets ParseLocationOf surface a proto-free API on the
// Node interface side while reusing the per-node implementation.
type parseLocator interface {
	ParseLocationRange() *generated.ParseLocationRangeProto
}

// ParseLocationOf returns the source byte range from which n was parsed.
// The second return is false when n has no parse_location_range — either
// because it is a synthesised node that did not originate from any source
// text, or because the wire form arrived with the field unset (the parent
// chain broke before ASTNodeProto).
//
// Sliceable example: a caller holding the original SQL string can extract
// just the sub-statement that produced n with
//
//	loc, ok := ast.ParseLocationOf(n)
//	if ok {
//	    sub := sql[loc.Start:loc.End]
//	}
//
// This is what bigquery-emulator's parseScript uses to re-analyze a
// sub-statement of a BEGIN…END block without re-parsing the wrapping
// block (which would re-create the unsupported BeginEndBlock root).
func ParseLocationOf(n Node) (ParseLocation, bool) {
	locator, ok := n.(parseLocator)
	if !ok {
		return ParseLocation{}, false
	}
	r := locator.ParseLocationRange()
	if r == nil {
		return ParseLocation{}, false
	}
	return ParseLocation{Start: r.GetStart(), End: r.GetEnd()}, true
}
