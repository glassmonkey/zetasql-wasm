package ast

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRewriteNamedTimezoneInString locks edge cases of the string-
// level rewrite that the front-door integration tests in zetasql/
// engine_test.go (TestEngine_Analyze cases for named-zone TIMESTAMP
// literals) do not naturally exercise: the alternate DST season for a
// single zone, datetime shapes with fractional seconds or the T
// separator, and the pass-through paths where the caller wants the
// input returned byte-for-byte. The canonical PDT/numeric-offset/
// unknown-zone cases sit at the front door instead — duplicating them
// here would only double the maintenance cost.
func TestRewriteNamedTimezoneInString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantString  string
		wantChanged bool
	}{
		// Rewrite cases — DST and datetime-shape coverage. The PDT
		// (summer) baseline lives at the front door; this case pins
		// the winter offset for the same zone so a DST regression
		// would surface.
		{
			name:        "Los_Angeles in PST (winter)",
			input:       "2015-01-01 12:34:56 America/Los_Angeles",
			wantString:  "2015-01-01 12:34:56-08:00",
			wantChanged: true,
		},
		{
			name:        "fractional seconds preserved",
			input:       "2015-09-01 12:34:56.123 America/Los_Angeles",
			wantString:  "2015-09-01 12:34:56.123-07:00",
			wantChanged: true,
		},
		{
			name:        "T separator",
			input:       "2015-09-01T12:34:56 America/Los_Angeles",
			wantString:  "2015-09-01T12:34:56-07:00",
			wantChanged: true,
		},
		// Pass-through cases — the function must return the input
		// unchanged and signal changed=false so the caller can keep
		// the original source span byte-for-byte. The "unknown zone"
		// pass-through is observed at the front door (the analyzer
		// surfaces its native diagnostic); the cases below cover
		// pass-through paths that never reach the analyzer because
		// the regex / parser short-circuits first.
		{
			name:        "no timezone suffix",
			input:       "2015-09-01 12:34:56",
			wantString:  "2015-09-01 12:34:56",
			wantChanged: false,
		},
		{
			name:        "unparseable datetime — none of the layouts match",
			input:       "not-a-date 12:34:56 America/Los_Angeles",
			wantString:  "not-a-date 12:34:56 America/Los_Angeles",
			wantChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotString, gotChanged := rewriteNamedTimezoneInString(tt.input)
			assert.Equal(t, tt.wantString, gotString)
			assert.Equal(t, tt.wantChanged, gotChanged)
		})
	}
}

// TestFormatTZOffset pins the ±HH:MM serialization. Includes the
// half-hour and 45-minute zones (India, Newfoundland, Nepal) so the
// minute path is exercised, not only the hour-aligned common case.
func TestFormatTZOffset(t *testing.T) {
	tests := []struct {
		sec  int
		want string
	}{
		{0, "+00:00"},
		{3600, "+01:00"},
		{-3600, "-01:00"},
		{-7 * 3600, "-07:00"},
		{9 * 3600, "+09:00"},
		{14 * 3600, "+14:00"},      // Kiribati / Line Islands — max east
		{-12 * 3600, "-12:00"},     // Baker Island — max west
		{5*3600 + 30*60, "+05:30"}, // India
		{-3*3600 - 30*60, "-03:30"}, // Newfoundland
		{5*3600 + 45*60, "+05:45"}, // Nepal
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d sec", tt.sec), func(t *testing.T) {
			assert.Equal(t, tt.want, formatTZOffset(tt.sec))
		})
	}
}

// TestRequoteStringLiteral pins quote-style preservation. The
// rewritten content must come back wrapped in the same quoting
// (single, double, triple-single, triple-double) as the source span
// so the resulting SQL stays lexically identical to the original on
// every byte outside the substitution.
func TestRequoteStringLiteral(t *testing.T) {
	tests := []struct {
		name      string
		rawQuoted string
		content   string
		want      string
	}{
		{
			name:      "single quote",
			rawQuoted: "'2015-09-01 12:34:56 America/Los_Angeles'",
			content:   "2015-09-01 12:34:56-07:00",
			want:      "'2015-09-01 12:34:56-07:00'",
		},
		{
			name:      "double quote",
			rawQuoted: `"2015-09-01 12:34:56 America/Los_Angeles"`,
			content:   "2015-09-01 12:34:56-07:00",
			want:      `"2015-09-01 12:34:56-07:00"`,
		},
		{
			name:      "triple single quote",
			rawQuoted: "'''2015-09-01 12:34:56 America/Los_Angeles'''",
			content:   "2015-09-01 12:34:56-07:00",
			want:      "'''2015-09-01 12:34:56-07:00'''",
		},
		{
			name:      "triple double quote",
			rawQuoted: `"""2015-09-01 12:34:56 America/Los_Angeles"""`,
			content:   "2015-09-01 12:34:56-07:00",
			want:      `"""2015-09-01 12:34:56-07:00"""`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := requoteStringLiteral(tt.rawQuoted, tt.content)
			require.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

