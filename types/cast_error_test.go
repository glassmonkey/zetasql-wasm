package types_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/types"
)

// TestCastValueError_Error pins the canonical ZetaSQL runtime cast
// failure wording. Downstream evaluators (the emulator's SQLite
// runtime, any other consumer) must produce identical text so that
// the failure surface stays consistent with BigQuery across the
// ecosystem.
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
			name: "DOUBLE target renders in message",
			err:  &types.CastValueError{Value: "apple", ToType: types.Double},
			want: `Could not cast value "apple" to type DOUBLE`,
		},
		{
			name: "NUMERIC target renders in message",
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

// TestCastValueError_RecoverableViaErrorsAs pins the typed-error
// contract: a downstream caller that wraps the failure (adding row/
// column context, joining with sibling errors, etc.) can still
// recover the original Value and ToType via errors.As. This is the
// load-bearing reason for choosing a typed error over fmt.Errorf,
// so the table covers each wrap pattern an emulator-style caller
// realistically uses.
func TestCastValueError_RecoverableViaErrorsAs(t *testing.T) {
	original := &types.CastValueError{Value: "apple", ToType: types.Int64}

	cases := []struct {
		name string
		err  error
	}{
		{
			name: "direct typed error without wrapping",
			err:  original,
		},
		{
			name: "wrapped via fmt.Errorf %w",
			err:  fmt.Errorf("at row 42: %w", original),
		},
		{
			name: "double-wrapped via fmt.Errorf %w",
			err:  fmt.Errorf("query failed: %w", fmt.Errorf("at row 42: %w", original)),
		},
		{
			name: "joined via errors.Join with a sibling error",
			err:  errors.Join(errors.New("at column foo"), original),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			var got *types.CastValueError
			ok := errors.As(tc.err, &got)

			// Assert
			if !ok {
				t.Fatalf("errors.As did not recover *CastValueError from %v", tc.err)
			}
			if *got != *original {
				t.Errorf("recovered fields mismatch\n want: %+v\n  got: %+v", *original, *got)
			}
		})
	}
}
