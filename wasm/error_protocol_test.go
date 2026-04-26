package wasm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseResultMessage covers the WASM bridge's error-signalling protocol:
// the helper either extracts the user-visible message after the "Error: "
// prefix or returns "" when the buffer is a proto payload. Triangulated
// across distinct error messages and across non-error buffers that should
// not be interpreted as errors.
func TestParseResultMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "error with single-line message",
			data: []byte("Error: invalid SQL syntax"),
			want: "invalid SQL syntax",
		},
		{
			name: "error with multi-word message",
			data: []byte("Error: column not found"),
			want: "column not found",
		},
		{
			name: "proto payload starting with random bytes",
			data: []byte{0x0a, 0x05, 'h', 'e', 'l', 'l', 'o'},
			want: "",
		},
		{
			name: "buffer too short to match prefix",
			data: []byte("Err"),
			want: "",
		},
		{
			name: "prefix-shaped but missing the trailing space",
			data: []byte("Error:noSpace"),
			want: "",
		},
		{
			name: "prefix exactly equals the buffer (no message)",
			data: []byte("Error: "),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Arrange
			sut := ParseResultMessage

			// Act
			got := sut(tt.data)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
