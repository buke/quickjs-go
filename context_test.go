package quickjs_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestContextRuntime tests getting the runtime from context.
func TestContextRuntime(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test that context returns the correct runtime
	contextRuntime := ctx.Runtime()
	require.NotNil(t, contextRuntime)
	// We can't directly compare runtime pointers, but we can test they work together

	result, err := ctx.Eval(`"runtime test"`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "runtime test", result.String())
}

// TestContextClose tests proper context cleanup.
func TestContextClose(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()

	// Execute some code before closing
	result, err := ctx.Eval(`"before close"`)
	require.NoError(t, err)
	require.EqualValues(t, "before close", result.String())
	result.Free()

	// Close context
	ctx.Close()

	// After closing, we can't use the context anymore
	// This test mainly ensures no crashes occur during cleanup
}

// TestContextValueCreation tests all value creation methods.
func TestContextValueCreation(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Null
	nullVal := ctx.Null()
	defer nullVal.Free()
	require.True(t, nullVal.IsNull())

	// Test Undefined
	undefinedVal := ctx.Undefined()
	defer undefinedVal.Free()
	require.True(t, undefinedVal.IsUndefined())

	// Test Uninitialized
	uninitVal := ctx.Uninitialized()
	defer uninitVal.Free()
	require.True(t, uninitVal.IsUninitialized())

	// Test Bool
	boolVal := ctx.Bool(true)
	defer boolVal.Free()
	require.True(t, boolVal.IsBool())
	require.True(t, boolVal.ToBool())

	boolValFalse := ctx.Bool(false)
	defer boolValFalse.Free()
	require.False(t, boolValFalse.ToBool())

	// Test Int32
	int32Val := ctx.Int32(42)
	defer int32Val.Free()
	require.True(t, int32Val.IsNumber())
	require.EqualValues(t, 42, int32Val.ToInt32())

	// Test Int64 with a safer value that fits in JavaScript's Number range
	int64Val := ctx.Int64(1234567890123456)
	defer int64Val.Free()
	require.True(t, int64Val.IsNumber())
	require.EqualValues(t, 1234567890123456, int64Val.ToInt64())

	// Test Uint32
	uint32Val := ctx.Uint32(4294967295)
	defer uint32Val.Free()
	require.True(t, uint32Val.IsNumber())
	require.EqualValues(t, 4294967295, uint32Val.ToUint32())

	// Test BigInt64 with the max value (BigInt should handle this properly)
	bigInt64Val := ctx.BigInt64(9223372036854775807)
	defer bigInt64Val.Free()
	require.True(t, bigInt64Val.IsBigInt())

	// Test BigUint64 with a safer value
	bigUint64Val := ctx.BigUint64(18446744073709551615)
	defer bigUint64Val.Free()
	require.True(t, bigUint64Val.IsBigInt())

	// Test Float64
	float64Val := ctx.Float64(3.14159)
	defer float64Val.Free()
	require.True(t, float64Val.IsNumber())
	require.InDelta(t, 3.14159, float64Val.ToFloat64(), 0.00001)

	// Test String
	stringVal := ctx.String("Hello World")
	defer stringVal.Free()
	require.True(t, stringVal.IsString())
	require.EqualValues(t, "Hello World", stringVal.String())

	// Test empty string
	emptyStringVal := ctx.String("")
	defer emptyStringVal.Free()
	require.True(t, emptyStringVal.IsString())
	require.EqualValues(t, "", emptyStringVal.String())

	// Test additional edge cases for numbers
	// Test negative numbers
	negInt32 := ctx.Int32(-42)
	defer negInt32.Free()
	require.EqualValues(t, -42, negInt32.ToInt32())

	negInt64 := ctx.Int64(-1234567890123456)
	defer negInt64.Free()
	require.EqualValues(t, -1234567890123456, negInt64.ToInt64())

	// Test zero values
	zeroInt32 := ctx.Int32(0)
	defer zeroInt32.Free()
	require.EqualValues(t, 0, zeroInt32.ToInt32())

	zeroInt64 := ctx.Int64(0)
	defer zeroInt64.Free()
	require.EqualValues(t, 0, zeroInt64.ToInt64())

	zeroFloat := ctx.Float64(0.0)
	defer zeroFloat.Free()
	require.EqualValues(t, 0.0, zeroFloat.ToFloat64())
}

// TestContextObject tests object creation and manipulation.
func TestContextObject(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Object creation
	obj := ctx.Object()
	defer obj.Free()
	require.True(t, obj.IsObject())

	// Test setting properties
	obj.Set("name", ctx.String("test"))
	obj.Set("value", ctx.Int32(42))

	nameVal := obj.Get("name")
	defer nameVal.Free()
	require.EqualValues(t, "test", nameVal.String())

	valueVal := obj.Get("value")
	defer valueVal.Free()
	require.EqualValues(t, 42, valueVal.ToInt32())
}

// TestContextArrayBuffer tests ArrayBuffer creation and manipulation.
func TestContextArrayBuffer(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test with non-empty data
	data := []byte{1, 2, 3, 4, 5}
	arrayBuffer := ctx.ArrayBuffer(data)
	defer arrayBuffer.Free()

	require.True(t, arrayBuffer.IsByteArray())
	require.EqualValues(t, len(data), arrayBuffer.ByteLen())

	// Test with empty data
	emptyArrayBuffer := ctx.ArrayBuffer([]byte{})
	defer emptyArrayBuffer.Free()
	require.True(t, emptyArrayBuffer.IsByteArray())
	require.EqualValues(t, 0, emptyArrayBuffer.ByteLen())
}

// TestContextParseJSON tests JSON parsing.
func TestContextParseJSON(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test valid JSON
	jsonObj := ctx.ParseJSON(`{"name": "test", "value": 42}`)
	defer jsonObj.Free()
	require.True(t, jsonObj.IsObject())

	nameVal := jsonObj.Get("name")
	defer nameVal.Free()
	require.EqualValues(t, "test", nameVal.String())

	valueVal := jsonObj.Get("value")
	defer valueVal.Free()
	require.EqualValues(t, 42, valueVal.ToInt32())

	// Test JSON array
	jsonArray := ctx.ParseJSON(`[1, 2, 3]`)
	defer jsonArray.Free()
	require.True(t, jsonArray.IsArray())
	require.EqualValues(t, 3, jsonArray.Len())

	// Test invalid JSON
	invalidJSON := ctx.ParseJSON(`{invalid json}`)
	defer invalidJSON.Free()
	require.True(t, invalidJSON.IsException())
}

// TestContextFunction tests function creation and execution.
func TestContextFunction(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test function creation
	fn := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		if len(args) == 0 {
			return ctx.String("no args")
		}
		return ctx.String("Hello " + args[0].String())
	})
	require.True(t, fn.IsFunction())

	// Test function with no arguments
	ctx.Globals().Set("testFn", fn)
	result1, err := ctx.Eval(`testFn()`)
	require.NoError(t, err)
	defer result1.Free()
	require.EqualValues(t, "no args", result1.String())

	// Test function with arguments
	result2, err := ctx.Eval(`testFn("World")`)
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, "Hello World", result2.String())
}

// TestContextAsyncFunction tests async function creation and execution.
func TestContextAsyncFunction(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test async function creation
	asyncFn := ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		if len(args) == 0 {
			return promise.Call("resolve", ctx.String("async no args"))
		}
		return promise.Call("resolve", ctx.String("Async "+args[0].String()))
	})
	require.True(t, asyncFn.IsFunction())

	ctx.Globals().Set("testAsyncFn", asyncFn)

	// Test async function execution
	result, err := ctx.Eval(`
        var result = "";
        testAsyncFn("Hello").then(v => result = v);
    `)
	require.NoError(t, err)
	defer result.Free()

	// Wait for promise to resolve
	ctx.Loop()

	finalResult, err := ctx.Eval(`result`)
	require.NoError(t, err)
	defer finalResult.Free()
	require.EqualValues(t, "Async Hello", finalResult.String())
}

// TestContextAtom tests Atom creation and usage.
func TestContextAtom(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test string atom
	atom1 := ctx.Atom("testProperty")
	defer atom1.Free()
	require.EqualValues(t, "testProperty", atom1.String())

	// Test index atom
	atom2 := ctx.AtomIdx(42)
	defer atom2.Free()
	require.EqualValues(t, "42", atom2.String())
}

// TestContextInvoke tests function invocation.
func TestContextInvoke(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create a function
	fn := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		sum := 0
		for _, arg := range args {
			sum += int(arg.ToInt32())
		}
		return ctx.Int32(int32(sum))
	})
	defer fn.Free()

	// Test invoke with no arguments
	result1 := ctx.Invoke(fn, ctx.Null())
	defer result1.Free()
	require.EqualValues(t, 0, result1.ToInt32())

	// Test invoke with arguments
	result2 := ctx.Invoke(fn, ctx.Null(), ctx.Int32(1), ctx.Int32(2), ctx.Int32(3))
	defer result2.Free()
	require.EqualValues(t, 6, result2.ToInt32())
}

// TestContextEval tests code evaluation with various options.
func TestContextEval(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic evaluation
	result1, err := ctx.Eval(`1 + 2`)
	require.NoError(t, err)
	defer result1.Free()
	require.EqualValues(t, 3, result1.ToInt32())

	// Test evaluation with options
	result2, err := ctx.Eval(`"use strict"; var x = 42; x`,
		quickjs.EvalFlagStrict(true),
		quickjs.EvalFileName("test.js"))
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, 42, result2.ToInt32())

	// Test global evaluation
	result3, err := ctx.Eval(`globalThis.testVar = "global"`,
		quickjs.EvalFlagGlobal(true))
	require.NoError(t, err)
	defer result3.Free()

	// Verify global variable was set
	result4, err := ctx.Eval(`globalThis.testVar`)
	require.NoError(t, err)
	defer result4.Free()
	require.EqualValues(t, "global", result4.String())

	// Test module evaluation
	result5, err := ctx.Eval(`export const x = 42;`,
		quickjs.EvalFlagModule(true))
	require.NoError(t, err)
	defer result5.Free()

	// Test compile only
	result6, err := ctx.Eval(`1 + 1`,
		quickjs.EvalFlagCompileOnly(true))
	require.NoError(t, err)
	defer result6.Free()
}

// TestContextEvalFile tests file evaluation.
func TestContextEvalFile(t *testing.T) {
	rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test evaluating existing file (if test files exist)
	if _, err := os.Stat("./test/hello_module.js"); err == nil {
		result, err := ctx.EvalFile("./test/hello_module.js")
		require.NoError(t, err)
		defer result.Free()
	}

	// Test evaluating non-existent file
	_, err := ctx.EvalFile("./nonexistent.js")
	require.Error(t, err)
}

// TestContextCompile tests code compilation.
func TestContextCompile(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic compilation
	bytecode, err := ctx.Compile(`function test() { return 42; } test()`)
	require.NoError(t, err)
	require.NotEmpty(t, bytecode)

	// Test compilation with options
	bytecode2, err := ctx.Compile(`1 + 1`,
		quickjs.EvalFileName("test.js"),
		quickjs.EvalFlagStrict(true))
	require.NoError(t, err)
	require.NotEmpty(t, bytecode2)

	// Test compiling invalid syntax
	_, err = ctx.Compile(`invalid syntax {`)
	require.Error(t, err)
}

// TestContextCompileFile tests file compilation.
func TestContextCompileFile(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create a temporary test file
	testFile := "./test_temp.js"
	testContent := `function add(a, b) { return a + b; } add(2, 3);`

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Test file compilation
	bytecode, err := ctx.CompileFile(testFile)
	require.NoError(t, err)
	require.NotEmpty(t, bytecode)

	// Test compiling non-existent file
	_, err = ctx.CompileFile("./nonexistent.js")
	require.Error(t, err)
}

// TestContextEvalBytecode tests bytecode evaluation.
func TestContextEvalBytecode(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// First compile some code
	code := `function fibonacci(n) {
        if (n <= 1) return n;
        return fibonacci(n - 1) + fibonacci(n - 2);
    }
    fibonacci(10);`

	bytecode, err := ctx.Compile(code)
	require.NoError(t, err)

	// Create new context to test bytecode execution
	ctx2 := rt.NewContext()
	defer ctx2.Close()

	result, err := ctx2.EvalBytecode(bytecode)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, 55, result.ToInt32())

	// Test invalid bytecode
	invalidBytecode := []byte{0x01, 0x02, 0x03}
	_, err = ctx2.EvalBytecode(invalidBytecode)
	require.Error(t, err)
}

// TestContextModules tests module loading and management.
func TestContextModules(t *testing.T) {
	rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test LoadModule
	moduleCode := `export function add(a, b) { return a + b; }`
	result1, err := ctx.LoadModule(moduleCode, "math_module")
	require.NoError(t, err)
	defer result1.Free()

	// Test using the loaded module
	result2, err := ctx.Eval(`
        import {add} from 'math_module';
        globalThis.result = add(2, 3);
    `)
	require.NoError(t, err)
	defer result2.Free()

	finalResult := ctx.Globals().Get("result")
	defer finalResult.Free()
	require.EqualValues(t, 5, finalResult.ToInt32())

	// Test LoadModuleFile (if test files exist)
	if _, err := os.Stat("./test/fib_module.js"); err == nil {
		result3, err := ctx.LoadModuleFile("./test/fib_module.js", "fib_test")
		require.NoError(t, err)
		defer result3.Free()
	}

	// Test CompileModule and LoadModuleBytecode
	testModuleFile := "./test_module.js"
	testModuleContent := `export const PI = 3.14159;`

	err = os.WriteFile(testModuleFile, []byte(testModuleContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testModuleFile)

	bytecode, err := ctx.CompileModule(testModuleFile, "pi_module")
	require.NoError(t, err)

	result4, err := ctx.LoadModuleBytecode(bytecode)
	require.NoError(t, err)
	defer result4.Free()

	// Test invalid module code
	_, err = ctx.LoadModule(`invalid module syntax {`, "invalid_module")
	require.Error(t, err)
}

// TestContextError tests error creation and handling.
func TestContextError(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Error creation
	testErr := errors.New("test error")
	errorVal := ctx.Error(testErr)
	defer errorVal.Free()
	require.True(t, errorVal.IsError())

	// Test error message
	messageVal := errorVal.Get("message")
	defer messageVal.Free()
	require.EqualValues(t, "test error", messageVal.String())
}

// TestContextThrowErrors tests all throw error methods.
func TestContextThrowErrors(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test ThrowError
	ctx.Globals().Set("throwError", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowError(errors.New("custom error"))
	}))

	_, err := ctx.Eval(`throwError()`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "custom error")

	// Test ThrowSyntaxError
	ctx.Globals().Set("throwSyntaxError", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowSyntaxError("syntax error: %s", "invalid token")
	}))

	_, err = ctx.Eval(`throwSyntaxError()`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SyntaxError")
	require.Contains(t, err.Error(), "invalid token")

	// Test ThrowTypeError
	ctx.Globals().Set("throwTypeError", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowTypeError("type error: %s", "wrong type")
	}))

	_, err = ctx.Eval(`throwTypeError()`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TypeError")
	require.Contains(t, err.Error(), "wrong type")

	// Test ThrowReferenceError
	ctx.Globals().Set("throwReferenceError", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowReferenceError("reference error: %s", "undefined variable")
	}))

	_, err = ctx.Eval(`throwReferenceError()`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ReferenceError")
	require.Contains(t, err.Error(), "undefined variable")

	// Test ThrowRangeError
	ctx.Globals().Set("throwRangeError", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowRangeError("range error: %s", "out of range")
	}))

	_, err = ctx.Eval(`throwRangeError()`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "RangeError")
	require.Contains(t, err.Error(), "out of range")

	// Test ThrowInternalError
	ctx.Globals().Set("throwInternalError", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowInternalError("internal error: %s", "system failure")
	}))

	_, err = ctx.Eval(`throwInternalError()`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "InternalError")
	require.Contains(t, err.Error(), "system failure")
}

// TestContextThrow tests the Throw method.
func TestContextThrow(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test throwing a custom value
	ctx.Globals().Set("throwCustom", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		customError := ctx.Error(errors.New("custom thrown value"))
		return ctx.Throw(customError)
	}))

	_, err := ctx.Eval(`throwCustom()`)
	require.Error(t, err)
}

// TestContextException tests exception handling.
func TestContextException(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create an exception by evaluating invalid code
	_, err := ctx.Eval(`throw new Error("test exception")`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "test exception")

	// Test Exception method after an error
	exception := ctx.Exception()
	if exception != nil {
		require.Contains(t, exception.Error(), "Error")
	}
}

// TestContextGlobals tests global object access.
func TestContextGlobals(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test getting globals
	globals := ctx.Globals()
	require.True(t, globals.IsObject())

	// Test setting global variable
	globals.Set("testGlobal", ctx.String("hello global"))

	// Test accessing global variable from JavaScript
	result, err := ctx.Eval(`testGlobal`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "hello global", result.String())

	// Test that globals object is cached (same reference)
	globals2 := ctx.Globals()
	require.True(t, globals2.IsObject())
}

// TestContextLoop tests the event loop.
func TestContextLoop(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Set up a setTimeout
	result, err := ctx.Eval(`
        var executed = false;
        setTimeout(() => {
            executed = true;
        }, 10);
    `)
	require.NoError(t, err)
	defer result.Free()

	// Run the event loop
	ctx.Loop()

	// Check if the timeout was executed
	executedResult, err := ctx.Eval(`executed`)
	require.NoError(t, err)
	defer executedResult.Free()
	require.True(t, executedResult.ToBool())
}

// TestContextAwait tests promise awaiting.
func TestContextAwait(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create an async function that returns a promise
	ctx.Globals().Set("asyncTest", ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		return promise.Call("resolve", ctx.String("awaited result"))
	}))

	// Get the promise
	promiseResult, err := ctx.Eval(`asyncTest()`)
	require.NoError(t, err)
	require.True(t, promiseResult.IsPromise())

	// Await the promise
	awaitedResult, err := ctx.Await(promiseResult)
	require.NoError(t, err)
	defer awaitedResult.Free()
	require.EqualValues(t, "awaited result", awaitedResult.String())

	// Test awaiting with EvalAwait option
	awaitedResult2, err := ctx.Eval(`asyncTest()`, quickjs.EvalAwait(true))
	require.NoError(t, err)
	defer awaitedResult2.Free()
	require.EqualValues(t, "awaited result", awaitedResult2.String())

	// Test awaiting a rejected promise
	// Test awaiting a rejected promise
	ctx.Globals().Set("asyncReject", ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		// Create an error object for rejection
		errorObj := ctx.Error(errors.New("rejection reason"))
		defer errorObj.Free()
		return promise.Call("reject", errorObj)
	}))

	rejectPromise, err := ctx.Eval(`asyncReject()`)
	require.NoError(t, err)

	_, err = ctx.Await(rejectPromise)
	require.Error(t, err)
}

// TestContextSetInterruptHandler tests the deprecated interrupt handler.
func TestContextSetInterruptHandler(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	interruptCalled := false

	// Test the deprecated SetInterruptHandler method
	ctx.SetInterruptHandler(func() int {
		interruptCalled = true
		return 1 // Interrupt immediately
	})

	// This should be interrupted
	_, err := ctx.Eval(`while(true){}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "interrupted")
	require.True(t, interruptCalled)
}

// TestContextEvalOptions tests all evaluation options.
func TestContextEvalOptions(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test EvalFlagGlobal
	result1, err := ctx.Eval(`var globalVar = "test";`,
		quickjs.EvalFlagGlobal(true))
	require.NoError(t, err)
	defer result1.Free()

	// Test EvalFlagModule
	result2, err := ctx.Eval(`export const moduleVar = "test";`,
		quickjs.EvalFlagModule(true))
	require.NoError(t, err)
	defer result2.Free()

	// Test EvalFlagStrict
	result3, err := ctx.Eval(`"use strict"; var strictVar = "test";`,
		quickjs.EvalFlagStrict(true))
	require.NoError(t, err)
	defer result3.Free()

	// Test EvalFlagCompileOnly
	result4, err := ctx.Eval(`1 + 1`,
		quickjs.EvalFlagCompileOnly(true))
	require.NoError(t, err)
	defer result4.Free()

	// Test EvalFileName
	result5, err := ctx.Eval(`1 + 1`,
		quickjs.EvalFileName("custom_file.js"))
	require.NoError(t, err)
	defer result5.Free()

	// Test EvalAwait (tested in TestContextAwait but also here for completeness)
	ctx.Globals().Set("promiseTest", ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		return promise.Call("resolve", ctx.String("await test"))
	}))

	result6, err := ctx.Eval(`promiseTest()`,
		quickjs.EvalAwait(true))
	require.NoError(t, err)
	defer result6.Free()
	require.EqualValues(t, "await test", result6.String())
}

// TestContextEdgeCases tests various edge cases.
func TestContextEdgeCases(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test empty code evaluation
	result1, err := ctx.Eval(``)
	require.NoError(t, err)
	defer result1.Free()

	// Test whitespace only code
	result2, err := ctx.Eval("   \n\t   ")
	require.NoError(t, err)
	defer result2.Free()

	// Test very long string
	longString := strings.Repeat("a", 10000)
	longStringVal := ctx.String(longString)
	defer longStringVal.Free()
	require.EqualValues(t, longString, longStringVal.String())

	// Test special characters in string
	specialString := "Hello\nWorld\t\"quoted\"\r\n"
	specialStringVal := ctx.String(specialString)
	defer specialStringVal.Free()
	require.EqualValues(t, specialString, specialStringVal.String())

	// Test zero values
	zeroInt := ctx.Int32(0)
	defer zeroInt.Free()
	require.EqualValues(t, 0, zeroInt.ToInt32())

	zeroFloat := ctx.Float64(0.0)
	defer zeroFloat.Free()
	require.EqualValues(t, 0.0, zeroFloat.ToFloat64())

	// Test negative values
	negativeInt := ctx.Int32(-42)
	defer negativeInt.Free()
	require.EqualValues(t, -42, negativeInt.ToInt32())

	negativeFloat := ctx.Float64(-3.14)
	defer negativeFloat.Free()
	require.InDelta(t, -3.14, negativeFloat.ToFloat64(), 0.001)
}
