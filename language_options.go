package zetasql

import (
	"strings"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// LanguageFeature is a single ZetaSQL language feature flag.
//
// Use the Feature* constants in this package (or zetasql-wasm/wasm/generated
// for ones not yet exposed by name) to set values; LanguageOptions tracks the
// enabled set as a map.
type LanguageFeature int32

// String returns the canonical proto enum name (e.g. "FEATURE_ANALYTIC_FUNCTIONS").
func (f LanguageFeature) String() string {
	return generated.LanguageFeature(f).String()
}

func (f LanguageFeature) toProto() generated.LanguageFeature {
	return generated.LanguageFeature(f)
}

// NameResolutionMode controls how the analyzer resolves unqualified names.
type NameResolutionMode int32

const (
	NameResolutionDefault NameResolutionMode = NameResolutionMode(generated.NameResolutionMode_NAME_RESOLUTION_DEFAULT)
	NameResolutionStrict  NameResolutionMode = NameResolutionMode(generated.NameResolutionMode_NAME_RESOLUTION_STRICT)
)

// String returns the canonical proto enum name (e.g. "NAME_RESOLUTION_DEFAULT").
func (m NameResolutionMode) String() string {
	return generated.NameResolutionMode(m).String()
}

func (m NameResolutionMode) toProto() generated.NameResolutionMode {
	return generated.NameResolutionMode(m)
}

// ProductMode selects between INTERNAL (Google internal) and EXTERNAL
// (open-source / customer-facing) ZetaSQL behaviour.
type ProductMode int32

const (
	ProductInternal ProductMode = ProductMode(generated.ProductMode_PRODUCT_INTERNAL)
	ProductExternal ProductMode = ProductMode(generated.ProductMode_PRODUCT_EXTERNAL)
)

// String returns the canonical proto enum name (e.g. "PRODUCT_INTERNAL").
func (m ProductMode) String() string {
	return generated.ProductMode(m).String()
}

func (m ProductMode) toProto() generated.ProductMode {
	return generated.ProductMode(m)
}

// LanguageOptions controls which ZetaSQL language features are enabled.
//
// StatementKinds intentionally retains the proto type
// generated.ResolvedNodeKind: the enum has 287 members and a parallel
// named-type wrapper is not worth the cost. Callers pass values from the
// generated package directly.
type LanguageOptions struct {
	Features           map[LanguageFeature]bool
	StatementKinds     []generated.ResolvedNodeKind
	AllStatements      bool
	Keywords           map[string]bool
	NameResolutionMode NameResolutionMode
	ProductMode        ProductMode
}

// NewLanguageOptions creates LanguageOptions with no features enabled.
func NewLanguageOptions() *LanguageOptions {
	return &LanguageOptions{
		Features: map[LanguageFeature]bool{},
		Keywords: map[string]bool{},
	}
}

// EnableLanguageFeature enables the given language feature.
func (o *LanguageOptions) EnableLanguageFeature(f LanguageFeature) {
	o.Features[f] = true
}

// DisableAllLanguageFeatures disables all language features.
func (o *LanguageOptions) DisableAllLanguageFeatures() {
	o.Features = map[LanguageFeature]bool{}
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
			o.Features[LanguageFeature(id)] = true
		}
	}
}

// EnableMaximumLanguageFeaturesForDevelopment enables all language features
// including those still in development.
func (o *LanguageOptions) EnableMaximumLanguageFeaturesForDevelopment() {
	for id, name := range generated.LanguageFeature_name {
		if !isTestFeature(name) {
			o.Features[LanguageFeature(id)] = true
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
		c.Features = make(map[LanguageFeature]bool, len(o.Features))
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
		p.EnabledLanguageFeatures = append(p.EnabledLanguageFeatures, f.toProto())
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

	if o.NameResolutionMode != NameResolutionDefault {
		m := o.NameResolutionMode.toProto()
		p.NameResolutionMode = &m
	}

	if o.ProductMode != ProductInternal {
		m := o.ProductMode.toProto()
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
