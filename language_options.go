package zetasql

import (
	"strings"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// LanguageOptions controls which ZetaSQL language features are enabled.
type LanguageOptions struct {
	features       map[generated.LanguageFeature]bool
	statementKinds []generated.ResolvedNodeKind
	allStatements  bool
	keywords       map[string]bool // reserved keywords
}

// NewLanguageOptions creates LanguageOptions with no features enabled.
func NewLanguageOptions() *LanguageOptions {
	return &LanguageOptions{
		features: map[generated.LanguageFeature]bool{},
		keywords: map[string]bool{},
	}
}

// EnableLanguageFeature enables the given language feature.
func (o *LanguageOptions) EnableLanguageFeature(f generated.LanguageFeature) {
	o.features[f] = true
}

// LanguageFeatureEnabled returns whether the given feature is enabled.
func (o *LanguageOptions) LanguageFeatureEnabled(f generated.LanguageFeature) bool {
	return o.features[f]
}

// DisableAllLanguageFeatures disables all language features.
func (o *LanguageOptions) DisableAllLanguageFeatures() {
	o.features = map[generated.LanguageFeature]bool{}
}

// SetSupportedStatementKinds sets the allowed statement kinds.
func (o *LanguageOptions) SetSupportedStatementKinds(kinds []generated.ResolvedNodeKind) {
	o.statementKinds = kinds
	o.allStatements = false
}

// SetSupportsAllStatementKinds enables support for all statement kinds.
func (o *LanguageOptions) SetSupportsAllStatementKinds() {
	o.allStatements = true
	o.statementKinds = nil
}

// EnableMaximumLanguageFeatures enables all released (non-test) language features.
func (o *LanguageOptions) EnableMaximumLanguageFeatures() {
	for id, name := range generated.LanguageFeature_name {
		if isReleasedFeature(id, name) {
			o.features[generated.LanguageFeature(id)] = true
		}
	}
}

// EnableMaximumLanguageFeaturesForDevelopment enables all language features
// including those still in development.
func (o *LanguageOptions) EnableMaximumLanguageFeaturesForDevelopment() {
	for id, name := range generated.LanguageFeature_name {
		if !isTestFeature(name) {
			o.features[generated.LanguageFeature(id)] = true
		}
	}
}

// EnableReservableKeyword sets whether a keyword is reserved.
func (o *LanguageOptions) EnableReservableKeyword(keyword string, enable bool) {
	if enable {
		o.keywords[keyword] = true
	} else {
		delete(o.keywords, keyword)
	}
}

// ToProto converts to the protobuf representation.
func (o *LanguageOptions) ToProto() *generated.LanguageOptionsProto {
	p := &generated.LanguageOptionsProto{}

	for f := range o.features {
		p.EnabledLanguageFeatures = append(p.EnabledLanguageFeatures, f)
	}

	if o.allStatements {
		// Empty list signals "all supported" to the C++ side
		p.SupportedStatementKinds = nil
	} else if len(o.statementKinds) > 0 {
		p.SupportedStatementKinds = o.statementKinds
	}

	for kw := range o.keywords {
		p.ReservedKeywords = append(p.ReservedKeywords, kw)
	}

	return p
}

// isReleasedFeature returns true if the feature is released (not test, not in-development).
func isReleasedFeature(id int32, name string) bool {
	if isTestFeature(name) {
		return false
	}
	// Features with IN_DEVELOPMENT in their name are not yet released
	if strings.Contains(name, "IN_DEVELOPMENT") {
		return false
	}
	return true
}

// isTestFeature returns true if the feature is a test-only feature (id >= 999990).
func isTestFeature(name string) bool {
	return strings.HasPrefix(name, "FEATURE_TEST_")
}
