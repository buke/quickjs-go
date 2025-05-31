package quickjs_test

import (
	"sync"
	"testing"
	"time"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestRuntimeBasics tests basic runtime creation and operations
func TestRuntimeBasics(t *testing.T) {
	// Test default runtime
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	result, err := ctx.Eval(`1 + 1`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, 2, result.ToInt32())

	// Test runtime with all options
	rt2 := quickjs.NewRuntime(
		quickjs.WithExecuteTimeout(30),
		quickjs.WithMemoryLimit(128*1024),
		quickjs.WithGCThreshold(256*1024),
		quickjs.WithMaxStackSize(65534),
		quickjs.WithCanBlock(true),
		quickjs.WithModuleImport(true),
		quickjs.WithStripInfo(1),
	)
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result2, err := ctx2.Eval(`"Hello World"`)
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, "Hello World", result2.String())

	// Test with zero values (should work normally)
	rt3 := quickjs.NewRuntime(
		quickjs.WithExecuteTimeout(0),
		quickjs.WithMemoryLimit(0),
		quickjs.WithGCThreshold(0),
		quickjs.WithMaxStackSize(0),
	)
	defer rt3.Close()

	ctx3 := rt3.NewContext()
	defer ctx3.Close()

	result3, err := ctx3.Eval(`"zero values work"`)
	require.NoError(t, err)
	defer result3.Free()
	require.EqualValues(t, "zero values work", result3.String())
}

// TestRuntimeLimitsAndTimeouts tests memory limits, timeouts, and stack limits
func TestRuntimeLimitsAndTimeouts(t *testing.T) {
	// Test execute timeout
	rt1 := quickjs.NewRuntime(quickjs.WithExecuteTimeout(1))
	defer rt1.Close()

	ctx1 := rt1.NewContext()
	defer ctx1.Close()

	_, err := ctx1.Eval(`while(true){}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "interrupted")

	// Test memory limit
	rt2 := quickjs.NewRuntime(quickjs.WithMemoryLimit(512 * 1024))
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result, err := ctx2.Eval(`var array = []; while (true) { array.push(null) }`)
	defer result.Free()
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of memory")

	// Test stack overflow
	rt3 := quickjs.NewRuntime(quickjs.WithMaxStackSize(8192))
	defer rt3.Close()

	ctx3 := rt3.NewContext()
	defer ctx3.Close()

	_, err = ctx3.Eval(`
        function recursive(n) {
            if (n <= 0) return 0;
            return recursive(n - 1) + 1;
        }
        recursive(10000);
    `)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stack overflow")
}

// TestRuntimeConfigurationMethods tests runtime configuration setters
func TestRuntimeConfigurationMethods(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test SetMemoryLimit
	rt.SetMemoryLimit(512 * 1024)
	result, err := ctx.Eval(`var array = []; while (true) { array.push(null) }`)
	defer result.Free()
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of memory")

	// Create new runtime for other tests (previous one is memory limited)
	rt2 := quickjs.NewRuntime()
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	// Test SetExecuteTimeout
	rt2.SetExecuteTimeout(1)
	_, err = ctx2.Eval(`while(true){}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "interrupted")

	// Create new runtime for stack test
	rt3 := quickjs.NewRuntime()
	defer rt3.Close()

	ctx3 := rt3.NewContext()
	defer ctx3.Close()

	// Test SetMaxStackSize
	rt3.SetMaxStackSize(8192)
	_, err = ctx3.Eval(`
        function recursive(n) {
            if (n <= 0) return 0;
            return recursive(n - 1) + 1;
        }
        recursive(10000);
    `)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stack overflow")

	// Test other setters (these don't have easy ways to verify effect)
	rt4 := quickjs.NewRuntime()
	defer rt4.Close()

	rt4.SetGCThreshold(1024)
	rt4.SetCanBlock(true)
	rt4.SetCanBlock(false)
	rt4.SetStripInfo(1)

	ctx4 := rt4.NewContext()
	defer ctx4.Close()

	result4, err := ctx4.Eval(`"configuration methods work"`)
	require.NoError(t, err)
	defer result4.Free()
	require.EqualValues(t, "configuration methods work", result4.String())
}

// TestRuntimeGarbageCollection tests garbage collection functionality
func TestRuntimeGarbageCollection(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create objects for GC
	result, err := ctx.Eval(`
        var objects = [];
        for(let i = 0; i < 1000; i++) {
            objects.push({data: new Array(100).fill(i)});
        }
        objects.length;
    `)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, 1000, result.ToInt32())

	// Run garbage collection
	rt.RunGC()

	// Verify runtime still works after GC
	result2, err := ctx.Eval(`"GC completed"`)
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, "GC completed", result2.String())

	// Test GC threshold options
	rt2 := quickjs.NewRuntime(quickjs.WithGCThreshold(1024))
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result3, err := ctx2.Eval(`"GC enabled"`)
	require.NoError(t, err)
	defer result3.Free()
	require.EqualValues(t, "GC enabled", result3.String())

	// Test GC disabled
	rt3 := quickjs.NewRuntime(quickjs.WithGCThreshold(-1))
	defer rt3.Close()

	ctx3 := rt3.NewContext()
	defer ctx3.Close()

	result4, err := ctx3.Eval(`"GC disabled"`)
	require.NoError(t, err)
	defer result4.Free()
	require.EqualValues(t, "GC disabled", result4.String())
}

// TestRuntimeInterruptHandler tests interrupt handler functionality
func TestRuntimeInterruptHandler(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	startTime := time.Now()
	interruptCalled := false

	// Set interrupt handler that triggers after 1 second
	rt.SetInterruptHandler(func() int {
		interruptCalled = true
		if time.Since(startTime) > time.Second {
			return 1 // Signal to interrupt
		}
		return 0 // Continue execution
	})

	ctx := rt.NewContext()
	defer ctx.Close()

	// Execute infinite loop - should be interrupted
	_, err := ctx.Eval(`while(true){}`)

	require.Error(t, err)
	require.Contains(t, err.Error(), "interrupted")
	require.True(t, interruptCalled, "Interrupt handler should have been called")

	// Test replacing interrupt handler (covers the handler cleanup logic)
	secondInterruptCalled := false
	rt.SetInterruptHandler(func() int {
		secondInterruptCalled = true
		return 1 // Interrupt immediately
	})

	_, err = ctx.Eval(`while(true){}`)
	require.Error(t, err)
	require.True(t, secondInterruptCalled, "Second interrupt handler should have been called")

}

// TestRuntimeMultipleContexts tests creating and using multiple contexts
func TestRuntimeMultipleContexts(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Create multiple contexts
	ctx1 := rt.NewContext()
	defer ctx1.Close()

	ctx2 := rt.NewContext()
	defer ctx2.Close()

	// Test that contexts are independent
	result1, err := ctx1.Eval(`var x = "context1"; x`)
	require.NoError(t, err)
	defer result1.Free()
	require.EqualValues(t, "context1", result1.String())

	result2, err := ctx2.Eval(`var x = "context2"; x`)
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, "context2", result2.String())

	// Verify contexts don't interfere with each other
	result3, err := ctx1.Eval(`x`)
	require.NoError(t, err)
	defer result3.Free()
	require.EqualValues(t, "context1", result3.String())

	result4, err := ctx2.Eval(`x`)
	require.NoError(t, err)
	defer result4.Free()
	require.EqualValues(t, "context2", result4.String())
}

// TestRuntimeConcurrency tests concurrent usage of multiple runtime instances
func TestRuntimeConcurrency(t *testing.T) {
	n := 4  // Reduce concurrent goroutines
	m := 50 // Reduce operations per goroutine

	var wg sync.WaitGroup
	wg.Add(n)

	results := make(chan int64, n*m)

	// Start n goroutines, each with its own runtime
	for i := 0; i < n; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			// Each goroutine gets its own runtime and context
			rt := quickjs.NewRuntime()
			defer rt.Close()

			ctx := rt.NewContext()
			defer ctx.Close()

			// Perform m operations
			for j := 0; j < m; j++ {
				result, err := ctx.Eval(`new Date().getTime()`)
				require.NoError(t, err)

				results <- result.ToInt64()
				result.Free()
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	// Verify we got all expected results
	resultCount := 0
	for range results {
		resultCount++
	}
	require.Equal(t, n*m, resultCount)
}
