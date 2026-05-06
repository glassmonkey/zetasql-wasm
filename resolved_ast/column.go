package resolved_ast

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// Column is the typed Go view of ResolvedColumnProto, the column reference
// embedded throughout the resolved AST (every scan's ColumnList, group-by
// lists, INSERT target columns, etc).
//
// Type and AnnotationMap remain proto-typed for now: a full read-side wrap
// of *generated.TypeProto is recursive (struct of array of …) and tracked
// separately. Most callers use only ID / TableName / Name; those that need
// the typed shape can read the proto directly until WrapType lands.
type Column struct {
	// ID mirrors proto field column_id, Go-cased per the golint
	// initialism rule (Id → ID).
	ID            int64
	TableName     string
	Name          string
	Type          *generated.TypeProto
	AnnotationMap *generated.AnnotationMapProto
}

func wrapColumn(p *generated.ResolvedColumnProto) *Column {
	if p == nil {
		return nil
	}
	return &Column{
		ID:            p.GetColumnId(),
		TableName:     p.GetTableName(),
		Name:          p.GetName(),
		Type:          p.GetType(),
		AnnotationMap: p.GetAnnotationMap(),
	}
}

func wrapColumnSlice(ps []*generated.ResolvedColumnProto) []*Column {
	if ps == nil {
		return nil
	}
	out := make([]*Column, len(ps))
	for i, p := range ps {
		out[i] = wrapColumn(p)
	}
	return out
}

// WrapColumn lifts a *generated.ResolvedColumnProto into the typed Column
// view. Returns nil for nil input. Useful when a caller holds a raw proto
// and needs the Go-side shape.
func WrapColumn(p *generated.ResolvedColumnProto) *Column { return wrapColumn(p) }
