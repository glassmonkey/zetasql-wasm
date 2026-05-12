package types_test

import (
	"errors"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
)

// TestCastValueError pins the canonical ZetaSQL runtime cast failure
// wording. Downstream evaluators (the emulator's SQLite runtime, any
// other consumer) must produce identical text so that the failure
// surface stays consistent with BigQuery across the ecosystem.
func TestCastValueError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  *types.CastValueError
		want string
	}{
		{
			name: "string literal as source",
			err:  &types.CastValueError{Value: "apple", ToType: types.Int64},
			want: `Could not cast value "apple" to type INT64`,
		},
		{
			name: "string with embedded quote is escaped",
			err:  &types.CastValueError{Value: `say "hi"`, ToType: types.Int64},
			want: `Could not cast value "say \"hi\"" to type INT64`,
		},
		{
			name: "empty string is still quoted",
			err:  &types.CastValueError{Value: "", ToType: types.Int64},
			want: `Could not cast value "" to type INT64`,
		},
		{
			name: "bytes value gets the B prefix",
			err:  &types.CastValueError{Value: []byte("apple"), ToType: types.Int64},
			want: `Could not cast value B"apple" to type INT64`,
		},
		{
			name: "numeric source value renders unquoted",
			err:  &types.CastValueError{Value: float64(1.5), ToType: types.Int64},
			want: `Could not cast value 1.5 to type INT64`,
		},
		{
			name: "target type kind name comes from TypeKind.String()",
			err:  &types.CastValueError{Value: "apple", ToType: types.Double},
			want: `Could not cast value "apple" to type DOUBLE`,
		},
		{
			name: "numeric target",
			err:  &types.CastValueError{Value: "apple", ToType: types.Numeric},
			want: `Could not cast value "apple" to type NUMERIC`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := tc.err.Error()

			// Assert
			if got != tc.want {
				t.Errorf("error mismatch\n want: %q\n  got: %q", tc.want, got)
			}
		})
	}
}

// TestCastValueErrorIsRecoverable pins the typed-error contract: a
// downstream caller that wraps the failure (e.g. adding row/column
// context) can still recover the original Value and ToType via
// errors.As. This is the load-bearing reason for choosing a typed
// error over a fmt.Errorf string.
func TestCastValueError_RecoverableViaErrorsAs(t *testing.T) {
	// Arrange
	original := &types.CastValueError{Value: "apple", ToType: types.Int64}
	wrapped := errors.Join(errors.New("at column foo"), original)

	// Act
	var got *types.CastValueError
	ok := errors.As(wrapped, &got)

	// Assert
	if !ok {
		t.Fatalf("errors.As did not recover *CastValueError from %v", wrapped)
	}
	if got.Value != "apple" || got.ToType != types.Int64 {
		t.Errorf("recovered fields mismatch: got Value=%v ToType=%v", got.Value, got.ToType)
	}
}
