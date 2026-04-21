package catalog

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// SimpleTable represents a ZetaSQL table with columns.
type SimpleTable struct {
	name         string
	columns      []*SimpleColumn
	isValueTable bool
}

// NewSimpleTable creates a SimpleTable with the given name and optional columns.
func NewSimpleTable(name string, columns ...*SimpleColumn) *SimpleTable {
	return &SimpleTable{
		name:    name,
		columns: columns,
	}
}

func (t *SimpleTable) Name() string               { return t.name }
func (t *SimpleTable) NumColumns() int             { return len(t.columns) }
func (t *SimpleTable) Column(i int) *SimpleColumn  { return t.columns[i] }
func (t *SimpleTable) IsValueTable() bool          { return t.isValueTable }
func (t *SimpleTable) SetIsValueTable(v bool)      { t.isValueTable = v }

// AddColumn appends a column to this table.
func (t *SimpleTable) AddColumn(col *SimpleColumn) {
	t.columns = append(t.columns, col)
}

// ToProto serializes this table to a SimpleTableProto.
func (t *SimpleTable) ToProto() *generated.SimpleTableProto {
	name := t.name
	isVT := t.isValueTable
	cols := make([]*generated.SimpleColumnProto, len(t.columns))
	for i, c := range t.columns {
		cols[i] = c.ToProto()
	}
	return &generated.SimpleTableProto{
		Name:         &name,
		IsValueTable: &isVT,
		Column:       cols,
	}
}
