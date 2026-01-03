package quickjs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// BASIC MODULE FUNCTIONALITY TESTS
// =============================================================================

func TestModuleBuilder_Basic(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("ModuleWithExports", func(t *testing.T) {
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
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("NamedImports", func(t *testing.T) {
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
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("EmptyModule", func(t *testing.T) {
		module := NewModuleBuilder("empty")
		err := module.Build(ctx)
		require.Error(t, err)
	})

	t.Run("EmptyModuleName", func(t *testing.T) {
		module := NewModuleBuilder("")
		err := module.Build(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "module name cannot be empty")
	})

	t.Run("EmptyExportName", func(t *testing.T) {
		module := NewModuleBuilder("test").Export("", ctx.NewString("invalid"))
		err := module.Build(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "export name cannot be empty")
	})

	t.Run("DuplicateExportNames", func(t *testing.T) {
		module := NewModuleBuilder("test").
			Export("value", ctx.NewString("first")).
			Export("value", ctx.NewString("duplicate"))
		err := module.Build(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate export name: value")
	})
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

func TestModuleBuilder_Integration(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("MultipleModules", func(t *testing.T) {
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
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("ModuleInitContextError", func(t *testing.T) {
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
	})

	t.Run("ModuleInitHandleStoreError", func(t *testing.T) {
		// Create module
		module := NewModuleBuilder("error-test-2").
			Export("value", ctx.NewString("test"))
		err := module.Build(ctx)
		require.NoError(t, err)

		// Clear handle store before module initialization
		ctx.handleStore.Clear()

		// Import will fail during initialization
		result := ctx.Eval(`import('error-test-2')`, EvalAwait(true))
		defer result.Free()

		// Should get handle store error
		require.True(t, result.IsException())
		err = ctx.Exception()
		require.Contains(t, err.Error(), "Function not found")
	})
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
				fmt.Println("foo")
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
