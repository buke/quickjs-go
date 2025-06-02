package quickjs_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestContextBasics tests basic context operations
func TestContextBasics(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Runtime() method
	contextRuntime := ctx.Runtime()
	require.NotNil(t, contextRuntime)

	// Test ArrayBuffer with different data sizes (context-specific functionality)
	testArrayBuffer := func(data []byte) {
		ab := ctx.ArrayBuffer(data)
		defer ab.Free()
		require.True(t, ab.IsByteArray())
		require.EqualValues(t, len(data), ab.ByteLen())
	}

	testArrayBuffer([]byte{1, 2, 3, 4, 5})
	testArrayBuffer([]byte{})
	testArrayBuffer(nil)

	// NEW: Test BigUint64 creation (covers context.go:99.47,101.2)
	bigUint64Val := ctx.BigUint64(18446744073709551615) // max uint64
	defer bigUint64Val.Free()
	require.True(t, bigUint64Val.IsBigInt())

	// Test with smaller BigUint64 value
	smallBigUint64Val := ctx.BigUint64(42)
	defer smallBigUint64Val.Free()
	require.True(t, smallBigUint64Val.IsBigInt())
}

// TestContextEvaluation tests code evaluation and compilation
func TestContextEvaluation(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Basic evaluation
	result, err := ctx.Eval(`1 + 2`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, 3, result.ToInt32())

	// Test with options
	result2, err := ctx.Eval(`"use strict"; var x = 42; x`,
		quickjs.EvalFlagStrict(true),
		quickjs.EvalFileName("test.js"))
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, 42, result2.ToInt32())

	// Test module evaluation
	result3, err := ctx.Eval(`export const x = 42;`, quickjs.EvalFlagModule(true))
	require.NoError(t, err)
	defer result3.Free()

	// Test compile only
	result4, err := ctx.Eval(`1 + 1`, quickjs.EvalFlagCompileOnly(true))
	require.NoError(t, err)
	defer result4.Free()

	// Test evaluation errors
	_, err = ctx.Eval(`invalid syntax {`)
	require.Error(t, err)

	// Test empty code
	result5, err := ctx.Eval(``)
	require.NoError(t, err)
	defer result5.Free()

	// NEW: Test EvalFlagGlobal (covers context.go:238.45,239.34 and 239.34,241.3)
	result6, err := ctx.Eval(`var globalFlagTest = "global flag test"; globalFlagTest`,
		quickjs.EvalFlagGlobal(false))
	require.NoError(t, err)
	defer result6.Free()
	require.EqualValues(t, "global flag test", result6.String())

	// Test EvalFlagGlobal with true (default behavior)
	result7, err := ctx.Eval(`var globalFlagTest2 = "global flag test 2"; globalFlagTest2`,
		quickjs.EvalFlagGlobal(true))
	require.NoError(t, err)
	defer result7.Free()
	require.EqualValues(t, "global flag test 2", result7.String())
}

// TestContextBytecodeOperations tests compilation and bytecode execution
func TestContextBytecodeOperations(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic compilation and execution
	code := `function add(a, b) { return a + b; } add(2, 3);`
	bytecode, err := ctx.Compile(code)
	require.NoError(t, err)
	require.NotEmpty(t, bytecode)

	result, err := ctx.EvalBytecode(bytecode)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, 5, result.ToInt32())

	// Test file operations
	testFile := "./test_temp.js"
	testContent := `function multiply(a, b) { return a * b; } multiply(3, 4);`
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// NEW: Test EvalFile with custom options (covers context.go:376.107,379.2)
	resultFromFile, err := ctx.EvalFile(testFile, quickjs.EvalFlagStrict(true))
	require.NoError(t, err)
	defer resultFromFile.Free()
	require.EqualValues(t, 12, resultFromFile.ToInt32())

	// Test CompileFile with and without custom filename
	bytecode2, err := ctx.CompileFile(testFile)
	require.NoError(t, err)
	require.NotEmpty(t, bytecode2)

	bytecode3, err := ctx.CompileFile(testFile, quickjs.EvalFileName("custom.js"))
	require.NoError(t, err)
	require.NotEmpty(t, bytecode3)

	// Test error cases
	_, err = ctx.EvalBytecode([]byte{})
	require.Error(t, err)

	_, err = ctx.EvalBytecode([]byte{0x01, 0x02, 0x03})
	require.Error(t, err)

	_, err = ctx.EvalFile("./nonexistent.js")
	require.Error(t, err)

	_, err = ctx.CompileFile("./nonexistent.js")
	require.Error(t, err)

	// NEW: Test EvalBytecode with invalid function that causes exception (covers context.go:418.23,420.3)
	// Create bytecode that will throw exception during evaluation
	invalidCode := `throw new Error("test exception during evaluation");`
	invalidBytecode, err := ctx.Compile(invalidCode)
	require.NoError(t, err)

	_, err = ctx.EvalBytecode(invalidBytecode)
	require.Error(t, err)
	require.Contains(t, err.Error(), "test exception during evaluation")
}

// TestContextModules tests module operations
func TestContextModules(t *testing.T) {
	rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test valid module loading
	moduleCode := `export function add(a, b) { return a + b; }`
	result, err := ctx.LoadModule(moduleCode, "math_module")
	require.NoError(t, err)
	defer result.Free()

	// NEW: Test LoadModule with custom options (covers context.go:393.23,395.3)
	result2, err := ctx.LoadModule(moduleCode, "math_module2", quickjs.EvalLoadOnly(true))
	require.NoError(t, err)
	defer result2.Free()

	// Test LoadModuleBytecode
	bytecode, err := ctx.Compile(moduleCode, quickjs.EvalFlagModule(true), quickjs.EvalFlagCompileOnly(true))
	require.NoError(t, err)

	result3, err := ctx.LoadModuleBytecode(bytecode)
	require.NoError(t, err)
	defer result3.Free()

	// NEW: Test LoadModuleBytecode with load_only flag (covers context.go:418.23,420.3)
	result4, err := ctx.LoadModuleBytecode(bytecode, quickjs.EvalLoadOnly(true))
	require.NoError(t, err)
	defer result4.Free()

	// Test error cases that are not covered
	// Module detection failure - test the uncovered line 346-348
	_, err = ctx.LoadModule(`var x = 1; x;`, "not_module")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a module")

	// Module compilation error - test the uncovered line 358-360
	_, err = ctx.LoadModule(`export { unclosed_brace`, "invalid_module")
	require.Error(t, err)

	// Empty bytecode
	_, err = ctx.LoadModuleBytecode([]byte{})
	require.Error(t, err)

	// Invalid bytecode
	_, err = ctx.LoadModuleBytecode([]byte{0x01, 0x02, 0x03})
	require.Error(t, err)

	// NEW: Test LoadModuleFile (covers context.go:372.2,372.46)
	moduleFile := "./test_module.js"
	moduleContent := `export const value = 42;`
	err = os.WriteFile(moduleFile, []byte(moduleContent), 0644)
	require.NoError(t, err)
	defer os.Remove(moduleFile)

	moduleResult, err := ctx.LoadModuleFile(moduleFile, "test_module")
	require.NoError(t, err)
	defer moduleResult.Free()

	// NEW: Test CompileModule (covers context.go:376.107,379.2)
	compiledModule, err := ctx.CompileModule(moduleFile, "compiled_module")
	require.NoError(t, err)
	require.NotEmpty(t, compiledModule)

	// Test CompileModule with custom options
	compiledModule2, err := ctx.CompileModule(moduleFile, "compiled_module2", quickjs.EvalFlagStrict(true))
	require.NoError(t, err)
	require.NotEmpty(t, compiledModule2)

	// File not found error - test the uncovered line 369-371
	_, err = ctx.LoadModuleFile("./nonexistent_file.js", "missing")
	require.Error(t, err)
}

// TestContextFunctions tests function creation and execution
func TestContextFunctions(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test regular function
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

	// Test with arguments
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

	// Test async function
	asyncFn := ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		return promise.Call("resolve", ctx.String("async result"))
	})
	// defer asyncFn.Free() // cause testAsync will attach to globals,  so we don't free it here

	ctx.Globals().Set("testAsync", asyncFn)
	result5, err := ctx.Eval(`testAsync()`, quickjs.EvalAwait(true))
	require.NoError(t, err)
	defer result5.Free()
	require.EqualValues(t, "async result", result5.String())
}

// TestContextErrorHandling tests error creation and exception handling
func TestContextErrorHandling(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Error creation
	testErr := errors.New("test error")
	errorVal := ctx.Error(testErr)
	defer errorVal.Free()
	require.True(t, errorVal.IsError())

	// Test all throw methods
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

	// Test Exception() when no exception
	exception := ctx.Exception()
	require.Nil(t, exception)
	require.False(t, ctx.HasException())
}

// TestContextGlobalsAndUtilities tests globals access and utility methods
func TestContextGlobalsAndUtilities(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

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

	// Test JSON parsing
	jsonObj := ctx.ParseJSON(`{"name": "test", "value": 42}`)
	defer jsonObj.Free()
	require.True(t, jsonObj.IsObject())

	nameVal := jsonObj.Get("name")
	defer nameVal.Free()
	require.EqualValues(t, "test", nameVal.String())

	// Test invalid JSON
	invalidJSON := ctx.ParseJSON(`{invalid}`)
	defer invalidJSON.Free()
	require.True(t, invalidJSON.IsException())
}

// TestContextAsync tests async operations and event loop
func TestContextAsync(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test event loop
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

	// Test Await
	ctx.Globals().Set("asyncTest", ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		return promise.Call("resolve", ctx.String("awaited result"))
	}))

	promiseResult, err := ctx.Eval(`asyncTest()`)
	require.NoError(t, err)
	require.True(t, promiseResult.IsPromise())

	awaitedResult, err := ctx.Await(promiseResult)
	require.NoError(t, err)
	defer awaitedResult.Free()
	require.EqualValues(t, "awaited result", awaitedResult.String())

	// Test rejected promise
	ctx.Globals().Set("asyncReject", ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		errorObj := ctx.Error(errors.New("rejection reason"))
		defer errorObj.Free()
		return promise.Call("reject", errorObj)
	}))

	rejectPromise, err := ctx.Eval(`asyncReject()`)
	require.NoError(t, err)

	_, err = ctx.Await(rejectPromise)
	require.Error(t, err)
}

// TestContextInterruptHandler tests interrupt handler functionality
func TestContextInterruptHandler(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	interruptCalled := false

	// Test deprecated SetInterruptHandler (delegates to runtime)
	ctx.SetInterruptHandler(func() int {
		interruptCalled = true
		return 1 // Interrupt
	})

	_, err := ctx.Eval(`while(true){}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "interrupted")
	require.True(t, interruptCalled)
}

// TestContextCompilationErrors tests compilation error edge cases
func TestContextCompilationErrors(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic compilation errors
	_, err := ctx.Compile(`invalid syntax {`)
	require.Error(t, err)

	// Test compilation with invalid module syntax
	_, err = ctx.Compile(`export { unclosed`, quickjs.EvalFlagModule(true))
	require.Error(t, err)

	// Test empty code compilation
	bytecode, err := ctx.Compile(``)
	require.NoError(t, err)
	require.NotEmpty(t, bytecode)

	// Test normal compilation to ensure basic functionality works
	normalCode := `(function() { return 42; })` // Function expression returns the function object
	// Alternative: normalCode := `() => { return 42; }` // Arrow function

	r, e := ctx.Eval(normalCode)
	defer r.Free()
	require.NoError(t, e)

	bytecode, err = ctx.Compile(normalCode)
	require.NoError(t, err)
	require.NotEmpty(t, bytecode)

	// Verify the compiled code can be executed
	result, err := ctx.EvalBytecode(bytecode)
	require.NoError(t, err)
	defer result.Free()
	require.True(t, result.IsFunction())
}

// Most aggressive approach to trigger JS_WriteObject failure
func TestContextCompileExtremeMemoryPressure(t *testing.T) {
	// Use extremely restrictive memory limit
	rt := quickjs.NewRuntime(quickjs.WithMemoryLimit(32 * 1024)) // 32KB limit
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Fill memory first
	ctx.Eval(`
        var memoryFiller = [];
        try {
            for(let i = 0; i < 1000; i++) {
                memoryFiller.push(new Array(100).fill('x'.repeat(50)));
            }
        } catch(e) {
            // Expected to fail due to memory limit
        }
    `)

	// Now try to compile - this should fail at JS_WriteObject due to no available memory
	_, err := ctx.Compile(`
        var obj = {};
        for(let i = 0; i < 100; i++) {
            obj['prop_' + i] = function() { return 'value_' + i; };
        }
        obj;
    `)

	if err != nil {
		t.Logf("Extreme memory pressure compilation error: %v", err)
	}

	// Try multiple rapid compilations to exhaust memory
	for i := 0; i < 20; i++ {
		code := fmt.Sprintf(`var obj%d = { data: new Array(500).fill(%d) }; obj%d;`, i, i, i)
		_, err := ctx.Compile(code)
		if err != nil {
			t.Logf("Rapid compilation %d failed: %v", i, err)
			break
		}
	}
}

// TestContextTypedArrayCreation tests TypedArray creation from Go types
func TestContextTypedArrayCreation(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("Int8Array", func(t *testing.T) {
		// Test with data
		data := []int8{-128, -1, 0, 1, 127}
		arr := ctx.Int8Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsInt8Array())
		require.False(t, arr.IsUint8Array())
		require.EqualValues(t, len(data), arr.Len())

		// Test empty array
		emptyArr := ctx.Int8Array([]int8{})
		defer emptyArr.Free()
		require.True(t, emptyArr.IsInt8Array())
		require.EqualValues(t, 0, emptyArr.Len())

		// Test nil slice
		nilArr := ctx.Int8Array(nil)
		defer nilArr.Free()
		require.True(t, nilArr.IsInt8Array())
		require.EqualValues(t, 0, nilArr.Len())
	})

	t.Run("Uint8Array", func(t *testing.T) {
		data := []uint8{0, 1, 128, 255}
		arr := ctx.Uint8Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsUint8Array())
		require.False(t, arr.IsInt8Array())
		require.EqualValues(t, len(data), arr.Len())

		// Test empty array
		emptyArr := ctx.Uint8Array([]uint8{})
		defer emptyArr.Free()
		require.True(t, emptyArr.IsUint8Array())
		require.EqualValues(t, 0, emptyArr.Len())
	})

	t.Run("Uint8ClampedArray", func(t *testing.T) {
		data := []uint8{0, 127, 255}
		arr := ctx.Uint8ClampedArray(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsUint8ClampedArray())
		require.False(t, arr.IsUint8Array())
		require.EqualValues(t, len(data), arr.Len())

		// Test empty array
		emptyArr := ctx.Uint8ClampedArray([]uint8{})
		defer emptyArr.Free()
		require.True(t, emptyArr.IsUint8ClampedArray())
		require.EqualValues(t, 0, emptyArr.Len())

		// Test nil slice
		nilArr := ctx.Uint8ClampedArray(nil)
		defer nilArr.Free()
		require.True(t, nilArr.IsUint8ClampedArray())
		require.EqualValues(t, 0, nilArr.Len())
	})

	t.Run("Int16Array", func(t *testing.T) {
		data := []int16{-32768, -1, 0, 1, 32767}
		arr := ctx.Int16Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsInt16Array())
		require.False(t, arr.IsUint16Array())
		require.EqualValues(t, len(data), arr.Len())
	})

	t.Run("Uint16Array", func(t *testing.T) {
		data := []uint16{0, 1, 32768, 65535}
		arr := ctx.Uint16Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsUint16Array())
		require.False(t, arr.IsInt16Array())
		require.EqualValues(t, len(data), arr.Len())
	})

	t.Run("Int32Array", func(t *testing.T) {
		data := []int32{-2147483648, -1, 0, 1, 2147483647}
		arr := ctx.Int32Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsInt32Array())
		require.False(t, arr.IsUint32Array())
		require.EqualValues(t, len(data), arr.Len())
	})

	t.Run("Uint32Array", func(t *testing.T) {
		data := []uint32{0, 1, 2147483648, 4294967295}
		arr := ctx.Uint32Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsUint32Array())
		require.False(t, arr.IsInt32Array())
		require.EqualValues(t, len(data), arr.Len())
	})

	t.Run("Float32Array", func(t *testing.T) {
		data := []float32{-3.14, 0.0, 1.5, 3.14159}
		arr := ctx.Float32Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsFloat32Array())
		require.False(t, arr.IsFloat64Array())
		require.EqualValues(t, len(data), arr.Len())
	})

	t.Run("Float64Array", func(t *testing.T) {
		data := []float64{-3.141592653589793, 0.0, 1.5, 3.141592653589793}
		arr := ctx.Float64Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsFloat64Array())
		require.False(t, arr.IsFloat32Array())
		require.EqualValues(t, len(data), arr.Len())
	})

	t.Run("BigInt64Array", func(t *testing.T) {
		data := []int64{-9223372036854775808, -1, 0, 1, 9223372036854775807}
		arr := ctx.BigInt64Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsBigInt64Array())
		require.False(t, arr.IsBigUint64Array())
		require.EqualValues(t, len(data), arr.Len())
	})

	t.Run("BigUint64Array", func(t *testing.T) {
		data := []uint64{0, 1, 9223372036854775808, 18446744073709551615}
		arr := ctx.BigUint64Array(data)
		defer arr.Free()

		require.True(t, arr.IsTypedArray())
		require.True(t, arr.IsBigUint64Array())
		require.False(t, arr.IsBigInt64Array())
		require.EqualValues(t, len(data), arr.Len())
	})
}

// TestContextTypedArrayInterop tests TypedArray interoperability between Go and JavaScript
func TestContextTypedArrayInterop(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("GoToJavaScript", func(t *testing.T) {
		// Create TypedArray in Go and use in JavaScript
		goData := []int32{1, 2, 3, 4, 5}
		goArray := ctx.Int32Array(goData)
		// defer goArray.Free()

		// Set in global scope
		ctx.Globals().Set("goArray", goArray)

		// Use in JavaScript
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

		// Test modification from JavaScript
		_, err = ctx.Eval(`goArray[0] = 10;`)
		require.NoError(t, err)

		// Verify modification
		modifiedResult, err := ctx.Eval(`goArray[0]`)
		require.NoError(t, err)
		defer modifiedResult.Free()
		require.EqualValues(t, 10, modifiedResult.ToInt32())
	})

	t.Run("JavaScriptToGo", func(t *testing.T) {
		// Create TypedArray in JavaScript and convert to Go
		jsArray, err := ctx.Eval(`new Int32Array([10, 20, 30, 40, 50])`)
		require.NoError(t, err)
		defer jsArray.Free()

		// Verify it's a TypedArray
		require.True(t, jsArray.IsTypedArray())
		require.True(t, jsArray.IsInt32Array())

		// Convert to Go slice
		goSlice, err := jsArray.ToInt32Array()
		require.NoError(t, err)
		require.Equal(t, []int32{10, 20, 30, 40, 50}, goSlice)
	})

	t.Run("RoundTripConversion", func(t *testing.T) {
		// Test round-trip conversion for all TypedArray types
		testCases := []struct {
			name     string
			create   func() quickjs.Value
			convert  func(quickjs.Value) (interface{}, error)
			expected interface{}
		}{
			{
				"Int8Array",
				func() quickjs.Value { return ctx.Int8Array([]int8{-1, 0, 1}) },
				func(v quickjs.Value) (interface{}, error) { return v.ToInt8Array() },
				[]int8{-1, 0, 1},
			},
			{
				"Uint8Array",
				func() quickjs.Value { return ctx.Uint8Array([]uint8{0, 128, 255}) },
				func(v quickjs.Value) (interface{}, error) { return v.ToUint8Array() },
				[]uint8{0, 128, 255},
			},
			{
				"Float32Array",
				func() quickjs.Value { return ctx.Float32Array([]float32{1.5, 2.5, 3.5}) },
				func(v quickjs.Value) (interface{}, error) { return v.ToFloat32Array() },
				[]float32{1.5, 2.5, 3.5},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create in Go
				goArray := tc.create()
				defer goArray.Free()

				// Convert back to Go slice
				result, err := tc.convert(goArray)
				require.NoError(t, err)

				// Verify the data matches
				switch expected := tc.expected.(type) {
				case []int8:
					require.Equal(t, expected, result.([]int8))
				case []uint8:
					require.Equal(t, expected, result.([]uint8))
				case []float32:
					resultSlice := result.([]float32)
					require.Len(t, resultSlice, len(expected))
					for i, v := range expected {
						require.InDelta(t, v, resultSlice[i], 0.0001)
					}
				}
			})
		}
	})

	t.Run("SharedMemory", func(t *testing.T) {
		// Test that TypedArrays share memory with their underlying ArrayBuffer
		data := []uint8{1, 2, 3, 4, 5, 6, 7, 8}
		arrayBuffer := ctx.ArrayBuffer(data)

		// Set ArrayBuffer in global scope
		ctx.Globals().Set("sharedBuffer", arrayBuffer)

		// Create different views on the same buffer and store them globally
		ret, err := ctx.Eval(`
        globalThis.uint8View = new Uint8Array(sharedBuffer);
        globalThis.uint16View = new Uint16Array(sharedBuffer);
    `)
		defer ret.Free()
		require.NoError(t, err)

		// Verify initial values
		initialUint8, err := ctx.Eval(`uint8View[0]`)
		require.NoError(t, err)
		defer initialUint8.Free()
		require.EqualValues(t, 1, initialUint8.ToInt32())

		// Modify through uint8 view
		_, err = ctx.Eval(`uint8View[0] = 255;`)
		require.NoError(t, err)

		// Verify change is visible through the same view
		modifiedUint8, err := ctx.Eval(`uint8View[0]`)
		require.NoError(t, err)
		defer modifiedUint8.Free()
		require.EqualValues(t, 255, modifiedUint8.ToInt32())

		// Verify change is also visible through uint16 view (shared memory)
		uint16Value, err := ctx.Eval(`uint16View[0]`)
		require.NoError(t, err)
		defer uint16Value.Free()

		// The uint16 value should have changed because we modified the underlying byte
		// Original: bytes [1, 2] -> uint16: 513 (little-endian: 1 + 2*256)
		// Modified: bytes [255, 2] -> uint16: 767 (little-endian: 255 + 2*256)
		require.EqualValues(t, 767, uint16Value.ToInt32())

		// // Clean up global variables
		ctx.Eval(`delete globalThis.uint8View; delete globalThis.uint16View;`)
	})
}

// TestContextTypedArrayErrorCases tests TypedArray error handling
func TestContextTypedArrayErrorCases(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("EmptyArrayCreation", func(t *testing.T) {
		// Test creating empty TypedArrays
		emptyArrays := []struct {
			name   string
			create func() quickjs.Value
		}{
			{"Int8Array", func() quickjs.Value { return ctx.Int8Array([]int8{}) }},
			{"Uint8Array", func() quickjs.Value { return ctx.Uint8Array([]uint8{}) }},
			{"Int16Array", func() quickjs.Value { return ctx.Int16Array([]int16{}) }},
			{"Uint16Array", func() quickjs.Value { return ctx.Uint16Array([]uint16{}) }},
			{"Int32Array", func() quickjs.Value { return ctx.Int32Array([]int32{}) }},
			{"Uint32Array", func() quickjs.Value { return ctx.Uint32Array([]uint32{}) }},
			{"Float32Array", func() quickjs.Value { return ctx.Float32Array([]float32{}) }},
			{"Float64Array", func() quickjs.Value { return ctx.Float64Array([]float64{}) }},
			{"BigInt64Array", func() quickjs.Value { return ctx.BigInt64Array([]int64{}) }},
			{"BigUint64Array", func() quickjs.Value { return ctx.BigUint64Array([]uint64{}) }},
		}

		for _, tc := range emptyArrays {
			t.Run(tc.name, func(t *testing.T) {
				arr := tc.create()
				defer arr.Free()

				require.True(t, arr.IsTypedArray())
				require.EqualValues(t, 0, arr.Len())
			})
		}
	})

	t.Run("NilSliceCreation", func(t *testing.T) {
		// Test creating TypedArrays with nil slices
		nilArrays := []struct {
			name   string
			create func() quickjs.Value
		}{
			{"Int8Array", func() quickjs.Value { return ctx.Int8Array(nil) }},
			{"Uint8Array", func() quickjs.Value { return ctx.Uint8Array(nil) }},
			{"Int16Array", func() quickjs.Value { return ctx.Int16Array(nil) }},
			{"Uint16Array", func() quickjs.Value { return ctx.Uint16Array(nil) }},
			{"Int32Array", func() quickjs.Value { return ctx.Int32Array(nil) }},
			{"Uint32Array", func() quickjs.Value { return ctx.Uint32Array(nil) }},
			{"Float32Array", func() quickjs.Value { return ctx.Float32Array(nil) }},
			{"Float64Array", func() quickjs.Value { return ctx.Float64Array(nil) }},
			{"BigInt64Array", func() quickjs.Value { return ctx.BigInt64Array(nil) }},
			{"BigUint64Array", func() quickjs.Value { return ctx.BigUint64Array(nil) }},
		}

		for _, tc := range nilArrays {
			t.Run(tc.name, func(t *testing.T) {
				arr := tc.create()
				defer arr.Free()

				require.True(t, arr.IsTypedArray())
				require.EqualValues(t, 0, arr.Len())
			})
		}
	})

	t.Run("ConversionErrors", func(t *testing.T) {
		// Test conversion errors for wrong types
		wrongTypeVal := ctx.String("not a typed array")
		defer wrongTypeVal.Free()

		conversionTests := []struct {
			name      string
			convertFn func() (interface{}, error)
		}{
			{"ToInt8Array", func() (interface{}, error) { return wrongTypeVal.ToInt8Array() }},
			{"ToUint8Array", func() (interface{}, error) { return wrongTypeVal.ToUint8Array() }},
			{"ToInt16Array", func() (interface{}, error) { return wrongTypeVal.ToInt16Array() }},
			{"ToUint16Array", func() (interface{}, error) { return wrongTypeVal.ToUint16Array() }},
			{"ToInt32Array", func() (interface{}, error) { return wrongTypeVal.ToInt32Array() }},
			{"ToUint32Array", func() (interface{}, error) { return wrongTypeVal.ToUint32Array() }},
			{"ToFloat32Array", func() (interface{}, error) { return wrongTypeVal.ToFloat32Array() }},
			{"ToFloat64Array", func() (interface{}, error) { return wrongTypeVal.ToFloat64Array() }},
			{"ToBigInt64Array", func() (interface{}, error) { return wrongTypeVal.ToBigInt64Array() }},
			{"ToBigUint64Array", func() (interface{}, error) { return wrongTypeVal.ToBigUint64Array() }},
		}

		for _, tc := range conversionTests {
			t.Run(tc.name, func(t *testing.T) {
				_, err := tc.convertFn()
				require.Error(t, err)
			})
		}
	})

	t.Run("TypeMismatchConversion", func(t *testing.T) {
		// Test converting TypedArray to wrong type
		int8Array := ctx.Int8Array([]int8{1, 2, 3})
		defer int8Array.Free()

		// Try to convert Int8Array to Uint8Array (should fail)
		_, err := int8Array.ToUint8Array()
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a Uint8Array")

		// Try to convert Int8Array to Int16Array (should fail)
		_, err = int8Array.ToInt16Array()
		require.Error(t, err)
		require.Contains(t, err.Error(), "not an Int16Array")
	})
}

// TestContextTypedArrayEdgeCases tests TypedArray edge cases and boundary conditions
func TestContextTypedArrayEdgeCases(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("LargeArrays", func(t *testing.T) {
		// Test with relatively large arrays
		largeSize := 10000
		largeData := make([]int32, largeSize)
		for i := range largeData {
			largeData[i] = int32(i)
		}

		largeArray := ctx.Int32Array(largeData)
		defer largeArray.Free()

		require.True(t, largeArray.IsInt32Array())
		require.EqualValues(t, largeSize, largeArray.Len())

		// Convert back and verify first and last elements
		converted, err := largeArray.ToInt32Array()
		require.NoError(t, err)
		require.Len(t, converted, largeSize)
		require.EqualValues(t, 0, converted[0])
		require.EqualValues(t, largeSize-1, converted[largeSize-1])
	})

	t.Run("ExtremeValues", func(t *testing.T) {
		// Test with extreme values for each type
		testCases := []struct {
			name   string
			create func() quickjs.Value
			verify func(quickjs.Value)
		}{
			{
				"Int8Array extremes",
				func() quickjs.Value {
					return ctx.Int8Array([]int8{-128, 127}) // min and max int8
				},
				func(v quickjs.Value) {
					data, err := v.ToInt8Array()
					require.NoError(t, err)
					require.Equal(t, []int8{-128, 127}, data)
				},
			},
			{
				"Uint8Array extremes",
				func() quickjs.Value {
					return ctx.Uint8Array([]uint8{0, 255}) // min and max uint8
				},
				func(v quickjs.Value) {
					data, err := v.ToUint8Array()
					require.NoError(t, err)
					require.Equal(t, []uint8{0, 255}, data)
				},
			},
			{
				"Int32Array extremes",
				func() quickjs.Value {
					return ctx.Int32Array([]int32{-2147483648, 2147483647}) // min and max int32
				},
				func(v quickjs.Value) {
					data, err := v.ToInt32Array()
					require.NoError(t, err)
					require.Equal(t, []int32{-2147483648, 2147483647}, data)
				},
			},
			{
				"BigInt64Array extremes",
				func() quickjs.Value {
					return ctx.BigInt64Array([]int64{-9223372036854775808, 9223372036854775807}) // min and max int64
				},
				func(v quickjs.Value) {
					data, err := v.ToBigInt64Array()
					require.NoError(t, err)
					require.Equal(t, []int64{-9223372036854775808, 9223372036854775807}, data)
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				arr := tc.create()
				defer arr.Free()

				tc.verify(arr)
			})
		}
	})

	t.Run("FloatingPointPrecision", func(t *testing.T) {
		// Test floating point precision preservation
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
}
