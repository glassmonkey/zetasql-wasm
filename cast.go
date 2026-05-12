package zetasql

import (
	"fmt"
	"strconv"

	"github.com/glassmonkey/zetasql-wasm/types"
)

// CastValueError returns the canonical ZetaSQL runtime cast failure
// error so downstream evaluators surface a wording aligned with
// BigQuery rather than leaking the host language's conversion error
// detail (e.g. Go's strconv.ParseInt "invalid syntax").
//
// Use at any runtime cast point. The wording template matches the
// shape ZetaSQL's analyze-time path emits via MakeSqlError for the
// same failure ("Could not cast ..."); only the literal/value
// distinction differs because the analyzer abandons literal-fold on
// kInvalidArgument / kOutOfRange in zetasql 2025.x and defers value-
// level cast resolution to runtime
// (zetasql/analyzer/resolver_expr.cc, ResolveExplicitCast).
func CastValueError(value any, toType types.TypeKind) error {
	return fmt.Errorf("Could not cast value %s to type %s", formatCastValue(value), toType)
}

// formatCastValue renders a Go value the way ZetaSQL's MakeSqlError
// renders argument_value->DebugString(): strings are double-quoted,
// bytes carry the B"..." prefix, everything else falls back on Go's
// default formatting. Kept private so the wording rules live in one
// place and CastValueError stays the only public surface.
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
