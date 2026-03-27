package quickjs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBoundaryContracts provides a single CI-friendly entry point for
// cross-file boundary regression suites.
func TestBoundaryContracts(t *testing.T) {
	t.Run("RuntimeNilSafety", TestRuntimeNilSafety)
	t.Run("RuntimeCloseConcurrentSchedule", TestRuntimeCloseWithConcurrentSchedule)
	t.Run("RuntimePromiseCancelScheduleCloseInterleaving", TestRuntimePromiseCancelScheduleCloseInterleaving)
	t.Run("ContextInternalsCoverage", TestContextInternalsCoverage)
	t.Run("CrossContextContracts", TestBoundaryCrossContext)
	t.Run("MappingIntegrityContracts", TestMappingIntegrityContracts)
	t.Run("BridgeBoundaryContracts", TestBridgeBoundaryContracts)
	t.Run("ModuleErrorBranches", TestModuleBuilder_ErrorBranches)
	t.Run("HandleReplacementPattern", TestHandleStore_HandleReplacementPattern)
}

// TestBoundaryCrossContext groups protections against foreign-context value misuse.
func TestBoundaryCrossContext(t *testing.T) {
	t.Run("ValueCrossContextGuards", TestValueCrossContextGuards)
	t.Run("ContextInternalsCoverage", TestContextInternalsCoverage)
}

// TestBoundarySmoke is a fast subset for PR precheck.
func TestBoundarySmoke(t *testing.T) {
	t.Run("RuntimeSmoke", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		result := ctx.Eval(`1 + 1`)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, int32(2), result.ToInt32())
	})

	t.Run("ContextSmoke", func(t *testing.T) {
		var nilCtx *Context
		require.Nil(t, nilCtx.NewString("x"))
		require.Nil(t, nilCtx.NewPromise(func(resolve, reject func(*Value)) {}))
	})

	t.Run("BridgeSmoke", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value { return nil })
		require.NotNil(t, fn)
		ctx.Globals().Set("smokeNilFn", fn)

		result := ctx.Eval(`typeof smokeNilFn()`)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "undefined", result.ToString())
	})

	t.Run("CrossContextSmoke", func(t *testing.T) {
		rt1 := NewRuntime()
		defer rt1.Close()
		ctx1 := rt1.NewContext()
		defer ctx1.Close()

		rt2 := NewRuntime()
		defer rt2.Close()
		ctx2 := rt2.NewContext()
		defer ctx2.Close()

		fn := ctx1.Eval(`(function(v){ return v; })`)
		defer fn.Free()
		require.False(t, fn.IsException())

		foreignArg := ctx2.NewInt32(1)
		defer foreignArg.Free()

		result := fn.Execute(nil, foreignArg)
		defer result.Free()
		require.True(t, result.IsException())
		require.Contains(t, ctx1.Exception().Error(), "cross-context argument")
	})

	t.Run("ModuleSmoke", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		module := NewModuleBuilder("smoke-nil-export").
			Export("value", ctx.NewString("x"))
		require.NoError(t, module.Build(ctx))
		require.GreaterOrEqual(t, len(module.exports), 1)

		module.exports[0].Value = nil
		result := ctx.Eval(`import('smoke-nil-export')`, EvalAwait(true))
		defer result.Free()
		require.True(t, result.IsException())
		require.Contains(t, ctx.Exception().Error(), "invalid module export value")
	})

	t.Run("HandleSmoke", func(t *testing.T) {
		hs := newHandleStore()
		id := hs.Store("smoke")
		v, ok := hs.Load(id)
		require.True(t, ok)
		require.Equal(t, "smoke", v)
		require.True(t, hs.Delete(id))
	})

	t.Run("MappingIntegritySmoke", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		contextMapping.Store(ctx.ref, "bad-context")
		require.Nil(t, getContextFromJS(ctx.ref))
		registerContext(ctx.ref, ctx)
		require.Same(t, ctx, getContextFromJS(ctx.ref))
	})
}
