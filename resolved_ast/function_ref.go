package resolved_ast

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// FunctionRef is the typed Go view of FunctionRefProto. It carries only
// the fully-qualified function name (group:path/name), which is what the
// proto exposes today.
type FunctionRef struct {
	Name string
}

func wrapFunctionRef(p *generated.FunctionRefProto) *FunctionRef {
	if p == nil {
		return nil
	}
	return &FunctionRef{Name: p.GetName()}
}

// WrapFunctionRef lifts a *generated.FunctionRefProto into the typed
// FunctionRef view. Returns nil for nil input.
func WrapFunctionRef(p *generated.FunctionRefProto) *FunctionRef {
	return wrapFunctionRef(p)
}
