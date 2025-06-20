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
	require.Equal(t, "Hello World", result2.String())
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
	require.Equal(t, "configuration test", result.String())
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
	require.Equal(t, "ctx1", result1.String())

	result2 := ctx2.Eval(`var x = "ctx2"; x`)
	defer result2.Free()
	require.False(t, result2.IsException()) // Check for exceptions instead of error
	require.Equal(t, "ctx2", result2.String())

	// Verify isolation
	result3 := ctx1.Eval(`x`)
	defer result3.Free()
	require.False(t, result3.IsException()) // Check for exceptions instead of error
	require.Equal(t, "ctx1", result3.String())
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
	require.Equal(t, "canBlock disabled", result1.String())

	// Test WithModuleImport(true)
	rt2 := NewRuntime(WithModuleImport(true))
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result2 := ctx2.Eval(`"module import enabled"`)
	defer result2.Free()
	require.False(t, result2.IsException()) // Check for exceptions instead of error
	require.Equal(t, "module import enabled", result2.String())

	// Test WithStripInfo(0)
	rt3 := NewRuntime(WithStripInfo(0))
	defer rt3.Close()

	ctx3 := rt3.NewContext()
	defer ctx3.Close()

	result3 := ctx3.Eval(`"strip info test"`)
	defer result3.Free()
	require.False(t, result3.IsException()) // Check for exceptions instead of error
	require.Equal(t, "strip info test", result3.String())

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
	require.Equal(t, "GC test", result4.String())
}
