package types

import (
	"fmt"
	"strconv"
)

// CastValueError is the canonical ZetaSQL runtime cast failure error.
// Two callers construct it:
//
//   - Engine.Analyze itself, when AnalyzerOptions.RejectInvalidLiteralCasts
//     is set and the resolved AST contains a ResolvedCast whose source
//     is a STRING literal that cannot be parsed as the target type.
//     This opt-in surfaces BigQuery's analyze-time-reject behavior on
//     top of upstream ZetaSQL's defer-to-runtime resolution.
//   - Downstream evaluators that execute a resolved AST without the
//     analyze-time gate (the bigquery-emulator's SQLite runtime is
//     the immediate one) construct it themselves at the runtime
//     cast point, so the failure wording stays aligned with BigQuery
//     instead of leaking the host language's conversion detail.
//
// Mirrors the layout of AnalyzeError: public fields, Error() builds
// the surface text. Callers can therefore use errors.As to recover
// the original Value / ToType when wrapping the failure with row or
// column context.
type CastValueError struct {
	Value  any
	ToType TypeKind
}

func (e *CastValueError) Error() string {
	return fmt.Sprintf("Could not cast value %s to type %s", formatCastValue(e.Value), e.ToType)
}

// formatCastValue renders a Go value the way ZetaSQL's analyzer
// renders argument_value->DebugString(): strings double-quoted,
// bytes prefixed with B"...", everything else through Go's default
// formatting. Kept private so the wording rules live in one place
// and CastValueError stays the only public surface.
func formatCastValue(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case []byte:
		return "B" + strconv.Quote(string(x))
	default:
		return fmt.Sprint(x)
	}
}
