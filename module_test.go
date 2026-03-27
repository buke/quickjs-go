package quickjs

import (
	"fmt"
	"runtime/cgo"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// BASIC MODULE FUNCTIONALITY TESTS
// =============================================================================

func TestModuleBuilder_Basic(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("ModuleWithExports", func(t *testing.T) {
		ctx := newCtx(t)
		addFunc := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			if len(args) >= 2 {
				return ctx.NewFloat64(args[0].ToFloat64() + args[1].ToFloat64())
			}
			return ctx.NewFloat64(0)
		})

		module := NewModuleBuilder("math").
			Export("PI", ctx.NewFloat64(3.14159)).
			Export("add", addFunc).
			Export("version", ctx.NewString("1.0.0")).
			Export("default", ctx.NewString("Math Module"))

		err := module.Build(ctx)
		require.NoError(t, err)

		result := ctx.Eval(`
            (async function() {
                const { PI, add, version } = await import('math');
                return add(PI, 1.0);
            })()
        `, EvalAwait(true))
		defer result.Free()

		require.False(t, result.IsException())
		require.InDelta(t, 4.14159, result.ToFloat64(), 0.0001)
	})

	t.Run("DefaultExport", func(t *testing.T) {
		ctx := newCtx(t)
		module := NewModuleBuilder("default-test").
			Export("default", ctx.NewString("Default Export Value")).
			Export("name", ctx.NewString("test"))

		err := module.Build(ctx)
		require.NoError(t, err)

		result := ctx.Eval(`
            (async function() {
                const defaultValue = await import('default-test');
                return defaultValue.default;
            })()
        `, EvalAwait(true))
		defer result.Free()

		require.False(t, result.IsException())
		require.Equal(t, "Default Export Value", result.ToString())
	})
}

// =============================================================================
// MODULE IMPORT TESTS
// =============================================================================

func TestModuleBuilder_Import(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("NamedImports", func(t *testing.T) {
		ctx := newCtx(t)
		greetFunc := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			name := "World"
			if len(args) > 0 {
				name = args[0].ToString()
			}
			return ctx.NewString(fmt.Sprintf("Hello, %s!", name))
		})

		module := NewModuleBuilder("greeting").
			Export("greet", greetFunc).
			Export("defaultName", ctx.NewString("World"))

		err := module.Build(ctx)
		require.NoError(t, err)

		result := ctx.Eval(`
            (async function() {
                const { greet, defaultName } = await import('greeting');
                return greet('QuickJS');
            })()
        `, EvalAwait(true))
		defer result.Free()

		require.False(t, result.IsException())
		require.Equal(t, "Hello, QuickJS!", result.ToString())
	})

	t.Run("FunctionImports", func(t *testing.T) {
		ctx := newCtx(t)
		calculateFunc := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			if len(args) >= 2 {
				a, b := args[0].ToFloat64(), args[1].ToFloat64()
				return ctx.NewFloat64(a * b)
			}
			return ctx.NewFloat64(0)
		})

		module := NewModuleBuilder("calculator").
			Export("multiply", calculateFunc).
			Export("PI", ctx.NewFloat64(3.14159))

		err := module.Build(ctx)
		require.NoError(t, err)

		result := ctx.Eval(`
            (async function() {
                const { multiply, PI } = await import('calculator');
                return multiply(PI, 2);
            })()
        `, EvalAwait(true))
		defer result.Free()

		require.False(t, result.IsException())
		expected := 3.14159 * 2
		require.InDelta(t, expected, result.ToFloat64(), 0.0001)
	})

	t.Run("MixedImports", func(t *testing.T) {
		ctx := newCtx(t)
		module := NewModuleBuilder("utils").
			Export("version", ctx.NewString("1.0.0")).
			Export("debug", ctx.NewBool(true)).
			Export("default", ctx.NewString("Utils Library"))

		err := module.Build(ctx)
		require.NoError(t, err)

		result := ctx.Eval(`
            (async function() {
                const module = await import('utils');
                const { version, debug } = module;
                return version + ' - ' + module.default;
            })()
        `, EvalAwait(true))
		defer result.Free()

		require.False(t, result.IsException())
		require.Equal(t, "1.0.0 - Utils Library", result.ToString())
	})
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

func TestModuleBuilder_ErrorHandling(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("EmptyModule", func(t *testing.T) {
		ctx := newCtx(t)
		module := NewModuleBuilder("empty")
		err := module.Build(ctx)
		require.Error(t, err)
	})

	t.Run("EmptyModuleName", func(t *testing.T) {
		ctx := newCtx(t)
		module := NewModuleBuilder("")
		err := module.Build(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "module name cannot be empty")
	})

	t.Run("EmptyExportName", func(t *testing.T) {
		ctx := newCtx(t)
		module := NewModuleBuilder("test").Export("", ctx.NewString("invalid"))
		err := module.Build(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "export name cannot be empty")
	})

	t.Run("NilExportValue", func(t *testing.T) {
		ctx := newCtx(t)
		module := NewModuleBuilder("test").Export("value", nil)
		err := module.Build(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "export value cannot be nil: value")
	})

	t.Run("DuplicateExportNames", func(t *testing.T) {
		ctx := newCtx(t)
		module := NewModuleBuilder("test").
			Export("value", ctx.NewString("first")).
			Export("value", ctx.NewString("duplicate"))
		err := module.Build(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate export name: value")
	})

	t.Run("NilBuilder", func(t *testing.T) {
		ctx := newCtx(t)
		var module *ModuleBuilder
		err := module.Build(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "module builder is nil")
	})

	t.Run("NilContext", func(t *testing.T) {
		ctx := newCtx(t)
		module := NewModuleBuilder("test-nil-context").Export("value", ctx.NewString("ok"))
		err := module.Build(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid context")
	})

	t.Run("ClosedContext", func(t *testing.T) {
		rt2 := NewRuntime()
		defer rt2.Close()

		ctx2 := rt2.NewContext()
		module := NewModuleBuilder("test-closed-context").Export("value", ctx2.NewString("ok"))
		ctx2.Close()

		err := module.Build(ctx2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid context")
	})
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

func TestModuleBuilder_Integration(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("MultipleModules", func(t *testing.T) {
		ctx := newCtx(t)
		// Create math module
		addFunc := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			if len(args) >= 2 {
				return ctx.NewFloat64(args[0].ToFloat64() + args[1].ToFloat64())
			}
			return ctx.NewFloat64(0)
		})

		mathModule := NewModuleBuilder("math").
			Export("add", addFunc).
			Export("PI", ctx.NewFloat64(3.14159))

		// Create utils module
		utilsModule := NewModuleBuilder("utils").
			Export("name", ctx.NewString("UtilsModule")).
			Export("version", ctx.NewString("1.0.0"))

		err := mathModule.Build(ctx)
		require.NoError(t, err)

		err = utilsModule.Build(ctx)
		require.NoError(t, err)

		// Test math module
		mathResult := ctx.Eval(`
        (async function() {
            const { add, PI } = await import('math');
            return add(PI, 1);
        })()`, EvalAwait(true))
		defer mathResult.Free()
		require.False(t, mathResult.IsException())
		require.InDelta(t, 4.14159, mathResult.ToFloat64(), 0.0001)

		// Test utils module
		utilsResult := ctx.Eval(`
        (async function() {
            const { name, version } = await import('utils');
            return name + ' v' + version;
        })()`, EvalAwait(true))
		defer utilsResult.Free()
		require.False(t, utilsResult.IsException())
		require.Equal(t, "UtilsModule v1.0.0", utilsResult.ToString())
	})

	t.Run("ComplexModuleWithObjects", func(t *testing.T) {
		ctx := newCtx(t)
		config := ctx.NewObject()
		config.Set("name", ctx.NewString("TestApp"))
		config.Set("version", ctx.NewString("2.0.0"))
		config.Set("debug", ctx.NewBool(true))

		module := NewModuleBuilder("config").
			Export("config", config).
			Export("default", ctx.NewString("Configuration Module"))

		err := module.Build(ctx)
		require.NoError(t, err)

		result := ctx.Eval(`
            (async function() {
                const { config } = await import('config');
                return config.name + ' v' + config.version;
            })()
        `, EvalAwait(true))
		defer result.Free()

		require.False(t, result.IsException())
		require.Equal(t, "TestApp v2.0.0", result.ToString())
	})

	t.Run("ModuleWithErrorRecovery", func(t *testing.T) {
		ctx := newCtx(t)
		// Test that system can recover from errors
		badModule := NewModuleBuilder("").Export("test", ctx.NewString("value"))
		err := badModule.Build(ctx)
		require.Error(t, err)

		// Should be able to create good module after error
		goodModule := NewModuleBuilder("recovery").
			Export("message", ctx.NewString("Recovery successful"))
		err = goodModule.Build(ctx)
		require.NoError(t, err)

		result := ctx.Eval(`
            (async function() {
                const { message } = await import('recovery');
                return message;
            })()
        `, EvalAwait(true))
		defer result.Free()

		require.False(t, result.IsException())
		require.Equal(t, "Recovery successful", result.ToString())
	})
}

func TestModuleBuilder_ErrorBranches(t *testing.T) {
	newModuleCtx := func(t *testing.T) (*Runtime, *Context) {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return rt, ctx
	}

	t.Run("ModuleInitContextError", func(t *testing.T) {
		_, ctx := newModuleCtx(t)
		baseHandles := ctx.handleStore.Count()

		// Create module
		module := NewModuleBuilder("error-test-1").
			Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		// Unregister context before module initialization
		unregisterContext(ctx.ref)

		// Import will fail during initialization
		result := ctx.Eval(`import('error-test-1')`, EvalAwait(true))
		defer result.Free()

		// Re-register for cleanup
		registerContext(ctx.ref, ctx)

		// Should get context error
		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "Context not found")

		// This path cannot clean inside goModuleInitProxy because context lookup failed.
		assertModuleBuilderRetainedThenCleaned(t, ctx, "error-test-1", 1, baseHandles)
	})

	t.Run("ModuleInitHandleStoreError", func(t *testing.T) {
		_, ctx := newModuleCtx(t)
		// Create module
		module := NewModuleBuilder("error-test-2").
			Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		// Clear handle store before module initialization
		ctx.handleStore.Clear()
		baseHandles := ctx.handleStore.Count()

		// Import will fail during initialization
		result := ctx.Eval(`import('error-test-2')`, EvalAwait(true))
		defer result.Free()

		// Should get handle store error
		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "Function not found")
		assertModuleBuilderAbsent(t, ctx, "error-test-2", baseHandles)
	})

	t.Run("ModuleInitInvalidBuilderType", func(t *testing.T) {
		_, ctx := newModuleCtx(t)
		baseHandles := ctx.handleStore.Count()

		module := NewModuleBuilder("error-test-4").
			Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		corrupted := false
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			handleID := key.(int32)
			stored, ok := ctx.handleStore.Load(handleID)
			if !ok {
				return true
			}
			builder, ok := stored.(*ModuleBuilder)
			if !ok || builder.name != "error-test-4" {
				return true
			}

			oldHandle := value.(cgo.Handle)
			oldHandle.Delete()
			ctx.handleStore.handles.Store(handleID, cgo.NewHandle("not-a-builder"))
			corrupted = true
			return false
		})
		require.True(t, corrupted)

		result := ctx.Eval(`import('error-test-4')`, EvalAwait(true))
		defer result.Free()

		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "invalid module builder")
		assertModuleBuilderAbsent(t, ctx, "error-test-4", baseHandles)
	})

	t.Run("ModuleInitInvalidPrivateValue", func(t *testing.T) {
		_, ctx := newModuleCtx(t)
		baseHandles := ctx.handleStore.Count()

		module := NewModuleBuilder("error-test-5").
			Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		mutated := false
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			handleID := key.(int32)
			stored, ok := ctx.handleStore.Load(handleID)
			if !ok {
				return true
			}
			builder, ok := stored.(*ModuleBuilder)
			if !ok || builder.name != "error-test-5" {
				return true
			}

			oldHandle := value.(cgo.Handle)
			oldHandle.Delete()
			ctx.handleStore.handles.Store(handleID, cgo.NewHandle(3.14159))
			mutated = true
			return false
		})
		require.True(t, mutated)

		result := ctx.Eval(`import('error-test-5')`, EvalAwait(true))
		defer result.Free()

		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "invalid module builder")
		assertModuleBuilderAbsent(t, ctx, "error-test-5", baseHandles)
	})

	t.Run("ModuleInitSetModuleExportError", func(t *testing.T) {
		_, ctx := newModuleCtx(t)
		fooFunc := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewUndefined()
		})
		defer fooFunc.Free()
		baseHandles := ctx.handleStore.Count()

		module := NewModuleBuilder("error-test-3").Export("foo", fooFunc)
		err := module.Build(ctx)
		require.NoError(t, err)

		// Force a mismatch between declared exports ("foo") and init-time exports.
		// Build() declares exports based on the current builder.exports. Module init happens
		// later during import; by mutating the builder, JS_SetModuleExport will fail.
		require.GreaterOrEqual(t, len(module.exports), 1)
		module.exports[0].Name = "bar"

		result := ctx.Eval(`import('error-test-3')`, EvalAwait(true))
		defer result.Free()

		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "failed to set module export")
		assertModuleBuilderAbsent(t, ctx, "error-test-3", baseHandles)
	})

	t.Run("ModuleInitNilExportValue", func(t *testing.T) {
		_, ctx := newModuleCtx(t)
		baseHandles := ctx.handleStore.Count()

		module := NewModuleBuilder("error-test-nil-export").
			Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		require.GreaterOrEqual(t, len(module.exports), 1)
		module.exports[0].Value = nil

		result := ctx.Eval(`import('error-test-nil-export')`, EvalAwait(true))
		defer result.Free()

		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "invalid module export value")
		assertModuleBuilderAbsent(t, ctx, "error-test-nil-export", baseHandles)
	})

	t.Run("ModuleInitForeignExportValue", func(t *testing.T) {
		_, ctx := newModuleCtx(t)
		baseHandles := ctx.handleStore.Count()

		module := NewModuleBuilder("error-test-foreign-export").
			Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		rt2 := NewRuntime()
		defer rt2.Close()
		ctx2 := rt2.NewContext()
		defer ctx2.Close()

		require.GreaterOrEqual(t, len(module.exports), 1)
		module.exports[0].Value = ctx2.NewString("foreign")

		result := ctx.Eval(`import('error-test-foreign-export')`, EvalAwait(true))
		defer result.Free()

		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "invalid module export value")
		assertModuleBuilderAbsent(t, ctx, "error-test-foreign-export", baseHandles)
	})

	t.Run("ModuleInitForcedInvalidPrivateValueHook", func(t *testing.T) {
		_, ctx := newModuleCtx(t)
		baseHandles := ctx.handleStore.Count()

		setForceInvalidModulePrivateValueForTest(true)
		t.Cleanup(func() {
			setForceInvalidModulePrivateValueForTest(false)
		})

		module := NewModuleBuilder("error-test-forced-private").
			Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		result := ctx.Eval(`import('error-test-forced-private')`, EvalAwait(true))
		defer result.Free()

		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "invalid module private value")
		assertModuleBuilderRetainedThenCleaned(t, ctx, "error-test-forced-private", 1, baseHandles)
	})

	t.Run("ModuleInitForcedInvalidBuilderTypeHook", func(t *testing.T) {
		_, ctx := newModuleCtx(t)
		baseHandles := ctx.handleStore.Count()

		setForceInvalidModuleBuilderTypeForTest(true)
		t.Cleanup(func() {
			setForceInvalidModuleBuilderTypeForTest(false)
		})

		module := NewModuleBuilder("error-test-forced-builder-type").
			Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		result := ctx.Eval(`import('error-test-forced-builder-type')`, EvalAwait(true))
		defer result.Free()

		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "invalid module builder")
		assertModuleBuilderAbsent(t, ctx, "error-test-forced-builder-type", baseHandles)
	})
}

func TestModuleBuilder_InvalidHandleIDContracts(t *testing.T) {
	rt := NewRuntime(WithModuleImport(true))
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	obj, perr := loadObjectByHandleID(ctx, 0, errFunctionNotFound)
	require.Nil(t, obj)
	require.NotNil(t, perr)
	require.Equal(t, errFunctionNotFound.message, perr.message)

	obj, perr = loadObjectByHandleID(ctx, -1, errFunctionNotFound)
	require.Nil(t, obj)
	require.NotNil(t, perr)
	require.Equal(t, errFunctionNotFound.message, perr.message)

	originalStore := ctx.handleStore
	ctx.handleStore = nil
	defer func() { ctx.handleStore = originalStore }()

	obj, perr = loadObjectByHandleID(ctx, 0, errFunctionNotFound)
	require.Nil(t, obj)
	require.NotNil(t, perr)
	require.Equal(t, errHandleStoreUnavailable.message, perr.message)

	obj, perr = loadObjectByHandleID(ctx, -1, errFunctionNotFound)
	require.Nil(t, obj)
	require.NotNil(t, perr)
	require.Equal(t, errHandleStoreUnavailable.message, perr.message)
}

func TestModuleBuilder_CreateModuleHookFailureWithoutException(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseHandles := ctx.handleStore.Count()
	module := NewModuleBuilder("hook-fail-no-exception").
		Export("value", ctx.NewString("ok"))

	setCreateModuleResultHookForTest(func(_ *Context, _ *ModuleBuilder) (int, bool) {
		return -1, true
	})
	t.Cleanup(func() {
		setCreateModuleResultHookForTest(nil)
	})

	err := module.Build(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create module")
	require.Equal(t, baseHandles, ctx.handleStore.Count())
}

func TestModuleBuilder_CreateModuleHookPassthrough(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	module := NewModuleBuilder("hook-passthrough-module").
		Export("value", ctx.NewString("ok"))

	setCreateModuleResultHookForTest(func(_ *Context, _ *ModuleBuilder) (int, bool) {
		return 0, false
	})
	t.Cleanup(func() {
		setCreateModuleResultHookForTest(nil)
	})

	require.NoError(t, module.Build(ctx))

	result := ctx.Eval(`import('hook-passthrough-module').then(m => m.value)`, EvalAwait(true))
	defer result.Free()
	require.False(t, result.IsException())
	require.Equal(t, "ok", result.ToString())
}

// =============================================================================
// MINIMAL REPRO TEST (ISSUE #688)
// =============================================================================

func TestModuleBuilder_RuntimeClosePanic_Minimal(t *testing.T) {
	// Regression stress test for issue #688. Historically, closing the runtime after
	// importing a native module could randomly trigger a QuickJS abort in rt.Close().
	//
	// If QuickJS triggers an assertion/abort during rt.Close(), `go test` will terminate.
	const attempts = 50
	for i := 1; i <= attempts; i++ {
		t.Run(fmt.Sprintf("attempt_%d", i), func(t *testing.T) {
			// Use an inner scope so defers run per-iteration.
			rt := NewRuntime(WithModuleImport(true))
			defer rt.Close()

			ctx := rt.NewContext()
			defer ctx.Close()

			fooFunc := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
				return ctx.NewUndefined()
			})
			defer fooFunc.Free()

			module := NewModuleBuilder("testmodule")
			module.Export("foo", fooFunc)
			require.NoError(t, module.Build(ctx))

			script := `
import * as testmodule from "testmodule";

testmodule.foo();
`

			result := ctx.Eval(script, EvalFlagStrict(true), EvalAwait(true))
			defer result.Free()
			if result.IsException() {
				panic(ctx.Exception())
			}
		})
	}
}

func TestModuleBuilder_ErrorBranchStressNoBuilderLeak(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	for i := 0; i < 100; i++ {
		moduleName := fmt.Sprintf("stress-error-test-%d", i)

		fooFunc := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewUndefined()
		})
		defer fooFunc.Free()

		module := NewModuleBuilder(moduleName).Export("foo", fooFunc)
		err := module.Build(ctx)
		require.NoError(t, err)

		// Corrupt init-time export declaration to force JS_SetModuleExport failure.
		require.GreaterOrEqual(t, len(module.exports), 1)
		module.exports[0].Name = "bar"

		result := ctx.Eval(fmt.Sprintf(`import('%s')`, moduleName), EvalAwait(true))
		require.True(t, result.IsException())
		require.Error(t, ctx.Exception())
		result.Free()

		require.Equal(t, 0, moduleBuilderHandleCount(ctx, moduleName))
	}
}

func TestModuleBuilder_ContextErrorCleanupStress(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseHandles := ctx.handleStore.Count()

	for i := 0; i < 50; i++ {
		moduleName := fmt.Sprintf("stress-context-error-%d", i)
		module := NewModuleBuilder(moduleName).Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		unregisterContext(ctx.ref)
		result := ctx.Eval(fmt.Sprintf(`import('%s')`, moduleName), EvalAwait(true))
		registerContext(ctx.ref, ctx)

		require.True(t, result.IsException())
		require.Contains(t, ctx.Exception().Error(), "Context not found")
		result.Free()

		assertModuleBuilderRetainedThenCleaned(t, ctx, moduleName, 1, baseHandles)
	}
}
