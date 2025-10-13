package quickjs

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContextBasics(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Runtime() method
	require.NotNil(t, ctx.Runtime())

	// Test basic value creation
	t.Run("ValueCreation", func(t *testing.T) {
		values := []struct {
			name      string
			createVal func() *Value     // Changed to return pointer
			checkFunc func(*Value) bool // Changed parameter to pointer
		}{
			{"Null", func() *Value { return ctx.NewNull() }, func(v *Value) bool { return v.IsNull() }},
			{"Undefined", func() *Value { return ctx.NewUndefined() }, func(v *Value) bool { return v.IsUndefined() }},
			{"Uninitialized", func() *Value { return ctx.NewUninitialized() }, func(v *Value) bool { return v.IsUninitialized() }},
			{"Bool", func() *Value { return ctx.NewBool(true) }, func(v *Value) bool { return v.IsBool() }},
			{"Int32", func() *Value { return ctx.NewInt32(-42) }, func(v *Value) bool { return v.IsNumber() }},
			{"Int64", func() *Value { return ctx.NewInt64(1234567890) }, func(v *Value) bool { return v.IsNumber() }},
			{"Uint32", func() *Value { return ctx.NewUint32(42) }, func(v *Value) bool { return v.IsNumber() }},
			{"BigInt64", func() *Value { return ctx.NewBigInt64(9223372036854775807) }, func(v *Value) bool { return v.IsBigInt() }},
			{"BigUint64", func() *Value { return ctx.NewBigUint64(18446744073709551615) }, func(v *Value) bool { return v.IsBigInt() }},
			{"Float64", func() *Value { return ctx.NewFloat64(3.14159) }, func(v *Value) bool { return v.IsNumber() }},
			{"String", func() *Value { return ctx.NewString("test") }, func(v *Value) bool { return v.IsString() }},
			{"Object", func() *Value { return ctx.NewObject() }, func(v *Value) bool { return v.IsObject() }},
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
				ab := ctx.NewArrayBuffer(data)
				defer ab.Free()
				require.True(t, ab.IsByteArray())
				require.EqualValues(t, len(data), ab.ByteLen())
			})
		}
	})
}

func TestContextEvaluation(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("BasicEvaluation", func(t *testing.T) {
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
				result := ctx.Eval(tt.code, tt.options...)
				defer result.Free()
				require.False(t, result.IsException())
			})
		}
	})

	t.Run("EvaluationErrors", func(t *testing.T) {
		result := ctx.Eval(`invalid syntax {`)
		defer result.Free()
		require.True(t, result.IsException())

		err := ctx.Exception()
		require.Error(t, err)
	})
}

func TestContextBytecodeOperations(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("BasicCompilation", func(t *testing.T) {
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
		errorTests := []struct {
			name string
			test func() bool // Changed to return bool indicating if exception occurred
		}{
			{"EmptyBytecode", func() bool {
				result := ctx.EvalBytecode([]byte{})
				defer result.Free()
				return result.IsException()
			}},
			{"InvalidBytecode", func() bool {
				result := ctx.EvalBytecode([]byte{0x01, 0x02, 0x03})
				defer result.Free()
				return result.IsException()
			}},
			{"NonexistentFile", func() bool {
				result := ctx.EvalFile("./nonexistent.js")
				defer result.Free()
				return result.IsException()
			}},
			{"CompileNonexistentFile", func() bool {
				_, err := ctx.CompileFile("./nonexistent.js")
				return err != nil
			}},
			{"CompilationError", func() bool {
				_, err := ctx.Compile(`invalid syntax {`)
				return err != nil
			}},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				require.True(t, tt.test())
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
	rt := NewRuntime(WithModuleImport(true))
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	moduleCode := `export function add(a, b) { return a + b; }`

	t.Run("ModuleLoading", func(t *testing.T) {
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
			test func() bool // Changed to return bool indicating if exception occurred
		}{
			{"NotModule", func() bool {
				result := ctx.LoadModule(`var x = 1; x;`, "not_module")
				defer result.Free()
				return result.IsException()
			}},
			{"InvalidModule", func() bool {
				result := ctx.LoadModule(`export { unclosed_brace`, "invalid_module")
				defer result.Free()
				return result.IsException()
			}},
			{"EmptyBytecode", func() bool {
				result := ctx.LoadModuleBytecode([]byte{})
				defer result.Free()
				return result.IsException()
			}},
			{"InvalidBytecode", func() bool {
				result := ctx.LoadModuleBytecode([]byte{0x01, 0x02, 0x03})
				defer result.Free()
				return result.IsException()
			}},
			{"MissingFile", func() bool {
				result := ctx.LoadModuleFile("./nonexistent_file.js", "missing")
				defer result.Free()
				return result.IsException()
			}},
			{"ModuleThrowsError", func() bool {
				result := ctx.LoadModule(`export default 123; throw new Error('aah')`, "mod")
				defer result.Free()
				return result.IsException()
			}},
			{"ModuleUndefinedVariable", func() bool {
				result := ctx.LoadModule(`export default 123; blah`, "mod")
				defer result.Free()
				return result.IsException()
			}},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				require.True(t, tt.test())
			})
		}
	})

}

func TestContextFunctions(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("RegularFunctions", func(t *testing.T) {
		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			if len(args) == 0 {
				return ctx.NewString("no args")
			}
			return ctx.NewString("Hello " + args[0].ToString())
		})
		defer fn.Free()

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

	// Updated: Use Function + Promise instead of AsyncFunction
	t.Run("AsyncFunctions", func(t *testing.T) {
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

	t.Run("ExceptionHandling", func(t *testing.T) {
		// Test Exception() when no exception
		exception := ctx.Exception()
		require.Nil(t, exception)
		require.False(t, ctx.HasException())
	})
}

func TestContextUtilities(t *testing.T) {
	rt := NewRuntime()
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
		globals1.Set("testGlobal", ctx.NewString("global value"))
		retrieved := globals2.Get("testGlobal")
		defer retrieved.Free()
		require.EqualValues(t, "global value", retrieved.ToString())
	})

	t.Run("JSONParsing", func(t *testing.T) {
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
	})

	t.Run("InterruptHandler", func(t *testing.T) {
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

func TestContextAsync(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("EventLoop", func(t *testing.T) {
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
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("BasicPromise", func(t *testing.T) {
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
}

func TestContextTypedArrays(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("TypedArrayCreation", func(t *testing.T) {
		// Test all TypedArray creation methods
		typedArrayTests := []struct {
			name       string
			createFunc func() *Value     // Changed to return pointer
			checkFunc  func(*Value) bool // Changed parameter to pointer
			testEmpty  func() *Value     // Changed to return pointer
			testNil    func() *Value     // Changed to return pointer
		}{
			{
				"Int8Array",
				func() *Value { return ctx.NewInt8Array([]int8{-128, -1, 0, 1, 127}) },
				func(v *Value) bool { return v.IsInt8Array() },
				func() *Value { return ctx.NewInt8Array([]int8{}) },
				func() *Value { return ctx.NewInt8Array(nil) },
			},
			{
				"Uint8Array",
				func() *Value { return ctx.NewUint8Array([]uint8{0, 1, 128, 255}) },
				func(v *Value) bool { return v.IsUint8Array() },
				func() *Value { return ctx.NewUint8Array([]uint8{}) },
				func() *Value { return ctx.NewUint8Array(nil) },
			},
			{
				"Uint8ClampedArray",
				func() *Value { return ctx.NewUint8ClampedArray([]uint8{0, 127, 255}) },
				func(v *Value) bool { return v.IsUint8ClampedArray() },
				func() *Value { return ctx.NewUint8ClampedArray([]uint8{}) },
				func() *Value { return ctx.NewUint8ClampedArray(nil) },
			},
			{
				"Int16Array",
				func() *Value { return ctx.NewInt16Array([]int16{-32768, -1, 0, 1, 32767}) },
				func(v *Value) bool { return v.IsInt16Array() },
				func() *Value { return ctx.NewInt16Array([]int16{}) },
				func() *Value { return ctx.NewInt16Array(nil) },
			},
			{
				"Uint16Array",
				func() *Value { return ctx.NewUint16Array([]uint16{0, 1, 32768, 65535}) },
				func(v *Value) bool { return v.IsUint16Array() },
				func() *Value { return ctx.NewUint16Array([]uint16{}) },
				func() *Value { return ctx.NewUint16Array(nil) },
			},
			{
				"Int32Array",
				func() *Value { return ctx.NewInt32Array([]int32{-2147483648, -1, 0, 1, 2147483647}) },
				func(v *Value) bool { return v.IsInt32Array() },
				func() *Value { return ctx.NewInt32Array([]int32{}) },
				func() *Value { return ctx.NewInt32Array(nil) },
			},
			{
				"Uint32Array",
				func() *Value { return ctx.NewUint32Array([]uint32{0, 1, 2147483648, 4294967295}) },
				func(v *Value) bool { return v.IsUint32Array() },
				func() *Value { return ctx.NewUint32Array([]uint32{}) },
				func() *Value { return ctx.NewUint32Array(nil) },
			},
			{
				"Float32Array",
				func() *Value { return ctx.NewFloat32Array([]float32{-3.14, 0.0, 1.5, 3.14159}) },
				func(v *Value) bool { return v.IsFloat32Array() },
				func() *Value { return ctx.NewFloat32Array([]float32{}) },
				func() *Value { return ctx.NewFloat32Array(nil) },
			},
			{
				"Float64Array",
				func() *Value {
					return ctx.NewFloat64Array([]float64{-3.141592653589793, 0.0, 1.5, 3.141592653589793})
				},
				func(v *Value) bool { return v.IsFloat64Array() },
				func() *Value { return ctx.NewFloat64Array([]float64{}) },
				func() *Value { return ctx.NewFloat64Array(nil) },
			},
			{
				"BigInt64Array",
				func() *Value {
					return ctx.NewBigInt64Array([]int64{-9223372036854775808, -1, 0, 1, 9223372036854775807})
				},
				func(v *Value) bool { return v.IsBigInt64Array() },
				func() *Value { return ctx.NewBigInt64Array([]int64{}) },
				func() *Value { return ctx.NewBigInt64Array(nil) },
			},
			{
				"BigUint64Array",
				func() *Value {
					return ctx.NewBigUint64Array([]uint64{0, 1, 9223372036854775808, 18446744073709551615})
				},
				func(v *Value) bool { return v.IsBigUint64Array() },
				func() *Value { return ctx.NewBigUint64Array([]uint64{}) },
				func() *Value { return ctx.NewBigUint64Array(nil) },
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

		for i, testFn := range conversionTests {
			t.Run(fmt.Sprintf("ConversionError%d", i), func(t *testing.T) {
				require.Error(t, testFn())
			})
		}

		// Test type mismatch conversion
		int8Array := ctx.NewInt8Array([]int8{1, 2, 3})
		defer int8Array.Free()

		_, err := int8Array.ToUint8Array()
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a Uint8Array")
	})

	t.Run("SharedMemoryTest", func(t *testing.T) {
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
	rt := NewRuntime(WithMemoryLimit(128 * 1024)) // 128KB limit
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
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("AsyncFunctionResolveNoArgs", func(t *testing.T) {
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
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("DeprecatedValueCreation", func(t *testing.T) {
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
		// Test deprecated Atom methods
		atom1 := ctx.Atom("test")
		defer atom1.Free()
		require.Equal(t, "test", atom1.ToString())

		atom2 := ctx.AtomIdx(123)
		defer atom2.Free()
		require.NotNil(t, atom2)
	})

	t.Run("DeprecatedInvoke", func(t *testing.T) {
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
