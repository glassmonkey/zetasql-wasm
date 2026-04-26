package catalog

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// SimpleTable represents a ZetaSQL table with columns.
type SimpleTable struct {
	Name         string
	Columns      []*SimpleColumn
	IsValueTable bool
}

// NewSimpleTable creates a SimpleTable with the given name and optional columns.
func NewSimpleTable(name string, columns ...*SimpleColumn) *SimpleTable {
	return &SimpleTable{
		Name:    name,
		Columns: columns,
	}
}

// ToProto serializes this table to a SimpleTableProto.
func (t *SimpleTable) ToProto() *generated.SimpleTableProto {
	name := t.Name
	isVT := t.IsValueTable
	cols := make([]*generated.SimpleColumnProto, len(t.Columns))
	for i, c := range t.Columns {
		cols[i] = c.ToProto()
	}
	return &generated.SimpleTableProto{
		Name:         &name,
		IsValueTable: &isVT,
		Column:       cols,
	}
}
