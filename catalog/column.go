package catalog

import (
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// SimpleColumn represents a column in a ZetaSQL table.
type SimpleColumn struct {
	tableName      string
	name           string
	typ            types.Type
	isPseudoColumn bool
	isWritable     bool
}

// NewSimpleColumn creates a SimpleColumn with the given table name, column name, and type.
func NewSimpleColumn(tableName, name string, typ types.Type) *SimpleColumn {
	return &SimpleColumn{
		tableName:  tableName,
		name:       name,
		typ:        typ,
		isWritable: true,
	}
}

func (c *SimpleColumn) Name() string     { return c.name }
func (c *SimpleColumn) FullName() string { return c.tableName + "." + c.name }
func (c *SimpleColumn) Type() types.Type { return c.typ }
func (c *SimpleColumn) IsPseudoColumn() bool     { return c.isPseudoColumn }
func (c *SimpleColumn) SetIsPseudoColumn(v bool)  { c.isPseudoColumn = v }
func (c *SimpleColumn) IsWritable() bool          { return c.isWritable }
func (c *SimpleColumn) SetIsWritable(v bool)      { c.isWritable = v }

// ToProto serializes this column to a SimpleColumnProto.
func (c *SimpleColumn) ToProto() *generated.SimpleColumnProto {
	name := c.name
	isPseudo := c.isPseudoColumn
	isWritable := c.isWritable
	return &generated.SimpleColumnProto{
		Name:             &name,
		Type:             c.typ.ToProto(),
		IsPseudoColumn:   &isPseudo,
		IsWritableColumn: &isWritable,
	}
}
