package catalog

import (
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// SimpleCatalog represents a ZetaSQL catalog containing tables, functions, and sub-catalogs.
type SimpleCatalog struct {
	Name           string
	Tables         []*SimpleTable
	Functions      []*types.Function
	SubCatalogs    []*SimpleCatalog
	BuiltinOptions *generated.ZetaSQLBuiltinFunctionOptionsProto
}

// NewSimpleCatalog creates a SimpleCatalog with the given name.
func NewSimpleCatalog(name string) *SimpleCatalog {
	return &SimpleCatalog{Name: name}
}

// AddZetaSQLBuiltinFunctions signals that built-in ZetaSQL functions should be
// loaded on the WASM side. Pass nil for default behavior (load all builtins);
// nil is normalized to an empty options proto so the C++ side can distinguish
// "load builtins with defaults" from "do not load builtins" (BuiltinOptions == nil).
func (c *SimpleCatalog) AddZetaSQLBuiltinFunctions(opts *generated.ZetaSQLBuiltinFunctionOptionsProto) {
	if opts == nil {
		opts = &generated.ZetaSQLBuiltinFunctionOptionsProto{}
	}
	c.BuiltinOptions = opts
}

// ToProto serializes this catalog to a SimpleCatalogProto.
func (c *SimpleCatalog) ToProto() *generated.SimpleCatalogProto {
	name := c.Name
	p := &generated.SimpleCatalogProto{
		Name: &name,
	}
	for _, t := range c.Tables {
		p.Table = append(p.Table, t.ToProto())
	}
	for _, sub := range c.SubCatalogs {
		p.Catalog = append(p.Catalog, sub.ToProto())
	}
	for _, fn := range c.Functions {
		p.CustomFunction = append(p.CustomFunction, fn.ToProto())
	}
	if c.BuiltinOptions != nil {
		p.BuiltinFunctionOptions = c.BuiltinOptions
	}
	return p
}
