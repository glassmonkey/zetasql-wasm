package types

import (
	"fmt"
	"strconv"
)

// CastValueError is the canonical ZetaSQL cast failure error.
// Two callers construct it:
//
//   - Engine.Analyze itself, when AnalyzerOptions.RejectInvalidLiteralCasts
//     is set and the resolved AST contains a ResolvedCast whose source
//     is a STRING literal that cannot be parsed as the target type.
//     This opt-in surfaces BigQuery's analyze-time-reject behavior on
//     top of upstream ZetaSQL's defer-to-runtime resolution; the
//     analyzer populates Line / Col from the literal's parse location
//     so the surface error carries the BigQuery-style [at L:C] suffix.
//   - Downstream evaluators that execute a resolved AST without the
//     analyze-time gate (the bigquery-emulator's SQLite runtime is
//     the immediate one) construct it themselves at the runtime
//     cast point, where parse-location information is no longer in
//     scope; Line and Col are left zero and Error() falls back to the
//     short value-form wording.
//
// Mirrors the layout of AnalyzeError: public fields, Error() builds
// the surface text. Callers can therefore use errors.As to recover
// the original Value / ToType / Line / Col when wrapping the failure
// with row or column context.
type CastValueError struct {
	Value  any
	ToType TypeKind
	// Line is the 1-indexed source line that contains the literal.
	// Zero means no parse location was supplied (typical for runtime
	// construction); in that case Error() omits the [at L:C] suffix
	// and the BigQuery-style INVALID_ARGUMENT prefix.
	Line int
	// Col is the 1-indexed source column that contains the literal.
	// See Line for the meaning of zero.
	Col int
}

func (e *CastValueError) Error() string {
	if e.Line > 0 && e.Col > 0 {
		return fmt.Sprintf(`INVALID_ARGUMENT: Could not cast literal %s to type %s [at %d:%d]`,
			formatCastValue(e.Value), e.ToType, e.Line, e.Col)
	}
	return fmt.Sprintf(`Could not cast value %s to type %s`,
		formatCastValue(e.Value), e.ToType)
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
