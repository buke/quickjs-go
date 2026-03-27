package quickjs

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRuntimeBasics tests basic runtime creation and operations
func TestRuntimeBasics(t *testing.T) {
	// Test default runtime
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	result := ctx.Eval(`1 + 1`)
	defer result.Free()
	require.False(t, result.IsException()) // Check for exceptions instead of error
	require.EqualValues(t, 2, result.ToInt32())

	// Test runtime with all options in one go
	rt2 := NewRuntime(
		WithExecuteTimeout(30),
		WithMemoryLimit(512*1024),
		WithGCThreshold(256*1024),
		WithMaxStackSize(65534),
		WithCanBlock(true),
		WithModuleImport(true),
		WithStripInfo(1),
	)
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result2 := ctx2.Eval(`"Hello World"`)
	defer result2.Free()
	require.False(t, result2.IsException()) // Check for exceptions instead of error
	require.Equal(t, "Hello World", result2.ToString())
}

// TestRuntimeLimitsAndErrors tests memory limits, timeouts, and stack limits
func TestRuntimeLimitsAndErrors(t *testing.T) {
	t.Run("ExecuteTimeout", func(t *testing.T) {
		rt := NewRuntime(WithExecuteTimeout(1))
		defer rt.Close()

		ctx := rt.NewContext()
		defer ctx.Close()

		result := ctx.Eval(`while(true){}`)
		defer result.Free()
		require.True(t, result.IsException()) // Check for exceptions instead of error

		// Use Context.Exception() instead of result.ToError()
		err := ctx.Exception()
		require.Contains(t, err.Error(), "interrupted")
	})

	t.Run("MemoryLimit", func(t *testing.T) {
		rt := NewRuntime(WithMemoryLimit(512 * 1024))
		defer rt.Close()

		ctx := rt.NewContext()
		defer ctx.Close()

		result := ctx.Eval(`var array = []; while (true) { array.push(null) }`)
		defer result.Free()
		require.True(t, result.IsException()) // Check for exceptions instead of error

		// Use Context.Exception() instead of result.ToError()
		err := ctx.Exception()
		require.Contains(t, err.Error(), "out of memory")
	})

	t.Run("StackOverflow", func(t *testing.T) {
		rt := NewRuntime(WithMaxStackSize(8192))
		defer rt.Close()

		ctx := rt.NewContext()
		defer ctx.Close()

		result := ctx.Eval(`
            function recursive(n) {
                if (n <= 0) return 0;
                return recursive(n - 1) + 1;
            }
            recursive(10000);
        `)
		defer result.Free()
		require.True(t, result.IsException()) // Check for exceptions instead of error

		// Use Context.Exception() instead of result.ToError()
		err := ctx.Exception()
		require.True(t,
			strings.Contains(err.Error(), "stack overflow") || strings.Contains(err.Error(), "Maximum call stack size exceeded"),
			err.Error(),
		)
	})
}

// TestRuntimeConfiguration tests runtime configuration setters
func TestRuntimeConfiguration(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	// Test all setters for coverage
	rt.SetMemoryLimit(1024 * 1024)
	rt.SetExecuteTimeout(5)
	rt.SetMaxStackSize(16384)
	rt.SetGCThreshold(2048)
	rt.SetCanBlock(true)
	rt.SetCanBlock(false) // Test both branches
	rt.SetStripInfo(1)

	// Run garbage collection
	rt.RunGC()

	ctx := rt.NewContext()
	defer ctx.Close()

	result := ctx.Eval(`"configuration test"`)
	defer result.Free()
	require.False(t, result.IsException()) // Check for exceptions instead of error
	require.Equal(t, "configuration test", result.ToString())
}

// TestRuntimeInterruptHandler tests interrupt handler functionality and coverage
func TestRuntimeInterruptHandler(t *testing.T) {
	newCtx := func(t *testing.T) (*Runtime, *Context) {
		rt := NewRuntime()
		ctx := rt.NewContext()
		t.Cleanup(func() {
			ctx.Close()
			rt.Close()
		})
		return rt, ctx
	}

	t.Run("InterruptAfterDelay", func(t *testing.T) {
		rt, ctx := newCtx(t)
		startTime := time.Now()
		rt.SetInterruptHandler(func() int {
			if time.Since(startTime) > time.Second {
				return 1 // Interrupt after 1 second
			}
			return 0 // Continue
		})

		result := ctx.Eval(`while(true){}`)
		defer result.Free()
		require.True(t, result.IsException()) // Check for exceptions instead of error

		// Use Context.Exception() instead of result.ToError()
		err := ctx.Exception()
		require.Contains(t, err.Error(), "interrupted")
	})

	t.Run("ClearBySettingNil", func(t *testing.T) {
		rt, ctx := newCtx(t)
		// Set then clear by nil (covers else branch in SetInterruptHandler)
		rt.SetInterruptHandler(func() int { return 1 })
		rt.SetInterruptHandler(nil)

		result := ctx.Eval(`let sum = 0; for(let i = 0; i < 100000; i++) sum += i; sum`)
		defer result.Free()
		require.False(t, result.IsException())
	})

	t.Run("ClearExplicitly", func(t *testing.T) {
		rt, ctx := newCtx(t)
		rt.SetInterruptHandler(func() int { return 1 })
		rt.ClearInterruptHandler()

		result := ctx.Eval(`let result = 42; result`)
		defer result.Free()
		require.False(t, result.IsException())
	})
}

// TestCallInterruptHandler_DirectCall directly tests callInterruptHandler method for 100% coverage
func TestCallInterruptHandler_DirectCall(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	// Test return 0 branch when no handler is set
	rt.ClearInterruptHandler()
	require.Equal(t, 0, rt.callInterruptHandler())

	// Test handler invocation with different return values
	testCases := []int{0, 1, 42, -1}
	for _, expected := range testCases {
		rt.SetInterruptHandler(func() int { return expected })
		require.Equal(t, expected, rt.callInterruptHandler())
	}
}

// TestRuntimeTimeoutVsInterruptHandler tests precedence between timeout and interrupt handler
func TestRuntimeTimeoutVsInterruptHandler(t *testing.T) {
	t.Run("TimeoutOverridesHandler", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		ctx := rt.NewContext()
		defer ctx.Close()

		// Set handler first, then timeout (timeout should override)
		rt.SetInterruptHandler(func() int { return 0 })
		rt.SetExecuteTimeout(1)

		start := time.Now()
		result := ctx.Eval(`while(true){}`)
		defer result.Free()
		elapsed := time.Since(start)

		require.True(t, result.IsException()) // Check for exceptions instead of error

		// Use Context.Exception() instead of result.ToError()
		err := ctx.Exception()
		require.Contains(t, err.Error(), "interrupted")
		require.Less(t, elapsed, 3*time.Second)
	})

	t.Run("HandlerOverridesTimeout", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		ctx := rt.NewContext()
		defer ctx.Close()

		// Set timeout first, then handler (handler should override)
		rt.SetExecuteTimeout(10)
		rt.SetInterruptHandler(func() int { return 1 })

		start := time.Now()
		result := ctx.Eval(`while(true){}`)
		defer result.Free()
		elapsed := time.Since(start)

		require.True(t, result.IsException()) // Check for exceptions instead of error

		// Use Context.Exception() instead of result.ToError()
		err := ctx.Exception()
		require.Contains(t, err.Error(), "interrupted")
		require.Less(t, elapsed, 3*time.Second)
	})
}

func TestRuntimeTimeoutHandlerAllocationLifecycle(t *testing.T) {
	require.Equal(t, 0, currentTimeoutAllocationCount())
	require.Equal(t, 0, currentTimeoutRegistryEntryCount())

	t.Run("ClearInterruptHandlerFreesTimeoutPayload", func(t *testing.T) {
		rt := NewRuntime()

		rt.SetExecuteTimeout(10)
		require.Equal(t, 1, currentTimeoutAllocationCount())
		require.Equal(t, 1, currentTimeoutRegistryEntryCount())

		rt.ClearInterruptHandler()
		require.Equal(t, 0, currentTimeoutAllocationCount())
		require.Equal(t, 0, currentTimeoutRegistryEntryCount())

		rt.Close()
		require.Equal(t, 0, currentTimeoutAllocationCount())
		require.Equal(t, 0, currentTimeoutRegistryEntryCount())
	})

	t.Run("UserHandlerOverrideFreesTimeoutPayload", func(t *testing.T) {
		rt := NewRuntime()

		rt.SetExecuteTimeout(10)
		require.Equal(t, 1, currentTimeoutAllocationCount())
		require.Equal(t, 1, currentTimeoutRegistryEntryCount())

		rt.SetInterruptHandler(func() int { return 0 })
		require.Equal(t, 0, currentTimeoutAllocationCount())
		require.Equal(t, 0, currentTimeoutRegistryEntryCount())

		rt.Close()
		require.Equal(t, 0, currentTimeoutAllocationCount())
		require.Equal(t, 0, currentTimeoutRegistryEntryCount())
	})

	t.Run("ReplacingTimeoutFreesPreviousPayload", func(t *testing.T) {
		rt := NewRuntime()

		rt.SetExecuteTimeout(10)
		require.Equal(t, 1, currentTimeoutAllocationCount())
		require.Equal(t, 1, currentTimeoutRegistryEntryCount())

		rt.SetExecuteTimeout(20)
		require.Equal(t, 1, currentTimeoutAllocationCount())
		require.Equal(t, 1, currentTimeoutRegistryEntryCount())

		rt.Close()
		require.Equal(t, 0, currentTimeoutAllocationCount())
		require.Equal(t, 0, currentTimeoutRegistryEntryCount())
	})
}

func TestRuntimeCloseThreadOwnershipContract(t *testing.T) {
	rt := NewRuntime()

	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	panicCh := make(chan interface{}, 1)
	go func() {
		defer func() {
			panicCh <- recover()
		}()
		rt.Close()
	}()

	recovered := <-panicCh
	require.NotNil(t, recovered)
	require.Contains(t, fmt.Sprint(recovered), "Runtime.Close must be called on the runtime owner thread")

	result := ctx.Eval(`1 + 2`)
	defer result.Free()
	require.False(t, result.IsException())
	require.EqualValues(t, 3, result.ToInt32())

	ctx.Close()
	require.NotPanics(t, func() {
		rt.Close()
	})
}

func TestRuntimeOwnerThreadContractsParallel(t *testing.T) {
	t.Run("CrossGoroutineFailFast", func(t *testing.T) {
		t.Parallel()

		rt := NewRuntime()
		ctx := rt.NewContext()

		assertPanicFromForeignGoroutine := func(op string, fn func()) {
			t.Helper()
			panicCh := make(chan interface{}, 1)
			go func() {
				defer func() {
					panicCh <- recover()
				}()
				fn()
			}()
			recovered := <-panicCh
			require.NotNil(t, recovered)
			require.Contains(t, fmt.Sprint(recovered), op)
			require.Contains(t, fmt.Sprint(recovered), "runtime owner thread")
		}

		assertPanicFromForeignGoroutine("Context.Eval", func() {
			result := ctx.Eval(`1 + 1`)
			if result != nil {
				result.Free()
			}
		})
		assertPanicFromForeignGoroutine("Context.Loop", func() { ctx.Loop() })
		assertPanicFromForeignGoroutine("Context.Await", func() { _ = ctx.Await(nil) })
		assertPanicFromForeignGoroutine("Runtime.RunGC", func() { rt.RunGC() })
		assertPanicFromForeignGoroutine("Runtime.SetMemoryLimit", func() { rt.SetMemoryLimit(1024) })
		assertPanicFromForeignGoroutine("Runtime.NewContext", func() {
			child := rt.NewContext()
			if child != nil {
				child.Close()
			}
		})

		result := ctx.Eval(`40 + 2`)
		defer result.Free()
		require.False(t, result.IsException())
		require.EqualValues(t, 42, result.ToInt32())

		ctx.Close()
		rt.Close()
	})

	t.Run("MultiRuntimeCloseIsolation", func(t *testing.T) {
		t.Parallel()

		rtA := NewRuntime()
		ctxA := rtA.NewContext()
		ctorA, classIDA := NewClassBuilder("IsolationClassA").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 1, Y: 1}, nil
			}).
			Build(ctxA)
		require.False(t, ctorA.IsException())
		ctorARef := ctorA.ref
		keyA := runtimeKeyFromCRef(rtA.ref)

		rtB := NewRuntime()
		ctxB := rtB.NewContext()
		ctorB, classIDB := NewClassBuilder("IsolationClassB").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &Point{X: 2, Y: 2}, nil
			}).
			Build(ctxB)
		require.False(t, ctorB.IsException())
		ctorBRef := ctorB.ref
		ctorBKey := jsValueToKey(ctorBRef)
		keyB := runtimeKeyFromCRef(rtB.ref)

		gotA, okA := getConstructorClassID(ctxA, ctorARef)
		require.True(t, okA)
		require.Equal(t, classIDA, gotA)

		gotB, okB := getConstructorClassID(ctxB, ctorBRef)
		require.True(t, okB)
		require.Equal(t, classIDB, gotB)

		ctorA.Free()

		ctxA.Close()
		rtA.Close()

		_, existsA := constructorRegistryByRuntime.Load(keyA)
		require.False(t, existsA)

		bucketBAny, existsB := constructorRegistryByRuntime.Load(keyB)
		require.True(t, existsB)
		bucketB, ok := bucketBAny.(*sync.Map)
		require.True(t, ok)
		storedClassIDBAny, found := bucketB.Load(ctorBKey)
		require.True(t, found)
		require.Equal(t, classIDB, storedClassIDBAny.(uint32))

		gotBAfter, okBAfter := getConstructorClassID(ctxB, ctorBRef)
		require.True(t, okBAfter)
		require.Equal(t, classIDB, gotBAfter)

		ctorB.Free()

		ctxB.Close()
		rtB.Close()
	})
}

func TestRuntimeTimeoutRegistryConcurrentAccess(t *testing.T) {
	const workers = 16

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rt := NewRuntime()
			rt.SetExecuteTimeout(uint64(1 + (i % 3)))
			rt.SetInterruptHandler(func() int { return 0 })
			rt.SetExecuteTimeout(uint64(2 + (i % 2)))
			rt.ClearInterruptHandler()
			rt.Close()
		}(i)
	}
	wg.Wait()

	require.Equal(t, 0, currentTimeoutAllocationCount())
	require.Equal(t, 0, currentTimeoutRegistryEntryCount())
}

func TestRuntimeNilSafety(t *testing.T) {
	t.Run("NilReceiver", func(t *testing.T) {
		var nilRt *Runtime
		require.NotPanics(t, func() { nilRt.RunGC() })
		require.NotPanics(t, func() { nilRt.SetCanBlock(true) })
		require.NotPanics(t, func() { nilRt.SetMemoryLimit(1) })
		require.NotPanics(t, func() { nilRt.SetGCThreshold(1) })
		require.NotPanics(t, func() { nilRt.SetMaxStackSize(1) })
		require.NotPanics(t, func() { nilRt.SetExecuteTimeout(1) })
		require.NotPanics(t, func() { nilRt.SetStripInfo(1) })
		require.NotPanics(t, func() { nilRt.SetModuleImport(true) })
		require.NotPanics(t, func() { nilRt.SetInterruptHandler(func() int { return 0 }) })
		require.NotPanics(t, func() { nilRt.ClearInterruptHandler() })
		require.Equal(t, 0, nilRt.callInterruptHandler())
		require.Nil(t, nilRt.NewContext())
		require.NotPanics(t, func() { nilRt.Close() })
	})

	t.Run("OrphanRuntime", func(t *testing.T) {
		rt := &Runtime{}
		require.NotPanics(t, func() { rt.RunGC() })
		require.NotPanics(t, func() { rt.SetCanBlock(true) })
		require.NotPanics(t, func() { rt.SetMemoryLimit(1) })
		require.NotPanics(t, func() { rt.SetGCThreshold(1) })
		require.NotPanics(t, func() { rt.SetMaxStackSize(1) })
		require.NotPanics(t, func() { rt.SetExecuteTimeout(1) })
		require.NotPanics(t, func() { rt.SetStripInfo(1) })
		require.NotPanics(t, func() { rt.SetModuleImport(true) })
		require.NotPanics(t, func() { rt.SetInterruptHandler(func() int { return 0 }) })
		require.NotPanics(t, func() { rt.ClearInterruptHandler() })
		require.Equal(t, 0, rt.callInterruptHandler())
		require.Nil(t, rt.NewContext())
		require.NotPanics(t, func() { rt.Close() })
	})

	t.Run("CloseIdempotent", func(t *testing.T) {
		rt := NewRuntime()
		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		ctx.Close()
		require.NotPanics(t, func() { rt.Close() })
		require.NotPanics(t, func() { rt.Close() })
		require.Nil(t, rt.NewContext())
	})
}

// TestRuntimeMultipleContexts tests creating and using multiple contexts
func TestRuntimeMultipleContexts(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx1 := rt.NewContext()
	defer ctx1.Close()

	ctx2 := rt.NewContext()
	defer ctx2.Close()

	// Test context isolation
	result1 := ctx1.Eval(`var x = "ctx1"; x`)
	defer result1.Free()
	require.False(t, result1.IsException()) // Check for exceptions instead of error
	require.Equal(t, "ctx1", result1.ToString())

	result2 := ctx2.Eval(`var x = "ctx2"; x`)
	defer result2.Free()
	require.False(t, result2.IsException()) // Check for exceptions instead of error
	require.Equal(t, "ctx2", result2.ToString())

	// Verify isolation
	result3 := ctx1.Eval(`x`)
	defer result3.Free()
	require.False(t, result3.IsException()) // Check for exceptions instead of error
	require.Equal(t, "ctx1", result3.ToString())
}

// TestRuntimeConcurrency tests concurrent usage of runtime instances
func TestRuntimeConcurrency(t *testing.T) {
	const numGoroutines = 4
	const opsPerGoroutine = 20

	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines*opsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			rt := NewRuntime()
			defer rt.Close()

			ctx := rt.NewContext()
			defer ctx.Close()

			for j := 0; j < opsPerGoroutine; j++ {
				result := ctx.Eval(`new Date().getTime()`)
				success := !result.IsException() // Check for exceptions instead of error
				results <- success
				result.Free()
			}
		}()
	}

	wg.Wait()
	close(results)

	// Verify all operations succeeded
	successCount := 0
	for success := range results {
		if success {
			successCount++
		}
	}
	require.Equal(t, numGoroutines*opsPerGoroutine, successCount)
}

// TestRuntimeAdvancedOptions tests advanced runtime options for coverage
func TestRuntimeAdvancedOptions(t *testing.T) {
	// Test WithCanBlock(false)
	rt1 := NewRuntime(WithCanBlock(false))
	defer rt1.Close()

	ctx1 := rt1.NewContext()
	defer ctx1.Close()

	result1 := ctx1.Eval(`"canBlock disabled"`)
	defer result1.Free()
	require.False(t, result1.IsException()) // Check for exceptions instead of error
	require.Equal(t, "canBlock disabled", result1.ToString())

	// Test WithModuleImport(true)
	rt2 := NewRuntime(WithModuleImport(true))
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result2 := ctx2.Eval(`"module import enabled"`)
	defer result2.Free()
	require.False(t, result2.IsException()) // Check for exceptions instead of error
	require.Equal(t, "module import enabled", result2.ToString())

	// Test WithStripInfo(0)
	rt3 := NewRuntime(WithStripInfo(0))
	defer rt3.Close()

	ctx3 := rt3.NewContext()
	defer ctx3.Close()

	result3 := ctx3.Eval(`"strip info test"`)
	defer result3.Free()
	require.False(t, result3.IsException()) // Check for exceptions instead of error
	require.Equal(t, "strip info test", result3.ToString())

	// Test GC options
	rt4 := NewRuntime(WithGCThreshold(1024))
	defer rt4.Close()

	rt5 := NewRuntime(WithGCThreshold(-1)) // Disabled
	defer rt5.Close()

	ctx4 := rt4.NewContext()
	defer ctx4.Close()

	result4 := ctx4.Eval(`"GC test"`)
	defer result4.Free()
	require.False(t, result4.IsException()) // Check for exceptions instead of error
	require.Equal(t, "GC test", result4.ToString())
}

func TestRuntimeCloseWithConcurrentSchedule(t *testing.T) {
	const iterations = 40

	for i := 0; i < iterations; i++ {
		rt := NewRuntime()
		ctx := rt.NewContext()
		require.NotNil(t, ctx)

		var executed atomic.Int32
		stop := make(chan struct{})
		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				select {
				case <-stop:
					return
				default:
				}

				_ = ctx.Schedule(func(*Context) {
					executed.Add(1)
				})
				time.Sleep(100 * time.Microsecond)
			}
		}()

		time.Sleep(2 * time.Millisecond)
		require.NotPanics(t, func() { ctx.Close() })
		require.False(t, ctx.Schedule(func(*Context) {}))

		close(stop)
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("scheduler goroutine did not stop after context close")
		}

		require.NotPanics(t, func() { rt.Close() })
		require.GreaterOrEqual(t, executed.Load(), int32(0))
	}
}

func TestRuntimePromiseCancelScheduleCloseInterleaving(t *testing.T) {
	const iterations = 30

	for i := 0; i < iterations; i++ {
		rt := NewRuntime()
		ctx := rt.NewContext()
		require.NotNil(t, ctx)

		done := make(chan struct{})
		promise, cancel := ctx.NewPromiseWithCancel(func(resolve, reject func(*Value)) {
			go func() {
				defer close(done)
				time.Sleep(200 * time.Microsecond)
				_ = ctx.Schedule(func(inner *Context) {
					val := inner.NewString("late-resolve")
					if val != nil {
						defer val.Free()
					}
					resolve(val)
				})
			}()
		})

		require.NotNil(t, promise)
		require.NotPanics(t, func() { promise.Free() })

		time.Sleep(100 * time.Microsecond)
		require.NotPanics(t, func() { cancel() })
		require.NotPanics(t, func() { ctx.Close() })
		require.False(t, ctx.Schedule(func(*Context) {}))
		require.NotPanics(t, func() { rt.Close() })

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("promise scheduling goroutine did not finish")
		}
	}
}

func TestRuntimeInterruptHandlerToggleWithRunGCInterleaving(t *testing.T) {
	rt := NewRuntime()
	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	// QuickJS context execution is single-threaded; use tight sequential interleaving
	// to stress lifecycle paths without unsafe concurrent Eval/GC execution.
	for i := 0; i < 300; i++ {
		rt.SetInterruptHandler(func() int { return 0 })
		rt.ClearInterruptHandler()
		rt.SetExecuteTimeout(1)
		rt.SetInterruptHandler(func() int { return 0 })
		rt.RunGC()

		result := ctx.Eval(`1 + 1`)
		require.NotNil(t, result)
		require.False(t, result.IsException())
		require.Equal(t, int32(2), result.ToInt32())
		result.Free()
	}

	require.NotPanics(t, func() { ctx.Close() })
	require.NotPanics(t, func() { rt.Close() })
}

func TestRuntimeAndRegistryHelperGuards(t *testing.T) {
	require.Equal(t, uintptr(0), runtimeKeyFromCRef(nil))
	require.Equal(t, uintptr(0), constructorRegistryRuntimeKey(nil))
	require.Nil(t, constructorRegistryBucketForContext(nil, false))
	require.Nil(t, constructorRegistryBucketForContext(nil, true))

	var nilRuntime *Runtime
	require.NotPanics(t, func() { requireRuntimeOwnerThread(nilRuntime, "noop") })
	require.NotPanics(t, func() { requireRuntimeOwnerThread(&Runtime{}, "noop") })

	orphanCtx := &Context{}
	require.Equal(t, uintptr(0), constructorRegistryRuntimeKey(orphanCtx))
	require.Nil(t, constructorRegistryBucketForContext(orphanCtx, false))
	require.Nil(t, constructorRegistryBucketForContext(orphanCtx, true))

	rt := NewRuntime()
	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	key := constructorRegistryRuntimeKey(ctx)
	require.NotZero(t, key)

	// No bucket yet: !create branch should return nil.
	require.Nil(t, constructorRegistryBucketForContext(ctx, false))

	createdBucket := constructorRegistryBucketForContext(ctx, true)
	require.NotNil(t, createdBucket)
	reusedBucket := constructorRegistryBucketForContext(ctx, true)
	require.NotNil(t, reusedBucket)
	require.Same(t, createdBucket, reusedBucket)

	readBucket := constructorRegistryBucketForContext(ctx, false)
	require.NotNil(t, readBucket)
	require.Same(t, createdBucket, readBucket)

	constructorRegistryByRuntime.Store(key, "bad-bucket")
	require.Nil(t, constructorRegistryBucketForContext(ctx, false))
	_, exists := constructorRegistryByRuntime.Load(key)
	require.False(t, exists)

	constructorRegistryByRuntime.Store(key, "bad-bucket")
	require.Nil(t, constructorRegistryBucketForContext(ctx, true))
	_, exists = constructorRegistryByRuntime.Load(key)
	require.False(t, exists)

	bucket := constructorRegistryBucketForContext(ctx, true)
	require.NotNil(t, bucket)

	obj := ctx.NewObject()
	require.NotNil(t, obj)

	registerConstructorClassID(nil, obj.ref, 123)
	storeConstructorRegistryEntryForTest(nil, obj.ref, uint32(1))
	_, ok := loadConstructorRegistryEntryForTest(nil, obj.ref)
	require.False(t, ok)
	require.NotPanics(t, func() { deleteConstructorRegistryEntryForTest(nil, obj.ref) })

	bucket.Store(jsValueToKey(obj.ref), uint32(42))
	gotClassID, found := getConstructorClassID(ctx, obj.ref)
	require.True(t, found)
	require.Equal(t, uint32(42), gotClassID)

	clearConstructorRegistryForRuntime(nil)
	clearConstructorRegistryForRuntime(&Runtime{})
	obj.Free()

	ctx.Close()
	clearConstructorRegistryForRuntime(rt)
	_, exists = constructorRegistryByRuntime.Load(key)
	require.False(t, exists)
	rt.Close()
}

func TestRuntimeNewContextFailStageInjection(t *testing.T) {
	t.Cleanup(func() {
		setRuntimeNewContextFailStageForTest(runtimeNewContextFailStageNone)
		setRuntimeNewContextInitCodeForTest("")
	})

	t.Run("CompileStage", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setRuntimeNewContextFailStageForTest(runtimeNewContextFailStageCompile)
		require.Nil(t, rt.NewContext())

		setRuntimeNewContextFailStageForTest(runtimeNewContextFailStageNone)
		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		ctx.Close()
	})

	t.Run("ExecStage", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setRuntimeNewContextFailStageForTest(runtimeNewContextFailStageExec)
		require.Nil(t, rt.NewContext())

		setRuntimeNewContextFailStageForTest(runtimeNewContextFailStageNone)
		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		ctx.Close()
	})

	t.Run("AwaitStage", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setRuntimeNewContextFailStageForTest(runtimeNewContextFailStageAwait)
		require.Nil(t, rt.NewContext())

		setRuntimeNewContextFailStageForTest(runtimeNewContextFailStageNone)
		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		ctx.Close()
	})
}

func TestRuntimeNewContextInitCodeExceptionPaths(t *testing.T) {
	t.Cleanup(func() {
		setRuntimeNewContextInitCodeForTest("")
	})

	t.Run("CompileException", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setRuntimeNewContextInitCodeForTest(`import {`) // invalid syntax
		require.Nil(t, rt.NewContext())

		setRuntimeNewContextInitCodeForTest("")
		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		ctx.Close()
	})

	t.Run("ExecOrAwaitExceptionFromBadImport", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setRuntimeNewContextInitCodeForTest(`
            import { definitely_missing_symbol } from "os";
            globalThis.__qjs_tmp = definitely_missing_symbol;
        `)
		require.Nil(t, rt.NewContext())

		setRuntimeNewContextInitCodeForTest("")
		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		ctx.Close()
	})

	t.Run("AwaitExceptionFromRejectedTopLevelAwait", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setRuntimeNewContextInitCodeForTest(`
            await Promise.reject(new Error("await-init-fail"));
        `)
		require.Nil(t, rt.NewContext())

		setRuntimeNewContextInitCodeForTest("")
		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		ctx.Close()
	})
}

func TestFinalizerObservabilityHelperGuards(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	require.Nil(t, getFinalizerObservabilityCounters(nil, false))
	require.Nil(t, getFinalizerObservabilityCounters(nil, true))
	require.NotPanics(t, func() { observeFinalizerCounter(nil, finalizerCounterCleaned) })

	key := runtimeKeyFromCRef(rt.ref)
	require.NotZero(t, key)

	finalizerObservabilityByRuntime.Store(key, "bad-counters")
	require.Nil(t, getFinalizerObservabilityCounters(rt.ref, false))
	_, exists := finalizerObservabilityByRuntime.Load(key)
	require.False(t, exists)

	finalizerObservabilityByRuntime.Store(key, "bad-counters")
	require.NotPanics(t, func() { setFinalizerObservabilityForTest(rt, true) })
	_, exists = finalizerObservabilityByRuntime.Load(key)
	require.False(t, exists)

	finalizerObservabilityByRuntime.Store(key, "bad-counters")
	require.NotPanics(t, func() { resetFinalizerObservabilityForTest(rt) })
	_, exists = finalizerObservabilityByRuntime.Load(key)
	require.False(t, exists)

	finalizerObservabilityByRuntime.Store(key, "bad-counters")
	require.Nil(t, getFinalizerObservabilityCounters(rt.ref, true))
	_, exists = finalizerObservabilityByRuntime.Load(key)
	require.False(t, exists)

	setFinalizerObservabilityForTest(rt, true)
	resetFinalizerObservabilityForTest(rt)
	observeFinalizerCounter(rt.ref, -1)
	snapshot := snapshotFinalizerObservabilityForTest(rt)
	require.True(t, snapshot.Enabled)
	require.Equal(t, uint64(0), snapshot.Cleaned)

	setFinalizerObservabilityForTest(rt, false)
	observeFinalizerCounter(rt.ref, finalizerCounterCleaned)
	snapshot = snapshotFinalizerObservabilityForTest(rt)
	require.False(t, snapshot.Enabled)
	require.Equal(t, uint64(0), snapshot.Cleaned)

	clearFinalizerObservabilityForRuntime(nil)
	clearFinalizerObservabilityForRuntime(rt.ref)
	_, exists = finalizerObservabilityByRuntime.Load(key)
	require.False(t, exists)
}

func TestRuntimeNewRuntimeFailInjection(t *testing.T) {
	setJSNewRuntimeFailForTest(true)
	t.Cleanup(func() {
		setJSNewRuntimeFailForTest(false)
	})

	rt := NewRuntime()
	require.NotNil(t, rt)
	require.Nil(t, rt.ref)

	require.NotPanics(t, func() { rt.Close() })

	setJSNewRuntimeFailForTest(false)
	rt2 := NewRuntime()
	require.NotNil(t, rt2)
	require.NotNil(t, rt2.ref)
	rt2.Close()
}

func TestFinalizerObservabilityCounterAllBranches(t *testing.T) {
	var nilRuntime *Runtime
	require.NotPanics(t, func() { setFinalizerObservabilityForTest(nilRuntime, true) })
	require.NotPanics(t, func() { resetFinalizerObservabilityForTest(nilRuntime) })
	require.Equal(t, finalizerObservabilitySnapshot{}, snapshotFinalizerObservabilityForTest(nilRuntime))
	require.Equal(t, finalizerObservabilitySnapshot{}, snapshotAndResetFinalizerObservabilityForTest(nilRuntime))

	orphan := &Runtime{}
	require.NotPanics(t, func() { setFinalizerObservabilityForTest(orphan, true) })
	require.NotPanics(t, func() { resetFinalizerObservabilityForTest(orphan) })
	require.Equal(t, finalizerObservabilitySnapshot{}, snapshotFinalizerObservabilityForTest(orphan))
	require.Equal(t, finalizerObservabilitySnapshot{}, snapshotAndResetFinalizerObservabilityForTest(orphan))

	rt := NewRuntime()
	defer rt.Close()

	setFinalizerObservabilityForTest(rt, true)
	resetFinalizerObservabilityForTest(rt)

	observeFinalizerCounter(rt.ref, finalizerCounterOpaqueNil)
	observeFinalizerCounter(rt.ref, finalizerCounterOpaqueInvalid)
	observeFinalizerCounter(rt.ref, finalizerCounterHandleInvalid)
	observeFinalizerCounter(rt.ref, finalizerCounterContextRefInvalid)
	observeFinalizerCounter(rt.ref, finalizerCounterContextNotFound)
	observeFinalizerCounter(rt.ref, finalizerCounterContextStateInvalid)
	observeFinalizerCounter(rt.ref, finalizerCounterRuntimeMismatch)
	observeFinalizerCounter(rt.ref, finalizerCounterHandleMissing)
	observeFinalizerCounter(rt.ref, finalizerCounterCleaned)

	snapshot := snapshotFinalizerObservabilityForTest(rt)
	require.True(t, snapshot.Enabled)
	require.Equal(t, uint64(1), snapshot.OpaqueNil)
	require.Equal(t, uint64(1), snapshot.OpaqueInvalid)
	require.Equal(t, uint64(1), snapshot.HandleInvalid)
	require.Equal(t, uint64(1), snapshot.ContextRefInvalid)
	require.Equal(t, uint64(1), snapshot.ContextNotFound)
	require.Equal(t, uint64(1), snapshot.ContextStateInvalid)
	require.Equal(t, uint64(1), snapshot.RuntimeMismatch)
	require.Equal(t, uint64(1), snapshot.HandleMissing)
	require.Equal(t, uint64(1), snapshot.Cleaned)

	reset := snapshotAndResetFinalizerObservabilityForTest(rt)
	require.True(t, reset.Enabled)
	require.Equal(t, uint64(1), reset.OpaqueNil)
	require.Equal(t, uint64(1), reset.Cleaned)

	after := snapshotFinalizerObservabilityForTest(rt)
	require.True(t, after.Enabled)
	require.Equal(t, uint64(0), after.OpaqueNil)
	require.Equal(t, uint64(0), after.Cleaned)
}
