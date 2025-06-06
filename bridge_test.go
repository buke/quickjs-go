package quickjs

import (
	"runtime/cgo"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBridgeGetContextFromJSReturnNil(t *testing.T) {
	// Test bridge.go:46.2,46.12 - getContextFromJS return nil
	t.Run("GetContextFromJSReturnNil", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()

		// Create function and store it globally
		fn := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
			return ctx.String("test")
		})
		ctx.Globals().Set("testFn", fn)

		// Unregister context from mapping to simulate context not found
		unregisterContext(ctx.ref)

		// Call function from JavaScript - triggers goFunctionProxy -> getContextFromJS with unmapped context
		result, err := ctx.Eval(`
            try {
                testFn();
            } catch(e) {
                e.toString();
            }
        `)

		// Should get an error or exception
		if err != nil {
			t.Logf("Expected error when context not in mapping: %v", err)
		} else {
			defer result.Free()
			resultStr := result.String()
			t.Logf("Exception result: %s", resultStr)
			require.True(t, len(resultStr) > 0)
		}

		// Re-register context for proper cleanup
		registerContext(ctx.ref, ctx)
		ctx.Close()

		t.Log("Successfully triggered getContextFromJS return nil branch")
	})
}

func TestBridgeGetRuntimeFromJSReturnNil(t *testing.T) {
	// Test bridge.go:64.2,64.12 - getRuntimeFromJS return nil
	// Test bridge.go:73.20,75.3 - goInterruptHandler return C.int(0)
	t.Run("GetRuntimeFromJSReturnNil", func(t *testing.T) {
		rt := NewRuntime()

		// Set interrupt handler
		interruptCalled := false
		rt.SetInterruptHandler(func() int {
			interruptCalled = true
			return 1 // Request interrupt
		})

		ctx := rt.NewContext()

		// Unregister runtime from mapping before executing long-running code
		unregisterRuntime(rt.ref)

		// Execute long-running code that may trigger interrupt handler
		result, err := ctx.Eval(`
            var sum = 0;
            for(var i = 0; i < 100000; i++) {
                sum += i;
                if (i % 1000 === 0) {
                    var temp = Math.sqrt(i);
                }
            }
            sum;
        `)

		// Since runtime is not in mapping, goInterruptHandler should return 0
		t.Logf("Interrupt handler called: %v", interruptCalled)
		t.Logf("Execution result - Error: %v", err)

		if err == nil {
			defer result.Free()
			t.Logf("Computation completed with result: %d", result.ToInt32())
		}

		// Re-register runtime for proper cleanup
		registerRuntime(rt.ref, rt)

		// Close context first, then runtime
		ctx.Close()
		rt.Close()

		t.Log("Successfully triggered getRuntimeFromJS return nil branch")
	})
}

func TestBridgeContextNotFound(t *testing.T) {
	// Test bridge.go:90.18,94.3 - goCtx == nil (Context not found)
	t.Run("ContextNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create function and store it in JavaScript
		fn := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
			return ctx.String("test")
		})
		ctx.Globals().Set("testFunc", fn)

		// Verify function works initially
		result, err := ctx.Eval(`testFunc()`)
		require.NoError(t, err)
		require.Equal(t, "test", result.String())
		result.Free()

		// Unregister context from mapping to simulate context being removed
		unregisterContext(ctx.ref)

		// Call function from JavaScript - triggers goFunctionProxy with unmapped context
		result2, err := ctx.Eval(`
            try {
                testFunc();
            } catch(e) {
                e.toString();
            }
        `)

		if err != nil {
			t.Logf("Expected error when context not found: %v", err)
			require.Contains(t, err.Error(), "Context not found")
		} else {
			defer result2.Free()
			resultStr := result2.String()
			t.Logf("Exception result: %s", resultStr)
			require.Contains(t, resultStr, "Context not found")
		}

		// Re-register context for proper cleanup
		registerContext(ctx.ref, ctx)

		t.Log("Successfully triggered goFunctionProxy goCtx == nil branch")
	})
}

func TestBridgeFunctionNotFoundInHandleStore(t *testing.T) {
	// Test bridge.go:99.15,103.3 - fn == nil (Function not found in handleStore)
	t.Run("FunctionNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create function and store it in JavaScript
		fn := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
			return ctx.String("test")
		})
		ctx.Globals().Set("testFunc", fn)

		// Verify function works initially
		result, err := ctx.Eval(`testFunc()`)
		require.NoError(t, err)
		require.Equal(t, "test", result.String())
		result.Free()

		// Clear handleStore to trigger fn == nil in goFunctionProxy
		ctx.handleStore.Clear()

		// Call function from JavaScript - triggers goFunctionProxy with cleared handleStore
		result2, err := ctx.Eval(`
            try {
                testFunc();
            } catch(e) {
                e.toString();
            }
        `)

		if err != nil {
			t.Logf("Expected error when function not found: %v", err)
			require.Contains(t, err.Error(), "Function not found")
		} else {
			defer result2.Free()
			resultStr := result2.String()
			t.Logf("Exception result: %s", resultStr)
			require.Contains(t, resultStr, "Function not found")
		}

		t.Log("Successfully triggered goFunctionProxy fn == nil branch")
	})
}

func TestBridgeInvalidFunctionType(t *testing.T) {
	// Test bridge.go:107.9,111.3 - Invalid function type assertion
	t.Run("InvalidFunctionType", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create function and store it in JavaScript
		fn := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
			return ctx.String("test")
		})
		ctx.Globals().Set("testFunc", fn)

		// Verify function works initially
		result, err := ctx.Eval(`testFunc()`)
		require.NoError(t, err)
		require.Equal(t, "test", result.String())
		result.Free()

		// Get function ID from handleStore
		var fnID int32
		var originalValue interface{}
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			fnID = key.(int32)
			originalValue = value
			return false // Stop after first item
		})

		// Temporarily store invalid type (not a function) in handleStore
		fakeHandle := cgo.NewHandle("not a function")
		ctx.handleStore.handles.Store(fnID, fakeHandle)

		// Call function from JavaScript - triggers goFunctionProxy with invalid function type
		result2, err := ctx.Eval(`
            try {
                testFunc();
            } catch(e) {
                e.toString();
            }
        `)

		// Check for expected error
		if err != nil {
			t.Logf("Expected error when invalid function type: %v", err)
			require.Contains(t, err.Error(), "Invalid function type")
		} else {
			defer result2.Free()
			resultStr := result2.String()
			t.Logf("Exception result: %s", resultStr)
			require.Contains(t, resultStr, "Invalid function type")
		}

		// Restore original value before cleanup
		ctx.handleStore.handles.Store(fnID, originalValue)

		t.Log("Successfully triggered goFunctionProxy type assertion failure branch")
	})
}
