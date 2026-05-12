package zetasql

import (
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

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
//
// QueryParameters and PositionalQueryParameters tell the analyzer the
// type of each `@name` / positional `?` parameter that may appear in
// the SQL text. The analyzer needs only the type — values are bound at
// execution time by the caller's runtime. Entries with a nil Type are
// skipped on toProto so a partially-populated table does not poison
// the wire form; for positional parameters that means a nil entry will
// cause its successors to shift down, so callers building the slice
// should treat nil as a programming error rather than a placeholder.
type AnalyzerOptions struct {
	Language                  *LanguageOptions
	ParseLocationRecordType   *ParseLocationRecordType
	AllowUndeclaredParameters bool
	ParameterMode             ParameterMode
	QueryParameters           map[string]types.Type
	PositionalQueryParameters []types.Type

	// RejectInvalidLiteralCasts opts into BigQuery-compatible strict
	// cast checking at analyze time. When true, Engine.Analyze walks
	// the resolved AST after analysis and returns a
	// *types.CastValueError instead of the deferred resolved tree
	// when it finds a ResolvedCast whose source is a STRING literal
	// that cannot be parsed as the target type.
	//
	// Default false matches upstream ZetaSQL 2025.x, which
	// deliberately leaves such casts unfolded for runtime evaluation
	// (see types.CastValueError doc for the upstream rationale).
	// Callers that target BigQuery semantics -- where the equivalent
	// CAST fails before execution -- set this to true.
	//
	// This flag is Go-side only; it does not affect the C++ analyzer
	// and is therefore not propagated through toProto.
	RejectInvalidLiteralCasts bool
}

// NewAnalyzerOptions creates AnalyzerOptions with default settings.
func NewAnalyzerOptions() *AnalyzerOptions {
	return &AnalyzerOptions{}
}

// Clone returns a deep copy of the AnalyzerOptions. The returned value
// shares no pointer state with the receiver: Language is deep-copied,
// ParseLocationRecordType is duplicated, and QueryParameters /
// PositionalQueryParameters get fresh container backing arrays.
// types.Type values are themselves immutable, so the elements alias.
func (o *AnalyzerOptions) Clone() *AnalyzerOptions {
	clone := &AnalyzerOptions{
		AllowUndeclaredParameters: o.AllowUndeclaredParameters,
		ParameterMode:             o.ParameterMode,
		RejectInvalidLiteralCasts: o.RejectInvalidLiteralCasts,
	}
	if o.Language != nil {
		clone.Language = o.Language.clone()
	}
	if o.ParseLocationRecordType != nil {
		v := *o.ParseLocationRecordType
		clone.ParseLocationRecordType = &v
	}
	if o.QueryParameters != nil {
		clone.QueryParameters = make(map[string]types.Type, len(o.QueryParameters))
		for k, v := range o.QueryParameters {
			clone.QueryParameters[k] = v
		}
	}
	if o.PositionalQueryParameters != nil {
		clone.PositionalQueryParameters = make([]types.Type, len(o.PositionalQueryParameters))
		copy(clone.PositionalQueryParameters, o.PositionalQueryParameters)
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
	for name, t := range o.QueryParameters {
		if t == nil {
			continue
		}
		n := name
		p.QueryParameters = append(p.QueryParameters, &generated.AnalyzerOptionsProto_QueryParameterProto{
			Name: &n,
			Type: t.ToProto(),
		})
	}
	for _, t := range o.PositionalQueryParameters {
		if t == nil {
			continue
		}
		p.PositionalQueryParameters = append(p.PositionalQueryParameters, t.ToProto())
	}
	return p
}
