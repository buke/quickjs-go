package quickjs_test

import (
	"sync"
	"testing"
	"time"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestNewRuntimeDefault tests creating a runtime with default options.
func TestNewRuntimeDefault(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Should be able to create a context
	ctx := rt.NewContext()
	defer ctx.Close()

	// Should be able to execute basic JavaScript
	result, err := ctx.Eval(`1 + 1`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, 2, result.ToInt32())
}

// TestNewRuntimeWithOptions tests creating a runtime with various options.
func TestNewRuntimeWithOptions(t *testing.T) {
	rt := quickjs.NewRuntime(
		quickjs.WithExecuteTimeout(30),
		quickjs.WithMemoryLimit(128*1024),
		quickjs.WithGCThreshold(256*1024),
		quickjs.WithMaxStackSize(65534),
		quickjs.WithCanBlock(true),
		quickjs.WithModuleImport(true),
		quickjs.WithStripInfo(1),
	)
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test that the runtime works with all options set
	result, err := ctx.Eval(`"Hello World"`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "Hello World", result.String())
}

// TestWithExecuteTimeout tests the execute timeout option.
func TestWithExecuteTimeout(t *testing.T) {
	rt := quickjs.NewRuntime(quickjs.WithExecuteTimeout(1))
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// This should timeout
	_, err := ctx.Eval(`while(true){}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "interrupted")
}

// TestWithMemoryLimit tests the memory limit option.
func TestWithMemoryLimit(t *testing.T) {
	rt := quickjs.NewRuntime(quickjs.WithMemoryLimit(512 * 1024)) // 512KB limit
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Try to allocate more memory than the limit
	result, err := ctx.Eval(`var array = []; while (true) { array.push(null) }`)
	defer result.Free()
	require.Error(t, err)

	require.Contains(t, err.Error(), "out of memory")
}

// TestWithGCThreshold tests the GC threshold option.
func TestWithGCThreshold(t *testing.T) {
	// Test with GC enabled
	rt1 := quickjs.NewRuntime(quickjs.WithGCThreshold(1024))
	defer rt1.Close()

	ctx1 := rt1.NewContext()
	defer ctx1.Close()

	result1, err := ctx1.Eval(`"GC enabled"`)
	require.NoError(t, err)
	defer result1.Free()
	require.EqualValues(t, "GC enabled", result1.String())

	// Test with GC disabled
	rt2 := quickjs.NewRuntime(quickjs.WithGCThreshold(-1))
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result2, err := ctx2.Eval(`"GC disabled"`)
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, "GC disabled", result2.String())
}

// TestWithMaxStackSize tests the max stack size option.
func TestWithMaxStackSize(t *testing.T) {
	rt := quickjs.NewRuntime(quickjs.WithMaxStackSize(8192)) // Small stack
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// This should cause a stack overflow
	_, err := ctx.Eval(`
        function recursive(n) {
            if (n <= 0) return 0;
            return recursive(n - 1) + 1;
        }
        recursive(10000);
    `)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stack overflow")
}

// TestWithCanBlock tests the can block option.
func TestWithCanBlock(t *testing.T) {
	// Test with blocking enabled
	rt1 := quickjs.NewRuntime(quickjs.WithCanBlock(true))
	defer rt1.Close()

	ctx1 := rt1.NewContext()
	defer ctx1.Close()

	result1, err := ctx1.Eval(`"blocking enabled"`)
	require.NoError(t, err)
	defer result1.Free()
	require.EqualValues(t, "blocking enabled", result1.String())

	// Test with blocking disabled
	rt2 := quickjs.NewRuntime(quickjs.WithCanBlock(false))
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result2, err := ctx2.Eval(`"blocking disabled"`)
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, "blocking disabled", result2.String())
}

// TestWithModuleImport tests the module import option.
func TestWithModuleImport(t *testing.T) {
	rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic module functionality (we can't test actual file imports without test files)
	result, err := ctx.Eval(`"module import enabled"`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "module import enabled", result.String())
}

// TestWithStripInfo tests the strip info option.
func TestWithStripInfo(t *testing.T) {
	// Test with strip info enabled (1)
	rt1 := quickjs.NewRuntime(quickjs.WithStripInfo(1))
	defer rt1.Close()

	ctx1 := rt1.NewContext()
	defer ctx1.Close()

	result1, err := ctx1.Eval(`"strip info enabled"`)
	require.NoError(t, err)
	defer result1.Free()
	require.EqualValues(t, "strip info enabled", result1.String())

	// Test with strip info disabled (0)
	rt2 := quickjs.NewRuntime(quickjs.WithStripInfo(0))
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result2, err := ctx2.Eval(`"strip info disabled"`)
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, "strip info disabled", result2.String())
}

// TestRuntimeRunGC tests the garbage collection functionality.
func TestRuntimeRunGC(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create some objects to garbage collect
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
}

// TestRuntimeSetMemoryLimit tests setting memory limit after runtime creation.
func TestRuntimeSetMemoryLimit(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Set a restrictive memory limit
	rt.SetMemoryLimit(512 * 1024) // 512KB

	ctx := rt.NewContext()
	defer ctx.Close()

	// Try to allocate more memory than the limit
	result, err := ctx.Eval(`var array = []; while (true) { array.push(null) }`)
	defer result.Free()
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of memory")

}

// TestRuntimeSetGCThreshold tests setting GC threshold after runtime creation.
func TestRuntimeSetGCThreshold(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Set GC threshold
	rt.SetGCThreshold(1024)

	ctx := rt.NewContext()
	defer ctx.Close()

	result, err := ctx.Eval(`"GC threshold set"`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "GC threshold set", result.String())
}

// TestRuntimeSetMaxStackSize tests setting max stack size after runtime creation.
func TestRuntimeSetMaxStackSize(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Set a small stack size
	rt.SetMaxStackSize(8192)

	ctx := rt.NewContext()
	defer ctx.Close()

	// This should cause a stack overflow
	_, err := ctx.Eval(`
        function recursive(n) {
            if (n <= 0) return 0;
            return recursive(n - 1) + 1;
        }
        recursive(10000);
    `)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stack overflow")
}

// TestRuntimeSetCanBlock tests setting can block after runtime creation.
func TestRuntimeSetCanBlock(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Test setting can block to true
	rt.SetCanBlock(true)

	ctx := rt.NewContext()
	defer ctx.Close()

	result, err := ctx.Eval(`"blocking enabled"`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "blocking enabled", result.String())

	// Test setting can block to false
	rt.SetCanBlock(false)

	result2, err := ctx.Eval(`"blocking disabled"`)
	require.NoError(t, err)
	defer result2.Free()
	require.EqualValues(t, "blocking disabled", result2.String())
}

// TestRuntimeSetExecuteTimeout tests setting execute timeout after runtime creation.
func TestRuntimeSetExecuteTimeout(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Set a short timeout
	rt.SetExecuteTimeout(1)

	// This should timeout
	_, err := ctx.Eval(`while(true){}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "interrupted")
}

// TestRuntimeSetStripInfo tests setting strip info after runtime creation.
func TestRuntimeSetStripInfo(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Set strip info
	rt.SetStripInfo(1)

	ctx := rt.NewContext()
	defer ctx.Close()

	result, err := ctx.Eval(`"strip info set"`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "strip info set", result.String())
}

// TestRuntimeSetInterruptHandler tests the interrupt handler functionality.
func TestRuntimeSetInterruptHandler(t *testing.T) {
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
}

// TestRuntimeNewContext tests creating multiple contexts from the same runtime.
func TestRuntimeNewContext(t *testing.T) {
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
}

// TestRuntimeConcurrency tests concurrent usage of multiple runtime instances.
func TestRuntimeConcurrency(t *testing.T) {
	n := 8   // Reduce concurrent goroutines to avoid resource issues
	m := 100 // Reduce operations per goroutine

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

// TestRuntimeClose tests proper cleanup when closing runtime.
func TestRuntimeClose(t *testing.T) {
	rt := quickjs.NewRuntime()

	ctx := rt.NewContext()

	// Execute some code before closing
	result, err := ctx.Eval(`"before close"`)
	require.NoError(t, err)
	require.EqualValues(t, "before close", result.String())

	// Free the result before closing context/runtime
	result.Free()

	// Close context first
	ctx.Close()

	// Close runtime
	rt.Close()

	// After closing, we can't use the runtime anymore
	// This test mainly ensures no crashes occur during cleanup
}

// TestRuntimeWithZeroValues tests runtime with zero/default values for options.
func TestRuntimeWithZeroValues(t *testing.T) {
	rt := quickjs.NewRuntime(
		quickjs.WithExecuteTimeout(0), // No timeout
		quickjs.WithMemoryLimit(0),    // No memory limit
		quickjs.WithGCThreshold(0),    // Default GC
		quickjs.WithMaxStackSize(0),   // No stack limit
	)
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Should work normally with zero values
	result, err := ctx.Eval(`"zero values work"`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "zero values work", result.String())
}

// TestRuntimeEdgeCases tests various edge cases for runtime configuration.
func TestRuntimeEdgeCases(t *testing.T) {
	// Test with very small memory limit
	rt1 := quickjs.NewRuntime(quickjs.WithMemoryLimit(32768)) // 32k byte - extremely restrictive
	defer rt1.Close()

	ctx1 := rt1.NewContext()
	defer ctx1.Close()

	// Even basic operations might fail with such a small limit
	_, err := ctx1.Eval(`1`)
	// We don't require this to fail, but if it does, it should be memory-related
	if err != nil {
		require.Contains(t, err.Error(), "memory")
	}

	// Test with very large timeout
	rt2 := quickjs.NewRuntime(quickjs.WithExecuteTimeout(999999999)) // 999999999 ms - effectively no timeout
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result, err := ctx2.Eval(`"large timeout works"`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "large timeout works", result.String())
}
