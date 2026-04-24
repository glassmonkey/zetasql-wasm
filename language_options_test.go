package zetasql

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

func TestLanguageOptions_EnableFeature(t *testing.T) {
	opts := NewLanguageOptions()

	opts.EnableLanguageFeature(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS)
	opts.EnableLanguageFeature(generated.LanguageFeature_FEATURE_TABLESAMPLE)

	if got, want := opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS), true; got != want {
		t.Errorf("LanguageFeatureEnabled(ANALYTIC_FUNCTIONS) = %v, want %v", got, want)
	}
	if got, want := opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_TABLESAMPLE), true; got != want {
		t.Errorf("LanguageFeatureEnabled(TABLESAMPLE) = %v, want %v", got, want)
	}
	if got, want := opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_TIMESTAMP_NANOS), false; got != want {
		t.Errorf("LanguageFeatureEnabled(TIMESTAMP_NANOS) = %v, want %v", got, want)
	}

	proto := opts.ToProto()
	if got, want := len(proto.GetEnabledLanguageFeatures()), 2; got != want {
		t.Fatalf("len(EnabledLanguageFeatures) = %d, want %d", got, want)
	}
}

func TestLanguageOptions_DisableAll(t *testing.T) {
	opts := NewLanguageOptions()
	opts.EnableLanguageFeature(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS)
	opts.DisableAllLanguageFeatures()

	if got, want := opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS), false; got != want {
		t.Errorf("after DisableAll, LanguageFeatureEnabled(ANALYTIC_FUNCTIONS) = %v, want %v", got, want)
	}

	proto := opts.ToProto()
	if got, want := len(proto.GetEnabledLanguageFeatures()), 0; got != want {
		t.Errorf("len(EnabledLanguageFeatures) = %d, want %d", got, want)
	}
}

func TestLanguageOptions_SupportedStatementKinds(t *testing.T) {
	opts := NewLanguageOptions()
	kinds := []generated.ResolvedNodeKind{
		generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
		generated.ResolvedNodeKind_RESOLVED_INSERT_STMT,
	}
	opts.SetSupportedStatementKinds(kinds)

	proto := opts.ToProto()
	if got, want := len(proto.GetSupportedStatementKinds()), 2; got != want {
		t.Fatalf("len(SupportedStatementKinds) = %d, want %d", got, want)
	}
	if got, want := proto.GetSupportedStatementKinds()[0], generated.ResolvedNodeKind_RESOLVED_QUERY_STMT; got != want {
		t.Errorf("SupportedStatementKinds[0] = %v, want %v", got, want)
	}
}

func TestLanguageOptions_SupportsAllStatementKinds(t *testing.T) {
	opts := NewLanguageOptions()
	opts.SetSupportedStatementKinds([]generated.ResolvedNodeKind{
		generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
	})
	opts.SetSupportsAllStatementKinds()

	proto := opts.ToProto()
	// Empty signals "all supported"
	if got, want := len(proto.GetSupportedStatementKinds()), 0; got != want {
		t.Errorf("len(SupportedStatementKinds) = %d, want %d (empty = all)", got, want)
	}
}

func TestLanguageOptions_EnableMaximumLanguageFeatures(t *testing.T) {
	opts := NewLanguageOptions()
	opts.EnableMaximumLanguageFeatures()

	// Should enable many features
	proto := opts.ToProto()
	if got := len(proto.GetEnabledLanguageFeatures()); got < 50 {
		t.Errorf("EnableMaximumLanguageFeatures enabled only %d features, expected > 50", got)
	}

	// Test features should NOT be included
	if opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_TEST_IDEALLY_ENABLED_BUT_IN_DEVELOPMENT) {
		t.Error("test features should not be enabled by EnableMaximumLanguageFeatures")
	}

	// Released features SHOULD be included
	if got, want := opts.LanguageFeatureEnabled(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS), true; got != want {
		t.Errorf("LanguageFeatureEnabled(ANALYTIC_FUNCTIONS) = %v, want %v", got, want)
	}
}

func TestLanguageOptions_EnableMaximumLanguageFeaturesForDevelopment(t *testing.T) {
	opts := NewLanguageOptions()
	opts.EnableMaximumLanguageFeaturesForDevelopment()

	released := NewLanguageOptions()
	released.EnableMaximumLanguageFeatures()

	// Development should enable at least as many as released
	devCount := len(opts.ToProto().GetEnabledLanguageFeatures())
	relCount := len(released.ToProto().GetEnabledLanguageFeatures())
	if devCount < relCount {
		t.Errorf("development features (%d) < released features (%d)", devCount, relCount)
	}
}

func TestLanguageOptions_EnableReservableKeyword(t *testing.T) {
	opts := NewLanguageOptions()
	opts.EnableReservableKeyword("QUALIFY", true)
	opts.EnableReservableKeyword("PIVOT", true)

	proto := opts.ToProto()
	if got, want := len(proto.GetReservedKeywords()), 2; got != want {
		t.Fatalf("len(ReservedKeywords) = %d, want %d", got, want)
	}

	// Disable one
	opts.EnableReservableKeyword("PIVOT", false)
	proto = opts.ToProto()
	if got, want := len(proto.GetReservedKeywords()), 1; got != want {
		t.Fatalf("after disable, len(ReservedKeywords) = %d, want %d", got, want)
	}
	if got, want := proto.GetReservedKeywords()[0], "QUALIFY"; got != want {
		t.Errorf("ReservedKeywords[0] = %q, want %q", got, want)
	}
}

func TestLanguageOptions_IntegrationWithAnalyzer(t *testing.T) {
	ctx := t.Context()
	a, err := NewAnalyzer(ctx)
	if err != nil {
		t.Fatalf("NewAnalyzer: %v", err)
	}
	defer a.Close(ctx)

	lang := NewLanguageOptions()
	lang.EnableLanguageFeature(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS)
	lang.SetSupportedStatementKinds([]generated.ResolvedNodeKind{
		generated.ResolvedNodeKind_RESOLVED_QUERY_STMT,
	})

	opts := NewAnalyzerOptions()
	opts.SetLanguageOptions(lang)

	out, err := a.AnalyzeStatement(ctx, "SELECT 1", nil, opts)
	if err != nil {
		t.Fatalf("AnalyzeStatement: %v", err)
	}
	if out.ResolvedStatement() == nil {
		t.Fatal("ResolvedStatement() = nil")
	}
}
