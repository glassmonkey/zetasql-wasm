package wasm

// ParseResultMessage decodes the WASM bridge's error-signalling convention.
//
// The C++ bridge returns a buffer of bytes for both success and failure: on
// success it is a serialized proto, on failure it is the literal ASCII text
// "Error: <message>". This helper inspects the buffer and returns the
// user-visible message if it matches the error shape, or "" if the buffer
// should be decoded as a proto. The protocol guarantees a non-empty message
// after the prefix, so "" is a reliable "no error" signal.
func ParseResultMessage(data []byte) string {
	const prefix = "Error: "
	if len(data) <= len(prefix) || string(data[:len(prefix)]) != prefix {
		return ""
	}
	return string(data[len(prefix):])
}
