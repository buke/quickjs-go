package quickjs

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestContextBasics(t *testing.T) {
	newCtx := func(t *testing.T, opts ...Option) *Context {
		rt := NewRuntime(opts...)
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	ctx := newCtx(t)
	require.NotNil(t, ctx.Runtime())

	// Test basic value creation
	t.Run("ValueCreation", func(t *testing.T) {
		values := []struct {
			name      string
			createVal func(*Context) *Value
			checkFunc func(*Value) bool
		}{
			{"Null", func(ctx *Context) *Value { return ctx.NewNull() }, func(v *Value) bool { return v.IsNull() }},
			{"Undefined", func(ctx *Context) *Value { return ctx.NewUndefined() }, func(v *Value) bool { return v.IsUndefined() }},
			{"Uninitialized", func(ctx *Context) *Value { return ctx.NewUninitialized() }, func(v *Value) bool { return v.IsUninitialized() }},
			{"Bool", func(ctx *Context) *Value { return ctx.NewBool(true) }, func(v *Value) bool { return v.IsBool() }},
			{"Int32", func(ctx *Context) *Value { return ctx.NewInt32(-42) }, func(v *Value) bool { return v.IsNumber() }},
			{"Int64", func(ctx *Context) *Value { return ctx.NewInt64(1234567890) }, func(v *Value) bool { return v.IsNumber() }},
			{"Uint32", func(ctx *Context) *Value { return ctx.NewUint32(42) }, func(v *Value) bool { return v.IsNumber() }},
			{"BigInt64", func(ctx *Context) *Value { return ctx.NewBigInt64(9223372036854775807) }, func(v *Value) bool { return v.IsBigInt() }},
			{"BigUint64", func(ctx *Context) *Value { return ctx.NewBigUint64(18446744073709551615) }, func(v *Value) bool { return v.IsBigInt() }},
			{"Float64", func(ctx *Context) *Value { return ctx.NewFloat64(3.14159) }, func(v *Value) bool { return v.IsNumber() }},
			{"String", func(ctx *Context) *Value { return ctx.NewString("test") }, func(v *Value) bool { return v.IsString() }},
			{"Object", func(ctx *Context) *Value { return ctx.NewObject() }, func(v *Value) bool { return v.IsObject() }},
		}

		for _, tc := range values {
			t.Run(tc.name, func(t *testing.T) {
				ctx := newCtx(t)
				val := tc.createVal(ctx)
				defer val.Free()
				require.True(t, tc.checkFunc(val))
			})
		}
	})

	// Test ArrayBuffer with different data sizes
	t.Run("ArrayBuffer", func(t *testing.T) {
		testCases := [][]byte{
			{1, 2, 3, 4, 5},
			{},
			nil,
		}

		for i, data := range testCases {
			t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
				ctx := newCtx(t)
				ab := ctx.NewArrayBuffer(data)
				defer ab.Free()
				require.True(t, ab.IsByteArray())
				require.EqualValues(t, len(data), ab.ByteLen())
			})
		}
	})

	t.Run("StringWithEmbeddedNUL", func(t *testing.T) {
		ctx := newCtx(t)
		nulString := ctx.NewString("a\x00b")
		ctx.Globals().Set("nulString", nulString)

		check := ctx.Eval(`nulString.length === 3 && nulString.charCodeAt(1) === 0 && nulString.charCodeAt(2) === 98`)
		defer check.Free()
		require.False(t, check.IsException())
		require.True(t, check.ToBool())
	})

}

func TestContextEvaluation(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("BasicEvaluation", func(t *testing.T) {
		ctx := newCtx(t)
		// Simple expression
		result := ctx.Eval(`1 + 2`)
		defer result.Free()
		require.False(t, result.IsException())
		require.EqualValues(t, 3, result.ToInt32())

		// Empty code
		result2 := ctx.Eval(``)
		defer result2.Free()
		require.False(t, result2.IsException())
	})

	t.Run("EvaluationOptions", func(t *testing.T) {
		optionTests := []struct {
			name    string
			code    string
			options []EvalOption
		}{
			{"Strict", `"use strict"; var x = 42; x`, []EvalOption{EvalFlagStrict(true), EvalFileName("test.js")}},
			{"Module", `export const x = 42;`, []EvalOption{EvalFlagModule(true)}},
			{"CompileOnly", `1 + 1`, []EvalOption{EvalFlagCompileOnly(true)}},
			{"GlobalFalse", `var globalFlagTest = "test"; globalFlagTest`, []EvalOption{EvalFlagGlobal(false)}},
			{"GlobalTrue", `var globalFlagTest2 = "test2"; globalFlagTest2`, []EvalOption{EvalFlagGlobal(true)}},
		}

		for _, tt := range optionTests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := newCtx(t)
				result := ctx.Eval(tt.code, tt.options...)
				defer result.Free()
				require.False(t, result.IsException())
			})
		}
	})

	t.Run("EvaluationErrors", func(t *testing.T) {
		ctx := newCtx(t)
		result := ctx.Eval(`invalid syntax {`)
		defer result.Free()
		require.True(t, result.IsException())

		err := ctx.Exception()
		require.Error(t, err)
	})
}

func TestContextBytecodeOperations(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("BasicCompilation", func(t *testing.T) {
		ctx := newCtx(t)
		code := `function add(a, b) { return a + b; } add(2, 3);`
		bytecode, err := ctx.Compile(code)
		require.NoError(t, err)
		require.NotEmpty(t, bytecode)

		// Execute bytecode
		result := ctx.EvalBytecode(bytecode)
		defer result.Free()
		require.False(t, result.IsException())
		require.EqualValues(t, 5, result.ToInt32())
	})

	t.Run("FileOperations", func(t *testing.T) {
		ctx := newCtx(t)
		testFile := "./test_temp.js"
		testContent := `function multiply(a, b) { return a * b; } multiply(3, 4);`
		err := os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)
		defer os.Remove(testFile)

		// EvalFile with options
		resultFromFile := ctx.EvalFile(testFile, EvalFlagStrict(true))
		defer resultFromFile.Free()
		require.False(t, resultFromFile.IsException())
		require.EqualValues(t, 12, resultFromFile.ToInt32())

		// CompileFile tests
		bytecode, err := ctx.CompileFile(testFile)
		require.NoError(t, err)
		require.NotEmpty(t, bytecode)

		bytecode2, err := ctx.CompileFile(testFile, EvalFileName("custom.js"))
		require.NoError(t, err)
		require.NotEmpty(t, bytecode2)
	})

	t.Run("ErrorCases", func(t *testing.T) {
		ctx := newCtx(t)
		errorTests := []struct {
			name string
			test func(*Context) bool
		}{
			{"EmptyBytecode", func(ctx *Context) bool {
				result := ctx.EvalBytecode([]byte{})
				defer result.Free()
				return result.IsException()
			}},
			{"InvalidBytecode", func(ctx *Context) bool {
				result := ctx.EvalBytecode([]byte{0x01, 0x02, 0x03})
				defer result.Free()
				return result.IsException()
			}},
			{"NonexistentFile", func(ctx *Context) bool {
				result := ctx.EvalFile("./nonexistent.js")
				defer result.Free()
				return result.IsException()
			}},
			{"CompileNonexistentFile", func(ctx *Context) bool {
				_, err := ctx.CompileFile("./nonexistent.js")
				return err != nil
			}},
			{"CompilationError", func(ctx *Context) bool {
				_, err := ctx.Compile(`invalid syntax {`)
				return err != nil
			}},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := newCtx(t)
				require.True(t, tt.test(ctx))
			})
		}

		// Exception during bytecode evaluation
		invalidCode := `throw new Error("test exception during evaluation");`
		invalidBytecode, err := ctx.Compile(invalidCode)
		require.NoError(t, err)

		result := ctx.EvalBytecode(invalidBytecode)
		defer result.Free()
		require.True(t, result.IsException())

		err = ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "test exception during evaluation")
	})

	t.Run("CompilationVariants", func(t *testing.T) {
		ctx := newCtx(t)
		// Test empty code compilation
		bytecode, err := ctx.Compile(``)
		require.NoError(t, err)
		require.NotEmpty(t, bytecode)

		// Test normal function compilation
		normalCode := `(function() { return 42; })`
		r := ctx.Eval(normalCode)
		defer r.Free()
		require.False(t, r.IsException())

		bytecode, err = ctx.Compile(normalCode)
		require.NoError(t, err)
		require.NotEmpty(t, bytecode)

		result := ctx.EvalBytecode(bytecode)
		defer result.Free()
		require.False(t, result.IsException())
		require.True(t, result.IsFunction())
	})
}

func TestContextModules(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime(WithModuleImport(true))
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	moduleCode := `export function add(a, b) { return a + b; }`

	t.Run("ModuleLoading", func(t *testing.T) {
		ctx := newCtx(t)
		// Basic module loading
		result := ctx.LoadModule(moduleCode, "math_module")
		defer result.Free()
		require.False(t, result.IsException())

		// Module with load_only option
		result2 := ctx.LoadModule(moduleCode, "math_module2", EvalLoadOnly(true))
		defer result2.Free()
		require.False(t, result2.IsException())
	})

	t.Run("ModuleBytecode", func(t *testing.T) {
		ctx := newCtx(t)
		bytecode, err := ctx.Compile(moduleCode, EvalFlagModule(true), EvalFlagCompileOnly(true))
		require.NoError(t, err)

		// Basic bytecode loading
		result := ctx.LoadModuleBytecode(bytecode)
		defer result.Free()
		require.False(t, result.IsException())

		// Bytecode loading with load_only flag
		result2 := ctx.LoadModuleBytecode(bytecode, EvalLoadOnly(true))
		defer result2.Free()
		require.False(t, result2.IsException())
	})

	t.Run("ModuleFiles", func(t *testing.T) {
		ctx := newCtx(t)
		moduleFile := "./test_module.js"
		moduleContent := `export const value = 42;`
		err := os.WriteFile(moduleFile, []byte(moduleContent), 0644)
		require.NoError(t, err)
		defer os.Remove(moduleFile)

		// LoadModuleFile
		moduleResult := ctx.LoadModuleFile(moduleFile, "test_module")
		defer moduleResult.Free()
		require.False(t, moduleResult.IsException())

		// CompileModule tests
		compiledModule, err := ctx.CompileModule(moduleFile, "compiled_module")
		require.NoError(t, err)
		require.NotEmpty(t, compiledModule)

		compiledModule2, err := ctx.CompileModule(moduleFile, "compiled_module2", EvalFlagStrict(true))
		require.NoError(t, err)
		require.NotEmpty(t, compiledModule2)
	})

	t.Run("ModuleErrors", func(t *testing.T) {
		errorTests := []struct {
			name string
			test func(*Context) bool
		}{
			{"NotModule", func(ctx *Context) bool {
				result := ctx.LoadModule(`var x = 1; x;`, "not_module")
				defer result.Free()
				return result.IsException()
			}},
			{"InvalidModule", func(ctx *Context) bool {
				result := ctx.LoadModule(`export { unclosed_brace`, "invalid_module")
				defer result.Free()
				return result.IsException()
			}},
			{"ModuleCompileError", func(ctx *Context) bool {
				// Use a dedicated runtime with a lowered memory limit to force Compile() error path.
				rt2 := NewRuntime(WithModuleImport(true))
				defer rt2.Close()
				ctx2 := rt2.NewContext()
				defer ctx2.Close()

				rt2.SetMemoryLimit(512 * 1024)
				code := "export const s = `" + strings.Repeat("a", 4*1024*1024) + "`;"
				result := ctx2.LoadModule(code, "compile_error_module")
				defer result.Free()
				return result.IsException()
			}},
			{"EmptyBytecode", func(ctx *Context) bool {
				result := ctx.LoadModuleBytecode([]byte{})
				defer result.Free()
				return result.IsException()
			}},
			{"InvalidBytecode", func(ctx *Context) bool {
				result := ctx.LoadModuleBytecode([]byte{0x01, 0x02, 0x03})
				defer result.Free()
				return result.IsException()
			}},
			{"MissingFile", func(ctx *Context) bool {
				result := ctx.LoadModuleFile("./nonexistent_file.js", "missing")
				defer result.Free()
				return result.IsException()
			}},
			{"ModuleThrowsError", func(ctx *Context) bool {
				result := ctx.LoadModule(`export default 123; throw new Error('aah')`, "mod")
				defer result.Free()
				return result.IsException()
			}},
			{"ModuleUndefinedVariable", func(ctx *Context) bool {
				result := ctx.LoadModule(`export default 123; blah`, "mod")
				defer result.Free()
				return result.IsException()
			}},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := newCtx(t)
				require.True(t, tt.test(ctx))
			})
		}
	})

}

func TestContextFunctions(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("RegularFunctions", func(t *testing.T) {
		ctx := newCtx(t)
		baseHandles := ctx.handleStore.Count()

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			if len(args) == 0 {
				return ctx.NewString("no args")
			}
			return ctx.NewString("Hello " + args[0].ToString())
		})
		defer fn.Free()
		require.Equal(t, baseHandles+1, ctx.handleStore.Count())

		// Test function execution
		result := fn.Execute(ctx.NewNull())
		defer result.Free()
		require.EqualValues(t, "no args", result.ToString())

		result2 := fn.Execute(ctx.NewNull(), ctx.NewString("World"))
		defer result2.Free()
		require.EqualValues(t, "Hello World", result2.ToString())

		// Test Invoke method with different argument counts
		result3 := ctx.Invoke(fn, ctx.NewNull())
		defer result3.Free()
		require.EqualValues(t, "no args", result3.ToString())

		result4 := ctx.Invoke(fn, ctx.NewNull(), ctx.NewString("Test"))
		defer result4.Free()
		require.EqualValues(t, "Hello Test", result4.ToString())
	})

	t.Run("ReleaseFunction", func(t *testing.T) {
		ctx := newCtx(t)
		baseHandles := ctx.handleStore.Count()

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewString("released")
		})
		defer fn.Free()

		require.Equal(t, baseHandles+1, ctx.handleStore.Count())
		require.True(t, ctx.ReleaseFunction(fn))
		require.Equal(t, baseHandles, ctx.handleStore.Count())

		// Releasing again should be no-op.
		require.False(t, ctx.ReleaseFunction(fn))

		result := fn.Execute(ctx.NewNull())
		defer result.Free()
		require.True(t, result.IsException())
		require.Contains(t, ctx.Exception().Error(), "Function not found")
	})

	// Updated: Use Function + Promise instead of AsyncFunction
	t.Run("AsyncFunctions", func(t *testing.T) {
		ctx := newCtx(t)
		// New approach using Function + Promise
		asyncFn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewPromise(func(resolve, reject func(*Value)) {
				resolve(ctx.NewString("async result"))
			})
		})

		ctx.Globals().Set("testAsync", asyncFn)
		result := ctx.Eval(`testAsync()`, EvalAwait(true))
		defer result.Free()
		require.False(t, result.IsException())
		require.EqualValues(t, "async result", result.ToString())
	})
}

func TestNewAsyncFunctionTemporaryCallbacksAutoRelease(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	featureCheck := ctx.Eval(`typeof FinalizationRegistry !== "undefined"`)
	require.False(t, featureCheck.IsException())
	if !featureCheck.Bool() {
		featureCheck.Free()
		t.Skip("FinalizationRegistry is not available")
	}
	featureCheck.Free()

	baseHandles := ctx.handleStore.Count()

	asyncFn := ctx.NewAsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
		resolve := promise.Get("resolve")
		defer resolve.Free()

		payload := ctx.NewString("auto-release-ok")
		defer payload.Free()

		ret := resolve.Execute(ctx.NewUndefined(), payload)
		defer ret.Free()
		return ctx.NewUndefined()
	})
	defer asyncFn.Free()

	promise := asyncFn.Execute(ctx.NewUndefined())
	defer promise.Free()
	require.True(t, promise.IsPromise())

	result := ctx.Await(promise)
	defer result.Free()
	require.False(t, result.IsException())
	require.Equal(t, "auto-release-ok", result.ToString())

	// asyncFn + registry cleanup callback can stay alive; resolve/reject temp callbacks should be reclaimed.
	stableUpperBound := baseHandles + 2
	for i := 0; i < 50 && ctx.handleStore.Count() > stableUpperBound; i++ {
		junk := ctx.Eval(`(() => { let x = []; for (let i = 0; i < 2000; i++) x.push({i}); return x.length; })()`)
		if junk != nil {
			junk.Free()
		}
		rt.RunGC()
		ctx.Loop()
	}

	require.LessOrEqual(t, ctx.handleStore.Count(), stableUpperBound)
}

func TestNewPromiseTemporaryCallbacksAutoRelease(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	featureCheck := ctx.Eval(`typeof FinalizationRegistry !== "undefined"`)
	require.False(t, featureCheck.IsException())
	if !featureCheck.Bool() {
		featureCheck.Free()
		t.Skip("FinalizationRegistry is not available")
	}
	featureCheck.Free()

	baseHandles := ctx.handleStore.Count()

	promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
		resolve(ctx.NewString("promise-auto-release-ok"))
	})
	defer promise.Free()

	result := ctx.Await(promise)
	defer result.Free()
	require.False(t, result.IsException())
	require.Equal(t, "promise-auto-release-ok", result.ToString())

	// FinalizationRegistry cleanup callback may stay alive; temp cleanup function should not keep accumulating.
	stableUpperBound := baseHandles + 1
	for i := 0; i < 50 && ctx.handleStore.Count() > stableUpperBound; i++ {
		junk := ctx.Eval(`(() => { let x = []; for (let i = 0; i < 2000; i++) x.push({i}); return x.length; })()`)
		if junk != nil {
			junk.Free()
		}
		rt.RunGC()
		ctx.Loop()
	}

	require.LessOrEqual(t, ctx.handleStore.Count(), stableUpperBound)
}

func TestNewPromiseTemporaryCallbacksAutoReleaseWithoutFinalizationRegistry(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	disableFR := ctx.Eval(`delete globalThis.FinalizationRegistry; typeof FinalizationRegistry`)
	require.False(t, disableFR.IsException())
	require.Equal(t, "undefined", disableFR.ToString())
	disableFR.Free()

	baseHandles := ctx.handleStore.Count()

	for i := 0; i < 200; i++ {
		promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
			resolve(ctx.NewInt32(int32(i)))
		})
		result := ctx.Await(promise)
		require.False(t, result.IsException())
		result.Free()
		promise.Free()
	}

	// Without FinalizationRegistry, cleanup callback should still self-release after settlement.
	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles)
}

func TestEnsureAutoReleaseFinalizerRegistryIdempotentInit(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	featureCheck := ctx.Eval(`typeof FinalizationRegistry !== "undefined"`)
	require.False(t, featureCheck.IsException())
	if !featureCheck.Bool() {
		featureCheck.Free()
		t.Skip("FinalizationRegistry is not available")
	}
	featureCheck.Free()

	baseHandles := ctx.handleStore.Count()
	registry1 := ctx.ensureAutoReleaseFinalizerRegistry()
	registry2 := ctx.ensureAutoReleaseFinalizerRegistry()
	registry3 := ctx.ensureAutoReleaseFinalizerRegistry()

	require.NotNil(t, registry1)
	require.NotNil(t, registry2)
	require.NotNil(t, registry3)
	require.Equal(t, registry1.ref, registry2.ref)
	require.Equal(t, registry2.ref, registry3.ref)
	// Only the finalizer cleanup callback should remain registered in handleStore.
	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles+1)
}

func TestEnsureAutoReleaseFinalizerRegistryRetryAfterInitialFailure(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	backup := ctx.Eval(`
        (() => {
            globalThis.__fr_backup_for_retry_test = globalThis.FinalizationRegistry;
            delete globalThis.FinalizationRegistry;
            return typeof FinalizationRegistry;
        })()
    `)
	require.False(t, backup.IsException())
	require.Equal(t, "undefined", backup.ToString())
	backup.Free()

	registry1 := ctx.ensureAutoReleaseFinalizerRegistry()
	require.Nil(t, registry1)

	restore := ctx.Eval(`
        (() => {
            globalThis.FinalizationRegistry = globalThis.__fr_backup_for_retry_test;
            delete globalThis.__fr_backup_for_retry_test;
            return typeof FinalizationRegistry;
        })()
    `)
	require.False(t, restore.IsException())
	require.Equal(t, "function", restore.ToString())
	restore.Free()

	registry2 := ctx.ensureAutoReleaseFinalizerRegistry()
	require.NotNil(t, registry2)
	registry3 := ctx.ensureAutoReleaseFinalizerRegistry()
	require.NotNil(t, registry3)
	require.Equal(t, registry2.ref, registry3.ref)
}

func TestPromiseSettlementCleanupHandleCountStableAcrossBranches(t *testing.T) {
	t.Run("CancelBranch", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		baseHandles := ctx.handleStore.Count()
		baseRefs := ctx.currentPromiseCallbackRefCount()

		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			// keep pending
		})
		require.NotNil(t, promise)
		require.Equal(t, baseRefs+2, ctx.currentPromiseCallbackRefCount())

		cancel()
		promise.Free()

		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
		require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles+1)
	})

	t.Run("FinallyBranch", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		baseHandles := ctx.handleStore.Count()
		baseRefs := ctx.currentPromiseCallbackRefCount()

		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			v := ctx.NewString("ok")
			defer v.Free()
			resolve(v)
		})
		require.NotNil(t, promise)

		result := ctx.Await(promise)
		require.NotNil(t, result)
		require.False(t, result.IsException())
		result.Free()

		cancel()
		promise.Free()

		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
		require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles+1)
	})

	t.Run("FinallyExceptionBranch", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		patch := ctx.Eval(`
            (() => {
                if (!Promise.prototype.__origFinallyForCleanupTest) {
                    Promise.prototype.__origFinallyForCleanupTest = Promise.prototype.finally;
                }
                Promise.prototype.finally = function() {
                    throw new Error("finally patched failure");
                };
                return true;
            })()
        `)
		require.False(t, patch.IsException())
		patch.Free()
		defer func() {
			restore := ctx.Eval(`
                (() => {
                    if (Promise.prototype.__origFinallyForCleanupTest) {
                        Promise.prototype.finally = Promise.prototype.__origFinallyForCleanupTest;
                        delete Promise.prototype.__origFinallyForCleanupTest;
                    }
                    return true;
                })()
            `)
			if restore != nil {
				restore.Free()
			}
		}()

		baseHandles := ctx.handleStore.Count()
		baseRefs := ctx.currentPromiseCallbackRefCount()

		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			// keep pending
		})
		require.NotNil(t, promise)
		cancel()
		promise.Free()

		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
		require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles+1)
	})
}

func TestNewPromiseWithCancelLongRandomizedNoHandleStoreGrowth(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseHandles := ctx.handleStore.Count()
	baseRefs := ctx.currentPromiseCallbackRefCount()
	rng := rand.New(rand.NewSource(20260326))

	maxHandles := baseHandles
	for i := 0; i < 800; i++ {
		var resolveLater func(*Value)
		var rejectLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			resolveLater = resolve
			rejectLater = reject
		})
		require.NotNil(t, promise)

		switch rng.Intn(6) {
		case 0:
			cancel()
		case 1:
			v := ctx.NewInt32(int32(i))
			resolveLater(v)
			v.Free()
		case 2:
			errVal := ctx.NewError(errors.New("random-reject"))
			rejectLater(errVal)
			errVal.Free()
		case 3:
			cancel()
			v := ctx.NewString("late-resolve")
			resolveLater(v)
			v.Free()
		case 4:
			v := ctx.NewString("resolve-then-cancel")
			resolveLater(v)
			v.Free()
			cancel()
		default:
			errVal := ctx.NewError(errors.New("reject-then-cancel"))
			rejectLater(errVal)
			errVal.Free()
			cancel()
		}

		ctx.ProcessJobs()
		if promise.PromiseState() != PromisePending {
			result := ctx.Await(promise)
			if result != nil {
				result.Free()
			}
		}

		cancel()
		promise.Free()

		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
		if current := ctx.handleStore.Count(); current > maxHandles {
			maxHandles = current
		}

		if i%80 == 0 {
			rt.RunGC()
			ctx.Loop()
		}
	}

	for i := 0; i < 20 && ctx.handleStore.Count() > baseHandles+1; i++ {
		rt.RunGC()
		ctx.Loop()
	}

	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles+1)
	// Allow transient spikes during randomized interleaving, but ensure no runaway growth.
	require.LessOrEqual(t, maxHandles, baseHandles+24)
}

func TestNewPromiseWithCancelGoroutineCancelSettleRaceOwnerThreadSafe(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	baseHandles := ctx.handleStore.Count()

	var resolveLater func(*Value)
	promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
		resolveLater = resolve
	})
	require.NotNil(t, promise)

	panicCh := make(chan interface{}, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer func() { panicCh <- recover() }()
		cancel()
	}()

	go func() {
		defer wg.Done()
		defer func() { panicCh <- recover() }()
		resolveLater(nil)
	}()

	wg.Wait()
	close(panicCh)
	for rec := range panicCh {
		require.Nil(t, rec)
	}

	ctx.ProcessJobs()
	if promise.PromiseState() != PromisePending {
		result := ctx.Await(promise)
		if result != nil {
			result.Free()
		}
	}

	cancel()
	promise.Free()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles+1)
}

func TestNewPromiseWithCancelLongStressGoroutineCancelNoRunawayGrowth(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	baseHandles := ctx.handleStore.Count()
	rng := rand.New(rand.NewSource(2026032602))
	maxHandles := baseHandles

	for i := 0; i < 1200; i++ {
		var resolveLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			resolveLater = resolve
		})
		require.NotNil(t, promise)

		panicCh := make(chan interface{}, 2)
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			defer func() { panicCh <- recover() }()
			cancel()
		}()

		go func(mode int) {
			defer wg.Done()
			defer func() { panicCh <- recover() }()
			switch mode {
			case 0:
				resolveLater(nil)
			case 1:
				ctx.Schedule(func(inner *Context) {
					v := inner.NewInt32(int32(i))
					defer v.Free()
					resolveLater(v)
				})
			default:
				// no settle
			}
		}(rng.Intn(3))

		wg.Wait()
		close(panicCh)
		for rec := range panicCh {
			require.Nil(t, rec)
		}

		ctx.ProcessJobs()
		if promise.PromiseState() != PromisePending {
			result := ctx.Await(promise)
			if result != nil {
				result.Free()
			}
		}

		cancel()
		promise.Free()
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
		if current := ctx.handleStore.Count(); current > maxHandles {
			maxHandles = current
		}

		if i%120 == 0 {
			rt.RunGC()
			ctx.Loop()
		}
	}

	for i := 0; i < 20 && ctx.handleStore.Count() > baseHandles+1; i++ {
		rt.RunGC()
		ctx.Loop()
	}

	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles+1)
	require.LessOrEqual(t, maxHandles, baseHandles+28)
}

func TestContextCloseInterleavingWithConcurrentCancelResolveStress(t *testing.T) {
	rng := rand.New(rand.NewSource(2026032603))

	for i := 0; i < 320; i++ {
		rt := NewRuntime()
		ctx := rt.NewContext()
		require.NotNil(t, ctx)

		var resolveLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			resolveLater = resolve
		})
		require.NotNil(t, promise)

		start := make(chan struct{})
		panicCh := make(chan interface{}, 2)
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			defer func() { panicCh <- recover() }()
			<-start
			for j := 0; j < 6; j++ {
				_ = ctx.Schedule(func(inner *Context) {
					cancel()
				})
			}
		}()

		go func(mode int) {
			defer wg.Done()
			defer func() { panicCh <- recover() }()
			<-start
			for j := 0; j < 6; j++ {
				switch mode {
				case 0:
					_ = ctx.Schedule(func(inner *Context) {
						resolveLater(nil)
					})
				case 1:
					_ = ctx.Schedule(func(inner *Context) {
						v := inner.NewInt32(int32(i*10 + j))
						defer v.Free()
						resolveLater(v)
					})
				default:
					_ = ctx.Schedule(func(inner *Context) {
						// leave pending in this branch
					})
				}
			}
		}(rng.Intn(3))

		close(start)
		wg.Wait()
		close(panicCh)
		for rec := range panicCh {
			require.Nil(t, rec)
		}
		// Release local Promise handle before context teardown.
		promise.Free()
		if i%2 == 0 {
			ctx.Close()
		} else {
			ctx.ProcessJobs()
			ctx.Close()
		}
		rt.Close()
	}
}

func TestPromiseCleanupObservabilityCounters(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		featureCheck := ctx.Eval(`typeof FinalizationRegistry !== "undefined"`)
		require.False(t, featureCheck.IsException())
		if !featureCheck.Bool() {
			featureCheck.Free()
			ctx.Close()
			rt.Close()
			t.Skip("FinalizationRegistry is not available")
		}
		featureCheck.Free()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("CancelTriggered", func(t *testing.T) {
		ctx := newCtx(t)
		ctx.SnapshotAndResetPromiseCleanupObservability()

		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			// keep pending
		})
		require.NotNil(t, promise)
		cancel()
		promise.Free()

		snap := ctx.SnapshotAndResetPromiseCleanupObservability()
		require.GreaterOrEqual(t, snap.CancelTriggered, uint64(1))
		require.Equal(t, uint64(0), snap.FinallyTriggered)
		require.Equal(t, uint64(0), snap.FallbackTriggered)
	})

	t.Run("FinallyTriggered", func(t *testing.T) {
		ctx := newCtx(t)
		ctx.SnapshotAndResetPromiseCleanupObservability()

		promise, _ := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			v := ctx.NewString("ok")
			defer v.Free()
			resolve(v)
		})
		require.NotNil(t, promise)

		result := ctx.Await(promise)
		require.NotNil(t, result)
		result.Free()
		promise.Free()

		snap := ctx.SnapshotPromiseCleanupObservability()
		for i := 0; i < 10 && snap.FinallyTriggered == 0; i++ {
			ctx.ProcessJobs()
			ctx.Loop()
			time.Sleep(time.Millisecond)
			snap = ctx.SnapshotPromiseCleanupObservability()
		}
		snap = ctx.SnapshotAndResetPromiseCleanupObservability()
		require.Equal(t, uint64(0), snap.CancelTriggered)
		require.GreaterOrEqual(t, snap.FinallyTriggered, uint64(1))
		require.Equal(t, uint64(0), snap.FallbackTriggered)
	})

	t.Run("FallbackTriggered", func(t *testing.T) {
		ctx := newCtx(t)
		ctx.SnapshotAndResetPromiseCleanupObservability()

		patch := ctx.Eval(`
            (() => {
                if (!Promise.prototype.__origFinallyForObsTest) {
                    Promise.prototype.__origFinallyForObsTest = Promise.prototype.finally;
                }
                Promise.prototype.finally = function() {
                    throw new Error("finally patched failure");
                };
                return true;
            })()
        `)
		require.False(t, patch.IsException())
		patch.Free()
		defer func() {
			restore := ctx.Eval(`
                (() => {
                    if (Promise.prototype.__origFinallyForObsTest) {
                        Promise.prototype.finally = Promise.prototype.__origFinallyForObsTest;
                        delete Promise.prototype.__origFinallyForObsTest;
                    }
                    return true;
                })()
            `)
			if restore != nil {
				restore.Free()
			}
		}()

		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			// keep pending
		})
		require.NotNil(t, promise)
		cancel()
		promise.Free()

		snap := ctx.SnapshotAndResetPromiseCleanupObservability()
		require.Equal(t, uint64(0), snap.CancelTriggered)
		require.Equal(t, uint64(0), snap.FinallyTriggered)
		require.GreaterOrEqual(t, snap.FallbackTriggered, uint64(1))
	})
}

func TestCloseHighQueuePressureEnqueueDropCounterStable(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()

	ctx.SnapshotAndResetCloseEnqueueObservability()

	// Saturate queue first to force fallback enqueue drops.
	for i := 0; i < defaultJobQueueSize; i++ {
		require.True(t, ctx.enqueueJobDuringClose(func(*Context) {}))
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	workers := 64
	attemptsPerWorker := 48
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < attemptsPerWorker; j++ {
				_ = ctx.enqueueJobDuringClose(func(*Context) {})
			}
		}()
	}

	close(start)
	time.Sleep(2 * time.Millisecond)
	ctx.Close()
	wg.Wait()

	snap := ctx.SnapshotCloseEnqueueObservability()
	require.Greater(t, snap.Dropped, uint64(0))
	require.LessOrEqual(t, snap.Dropped, uint64(workers*attemptsPerWorker))
	require.Greater(t, snap.OtherDropped, uint64(0))
	require.Equal(t, snap.Dropped, snap.ValueFreeDropped+snap.PromiseCallbackDropped+snap.OtherDropped)

	stable := ctx.SnapshotCloseEnqueueObservability()
	time.Sleep(2 * time.Millisecond)
	idle := ctx.SnapshotCloseEnqueueObservability()
	require.Equal(t, stable, idle)

	ctx.Close()
}

func TestCloseQueueOverflowJobsDrainedDuringClose(t *testing.T) {
	rt := NewRuntime()
	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	for i := 0; i < defaultJobQueueSize; i++ {
		require.True(t, ctx.enqueueJobDuringClose(func(*Context) {}))
	}

	ran := 0
	require.False(t, ctx.enqueueJobDuringClose(func(*Context) { ran++ }))

	ctx.Close()
	require.Equal(t, 1, ran)

	rt.Close()
}

func TestCloseQueueSustainedPressureCloseRecreateExtremeGate(t *testing.T) {
	const rounds = 2000
	const warmupQueueFill = defaultJobQueueSize

	var totalDropped uint64
	var maxDroppedPerRound uint64
	var firstWindowDropped uint64
	var lastWindowDropped uint64

	for i := 0; i < rounds; i++ {
		rt := NewRuntime()
		ctx := rt.NewContext()
		require.NotNil(t, ctx)

		baseRefs := ctx.currentPromiseCallbackRefCount()
		baseHandles := ctx.handleStore.Count()
		ctx.SnapshotAndResetCloseEnqueueObservability()

		for j := 0; j < warmupQueueFill; j++ {
			_ = ctx.enqueueJobDuringClose(func(*Context) {})
		}

		// Use an immediate value here so dropped close-window free jobs do not
		// leave GC-tracked heap objects behind and cause runtime close asserts.
		val := ctx.NewInt32(1)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			// keep pending; cancel path should release callback refs
		})
		require.NotNil(t, promise)
		promise.Free()

		start := make(chan struct{})
		panicCh := make(chan interface{}, 3)
		var wg sync.WaitGroup
		wg.Add(3)

		go func() {
			defer wg.Done()
			defer func() { panicCh <- recover() }()
			<-start
			val.Free()
		}()

		go func() {
			defer wg.Done()
			defer func() { panicCh <- recover() }()
			<-start
			cancel()
		}()

		go func() {
			defer wg.Done()
			defer func() { panicCh <- recover() }()
			<-start
			for k := 0; k < 16; k++ {
				_ = ctx.enqueueJobDuringClose(func(*Context) {})
			}
		}()

		close(start)
		time.Sleep(100 * time.Microsecond)
		ctx.Close()
		wg.Wait()
		close(panicCh)
		for rec := range panicCh {
			require.Nil(t, rec)
		}

		snap := ctx.SnapshotCloseEnqueueObservability()
		totalDropped += snap.Dropped
		if snap.Dropped > maxDroppedPerRound {
			maxDroppedPerRound = snap.Dropped
		}
		if i < 200 {
			firstWindowDropped += snap.Dropped
		}
		if i >= rounds-200 {
			lastWindowDropped += snap.Dropped
		}

		require.Equal(t, snap.Dropped, snap.ValueFreeDropped+snap.PromiseCallbackDropped+snap.OtherDropped)
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
		require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles)

		rt.Close()
	}

	// Bound worst-case burst and check trend does not diverge across recreate rounds.
	require.LessOrEqual(t, maxDroppedPerRound, uint64(defaultJobQueueSize+96))
	if totalDropped > 0 {
		require.LessOrEqual(t, lastWindowDropped, firstWindowDropped+uint64(200*64))
	}
}

func TestPromiseCleanupObservabilitySoakGate(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	featureCheck := ctx.Eval(`typeof FinalizationRegistry !== "undefined"`)
	require.False(t, featureCheck.IsException())
	if !featureCheck.Bool() {
		featureCheck.Free()
		t.Skip("FinalizationRegistry is not available")
	}
	featureCheck.Free()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	baseHandles := ctx.handleStore.Count()
	ctx.SnapshotAndResetPromiseCleanupObservability()

	const total = 6000
	for i := 0; i < total; i++ {
		switch i % 3 {
		case 0:
			promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
				// pending -> cancel branch
			})
			require.NotNil(t, promise)
			cancel()
			promise.Free()
		case 1:
			promise, _ := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
				v := ctx.NewInt32(int32(i))
				defer v.Free()
				resolve(v)
			})
			require.NotNil(t, promise)
			result := ctx.Await(promise)
			if result != nil {
				result.Free()
			}
			promise.Free()
		default:
			patch := ctx.Eval(`
                (() => {
                    if (!Promise.prototype.__origFinallyForSoakGate) {
                        Promise.prototype.__origFinallyForSoakGate = Promise.prototype.finally;
                    }
                    Promise.prototype.finally = function() { throw new Error("soak-finally-fallback"); };
                    return true;
                })()
            `)
			require.False(t, patch.IsException())
			patch.Free()

			promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
				// force fallback through patched finally
			})
			require.NotNil(t, promise)
			cancel()
			promise.Free()

			restore := ctx.Eval(`
                (() => {
                    if (Promise.prototype.__origFinallyForSoakGate) {
                        Promise.prototype.finally = Promise.prototype.__origFinallyForSoakGate;
                        delete Promise.prototype.__origFinallyForSoakGate;
                    }
                    return true;
                })()
            `)
			if restore != nil {
				restore.Free()
			}
		}

		if i%250 == 0 {
			ctx.ProcessJobs()
			ctx.Loop()
			rt.RunGC()
		}
	}

	snap := ctx.SnapshotPromiseCleanupObservability()
	for i := 0; i < 20 && snap.FinallyTriggered == 0; i++ {
		ctx.ProcessJobs()
		ctx.Loop()
		time.Sleep(time.Millisecond)
		snap = ctx.SnapshotPromiseCleanupObservability()
	}

	snap = ctx.SnapshotAndResetPromiseCleanupObservability()
	require.Greater(t, snap.CancelTriggered, uint64(0))
	require.Greater(t, snap.FinallyTriggered, uint64(0))
	require.Greater(t, snap.FallbackTriggered, uint64(0))
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())

	for i := 0; i < 30 && ctx.handleStore.Count() > baseHandles+2; i++ {
		rt.RunGC()
		ctx.ProcessJobs()
		ctx.Loop()
	}
	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles+2)
}

func TestValueFreeSafeWhenCalledOffOwnerThread(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	v := ctx.NewString("owner-thread-only")
	recCh := make(chan interface{}, 1)
	go func() {
		defer func() { recCh <- recover() }()
		v.Free()
	}()

	rec := <-recCh
	require.Nil(t, rec)

	ctx.ProcessJobs()

	// Free remains idempotent after deferred owner-thread cleanup.
	v.Free()
}

func TestNewPromiseWithCancelReleasesPendingCallbacksWithoutFinalizationRegistry(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	disableFR := ctx.Eval(`delete globalThis.FinalizationRegistry; typeof FinalizationRegistry`)
	require.False(t, disableFR.IsException())
	require.Equal(t, "undefined", disableFR.ToString())
	disableFR.Free()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
		// Keep pending on purpose.
	})
	defer promise.Free()

	require.Equal(t, PromisePending, promise.PromiseState())
	require.Equal(t, baseRefs+2, ctx.currentPromiseCallbackRefCount())

	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())

	// Cancel must be idempotent.
	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestNewPromiseWithCancelStressNoAccumulation(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	disableFR := ctx.Eval(`delete globalThis.FinalizationRegistry; typeof FinalizationRegistry`)
	require.False(t, disableFR.IsException())
	require.Equal(t, "undefined", disableFR.ToString())
	disableFR.Free()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	for i := 0; i < 200; i++ {
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			// keep pending
		})
		require.Equal(t, PromisePending, promise.PromiseState())
		cancel()
		promise.Free()
	}

	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestNewPromiseWithCancelExecutorPanicReleasesCallbacks(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	require.PanicsWithValue(t, "executor panic", func() {
		_, _ = ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			panic("executor panic")
		})
	})
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestNewPromiseWithCancelPromiseSetupException(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	executorCalled := false

	patch := ctx.Eval(`
        (() => {
            globalThis.__promise_backup_for_setup_exception_test = globalThis.Promise;
            globalThis.Promise = function() {
                throw new Error("promise-ctor-fail");
            };
            return true;
        })()
    `)
	require.False(t, patch.IsException())
	patch.Free()

	promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
		executorCalled = true
	})
	require.NotNil(t, promise)
	defer promise.Free()
	require.True(t, promise.IsException())
	require.False(t, executorCalled)
	require.NotPanics(t, cancel)
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())

	restore := ctx.Eval(`
        (() => {
            globalThis.Promise = globalThis.__promise_backup_for_setup_exception_test;
            delete globalThis.__promise_backup_for_setup_exception_test;
            return true;
        })()
    `)
	require.False(t, restore.IsException())
	restore.Free()
}

func TestNewPromiseWithCancelPostSettleCASHookPath(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()

	var resolveLater func(*Value)
	var cancel func()

	setNewPromiseWithCancelPostSettleCASHookForTest(func() {
		if cancel != nil {
			cancel()
		}
	})
	t.Cleanup(func() {
		setNewPromiseWithCancelPostSettleCASHookForTest(nil)
	})

	promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
		resolveLater = resolve
	})
	defer promise.Free()

	require.NotNil(t, resolveLater)
	resolveLater(nil)
	ctx.ProcessJobs()

	// cancel hook runs between settled CAS and callback dispatch;
	// Promise remains pending while callbacks are released safely.
	require.Equal(t, PromisePending, promise.PromiseState())
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())

	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestNewPromiseWithCancelScheduledCancelSettleRace(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	start := make(chan struct{})
	resolveScheduled := make(chan bool, 1)
	cancelScheduled := make(chan bool, 1)

	var resolveLater func(*Value)
	var cancel func()
	promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
		resolveLater = resolve

		go func() {
			<-start
			ok := ctx.Schedule(func(inner *Context) {
				val := inner.NewString("resolve-before-cancel")
				defer val.Free()
				resolveLater(val)
			})
			resolveScheduled <- ok
		}()

		go func() {
			<-start
			ok := ctx.Schedule(func(inner *Context) {
				cancel()
			})
			cancelScheduled <- ok
		}()
	})
	defer promise.Free()

	require.NotNil(t, resolveLater)
	close(start)

	select {
	case ok := <-resolveScheduled:
		require.True(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled resolve job was not enqueued")
	}

	select {
	case ok := <-cancelScheduled:
		require.True(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled cancel job was not enqueued")
	}

	ctx.ProcessJobs()
	state := promise.PromiseState()
	require.True(t, state == PromisePending || state == PromiseFulfilled)
	if state == PromiseFulfilled {
		result := ctx.Await(promise)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "resolve-before-cancel", result.ToString())
	}

	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestNewPromiseWithCancelNoopAfterSettled(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
		resolve(ctx.NewString("settled"))
	})
	defer promise.Free()

	result := ctx.Await(promise)
	defer result.Free()
	require.False(t, result.IsException())
	require.Equal(t, "settled", result.ToString())
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())

	// cancel after settled must be a no-op and idempotent.
	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestNewPromiseWithCancelNoopAfterRejected(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
		errVal := ctx.NewError(errors.New("rejected"))
		defer errVal.Free()
		reject(errVal)
	})
	defer promise.Free()

	result := ctx.Await(promise)
	defer result.Free()
	require.True(t, result.IsException())
	require.Contains(t, ctx.Exception().Error(), "rejected")
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())

	// cancel after rejected must be a no-op and idempotent.
	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestNewPromiseWithCancelDoesNotSettlePromise(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	var resolveLater func(*Value)
	promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
		resolveLater = resolve
	})
	defer promise.Free()

	require.NotNil(t, resolveLater)
	require.Equal(t, PromisePending, promise.PromiseState())

	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())

	val := ctx.NewString("should-not-settle")
	defer val.Free()
	resolveLater(val)
	ctx.ProcessJobs()

	// cancel only releases callback references; it must not force settlement.
	require.Equal(t, PromisePending, promise.PromiseState())
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestNewPromiseWithCancelInterleaving(t *testing.T) {
	t.Run("CancelThenResolve", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		baseRefs := ctx.currentPromiseCallbackRefCount()
		var resolveLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			resolveLater = resolve
		})
		defer promise.Free()

		require.NotNil(t, resolveLater)
		cancel()
		resolveLater(nil)
		ctx.ProcessJobs()

		require.Equal(t, PromisePending, promise.PromiseState())
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	})

	t.Run("CancelThenReject", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		baseRefs := ctx.currentPromiseCallbackRefCount()
		var rejectLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			rejectLater = reject
		})
		defer promise.Free()

		require.NotNil(t, rejectLater)
		cancel()
		errVal := ctx.NewError(errors.New("should-not-reject"))
		defer errVal.Free()
		rejectLater(errVal)
		ctx.ProcessJobs()

		require.Equal(t, PromisePending, promise.PromiseState())
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	})

	t.Run("ResolveThenCancel", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		baseRefs := ctx.currentPromiseCallbackRefCount()
		var resolveLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			resolveLater = resolve
		})
		defer promise.Free()

		require.NotNil(t, resolveLater)
		resolveLater(nil)
		result := ctx.Await(promise)
		defer result.Free()
		require.False(t, result.IsException())
		require.True(t, result.IsUndefined())

		cancel()
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	})

	t.Run("RejectThenCancel", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		baseRefs := ctx.currentPromiseCallbackRefCount()
		var rejectLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			rejectLater = reject
		})
		defer promise.Free()

		require.NotNil(t, rejectLater)
		errVal := ctx.NewError(errors.New("reject-first"))
		defer errVal.Free()
		rejectLater(errVal)

		result := ctx.Await(promise)
		defer result.Free()
		require.True(t, result.IsException())
		require.Contains(t, ctx.Exception().Error(), "reject-first")

		cancel()
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	})
}

func TestNewPromiseWithCancelRandomizedOrdering(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	rng := rand.New(rand.NewSource(42))

	permutations := [][]string{
		{"cancel", "resolve", "reject"},
		{"cancel", "reject", "resolve"},
		{"resolve", "cancel", "reject"},
		{"resolve", "reject", "cancel"},
		{"reject", "cancel", "resolve"},
		{"reject", "resolve", "cancel"},
	}

	for i := 0; i < 200; i++ {
		var resolveLater func(*Value)
		var rejectLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			resolveLater = resolve
			rejectLater = reject
		})

		require.NotNil(t, resolveLater)
		require.NotNil(t, rejectLater)

		order := permutations[rng.Intn(len(permutations))]
		for _, op := range order {
			switch op {
			case "cancel":
				cancel()
			case "resolve":
				val := ctx.NewString("resolved")
				resolveLater(val)
				val.Free()
			case "reject":
				errVal := ctx.NewError(errors.New("rejected"))
				rejectLater(errVal)
				errVal.Free()
			}
		}

		ctx.ProcessJobs()

		switch order[0] {
		case "cancel":
			require.Equal(t, PromisePending, promise.PromiseState())
		case "resolve":
			require.Equal(t, PromiseFulfilled, promise.PromiseState())
		case "reject":
			require.Equal(t, PromiseRejected, promise.PromiseState())
		}

		promise.Free()
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	}
}

func TestNewPromiseWithCancelScheduledInterleaving(t *testing.T) {
	t.Run("CancelThenScheduledResolve", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		baseRefs := ctx.currentPromiseCallbackRefCount()
		trigger := make(chan struct{})
		scheduled := make(chan bool, 1)

		var resolveLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			resolveLater = resolve
			go func() {
				<-trigger
				ok := ctx.Schedule(func(inner *Context) {
					val := inner.NewString("scheduled-resolve")
					defer val.Free()
					resolveLater(val)
				})
				scheduled <- ok
			}()
		})
		defer promise.Free()

		require.NotNil(t, resolveLater)
		cancel()
		close(trigger)

		select {
		case ok := <-scheduled:
			require.True(t, ok)
		case <-time.After(2 * time.Second):
			t.Fatal("scheduled resolve job was not enqueued")
		}

		ctx.ProcessJobs()
		require.Equal(t, PromisePending, promise.PromiseState())
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	})

	t.Run("ScheduledRejectThenCancel", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		baseRefs := ctx.currentPromiseCallbackRefCount()
		scheduled := make(chan bool, 1)

		var rejectLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			rejectLater = reject
			go func() {
				ok := ctx.Schedule(func(inner *Context) {
					errVal := inner.NewError(errors.New("scheduled-reject"))
					defer errVal.Free()
					rejectLater(errVal)
				})
				scheduled <- ok
			}()
		})
		defer promise.Free()

		require.NotNil(t, rejectLater)
		select {
		case ok := <-scheduled:
			require.True(t, ok)
		case <-time.After(2 * time.Second):
			t.Fatal("scheduled reject job was not enqueued")
		}

		result := ctx.Await(promise)
		defer result.Free()
		require.True(t, result.IsException())
		require.Contains(t, ctx.Exception().Error(), "scheduled-reject")

		cancel()
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	})
}

func TestNewPromiseWithCancelScheduledSettleRace(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	start := make(chan struct{})
	resolveScheduled := make(chan bool, 1)
	rejectScheduled := make(chan bool, 1)

	var resolveLater func(*Value)
	var rejectLater func(*Value)
	promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
		resolveLater = resolve
		rejectLater = reject

		go func() {
			<-start
			ok := ctx.Schedule(func(inner *Context) {
				val := inner.NewString("race-resolve")
				defer val.Free()
				resolveLater(val)
			})
			resolveScheduled <- ok
		}()

		go func() {
			<-start
			ok := ctx.Schedule(func(inner *Context) {
				errVal := inner.NewError(errors.New("race-reject"))
				defer errVal.Free()
				rejectLater(errVal)
			})
			rejectScheduled <- ok
		}()
	})
	defer promise.Free()

	require.NotNil(t, resolveLater)
	require.NotNil(t, rejectLater)
	close(start)

	select {
	case ok := <-resolveScheduled:
		require.True(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled resolve job was not enqueued")
	}

	select {
	case ok := <-rejectScheduled:
		require.True(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled reject job was not enqueued")
	}

	result := ctx.Await(promise)
	defer result.Free()
	if result.IsException() {
		require.Contains(t, ctx.Exception().Error(), "race-reject")
	} else {
		require.Equal(t, "race-resolve", result.ToString())
	}

	cancel()
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestNewPromiseWithCancelScheduledSettleRaceStress(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	rng := rand.New(rand.NewSource(7))

	for i := 0; i < 50; i++ {
		start := make(chan struct{})
		resolveScheduled := make(chan bool, 1)
		rejectScheduled := make(chan bool, 1)

		var resolveLater func(*Value)
		var rejectLater func(*Value)
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			resolveLater = resolve
			rejectLater = reject

			launch := func(f func()) {
				go func() {
					<-start
					f()
				}()
			}

			launch(func() {
				ok := ctx.Schedule(func(inner *Context) {
					val := inner.NewString("race-resolve")
					defer val.Free()
					resolveLater(val)
				})
				resolveScheduled <- ok
			})

			launch(func() {
				ok := ctx.Schedule(func(inner *Context) {
					errVal := inner.NewError(errors.New("race-reject"))
					defer errVal.Free()
					rejectLater(errVal)
				})
				rejectScheduled <- ok
			})
		})

		require.NotNil(t, resolveLater)
		require.NotNil(t, rejectLater)

		// Randomly introduce cancel into the race in roughly half the rounds.
		if rng.Intn(2) == 0 {
			cancel()
		}

		close(start)

		select {
		case ok := <-resolveScheduled:
			require.True(t, ok)
		case <-time.After(2 * time.Second):
			t.Fatal("scheduled resolve job was not enqueued")
		}

		select {
		case ok := <-rejectScheduled:
			require.True(t, ok)
		case <-time.After(2 * time.Second):
			t.Fatal("scheduled reject job was not enqueued")
		}

		if promise.PromiseState() == PromisePending {
			ctx.ProcessJobs()
		}

		if promise.PromiseState() != PromisePending {
			result := ctx.Await(promise)
			result.Free()
		}

		cancel()
		promise.Free()
		require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
	}
}

func TestContextErrorHandling(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("ErrorCreation", func(t *testing.T) {
		testErr := errors.New("test error message")
		errorVal := ctx.NewError(testErr)
		defer errorVal.Free()
		require.True(t, errorVal.IsError())

		nilErrVal := ctx.NewError(nil)
		require.NotNil(t, nilErrVal)
		defer nilErrVal.Free()
		require.True(t, nilErrVal.IsError())
		nilErrMsg := nilErrVal.Get("message")
		require.NotNil(t, nilErrMsg)
		require.Equal(t, "unknown error", nilErrMsg.ToString())
		nilErrMsg.Free()

		var nilCtx *Context
		require.Nil(t, nilCtx.NewError(errors.New("x")))
	})

	t.Run("ThrowMethods", func(t *testing.T) {
		throwTests := []struct {
			name     string
			throwFn  func() *Value // Changed to return pointer
			errorStr string
		}{
			{"ThrowError", func() *Value { return ctx.ThrowError(errors.New("custom error")) }, "custom error"},
			{"ThrowSyntax", func() *Value { return ctx.ThrowSyntaxError("syntax: %s", "invalid") }, "SyntaxError"},
			{"ThrowType", func() *Value { return ctx.ThrowTypeError("type error") }, "TypeError"},
			{"ThrowReference", func() *Value { return ctx.ThrowReferenceError("ref error") }, "ReferenceError"},
			{"ThrowRange", func() *Value { return ctx.ThrowRangeError("range error") }, "RangeError"},
			{"ThrowInternal", func() *Value { return ctx.ThrowInternalError("internal error") }, "InternalError"},
		}

		for _, tt := range throwTests {
			t.Run(tt.name, func(t *testing.T) {
				throwingFunc := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
					return tt.throwFn()
				})
				defer throwingFunc.Free()

				result := throwingFunc.Execute(ctx.NewNull())
				defer result.Free()
				require.True(t, result.IsException())
				require.True(t, ctx.HasException())

				exception := ctx.Exception()
				require.NotNil(t, exception)
				require.Contains(t, exception.Error(), tt.errorStr)
				require.False(t, ctx.HasException()) // Should be cleared
			})
		}
	})

	t.Run("ThrowOwnershipTransfer", func(t *testing.T) {
		errVal := ctx.NewError(errors.New("throw ownership transfer"))
		result := ctx.Throw(errVal)
		defer result.Free()

		require.True(t, result.IsException())
		require.True(t, errVal.IsUndefined())

		// Must be safe because Throw consumed and invalidated the source value.
		errVal.Free()

		exception := ctx.Exception()
		require.Error(t, exception)
		require.Contains(t, exception.Error(), "throw ownership transfer")
	})

	t.Run("ThrowNilAndCrossContextSafety", func(t *testing.T) {
		nilResult := ctx.Throw(nil)
		require.NotNil(t, nilResult)
		defer nilResult.Free()
		require.True(t, nilResult.IsException())
		nilErr := ctx.Exception()
		require.Error(t, nilErr)
		require.Contains(t, nilErr.Error(), "throw value cannot be nil")

		rt2 := NewRuntime()
		defer rt2.Close()
		ctx2 := rt2.NewContext()
		defer ctx2.Close()
		foreignVal := ctx2.NewString("foreign")
		defer foreignVal.Free()

		crossResult := ctx.Throw(foreignVal)
		require.NotNil(t, crossResult)
		defer crossResult.Free()
		require.True(t, crossResult.IsException())
		crossErr := ctx.Exception()
		require.Error(t, crossErr)
		require.Contains(t, crossErr.Error(), "throw value must belong to current context")
		require.False(t, foreignVal.IsUndefined())
	})

	t.Run("ThrowErrorNilSafety", func(t *testing.T) {
		result := ctx.ThrowError(nil)
		require.NotNil(t, result)
		defer result.Free()
		require.True(t, result.IsException())
		err := ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "nil error")
	})

	t.Run("ExceptionHandling", func(t *testing.T) {
		// Test Exception() when no exception
		exception := ctx.Exception()
		require.Nil(t, exception)
		require.False(t, ctx.HasException())
	})
}

func TestContextUtilities(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("Globals", func(t *testing.T) {
		ctx := newCtx(t)
		// Test globals caching
		globals1 := ctx.Globals()
		globals2 := ctx.Globals()
		require.True(t, globals1.IsObject())
		require.True(t, globals2.IsObject())

		// Test global variable operations
		globals1.Set("testGlobal", ctx.NewString("global value"))
		retrieved := globals2.Get("testGlobal")
		defer retrieved.Free()
		require.EqualValues(t, "global value", retrieved.ToString())
	})

	t.Run("JSONParsing", func(t *testing.T) {
		ctx := newCtx(t)
		// Valid JSON
		jsonObj := ctx.ParseJSON(`{"name": "test", "value": 42}`)
		defer jsonObj.Free()
		require.True(t, jsonObj.IsObject())

		nameVal := jsonObj.Get("name")
		defer nameVal.Free()
		require.EqualValues(t, "test", nameVal.ToString())

		// Invalid JSON
		invalidJSON := ctx.ParseJSON(`{invalid}`)
		defer invalidJSON.Free()
		require.True(t, invalidJSON.IsException())

		// Empty JSON input should also be a JSON parse exception.
		emptyJSON := ctx.ParseJSON("")
		defer emptyJSON.Free()
		require.True(t, emptyJSON.IsException())
	})

	t.Run("InterruptHandler", func(t *testing.T) {
		ctx := newCtx(t)
		interruptCalled := false
		ctx.SetInterruptHandler(func() int {
			interruptCalled = true
			return 1 // Interrupt
		})

		result := ctx.Eval(`while(true){}`)
		defer result.Free()
		require.True(t, result.IsException())

		err := ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "interrupted")
		require.True(t, interruptCalled)
	})
}

func TestContextNilReceiverCoverageHelpers(t *testing.T) {
	var nilCtx *Context
	emptyCtx := &Context{}

	require.Equal(t, int64(0), nilCtx.currentPromiseCallbackRefCount())
	require.Equal(t, PromiseCleanupObservabilitySnapshot{}, nilCtx.SnapshotPromiseCleanupObservability())
	require.Equal(t, PromiseCleanupObservabilitySnapshot{}, nilCtx.SnapshotAndResetPromiseCleanupObservability())
	require.Equal(t, CloseEnqueueObservabilitySnapshot{}, nilCtx.SnapshotCloseEnqueueObservability())
	require.Equal(t, CloseEnqueueObservabilitySnapshot{}, nilCtx.SnapshotAndResetCloseEnqueueObservability())
	require.Equal(t, uintptr(0), nilCtx.cContextKeyForTest())
	require.False(t, nilCtx.enqueueJobDuringClose(nil))
	require.False(t, nilCtx.enqueueJobDuringCloseWithSource(func(*Context) {}, closeEnqueueSourceOther))
	require.Nil(t, nilCtx.ensureAutoReleaseFinalizerRegistry())
	require.Nil(t, emptyCtx.ensureAutoReleaseFinalizerRegistry())
	require.False(t, nilCtx.releaseFunctionByID(1))
	require.False(t, nilCtx.ReleaseFunction(nil))
	nilCtx.requireOwnerThread("noop")

	// Constructors on empty context should fail closed.
	require.Nil(t, emptyCtx.NewInt64(1))
	require.Nil(t, emptyCtx.NewUint32(1))
	require.Nil(t, emptyCtx.NewBigInt64(1))
	require.Nil(t, emptyCtx.NewBigUint64(1))
	require.Nil(t, emptyCtx.NewFloat64(1.25))
	require.Nil(t, nilCtx.NewFunction(nil))
	require.Nil(t, emptyCtx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return nil
	}))
	require.Nil(t, nilCtx.NewInt8Array(nil))
	require.Nil(t, nilCtx.NewUint8Array(nil))
	require.Nil(t, nilCtx.NewUint8ClampedArray(nil))
	require.Nil(t, nilCtx.NewInt16Array(nil))
	require.Nil(t, nilCtx.NewUint16Array(nil))
	require.Nil(t, nilCtx.NewInt32Array(nil))
	require.Nil(t, nilCtx.NewUint32Array(nil))
	require.Nil(t, nilCtx.NewFloat32Array(nil))
	require.Nil(t, nilCtx.NewFloat64Array(nil))
	require.Nil(t, nilCtx.NewBigInt64Array(nil))
	require.Nil(t, nilCtx.NewBigUint64Array(nil))
	require.Nil(t, emptyCtx.NewInt8Array([]int8{1}))
	require.Nil(t, emptyCtx.NewUint8Array([]uint8{1}))
	require.Nil(t, emptyCtx.NewUint8ClampedArray([]uint8{1}))
	require.Nil(t, emptyCtx.NewInt16Array([]int16{1}))
	require.Nil(t, emptyCtx.NewUint16Array([]uint16{1}))
	require.Nil(t, emptyCtx.NewInt32Array([]int32{1}))
	require.Nil(t, emptyCtx.NewUint32Array([]uint32{1}))
	require.Nil(t, emptyCtx.NewFloat32Array([]float32{1}))
	require.Nil(t, emptyCtx.NewFloat64Array([]float64{1}))
	require.Nil(t, emptyCtx.NewBigInt64Array([]int64{1}))
	require.Nil(t, emptyCtx.NewBigUint64Array([]uint64{1}))
}

func TestContextNewFunctionHandleStoreGuard(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	require.Nil(t, ctx.NewFunction(nil))

	originalStore := ctx.handleStore
	ctx.handleStore = nil
	require.Nil(t, ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	}))
	ctx.handleStore = originalStore
}

func TestContextEnsureAutoReleaseFinalizerRegistryHandleStoreGuard(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	originalStore := ctx.handleStore
	ctx.handleStore = nil
	require.Nil(t, ctx.ensureAutoReleaseFinalizerRegistry())
	ctx.handleStore = originalStore
}

func TestContextWrapPromiseCallbackGuardBranches(t *testing.T) {
	var nilCtx *Context
	callback, release := nilCtx.wrapPromiseCallback(nil)
	require.NotNil(t, callback)
	require.NotNil(t, release)
	require.NotPanics(t, func() { callback(nil) })
	require.NotPanics(t, func() { release() })

	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseRefs := ctx.currentPromiseCallbackRefCount()
	orphanFn := &Value{}
	_, release2 := ctx.wrapPromiseCallback(orphanFn)
	require.NotPanics(t, func() { release2() })
	require.Equal(t, baseRefs, ctx.currentPromiseCallbackRefCount())
}

func TestContextNewFunctionForcedExceptionForCoverage(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseHandles := ctx.handleStore.Count()

	setContextNewFunctionForceExceptionForTest(true)
	t.Cleanup(func() {
		setContextNewFunctionForceExceptionForTest(false)
	})

	fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	require.NotNil(t, fn)
	defer fn.Free()
	require.True(t, fn.IsException())
	require.Equal(t, baseHandles, ctx.handleStore.Count())
}

func TestContextNewFunctionZeroKeyPathForCoverage(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	setContextNewFunctionForceZeroKeyForTest(true)
	t.Cleanup(func() {
		setContextNewFunctionForceZeroKeyForTest(false)
	})

	fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	require.NotNil(t, fn)
	defer fn.Free()
	require.False(t, fn.IsException())

	_, ok := ctx.functionHandleID(fn)
	require.False(t, ok)
}

func TestContextEnqueueDropSourceBuckets(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.SnapshotAndResetCloseEnqueueObservability()
	for i := 0; i < defaultJobQueueSize; i++ {
		require.True(t, ctx.enqueueJobDuringClose(func(*Context) {}))
	}

	require.False(t, ctx.enqueueJobDuringCloseWithSource(func(*Context) {}, closeEnqueueSourceValueFree))
	require.False(t, ctx.enqueueJobDuringCloseWithSource(func(*Context) {}, closeEnqueueSourcePromiseCallback))
	require.False(t, ctx.enqueueJobDuringCloseWithSource(func(*Context) {}, closeEnqueueSourceOther))

	snap := ctx.SnapshotAndResetCloseEnqueueObservability()
	require.GreaterOrEqual(t, snap.ValueFreeDropped, uint64(1))
	require.GreaterOrEqual(t, snap.PromiseCallbackDropped, uint64(1))
	require.GreaterOrEqual(t, snap.OtherDropped, uint64(1))
	require.Equal(t, snap.Dropped, snap.ValueFreeDropped+snap.PromiseCallbackDropped+snap.OtherDropped)
}

func TestContextObservePromiseCleanupAllBranches(t *testing.T) {
	var nilCtx *Context
	require.NotPanics(t, func() { nilCtx.observePromiseCleanup(promiseCleanupSourceCancel) })
	require.NotPanics(t, func() { nilCtx.observePromiseCleanup(promiseCleanupSourceFinally) })
	require.NotPanics(t, func() { nilCtx.observePromiseCleanup(promiseCleanupSourceFallback) })

	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.SnapshotAndResetPromiseCleanupObservability()
	ctx.observePromiseCleanup(promiseCleanupSourceCancel)
	ctx.observePromiseCleanup(promiseCleanupSourceFinally)
	ctx.observePromiseCleanup(promiseCleanupSourceFallback)
	ctx.observePromiseCleanup(promiseCleanupSource(255)) // default -> fallback

	snap := ctx.SnapshotAndResetPromiseCleanupObservability()
	require.Equal(t, uint64(1), snap.CancelTriggered)
	require.Equal(t, uint64(1), snap.FinallyTriggered)
	require.Equal(t, uint64(2), snap.FallbackTriggered)
}

func TestContextFunctionHandleIDGuardBranches(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	id, ok := ctx.functionHandleID(nil)
	require.False(t, ok)
	require.Zero(t, id)

	nonFn := ctx.NewString("not-fn")
	defer nonFn.Free()
	id, ok = ctx.functionHandleID(nonFn)
	require.False(t, ok)
	require.Zero(t, id)

	fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	defer fn.Free()

	id, ok = ctx.functionHandleID(fn)
	require.True(t, ok)
	require.Greater(t, id, int32(0))

	require.True(t, ctx.releaseFunctionByID(id))
	require.False(t, ctx.releaseFunctionByID(id))
	require.False(t, ctx.releaseFunctionByID(-1))
	require.False(t, ctx.releaseFunctionByID(0))

	id, ok = ctx.functionHandleID(fn)
	require.False(t, ok)
	require.Zero(t, id)

	fn2 := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	defer fn2.Free()

	setContextFunctionHandleIDForceZeroKeyForTest(true)
	id, ok = ctx.functionHandleID(fn2)
	setContextFunctionHandleIDForceZeroKeyForTest(false)
	require.False(t, ok)
	require.Zero(t, id)
}

func TestContextFunctionHandleIDInvalidMapValueType(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	defer fn.Free()

	id, ok := ctx.functionHandleID(fn)
	require.True(t, ok)

	var matchedKey interface{}
	ctx.fnHandleMap.Range(func(key, value interface{}) bool {
		mappedID, castOK := value.(int32)
		if castOK && mappedID == id {
			matchedKey = key
			return false
		}
		return true
	})
	require.NotNil(t, matchedKey)

	ctx.fnHandleMap.Store(matchedKey, "invalid-type")
	gotID, gotOK := ctx.functionHandleID(fn)
	require.False(t, gotOK)
	require.Zero(t, gotID)

	_, exists := ctx.fnHandleMap.Load(matchedKey)
	require.False(t, exists)
}

func TestContextLoadFunctionFromHandleIDFailClosed(t *testing.T) {
	var nilCtx *Context
	require.Nil(t, nilCtx.loadFunctionFromHandleID(1))

	orphan := &Context{}
	require.Nil(t, orphan.loadFunctionFromHandleID(1))

	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	require.Nil(t, ctx.loadFunctionFromHandleID(1))

	fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewString("ok")
	})
	defer fn.Free()

	id, ok := ctx.functionHandleID(fn)
	require.True(t, ok)
	require.Greater(t, id, int32(0))

	loaded := ctx.loadFunctionFromHandleID(id)
	require.NotNil(t, loaded)
	_, castOK := loaded.(func(*Context, *Value, []*Value) *Value)
	require.True(t, castOK)

	require.True(t, ctx.releaseFunctionByID(id))
	require.Nil(t, ctx.loadFunctionFromHandleID(id))
}

func TestContextRegisterFunctionForAutoReleaseGuards(t *testing.T) {
	var nilCtx *Context
	require.NotPanics(t, func() { nilCtx.registerFunctionForAutoRelease(nil) })

	orphan := &Context{}
	require.NotPanics(t, func() { orphan.registerFunctionForAutoRelease(nil) })

	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	nonFn := ctx.NewString("not-fn")
	defer nonFn.Free()
	require.NotPanics(t, func() { ctx.registerFunctionForAutoRelease(nonFn) })

	fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	defer fn.Free()

	id, ok := ctx.functionHandleID(fn)
	require.True(t, ok)

	var key interface{}
	ctx.fnHandleMap.Range(func(k, v interface{}) bool {
		mappedID, castOK := v.(int32)
		if castOK && mappedID == id {
			key = k
			return false
		}
		return true
	})
	require.NotNil(t, key)

	ctx.fnHandleMap.Store(key, "invalid-id-type")
	require.NotPanics(t, func() { ctx.registerFunctionForAutoRelease(fn) })

	_, exists := ctx.fnHandleMap.Load(key)
	require.False(t, exists)

	patch := ctx.Eval(`
        (() => {
            globalThis.__fr_backup_for_auto_release_guard = globalThis.FinalizationRegistry;
            delete globalThis.FinalizationRegistry;
            return typeof FinalizationRegistry;
        })()
    `)
	require.False(t, patch.IsException())
	require.Equal(t, "undefined", patch.ToString())
	patch.Free()

	fn2 := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	defer fn2.Free()
	require.NotPanics(t, func() { ctx.registerFunctionForAutoRelease(fn2) })

	restore := ctx.Eval(`
        (() => {
            globalThis.FinalizationRegistry = globalThis.__fr_backup_for_auto_release_guard;
            delete globalThis.__fr_backup_for_auto_release_guard;
            return typeof FinalizationRegistry;
        })()
    `)
	require.False(t, restore.IsException())
	restore.Free()
}

func TestContextReleaseFunctionGuardBranches(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	var nilCtx *Context
	require.False(t, nilCtx.ReleaseFunction(nil))

	nonFn := ctx.NewString("x")
	defer nonFn.Free()
	require.False(t, ctx.ReleaseFunction(nonFn))

	rt2 := NewRuntime()
	defer rt2.Close()
	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	foreignFn := ctx2.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	defer foreignFn.Free()
	require.False(t, ctx.ReleaseFunction(foreignFn))

	fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	defer fn.Free()

	id, ok := ctx.functionHandleID(fn)
	require.True(t, ok)

	var key interface{}
	ctx.fnHandleMap.Range(func(k, v interface{}) bool {
		mappedID, castOK := v.(int32)
		if castOK && mappedID == id {
			key = k
			return false
		}
		return true
	})
	require.NotNil(t, key)

	ctx.fnHandleMap.Store(key, "bad-id-type")
	require.False(t, ctx.ReleaseFunction(fn))

	_, exists := ctx.fnHandleMap.Load(key)
	require.False(t, exists)
}

func TestContextReleaseFunctionZeroKeyInjectedPath(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewUndefined()
	})
	defer fn.Free()

	setContextReleaseFunctionForceZeroKeyForTest(true)
	t.Cleanup(func() {
		setContextReleaseFunctionForceZeroKeyForTest(false)
	})

	require.False(t, ctx.ReleaseFunction(fn))
}

func TestContextRegisterPromiseSettlementCleanupGuards(t *testing.T) {
	var nilCtx *Context
	cancel := nilCtx.registerPromiseSettlementCleanup(nil, nil)
	require.NotNil(t, cancel)

	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	cleanupCount := 0
	nonPromise := ctx.NewString("x")
	defer nonPromise.Free()
	cancel = ctx.registerPromiseSettlementCleanup(nonPromise, func() { cleanupCount++ })
	require.NotNil(t, cancel)
	cancel()
	require.Equal(t, 1, cleanupCount)

	promise := ctx.Eval(`Promise.resolve(1)`)
	defer promise.Free()
	require.False(t, promise.IsException())
	require.True(t, promise.IsPromise())

	cleanupCount2 := 0
	cancel = ctx.registerPromiseSettlementCleanup(promise, func() { cleanupCount2++ })
	require.NotNil(t, cancel)
	cancel()
	ctx.ProcessJobs()
	require.Equal(t, 1, cleanupCount2)

	// ensureAutoReleaseFinalizerRegistry returns nil branch
	patch := ctx.Eval(`
        (() => {
            globalThis.__fr_backup_for_cleanup_guard = globalThis.FinalizationRegistry;
            delete globalThis.FinalizationRegistry;
            return typeof FinalizationRegistry;
        })()
    `)
	require.False(t, patch.IsException())
	patch.Free()

	promise2 := ctx.Eval(`Promise.resolve(2)`)
	defer promise2.Free()
	require.False(t, promise2.IsException())
	require.True(t, promise2.IsPromise())

	cleanupCount3 := 0
	cancel = ctx.registerPromiseSettlementCleanup(promise2, func() { cleanupCount3++ })
	require.NotNil(t, cancel)
	cancel()
	ctx.ProcessJobs()
	require.Equal(t, 1, cleanupCount3)

	promise3 := ctx.Eval(`Promise.resolve(3)`)
	defer promise3.Free()
	require.False(t, promise3.IsException())
	require.True(t, promise3.IsPromise())

	cleanupCount4 := 0
	setContextNewFunctionForceExceptionForTest(true)
	cancel = ctx.registerPromiseSettlementCleanup(promise3, func() { cleanupCount4++ })
	setContextNewFunctionForceExceptionForTest(false)
	require.NotNil(t, cancel)
	cancel()
	ctx.ProcessJobs()
	require.Equal(t, 1, cleanupCount4)

	restore := ctx.Eval(`
        (() => {
            globalThis.FinalizationRegistry = globalThis.__fr_backup_for_cleanup_guard;
            delete globalThis.__fr_backup_for_cleanup_guard;
            return typeof FinalizationRegistry;
        })()
    `)
	require.False(t, restore.IsException())
	restore.Free()
}

func TestEnsureAutoReleaseFinalizerRegistryConstructorThrows(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	featureCheck := ctx.Eval(`typeof FinalizationRegistry !== "undefined"`)
	require.False(t, featureCheck.IsException())
	if !featureCheck.Bool() {
		featureCheck.Free()
		t.Skip("FinalizationRegistry is not available")
	}
	featureCheck.Free()

	patch := ctx.Eval(`
        (() => {
            globalThis.__origFRForCtorThrowTest = globalThis.FinalizationRegistry;
            globalThis.FinalizationRegistry = function() { throw new Error("ctor-fail"); };
            return true;
        })()
    `)
	require.False(t, patch.IsException())
	patch.Free()

	registry := ctx.ensureAutoReleaseFinalizerRegistry()
	require.Nil(t, registry)

	restore := ctx.Eval(`
        (() => {
            globalThis.FinalizationRegistry = globalThis.__origFRForCtorThrowTest;
            delete globalThis.__origFRForCtorThrowTest;
            return true;
        })()
    `)
	require.False(t, restore.IsException())
	restore.Free()

	retry := ctx.ensureAutoReleaseFinalizerRegistry()
	require.NotNil(t, retry)
}

func TestEnsureAutoReleaseFinalizerRegistryInterruptSafety(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseHandles := ctx.handleStore.Count()

	ctx.SetInterruptHandler(func() int { return 1 })
	defer ctx.SetInterruptHandler(nil)

	registry := ctx.ensureAutoReleaseFinalizerRegistry()
	if registry != nil && registry.IsException() {
		registry.Free()
		registry = nil
	}
	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles+1)
}

func TestEnsureAutoReleaseFinalizerRegistryCleanupFnExceptionPath(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseHandles := ctx.handleStore.Count()

	setContextNewFunctionForceExceptionForTest(true)
	t.Cleanup(func() {
		setContextNewFunctionForceExceptionForTest(false)
	})

	registry := ctx.ensureAutoReleaseFinalizerRegistry()
	require.Nil(t, registry)
	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles)
}

func TestEnsureAutoReleaseFinalizerRegistryFactoryExceptionPath(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseHandles := ctx.handleStore.Count()

	setContextEnsureAutoReleaseForceFactoryExceptionForTest(true)
	t.Cleanup(func() {
		setContextEnsureAutoReleaseForceFactoryExceptionForTest(false)
	})

	registry := ctx.ensureAutoReleaseFinalizerRegistry()
	require.Nil(t, registry)
	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles)
}

func TestEnsureAutoReleaseFinalizerRegistryFactoryEvalExceptionPath(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	baseHandles := ctx.handleStore.Count()

	setContextEnsureAutoReleaseForceFactoryEvalExceptionForTest(true)
	t.Cleanup(func() {
		setContextEnsureAutoReleaseForceFactoryEvalExceptionForTest(false)
	})

	registry := ctx.ensureAutoReleaseFinalizerRegistry()
	require.Nil(t, registry)
	require.LessOrEqual(t, ctx.handleStore.Count(), baseHandles)
}

func TestContextFailClosedGuardBranches(t *testing.T) {
	emptyCtx := &Context{}
	require.Nil(t, emptyCtx.NewFunction(nil))
	require.Nil(t, emptyCtx.Throw(nil))
	require.Nil(t, emptyCtx.ThrowError(errors.New("x")))
	require.Nil(t, emptyCtx.ThrowSyntaxError("x"))
	require.Nil(t, emptyCtx.ThrowTypeError("x"))
	require.Nil(t, emptyCtx.ThrowReferenceError("x"))
	require.Nil(t, emptyCtx.ParseJSON("{}"))
}

func TestContextAsync(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("EventLoop", func(t *testing.T) {
		ctx := newCtx(t)
		result := ctx.Eval(`
            var executed = false;
            setTimeout(() => { executed = true; }, 10);
        `)
		defer result.Free()
		require.False(t, result.IsException())

		ctx.Loop()

		executedResult := ctx.Eval(`executed`)
		defer executedResult.Free()
		require.False(t, executedResult.IsException())
		require.True(t, executedResult.ToBool())
	})

	// Updated: Use Function + Promise instead of AsyncFunction
	t.Run("AwaitPromises", func(t *testing.T) {
		ctx := newCtx(t)
		// Test successful promise using new Promise API
		asyncTestFn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewPromise(func(resolve, reject func(*Value)) {
				resolve(ctx.NewString("awaited result"))
			})
		})
		ctx.Globals().Set("asyncTest", asyncTestFn)

		promiseResult := ctx.Eval(`asyncTest()`)
		require.False(t, promiseResult.IsException())
		require.True(t, promiseResult.IsPromise())

		awaitedResult := ctx.Await(promiseResult)
		defer awaitedResult.Free()
		require.False(t, awaitedResult.IsException())
		require.EqualValues(t, "awaited result", awaitedResult.ToString())

		// Test rejected promise using new Promise API
		asyncRejectFn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewPromise(func(resolve, reject func(*Value)) {
				errorObj := ctx.NewError(errors.New("rejection reason"))
				defer errorObj.Free()
				reject(errorObj)
			})
		})
		ctx.Globals().Set("asyncReject", asyncRejectFn)

		rejectPromise := ctx.Eval(`asyncReject()`)
		require.False(t, rejectPromise.IsException())

		rejectedResult := ctx.Await(rejectPromise)
		defer rejectedResult.Free()
		require.True(t, rejectedResult.IsException())
		require.Contains(t, ctx.Exception().Error(), "rejection reason")
	})
}

func TestContextPromise(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("BasicPromise", func(t *testing.T) {
		ctx := newCtx(t)
		// Test immediate resolve
		promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
			resolve(ctx.NewString("success"))
		})

		require.True(t, promise.IsPromise())
		require.Equal(t, PromiseFulfilled, promise.PromiseState())

		result := promise.Await()
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "success", result.ToString())
	})

	t.Run("RejectedPromise", func(t *testing.T) {
		ctx := newCtx(t)
		promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
			errorObj := ctx.NewError(errors.New("error"))
			defer errorObj.Free()
			reject(errorObj)
		})

		require.True(t, promise.IsPromise())

		state := promise.PromiseState()
		require.Equal(t, PromiseRejected, state)

		result := promise.Await()
		defer result.Free()
		require.True(t, result.IsException())
	})

	t.Run("PromiseFunction", func(t *testing.T) {
		ctx := newCtx(t)
		// Create function that returns Promise
		asyncFn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewPromise(func(resolve, reject func(*Value)) {
				if len(args) == 0 {
					errObj := ctx.NewError(errors.New("no arguments provided"))
					defer errObj.Free()
					reject(errObj)
					return
				}
				resolve(ctx.NewString("Hello " + args[0].ToString()))
			})
		})

		// Test in JavaScript
		global := ctx.Globals()
		global.Set("asyncGreet", asyncFn)

		// Test with argument
		result1 := ctx.Eval(`asyncGreet("World")`)
		require.False(t, result1.IsException())

		final1 := result1.Await()
		defer final1.Free()
		require.False(t, final1.IsException())
		require.Equal(t, "Hello World", final1.ToString())

		// Test without argument (should reject)
		result2 := ctx.Eval(`asyncGreet()`)
		require.False(t, result2.IsException())

		final2 := result2.Await()
		defer final2.Free()
		require.True(t, final2.IsException())
	})

	t.Run("PromiseChaining", func(t *testing.T) {
		ctx := newCtx(t)
		// Create async function for chaining
		asyncDouble := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewPromise(func(resolve, reject func(*Value)) {
				if len(args) == 0 {
					errObj := ctx.NewError(errors.New("no number provided"))
					defer errObj.Free()
					reject(errObj)
					return
				}
				value := args[0].ToInt32()
				resolve(ctx.NewInt32(value * 2))
			})
		})

		global := ctx.Globals()
		global.Set("asyncDouble", asyncDouble)

		// Test promise chaining
		result := ctx.Eval(`
            asyncDouble(5)
                .then(x => asyncDouble(x))
                .then(x => x + 10)
        `)
		require.False(t, result.IsException())

		final := result.Await()
		defer final.Free()
		require.False(t, final.IsException())
		require.Equal(t, int32(30), final.ToInt32()) // 5 * 2 * 2 + 10 = 30
	})

	t.Run("PromiseState", func(t *testing.T) {
		ctx := newCtx(t)
		// Test different promise states
		pendingPromise := ctx.Eval(`new Promise(() => {})`) // Never resolves
		defer pendingPromise.Free()
		require.False(t, pendingPromise.IsException())
		require.Equal(t, PromisePending, pendingPromise.PromiseState())

		fulfilledPromise := ctx.Eval(`Promise.resolve("fulfilled")`)
		defer fulfilledPromise.Free()
		require.False(t, fulfilledPromise.IsException())
		require.Equal(t, PromiseFulfilled, fulfilledPromise.PromiseState())

		rejectedPromise := ctx.Eval(`Promise.reject("rejected")`)
		defer rejectedPromise.Free()
		require.False(t, rejectedPromise.IsException())
		require.Equal(t, PromiseRejected, rejectedPromise.PromiseState())

		// Test PromiseState on non-Promise
		nonPromise := ctx.NewString("not a promise")
		defer nonPromise.Free()
		require.Equal(t, PromisePending, nonPromise.PromiseState()) // Should return default
	})

	t.Run("ValueAwait", func(t *testing.T) {
		ctx := newCtx(t)
		// Test Value.Await() method
		promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
			resolve(ctx.NewString("awaited via Value.Await"))
		})

		result := promise.Await()
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "awaited via Value.Await", result.ToString())

		// Test Await on non-Promise (should return equivalent value)
		nonPromise := ctx.NewString("not a promise")

		result2 := nonPromise.Await()
		defer result2.Free()
		require.False(t, result2.IsException())

		// Verify the content is the same
		require.Equal(t, nonPromise.ToString(), result2.ToString())
		require.Equal(t, "not a promise", result2.ToString())

		// Verify it's still a string
		require.True(t, result2.IsString())
	})

	t.Run("ComplexAsync", func(t *testing.T) {
		ctx := newCtx(t)
		// Test more complex async scenario
		asyncProcessor := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewPromise(func(resolve, reject func(*Value)) {
				if len(args) == 0 {
					errObj := ctx.NewError(errors.New("no data to process"))
					defer errObj.Free()
					reject(errObj)
					return
				}

				// Simulate processing
				input := args[0].ToString()
				if input == "error" {
					errObj := ctx.NewError(errors.New("processing failed"))
					defer errObj.Free()
					reject(errObj)
					return
				}

				result := ctx.NewString("processed: " + input)
				resolve(result)
			})
		})

		global := ctx.Globals()
		global.Set("process", asyncProcessor)

		// Test successful processing
		success := ctx.Eval(`process("data").then(result => "Success: " + result)`)
		require.False(t, success.IsException())

		successResult := success.Await()
		defer successResult.Free()
		require.False(t, successResult.IsException())
		require.Equal(t, "Success: processed: data", successResult.ToString())

		// Test error handling
		errorCase := ctx.Eval(`process("error").catch(err =>  err)`)
		require.False(t, errorCase.IsException())

		errorResult := errorCase.Await()
		defer errorResult.Free()
		require.False(t, errorResult.IsException())
		require.Equal(t, "Error: processing failed", errorResult.ToString())
	})

	t.Run("AwaitHandlesScheduledResolve", func(t *testing.T) {
		ctx := newCtx(t)
		scheduled := make(chan bool, 1)
		promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
			go func() {
				time.Sleep(10 * time.Millisecond)
				ok := ctx.Schedule(func(inner *Context) {
					val := inner.NewString("async scheduler value")
					defer val.Free()
					resolve(val)
				})
				scheduled <- ok
			}()
		})
		defer promise.Free()

		select {
		case ok := <-scheduled:
			require.True(t, ok, "failed to enqueue resolve job")
		case <-time.After(2 * time.Second):
			t.Fatal("resolve job was never scheduled")
		}

		// 主动驱动调度器，确保 resolve job 真正执行
		ctx.ProcessJobs()

		result := promise.Await()
		defer result.Free()

		require.False(t, result.IsException())
		require.Equal(t, "async scheduler value", result.ToString())
	})
}

func TestContextScheduler(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("LoopProcessesScheduledJobs", func(t *testing.T) {
		ctx := newCtx(t)
		results := make(chan struct {
			sum int32
			err error
		}, 1)

		require.True(t, ctx.Schedule(func(inner *Context) {
			val := inner.Eval(`40 + 2`)
			defer val.Free()

			if val.IsException() {
				results <- struct {
					sum int32
					err error
				}{err: inner.Exception()}
				return
			}

			results <- struct {
				sum int32
				err error
			}{sum: int32(val.ToInt32())}
		}))

		ctx.Loop()

		select {
		case res := <-results:
			require.NoError(t, res.err)
			require.Equal(t, int32(42), res.sum)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("scheduled job was not executed")
		}
	})
}

func TestContextInternalsCoverage(t *testing.T) {
	t.Run("ScheduleEdgeCases", func(t *testing.T) {
		var nilCtx *Context
		require.False(t, nilCtx.Schedule(func(*Context) {}))
		require.Nil(t, nilCtx.NewBool(true))
		require.Nil(t, nilCtx.NewInt32(1))
		require.Nil(t, nilCtx.NewString("x"))
		require.Nil(t, nilCtx.NewArrayBuffer(nil))
		require.Nil(t, nilCtx.NewObject())
		require.Nil(t, nilCtx.ParseJSON("{}"))
		require.Nil(t, nilCtx.NewFunction(func(*Context, *Value, []*Value) *Value { return nil }))
		require.Nil(t, nilCtx.NewAtom("x"))
		require.Nil(t, nilCtx.NewAtomIdx(1))
		require.Nil(t, nilCtx.NewPromise(func(resolve, reject func(*Value)) {}))
		promiseNil, cancelNil := nilCtx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {})
		require.Nil(t, promiseNil)
		require.NotNil(t, cancelNil)
		require.NotPanics(t, func() { cancelNil() })
		require.Nil(t, nilCtx.Invoke(nil, nil))
		require.Nil(t, nilCtx.ThrowSyntaxError("x"))
		require.Nil(t, nilCtx.ThrowTypeError("x"))
		require.Nil(t, nilCtx.ThrowReferenceError("x"))
		require.Nil(t, nilCtx.ThrowRangeError("x"))
		require.Nil(t, nilCtx.ThrowInternalError("x"))
		require.False(t, nilCtx.HasException())
		require.Nil(t, nilCtx.Exception())
		require.NotPanics(t, func() { nilCtx.Loop() })
		require.Nil(t, nilCtx.Await(nil))

		ctx := &Context{}
		require.False(t, ctx.Schedule(func(*Context) {}))
		require.Nil(t, ctx.NewBool(true))
		require.Nil(t, ctx.NewInt32(1))
		require.Nil(t, ctx.NewString("x"))
		require.Nil(t, ctx.NewArrayBuffer(nil))
		require.Nil(t, ctx.NewObject())
		require.Nil(t, ctx.ParseJSON("{}"))
		require.Nil(t, ctx.NewFunction(func(*Context, *Value, []*Value) *Value { return nil }))
		require.Nil(t, ctx.NewAtom("x"))
		require.Nil(t, ctx.NewAtomIdx(1))
		require.Nil(t, ctx.NewPromise(func(resolve, reject func(*Value)) {}))
		promiseOrphan, cancelOrphan := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {})
		require.Nil(t, promiseOrphan)
		require.NotNil(t, cancelOrphan)
		require.NotPanics(t, func() { cancelOrphan() })
		require.Nil(t, ctx.Invoke(nil, nil))
		require.Nil(t, ctx.ThrowSyntaxError("x"))
		require.Nil(t, ctx.ThrowTypeError("x"))
		require.Nil(t, ctx.ThrowReferenceError("x"))
		require.Nil(t, ctx.ThrowRangeError("x"))
		require.Nil(t, ctx.ThrowInternalError("x"))
		require.False(t, ctx.HasException())
		require.Nil(t, ctx.Exception())
		require.NotPanics(t, func() { ctx.Loop() })
		orphanPromise := &Value{}
		require.Same(t, orphanPromise, ctx.Await(orphanPromise))

		ctx.initScheduler()
		require.False(t, ctx.Schedule(nil))

		close(ctx.jobClosed)
		require.False(t, ctx.Schedule(func(*Context) {}))

		blockingCtx := &Context{
			jobQueue:  make(chan func(*Context), 1),
			jobClosed: make(chan struct{}),
		}
		blockingCtx.jobQueue <- func(*Context) {}
		done := make(chan struct{})
		go func() {
			time.Sleep(5 * time.Millisecond)
			close(blockingCtx.jobClosed)
			close(done)
		}()
		require.False(t, blockingCtx.Schedule(func(*Context) {}))
		<-done

		rt := NewRuntime()
		defer rt.Close()
		realCtx := rt.NewContext()
		defer realCtx.Close()
		promiseErr, cancelErr := realCtx.NewPromiseWithCancel(nil)
		require.NotNil(t, promiseErr)
		require.True(t, promiseErr.IsException())
		require.Contains(t, realCtx.Exception().Error(), "promise executor is nil")
		promiseErr.Free()
		require.NotPanics(t, func() { cancelErr() })
	})

	t.Run("ProcessJobsAndDrainJobs", func(t *testing.T) {
		var nilCtx *Context
		require.NotPanics(t, func() { nilCtx.ProcessJobs() })
		require.NotPanics(t, func() { nilCtx.drainJobs() })

		emptyCtx := &Context{}
		require.NotPanics(t, func() { emptyCtx.ProcessJobs() })
		require.NotPanics(t, func() { emptyCtx.drainJobs() })

		emptyCtx.initScheduler()
		var calls int
		require.True(t, emptyCtx.Schedule(func(inner *Context) {
			require.Same(t, emptyCtx, inner)
			calls++
		}))
		emptyCtx.jobQueue <- nil
		emptyCtx.ProcessJobs()
		require.Equal(t, 1, calls)

		emptyCtx.jobQueue <- func(*Context) {}
		emptyCtx.jobQueue <- nil
		emptyCtx.drainJobs()
		require.Equal(t, 0, len(emptyCtx.jobQueue))
	})

	t.Run("CloseNilSafety", func(t *testing.T) {
		var nilCtx *Context
		require.NotPanics(t, func() { nilCtx.Close() })

		orphan := &Context{}
		require.NotPanics(t, func() { orphan.Close() })

		rt := NewRuntime()
		defer rt.Close()
		realCtx := rt.NewContext()
		require.NotNil(t, realCtx)
		require.NotPanics(t, func() { realCtx.Close() })
		require.NotPanics(t, func() { realCtx.Close() })
		require.Nil(t, realCtx.ref)
	})

	t.Run("CloseAdditionalBranches", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		ctx := rt.NewContext()
		require.NotNil(t, ctx)

		featureCheck := ctx.Eval(`typeof FinalizationRegistry !== "undefined"`)
		require.False(t, featureCheck.IsException())
		if featureCheck.Bool() {
			require.NotNil(t, ctx.ensureAutoReleaseFinalizerRegistry())
		}
		featureCheck.Free()

		close(ctx.jobClosed) // already-closed branch in Close
		ctx.globals = nil    // globals nil branch in Close
		require.NotPanics(t, func() { ctx.Close() })
	})

	t.Run("CloseWithoutJobClosedChannel", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		ctx.jobClosed = nil // jobClosed nil branch in Close
		require.NotPanics(t, func() { ctx.Close() })
	})

	t.Run("CloseWithNilJobQueue", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		ctx.jobQueue = nil
		require.NotPanics(t, func() { ctx.Close() })
	})

	t.Run("CloseDrainUntilStableTimeoutPath", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		ctx := rt.NewContext()
		require.NotNil(t, ctx)

		ctx.jobQueue <- func(*Context) {
			time.Sleep(4 * time.Millisecond)
		}
		require.NotPanics(t, func() { ctx.Close() })
	})

	t.Run("DuplicateValue", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		original := ctx.NewString("duplicate-target")
		defer original.Free()

		dup := ctx.duplicateValue(original)
		require.NotNil(t, dup)
		require.Equal(t, "duplicate-target", dup.ToString())
		dup.Free()

		require.Nil(t, ctx.duplicateValue(nil))

		orphan := &Value{}
		require.Nil(t, ctx.duplicateValue(orphan))
	})

	t.Run("WrapPromiseCallback", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		noop, releaseNoop := ctx.wrapPromiseCallback(nil)
		require.NotPanics(t, func() {
			noop(nil)
			releaseNoop()
		})

		initGlobals := ctx.Eval(`
			globalThis.__promise_cb = "unset";
			globalThis.__should_not_change = "initial";
			globalThis.__promise_cb_nil = "unset";
		`)
		defer initGlobals.Free()

		cbValue := ctx.Eval(`(value) => { globalThis.__promise_cb = value; }`)
		defer cbValue.Free()

		callback, releaseCallback := ctx.wrapPromiseCallback(cbValue)
		arg := ctx.NewString("scheduled value")
		callback(arg)
		arg.Free()
		ctx.ProcessJobs()
		stored := ctx.Eval(`__promise_cb`)
		require.Equal(t, "scheduled value", stored.ToString())
		stored.Free()
		releaseCallback()

		cbValue2 := ctx.Eval(`(value) => { globalThis.__should_not_change = value; }`)
		defer cbValue2.Free()
		callback2, release2 := ctx.wrapPromiseCallback(cbValue2)
		release2()
		ignored := ctx.NewString("ignored")
		callback2(ignored)
		ignored.Free()
		ctx.ProcessJobs()
		unchanged := ctx.Eval(`__should_not_change`)
		require.Equal(t, "initial", unchanged.ToString())
		unchanged.Free()

		cbNilArg := ctx.Eval(`(function (value) { globalThis.__promise_cb_nil = value === undefined ? "was-undefined" : "had-value"; })`)
		defer cbNilArg.Free()
		require.False(t, cbNilArg.IsException())
		callbackNil, releaseNil := ctx.wrapPromiseCallback(cbNilArg)
		callbackNil(nil)
		ctx.ProcessJobs()
		nilResult := ctx.Eval(`__promise_cb_nil`)
		require.Equal(t, "was-undefined", nilResult.ToString())
		nilResult.Free()
		releaseNil()

		ctxClosed := rt.NewContext()
		defer ctxClosed.Close()
		initClosed := ctxClosed.Eval(`globalThis.__closed_cb = "unset";`)
		defer initClosed.Free()
		cbClosed := ctxClosed.Eval(`(value) => { globalThis.__closed_cb = value; }`)
		defer cbClosed.Free()
		callbackClosed, releaseClosed := ctxClosed.wrapPromiseCallback(cbClosed)
		close(ctxClosed.jobClosed)
		argClosed := ctxClosed.NewString("should not run")
		callbackClosed(argClosed)
		argClosed.Free()
		ctxClosed.ProcessJobs()
		closedResult := ctxClosed.Eval(`__closed_cb`)
		require.Equal(t, "unset", closedResult.ToString())
		closedResult.Free()
		releaseClosed()

		ctxOffThread := rt.NewContext()
		defer ctxOffThread.Close()
		cbOffThread := ctxOffThread.Eval(`(value) => { globalThis.__off_thread_cb = value; }`)
		defer cbOffThread.Free()
		require.False(t, cbOffThread.IsException())

		baseRefs := ctxOffThread.currentPromiseCallbackRefCount()
		callbackOffThread, releaseOffThread := ctxOffThread.wrapPromiseCallback(cbOffThread)
		require.Equal(t, baseRefs+1, ctxOffThread.currentPromiseCallbackRefCount())

		close(ctxOffThread.jobClosed)
		done := make(chan interface{}, 2)
		go func() {
			defer func() { done <- recover() }()
			releaseOffThread()
		}()
		go func() {
			defer func() { done <- recover() }()
			callbackOffThread(nil)
		}()
		require.Nil(t, <-done)
		require.Nil(t, <-done)

		ctxOffThread.ProcessJobs()
		require.Equal(t, baseRefs, ctxOffThread.currentPromiseCallbackRefCount())
	})

	t.Run("InvokeAndAsyncNilSafety", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			if len(args) == 0 || args[0] == nil || args[0].IsUndefined() {
				return ctx.NewString("undefined")
			}
			return ctx.NewString(args[0].ToString())
		})
		defer fn.Free()

		require.NotPanics(t, func() {
			result := ctx.Invoke(fn, nil, nil)
			require.NotNil(t, result)
			defer result.Free()
			require.Equal(t, "undefined", result.ToString())
		})

		orphanArg := &Value{}
		orphanArgResult := ctx.Invoke(fn, nil, orphanArg)
		require.NotNil(t, orphanArgResult)
		defer orphanArgResult.Free()
		require.False(t, orphanArgResult.IsException())
		require.Equal(t, "undefined", orphanArgResult.ToString())

		rt2 := NewRuntime()
		defer rt2.Close()
		ctx2 := rt2.NewContext()
		defer ctx2.Close()

		foreignArg := ctx2.NewString("foreign")
		defer foreignArg.Free()
		crossArgResult := ctx.Invoke(fn, nil, foreignArg)
		require.NotNil(t, crossArgResult)
		defer crossArgResult.Free()
		require.True(t, crossArgResult.IsException())
		require.Contains(t, ctx.Exception().Error(), "cross-context argument")

		foreignThis := ctx2.NewObject()
		defer foreignThis.Free()
		crossThisResult := ctx.Invoke(fn, foreignThis)
		require.NotNil(t, crossThisResult)
		defer crossThisResult.Free()
		require.True(t, crossThisResult.IsException())
		require.Contains(t, ctx.Exception().Error(), "cross-context this value")

		foreignFn := ctx2.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewString("foreign")
		})
		defer foreignFn.Free()
		crossFnResult := ctx.Invoke(foreignFn, nil)
		require.NotNil(t, crossFnResult)
		defer crossFnResult.Free()
		require.True(t, crossFnResult.IsException())
		require.Contains(t, ctx.Exception().Error(), "cross-context function")

		orphanCtx := &Context{}
		require.Nil(t, orphanCtx.Invoke(fn, nil))

		asyncFn := ctx.NewAsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
			return nil
		})
		defer asyncFn.Free()
		ctx.Globals().Set("asyncNil", asyncFn)

		result := ctx.Eval(`asyncNil()`, EvalAwait(true))
		defer result.Free()
		require.False(t, result.IsException())
		require.True(t, result.IsUndefined())
	})

	t.Run("AwaitHandlesNilAndNonPromise", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		nonPromise := ctx.NewString("plain value")
		defer nonPromise.Free()
		require.Same(t, nonPromise, ctx.Await(nonPromise))

		var nilPromise *Value
		require.Nil(t, ctx.Await(nilPromise))
	})

	t.Run("AwaitDrivesScheduleForResolve", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Test that Await processes Go-scheduled work (via ctx.Schedule)
		// instead of relying on js_std_loop for C-level timers.
		promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
			go func() {
				ctx.Schedule(func(ctx *Context) {
					val := ctx.NewString("scheduled result")
					defer val.Free()
					resolve(val)
				})
			}()
		})
		defer promise.Free()
		result := ctx.Await(promise)
		defer result.Free()
		require.Equal(t, "scheduled result", result.ToString())
	})

	t.Run("AwaitPollsUntilDelayedResolve", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Test that the Await polling loop correctly yields and re-checks
		// when the Promise is resolved after a delay from a goroutine.
		// This exercises the time.Sleep path in Await's pending case.
		promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
			go func() {
				time.Sleep(5 * time.Millisecond) // force Await to poll a few times
				ctx.Schedule(func(ctx *Context) {
					val := ctx.NewString("delayed result")
					defer val.Free()
					resolve(val)
				})
			}()
		})
		defer promise.Free()
		result := ctx.Await(promise)
		defer result.Free()
		require.Equal(t, "delayed result", result.ToString())
	})

	t.Run("AwaitHandlesPendingJobFailure", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		promise := ctx.Eval(`new Promise(() => {})`)
		defer promise.Free()

		triggered := false
		awaitExecutePendingJobHook = func(hookCtx *Context, _ *Value, current int) (int, bool) {
			if hookCtx == ctx && !triggered {
				triggered = true
				return -1, true
			}
			return current, false
		}
		t.Cleanup(func() { awaitExecutePendingJobHook = nil })

		result := ctx.Await(promise)
		defer result.Free()
		require.True(t, result.IsException())

		err := ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to execute pending job")
	})

	// Cover line 1049-1050: executed==0, ProcessJobs resolves the promise,
	// re-check sees non-pending state → continue.
	t.Run("AwaitReCheckResolvesAfterProcessJobs", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		var resolvePromise func(*Value)
		promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
			resolvePromise = resolve
		})
		defer promise.Free()

		firstCall := true
		awaitExecutePendingJobHook = func(hookCtx *Context, _ *Value, current int) (int, bool) {
			if hookCtx != ctx {
				return current, false
			}
			if firstCall {
				firstCall = false
				// Schedule a job that resolves the promise. ProcessJobs() at
				// line 1045 will pick it up, so the re-check at line 1048
				// sees fulfilled state.
				ctx.Schedule(func(inner *Context) {
					val := inner.NewString("resolved-via-recheck")
					defer val.Free()
					resolvePromise(val)
				})
				return 0, true // force executed=0
			}
			return current, false
		}
		t.Cleanup(func() { awaitExecutePendingJobHook = nil })

		result := ctx.Await(promise)
		defer result.Free()
		require.Equal(t, "resolved-via-recheck", result.ToString())
	})

	// Cover line 1056-1057: force executed==0 and pending-go-jobs=true,
	// then verify Await takes the continue path before next iteration resolves.
	t.Run("AwaitContinuesWhenJobQueueNonEmpty", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		var resolvePromise func(*Value)
		promise := ctx.NewPromise(func(resolve, reject func(*Value)) {
			resolvePromise = resolve
		})
		defer promise.Free()

		forcedPendingGoJobs := false
		awaitExecutePendingJobHook = func(hookCtx *Context, _ *Value, current int) (int, bool) {
			if hookCtx != ctx {
				return current, false
			}
			return 0, true
		}
		awaitHasPendingGoJobsHook = func(hookCtx *Context, _ *Value, current bool) (bool, bool) {
			if hookCtx != ctx || forcedPendingGoJobs {
				return current, false
			}
			forcedPendingGoJobs = true
			ctx.Schedule(func(inner *Context) {
				val := inner.NewString("after-queue-check")
				defer val.Free()
				resolvePromise(val)
			})
			return true, true
		}
		t.Cleanup(func() {
			awaitExecutePendingJobHook = nil
			awaitHasPendingGoJobsHook = nil
		})

		result := ctx.Await(promise)
		defer result.Free()
		require.Equal(t, "after-queue-check", result.ToString())
	})

	t.Run("AwaitFallsBackOnUnexpectedState", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctxA := rt.NewContext()
		defer ctxA.Close()
		ctxB := rt.NewContext()
		defer ctxB.Close()

		promise := ctxA.Eval(`new Promise(() => {})`)
		defer promise.Free()

		forced := false
		awaitPromiseStateHook = func(hookCtx *Context, _ *Value, current int) (int, bool) {
			if hookCtx == ctxB && !forced {
				forced = true
				return current + 99, true
			}
			return current, false
		}
		t.Cleanup(func() { awaitPromiseStateHook = nil })

		result := ctxB.Await(promise)
		require.Same(t, promise, result)
		require.True(t, result.IsUndefined())
	})
}

func TestContextCloseReadOnlyQueryContracts(t *testing.T) {
	rt := NewRuntime()
	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	require.NotNil(t, ctx.Runtime())

	result := ctx.Eval(`(() => { throw new Error("boom") })()`)
	require.True(t, result.IsException())
	result.Free()
	require.True(t, ctx.HasException())
	require.NotNil(t, ctx.Exception())

	ctx.Close()
	require.Nil(t, ctx.Runtime())
	require.False(t, ctx.HasException())
	require.Nil(t, ctx.Exception())

	// Read-only queries remain fail-closed after close.
	require.NotPanics(t, func() { ctx.Loop() })

	rt.Close()
}

func TestContextTypedArrays(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("TypedArrayCreation", func(t *testing.T) {
		// Test all TypedArray creation methods
		typedArrayTests := []struct {
			name       string
			createFunc func(*Context) *Value
			checkFunc  func(*Value) bool // Changed parameter to pointer
			testEmpty  func(*Context) *Value
			testNil    func(*Context) *Value
		}{
			{
				"Int8Array",
				func(ctx *Context) *Value { return ctx.NewInt8Array([]int8{-128, -1, 0, 1, 127}) },
				func(v *Value) bool { return v.IsInt8Array() },
				func(ctx *Context) *Value { return ctx.NewInt8Array([]int8{}) },
				func(ctx *Context) *Value { return ctx.NewInt8Array(nil) },
			},
			{
				"Uint8Array",
				func(ctx *Context) *Value { return ctx.NewUint8Array([]uint8{0, 1, 128, 255}) },
				func(v *Value) bool { return v.IsUint8Array() },
				func(ctx *Context) *Value { return ctx.NewUint8Array([]uint8{}) },
				func(ctx *Context) *Value { return ctx.NewUint8Array(nil) },
			},
			{
				"Uint8ClampedArray",
				func(ctx *Context) *Value { return ctx.NewUint8ClampedArray([]uint8{0, 127, 255}) },
				func(v *Value) bool { return v.IsUint8ClampedArray() },
				func(ctx *Context) *Value { return ctx.NewUint8ClampedArray([]uint8{}) },
				func(ctx *Context) *Value { return ctx.NewUint8ClampedArray(nil) },
			},
			{
				"Int16Array",
				func(ctx *Context) *Value { return ctx.NewInt16Array([]int16{-32768, -1, 0, 1, 32767}) },
				func(v *Value) bool { return v.IsInt16Array() },
				func(ctx *Context) *Value { return ctx.NewInt16Array([]int16{}) },
				func(ctx *Context) *Value { return ctx.NewInt16Array(nil) },
			},
			{
				"Uint16Array",
				func(ctx *Context) *Value { return ctx.NewUint16Array([]uint16{0, 1, 32768, 65535}) },
				func(v *Value) bool { return v.IsUint16Array() },
				func(ctx *Context) *Value { return ctx.NewUint16Array([]uint16{}) },
				func(ctx *Context) *Value { return ctx.NewUint16Array(nil) },
			},
			{
				"Int32Array",
				func(ctx *Context) *Value { return ctx.NewInt32Array([]int32{-2147483648, -1, 0, 1, 2147483647}) },
				func(v *Value) bool { return v.IsInt32Array() },
				func(ctx *Context) *Value { return ctx.NewInt32Array([]int32{}) },
				func(ctx *Context) *Value { return ctx.NewInt32Array(nil) },
			},
			{
				"Uint32Array",
				func(ctx *Context) *Value { return ctx.NewUint32Array([]uint32{0, 1, 2147483648, 4294967295}) },
				func(v *Value) bool { return v.IsUint32Array() },
				func(ctx *Context) *Value { return ctx.NewUint32Array([]uint32{}) },
				func(ctx *Context) *Value { return ctx.NewUint32Array(nil) },
			},
			{
				"Float32Array",
				func(ctx *Context) *Value { return ctx.NewFloat32Array([]float32{-3.14, 0.0, 1.5, 3.14159}) },
				func(v *Value) bool { return v.IsFloat32Array() },
				func(ctx *Context) *Value { return ctx.NewFloat32Array([]float32{}) },
				func(ctx *Context) *Value { return ctx.NewFloat32Array(nil) },
			},
			{
				"Float64Array",
				func(ctx *Context) *Value {
					return ctx.NewFloat64Array([]float64{-3.141592653589793, 0.0, 1.5, 3.141592653589793})
				},
				func(v *Value) bool { return v.IsFloat64Array() },
				func(ctx *Context) *Value { return ctx.NewFloat64Array([]float64{}) },
				func(ctx *Context) *Value { return ctx.NewFloat64Array(nil) },
			},
			{
				"BigInt64Array",
				func(ctx *Context) *Value {
					return ctx.NewBigInt64Array([]int64{-9223372036854775808, -1, 0, 1, 9223372036854775807})
				},
				func(v *Value) bool { return v.IsBigInt64Array() },
				func(ctx *Context) *Value { return ctx.NewBigInt64Array([]int64{}) },
				func(ctx *Context) *Value { return ctx.NewBigInt64Array(nil) },
			},
			{
				"BigUint64Array",
				func(ctx *Context) *Value {
					return ctx.NewBigUint64Array([]uint64{0, 1, 9223372036854775808, 18446744073709551615})
				},
				func(v *Value) bool { return v.IsBigUint64Array() },
				func(ctx *Context) *Value { return ctx.NewBigUint64Array([]uint64{}) },
				func(ctx *Context) *Value { return ctx.NewBigUint64Array(nil) },
			},
		}

		for _, tt := range typedArrayTests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := newCtx(t)
				// Test with data
				arr := tt.createFunc(ctx)
				defer arr.Free()
				require.True(t, arr.IsTypedArray())
				require.True(t, tt.checkFunc(arr))

				// Test empty array
				emptyArr := tt.testEmpty(ctx)
				defer emptyArr.Free()
				require.True(t, tt.checkFunc(emptyArr))
				require.EqualValues(t, 0, emptyArr.Len())

				// Test nil slice
				nilArr := tt.testNil(ctx)
				defer nilArr.Free()
				require.True(t, tt.checkFunc(nilArr))
				require.EqualValues(t, 0, nilArr.Len())
			})
		}
	})

	t.Run("TypedArrayInterop", func(t *testing.T) {
		ctx := newCtx(t)
		// Go to JavaScript
		goData := []int32{1, 2, 3, 4, 5}
		goArray := ctx.NewInt32Array(goData)
		ctx.Globals().Set("goArray", goArray)

		result := ctx.Eval(`
            let sum = 0;
            for (let i = 0; i < goArray.length; i++) {
                sum += goArray[i];
            }
            sum;
        `)
		defer result.Free()
		require.False(t, result.IsException())
		require.EqualValues(t, 15, result.ToInt32()) // 1+2+3+4+5 = 15

		// JavaScript to Go
		jsArray := ctx.Eval(`new Int32Array([10, 20, 30, 40, 50])`)
		defer jsArray.Free()
		require.False(t, jsArray.IsException())

		require.True(t, jsArray.IsTypedArray())
		require.True(t, jsArray.IsInt32Array())

		goSlice, err := jsArray.ToInt32Array()
		require.NoError(t, err)
		require.Equal(t, []int32{10, 20, 30, 40, 50}, goSlice)
	})

	t.Run("TypedArrayPrecision", func(t *testing.T) {
		ctx := newCtx(t)
		// Test Float32 precision
		float32Data := []float32{3.14159265359, -2.718281828, 0.0, 1.23456789}
		float32Array := ctx.NewFloat32Array(float32Data)
		defer float32Array.Free()

		converted32, err := float32Array.ToFloat32Array()
		require.NoError(t, err)
		require.Len(t, converted32, len(float32Data))

		for i, expected := range float32Data {
			require.InDelta(t, expected, converted32[i], 0.0001)
		}

		// Test Float64 precision
		float64Data := []float64{3.141592653589793, -2.718281828459045, 0.0, 1.2345678901234567}
		float64Array := ctx.NewFloat64Array(float64Data)
		defer float64Array.Free()

		converted64, err := float64Array.ToFloat64Array()
		require.NoError(t, err)
		require.Len(t, converted64, len(float64Data))

		for i, expected := range float64Data {
			require.InDelta(t, expected, converted64[i], 0.000000000001)
		}
	})

	t.Run("TypedArrayErrors", func(t *testing.T) {
		ctx := newCtx(t)
		// Test conversion errors for wrong types
		wrongTypeVal := ctx.NewString("not a typed array")
		defer wrongTypeVal.Free()

		conversionTests := []func() error{
			func() error { _, err := wrongTypeVal.ToInt8Array(); return err },
			func() error { _, err := wrongTypeVal.ToUint8Array(); return err },
			func() error { _, err := wrongTypeVal.ToInt16Array(); return err },
			func() error { _, err := wrongTypeVal.ToUint16Array(); return err },
			func() error { _, err := wrongTypeVal.ToInt32Array(); return err },
			func() error { _, err := wrongTypeVal.ToUint32Array(); return err },
			func() error { _, err := wrongTypeVal.ToFloat32Array(); return err },
			func() error { _, err := wrongTypeVal.ToFloat64Array(); return err },
			func() error { _, err := wrongTypeVal.ToBigInt64Array(); return err },
			func() error { _, err := wrongTypeVal.ToBigUint64Array(); return err },
		}

		for _, testFn := range conversionTests {
			require.Error(t, testFn())
		}

		// Test type mismatch conversion
		int8Array := ctx.NewInt8Array([]int8{1, 2, 3})
		defer int8Array.Free()

		_, err := int8Array.ToUint8Array()
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a Uint8Array")
	})

	t.Run("SharedMemoryTest", func(t *testing.T) {
		ctx := newCtx(t)
		// Test that TypedArrays share memory with their underlying ArrayBuffer
		data := []uint8{1, 2, 3, 4, 5, 6, 7, 8}
		arrayBuffer := ctx.NewArrayBuffer(data)
		ctx.Globals().Set("sharedBuffer", arrayBuffer)

		// Create different views on the same buffer
		ret := ctx.Eval(`
            globalThis.uint8View = new Uint8Array(sharedBuffer);
            globalThis.uint16View = new Uint16Array(sharedBuffer);
        `)
		defer ret.Free()
		require.False(t, ret.IsException())

		// Modify through uint8 view
		modifyResult := ctx.Eval(`uint8View[0] = 255;`)
		defer modifyResult.Free()
		require.False(t, modifyResult.IsException())

		// Verify change is visible through uint16 view (shared memory)
		uint16Value := ctx.Eval(`uint16View[0]`)
		defer uint16Value.Free()
		require.False(t, uint16Value.IsException())

		// The uint16 value should have changed because we modified the underlying byte
		// Original: bytes [1, 2] -> uint16: 513 (little-endian: 1 + 2*256)
		// Modified: bytes [255, 2] -> uint16: 767 (little-endian: 255 + 2*256)
		require.EqualValues(t, 767, uint16Value.ToInt32())

		// Clean up
		cleanupResult := ctx.Eval(`delete globalThis.uint8View; delete globalThis.uint16View;`)
		defer cleanupResult.Free()
		require.False(t, cleanupResult.IsException())
	})
}

func TestContextMemoryPressure(t *testing.T) {
	// Test extreme memory pressure to trigger compilation failures
	rt := NewRuntime(WithMemoryLimit(256 * 1024)) // 256KB limit
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Fill memory first
	memoryResult := ctx.Eval(`
        var memoryFiller = [];
        try {
            for(let i = 0; i < 1000; i++) {
                memoryFiller.push(new Array(100).fill('x'.repeat(50)));
            }
        } catch(e) {
            // Expected to fail due to memory limit
        }
    `)
	defer memoryResult.Free()

	// Try to compile - this should fail at JS_WriteObject due to no available memory
	_, err := ctx.Compile(`
        var obj = {};
        for(let i = 0; i < 100; i++) {
            obj['prop_' + i] = function() { return 'value_' + i; };
        }
        obj;
    `)

	if err != nil {
		t.Logf("Memory pressure compilation error (expected): %v", err)
	}

	// Try multiple rapid compilations to exhaust memory
	for i := 0; i < 20; i++ {
		code := fmt.Sprintf(`var obj%d = { data: new Array(500).fill(%d) }; obj%d;`, i, i, i)
		_, err := ctx.Compile(code)
		if err != nil {
			t.Logf("Rapid compilation %d failed (expected): %v", i, err)
			break
		}
	}
}

func TestContextAsyncFunction(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("AsyncFunctionResolveNoArgs", func(t *testing.T) {
		ctx := newCtx(t)
		// Test the resolve(ctx.NewUndefined()) branch when no arguments are passed
		asyncFn := ctx.NewAsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
			resolve := promise.Get("resolve")
			defer resolve.Free()

			// Call resolve without passing any arguments to cover resolve(ctx.NewUndefined()) branch
			resolve.Execute(ctx.NewUndefined()) // No arguments passed
			return ctx.NewUndefined()
		})

		ctx.Globals().Set("testAsyncResolveNoArgs", asyncFn)
		result := ctx.Eval(`testAsyncResolveNoArgs()`, EvalAwait(true))
		defer result.Free()
		require.False(t, result.IsException())
		require.True(t, result.IsUndefined()) // Should resolve to undefined
	})

	t.Run("AsyncFunctionRejectWithArgs", func(t *testing.T) {
		ctx := newCtx(t)
		// Test the reject(args[0]) branch when arguments are passed to reject
		asyncFn := ctx.NewAsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
			reject := promise.Get("reject")
			defer reject.Free()

			// Call reject with an error argument to cover reject(args[0]) branch
			errorVal := ctx.NewError(errors.New("specific error message"))
			defer errorVal.Free()
			reject.Execute(ctx.NewUndefined(), errorVal) // Pass argument
			return ctx.NewUndefined()
		})

		ctx.Globals().Set("testAsyncRejectWithArgs", asyncFn)
		result := ctx.Eval(`testAsyncRejectWithArgs()`, EvalAwait(true))
		defer result.Free()
		require.True(t, result.IsException())

		err := ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "specific error message")
	})

	t.Run("AsyncFunctionRejectNoArgs", func(t *testing.T) {
		ctx := newCtx(t)
		// Test the reject without arguments branch (else clause in reject function)
		asyncFn := ctx.NewAsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
			reject := promise.Get("reject")
			defer reject.Free()

			// Call reject without passing any arguments to cover the else branch
			// This will trigger: errObj := ctx.NewError(fmt.Errorf("Promise rejected without reason"))
			reject.Execute(ctx.NewUndefined()) // No arguments passed
			return ctx.NewUndefined()
		})

		ctx.Globals().Set("testAsyncRejectNoArgs", asyncFn)
		result := ctx.Eval(`testAsyncRejectNoArgs()`, EvalAwait(true))
		defer result.Free()
		require.True(t, result.IsException())

		err := ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "Promise rejected without reason")
	})

	t.Run("AsyncFunctionDirectReturnValue", func(t *testing.T) {
		ctx := newCtx(t)
		// Test the resolve(result) branch when function returns a non-undefined value
		asyncFn := ctx.NewAsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
			// Don't call promise.resolve or promise.reject, return a value directly
			// This covers the resolve(result) and result.Free() branches
			return ctx.NewString("direct return value")
		})

		ctx.Globals().Set("testAsyncDirectReturn", asyncFn)
		result := ctx.Eval(`testAsyncDirectReturn()`, EvalAwait(true))
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "direct return value", result.ToString())
	})

	t.Run("AsyncFunctionReturnUndefined", func(t *testing.T) {
		ctx := newCtx(t)
		// Test that returning undefined doesn't trigger the resolve(result) branch
		resolvedByPromise := false

		asyncFn := ctx.NewAsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
			resolve := promise.Get("resolve")
			defer resolve.Free()

			// Manually call resolve, then return undefined
			resolve.Execute(ctx.NewUndefined(), ctx.NewString("resolved by promise"))
			resolvedByPromise = true

			// Return undefined so the if !result.IsUndefined() branch is not executed
			return ctx.NewUndefined() // ADD missing 'return' keyword here
		})

		ctx.Globals().Set("testAsyncReturnUndefined", asyncFn)
		result := ctx.Eval(`testAsyncReturnUndefined()`, EvalAwait(true))
		defer result.Free()
		require.False(t, result.IsException())
		require.True(t, resolvedByPromise)
		require.Equal(t, "resolved by promise", result.ToString())
	})

	t.Run("AsyncFunctionComplexScenario", func(t *testing.T) {
		ctx := newCtx(t)
		// Test complex async function scenario to ensure complete coverage
		asyncFn := ctx.NewAsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
			resolve := promise.Get("resolve")
			reject := promise.Get("reject")
			defer resolve.Free()
			defer reject.Free()

			if len(args) == 0 {
				// Test reject without arguments (already covered in other tests)
				reject.Execute(ctx.NewUndefined())
				return ctx.NewUndefined()
			}

			command := args[0].ToString()
			switch command {
			case "resolve_no_args":
				// Cover resolve without arguments branch
				resolve.Execute(ctx.NewUndefined())
			case "reject_with_args":
				// Cover reject with arguments branch
				errObj := ctx.NewError(errors.New("custom rejection"))
				defer errObj.Free()
				reject.Execute(ctx.NewUndefined(), errObj)
			case "direct_return":
				// Cover direct return value branch
				return ctx.NewString("returned directly")
			default:
				// Default case
				resolve.Execute(ctx.NewUndefined(), ctx.NewString("default case"))
			}

			return ctx.NewUndefined()
		})

		ctx.Globals().Set("testAsyncComplex", asyncFn)

		// Test resolve without arguments
		result1 := ctx.Eval(`testAsyncComplex("resolve_no_args")`, EvalAwait(true))
		defer result1.Free()
		require.False(t, result1.IsException())
		require.True(t, result1.IsUndefined())

		// Test reject with arguments
		result2 := ctx.Eval(`testAsyncComplex("reject_with_args")`, EvalAwait(true))
		defer result2.Free()
		require.True(t, result2.IsException())

		err := ctx.Exception()
		require.Contains(t, err.Error(), "custom rejection")

		// Test direct return value
		result3 := ctx.Eval(`testAsyncComplex("direct_return")`, EvalAwait(true))
		defer result3.Free()
		require.False(t, result3.IsException())
		require.Equal(t, "returned directly", result3.ToString())
	})
}

// TestDeprecatedAPIs tests all deprecated methods to ensure they still work
// Each deprecated method is called once for test coverage
func TestDeprecatedAPIs(t *testing.T) {
	newCtx := func(t *testing.T) *Context {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("DeprecatedValueCreation", func(t *testing.T) {
		ctx := newCtx(t)
		// Test all deprecated value creation methods
		val1 := ctx.Null()
		defer val1.Free()
		require.True(t, val1.IsNull())

		val2 := ctx.Undefined()
		defer val2.Free()
		require.True(t, val2.IsUndefined())

		val3 := ctx.Uninitialized()
		defer val3.Free()
		require.True(t, val3.IsUninitialized())

		val4 := ctx.Bool(true)
		defer val4.Free()
		require.True(t, val4.IsBool())

		val5 := ctx.Int32(42)
		defer val5.Free()
		require.True(t, val5.IsNumber())

		val6 := ctx.Int64(1234567890)
		defer val6.Free()
		require.True(t, val6.IsNumber())

		val7 := ctx.Uint32(42)
		defer val7.Free()
		require.True(t, val7.IsNumber())

		val8 := ctx.BigInt64(9223372036854775807)
		defer val8.Free()
		require.True(t, val8.IsBigInt())

		val9 := ctx.BigUint64(18446744073709551615)
		defer val9.Free()
		require.True(t, val9.IsBigInt())

		val10 := ctx.Float64(3.14159)
		defer val10.Free()
		require.True(t, val10.IsNumber())

		val11 := ctx.String("test")
		defer val11.Free()
		require.True(t, val11.IsString())

		val12 := ctx.Object()
		defer val12.Free()
		require.True(t, val12.IsObject())

		val13 := ctx.ArrayBuffer([]byte{1, 2, 3})
		defer val13.Free()
		require.True(t, val13.IsByteArray())

		val14 := ctx.Error(errors.New("test error"))
		defer val14.Free()
		require.True(t, val14.IsError())
	})

	t.Run("DeprecatedTypedArrays", func(t *testing.T) {
		ctx := newCtx(t)
		// Test all deprecated TypedArray creation methods
		val1 := ctx.Int8Array([]int8{1, 2, 3})
		defer val1.Free()
		require.True(t, val1.IsInt8Array())

		val2 := ctx.Uint8Array([]uint8{1, 2, 3})
		defer val2.Free()
		require.True(t, val2.IsUint8Array())

		val3 := ctx.Uint8ClampedArray([]uint8{1, 2, 3})
		defer val3.Free()
		require.True(t, val3.IsUint8ClampedArray())

		val4 := ctx.Int16Array([]int16{1, 2, 3})
		defer val4.Free()
		require.True(t, val4.IsInt16Array())

		val5 := ctx.Uint16Array([]uint16{1, 2, 3})
		defer val5.Free()
		require.True(t, val5.IsUint16Array())

		val6 := ctx.Int32Array([]int32{1, 2, 3})
		defer val6.Free()
		require.True(t, val6.IsInt32Array())

		val7 := ctx.Uint32Array([]uint32{1, 2, 3})
		defer val7.Free()
		require.True(t, val7.IsUint32Array())

		val8 := ctx.Float32Array([]float32{1.0, 2.0, 3.0})
		defer val8.Free()
		require.True(t, val8.IsFloat32Array())

		val9 := ctx.Float64Array([]float64{1.0, 2.0, 3.0})
		defer val9.Free()
		require.True(t, val9.IsFloat64Array())

		val10 := ctx.BigInt64Array([]int64{1, 2, 3})
		defer val10.Free()
		require.True(t, val10.IsBigInt64Array())

		val11 := ctx.BigUint64Array([]uint64{1, 2, 3})
		defer val11.Free()
		require.True(t, val11.IsBigUint64Array())
	})

	t.Run("DeprecatedFunctions", func(t *testing.T) {
		ctx := newCtx(t)
		// Test deprecated Function method
		fn := ctx.Function(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewString("hello")
		})
		defer fn.Free()
		require.True(t, fn.IsFunction())

		// Test deprecated AsyncFunction method
		asyncFn := ctx.AsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
			resolve := promise.Get("resolve")
			defer resolve.Free()
			resolve.Execute(ctx.NewUndefined(), ctx.NewString("async hello"))
			return ctx.NewUndefined()
		})
		defer asyncFn.Free()
		require.True(t, asyncFn.IsFunction())

		// Test deprecated Promise method
		promise := ctx.Promise(func(resolve, reject func(*Value)) {
			resolve(ctx.NewString("promise result"))
		})
		defer promise.Free()
		require.True(t, promise.IsPromise())
	})

	t.Run("DeprecatedAtoms", func(t *testing.T) {
		ctx := newCtx(t)
		// Test deprecated Atom methods
		atom1 := ctx.Atom("test")
		defer atom1.Free()
		require.Equal(t, "test", atom1.ToString())

		atom2 := ctx.AtomIdx(123)
		defer atom2.Free()
		require.NotNil(t, atom2)
	})

	t.Run("DeprecatedInvoke", func(t *testing.T) {
		ctx := newCtx(t)
		// Test deprecated Invoke method
		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewString("invoked")
		})
		defer fn.Free()

		result := ctx.Invoke(fn, ctx.NewNull(), ctx.NewString("arg"))
		defer result.Free()
		require.Equal(t, "invoked", result.ToString())
	})
}
