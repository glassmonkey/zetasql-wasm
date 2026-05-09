package types

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// ErrNotFound is returned by SimpleCatalog Find* methods when the requested
// object does not exist.
var ErrNotFound = errors.New("not found")

// SimpleCatalog represents a ZetaSQL catalog containing tables, functions, and sub-catalogs.
// ZetaSQL built-in functions are loaded automatically by Engine.Analyze, with
// the analyzer's LanguageOptions reflected, so callers do not need to register
// them manually.
type SimpleCatalog struct {
	Name        string
	Tables      []*SimpleTable
	Functions   []*Function
	SubCatalogs []*SimpleCatalog
}

// NewSimpleCatalog creates a SimpleCatalog with the given name.
func NewSimpleCatalog(name string) *SimpleCatalog {
	return &SimpleCatalog{Name: name}
}

// FindTable looks up a table by name path, descending through sub-catalogs
// for multi-segment paths. The last segment matches against Tables, earlier
// segments match against SubCatalogs. Comparisons are case-insensitive
// (ZetaSQL's convention). Returns ErrNotFound when no match exists.
func (c *SimpleCatalog) FindTable(namePath []string) (*SimpleTable, error) {
	if len(namePath) == 0 {
		return nil, fmt.Errorf("FindTable: empty namePath: %w", ErrNotFound)
	}
	cur, err := c.descend(namePath[:len(namePath)-1])
	if err != nil {
		return nil, fmt.Errorf("FindTable %v: %w", namePath, err)
	}
	last := namePath[len(namePath)-1]
	for _, t := range cur.Tables {
		if strings.EqualFold(t.Name, last) {
			return t, nil
		}
	}
	return nil, fmt.Errorf("FindTable %v: %w", namePath, ErrNotFound)
}

// FindFunction looks up a function whose NamePath equals the given path
// (case-insensitive). Functions registered at the root catalog are searched
// first, then each sub-catalog recursively. Returns ErrNotFound when no
// match exists.
func (c *SimpleCatalog) FindFunction(namePath []string) (*Function, error) {
	if fn := c.findFunctionLocal(namePath); fn != nil {
		return fn, nil
	}
	for _, sub := range c.SubCatalogs {
		if fn, err := sub.FindFunction(namePath); err == nil {
			return fn, nil
		}
	}
	return nil, fmt.Errorf("FindFunction %v: %w", namePath, ErrNotFound)
}

func (c *SimpleCatalog) findFunctionLocal(namePath []string) *Function {
	for _, fn := range c.Functions {
		if equalNamePathFold(fn.NamePath, namePath) {
			return fn
		}
	}
	return nil
}

func (c *SimpleCatalog) descend(prefix []string) (*SimpleCatalog, error) {
	cur := c
	for _, part := range prefix {
		var next *SimpleCatalog
		for _, sub := range cur.SubCatalogs {
			if strings.EqualFold(sub.Name, part) {
				next = sub
				break
			}
		}
		if next == nil {
			return nil, fmt.Errorf("sub-catalog %q: %w", part, ErrNotFound)
		}
		cur = next
	}
	return cur, nil
}

func equalNamePathFold(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
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
	return p
}
