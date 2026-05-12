package ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// TestParseLocationRangeOf locks the navigation contract: walk the proto
// Parent chain up to ASTNodeProto and surface its parse_location_range,
// returning nil when the chain breaks. Cases triangulate the depth-1
// terminal (ASTNodeProto itself), the 2-hop chain (Identifier →
// Expression → Node), the 3-hop chain (PathExpression → GeneralizedPath
// → Expression → Node), an unset parent breaking mid-chain, an unset
// parse_location_range at the terminal, and a nil receiver.
func TestParseLocationRangeOf(t *testing.T) {
	loc := &generated.ParseLocationRangeProto{
		Start: ptr(int32(10)),
		End:   ptr(int32(20)),
	}

	tests := []struct {
		name string
		msg  proto.Message
		want *generated.ParseLocationRangeProto
	}{
		{
			name: "ASTNodeProto root carries the location",
			msg:  &generated.ASTNodeProto{ParseLocationRange: loc},
			want: loc,
		},
		{
			name: "Identifier walks 2 hops to ASTNodeProto",
			msg: &generated.ASTIdentifierProto{
				Parent: &generated.ASTExpressionProto{
					Parent: &generated.ASTNodeProto{ParseLocationRange: loc},
				},
			},
			want: loc,
		},
		{
			name: "PathExpression walks 3 hops to ASTNodeProto",
			msg: &generated.ASTPathExpressionProto{
				Parent: &generated.ASTGeneralizedPathExpressionProto{
					Parent: &generated.ASTExpressionProto{
						Parent: &generated.ASTNodeProto{ParseLocationRange: loc},
					},
				},
			},
			want: loc,
		},
		{
			name: "broken chain (unset intermediate parent) returns nil",
			msg:  &generated.ASTIdentifierProto{Parent: nil},
			want: nil,
		},
		{
			name: "ASTNodeProto without parse_location_range returns nil",
			msg: &generated.ASTIdentifierProto{
				Parent: &generated.ASTExpressionProto{
					Parent: &generated.ASTNodeProto{},
				},
			},
			want: nil,
		},
		{
			name: "nil receiver returns nil",
			msg:  nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLocationRangeOf(tt.msg)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseLocationRange_GeneratedDelegate is a small sanity check that
// the per-node generated wrapper method (a one-liner that calls
// parseLocationRangeOf) is wired correctly. The full navigation behavior
// is locked by TestParseLocationRangeOf above; this case only ensures the
// codegen output didn't lose the call.
func TestParseLocationRange_GeneratedDelegate(t *testing.T) {
	loc := &generated.ParseLocationRangeProto{Start: ptr(int32(3)), End: ptr(int32(9))}
	sut := newIdentifierNode(&generated.ASTIdentifierProto{
		Parent: &generated.ASTExpressionProto{
			Parent: &generated.ASTNodeProto{ParseLocationRange: loc},
		},
	})

	got := sut.ParseLocationRange()

	assert.Equal(t, loc, got)
}

// TestParseLocationOf locks the public ParseLocationOf contract: handed
// any Node, extract the (Start, End) byte range without leaking the proto
// type to callers. Cases triangulate the happy path with two different
// ranges to force the impl to actually read the per-node values, plus
// edge cases for an unset range and a Node that does not carry parse
// location (the structural-interface assertion miss).
func TestParseLocationOf(t *testing.T) {
	tests := []struct {
		name string
		node Node
		want ParseLocation
		ok   bool
	}{
		{
			name: "identifier carries range",
			node: newIdentifierNode(&generated.ASTIdentifierProto{
				Parent: &generated.ASTExpressionProto{
					Parent: &generated.ASTNodeProto{
						ParseLocationRange: &generated.ParseLocationRangeProto{
							Start: ptr(int32(3)),
							End:   ptr(int32(9)),
						},
					},
				},
			}),
			want: ParseLocation{Start: 3, End: 9},
			ok:   true,
		},
		{
			// Triangulation axis: different byte range so an impl that
			// hard-coded (3, 9) to pass the first case still has to read
			// the per-node values.
			name: "identifier carries a different range",
			node: newIdentifierNode(&generated.ASTIdentifierProto{
				Parent: &generated.ASTExpressionProto{
					Parent: &generated.ASTNodeProto{
						ParseLocationRange: &generated.ParseLocationRangeProto{
							Start: ptr(int32(42)),
							End:   ptr(int32(57)),
						},
					},
				},
			}),
			want: ParseLocation{Start: 42, End: 57},
			ok:   true,
		},
		{
			name: "identifier with no range returns ok=false",
			node: newIdentifierNode(&generated.ASTIdentifierProto{
				Parent: &generated.ASTExpressionProto{
					Parent: &generated.ASTNodeProto{},
				},
			}),
			want: ParseLocation{},
			ok:   false,
		},
		{
			name: "broken parent chain returns ok=false",
			node: newIdentifierNode(&generated.ASTIdentifierProto{Parent: nil}),
			want: ParseLocation{},
			ok:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.node

			// Act
			got, ok := ParseLocationOf(sut)

			// Assert
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
