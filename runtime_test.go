package quickjs

import (
	"sync"
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
		WithMemoryLimit(128*1024),
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
		require.Contains(t, err.Error(), "stack overflow")
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
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("InterruptAfterDelay", func(t *testing.T) {
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
		// Set then clear by nil (covers else branch in SetInterruptHandler)
		rt.SetInterruptHandler(func() int { return 1 })
		rt.SetInterruptHandler(nil)

		done := make(chan bool, 1)
		go func() {
			result := ctx.Eval(`let sum = 0; for(let i = 0; i < 100000; i++) sum += i; sum`)
			defer result.Free()
			done <- !result.IsException() // Check for exceptions instead of error
		}()

		select {
		case success := <-done:
			require.True(t, success)
		case <-time.After(3 * time.Second):
			t.Fatal("Code took too long")
		}
	})

	t.Run("ClearExplicitly", func(t *testing.T) {
		rt.SetInterruptHandler(func() int { return 1 })
		rt.ClearInterruptHandler()

		done := make(chan bool, 1)
		go func() {
			result := ctx.Eval(`let result = 42; result`)
			defer result.Free()
			done <- !result.IsException() // Check for exceptions instead of error
		}()

		select {
		case success := <-done:
			require.True(t, success)
		case <-time.After(2 * time.Second):
			t.Fatal("Code took too long")
		}
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

func TestRuntimeTimeoutOpaqueLifecycle(t *testing.T) {
	base := timeoutOpaqueCount()

	rt := NewRuntime()
	defer rt.Close()

	require.Equal(t, base, timeoutOpaqueCount())

	rt.SetExecuteTimeout(5)
	require.Equal(t, base+1, timeoutOpaqueCount())

	// Replacing timeout should not accumulate opaque states.
	rt.SetExecuteTimeout(10)
	require.Equal(t, base+1, timeoutOpaqueCount())

	rt.SetInterruptHandler(func() int { return 0 })
	require.Equal(t, base, timeoutOpaqueCount())

	rt.SetExecuteTimeout(5)
	require.Equal(t, base+1, timeoutOpaqueCount())

	rt.ClearInterruptHandler()
	require.Equal(t, base, timeoutOpaqueCount())

	rt.SetExecuteTimeout(5)
	require.Equal(t, base+1, timeoutOpaqueCount())

	rt.SetExecuteTimeout(0)
	require.Equal(t, base, timeoutOpaqueCount())
}

func TestRuntimeTimeoutOpaqueNotFreedInHandler(t *testing.T) {
	base := timeoutOpaqueCount()

	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	rt.SetExecuteTimeout(1)
	require.Equal(t, base+1, timeoutOpaqueCount())

	result := ctx.Eval(`while(true){}`)
	defer result.Free()
	require.True(t, result.IsException())

	err := ctx.Exception()
	require.Error(t, err)
	require.Contains(t, err.Error(), "interrupted")

	// timeoutHandler should not free opaque state; cleanup happens on clear/replace.
	require.Equal(t, base+1, timeoutOpaqueCount())

	rt.ClearInterruptHandler()
	require.Equal(t, base, timeoutOpaqueCount())
}

func TestRuntimeTimeoutOpaqueConcurrentLifecycle(t *testing.T) {
	base := timeoutOpaqueCount()

	const workers = 4
	const loops = 50

	var wg sync.WaitGroup
	errCh := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			rt := NewRuntime()
			ctx := rt.NewContext()
			if ctx == nil {
				errCh <- "NewContext returned nil"
				rt.Close()
				return
			}

			for j := 0; j < loops; j++ {
				rt.SetExecuteTimeout(1)
				rt.SetExecuteTimeout(2)
				rt.ClearInterruptHandler()
			}

			ctx.Close()
			rt.Close()
		}()
	}
	wg.Wait()
	close(errCh)
	for errMsg := range errCh {
		t.Error(errMsg)
	}

	require.Equal(t, base, timeoutOpaqueCount())
}

func TestRuntimeCrossGoroutineLifecycleWithoutInternalThreadBinding(t *testing.T) {
	created := make(chan *Runtime, 1)

	go func() {
		rt := NewRuntime()
		created <- rt
	}()

	rt := <-created
	require.NotNil(t, rt)

	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	result := ctx.Eval(`1 + 2`)
	require.NotNil(t, result)
	require.False(t, result.IsException())
	require.EqualValues(t, 3, result.ToInt32())
	result.Free()

	closed := make(chan struct{})
	go func() {
		ctx.Close()
		rt.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("cross-goroutine close blocked")
	}
}

func TestRuntimeStdHandlersLifecycle(t *testing.T) {
	rt := NewRuntime()
	require.False(t, rt.stdHandlersInitialized)

	ctx1 := rt.NewContext()
	require.NotNil(t, ctx1)
	require.True(t, rt.stdHandlersInitialized)

	ctx2 := rt.NewContext()
	require.NotNil(t, ctx2)
	require.True(t, rt.stdHandlersInitialized)

	ctx1.Close()
	ctx2.Close()
	rt.Close()

	require.False(t, rt.stdHandlersInitialized)
	require.Equal(t, 0, timeoutOpaqueCount())
}

func TestRuntimeCloseIdempotentAndCloseOrder(t *testing.T) {
	rt := NewRuntime()
	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	result := ctx.Eval(`1 + 1`)
	require.False(t, result.IsException())
	result.Free()

	require.NotPanics(t, func() {
		rt.Close()
	})
	require.NotPanics(t, func() {
		rt.Close()
	})
	require.NotPanics(t, func() {
		ctx.Close()
	})

	require.Nil(t, rt.NewContext())

	require.NotPanics(t, func() {
		rt.SetExecuteTimeout(1)
		rt.SetInterruptHandler(func() int { return 1 })
		rt.ClearInterruptHandler()
		rt.SetMemoryLimit(1024)
		rt.SetGCThreshold(2048)
		rt.SetMaxStackSize(4096)
		rt.SetCanBlock(true)
		rt.SetStripInfo(1)
		rt.SetModuleImport(true)
		rt.RunGC()
	})
}

func TestRuntimeNilAndZeroValueGuards(t *testing.T) {
	var nilRT *Runtime
	dummyRef := (Value{}).ref

	require.NotPanics(t, func() {
		nilRT.RunGC()
		nilRT.Close()
		nilRT.SetCanBlock(true)
		nilRT.SetMemoryLimit(1)
		nilRT.SetGCThreshold(1)
		nilRT.SetMaxStackSize(1)
		nilRT.SetExecuteTimeout(1)
		nilRT.SetStripInfo(1)
		nilRT.SetModuleImport(true)
		nilRT.SetInterruptHandler(func() int { return 0 })
		nilRT.ClearInterruptHandler()
		require.Nil(t, nilRT.NewContext())
		require.Equal(t, 0, nilRT.callInterruptHandler())
		nilRT.registerOwnedContext(nil)
		nilRT.unregisterOwnedContext(nil)
		nilRT.registerConstructorClassID(dummyRef, 1)
		_, _ = nilRT.getConstructorClassID(dummyRef)
	})

	zeroRT := &Runtime{}
	require.NotPanics(t, func() {
		zeroRT.RunGC()
		zeroRT.SetCanBlock(true)
		zeroRT.SetMemoryLimit(1)
		zeroRT.SetGCThreshold(1)
		zeroRT.SetMaxStackSize(1)
		zeroRT.SetExecuteTimeout(1)
		zeroRT.SetStripInfo(1)
		zeroRT.SetModuleImport(true)
		zeroRT.SetInterruptHandler(func() int { return 1 })
		zeroRT.ClearInterruptHandler()
		zeroRT.registerOwnedContext(nil)
		zeroRT.unregisterOwnedContext(nil)
		require.Nil(t, zeroRT.NewContext())
		zeroRT.Close()
		zeroRT.Close()
	})
}

func TestRuntimeNewContextFailureHook(t *testing.T) {
	restore := forceRuntimeNewContextFailureForTest(true)
	defer restore()

	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	require.Nil(t, ctx)
}

func TestRuntimeNewContextFailureHookDisable(t *testing.T) {
	restore := forceRuntimeNewContextFailureForTest(false)
	defer restore()

	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	ctx.Close()
}
