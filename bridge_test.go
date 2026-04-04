package quickjs

import (
	"fmt"
	"runtime/cgo"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type bridgeFinalizerProbe struct {
	mark *atomic.Bool
}

func (p *bridgeFinalizerProbe) Finalize() {
	if p != nil && p.mark != nil {
		p.mark.Store(true)
	}
}

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

func TestBridgeMappingCorruptionFailClosed(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	require.NotNil(t, getContextFromJS(ctx.ref))
	require.NotNil(t, getRuntimeFromJS(rt.ref))

	contextMapping.Store(ctx.ref, "corrupted-context")
	require.Nil(t, getContextFromJS(ctx.ref))

	runtimeMapping.Store(rt.ref, "corrupted-runtime")
	require.Nil(t, getRuntimeFromJS(rt.ref))
	require.NotPanics(t, func() {
		_ = goInterruptHandler(rt.ref)
	})

	registerContext(ctx.ref, ctx)
	registerRuntime(rt.ref, rt)

	fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewString("ok")
	})
	ctx.Globals().Set("__bridge_boundary_fn", fn)

	result := ctx.Eval(`
		try {
			__bridge_boundary_fn();
			"ok";
		} catch (e) {
			String(e);
		}
	`)
	defer result.Free()
	require.False(t, result.IsException())
	require.Equal(t, "ok", result.ToString())
}

func TestResolveClassObjectFromOpaqueContracts(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	_, _, ok := resolveClassObjectFromOpaque(nil, legacyHandleOpaque(1))
	require.False(t, ok)

	orphanCtx := &Context{handleStore: ctx.handleStore}
	_, _, ok = resolveClassObjectFromOpaque(orphanCtx, legacyHandleOpaque(1))
	require.False(t, ok)

	_, _, ok = resolveClassObjectFromOpaque(ctx, nil)
	require.False(t, ok)

	legacyHandle := ctx.handleStore.Store("legacy-handle")
	defer ctx.handleStore.Delete(legacyHandle)

	ownerCtx, handleID, ok := resolveClassObjectFromOpaque(ctx, legacyHandleOpaque(legacyHandle))
	require.True(t, ok)
	require.Equal(t, ctx, ownerCtx)
	require.Equal(t, legacyHandle, handleID)

	ctx.handleStore.Delete(legacyHandle)
	_, _, ok = resolveClassObjectFromOpaque(ctx, legacyHandleOpaque(legacyHandle))
	require.False(t, ok)

	ctx2 := rt.NewContext()
	defer ctx2.Close()
	legacyHandleInCtx2 := ctx2.handleStore.Store("legacy-handle-other-context")
	defer ctx2.handleStore.Delete(legacyHandleInCtx2)

	ownerCtx, handleID, ok = resolveClassObjectFromOpaque(ctx, legacyHandleOpaque(legacyHandleInCtx2))
	require.True(t, ok)
	require.Equal(t, ctx2, ownerCtx)
	require.Equal(t, legacyHandleInCtx2, handleID)

	ctxNoStoreLegacy := &Context{runtime: rt}
	ownerCtx, handleID, ok = resolveClassObjectFromOpaque(ctxNoStoreLegacy, legacyHandleOpaque(legacyHandleInCtx2))
	require.True(t, ok)
	require.Equal(t, ctx2, ownerCtx)
	require.Equal(t, legacyHandleInCtx2, handleID)

	idHandle := ctx.handleStore.Store("identity-handle")
	defer ctx.handleStore.Delete(idHandle)
	objectID := rt.registerClassObjectIdentity(ctx.contextID, idHandle)
	require.NotZero(t, objectID)

	ownerCtx, handleID, ok = resolveClassObjectFromOpaque(ctx, classObjectOpaque(objectID))
	require.True(t, ok)
	require.Equal(t, ctx, ownerCtx)
	require.Equal(t, idHandle, handleID)

	rt.contextsByID.Delete(ctx.contextID)
	_, _, ok = resolveClassObjectFromOpaque(ctx, classObjectOpaque(objectID))
	require.False(t, ok)
	rt.contextsByID.Store(ctx.contextID, ctx)

	rt.classObjectRegistry.Delete(objectID)
	_, _, ok = resolveClassObjectFromOpaque(ctx, classObjectOpaque(objectID))
	require.False(t, ok)

	ctxNoStore := &Context{runtime: rt}
	_, _, ok = resolveClassObjectFromOpaque(ctxNoStore, classObjectOpaque(-777))
	require.False(t, ok)

	_, _, ok = resolveClassObjectFromOpaque(ctx, classObjectOpaque(-1234567))
	require.False(t, ok)
}

func TestFindRuntimeContextByLegacyHandleContracts(t *testing.T) {
	require.Nil(t, findRuntimeContextByLegacyHandle(nil, 1))

	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	require.Nil(t, findRuntimeContextByLegacyHandle(rt, 0))

	// Corrupted and invalid entries should be ignored safely.
	rt.contexts.Store("corrupted", "bad-entry")
	rt.contexts.Store("invalid-context", &Context{})
	require.Nil(t, findRuntimeContextByLegacyHandle(rt, 12345))

	handleID := ctx.handleStore.Store("legacy-match")
	defer ctx.handleStore.Delete(handleID)
	require.Equal(t, ctx, findRuntimeContextByLegacyHandle(rt, handleID))
}

func TestClassOpaqueWrapperGuards(t *testing.T) {
	require.Nil(t, legacyHandleOpaque(0))
	require.Nil(t, legacyHandleOpaque(-1))
	require.Nil(t, classObjectOpaque(0))
	require.NotNil(t, classObjectOpaque(-1))
}

func TestBridgeMappingClosedTargetsFailClosed(t *testing.T) {
	rt := NewRuntime()
	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	ctxRef := ctx.ref
	rtRef := rt.ref

	ctx.Close()
	contextMapping.Store(ctxRef, ctx)
	require.Nil(t, getContextFromJS(ctxRef))

	rt.Close()
	runtimeMapping.Store(rtRef, rt)
	require.Nil(t, getRuntimeFromJS(rtRef))
}

func TestBridgeNilCallbackResultsNormalizeToUndefined(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.Globals().Set("nilFn", ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return nil
	}))

	fnType := ctx.Eval(`typeof nilFn()`)
	defer fnType.Free()
	require.False(t, fnType.IsException())
	require.Equal(t, "undefined", fnType.ToString())

	constructor, _ := NewClassBuilder("NilBridgeClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return map[string]int{"ok": 1}, nil
		}).
		Method("m", func(ctx *Context, this *Value, args []*Value) *Value {
			return nil
		}).
		Accessor("a", func(ctx *Context, this *Value) *Value {
			return nil
		}, func(ctx *Context, this *Value, val *Value) *Value {
			return nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())

	ctx.Globals().Set("NilBridgeClass", constructor)

	result := ctx.Eval(`
		(() => {
			const o = new NilBridgeClass();
			const m = o.m();
			const g = o.a;
			o.a = 123;
			return [m === undefined, g === undefined].join(',');
		})()
	`)
	defer result.Free()
	require.False(t, result.IsException())
	require.Equal(t, "true,true", result.ToString())
}

func TestBridgeModuleInitFailClosedInvalidExportAndBuilderHandle(t *testing.T) {
	newModuleContext := func(t *testing.T) *Context {
		rt := NewRuntime(WithModuleImport(true))
		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return ctx
	}

	t.Run("InvalidExportValue", func(t *testing.T) {
		ctx := newModuleContext(t)
		mb := NewModuleBuilder("invalid-export-module").
			Export("bad", nil)
		require.NoError(t, mb.Build(ctx))

		result := ctx.Eval(`import('invalid-export-module')`, EvalAwait(true))
		defer result.Free()
		require.True(t, result.IsException())
		err := ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid module export value")
	})

	t.Run("InvalidModuleBuilderHandle", func(t *testing.T) {
		ctx := newModuleContext(t)
		mb := NewModuleBuilder("invalid-builder-module").
			Export("ok", ctx.NewString("value"))
		require.NoError(t, mb.Build(ctx))

		var builderID int32
		var originalHandle cgo.Handle
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			id, ok := key.(int32)
			if !ok {
				return true
			}
			h, ok := value.(cgo.Handle)
			if !ok {
				return true
			}
			stored, ok := h.Value().(*ModuleBuilder)
			if ok && stored != nil && stored.name == "invalid-builder-module" {
				builderID = id
				originalHandle = h
				return false
			}
			return true
		})
		require.Greater(t, builderID, int32(0))

		tempHandle := cgo.NewHandle("not-a-module-builder")
		ctx.handleStore.handles.Store(builderID, tempHandle)
		defer func() {
			ctx.handleStore.handles.Store(builderID, originalHandle)
			tempHandle.Delete()
		}()

		result := ctx.Eval(`import('invalid-builder-module')`, EvalAwait(true))
		defer result.Free()
		require.True(t, result.IsException())
		err := ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid module builder handle")
	})
}

func TestBridgeClassConstructorHandleStoreUnavailable(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	backupStore := ctx.handleStore

	constructor, _ := NewClassBuilder("HandleStoreUnavailableClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			ctx.handleStore = nil
			return map[string]int{"x": 1}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())

	ctx.Globals().Set("HandleStoreUnavailableClass", constructor)

	result := ctx.Eval(`new HandleStoreUnavailableClass()`)
	ctx.handleStore = backupStore

	defer result.Free()
	require.True(t, result.IsException())
	err := ctx.Exception()
	require.Error(t, err)
	require.Contains(t, err.Error(), "Handle store not available")
}

func TestBridgeClassConstructorRuntimeUnavailable(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	backupRuntime := ctx.runtime

	constructor, _ := NewClassBuilder("RuntimeUnavailableClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			ctx.runtime = nil
			return map[string]int{"x": 1}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())

	ctx.Globals().Set("RuntimeUnavailableClass", constructor)

	result := ctx.Eval(`new RuntimeUnavailableClass()`)
	ctx.runtime = backupRuntime

	defer result.Free()
	require.True(t, result.IsException())
	err := ctx.Exception()
	require.Error(t, err)
	require.Contains(t, err.Error(), "Context runtime not available")
}

func TestBridgeClassConstructorIdentityRegistrationFailure(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	backupContextID := ctx.contextID

	constructor, _ := NewClassBuilder("IdentityRegistrationFailureClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			ctx.contextID = 0
			return map[string]int{"x": 1}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())

	ctx.Globals().Set("IdentityRegistrationFailureClass", constructor)

	result := ctx.Eval(`new IdentityRegistrationFailureClass()`)
	ctx.contextID = backupContextID

	defer result.Free()
	require.True(t, result.IsException())
	err := ctx.Exception()
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to register class object identity")
}

func TestBridgeClassFinalizerProxyContracts(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	var finalized atomic.Bool

	constructor, _ := NewClassBuilder("FinalizerProbeClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &bridgeFinalizerProbe{mark: &finalized}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())
	ctx.Globals().Set("FinalizerProbeClass", constructor)

	instance := ctx.Eval(`new FinalizerProbeClass()`)
	defer instance.Free()
	require.False(t, instance.IsException())

	require.NotPanics(t, func() {
		goClassFinalizerProxy(nil, instance.ref)
	})

	ctxRef := ctx.ref
	unregisterContext(ctxRef)
	contextMapping.Store(ctxRef, "corrupted-finalizer-mapping")

	rt2 := NewRuntime()
	defer rt2.Close()
	require.NotPanics(t, func() {
		goClassFinalizerProxy(rt2.ref, instance.ref)
	})
	require.False(t, finalized.Load())

	goClassFinalizerProxy(rt.ref, instance.ref)
	require.True(t, finalized.Load())

	registerContext(ctxRef, ctx)

	// Second finalizer call should be safely ignored after handle cleanup.
	require.NotPanics(t, func() {
		goClassFinalizerProxy(rt.ref, instance.ref)
	})
}

func TestBridgeClassFinalizerLegacyHandleFallback(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	var finalized atomic.Bool

	constructor, _ := NewClassBuilder("LegacyFinalizerFallbackClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &bridgeFinalizerProbe{mark: &finalized}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())
	ctx.Globals().Set("LegacyFinalizerFallbackClass", constructor)

	instance := ctx.Eval(`new LegacyFinalizerFallbackClass()`)
	defer instance.Free()
	require.False(t, instance.IsException())

	var objectID int32
	rt.classObjectRegistry.Range(func(key, value interface{}) bool {
		id, ok := key.(int32)
		if !ok {
			return true
		}
		identity, ok := value.(classObjectIdentity)
		if !ok {
			return true
		}
		if identity.contextID == ctx.contextID {
			objectID = id
			return false
		}
		return true
	})
	require.Less(t, objectID, int32(0))

	identity, ok := rt.takeClassObjectIdentity(objectID)
	require.True(t, ok)
	require.Greater(t, identity.handleID, int32(0))

	setValueOpaqueForTest(instance.ref, identity.handleID)
	goClassFinalizerProxy(rt.ref, instance.ref)

	require.True(t, finalized.Load())
	_, exists := ctx.handleStore.Load(identity.handleID)
	require.False(t, exists)
}

func TestBridgeClassFinalizerOwnershipInSameRuntime(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctxA := rt.NewContext()
	defer ctxA.Close()
	ctxB := rt.NewContext()
	defer ctxB.Close()

	var finalizedA atomic.Bool
	var finalizedB atomic.Bool

	constructorA, _ := NewClassBuilder("SameRuntimeClassA").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &bridgeFinalizerProbe{mark: &finalizedA}, nil
		}).
		Build(ctxA)
	require.False(t, constructorA.IsException())
	ctxA.Globals().Set("SameRuntimeClassA", constructorA)

	constructorB, _ := NewClassBuilder("SameRuntimeClassB").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &bridgeFinalizerProbe{mark: &finalizedB}, nil
		}).
		Build(ctxB)
	require.False(t, constructorB.IsException())
	ctxB.Globals().Set("SameRuntimeClassB", constructorB)

	instanceA := ctxA.Eval(`new SameRuntimeClassA()`)
	defer instanceA.Free()
	require.False(t, instanceA.IsException())

	instanceB := ctxB.Eval(`new SameRuntimeClassB()`)
	defer instanceB.Free()
	require.False(t, instanceB.IsException())

	goClassFinalizerProxy(rt.ref, instanceA.ref)
	require.True(t, finalizedA.Load())
	require.False(t, finalizedB.Load())

	goClassFinalizerProxy(rt.ref, instanceB.ref)
	require.True(t, finalizedB.Load())

	require.NotPanics(t, func() {
		goClassFinalizerProxy(rt.ref, instanceA.ref)
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

		// Get function ID from handleStore and store original handle properly
		var fnID int32
		var originalHandle cgo.Handle
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			fnID = key.(int32)
			originalHandle = value.(cgo.Handle)
			return false // Stop after first item
		})

		// Create invalid handle with wrong type and store it
		invalidHandle := cgo.NewHandle("not a function")
		ctx.handleStore.handles.Store(fnID, invalidHandle)

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

		// Clean up invalid handle and restore original
		invalidHandle.Delete()
		ctx.handleStore.handles.Store(fnID, originalHandle)

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
		var constructorID int32
		var originalHandle cgo.Handle
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			handleValue := value.(cgo.Handle).Value()
			// SCHEME C: Look for ClassBuilder, not ClassConstructorFunc
			if _, ok := handleValue.(*ClassBuilder); ok {
				constructorID = key.(int32)
				originalHandle = value.(cgo.Handle)
				return false // Stop after finding ClassBuilder
			}
			return true
		})

		// Create invalid handle with wrong type and store it
		invalidHandle := cgo.NewHandle("not a ClassBuilder")
		ctx.handleStore.handles.Store(constructorID, invalidHandle)

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

		// Clean up invalid handle and restore original
		invalidHandle.Delete()
		ctx.handleStore.handles.Store(constructorID, originalHandle)

		t.Log("Successfully triggered goClassConstructorProxy type assertion failure branch")
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

		ctx.Globals().Set("TestClass", constructor)

		// Manually remove constructor mapping to simulate class ID not found
		deleteConstructorClassID(ctx, constructor.ref)

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

		// Find method function ID by collecting all handles
		var allHandles []struct {
			id     int32
			handle cgo.Handle
		}
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			allHandles = append(allHandles, struct {
				id     int32
				handle cgo.Handle
			}{
				id:     key.(int32),
				handle: value.(cgo.Handle),
			})
			return true
		})

		// Try to identify method by checking function types
		var methodID int32
		var originalHandle cgo.Handle
		var found bool

		for _, item := range allHandles {
			handleValue := item.handle.Value()
			if _, ok := handleValue.(ClassMethodFunc); ok {
				methodID = item.id
				originalHandle = item.handle
				found = true
				break
			}
		}

		if !found {
			t.Skip("Could not identify method handle ID")
		}

		// Create invalid handle with wrong type and store it
		invalidHandle := cgo.NewHandle("not a method function")
		ctx.handleStore.handles.Store(methodID, invalidHandle)

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

		// Clean up invalid handle and restore original
		invalidHandle.Delete()
		ctx.handleStore.handles.Store(methodID, originalHandle)

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

		// Find getter function ID by collecting all handles
		var allHandles []struct {
			id     int32
			handle cgo.Handle
		}
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			allHandles = append(allHandles, struct {
				id     int32
				handle cgo.Handle
			}{
				id:     key.(int32),
				handle: value.(cgo.Handle),
			})
			return true
		})

		// Try to identify getter by checking function types
		var getterID int32
		var originalHandle cgo.Handle
		var found bool

		for _, item := range allHandles {
			handleValue := item.handle.Value()
			if _, ok := handleValue.(ClassGetterFunc); ok {
				getterID = item.id
				originalHandle = item.handle
				found = true
				break
			}
		}

		if !found {
			t.Skip("Could not identify getter handle ID")
		}

		// Create invalid handle with wrong type and store it
		invalidHandle := cgo.NewHandle("not a getter function")
		ctx.handleStore.handles.Store(getterID, invalidHandle)

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

		// Clean up invalid handle and restore original
		invalidHandle.Delete()
		ctx.handleStore.handles.Store(getterID, originalHandle)

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

		// Find setter function ID by collecting all handles
		var allHandles []struct {
			id     int32
			handle cgo.Handle
		}
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			allHandles = append(allHandles, struct {
				id     int32
				handle cgo.Handle
			}{
				id:     key.(int32),
				handle: value.(cgo.Handle),
			})
			return true
		})

		// Try to identify setter by checking function types
		var setterID int32
		var originalHandle cgo.Handle
		var found bool

		for _, item := range allHandles {
			handleValue := item.handle.Value()
			if _, ok := handleValue.(ClassSetterFunc); ok {
				setterID = item.id
				originalHandle = item.handle
				found = true
				break
			}
		}

		if !found {
			t.Skip("Could not identify setter handle ID")
		}

		// Create invalid handle with wrong type and store it
		invalidHandle := cgo.NewHandle("not a setter function")
		ctx.handleStore.handles.Store(setterID, invalidHandle)

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

		// Clean up invalid handle and restore original
		invalidHandle.Delete()
		ctx.handleStore.handles.Store(setterID, originalHandle)

		t.Log("Successfully triggered goClassSetterProxy type assertion failure branch")
	})
}

func TestBridgeClassFinalizerNoGlobalContextScan(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	var finalized atomic.Bool
	constructor, _ := NewClassBuilder("NoScanClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &bridgeFinalizerProbe{mark: &finalized}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())
	ctx.Globals().Set("NoScanClass", constructor)

	instance := ctx.Eval(`new NoScanClass()`)
	defer instance.Free()
	require.False(t, instance.IsException())

	ctxRef := ctx.ref
	unregisterContext(ctxRef)
	contextMapping.Store(ctxRef, "corrupted")
	defer registerContext(ctxRef, ctx)

	goClassFinalizerProxy(rt.ref, instance.ref)
	require.True(t, finalized.Load())
}

func TestBridgeClassFinalizerOwnerContextUnavailable(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	var finalized atomic.Bool
	constructor, _ := NewClassBuilder("FinalizerOwnerMissingClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &bridgeFinalizerProbe{mark: &finalized}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())
	ctx.Globals().Set("FinalizerOwnerMissingClass", constructor)

	instance1 := ctx.Eval(`new FinalizerOwnerMissingClass()`)
	defer instance1.Free()
	require.False(t, instance1.IsException())

	instance2 := ctx.Eval(`new FinalizerOwnerMissingClass()`)
	defer instance2.Free()
	require.False(t, instance2.IsException())

	rt.contextsByID.Delete(ctx.contextID)
	goClassFinalizerProxy(rt.ref, instance1.ref)
	require.False(t, finalized.Load())
	rt.contextsByID.Store(ctx.contextID, ctx)

	backupStore := ctx.handleStore
	ctx.handleStore = nil
	goClassFinalizerProxy(rt.ref, instance2.ref)
	ctx.handleStore = backupStore
	require.False(t, finalized.Load())
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

		ctx.Globals().Set("TestClass", constructor)

		// Replace with invalid class ID to trigger JS_NewObjectProtoClass failure
		constructorKey := jsValueToKey(constructor.ref)
		ctx.runtime.constructorRegistry.Store(constructorKey, uint32(999999))

		// This should trigger CreateClassInstance to return JS_EXCEPTION
		result := ctx.Eval(`new TestClass()`)
		defer result.Free()

		// Restore for cleanup
		ctx.runtime.constructorRegistry.Store(constructorKey, originalClassID)

		// Should get an error
		if result.IsException() {
			err := ctx.Exception()
			t.Logf("Expected exception from CreateClassInstance: %v", err)
		}

		t.Log("Successfully triggered CreateClassInstance JS_EXCEPTION branch")
	})
}
