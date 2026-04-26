package catalog

import (
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// SimpleColumn represents a column in a ZetaSQL table.
type SimpleColumn struct {
	TableName      string
	Name           string
	Type           types.Type
	IsPseudoColumn bool
	IsWritable     bool
}

// NewSimpleColumn creates a SimpleColumn with the given table name, column
// name, and type. IsWritable defaults to true to match ZetaSQL's expectation
// that ordinary columns can be written.
func NewSimpleColumn(tableName, name string, typ types.Type) *SimpleColumn {
	return &SimpleColumn{
		TableName:  tableName,
		Name:       name,
		Type:       typ,
		IsWritable: true,
	}
}

// FullName returns "<TableName>.<Name>".
func (c *SimpleColumn) FullName() string { return c.TableName + "." + c.Name }

// ToProto serializes this column to a SimpleColumnProto.
func (c *SimpleColumn) ToProto() *generated.SimpleColumnProto {
	name := c.Name
	isPseudo := c.IsPseudoColumn
	isWritable := c.IsWritable
	return &generated.SimpleColumnProto{
		Name:             &name,
		Type:             c.Type.ToProto(),
		IsPseudoColumn:   &isPseudo,
		IsWritableColumn: &isWritable,
	}
}
