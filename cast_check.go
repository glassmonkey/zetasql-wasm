package zetasql

import (
	"strconv"
	"strings"

	resolved_ast "github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
)

// rejectInvalidLiteralCasts walks the resolved AST in out and
// returns a *types.CastValueError for the first ResolvedCast whose
// source is a STRING literal that cannot be parsed as the target
// type. Returns nil when no such cast is present.
//
// Reached only when AnalyzerOptions.RejectInvalidLiteralCasts is
// true, matching BigQuery's analyze-time-reject behavior on top of
// upstream ZetaSQL's defer-to-runtime resolution. SAFE_CAST
// (ReturnNullOnError) is skipped because its contract is to return
// NULL on failure rather than error. Currently handles INT64
// targets only; other numeric targets will be added as call sites
// require them.
func rejectInvalidLiteralCasts(out *AnalyzeOutput) error {
	return resolved_ast.Walk(out.Resolved, func(n resolved_ast.Node) error {
		cast, ok := n.(*resolved_ast.CastNode)
		if !ok || cast.ReturnNullOnError() {
			return nil
		}
		lit, ok := cast.Expr().(*resolved_ast.LiteralNode)
		if !ok {
			return nil
		}
		src := types.WrapLiteralValue(lit.Value())
		if src == nil {
			return nil
		}
		s, ok := src.AsString()
		if !ok {
			return nil
		}
		target := types.WrapType(cast.Type())
		if target == nil || target.Kind() != types.Int64 {
			return nil
		}
		if _, err := castStringToInt64(s); err == nil {
			return nil
		}
		return &types.CastValueError{Value: s, ToType: types.Int64}
	})
}

// castStringToInt64 mirrors BigQuery's CAST(string AS INT64)
// behavior: empty string folds to 0, a "0x"-containing image is
// reparsed in base 0 so hex literals like "0x87a" succeed, and
// everything else goes through base 10. Kept private because the
// only caller is rejectInvalidLiteralCasts; this is not a runtime
// evaluator surface, just the contract the strict-cast gate uses
// to decide which literals are unfoldable.
func castStringToInt64(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	base := 10
	if strings.Contains(strings.ToLower(s), "0x") {
		base = 0
	}
	return strconv.ParseInt(s, base, 64)
}
