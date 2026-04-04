package quickjs

import "errors"

var errValueSpecFactoryRequired = errors.New("value spec factory is required")

// ValueSpec describes how to materialize a JavaScript value for a target Context.
// Module/Class builders use ValueSpec to store definitions instead of long-lived JSValue pointers.
type ValueSpec interface {
	Materialize(ctx *Context) (*Value, error)
}

// LiteralSpec materializes a value from a literal Go value.
type LiteralSpec struct {
	Value interface{}
}

func (s LiteralSpec) Materialize(ctx *Context) (*Value, error) {
	if s.Value == nil {
		return ctx.NewNull(), nil
	}
	return ctx.Marshal(s.Value)
}

// MarshalSpec materializes a value via Context.Marshal.
type MarshalSpec struct {
	Value interface{}
}

func (s MarshalSpec) Materialize(ctx *Context) (*Value, error) {
	return ctx.Marshal(s.Value)
}

// FactorySpec materializes a value by running a Context-aware factory.
type FactorySpec struct {
	Factory func(ctx *Context) (*Value, error)
}

func (s FactorySpec) Materialize(ctx *Context) (*Value, error) {
	if s.Factory == nil {
		return nil, errValueSpecFactoryRequired
	}
	return s.Factory(ctx)
}

// contextValueSpec preserves legacy, context-bound Export(name, *Value) behavior.
type contextValueSpec struct {
	value *Value
}

func (s contextValueSpec) Materialize(_ *Context) (*Value, error) {
	if s.value == nil {
		return nil, errors.New("module export value is nil")
	}
	return s.value, nil
}

func isContextValueSpec(spec ValueSpec) bool {
	if spec == nil {
		return false
	}
	_, ok := spec.(contextValueSpec)
	if ok {
		return true
	}
	_, ok = spec.(*contextValueSpec)
	return ok
}
