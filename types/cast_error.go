package types

import (
	"fmt"
	"strconv"
)

// CastValueError is the canonical ZetaSQL runtime cast failure error.
// ZetaSQL's analyzer defers value-level cast failures to runtime
// evaluation, so downstream evaluators own the failure surface;
// constructing this error keeps the wording aligned with BigQuery
// instead of leaking the host language's conversion detail.
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
