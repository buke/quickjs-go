package quickjs_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
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

	// Test large code that might trigger compilation serialization error (line 437-439)
	var largeCode strings.Builder
	largeCode.WriteString("var obj = {\n")
	for i := 0; i < 30000; i++ {
		largeCode.WriteString(fmt.Sprintf("  prop_%d: function() { return %d; },\n", i, i))
	}
	largeCode.WriteString("  final: 'end'\n}; obj;")

	_, err := ctx.Compile(largeCode.String())
	if err != nil {
		t.Logf("Successfully triggered compilation serialization error: %v", err)
		// This covers the uncovered line 437-439 where ptr == nil
	} else {
		t.Logf("Large code compiled successfully")
	}

	// Test very deep nesting
	deepCode := "var obj = "
	for i := 0; i < 1500; i++ {
		deepCode += "{ nested: "
	}
	deepCode += "42"
	for i := 0; i < 1500; i++ {
		deepCode += " }"
	}
	deepCode += "; obj;"

	_, err = ctx.Compile(deepCode)
	if err != nil {
		t.Logf("Successfully triggered error with deep nesting: %v", err)
	}
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
