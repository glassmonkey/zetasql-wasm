package zetasql

// ParseResumeLocation tracks the position in a multi-statement SQL string
// for incremental parsing with AnalyzeNextStatement.
type ParseResumeLocation struct {
	Input        string
	BytePosition int32
}

// NewParseResumeLocation creates a new resume location for the given SQL input.
func NewParseResumeLocation(sql string) *ParseResumeLocation {
	return &ParseResumeLocation{Input: sql}
}

// Reset sets BytePosition back to 0 so the same Input can be re-parsed
// from the beginning.
func (l *ParseResumeLocation) Reset() {
	l.BytePosition = 0
}
