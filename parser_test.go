package zetasql

import (
	"context"
	"errors"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

// ignoreParseLocationRange filters out only parse_location_range (byte offsets)
// from proto comparison. All other fields including parent chains are compared,
// so the test documents the full ZetaSQL proto AST structure.
var ignoreParseLocationRange = cmp.Options{
	protocmp.Transform(),
	protocmp.IgnoreFields(&generated.ASTNodeProto{}, "parse_location_range"),
}

// Parent chain builders — ZetaSQL proto encodes class hierarchy via nested Parent fields.

func astNodeParent() *generated.ASTNodeProto {
	return &generated.ASTNodeProto{} // parse_location_range is ignored
}

func stmtParent() *generated.ASTStatementProto {
	return &generated.ASTStatementProto{Parent: astNodeParent()}
}

func queryExprParent() *generated.ASTQueryExpressionProto {
	return &generated.ASTQueryExpressionProto{
		Parent:        astNodeParent(),
		Parenthesized: proto.Bool(false),
	}
}

func exprParent() *generated.ASTExpressionProto {
	return &generated.ASTExpressionProto{
		Parent:        astNodeParent(),
		Parenthesized: proto.Bool(false),
	}
}

func leafParent() *generated.ASTLeafProto {
	return &generated.ASTLeafProto{Parent: exprParent()}
}

func printableLeafParent(image string) *generated.ASTPrintableLeafProto {
	return &generated.ASTPrintableLeafProto{
		Parent: leafParent(),
		Image:  proto.String(image),
	}
}

func tableExprParent() *generated.ASTTableExpressionProto {
	return &generated.ASTTableExpressionProto{Parent: astNodeParent()}
}

func genPathExprParent() *generated.ASTGeneralizedPathExpressionProto {
	return &generated.ASTGeneralizedPathExpressionProto{Parent: exprParent()}
}

// AST node builders

func intLiteralExpr(image string) *generated.AnyASTExpressionProto {
	return &generated.AnyASTExpressionProto{
		Node: &generated.AnyASTExpressionProto_AstLeafNode{
			AstLeafNode: &generated.AnyASTLeafProto{
				Node: &generated.AnyASTLeafProto_AstPrintableLeafNode{
					AstPrintableLeafNode: &generated.AnyASTPrintableLeafProto{
						Node: &generated.AnyASTPrintableLeafProto_AstIntLiteralNode{
							AstIntLiteralNode: &generated.ASTIntLiteralProto{
								Parent: printableLeafParent(image),
							},
						},
					},
				},
			},
		},
	}
}

func stringLiteralExpr(value string, quotedImage string) *generated.AnyASTExpressionProto {
	return &generated.AnyASTExpressionProto{
		Node: &generated.AnyASTExpressionProto_AstLeafNode{
			AstLeafNode: &generated.AnyASTLeafProto{
				Node: &generated.AnyASTLeafProto_AstStringLiteralNode{
					AstStringLiteralNode: &generated.ASTStringLiteralProto{
						Parent:      leafParent(),
						StringValue: proto.String(value),
						Components: []*generated.ASTStringLiteralComponentProto{
							{
								Parent:      printableLeafParent(quotedImage),
								StringValue: proto.String(value),
							},
						},
					},
				},
			},
		},
	}
}

func starExpr() *generated.AnyASTExpressionProto {
	return &generated.AnyASTExpressionProto{
		Node: &generated.AnyASTExpressionProto_AstLeafNode{
			AstLeafNode: &generated.AnyASTLeafProto{
				Node: &generated.AnyASTLeafProto_AstPrintableLeafNode{
					AstPrintableLeafNode: &generated.AnyASTPrintableLeafProto{
						Node: &generated.AnyASTPrintableLeafProto_AstStarNode{
							AstStarNode: &generated.ASTStarProto{
								Parent: printableLeafParent("*"),
							},
						},
					},
				},
			},
		},
	}
}

func pathExpr(names ...string) *generated.AnyASTExpressionProto {
	ids := make([]*generated.ASTIdentifierProto, len(names))
	for i, name := range names {
		ids[i] = &generated.ASTIdentifierProto{
			Parent:   exprParent(),
			IdString: proto.String(name),
			IsQuoted: proto.Bool(false),
		}
	}
	return &generated.AnyASTExpressionProto{
		Node: &generated.AnyASTExpressionProto_AstGeneralizedPathExpressionNode{
			AstGeneralizedPathExpressionNode: &generated.AnyASTGeneralizedPathExpressionProto{
				Node: &generated.AnyASTGeneralizedPathExpressionProto_AstPathExpressionNode{
					AstPathExpressionNode: &generated.ASTPathExpressionProto{
						Parent: genPathExprParent(),
						Names:  ids,
					},
				},
			},
		},
	}
}

func tablePathFrom(names ...string) *generated.ASTFromClauseProto {
	ids := make([]*generated.ASTIdentifierProto, len(names))
	for i, name := range names {
		ids[i] = &generated.ASTIdentifierProto{
			Parent:   exprParent(),
			IdString: proto.String(name),
			IsQuoted: proto.Bool(false),
		}
	}
	return &generated.ASTFromClauseProto{
		Parent: astNodeParent(),
		TableExpression: &generated.AnyASTTableExpressionProto{
			Node: &generated.AnyASTTableExpressionProto_AstTablePathExpressionNode{
				AstTablePathExpressionNode: &generated.ASTTablePathExpressionProto{
					Parent: tableExprParent(),
					PathExpr: &generated.ASTPathExpressionProto{
						Parent: genPathExprParent(),
						Names:  ids,
					},
				},
			},
		},
	}
}

func TestParseStatement(t *testing.T) {
	ctx := context.Background()
	parser, err := NewParser(ctx)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	defer parser.Close(ctx)

	tests := []struct {
		name    string
		sql     string
		wantErr bool
		wantAST *generated.AnyASTStatementProto
	}{
		{
			name: "Simple SELECT literal",
			sql:  "SELECT 1",
			wantAST: &generated.AnyASTStatementProto{
				Node: &generated.AnyASTStatementProto_AstQueryStatementNode{
					AstQueryStatementNode: &generated.ASTQueryStatementProto{
						Parent: stmtParent(),
						Query: &generated.ASTQueryProto{
							Parent:       queryExprParent(),
							IsNested:     proto.Bool(false),
							IsPivotInput: proto.Bool(false),
							QueryExpr: &generated.AnyASTQueryExpressionProto{
								Node: &generated.AnyASTQueryExpressionProto_AstSelectNode{
									AstSelectNode: &generated.ASTSelectProto{
										Parent:   queryExprParent(),
										Distinct: proto.Bool(false),
										SelectList: &generated.ASTSelectListProto{
											Parent: astNodeParent(),
											Columns: []*generated.ASTSelectColumnProto{
												{
													Parent:     astNodeParent(),
													Expression: intLiteralExpr("1"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "SELECT multiple literals",
			sql:  "SELECT 1, 2, 'hello'",
			wantAST: &generated.AnyASTStatementProto{
				Node: &generated.AnyASTStatementProto_AstQueryStatementNode{
					AstQueryStatementNode: &generated.ASTQueryStatementProto{
						Parent: stmtParent(),
						Query: &generated.ASTQueryProto{
							Parent:       queryExprParent(),
							IsNested:     proto.Bool(false),
							IsPivotInput: proto.Bool(false),
							QueryExpr: &generated.AnyASTQueryExpressionProto{
								Node: &generated.AnyASTQueryExpressionProto_AstSelectNode{
									AstSelectNode: &generated.ASTSelectProto{
										Parent:   queryExprParent(),
										Distinct: proto.Bool(false),
										SelectList: &generated.ASTSelectListProto{
											Parent: astNodeParent(),
											Columns: []*generated.ASTSelectColumnProto{
												{Parent: astNodeParent(), Expression: intLiteralExpr("1")},
												{Parent: astNodeParent(), Expression: intLiteralExpr("2")},
												{Parent: astNodeParent(), Expression: stringLiteralExpr("hello", "'hello'")},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "SELECT star FROM table",
			sql:  "SELECT * FROM users",
			wantAST: &generated.AnyASTStatementProto{
				Node: &generated.AnyASTStatementProto_AstQueryStatementNode{
					AstQueryStatementNode: &generated.ASTQueryStatementProto{
						Parent: stmtParent(),
						Query: &generated.ASTQueryProto{
							Parent:       queryExprParent(),
							IsNested:     proto.Bool(false),
							IsPivotInput: proto.Bool(false),
							QueryExpr: &generated.AnyASTQueryExpressionProto{
								Node: &generated.AnyASTQueryExpressionProto_AstSelectNode{
									AstSelectNode: &generated.ASTSelectProto{
										Parent:   queryExprParent(),
										Distinct: proto.Bool(false),
										SelectList: &generated.ASTSelectListProto{
											Parent: astNodeParent(),
											Columns: []*generated.ASTSelectColumnProto{
												{Parent: astNodeParent(), Expression: starExpr()},
											},
										},
										FromClause: tablePathFrom("users"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "SELECT with WHERE",
			sql:  "SELECT * FROM users WHERE age > 20",
			wantAST: &generated.AnyASTStatementProto{
				Node: &generated.AnyASTStatementProto_AstQueryStatementNode{
					AstQueryStatementNode: &generated.ASTQueryStatementProto{
						Parent: stmtParent(),
						Query: &generated.ASTQueryProto{
							Parent:       queryExprParent(),
							IsNested:     proto.Bool(false),
							IsPivotInput: proto.Bool(false),
							QueryExpr: &generated.AnyASTQueryExpressionProto{
								Node: &generated.AnyASTQueryExpressionProto_AstSelectNode{
									AstSelectNode: &generated.ASTSelectProto{
										Parent:   queryExprParent(),
										Distinct: proto.Bool(false),
										SelectList: &generated.ASTSelectListProto{
											Parent: astNodeParent(),
											Columns: []*generated.ASTSelectColumnProto{
												{Parent: astNodeParent(), Expression: starExpr()},
											},
										},
										FromClause: tablePathFrom("users"),
										WhereClause: &generated.ASTWhereClauseProto{
											Parent: astNodeParent(),
											Expression: &generated.AnyASTExpressionProto{
												Node: &generated.AnyASTExpressionProto_AstBinaryExpressionNode{
													AstBinaryExpressionNode: &generated.ASTBinaryExpressionProto{
														Parent: exprParent(),
														Op:     generated.ASTBinaryExpressionEnums_GT.Enum(),
														IsNot:  proto.Bool(false),
														Lhs:    pathExpr("age"),
														Rhs:    intLiteralExpr("20"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "SELECT columns from table",
			sql:  "SELECT name, email FROM customers",
			wantAST: &generated.AnyASTStatementProto{
				Node: &generated.AnyASTStatementProto_AstQueryStatementNode{
					AstQueryStatementNode: &generated.ASTQueryStatementProto{
						Parent: stmtParent(),
						Query: &generated.ASTQueryProto{
							Parent:       queryExprParent(),
							IsNested:     proto.Bool(false),
							IsPivotInput: proto.Bool(false),
							QueryExpr: &generated.AnyASTQueryExpressionProto{
								Node: &generated.AnyASTQueryExpressionProto_AstSelectNode{
									AstSelectNode: &generated.ASTSelectProto{
										Parent:   queryExprParent(),
										Distinct: proto.Bool(false),
										SelectList: &generated.ASTSelectListProto{
											Parent: astNodeParent(),
											Columns: []*generated.ASTSelectColumnProto{
												{Parent: astNodeParent(), Expression: pathExpr("name")},
												{Parent: astNodeParent(), Expression: pathExpr("email")},
											},
										},
										FromClause: tablePathFrom("customers"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "Invalid SQL",
			sql:     "SELECT",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := parser.ParseStatement(ctx, tt.sql)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}
				var parseErr *ParseError
				if !errors.As(err, &parseErr) {
					t.Fatalf("Expected *ParseError, got: %T: %v", err, err)
				}
				if parseErr.Message == "" {
					t.Error("Expected non-empty error message")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseStatement returned error: %v", err)
			}
			if stmt == nil {
				t.Fatal("ParseStatement returned nil statement")
			}
			if stmt.SQL() != tt.sql {
				t.Errorf("SQL() = %q, want %q", stmt.SQL(), tt.sql)
			}
			if diff := cmp.Diff(tt.wantAST, stmt.proto(), ignoreParseLocationRange...); diff != "" {
				t.Errorf("AST mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParserClose(t *testing.T) {
	ctx := context.Background()
	parser, err := NewParser(ctx)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Parse a simple statement
	stmt, err := parser.ParseStatement(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("ParseStatement failed: %v", err)
	}
	if stmt.SQL() != "SELECT 1" {
		t.Errorf("SQL() = %q, want %q", stmt.SQL(), "SELECT 1")
	}

	// Close the parser
	if err := parser.Close(ctx); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Trying to parse after close should fail
	_, err = parser.ParseStatement(ctx, "SELECT 1")
	if err == nil {
		t.Error("Expected error when parsing after close, got nil")
	}
}
