package zetasql

import (
	"strings"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// LanguageOptions controls which ZetaSQL language features are enabled.
type LanguageOptions struct {
	Features           map[generated.LanguageFeature]bool
	StatementKinds     []generated.ResolvedNodeKind
	AllStatements      bool
	Keywords           map[string]bool
	NameResolutionMode generated.NameResolutionMode
	ProductMode        generated.ProductMode
}

// NewLanguageOptions creates LanguageOptions with no features enabled.
func NewLanguageOptions() *LanguageOptions {
	return &LanguageOptions{
		Features: map[generated.LanguageFeature]bool{},
		Keywords: map[string]bool{},
	}
}

// EnableLanguageFeature enables the given language feature.
func (o *LanguageOptions) EnableLanguageFeature(f generated.LanguageFeature) {
	o.Features[f] = true
}

// DisableAllLanguageFeatures disables all language features.
func (o *LanguageOptions) DisableAllLanguageFeatures() {
	o.Features = map[generated.LanguageFeature]bool{}
}

// SetSupportedStatementKinds sets the allowed statement kinds.
func (o *LanguageOptions) SetSupportedStatementKinds(kinds []generated.ResolvedNodeKind) {
	o.StatementKinds = kinds
	o.AllStatements = false
}

// SetSupportsAllStatementKinds enables support for all statement kinds.
func (o *LanguageOptions) SetSupportsAllStatementKinds() {
	o.AllStatements = true
	o.StatementKinds = nil
}

// EnableMaximumLanguageFeatures enables all released (non-test) language features.
func (o *LanguageOptions) EnableMaximumLanguageFeatures() {
	for id, name := range generated.LanguageFeature_name {
		if isReleasedFeature(id, name) {
			o.Features[generated.LanguageFeature(id)] = true
		}
	}
}

// EnableMaximumLanguageFeaturesForDevelopment enables all language features
// including those still in development.
func (o *LanguageOptions) EnableMaximumLanguageFeaturesForDevelopment() {
	for id, name := range generated.LanguageFeature_name {
		if !isTestFeature(name) {
			o.Features[generated.LanguageFeature(id)] = true
		}
	}
}

// EnableReservableKeyword sets whether a keyword is reserved.
func (o *LanguageOptions) EnableReservableKeyword(keyword string, enable bool) {
	if enable {
		o.Keywords[keyword] = true
	} else {
		delete(o.Keywords, keyword)
	}
}

// clone returns a deep copy of the LanguageOptions, including independent
// copies of the Features map, StatementKinds slice, and Keywords map.
func (o *LanguageOptions) clone() *LanguageOptions {
	c := &LanguageOptions{
		AllStatements:      o.AllStatements,
		NameResolutionMode: o.NameResolutionMode,
		ProductMode:        o.ProductMode,
	}
	if o.Features != nil {
		c.Features = make(map[generated.LanguageFeature]bool, len(o.Features))
		for k, v := range o.Features {
			c.Features[k] = v
		}
	}
	if o.StatementKinds != nil {
		c.StatementKinds = make([]generated.ResolvedNodeKind, len(o.StatementKinds))
		copy(c.StatementKinds, o.StatementKinds)
	}
	if o.Keywords != nil {
		c.Keywords = make(map[string]bool, len(o.Keywords))
		for k, v := range o.Keywords {
			c.Keywords[k] = v
		}
	}
	return c
}

// toProto converts to the protobuf representation.
func (o *LanguageOptions) toProto() *generated.LanguageOptionsProto {
	p := &generated.LanguageOptionsProto{}

	for f := range o.Features {
		p.EnabledLanguageFeatures = append(p.EnabledLanguageFeatures, f)
	}

	if o.AllStatements {
		// Empty list signals "all supported" to the C++ side
		p.SupportedStatementKinds = nil
	} else if len(o.StatementKinds) > 0 {
		p.SupportedStatementKinds = o.StatementKinds
	}

	for kw := range o.Keywords {
		p.ReservedKeywords = append(p.ReservedKeywords, kw)
	}

	if o.NameResolutionMode != generated.NameResolutionMode_NAME_RESOLUTION_DEFAULT {
		m := o.NameResolutionMode
		p.NameResolutionMode = &m
	}

	if o.ProductMode != generated.ProductMode_PRODUCT_INTERNAL {
		m := o.ProductMode
		p.ProductMode = &m
	}

	return p
}

// isReleasedFeature returns true if the feature is released (not test, not in-development).
func isReleasedFeature(id int32, name string) bool {
	if isTestFeature(name) {
		return false
	}
	if strings.Contains(name, "IN_DEVELOPMENT") {
		return false
	}
	return true
}

// isTestFeature returns true if the feature is a test-only feature (id >= 999990).
func isTestFeature(name string) bool {
	return strings.HasPrefix(name, "FEATURE_TEST_")
}
