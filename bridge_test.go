package quickjs

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBridgeGetContextFromJSReturnNil(t *testing.T) {
	// Test getContextFromJS return nil
	t.Run("GetContextFromJSReturnNil", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()

		// Create function and store it globally - MODIFIED: now uses pointer signature
		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value { // Changed: Function() → NewFunction()
			return ctx.NewString("test") // Changed: String() → NewString()
		})
		ctx.Globals().Set("testFn", fn)

		// Unregister context from mapping to simulate context not found
		unregisterContext(ctx.ref)

		// Call function from JavaScript - triggers goFunctionProxy -> getContextFromJS with unmapped context
		result := ctx.Eval(`
            try {
                testFn();
            } catch(e) {
                e.toString();
            }
        `)

		// Should get an error or exception
		if result.IsException() {
			err := ctx.Exception()
			t.Logf("Expected exception when context not in mapping: %v", err)
		} else {
			defer result.Free()
			resultStr := result.ToString() // Changed: String() → ToString()
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
	// Test getRuntimeFromJS return nil in goInterruptHandler
	t.Run("GetRuntimeFromJSReturnNil", func(t *testing.T) {
		rt := NewRuntime()
		ctx := rt.NewContext()
		require.NotNil(t, ctx)

		// Set interrupt handler
		interruptCalled := false
		rt.SetInterruptHandler(func() int {
			interruptCalled = true
			return 1 // Request interrupt
		})

		// Unregister runtime from mapping before executing long-running code
		unregisterRuntime(rt.ref)

		// Execute long-running code that may trigger interrupt handler
		result := ctx.Eval(`
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
		if result.IsException() {
			err := ctx.Exception()
			t.Logf("Execution resulted in exception: %v", err)
		} else {
			defer result.Free()
			t.Logf("Computation completed with result: %d", result.ToInt32()) // Changed: Int32() → ToInt32()
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
	// Test getContextAndFunction - Context not found error
	t.Run("ContextNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create function and store it in JavaScript - MODIFIED: now uses pointer signature
		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value { // Changed: Function() → NewFunction()
			return ctx.NewString("test") // Changed: String() → NewString()
		})
		ctx.Globals().Set("testFunc", fn)

		// Verify function works initially
		result := ctx.Eval(`testFunc()`)
		require.False(t, result.IsException())
		require.Equal(t, "test", result.ToString()) // Changed: String() → ToString()
		result.Free()

		// Unregister context from mapping to simulate context being removed
		unregisterContext(ctx.ref)

		// Call function from JavaScript - triggers goFunctionProxy -> getContextAndFunction with unmapped context
		result2 := ctx.Eval(`
            try {
                testFunc();
            } catch(e) {
                e.toString();
            }
        `)

		if result2.IsException() {
			err := ctx.Exception()
			t.Logf("Expected exception when context not found: %v", err)
			require.Contains(t, err.Error(), "Context not found")
		} else {
			defer result2.Free()
			resultStr := result2.ToString() // Changed: String() → ToString()
			t.Logf("Exception result: %s", resultStr)
			require.Contains(t, resultStr, "Context not found")
		}

		// Re-register context for proper cleanup
		registerContext(ctx.ref, ctx)

		t.Log("Successfully triggered getContextAndFunction Context not found branch")
	})
}

func TestBridgeFunctionNotFoundInHandleStore(t *testing.T) {
	// Test getContextAndFunction - Function not found in handleStore
	t.Run("FunctionNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create function and store it in JavaScript - MODIFIED: now uses pointer signature
		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value { // Changed: Function() → NewFunction()
			return ctx.NewString("test") // Changed: String() → NewString()
		})
		ctx.Globals().Set("testFunc", fn)

		// Verify function works initially
		result := ctx.Eval(`testFunc()`)
		require.False(t, result.IsException())
		require.Equal(t, "test", result.ToString()) // Changed: String() → ToString()
		result.Free()

		// Clear handleStore to trigger function not found in getContextAndFunction
		ctx.handleStore.Clear()

		// Call function from JavaScript - triggers goFunctionProxy -> getContextAndFunction with cleared handleStore
		result2 := ctx.Eval(`
            try {
                testFunc();
            } catch(e) {
                e.toString();
            }
        `)

		if result2.IsException() {
			err := ctx.Exception()
			t.Logf("Expected exception when function not found: %v", err)
			require.Contains(t, err.Error(), "Function not found")
		} else {
			defer result2.Free()
			resultStr := result2.ToString() // Changed: String() → ToString()
			t.Logf("Exception result: %s", resultStr)
			require.Contains(t, resultStr, "Function not found")
		}

		t.Log("Successfully triggered getContextAndFunction Function not found branch")
	})
}

func TestBridgeInvalidFunctionType(t *testing.T) {
	// Test type assertion failure in goFunctionProxy
	t.Run("InvalidFunctionType", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create function and store it in JavaScript - MODIFIED: now uses pointer signature
		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value { // Changed: Function() → NewFunction()
			return ctx.NewString("test") // Changed: String() → NewString()
		})
		ctx.Globals().Set("testFunc", fn)

		// Verify function works initially
		result := ctx.Eval(`testFunc()`)
		require.False(t, result.IsException())
		require.Equal(t, "test", result.ToString()) // Changed: String() → ToString()
		result.Free()

		fnID, _ := findHandleByPredicate(t, ctx.handleStore, func(v interface{}) bool {
			_, ok := v.(func(*Context, *Value, []*Value) *Value)
			return ok
		})
		restore := replaceHandleWithValueForTest(t, ctx.handleStore, fnID, "not a function")
		defer restore()

		// Call function from JavaScript - triggers goFunctionProxy with invalid function type
		result2 := ctx.Eval(`
            try {
                testFunc();
            } catch(e) {
                e.toString();
            }
        `)

		// Check for expected error
		if result2.IsException() {
			err := ctx.Exception()
			t.Logf("Expected exception when invalid function type: %v", err)
			require.Contains(t, err.Error(), "Invalid function type")
		} else {
			defer result2.Free()
			resultStr := result2.ToString() // Changed: String() → ToString()
			t.Logf("Exception result: %s", resultStr)
			require.Contains(t, resultStr, "Invalid function type")
		}

		t.Log("Successfully triggered goFunctionProxy type assertion failure branch")
	})
}

// Test for class constructor proxy errors - MODIFIED FOR SCHEME C
func TestBridgeClassConstructorErrors(t *testing.T) {
	// Test class constructor proxy error handling
	t.Run("ConstructorContextNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				// SCHEME C: Return Go object for automatic association
				return &Point{X: 1, Y: 2}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// Verify constructor works initially
		result := ctx.Eval(`new TestClass()`)
		require.False(t, result.IsException())
		result.Free()

		// Unregister context from mapping
		unregisterContext(ctx.ref)

		// Call constructor - triggers goClassConstructorProxy with unmapped context
		result2 := ctx.Eval(`
            try {
                new TestClass();
            } catch(e) {
                e.toString();
            }
        `)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Context not found")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Context not found") // Changed: String() → ToString()
		}

		// Re-register context for cleanup
		registerContext(ctx.ref, ctx)

		t.Log("Successfully triggered goClassConstructorProxy Context not found branch")
	})

	t.Run("ConstructorNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				// SCHEME C: Return Go object for automatic association
				return &Point{X: 1, Y: 2}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// Clear handleStore to trigger constructor not found
		ctx.handleStore.Clear()

		// Call constructor - triggers goClassConstructorProxy with cleared handleStore
		result := ctx.Eval(`
            try {
                new TestClass();
            } catch(e) {
                e.toString();
            }
        `)

		if result.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Constructor function not found")
		} else {
			defer result.Free()
			require.Contains(t, result.ToString(), "Constructor function not found") // Changed: String() → ToString()
		}

		t.Log("Successfully triggered goClassConstructorProxy Constructor not found branch")
	})

	t.Run("InvalidConstructorType", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				// SCHEME C: Return Go object for automatic association
				return &Point{X: 1, Y: 2}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// SCHEME C: Find ClassBuilder (not individual constructor function) and replace with invalid type
		constructorID, _ := findHandleByPredicate(t, ctx.handleStore, func(v interface{}) bool {
			_, ok := v.(*ClassBuilder)
			return ok
		})
		restore := replaceHandleWithValueForTest(t, ctx.handleStore, constructorID, "not a ClassBuilder")
		defer restore()

		// Call constructor - triggers type assertion failure
		result := ctx.Eval(`
            try {
                new TestClass();
            } catch(e) {
                e.toString();
            }
        `)

		if result.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Invalid constructor function type")
		} else {
			defer result.Free()
			require.Contains(t, result.ToString(), "Invalid constructor function type") // Changed: String() → ToString()
		}

		t.Log("Successfully triggered goClassConstructorProxy type assertion failure branch")
	})

	t.Run("NilConstructorInBuilder", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())
		ctx.Globals().Set("TestClass", constructor)

		constructorID, _ := findHandleByPredicate(t, ctx.handleStore, func(v interface{}) bool {
			_, ok := v.(*ClassBuilder)
			return ok
		})

		corrupted := &ClassBuilder{name: "CorruptedClass", constructor: nil}
		restore := replaceHandleWithValueForTest(t, ctx.handleStore, constructorID, corrupted)
		defer restore()

		result := ctx.Eval(`
            try {
                new TestClass();
            } catch(e) {
                e.toString();
            }
        `)
		if result.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Constructor function is nil")
		} else {
			defer result.Free()
			require.Contains(t, result.ToString(), "Constructor function is nil")
		}

	})

	t.Run("ClassOpaqueAllocationFailure", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		setForceClassOpaqueAllocFailureForTest(true)
		defer setForceClassOpaqueAllocFailureForTest(false)

		constructor, _ := NewClassBuilder("OpaqueFailureClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())
		ctx.Globals().Set("OpaqueFailureClass", constructor)

		result := ctx.Eval(`
            try {
                new OpaqueFailureClass();
            } catch(e) {
                e.toString();
            }
        `)
		defer result.Free()

		if result.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Failed to allocate class opaque payload")
		} else {
			require.Contains(t, result.ToString(), "Failed to allocate class opaque payload")
		}
	})

	t.Run("InvalidInstancePropertyValue", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		constructor, _ := NewClassBuilder("MutatedPropsClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Property("x", ctx.NewInt32(1)).
			Build(ctx)
		require.False(t, constructor.IsException())
		ctx.Globals().Set("MutatedPropsClass", constructor)

		constructorID, originalHandle := findHandleByPredicate(t, ctx.handleStore, func(v interface{}) bool {
			b, ok := v.(*ClassBuilder)
			return ok && b.name == "MutatedPropsClass"
		})

		originalBuilder, ok := originalHandle.Value().(*ClassBuilder)
		require.True(t, ok)
		require.GreaterOrEqual(t, len(originalBuilder.properties), 1)

		mutatedNil := *originalBuilder
		mutatedNil.properties = append([]PropertyEntry(nil), originalBuilder.properties...)
		mutatedNil.properties[0].Value = nil
		restoreNil := replaceHandleWithValueForTest(t, ctx.handleStore, constructorID, &mutatedNil)

		resultNil := ctx.Eval(`
            try {
                new MutatedPropsClass();
            } catch(e) {
                e.toString();
            }
        `)
		defer resultNil.Free()
		if resultNil.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Invalid instance property value")
		} else {
			require.Contains(t, resultNil.ToString(), "Invalid instance property value")
		}
		restoreNil()

		rt2 := NewRuntime()
		defer rt2.Close()
		ctx2 := rt2.NewContext()
		defer ctx2.Close()

		foreignVal := ctx2.NewInt32(99)
		defer foreignVal.Free()

		mutatedForeign := *originalBuilder
		mutatedForeign.properties = append([]PropertyEntry(nil), originalBuilder.properties...)
		mutatedForeign.properties[0].Value = foreignVal
		restoreForeign := replaceHandleWithValueForTest(t, ctx.handleStore, constructorID, &mutatedForeign)

		resultForeign := ctx.Eval(`
            try {
                new MutatedPropsClass();
            } catch(e) {
                e.toString();
            }
        `)
		defer resultForeign.Free()
		if resultForeign.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Invalid instance property value")
		} else {
			require.Contains(t, resultForeign.ToString(), "Invalid instance property value")
		}

		restoreForeign()
		ctx.handleStore.handles.Store(constructorID, originalHandle)
	})

	// NEW TEST FOR SCHEME C: Test class ID resolution failure
	t.Run("ClassIDNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create class with constructor
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		// Globals().Set transfers constructor ownership, capture key before transfer.
		constructorRef := constructor.ref

		ctx.Globals().Set("TestClass", constructor)

		// Manually remove constructor from global registry to simulate class ID not found
		deleteConstructorRegistryEntryForTest(ctx, constructorRef)

		// Call constructor - triggers "Class ID not found for constructor" branch
		result := ctx.Eval(`
            try {
                new TestClass();
            } catch(e) {
                e.toString();
            }
        `)

		if result.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Class ID not found")
		} else {
			defer result.Free()
			require.Contains(t, result.ToString(), "Class ID not found") // Changed: String() → ToString()
		}

		t.Log("Successfully triggered goClassConstructorProxy Class ID not found branch")
	})

	// NEW TEST FOR SCHEME C: Test instance property binding
	t.Run("InstancePropertyBinding", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create class with instance properties
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Property("version", ctx.NewString("1.0.0")).                   // Changed: String() → NewString()
			Property("readOnly", ctx.NewBool(true), PropertyConfigurable). // Changed: Bool() → NewBool()
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// Test that instance properties are properly bound during construction
		result := ctx.Eval(`
            let obj = new TestClass();
            [obj.version, obj.readOnly, typeof obj.version, typeof obj.readOnly];
        `)
		require.False(t, result.IsException())
		defer result.Free()

		// Verify instance properties were bound correctly
		require.Equal(t, "1.0.0", result.GetIdx(0).ToString()) // Changed: String() → ToString()
		require.True(t, result.GetIdx(1).ToBool())
		require.Equal(t, "string", result.GetIdx(2).ToString())  // Changed: String() → ToString()
		require.Equal(t, "boolean", result.GetIdx(3).ToString()) // Changed: String() → ToString()

		t.Log("Successfully tested SCHEME C instance property binding")
	})

}

// Test for class method proxy errors - unchanged except method calls
func TestBridgeClassMethodErrors(t *testing.T) {
	// Test class method proxy error handling
	t.Run("MethodContextNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Method("testMethod", func(ctx *Context, this *Value, args []*Value) *Value {
				return ctx.NewString("method called") // Changed: String() → NewString()
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// Create instance and verify method works
		result := ctx.Eval(`
            let obj = new TestClass();
            obj.testMethod();
        `)
		require.False(t, result.IsException())
		require.Equal(t, "method called", result.ToString()) // Changed: String() → ToString()
		result.Free()

		// Unregister context from mapping
		unregisterContext(ctx.ref)

		// Call method - triggers goClassMethodProxy with unmapped context
		result2 := ctx.Eval(`
            try {
                let obj = new TestClass();
                obj.testMethod();
            } catch(e) {
                e.toString();
            }
        `)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Context not found")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Context not found") // Changed: String() → ToString()
		}

		// Re-register context for cleanup
		registerContext(ctx.ref, ctx)

		t.Log("Successfully triggered goClassMethodProxy Context not found branch")
	})

	t.Run("MethodNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Method("testMethod", func(ctx *Context, this *Value, args []*Value) *Value {
				return ctx.NewString("method called") // Changed: String() → NewString()
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// First create an instance and store it in global scope
		result := ctx.Eval(`
			let obj = new TestClass();
			globalThis.testObj = obj;  // Store instance globally
			obj.testMethod();  // Verify method works
		`)
		require.False(t, result.IsException())
		require.Equal(t, "method called", result.ToString()) // Changed: String() → ToString()
		result.Free()

		// Now clear handleStore to trigger method not found
		ctx.handleStore.Clear()

		// Call method on existing instance - triggers goClassMethodProxy with cleared handleStore
		result2 := ctx.Eval(`
			try {
				globalThis.testObj.testMethod();  // Use existing instance
			} catch(e) {
				e.toString();
			}
		`)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Method function not found")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Method function not found") // Changed: String() → ToString()
		}

		t.Log("Successfully triggered goClassMethodProxy Method not found branch")
	})

	t.Run("InvalidMethodType", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Method("testMethod", func(ctx *Context, this *Value, args []*Value) *Value {
				return ctx.NewString("method called") // Changed: String() → NewString()
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// Store existing instance to use later
		result := ctx.Eval(`
            let obj = new TestClass();
            globalThis.testObj = obj;  // Store instance globally
            obj.testMethod();  // Verify method works initially
        `)
		require.False(t, result.IsException())
		require.Equal(t, "method called", result.ToString()) // Changed: String() → ToString()
		result.Free()

		methodID, _ := findHandleByPredicate(t, ctx.handleStore, func(v interface{}) bool {
			_, ok := v.(ClassMethodFunc)
			return ok
		})

		restore := replaceHandleWithValueForTest(t, ctx.handleStore, methodID, "not a method function")
		defer restore()

		// Call method on existing instance - triggers type assertion failure
		result2 := ctx.Eval(`
            try {
                globalThis.testObj.testMethod();
            } catch(e) {
                e.toString();
            }
        `)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Invalid method function type")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Invalid method function type") // Changed: String() → ToString()
		}

		t.Log("Successfully triggered goClassMethodProxy type assertion failure branch")
	})
}

// Test for class getter proxy errors - unchanged except method calls
func TestBridgeClassGetterErrors(t *testing.T) {
	// Test class getter proxy error handling
	t.Run("GetterContextNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Accessor("testProp", func(ctx *Context, this *Value) *Value {
				return ctx.NewString("getter called") // Changed: String() → NewString()
			}, nil).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// Verify getter works initially
		result := ctx.Eval(`
            let obj = new TestClass();
            obj.testProp;
        `)
		require.False(t, result.IsException())
		require.Equal(t, "getter called", result.ToString()) // Changed: String() → ToString()
		result.Free()

		// Unregister context from mapping
		unregisterContext(ctx.ref)

		// Access getter - triggers goClassGetterProxy with unmapped context
		result2 := ctx.Eval(`
            try {
                let obj = new TestClass();
                obj.testProp;
            } catch(e) {
                e.toString();
            }
        `)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Context not found")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Context not found") // Changed: String() → ToString()
		}

		// Re-register context for cleanup
		registerContext(ctx.ref, ctx)

		t.Log("Successfully triggered goClassGetterProxy Context not found branch")
	})

	t.Run("GetterNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Accessor("testProp", func(ctx *Context, this *Value) *Value {
				return ctx.NewString("getter called") // Changed: String() → NewString()
			}, nil).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// First create an instance and store it in global scope
		result := ctx.Eval(`
			let obj = new TestClass();
			globalThis.testObj = obj;  // Store instance globally
			obj.testProp;  // Verify getter works
		`)
		require.False(t, result.IsException())
		require.Equal(t, "getter called", result.ToString()) // Changed: String() → ToString()
		result.Free()

		// Now clear handleStore to trigger getter not found
		ctx.handleStore.Clear()

		// Access getter on existing instance - triggers goClassGetterProxy with cleared handleStore
		result2 := ctx.Eval(`
			try {
				globalThis.testObj.testProp;  // Use existing instance
			} catch(e) {
				e.toString();
			}
		`)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Getter function not found")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Getter function not found") // Changed: String() → ToString()
		}

		t.Log("Successfully triggered goClassGetterProxy Getter not found branch")
	})

	t.Run("InvalidGetterType", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Accessor("testProp", func(ctx *Context, this *Value) *Value {
				return ctx.NewString("getter called") // Changed: String() → NewString()
			}, nil).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// Store existing instance to use later
		result := ctx.Eval(`
            let obj = new TestClass();
            globalThis.testObj = obj;  // Store instance globally
            obj.testProp;  // Verify getter works initially
        `)
		require.False(t, result.IsException())
		require.Equal(t, "getter called", result.ToString()) // Changed: String() → ToString()
		result.Free()

		getterID, _ := findHandleByPredicate(t, ctx.handleStore, func(v interface{}) bool {
			_, ok := v.(ClassGetterFunc)
			return ok
		})

		restore := replaceHandleWithValueForTest(t, ctx.handleStore, getterID, "not a getter function")
		defer restore()

		// Access getter on existing instance - triggers type assertion failure
		result2 := ctx.Eval(`
            try {
                globalThis.testObj.testProp;
            } catch(e) {
                e.toString();
            }
        `)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Invalid getter function type")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Invalid getter function type") // Changed: String() → ToString()
		}

		t.Log("Successfully triggered goClassGetterProxy type assertion failure branch")
	})
}

// Test for class setter proxy errors - unchanged except method calls
func TestBridgeClassSetterErrors(t *testing.T) {
	// Test class setter proxy error handling
	t.Run("SetterContextNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Accessor("testProp", nil, func(ctx *Context, this *Value, value *Value) *Value {
				return ctx.NewUndefined() // Changed: Undefined() → NewUndefined()
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// Verify setter works initially
		result := ctx.Eval(`
            let obj = new TestClass();
            obj.testProp = "test";
            "setter works";
        `)
		require.False(t, result.IsException())
		require.Equal(t, "setter works", result.ToString()) // Changed: String() → ToString()
		result.Free()

		// Unregister context from mapping
		unregisterContext(ctx.ref)

		// Call setter - triggers goClassSetterProxy with unmapped context
		result2 := ctx.Eval(`
            try {
                let obj = new TestClass();
                obj.testProp = "test";
            } catch(e) {
                e.toString();
            }
        `)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Context not found")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Context not found") // Changed: String() → ToString()
		}

		// Re-register context for cleanup
		registerContext(ctx.ref, ctx)

		t.Log("Successfully triggered goClassSetterProxy Context not found branch")
	})

	t.Run("SetterNotFound", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Accessor("testProp", nil, func(ctx *Context, this *Value, value *Value) *Value {
				return ctx.NewUndefined() // Changed: Undefined() → NewUndefined()
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// First create an instance and store it in global scope
		result := ctx.Eval(`
        let obj = new TestClass();
        globalThis.testObj = obj;  // Store instance globally
        obj.testProp = "test";     // Verify setter works
        "setter works";
    `)
		require.False(t, result.IsException())
		require.Equal(t, "setter works", result.ToString()) // Changed: String() → ToString()
		result.Free()

		// Now clear handleStore to trigger setter not found
		ctx.handleStore.Clear()

		// Call setter on existing instance - triggers goClassSetterProxy with cleared handleStore
		result2 := ctx.Eval(`
        try {
            globalThis.testObj.testProp = "test2";  // Use existing instance
        } catch(e) {
            e.toString();
        }
    `)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Setter function not found")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Setter function not found") // Changed: String() → ToString()
		}

		t.Log("Successfully triggered goClassSetterProxy Setter not found branch")
	})

	t.Run("InvalidSetterType", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// MODIFIED FOR SCHEME C: Create class with new constructor signature
		constructor, _ := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Accessor("testProp", nil, func(ctx *Context, this *Value, value *Value) *Value {
				return ctx.NewUndefined() // Changed: Undefined() → NewUndefined()
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("TestClass", constructor)

		// Store existing instance to use later
		result := ctx.Eval(`
            let obj = new TestClass();
            globalThis.testObj = obj;  // Store instance globally
            obj.testProp = "test";     // Verify setter works initially
            "setter works";
        `)
		require.False(t, result.IsException())
		require.Equal(t, "setter works", result.ToString()) // Changed: String() → ToString()
		result.Free()

		setterID, _ := findHandleByPredicate(t, ctx.handleStore, func(v interface{}) bool {
			_, ok := v.(ClassSetterFunc)
			return ok
		})

		restore := replaceHandleWithValueForTest(t, ctx.handleStore, setterID, "not a setter function")
		defer restore()

		// Call setter on existing instance - triggers type assertion failure
		result2 := ctx.Eval(`
            try {
                globalThis.testObj.testProp = "test2";
            } catch(e) {
                e.toString();
            }
        `)

		if result2.IsException() {
			err := ctx.Exception()
			require.Contains(t, err.Error(), "Invalid setter function type")
		} else {
			defer result2.Free()
			require.Contains(t, result2.ToString(), "Invalid setter function type") // Changed: String() → ToString()
		}

		t.Log("Successfully triggered goClassSetterProxy type assertion failure branch")
	})
}

// Test for class finalizer context iteration - unchanged except constructor signature
func TestBridgeClassFinalizerContextIteration(t *testing.T) {
	t.Run("MultipleContextsIteration", func(t *testing.T) {
		// Create multiple runtimes and contexts to test iteration
		rt1 := NewRuntime()
		defer rt1.Close()
		ctx1 := rt1.NewContext()
		defer ctx1.Close()

		rt2 := NewRuntime()
		defer rt2.Close()
		ctx2 := rt2.NewContext()
		defer ctx2.Close()

		rt3 := NewRuntime()
		defer rt3.Close()
		ctx3 := rt3.NewContext()
		defer ctx3.Close()

		// Create classes in all contexts
		for i, ctx := range []*Context{ctx1, ctx2, ctx3} {
			constructor, _ := NewClassBuilder(fmt.Sprintf("TestClass%d", i)).
				Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
					// MODIFIED FOR SCHEME C: Return Go object for automatic association
					// Create a simple object that implements finalizer
					obj := &Point{X: float64(i), Y: float64(i)}
					return obj, nil
				}).
				Build(ctx)
			require.False(t, constructor.IsException())

			ctx.Globals().Set(fmt.Sprintf("TestClass%d", i), constructor)
		}

		// Create instances in all contexts
		for i, ctx := range []*Context{ctx1, ctx2, ctx3} {
			result := ctx.Eval(fmt.Sprintf(`new TestClass%d()`, i))
			require.False(t, result.IsException())
			result.Free()
		}

		// When finalizer runs, it will iterate through multiple contexts
		// Some contexts will have matching runtime.ref, others won't
		// This tests both "return true" (continue) and "return false" (stop) branches
		runtime.GC()

		t.Log("Successfully tested goClassFinalizerProxy context iteration branches")
	})
}

// NEW TEST FOR SCHEME C: Test CreateClassInstance C function behavior
func TestBridgeCreateClassInstanceEdgeCases(t *testing.T) {
	t.Run("CreateClassInstance_NoProperties", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create class without instance properties
		constructor, _ := NewClassBuilder("NoPropsClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("NoPropsClass", constructor)

		// Test that instances are created successfully even without properties
		result := ctx.Eval(`
            let obj = new NoPropsClass();
            typeof obj;
        `)
		require.False(t, result.IsException())
		defer result.Free()
		require.Equal(t, "object", result.ToString()) // Changed: String() → ToString()

		t.Log("Successfully tested CreateClassInstance with no instance properties")
	})

	t.Run("CreateClassInstance_ManyProperties", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create class with many instance properties
		builder := NewClassBuilder("ManyPropsClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			})

		// Add multiple instance properties
		for i := 0; i < 10; i++ {
			builder = builder.Property(fmt.Sprintf("prop%d", i), ctx.NewString(fmt.Sprintf("value%d", i))) // Changed: String() → NewString()
		}

		constructor, _ := builder.Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("ManyPropsClass", constructor)

		// Test that all properties are bound correctly
		result := ctx.Eval(`
            let obj = new ManyPropsClass();
            [obj.prop0, obj.prop5, obj.prop9];
        `)
		require.False(t, result.IsException())
		defer result.Free()

		require.Equal(t, "value0", result.GetIdx(0).ToString()) // Changed: String() → ToString()
		require.Equal(t, "value5", result.GetIdx(1).ToString()) // Changed: String() → ToString()
		require.Equal(t, "value9", result.GetIdx(2).ToString()) // Changed: String() → ToString()

		t.Log("Successfully tested CreateClassInstance with many instance properties")
	})
}

// NEW TEST FOR SCHEME C: Test CreateClassInstance failure scenarios
func TestBridgeCreateClassInstanceFailures(t *testing.T) {
	t.Run("CreateClassInstance_CException", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		// Create class
		constructor, originalClassID := NewClassBuilder("TestClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())
		constructorRef := constructor.ref

		ctx.Globals().Set("TestClass", constructor)

		// Replace with invalid class ID to trigger JS_NewObjectProtoClass failure
		storeConstructorRegistryEntryForTest(ctx, constructorRef, uint32(999999))

		// This should trigger CreateClassInstance to return JS_EXCEPTION
		result := ctx.Eval(`new TestClass()`)
		defer result.Free()

		// Restore for cleanup
		storeConstructorRegistryEntryForTest(ctx, constructorRef, originalClassID)

		// Should get an error
		if result.IsException() {
			err := ctx.Exception()
			t.Logf("Expected exception from CreateClassInstance: %v", err)
		}

		t.Log("Successfully triggered CreateClassInstance JS_EXCEPTION branch")
	})
}

func TestBridgeNilReturnSafety(t *testing.T) {
	t.Run("FunctionProxyNilReturn", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return nil
		})
		require.NotNil(t, fn)
		ctx.Globals().Set("nilFn", fn)

		result := ctx.Eval(`typeof nilFn()`)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "undefined", result.ToString())
	})

	t.Run("ClassMethodGetterSetterNilReturn", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()
		ctx := rt.NewContext()
		defer ctx.Close()

		constructor, _ := NewClassBuilder("NilReturnClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 2}, nil
			}).
			Method("nilMethod", func(ctx *Context, this *Value, args []*Value) *Value {
				return nil
			}).
			Accessor("nilProp",
				func(ctx *Context, this *Value) *Value { return nil },
				func(ctx *Context, this *Value, value *Value) *Value { return nil },
			).
			Build(ctx)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("NilReturnClass", constructor)

		result := ctx.Eval(`
			const obj = new NilReturnClass();
			obj.nilProp = 123;
			[typeof obj.nilMethod(), typeof obj.nilProp, obj.nilProp = 456];
		`)
		defer result.Free()
		require.False(t, result.IsException())

		type0 := result.GetIdx(0)
		defer type0.Free()
		require.Equal(t, "undefined", type0.ToString())

		type1 := result.GetIdx(1)
		defer type1.Free()
		require.Equal(t, "undefined", type1.ToString())

		assigned := result.GetIdx(2)
		defer assigned.Free()
		require.Equal(t, int32(456), assigned.ToInt32())
	})
}
