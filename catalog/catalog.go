package catalog

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// SimpleCatalog represents a ZetaSQL catalog containing tables, functions, and sub-catalogs.
type SimpleCatalog struct {
	name           string
	tables         []*SimpleTable
	subCatalogs    []*SimpleCatalog
	builtinOptions *generated.ZetaSQLBuiltinFunctionOptionsProto
}

// NewSimpleCatalog creates a SimpleCatalog with the given name.
func NewSimpleCatalog(name string) *SimpleCatalog {
	return &SimpleCatalog{name: name}
}

func (c *SimpleCatalog) Name() string { return c.name }

// AddTable adds a table to this catalog.
func (c *SimpleCatalog) AddTable(table *SimpleTable) {
	c.tables = append(c.tables, table)
}

// AddSubCatalog adds a nested catalog.
func (c *SimpleCatalog) AddSubCatalog(sub *SimpleCatalog) {
	c.subCatalogs = append(c.subCatalogs, sub)
}

// AddZetaSQLBuiltinFunctions signals that built-in ZetaSQL functions should be
// loaded on the WASM side. Pass nil for default behavior (load all builtins).
func (c *SimpleCatalog) AddZetaSQLBuiltinFunctions(opts *generated.ZetaSQLBuiltinFunctionOptionsProto) {
	if opts == nil {
		opts = &generated.ZetaSQLBuiltinFunctionOptionsProto{}
	}
	c.builtinOptions = opts
}

// ToProto serializes this catalog to a SimpleCatalogProto.
func (c *SimpleCatalog) ToProto() *generated.SimpleCatalogProto {
	name := c.name
	p := &generated.SimpleCatalogProto{
		Name: &name,
	}
	for _, t := range c.tables {
		p.Table = append(p.Table, t.ToProto())
	}
	for _, sub := range c.subCatalogs {
		p.Catalog = append(p.Catalog, sub.ToProto())
	}
	if c.builtinOptions != nil {
		p.BuiltinFunctionOptions = c.builtinOptions
	}
	return p
}
