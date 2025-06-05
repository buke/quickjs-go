package quickjs_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

func TestContextBasics(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Runtime() method
	require.NotNil(t, ctx.Runtime())

	// Test basic value creation
	t.Run("ValueCreation", func(t *testing.T) {
		values := []struct {
			name      string
			createVal func() quickjs.Value
			checkFunc func(quickjs.Value) bool
		}{
			{"Null", func() quickjs.Value { return ctx.Null() }, func(v quickjs.Value) bool { return v.IsNull() }},
			{"Undefined", func() quickjs.Value { return ctx.Undefined() }, func(v quickjs.Value) bool { return v.IsUndefined() }},
			{"Uninitialized", func() quickjs.Value { return ctx.Uninitialized() }, func(v quickjs.Value) bool { return v.IsUninitialized() }},
			{"Bool", func() quickjs.Value { return ctx.Bool(true) }, func(v quickjs.Value) bool { return v.IsBool() }},
			{"Int32", func() quickjs.Value { return ctx.Int32(-42) }, func(v quickjs.Value) bool { return v.IsNumber() }},
			{"Int64", func() quickjs.Value { return ctx.Int64(1234567890) }, func(v quickjs.Value) bool { return v.IsNumber() }},
			{"Uint32", func() quickjs.Value { return ctx.Uint32(42) }, func(v quickjs.Value) bool { return v.IsNumber() }},
			{"BigInt64", func() quickjs.Value { return ctx.BigInt64(9223372036854775807) }, func(v quickjs.Value) bool { return v.IsBigInt() }},
			{"BigUint64", func() quickjs.Value { return ctx.BigUint64(18446744073709551615) }, func(v quickjs.Value) bool { return v.IsBigInt() }},
			{"Float64", func() quickjs.Value { return ctx.Float64(3.14159) }, func(v quickjs.Value) bool { return v.IsNumber() }},
			{"String", func() quickjs.Value { return ctx.String("test") }, func(v quickjs.Value) bool { return v.IsString() }},
			{"Object", func() quickjs.Value { return ctx.Object() }, func(v quickjs.Value) bool { return v.IsObject() }},
		}

		for _, tc := range values {
			t.Run(tc.name, func(t *testing.T) {
				val := tc.createVal()
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
				ab := ctx.ArrayBuffer(data)
				defer ab.Free()
				require.True(t, ab.IsByteArray())
				require.EqualValues(t, len(data), ab.ByteLen())
			})
		}
	})
}

func TestContextEvaluation(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("BasicEvaluation", func(t *testing.T) {
		// Simple expression
		result, err := ctx.Eval(`1 + 2`)
		require.NoError(t, err)
		defer result.Free()
		require.EqualValues(t, 3, result.ToInt32())

		// Empty code
		result2, err := ctx.Eval(``)
		require.NoError(t, err)
		defer result2.Free()
	})

	t.Run("EvaluationOptions", func(t *testing.T) {
		optionTests := []struct {
			name    string
			code    string
			options []quickjs.EvalOption
		}{
			{"Strict", `"use strict"; var x = 42; x`, []quickjs.EvalOption{quickjs.EvalFlagStrict(true), quickjs.EvalFileName("test.js")}},
			{"Module", `export const x = 42;`, []quickjs.EvalOption{quickjs.EvalFlagModule(true)}},
			{"CompileOnly", `1 + 1`, []quickjs.EvalOption{quickjs.EvalFlagCompileOnly(true)}},
			{"GlobalFalse", `var globalFlagTest = "test"; globalFlagTest`, []quickjs.EvalOption{quickjs.EvalFlagGlobal(false)}},
			{"GlobalTrue", `var globalFlagTest2 = "test2"; globalFlagTest2`, []quickjs.EvalOption{quickjs.EvalFlagGlobal(true)}},
		}

		for _, tt := range optionTests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := ctx.Eval(tt.code, tt.options...)
				require.NoError(t, err)
				defer result.Free()
			})
		}
	})

	t.Run("EvaluationErrors", func(t *testing.T) {
		_, err := ctx.Eval(`invalid syntax {`)
		require.Error(t, err)
	})
}

func TestContextBytecodeOperations(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("BasicCompilation", func(t *testing.T) {
		code := `function add(a, b) { return a + b; } add(2, 3);`
		bytecode, err := ctx.Compile(code)
		require.NoError(t, err)
		require.NotEmpty(t, bytecode)

		// Execute bytecode
		result, err := ctx.EvalBytecode(bytecode)
		require.NoError(t, err)
		defer result.Free()
		require.EqualValues(t, 5, result.ToInt32())
	})

	t.Run("FileOperations", func(t *testing.T) {
		testFile := "./test_temp.js"
		testContent := `function multiply(a, b) { return a * b; } multiply(3, 4);`
		err := os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)
		defer os.Remove(testFile)

		// EvalFile with options
		resultFromFile, err := ctx.EvalFile(testFile, quickjs.EvalFlagStrict(true))
		require.NoError(t, err)
		defer resultFromFile.Free()
		require.EqualValues(t, 12, resultFromFile.ToInt32())

		// CompileFile tests
		bytecode, err := ctx.CompileFile(testFile)
		require.NoError(t, err)
		require.NotEmpty(t, bytecode)

		bytecode2, err := ctx.CompileFile(testFile, quickjs.EvalFileName("custom.js"))
		require.NoError(t, err)
		require.NotEmpty(t, bytecode2)
	})

	t.Run("ErrorCases", func(t *testing.T) {
		errorTests := []struct {
			name string
			test func() error
		}{
			{"EmptyBytecode", func() error { _, err := ctx.EvalBytecode([]byte{}); return err }},
			{"InvalidBytecode", func() error { _, err := ctx.EvalBytecode([]byte{0x01, 0x02, 0x03}); return err }},
			{"NonexistentFile", func() error { _, err := ctx.EvalFile("./nonexistent.js"); return err }},
			{"CompileNonexistentFile", func() error { _, err := ctx.CompileFile("./nonexistent.js"); return err }},
			{"CompilationError", func() error { _, err := ctx.Compile(`invalid syntax {`); return err }},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				require.Error(t, tt.test())
			})
		}

		// Exception during bytecode evaluation
		invalidCode := `throw new Error("test exception during evaluation");`
		invalidBytecode, err := ctx.Compile(invalidCode)
		require.NoError(t, err)

		_, err = ctx.EvalBytecode(invalidBytecode)
		require.Error(t, err)
		require.Contains(t, err.Error(), "test exception during evaluation")
	})

	t.Run("CompilationVariants", func(t *testing.T) {
		// Test empty code compilation
		bytecode, err := ctx.Compile(``)
		require.NoError(t, err)
		require.NotEmpty(t, bytecode)

		// Test normal function compilation
		normalCode := `(function() { return 42; })`
		r, e := ctx.Eval(normalCode)
		defer r.Free()
		require.NoError(t, e)

		bytecode, err = ctx.Compile(normalCode)
		require.NoError(t, err)
		require.NotEmpty(t, bytecode)

		result, err := ctx.EvalBytecode(bytecode)
		require.NoError(t, err)
		defer result.Free()
		require.True(t, result.IsFunction())
	})
}

func TestContextModules(t *testing.T) {
	rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	moduleCode := `export function add(a, b) { return a + b; }`

	t.Run("ModuleLoading", func(t *testing.T) {
		// Basic module loading
		result, err := ctx.LoadModule(moduleCode, "math_module")
		require.NoError(t, err)
		defer result.Free()

		// Module with load_only option
		result2, err := ctx.LoadModule(moduleCode, "math_module2", quickjs.EvalLoadOnly(true))
		require.NoError(t, err)
		defer result2.Free()
	})

	t.Run("ModuleBytecode", func(t *testing.T) {
		bytecode, err := ctx.Compile(moduleCode, quickjs.EvalFlagModule(true), quickjs.EvalFlagCompileOnly(true))
		require.NoError(t, err)

		// Basic bytecode loading
		result, err := ctx.LoadModuleBytecode(bytecode)
		require.NoError(t, err)
		defer result.Free()

		// Bytecode loading with load_only flag
		result2, err := ctx.LoadModuleBytecode(bytecode, quickjs.EvalLoadOnly(true))
		require.NoError(t, err)
		defer result2.Free()
	})

	t.Run("ModuleFiles", func(t *testing.T) {
		moduleFile := "./test_module.js"
		moduleContent := `export const value = 42;`
		err := os.WriteFile(moduleFile, []byte(moduleContent), 0644)
		require.NoError(t, err)
		defer os.Remove(moduleFile)

		// LoadModuleFile
		moduleResult, err := ctx.LoadModuleFile(moduleFile, "test_module")
		require.NoError(t, err)
		defer moduleResult.Free()

		// CompileModule tests
		compiledModule, err := ctx.CompileModule(moduleFile, "compiled_module")
		require.NoError(t, err)
		require.NotEmpty(t, compiledModule)

		compiledModule2, err := ctx.CompileModule(moduleFile, "compiled_module2", quickjs.EvalFlagStrict(true))
		require.NoError(t, err)
		require.NotEmpty(t, compiledModule2)
	})

	t.Run("ModuleErrors", func(t *testing.T) {
		errorTests := []struct {
			name string
			test func() error
		}{
			{"NotModule", func() error { _, err := ctx.LoadModule(`var x = 1; x;`, "not_module"); return err }},
			{"InvalidModule", func() error { _, err := ctx.LoadModule(`export { unclosed_brace`, "invalid_module"); return err }},
			{"EmptyBytecode", func() error { _, err := ctx.LoadModuleBytecode([]byte{}); return err }},
			{"InvalidBytecode", func() error { _, err := ctx.LoadModuleBytecode([]byte{0x01, 0x02, 0x03}); return err }},
			{"MissingFile", func() error { _, err := ctx.LoadModuleFile("./nonexistent_file.js", "missing"); return err }},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				require.Error(t, tt.test())
			})
		}
	})
}

func TestContextFunctions(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("RegularFunctions", func(t *testing.T) {
		fn := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			if len(args) == 0 {
				return ctx.String("no args")
			}
			return ctx.String("Hello " + args[0].String())
		})
		defer fn.Free()

		// Test function execution
		result := fn.Execute(ctx.Null())
		defer result.Free()
		require.EqualValues(t, "no args", result.String())

		result2 := fn.Execute(ctx.Null(), ctx.String("World"))
		defer result2.Free()
		require.EqualValues(t, "Hello World", result2.String())

		// Test Invoke method with different argument counts
		result3 := ctx.Invoke(fn, ctx.Null())
		defer result3.Free()
		require.EqualValues(t, "no args", result3.String())

		result4 := ctx.Invoke(fn, ctx.Null(), ctx.String("Test"))
		defer result4.Free()
		require.EqualValues(t, "Hello Test", result4.String())
	})

	// Updated: Use Function + Promise instead of AsyncFunction
	t.Run("AsyncFunctions", func(t *testing.T) {
		// New approach using Function + Promise
		asyncFn := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			return ctx.Promise(func(resolve, reject func(quickjs.Value)) {
				resolve(ctx.String("async result"))
			})
		})

		ctx.Globals().Set("testAsync", asyncFn)
		result, err := ctx.Eval(`testAsync()`, quickjs.EvalAwait(true))
		require.NoError(t, err)
		defer result.Free()
		require.EqualValues(t, "async result", result.String())
	})
}

func TestContextErrorHandling(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("ErrorCreation", func(t *testing.T) {
		testErr := errors.New("test error")
		errorVal := ctx.Error(testErr)
		defer errorVal.Free()
		require.True(t, errorVal.IsError())
	})

	t.Run("ThrowMethods", func(t *testing.T) {
		throwTests := []struct {
			name     string
			throwFn  func() quickjs.Value
			errorStr string
		}{
			{"ThrowError", func() quickjs.Value { return ctx.ThrowError(errors.New("custom error")) }, "custom error"},
			{"ThrowSyntax", func() quickjs.Value { return ctx.ThrowSyntaxError("syntax: %s", "invalid") }, "SyntaxError"},
			{"ThrowType", func() quickjs.Value { return ctx.ThrowTypeError("type error") }, "TypeError"},
			{"ThrowReference", func() quickjs.Value { return ctx.ThrowReferenceError("ref error") }, "ReferenceError"},
			{"ThrowRange", func() quickjs.Value { return ctx.ThrowRangeError("range error") }, "RangeError"},
			{"ThrowInternal", func() quickjs.Value { return ctx.ThrowInternalError("internal error") }, "InternalError"},
		}

		for _, tt := range throwTests {
			t.Run(tt.name, func(t *testing.T) {
				throwingFunc := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
					return tt.throwFn()
				})
				defer throwingFunc.Free()

				result := throwingFunc.Execute(ctx.Null())
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

	t.Run("ExceptionHandling", func(t *testing.T) {
		// Test Exception() when no exception
		exception := ctx.Exception()
		require.Nil(t, exception)
		require.False(t, ctx.HasException())
	})
}

func TestContextUtilities(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("Globals", func(t *testing.T) {
		// Test globals caching
		globals1 := ctx.Globals()
		globals2 := ctx.Globals()
		require.True(t, globals1.IsObject())
		require.True(t, globals2.IsObject())

		// Test global variable operations
		globals1.Set("testGlobal", ctx.String("global value"))
		retrieved := globals2.Get("testGlobal")
		defer retrieved.Free()
		require.EqualValues(t, "global value", retrieved.String())
	})

	t.Run("JSONParsing", func(t *testing.T) {
		// Valid JSON
		jsonObj := ctx.ParseJSON(`{"name": "test", "value": 42}`)
		defer jsonObj.Free()
		require.True(t, jsonObj.IsObject())

		nameVal := jsonObj.Get("name")
		defer nameVal.Free()
		require.EqualValues(t, "test", nameVal.String())

		// Invalid JSON
		invalidJSON := ctx.ParseJSON(`{invalid}`)
		defer invalidJSON.Free()
		require.True(t, invalidJSON.IsException())
	})

	t.Run("InterruptHandler", func(t *testing.T) {
		interruptCalled := false
		ctx.SetInterruptHandler(func() int {
			interruptCalled = true
			return 1 // Interrupt
		})

		_, err := ctx.Eval(`while(true){}`)
		require.Error(t, err)
		require.Contains(t, err.Error(), "interrupted")
		require.True(t, interruptCalled)
	})
}

func TestContextAsync(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("EventLoop", func(t *testing.T) {
		result, err := ctx.Eval(`
            var executed = false;
            setTimeout(() => { executed = true; }, 10);
        `)
		require.NoError(t, err)
		defer result.Free()

		ctx.Loop()

		executedResult, err := ctx.Eval(`executed`)
		require.NoError(t, err)
		defer executedResult.Free()
		require.True(t, executedResult.ToBool())
	})

	// Updated: Use Function + Promise instead of AsyncFunction
	t.Run("AwaitPromises", func(t *testing.T) {
		// Test successful promise using new Promise API
		asyncTestFn := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			return ctx.Promise(func(resolve, reject func(quickjs.Value)) {
				resolve(ctx.String("awaited result"))
			})
		})
		ctx.Globals().Set("asyncTest", asyncTestFn)

		promiseResult, err := ctx.Eval(`asyncTest()`)
		require.NoError(t, err)
		require.True(t, promiseResult.IsPromise())

		awaitedResult, err := ctx.Await(promiseResult)
		require.NoError(t, err)
		defer awaitedResult.Free()
		require.EqualValues(t, "awaited result", awaitedResult.String())

		// Test rejected promise using new Promise API
		asyncRejectFn := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			return ctx.Promise(func(resolve, reject func(quickjs.Value)) {
				errorObj := ctx.Error(errors.New("rejection reason"))
				defer errorObj.Free()
				reject(errorObj)
			})
		})
		ctx.Globals().Set("asyncReject", asyncRejectFn)

		rejectPromise, err := ctx.Eval(`asyncReject()`)
		require.NoError(t, err)

		_, err = ctx.Await(rejectPromise)
		require.Error(t, err)
	})
}

func TestContextPromise(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("BasicPromise", func(t *testing.T) {
		// Test immediate resolve
		promise := ctx.Promise(func(resolve, reject func(quickjs.Value)) {
			resolve(ctx.String("success"))
		})

		require.True(t, promise.IsPromise())
		require.Equal(t, quickjs.PromiseFulfilled, promise.PromiseState())

		result, err := promise.Await()
		require.NoError(t, err)
		defer result.Free()
		require.Equal(t, "success", result.String())
	})

	t.Run("RejectedPromise", func(t *testing.T) {
		promise := ctx.Promise(func(resolve, reject func(quickjs.Value)) {
			errorObj := ctx.Error(errors.New("error"))
			defer errorObj.Free()
			reject(errorObj)
		})

		require.True(t, promise.IsPromise())

		state := promise.PromiseState()
		require.Equal(t, quickjs.PromiseRejected, state)

		_, err := promise.Await()
		require.Error(t, err)
		require.Contains(t, err.Error(), "error")
	})

	t.Run("PromiseFunction", func(t *testing.T) {
		// Create function that returns Promise
		asyncFn := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			return ctx.Promise(func(resolve, reject func(quickjs.Value)) {
				if len(args) == 0 {
					errObj := ctx.Error(errors.New("no arguments provided"))
					defer errObj.Free()
					reject(errObj)
					return
				}
				resolve(ctx.String("Hello " + args[0].String()))
			})
		})

		// Test in JavaScript
		global := ctx.Globals()
		global.Set("asyncGreet", asyncFn)

		// Test with argument
		result1, err := ctx.Eval(`asyncGreet("World")`)
		require.NoError(t, err)

		final1, err := result1.Await()
		require.NoError(t, err)
		require.Equal(t, "Hello World", final1.String())

		// Test without argument (should reject)
		result2, err := ctx.Eval(`asyncGreet()`)
		require.NoError(t, err)

		_, err = result2.Await()
		require.Error(t, err)
	})

	t.Run("PromiseChaining", func(t *testing.T) {
		// Create async function for chaining
		asyncDouble := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			return ctx.Promise(func(resolve, reject func(quickjs.Value)) {
				if len(args) == 0 {
					errObj := ctx.Error(errors.New("no number provided"))
					defer errObj.Free()
					reject(errObj)
					return
				}
				value := args[0].ToInt32()
				resolve(ctx.Int32(value * 2))
			})
		})

		global := ctx.Globals()
		global.Set("asyncDouble", asyncDouble)

		// Test promise chaining
		result, err := ctx.Eval(`
            asyncDouble(5)
                .then(x => asyncDouble(x))
                .then(x => x + 10)
        `)
		require.NoError(t, err)

		final, err := result.Await()
		require.NoError(t, err)
		defer final.Free()
		require.Equal(t, int32(30), final.ToInt32()) // 5 * 2 * 2 + 10 = 30
	})

	t.Run("PromiseState", func(t *testing.T) {
		// Test different promise states
		pendingPromise, err := ctx.Eval(`new Promise(() => {})`) // Never resolves
		require.NoError(t, err)
		defer pendingPromise.Free()
		require.Equal(t, quickjs.PromisePending, pendingPromise.PromiseState())

		fulfilledPromise, err := ctx.Eval(`Promise.resolve("fulfilled")`)
		require.NoError(t, err)
		defer fulfilledPromise.Free()
		require.Equal(t, quickjs.PromiseFulfilled, fulfilledPromise.PromiseState())

		rejectedPromise, err := ctx.Eval(`Promise.reject("rejected")`)
		require.NoError(t, err)
		defer rejectedPromise.Free()
		require.Equal(t, quickjs.PromiseRejected, rejectedPromise.PromiseState())

		// Test PromiseState on non-Promise
		nonPromise := ctx.String("not a promise")
		defer nonPromise.Free()
		require.Equal(t, quickjs.PromisePending, nonPromise.PromiseState()) // Should return default
	})

	t.Run("ValueAwait", func(t *testing.T) {
		// Test Value.Await() method
		promise := ctx.Promise(func(resolve, reject func(quickjs.Value)) {
			resolve(ctx.String("awaited via Value.Await"))
		})

		result, err := promise.Await()
		require.NoError(t, err)
		require.Equal(t, "awaited via Value.Await", result.String())

		// Test Await on non-Promise (should return equivalent value)
		nonPromise := ctx.String("not a promise")

		result2, err := nonPromise.Await()
		require.NoError(t, err)
		defer result2.Free()

		// Verify the content is the same
		require.Equal(t, nonPromise.String(), result2.String())
		require.Equal(t, "not a promise", result2.String())

		// Verify it's still a string
		require.True(t, result2.IsString())
	})

	t.Run("ComplexAsync", func(t *testing.T) {
		// Test more complex async scenario
		asyncProcessor := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			return ctx.Promise(func(resolve, reject func(quickjs.Value)) {
				if len(args) == 0 {
					errObj := ctx.Error(errors.New("no data to process"))
					defer errObj.Free()
					reject(errObj)
					return
				}

				// Simulate processing
				input := args[0].String()
				if input == "error" {
					errObj := ctx.Error(errors.New("processing failed"))
					defer errObj.Free()
					reject(errObj)
					return
				}

				result := ctx.String("processed: " + input)
				resolve(result)
			})
		})

		global := ctx.Globals()
		global.Set("process", asyncProcessor)

		// Test successful processing
		success, err := ctx.Eval(`process("data").then(result => "Success: " + result)`)
		require.NoError(t, err)

		successResult, err := success.Await()
		require.NoError(t, err)
		defer successResult.Free()
		require.Equal(t, "Success: processed: data", successResult.String())

		// Test error handling
		errorCase, err := ctx.Eval(`process("error").catch(err =>  err)`)
		require.NoError(t, err)

		errorResult, err := errorCase.Await()
		require.NoError(t, err)
		defer errorResult.Free()
		require.Equal(t, "Error: processing failed", errorResult.String())
	})
}

func TestContextTypedArrays(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("TypedArrayCreation", func(t *testing.T) {
		// Test all TypedArray creation methods
		typedArrayTests := []struct {
			name       string
			createFunc func() quickjs.Value
			checkFunc  func(quickjs.Value) bool
			testEmpty  func() quickjs.Value
			testNil    func() quickjs.Value
		}{
			{
				"Int8Array",
				func() quickjs.Value { return ctx.Int8Array([]int8{-128, -1, 0, 1, 127}) },
				func(v quickjs.Value) bool { return v.IsInt8Array() },
				func() quickjs.Value { return ctx.Int8Array([]int8{}) },
				func() quickjs.Value { return ctx.Int8Array(nil) },
			},
			{
				"Uint8Array",
				func() quickjs.Value { return ctx.Uint8Array([]uint8{0, 1, 128, 255}) },
				func(v quickjs.Value) bool { return v.IsUint8Array() },
				func() quickjs.Value { return ctx.Uint8Array([]uint8{}) },
				func() quickjs.Value { return ctx.Uint8Array(nil) },
			},
			{
				"Uint8ClampedArray",
				func() quickjs.Value { return ctx.Uint8ClampedArray([]uint8{0, 127, 255}) },
				func(v quickjs.Value) bool { return v.IsUint8ClampedArray() },
				func() quickjs.Value { return ctx.Uint8ClampedArray([]uint8{}) },
				func() quickjs.Value { return ctx.Uint8ClampedArray(nil) },
			},
			{
				"Int16Array",
				func() quickjs.Value { return ctx.Int16Array([]int16{-32768, -1, 0, 1, 32767}) },
				func(v quickjs.Value) bool { return v.IsInt16Array() },
				func() quickjs.Value { return ctx.Int16Array([]int16{}) },
				func() quickjs.Value { return ctx.Int16Array(nil) },
			},
			{
				"Uint16Array",
				func() quickjs.Value { return ctx.Uint16Array([]uint16{0, 1, 32768, 65535}) },
				func(v quickjs.Value) bool { return v.IsUint16Array() },
				func() quickjs.Value { return ctx.Uint16Array([]uint16{}) },
				func() quickjs.Value { return ctx.Uint16Array(nil) },
			},
			{
				"Int32Array",
				func() quickjs.Value { return ctx.Int32Array([]int32{-2147483648, -1, 0, 1, 2147483647}) },
				func(v quickjs.Value) bool { return v.IsInt32Array() },
				func() quickjs.Value { return ctx.Int32Array([]int32{}) },
				func() quickjs.Value { return ctx.Int32Array(nil) },
			},
			{
				"Uint32Array",
				func() quickjs.Value { return ctx.Uint32Array([]uint32{0, 1, 2147483648, 4294967295}) },
				func(v quickjs.Value) bool { return v.IsUint32Array() },
				func() quickjs.Value { return ctx.Uint32Array([]uint32{}) },
				func() quickjs.Value { return ctx.Uint32Array(nil) },
			},
			{
				"Float32Array",
				func() quickjs.Value { return ctx.Float32Array([]float32{-3.14, 0.0, 1.5, 3.14159}) },
				func(v quickjs.Value) bool { return v.IsFloat32Array() },
				func() quickjs.Value { return ctx.Float32Array([]float32{}) },
				func() quickjs.Value { return ctx.Float32Array(nil) },
			},
			{
				"Float64Array",
				func() quickjs.Value {
					return ctx.Float64Array([]float64{-3.141592653589793, 0.0, 1.5, 3.141592653589793})
				},
				func(v quickjs.Value) bool { return v.IsFloat64Array() },
				func() quickjs.Value { return ctx.Float64Array([]float64{}) },
				func() quickjs.Value { return ctx.Float64Array(nil) },
			},
			{
				"BigInt64Array",
				func() quickjs.Value {
					return ctx.BigInt64Array([]int64{-9223372036854775808, -1, 0, 1, 9223372036854775807})
				},
				func(v quickjs.Value) bool { return v.IsBigInt64Array() },
				func() quickjs.Value { return ctx.BigInt64Array([]int64{}) },
				func() quickjs.Value { return ctx.BigInt64Array(nil) },
			},
			{
				"BigUint64Array",
				func() quickjs.Value {
					return ctx.BigUint64Array([]uint64{0, 1, 9223372036854775808, 18446744073709551615})
				},
				func(v quickjs.Value) bool { return v.IsBigUint64Array() },
				func() quickjs.Value { return ctx.BigUint64Array([]uint64{}) },
				func() quickjs.Value { return ctx.BigUint64Array(nil) },
			},
		}

		for _, tt := range typedArrayTests {
			t.Run(tt.name, func(t *testing.T) {
				// Test with data
				arr := tt.createFunc()
				defer arr.Free()
				require.True(t, arr.IsTypedArray())
				require.True(t, tt.checkFunc(arr))

				// Test empty array
				emptyArr := tt.testEmpty()
				defer emptyArr.Free()
				require.True(t, tt.checkFunc(emptyArr))
				require.EqualValues(t, 0, emptyArr.Len())

				// Test nil slice
				nilArr := tt.testNil()
				defer nilArr.Free()
				require.True(t, tt.checkFunc(nilArr))
				require.EqualValues(t, 0, nilArr.Len())
			})
		}
	})

	t.Run("TypedArrayInterop", func(t *testing.T) {
		// Go to JavaScript
		goData := []int32{1, 2, 3, 4, 5}
		goArray := ctx.Int32Array(goData)
		ctx.Globals().Set("goArray", goArray)

		result, err := ctx.Eval(`
            let sum = 0;
            for (let i = 0; i < goArray.length; i++) {
                sum += goArray[i];
            }
            sum;
        `)
		require.NoError(t, err)
		defer result.Free()
		require.EqualValues(t, 15, result.ToInt32()) // 1+2+3+4+5 = 15

		// JavaScript to Go
		jsArray, err := ctx.Eval(`new Int32Array([10, 20, 30, 40, 50])`)
		require.NoError(t, err)
		defer jsArray.Free()

		require.True(t, jsArray.IsTypedArray())
		require.True(t, jsArray.IsInt32Array())

		goSlice, err := jsArray.ToInt32Array()
		require.NoError(t, err)
		require.Equal(t, []int32{10, 20, 30, 40, 50}, goSlice)
	})

	t.Run("TypedArrayPrecision", func(t *testing.T) {
		// Test Float32 precision
		float32Data := []float32{3.14159265359, -2.718281828, 0.0, 1.23456789}
		float32Array := ctx.Float32Array(float32Data)
		defer float32Array.Free()

		converted32, err := float32Array.ToFloat32Array()
		require.NoError(t, err)
		require.Len(t, converted32, len(float32Data))

		for i, expected := range float32Data {
			require.InDelta(t, expected, converted32[i], 0.0001)
		}

		// Test Float64 precision
		float64Data := []float64{3.141592653589793, -2.718281828459045, 0.0, 1.2345678901234567}
		float64Array := ctx.Float64Array(float64Data)
		defer float64Array.Free()

		converted64, err := float64Array.ToFloat64Array()
		require.NoError(t, err)
		require.Len(t, converted64, len(float64Data))

		for i, expected := range float64Data {
			require.InDelta(t, expected, converted64[i], 0.000000000001)
		}
	})

	t.Run("TypedArrayErrors", func(t *testing.T) {
		// Test conversion errors for wrong types
		wrongTypeVal := ctx.String("not a typed array")
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

		for i, testFn := range conversionTests {
			t.Run(fmt.Sprintf("ConversionError%d", i), func(t *testing.T) {
				require.Error(t, testFn())
			})
		}

		// Test type mismatch conversion
		int8Array := ctx.Int8Array([]int8{1, 2, 3})
		defer int8Array.Free()

		_, err := int8Array.ToUint8Array()
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a Uint8Array")
	})

	t.Run("SharedMemoryTest", func(t *testing.T) {
		// Test that TypedArrays share memory with their underlying ArrayBuffer
		data := []uint8{1, 2, 3, 4, 5, 6, 7, 8}
		arrayBuffer := ctx.ArrayBuffer(data)
		ctx.Globals().Set("sharedBuffer", arrayBuffer)

		// Create different views on the same buffer
		ret, err := ctx.Eval(`
            globalThis.uint8View = new Uint8Array(sharedBuffer);
            globalThis.uint16View = new Uint16Array(sharedBuffer);
        `)
		defer ret.Free()
		require.NoError(t, err)

		// Modify through uint8 view
		modifyResult, err := ctx.Eval(`uint8View[0] = 255;`)
		require.NoError(t, err)
		defer modifyResult.Free() // Fixed: Added missing defer Free()

		// Verify change is visible through uint16 view (shared memory)
		uint16Value, err := ctx.Eval(`uint16View[0]`)
		require.NoError(t, err)
		defer uint16Value.Free()

		// The uint16 value should have changed because we modified the underlying byte
		// Original: bytes [1, 2] -> uint16: 513 (little-endian: 1 + 2*256)
		// Modified: bytes [255, 2] -> uint16: 767 (little-endian: 255 + 2*256)
		require.EqualValues(t, 767, uint16Value.ToInt32())

		// Clean up
		cleanupResult, err := ctx.Eval(`delete globalThis.uint8View; delete globalThis.uint16View;`)
		require.NoError(t, err)
		defer cleanupResult.Free() // Fixed: Added missing defer Free()
	})
}

func TestContextMemoryPressure(t *testing.T) {
	// Test extreme memory pressure to trigger compilation failures
	rt := quickjs.NewRuntime(quickjs.WithMemoryLimit(32 * 1024)) // 32KB limit
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Fill memory first
	memoryResult, err := ctx.Eval(`
        var memoryFiller = [];
        try {
            for(let i = 0; i < 1000; i++) {
                memoryFiller.push(new Array(100).fill('x'.repeat(50)));
            }
        } catch(e) {
            // Expected to fail due to memory limit
        }
    `)
	if err == nil {
		defer memoryResult.Free() // Fixed: Added defer Free() for successful evaluation
	}

	// Try to compile - this should fail at JS_WriteObject due to no available memory
	_, err = ctx.Compile(`
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
