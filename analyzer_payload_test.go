package zetasql

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSplitAnalyzePayload exercises the bridge framing parser. Got is the
// (parsed, response) pair that splitAnalyzePayload extracts; want is the
// pair the test built. Cases cover happy path, empty parsed section,
// truncation at each length prefix, and truncation in each body section
// so a malformed bridge response surfaces as an error rather than a panic.
func TestSplitAnalyzePayload(t *testing.T) {
	frame := func(parsed, response []byte) []byte {
		buf := make([]byte, 0, 8+len(parsed)+len(response))
		var size [4]byte
		binary.LittleEndian.PutUint32(size[:], uint32(len(parsed)))
		buf = append(buf, size[:]...)
		buf = append(buf, parsed...)
		binary.LittleEndian.PutUint32(size[:], uint32(len(response)))
		buf = append(buf, size[:]...)
		buf = append(buf, response...)
		return buf
	}

	tests := []struct {
		name         string
		payload      []byte
		wantParsed   []byte
		wantResponse []byte
		wantErr      bool
	}{
		{
			name:         "both sections present",
			payload:      frame([]byte{0x0a, 0x0b}, []byte{0x01, 0x02, 0x03}),
			wantParsed:   []byte{0x0a, 0x0b},
			wantResponse: []byte{0x01, 0x02, 0x03},
		},
		{
			name:         "parsed empty, response present",
			payload:      frame(nil, []byte{0xaa}),
			wantParsed:   []byte{},
			wantResponse: []byte{0xaa},
		},
		{
			name:    "payload too short for parsed length prefix",
			payload: []byte{0x01, 0x02},
			wantErr: true,
		},
		{
			name: "parsed length larger than remaining buffer",
			payload: func() []byte {
				b := make([]byte, 4)
				binary.LittleEndian.PutUint32(b, 99)
				return b
			}(),
			wantErr: true,
		},
		{
			name: "missing response length prefix after parsed body",
			payload: func() []byte {
				b := make([]byte, 4)
				binary.LittleEndian.PutUint32(b, 0)
				return b
			}(),
			wantErr: true,
		},
		{
			name: "response length larger than remaining buffer",
			payload: func() []byte {
				buf := make([]byte, 0, 8)
				var size [4]byte
				binary.LittleEndian.PutUint32(size[:], 0)
				buf = append(buf, size[:]...)
				binary.LittleEndian.PutUint32(size[:], 99)
				buf = append(buf, size[:]...)
				return buf
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, response, err := splitAnalyzePayload(tt.payload)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantParsed, parsed)
			assert.Equal(t, tt.wantResponse, response)
		})
	}
}
