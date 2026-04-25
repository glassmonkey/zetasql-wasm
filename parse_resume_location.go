package zetasql

// ParseResumeLocation tracks the position in a multi-statement SQL string
// for incremental parsing with AnalyzeNextStatement.
type ParseResumeLocation struct {
	input        string
	bytePosition int32
}

// NewParseResumeLocation creates a new resume location for the given SQL input.
func NewParseResumeLocation(sql string) *ParseResumeLocation {
	return &ParseResumeLocation{input: sql, bytePosition: 0}
}

// Input returns the original SQL string.
func (l *ParseResumeLocation) Input() string { return l.input }

// BytePosition returns the current byte position in the input.
func (l *ParseResumeLocation) BytePosition() int32 { return l.bytePosition }

// AtEnd returns true if the entire input has been consumed.
func (l *ParseResumeLocation) AtEnd() bool {
	return int(l.bytePosition) >= len(l.input)
}
