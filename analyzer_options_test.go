package zetasql

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestAnalyzerOptions_toProto(t *testing.T) {
	fullScope := ParseLocationRecordFullNodeScope
	codeSearch := ParseLocationRecordCodeSearch
	allowUndecl := true
	fullScopeProto := generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE
	codeSearchProto := generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_CODE_SEARCH
	positionalProto := generated.ParameterMode_PARAMETER_POSITIONAL
	noneProto := generated.ParameterMode_PARAMETER_NONE

	tests := []struct {
		name string
		opts *AnalyzerOptions
		want *generated.AnalyzerOptionsProto
	}{
		{
			name: "default options yield empty proto",
			opts: NewAnalyzerOptions(),
			want: &generated.AnalyzerOptionsProto{},
		},
		{
			name: "ParseLocationRecordType FULL_NODE_SCOPE is propagated",
			opts: &AnalyzerOptions{ParseLocationRecordType: &fullScope},
			want: &generated.AnalyzerOptionsProto{ParseLocationRecordType: &fullScopeProto},
		},
		{
			name: "ParseLocationRecordType CODE_SEARCH is propagated",
			opts: &AnalyzerOptions{ParseLocationRecordType: &codeSearch},
			want: &generated.AnalyzerOptionsProto{ParseLocationRecordType: &codeSearchProto},
		},
		{
			name: "AllowUndeclaredParameters true is propagated",
			opts: &AnalyzerOptions{AllowUndeclaredParameters: true},
			want: &generated.AnalyzerOptionsProto{AllowUndeclaredParameters: &allowUndecl},
		},
		{
			name: "ParameterMode POSITIONAL is propagated",
			opts: &AnalyzerOptions{ParameterMode: ParameterPositional},
			want: &generated.AnalyzerOptionsProto{ParameterMode: &positionalProto},
		},
		{
			name: "ParameterMode NONE is propagated",
			opts: &AnalyzerOptions{ParameterMode: ParameterNone},
			want: &generated.AnalyzerOptionsProto{ParameterMode: &noneProto},
		},
		{
			name: "ParameterMode NAMED (zero) is omitted from proto",
			opts: &AnalyzerOptions{ParameterMode: ParameterNamed},
			want: &generated.AnalyzerOptionsProto{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.opts

			// Act
			got := sut.toProto()

			// Assert
			assert.Empty(t, cmp.Diff(tt.want, got, protocmp.Transform()))
		})
	}
}

// TestAnalyzerOptions_Clone verifies that Clone returns a deep copy whose
// fields equal the original by value. Pointer-independence is checked in a
// separate test so each behaviour has a single observable assertion.
func TestAnalyzerOptions_Clone(t *testing.T) {
	fullScope := ParseLocationRecordFullNodeScope

	tests := []struct {
		name string
		opts *AnalyzerOptions
		want *AnalyzerOptions
	}{
		{
			name: "empty options clone equals empty options",
			opts: &AnalyzerOptions{},
			want: &AnalyzerOptions{},
		},
		{
			name: "populated options clone equals original by value",
			opts: &AnalyzerOptions{
				Language: &LanguageOptions{
					Features: map[LanguageFeature]bool{
						FeatureAnalyticFunctions: true,
					},
					StatementKinds: []generated.ResolvedNodeKind{generated.ResolvedNodeKind_RESOLVED_QUERY_STMT},
					AllStatements:  false,
					Keywords:       map[string]bool{"QUALIFY": true},
				},
				ParseLocationRecordType: &fullScope,
			},
			want: &AnalyzerOptions{
				Language: &LanguageOptions{
					Features: map[LanguageFeature]bool{
						FeatureAnalyticFunctions: true,
					},
					StatementKinds: []generated.ResolvedNodeKind{generated.ResolvedNodeKind_RESOLVED_QUERY_STMT},
					AllStatements:  false,
					Keywords:       map[string]bool{"QUALIFY": true},
				},
				ParseLocationRecordType: &fullScope,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.opts

			// Act
			got := sut.Clone()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAnalyzerOptions_Clone_doesNotShareLanguagePointer verifies that the
// Clone method produces an AnalyzerOptions whose Language field is a distinct
// pointer from the original — mutating the clone's Language must not leak
// into the original.
func TestAnalyzerOptions_Clone_doesNotShareLanguagePointer(t *testing.T) {
	// Arrange
	sut := &AnalyzerOptions{
		Language: &LanguageOptions{
			Features: map[LanguageFeature]bool{
				FeatureAnalyticFunctions: true,
			},
			Keywords: map[string]bool{"QUALIFY": true},
		},
	}

	// Act
	clone := sut.Clone()
	clone.Language.Features[FeatureTablesample] = true
	clone.Language.Keywords["OFFSET"] = true
	got := sut.Language

	// Assert
	want := &LanguageOptions{
		Features: map[LanguageFeature]bool{
			FeatureAnalyticFunctions: true,
		},
		Keywords: map[string]bool{"QUALIFY": true},
	}
	assert.Equal(t, want, got)
}
