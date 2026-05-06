package zetasql

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// ParameterMode selects how query parameters are referenced in the SQL text.
type ParameterMode int32

const (
	ParameterNamed      ParameterMode = ParameterMode(generated.ParameterMode_PARAMETER_NAMED)
	ParameterPositional ParameterMode = ParameterMode(generated.ParameterMode_PARAMETER_POSITIONAL)
	ParameterNone       ParameterMode = ParameterMode(generated.ParameterMode_PARAMETER_NONE)
)

// String returns the canonical proto enum name (e.g. "PARAMETER_NAMED").
func (m ParameterMode) String() string {
	return generated.ParameterMode(m).String()
}

func (m ParameterMode) toProto() generated.ParameterMode {
	return generated.ParameterMode(m)
}

// ParseLocationRecordType controls how the analyzer attaches source-location
// information to resolved AST nodes.
type ParseLocationRecordType int32

const (
	ParseLocationRecordNone          ParseLocationRecordType = ParseLocationRecordType(generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_NONE)
	ParseLocationRecordFullNodeScope ParseLocationRecordType = ParseLocationRecordType(generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE)
	ParseLocationRecordCodeSearch    ParseLocationRecordType = ParseLocationRecordType(generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_CODE_SEARCH)
)

// String returns the canonical proto enum name (e.g. "PARSE_LOCATION_RECORD_FULL_NODE_SCOPE").
func (t ParseLocationRecordType) String() string {
	return generated.ParseLocationRecordType(t).String()
}

func (t ParseLocationRecordType) toProto() generated.ParseLocationRecordType {
	return generated.ParseLocationRecordType(t)
}

// AnalyzerOptions configures the behavior of the ZetaSQL analyzer.
type AnalyzerOptions struct {
	Language                  *LanguageOptions
	ParseLocationRecordType   *ParseLocationRecordType
	AllowUndeclaredParameters bool
	ParameterMode             ParameterMode
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
	clone := &AnalyzerOptions{
		AllowUndeclaredParameters: o.AllowUndeclaredParameters,
		ParameterMode:             o.ParameterMode,
	}
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
		v := o.ParseLocationRecordType.toProto()
		p.ParseLocationRecordType = &v
	}
	if o.AllowUndeclaredParameters {
		b := true
		p.AllowUndeclaredParameters = &b
	}
	if o.ParameterMode != ParameterNamed {
		m := o.ParameterMode.toProto()
		p.ParameterMode = &m
	}
	return p
}
