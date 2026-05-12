package zetasql_test

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm"
	"github.com/glassmonkey/zetasql-wasm/types"
)

// TestCastValueError pins the canonical ZetaSQL runtime cast failure
// wording. Downstream evaluators (the emulator's SQLite runtime, any
// other consumer) must produce identical text so that the failure
// surface stays consistent with BigQuery across the ecosystem.
func TestCastValueError(t *testing.T) {
	cases := []struct {
		name   string
		value  any
		toType types.TypeKind
		want   string
	}{
		{
			name:   "string literal as source",
			value:  "apple",
			toType: types.Int64,
			want:   `Could not cast value "apple" to type INT64`,
		},
		{
			name:   "string with embedded quote is escaped",
			value:  `say "hi"`,
			toType: types.Int64,
			want:   `Could not cast value "say \"hi\"" to type INT64`,
		},
		{
			name:   "empty string is still quoted",
			value:  "",
			toType: types.Int64,
			want:   `Could not cast value "" to type INT64`,
		},
		{
			name:   "bytes value gets the B prefix",
			value:  []byte("apple"),
			toType: types.Int64,
			want:   `Could not cast value B"apple" to type INT64`,
		},
		{
			name:   "numeric source value renders unquoted",
			value:  float64(1.5),
			toType: types.Int64,
			want:   `Could not cast value 1.5 to type INT64`,
		},
		{
			name:   "target type kind name comes from TypeKind.String()",
			value:  "apple",
			toType: types.Double,
			want:   `Could not cast value "apple" to type DOUBLE`,
		},
		{
			name:   "numeric target",
			value:  "apple",
			toType: types.Numeric,
			want:   `Could not cast value "apple" to type NUMERIC`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := zetasql.CastValueError(tc.value, tc.toType)

			// Assert
			if got == nil {
				t.Fatal("got nil error, want non-nil")
			}
			if got.Error() != tc.want {
				t.Errorf("error mismatch\n want: %q\n  got: %q", tc.want, got.Error())
			}
		})
	}
}
