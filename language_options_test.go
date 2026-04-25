package zetasql

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
)

// TestLanguageOptions_EnableFeature verifies that individually enabled features
// appear in the proto and that non-enabled features do not.
func TestLanguageOptions_EnableFeature(t *testing.T) {
	opts := NewLanguageOptions()
	opts.EnableLanguageFeature(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS)
	opts.EnableLanguageFeature(generated.LanguageFeature_FEATURE_TABLESAMPLE)

	type payload struct {
		AnalyticEnabled   bool
		TablesampleEnabled bool
		NanosEnabled      bool
		ProtoFeatureCount int
	}
	got := payload{
		AnalyticEnabled:   opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS),
		TablesampleEnabled: opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_TABLESAMPLE),
		NanosEnabled:      opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_TIMESTAMP_NANOS),
		ProtoFeatureCount: len(opts.ToProto().GetEnabledLanguageFeatures()),
	}
	want := payload{
		AnalyticEnabled:   true,
		TablesampleEnabled: true,
		NanosEnabled:      false,
		ProtoFeatureCount: 2,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// TestLanguageOptions_DisableAll verifies that DisableAllLanguageFeatures
// clears all previously enabled features.
func TestLanguageOptions_DisableAll(t *testing.T) {
	opts := NewLanguageOptions()
	opts.EnableLanguageFeature(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS)
	opts.EnableLanguageFeature(generated.LanguageFeature_FEATURE_TABLESAMPLE)
	opts.DisableAllLanguageFeatures()

	type payload struct {
		AnalyticEnabled    bool
		TablesampleEnabled bool
		ProtoFeatureCount  int
	}
	got := payload{
		AnalyticEnabled:    opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS),
		TablesampleEnabled: opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_TABLESAMPLE),
		ProtoFeatureCount:  len(opts.ToProto().GetEnabledLanguageFeatures()),
	}
	want := payload{
		AnalyticEnabled:    false,
		TablesampleEnabled: false,
		ProtoFeatureCount:  0,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// TestLanguageOptions_SupportedStatementKinds verifies that statement kinds
// set via SetSupportedStatementKinds appear in the proto output in order.
func TestLanguageOptions_SupportedStatementKinds(t *testing.T) {
	tests := []struct {
		name      string
		kinds     []generated.ResolvedNodeKind
		wantKinds []generated.ResolvedNodeKind
	}{
		{
			name: "query and insert",
			kinds: []generated.ResolvedNodeKind{
				generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
				generated.ResolvedNodeKind_RESOLVED_INSERT_STMT,
			},
			wantKinds: []generated.ResolvedNodeKind{
				generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
				generated.ResolvedNodeKind_RESOLVED_INSERT_STMT,
			},
		},
		{
			name: "query only",
			kinds: []generated.ResolvedNodeKind{
				generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
			},
			wantKinds: []generated.ResolvedNodeKind{
				generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewLanguageOptions()
			opts.SetSupportedStatementKinds(tt.kinds)
			got := opts.ToProto().GetSupportedStatementKinds()
			if diff := cmp.Diff(tt.wantKinds, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// TestLanguageOptions_SupportsAllStatementKinds verifies that after
// SetSupportsAllStatementKinds, the proto has an empty statement kinds list
// (which signals "all supported").
func TestLanguageOptions_SupportsAllStatementKinds(t *testing.T) {
	opts := NewLanguageOptions()
	opts.SetSupportedStatementKinds([]generated.ResolvedNodeKind{
		generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
	})
	opts.SetSupportsAllStatementKinds()

	got := len(opts.ToProto().GetSupportedStatementKinds())
	if got != 0 {
		t.Errorf("len(SupportedStatementKinds) = %d, want 0 (empty = all)", got)
	}
}

// TestLanguageOptions_EnableMaximumLanguageFeatures verifies that
// EnableMaximumLanguageFeatures enables many released features but excludes
// test/in-development features.
func TestLanguageOptions_EnableMaximumLanguageFeatures(t *testing.T) {
	opts := NewLanguageOptions()
	opts.EnableMaximumLanguageFeatures()

	type payload struct {
		FeatureCountAbove50    bool
		TestFeatureExcluded    bool
		AnalyticFunctionEnabled bool
	}
	got := payload{
		FeatureCountAbove50:    len(opts.ToProto().GetEnabledLanguageFeatures()) > 50,
		TestFeatureExcluded:    !opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_TEST_IDEALLY_ENABLED_BUT_IN_DEVELOPMENT),
		AnalyticFunctionEnabled: opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS),
	}
	want := payload{
		FeatureCountAbove50:    true,
		TestFeatureExcluded:    true,
		AnalyticFunctionEnabled: true,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// TestLanguageOptions_EnableMaximumLanguageFeaturesForDevelopment verifies that
// development mode enables at least as many features as released mode.
func TestLanguageOptions_EnableMaximumLanguageFeaturesForDevelopment(t *testing.T) {
	dev := NewLanguageOptions()
	dev.EnableMaximumLanguageFeaturesForDevelopment()

	rel := NewLanguageOptions()
	rel.EnableMaximumLanguageFeatures()

	devCount := len(dev.ToProto().GetEnabledLanguageFeatures())
	relCount := len(rel.ToProto().GetEnabledLanguageFeatures())
	if devCount < relCount {
		t.Errorf("development features (%d) < released features (%d)", devCount, relCount)
	}
}

// TestLanguageOptions_EnableReservableKeyword verifies that enabling and
// disabling reservable keywords is reflected in the proto output.
func TestLanguageOptions_EnableReservableKeyword(t *testing.T) {
	tests := []struct {
		name    string
		enable  []string
		disable []string
		want    []string
	}{
		{
			name:    "enable two keywords",
			enable:  []string{"QUALIFY", "PIVOT"},
			disable: nil,
			want:    []string{"PIVOT", "QUALIFY"},
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
			opts := NewLanguageOptions()
			for _, kw := range tt.enable {
				opts.EnableReservableKeyword(kw, true)
			}
			for _, kw := range tt.disable {
				opts.EnableReservableKeyword(kw, false)
			}
			got := opts.ToProto().GetReservedKeywords()
			sort.Strings(got)
			sort.Strings(tt.want)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// TestLanguageOptions_IntegrationWithAnalyzer verifies that language options
// set via SetLanguageOptions are correctly propagated to the WASM analyzer.
// Two distinct literals triangulate that the resolved output reflects the input.
func TestLanguageOptions_IntegrationWithAnalyzer(t *testing.T) {
	a := newTestAnalyzer(t)
	ctx := context.Background()

	type payload struct {
		ColumnCount  int
		LiteralValue int64
	}

	tests := []struct {
		name string
		sql  string
		want payload
	}{
		{name: "literal 1", sql: "SELECT 1", want: payload{ColumnCount: 1, LiteralValue: 1}},
		{name: "literal 77", sql: "SELECT 77", want: payload{ColumnCount: 1, LiteralValue: 77}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang := NewLanguageOptions()
			lang.EnableLanguageFeature(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS)
			lang.SetSupportedStatementKinds([]generated.ResolvedNodeKind{
				generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
			})

			opts := NewAnalyzerOptions()
			opts.SetLanguageOptions(lang)

			out, err := a.AnalyzeStatement(ctx, tt.sql, nil, opts)
			if err != nil {
				t.Fatalf("AnalyzeStatement: %v", err)
			}

			stmt := out.ResolvedStatement().(*resolved_ast.QueryStmtNode)
			literal := findNode[*resolved_ast.LiteralNode](t, stmt)
			got := payload{
				ColumnCount:  len(stmt.OutputColumnList()),
				LiteralValue: literal.Value().GetValue().GetInt64Value(),
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// TestLanguageOptions_RejectsUnsupportedStatementKind verifies that
// SupportedStatementKinds constraints are enforced by the WASM analyzer.
// Two different constraint scenarios triangulate that enforcement is general.
func TestLanguageOptions_RejectsUnsupportedStatementKind(t *testing.T) {
	a := newTestAnalyzer(t)
	ctx := context.Background()

	tests := []struct {
		name          string
		allowedKinds  []generated.ResolvedNodeKind
		sql           string
		wantErrorType bool
	}{
		{
			name:          "INSERT-only rejects SELECT",
			allowedKinds:  []generated.ResolvedNodeKind{generated.ResolvedNodeKind_RESOLVED_INSERT_STMT},
			sql:           "SELECT 1",
			wantErrorType: true,
		},
		{
			name:          "DELETE-only rejects SELECT",
			allowedKinds:  []generated.ResolvedNodeKind{generated.ResolvedNodeKind_RESOLVED_DELETE_STMT},
			sql:           "SELECT 1",
			wantErrorType: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang := NewLanguageOptions()
			lang.SetSupportedStatementKinds(tt.allowedKinds)

			opts := NewAnalyzerOptions()
			opts.SetLanguageOptions(lang)

			_, err := a.AnalyzeStatement(ctx, tt.sql, nil, opts)
			if err == nil {
				t.Fatal("expected error but got nil")
			}
			var analyzeErr *AnalyzeError
			gotIsAnalyzeError := errors.As(err, &analyzeErr)
			if gotIsAnalyzeError != tt.wantErrorType {
				t.Errorf("errors.As(*AnalyzeError) = %v, want %v; err = %v", gotIsAnalyzeError, tt.wantErrorType, err)
			}
		})
	}
}
