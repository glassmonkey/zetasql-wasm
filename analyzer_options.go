package zetasql

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// AnalyzerOptions configures the behavior of the ZetaSQL analyzer.
type AnalyzerOptions struct {
	Language                *LanguageOptions
	ParseLocationRecordType *generated.ParseLocationRecordType
}

// NewAnalyzerOptions creates AnalyzerOptions with default settings.
func NewAnalyzerOptions() *AnalyzerOptions {
	return &AnalyzerOptions{}
}

// Clone returns a deep copy of the AnalyzerOptions. The returned value
// shares no pointer state with the receiver: Language is deep-copied, and
// ParseLocationRecordType is duplicated so that mutations of either side
// do not leak across.
func (o *AnalyzerOptions) Clone() *AnalyzerOptions {
	clone := &AnalyzerOptions{}
	if o.Language != nil {
		clone.Language = o.Language.clone()
	}
	if o.ParseLocationRecordType != nil {
		v := *o.ParseLocationRecordType
		clone.ParseLocationRecordType = &v
	}
	return clone
}

func (o *AnalyzerOptions) toProto() *generated.AnalyzerOptionsProto {
	p := &generated.AnalyzerOptionsProto{}
	if o.Language != nil {
		p.LanguageOptions = o.Language.toProto()
	}
	if o.ParseLocationRecordType != nil {
		p.ParseLocationRecordType = o.ParseLocationRecordType
	}
	return p
}
