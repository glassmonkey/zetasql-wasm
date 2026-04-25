package zetasql

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestAnalyzerOptions_toProto(t *testing.T) {
	fullScope := generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE
	codeSearch := generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_CODE_SEARCH

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
			want: &generated.AnalyzerOptionsProto{ParseLocationRecordType: &fullScope},
		},
		{
			name: "ParseLocationRecordType CODE_SEARCH is propagated",
			opts: &AnalyzerOptions{ParseLocationRecordType: &codeSearch},
			want: &generated.AnalyzerOptionsProto{ParseLocationRecordType: &codeSearch},
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

func TestNewAnalyzerOptions(t *testing.T) {
	// Arrange
	sut := NewAnalyzerOptions

	// Act
	got := sut()

	// Assert
	want := &AnalyzerOptions{}
	assert.Equal(t, want, got)
}

// TestAnalyzerOptions_Clone verifies that Clone returns a deep copy whose
// fields equal the original by value. Pointer-independence is checked in a
// separate test so each behaviour has a single observable assertion.
func TestAnalyzerOptions_Clone(t *testing.T) {
	fullScope := generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE

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
					Features: map[generated.LanguageFeature]bool{
						generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS: true,
					},
					StatementKinds: []generated.ResolvedNodeKind{generated.ResolvedNodeKind_RESOLVED_QUERY_STMT},
					AllStatements:  false,
					Keywords:       map[string]bool{"QUALIFY": true},
				},
				ParseLocationRecordType: &fullScope,
			},
			want: &AnalyzerOptions{
				Language: &LanguageOptions{
					Features: map[generated.LanguageFeature]bool{
						generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS: true,
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
			Features: map[generated.LanguageFeature]bool{
				generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS: true,
			},
			Keywords: map[string]bool{"QUALIFY": true},
		},
	}

	// Act
	clone := sut.Clone()
	clone.Language.Features[generated.LanguageFeature_FEATURE_TABLESAMPLE] = true
	clone.Language.Keywords["OFFSET"] = true
	got := sut.Language

	// Assert
	want := &LanguageOptions{
		Features: map[generated.LanguageFeature]bool{
			generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS: true,
		},
		Keywords: map[string]bool{"QUALIFY": true},
	}
	assert.Equal(t, want, got)
}
