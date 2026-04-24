package zetasql

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// AnalyzerOptions configures the behavior of the ZetaSQL analyzer.
type AnalyzerOptions struct {
	language                *LanguageOptions
	parseLocationRecordType *generated.ParseLocationRecordType
}

// NewAnalyzerOptions creates AnalyzerOptions with default settings.
// By default, all statement kinds are supported.
func NewAnalyzerOptions() *AnalyzerOptions {
	return &AnalyzerOptions{}
}

// SetLanguageOptions sets the language options for analysis.
func (o *AnalyzerOptions) SetLanguageOptions(lang *LanguageOptions) {
	o.language = lang
}

// SetParseLocationRecordType sets how parse locations are recorded in the resolved AST.
func (o *AnalyzerOptions) SetParseLocationRecordType(t generated.ParseLocationRecordType) {
	o.parseLocationRecordType = &t
}

func (o *AnalyzerOptions) toProto() *generated.AnalyzerOptionsProto {
	p := &generated.AnalyzerOptionsProto{}
	if o.language != nil {
		p.LanguageOptions = o.language.ToProto()
	}
	if o.parseLocationRecordType != nil {
		p.ParseLocationRecordType = o.parseLocationRecordType
	}
	return p
}
