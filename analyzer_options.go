package zetasql

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// AnalyzerOptions configures the behavior of the ZetaSQL analyzer.
// This is a minimal implementation for Phase 1; it will be extended in Phase 3.
type AnalyzerOptions struct {
	language *generated.LanguageOptionsProto
}

// NewAnalyzerOptions creates AnalyzerOptions with default settings.
// By default, all statement kinds are supported.
func NewAnalyzerOptions() *AnalyzerOptions {
	return &AnalyzerOptions{}
}

// SetLanguageOptions sets the language options for analysis.
func (o *AnalyzerOptions) SetLanguageOptions(lang *generated.LanguageOptionsProto) {
	o.language = lang
}

func (o *AnalyzerOptions) toProto() *generated.AnalyzerOptionsProto {
	p := &generated.AnalyzerOptionsProto{}
	if o.language != nil {
		p.LanguageOptions = o.language
	}
	return p
}
