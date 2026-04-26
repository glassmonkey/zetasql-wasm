package zetasql

import (
	"sort"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLanguageOptions_SetSupportedStatementKinds verifies that the kinds
// passed to SetSupportedStatementKinds are reflected in ToProto output.
func TestLanguageOptions_SetSupportedStatementKinds(t *testing.T) {
	tests := []struct {
		name  string
		kinds []generated.ResolvedNodeKind
		want  []generated.ResolvedNodeKind
	}{
		{
			name: "query and insert",
			kinds: []generated.ResolvedNodeKind{
				generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
				generated.ResolvedNodeKind_RESOLVED_INSERT_STMT,
			},
			want: []generated.ResolvedNodeKind{
				generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
				generated.ResolvedNodeKind_RESOLVED_INSERT_STMT,
			},
		},
		{
			name:  "query only",
			kinds: []generated.ResolvedNodeKind{generated.ResolvedNodeKind_RESOLVED_QUERY_STMT},
			want:  []generated.ResolvedNodeKind{generated.ResolvedNodeKind_RESOLVED_QUERY_STMT},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := NewLanguageOptions()
			sut.SetSupportedStatementKinds(tt.kinds)

			// Act
			got := sut.toProto().GetSupportedStatementKinds()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestLanguageOptions_SetSupportsAllStatementKinds verifies that the
// "all kinds supported" signal is an empty slice in ToProto output.
func TestLanguageOptions_SetSupportsAllStatementKinds(t *testing.T) {
	// Arrange
	sut := NewLanguageOptions()
	sut.SetSupportedStatementKinds([]generated.ResolvedNodeKind{
		generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
	})
	sut.SetSupportsAllStatementKinds()

	// Act
	got := sut.toProto().GetSupportedStatementKinds()

	// Assert
	var want []generated.ResolvedNodeKind
	assert.Equal(t, want, got)
}

// TestLanguageOptions_EnableMaximumLanguageFeatures verifies that
// representative released features are enabled and test-only features are not.
// Triangulated across feature flags rather than count thresholds.
func TestLanguageOptions_EnableMaximumLanguageFeatures(t *testing.T) {
	tests := []struct {
		name    string
		feature generated.LanguageFeature
		want    bool
	}{
		{
			name:    "released feature is enabled",
			feature: generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS,
			want:    true,
		},
		{
			name:    "another released feature is enabled",
			feature: generated.LanguageFeature_FEATURE_TABLESAMPLE,
			want:    true,
		},
		{
			name:    "test-only feature is not enabled",
			feature: generated.LanguageFeature_FEATURE_TEST_IDEALLY_ENABLED_BUT_IN_DEVELOPMENT,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := NewLanguageOptions()
			sut.EnableMaximumLanguageFeatures()

			// Act
			got := sut.Features[tt.feature]

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestLanguageOptions_EnableMaximumLanguageFeaturesForDevelopment verifies
// that development mode enables features the released mode skips (e.g.,
// FEATURE_TEST_IDEALLY_ENABLED_BUT_IN_DEVELOPMENT, which is excluded from
// released because of its FEATURE_TEST_ prefix).
func TestLanguageOptions_EnableMaximumLanguageFeaturesForDevelopment(t *testing.T) {
	tests := []struct {
		name    string
		feature generated.LanguageFeature
		want    bool
	}{
		{
			name:    "released feature is enabled",
			feature: generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS,
			want:    true,
		},
		{
			name:    "another released feature is enabled",
			feature: generated.LanguageFeature_FEATURE_TABLESAMPLE,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := NewLanguageOptions()
			sut.EnableMaximumLanguageFeaturesForDevelopment()

			// Act
			got := sut.Features[tt.feature]

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestLanguageOptions_EnableReservableKeyword verifies the reserved-keyword
// list in ToProto reflects EnableReservableKeyword calls.
func TestLanguageOptions_EnableReservableKeyword(t *testing.T) {
	tests := []struct {
		name    string
		enable  []string
		disable []string
		want    []string
	}{
		{
			name:   "enable two keywords",
			enable: []string{"QUALIFY", "PIVOT"},
			want:   []string{"PIVOT", "QUALIFY"},
		},
		{
			name:    "enable two then disable one",
			enable:  []string{"QUALIFY", "PIVOT"},
			disable: []string{"PIVOT"},
			want:    []string{"QUALIFY"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := NewLanguageOptions()
			for _, kw := range tt.enable {
				sut.EnableReservableKeyword(kw, true)
			}
			for _, kw := range tt.disable {
				sut.EnableReservableKeyword(kw, false)
			}

			// Act
			got := sut.toProto().GetReservedKeywords()
			sort.Strings(got)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestLanguageOptions_AnalyzerIntegration verifies that LanguageOptions
// set on AnalyzerOptions are honored by the analyzer. The got is the
// resolved literal value extracted from the analysis output. Triangulated
// across two literal values.
func TestLanguageOptions_AnalyzerIntegration(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want int64
	}{
		{name: "literal 1", sql: "SELECT 1", want: 1},
		{name: "literal 77", sql: "SELECT 77", want: 77},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			a := newTestAnalyzer(t)
			lang := NewLanguageOptions()
			lang.EnableLanguageFeature(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS)
			lang.SetSupportedStatementKinds([]generated.ResolvedNodeKind{
				generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
			})
			opts := &AnalyzerOptions{Language: lang}

			// Act
			out, err := a.AnalyzeStatement(ctx, tt.sql, nil, opts)
			require.NoError(t, err)
			stmt := out.Statement.(*resolved_ast.QueryStmtNode)
			literal := findNode[*resolved_ast.LiteralNode](t, stmt)
			got := literal.Value().GetValue().GetInt64Value()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestLanguageOptions_RejectsUnsupportedStatementKind verifies that the
// analyzer rejects SQL whose statement kind is not in the supported list.
// wantErr is a type witness compared via assert.IsType.
func TestLanguageOptions_RejectsUnsupportedStatementKind(t *testing.T) {
	tests := []struct {
		name         string
		allowedKinds []generated.ResolvedNodeKind
		sql          string
		wantErr      error
	}{
		{
			name:         "INSERT-only rejects SELECT",
			allowedKinds: []generated.ResolvedNodeKind{generated.ResolvedNodeKind_RESOLVED_INSERT_STMT},
			sql:          "SELECT 1",
			wantErr:      &AnalyzeError{},
		},
		{
			name:         "DELETE-only rejects SELECT",
			allowedKinds: []generated.ResolvedNodeKind{generated.ResolvedNodeKind_RESOLVED_DELETE_STMT},
			sql:          "SELECT 1",
			wantErr:      &AnalyzeError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			a := newTestAnalyzer(t)
			lang := NewLanguageOptions()
			lang.SetSupportedStatementKinds(tt.allowedKinds)
			opts := &AnalyzerOptions{Language: lang}

			// Act
			_, got := a.AnalyzeStatement(ctx, tt.sql, nil, opts)

			// Assert
			assert.IsType(t, tt.wantErr, got)
		})
	}
}
