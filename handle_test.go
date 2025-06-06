package quickjs

import (
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleStore_Basic(t *testing.T) {
	hs := newHandleStore()
	require.NotNil(t, hs)

	// Test initial state
	assert.Equal(t, 0, hs.Count())

	// Test basic store/load/delete cycle
	id := hs.Store("test value")
	assert.Greater(t, id, int32(0))
	assert.Equal(t, 1, hs.Count())

	// Load
	value, ok := hs.Load(id)
	assert.True(t, ok)
	assert.Equal(t, "test value", value)

	// Delete
	assert.True(t, hs.Delete(id))
	assert.Equal(t, 0, hs.Count())

	// Load after delete should fail
	value, ok = hs.Load(id)
	assert.False(t, ok)
	assert.Nil(t, value)
}

func TestHandleStore_MultipleValues(t *testing.T) {
	hs := newHandleStore()

	// Store multiple different types
	testValues := []interface{}{
		"string",
		42,
		[]int{1, 2, 3},
		map[string]int{"key": 123},
		nil,
	}

	ids := make([]int32, len(testValues))
	for i, value := range testValues {
		ids[i] = hs.Store(value)
		assert.Greater(t, ids[i], int32(0))
	}

	assert.Equal(t, len(testValues), hs.Count())

	// Verify IDs are sequential
	for i := 1; i < len(ids); i++ {
		assert.Greater(t, ids[i], ids[i-1])
	}

	// Verify all values can be loaded
	for i, expectedValue := range testValues {
		loadedValue, ok := hs.Load(ids[i])
		assert.True(t, ok)
		assert.Equal(t, expectedValue, loadedValue)
	}

	// Test function storage separately (can't use == comparison)
	funcValue := func() int { return 42 }
	funcID := hs.Store(funcValue)
	loadedFunc, ok := hs.Load(funcID)
	assert.True(t, ok)
	assert.NotNil(t, loadedFunc)
	// Verify it's actually a function by calling it
	if fn, ok := loadedFunc.(func() int); ok {
		assert.Equal(t, 42, fn())
	}
	assert.True(t, hs.Delete(funcID))

	// Clear all
	hs.Clear()
	assert.Equal(t, 0, hs.Count())

	// Verify all values are gone after clear
	for _, id := range ids {
		value, ok := hs.Load(id)
		assert.False(t, ok)
		assert.Nil(t, value)
	}
}

func TestHandleStore_EdgeCases(t *testing.T) {
	t.Run("IDOverflow", func(t *testing.T) {
		// Test ID overflow at MaxInt32
		hs := newHandleStoreWithStartID(math.MaxInt32 - 2)

		// First store should succeed
		id1 := hs.Store("test1")
		assert.Equal(t, int32(math.MaxInt32-1), id1)

		// Second store should panic at MaxInt32
		assert.Panics(t, func() {
			hs.Store("test2")
		})

		assert.True(t, hs.Delete(id1))
	})

	t.Run("NegativeStartID", func(t *testing.T) {
		// Test immediate panic with negative start ID
		hs := newHandleStoreWithStartID(-1)
		assert.Panics(t, func() {
			hs.Store("test")
		}, "Should panic when ID becomes 0")
	})

	t.Run("LoadNonExistent", func(t *testing.T) {
		hs := newHandleStore()
		value, ok := hs.Load(999)
		assert.False(t, ok)
		assert.Nil(t, value)
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		hs := newHandleStore()
		success := hs.Delete(999)
		assert.False(t, success)
	})

	t.Run("DoubleDelete", func(t *testing.T) {
		hs := newHandleStore()
		id := hs.Store("test")
		assert.True(t, hs.Delete(id))  // First delete succeeds
		assert.False(t, hs.Delete(id)) // Second delete fails
	})

	t.Run("ClearEmpty", func(t *testing.T) {
		hs := newHandleStore()
		hs.Clear() // Should not panic
		assert.Equal(t, 0, hs.Count())
	})
}

func TestHandleStore_CustomStartID(t *testing.T) {
	t.Run("StartFromZero", func(t *testing.T) {
		hs := newHandleStoreWithStartID(0)
		id := hs.Store("test")
		assert.Equal(t, int32(1), id)
		assert.True(t, hs.Delete(id))
	})

	t.Run("StartFromCustom", func(t *testing.T) {
		const startID = int32(1000)
		hs := newHandleStoreWithStartID(startID)
		id := hs.Store("test")
		assert.Equal(t, startID+1, id)
		assert.True(t, hs.Delete(id))
	})

	t.Run("BoundaryNearOverflow", func(t *testing.T) {
		// Test behavior near MaxInt32
		hs := newHandleStoreWithStartID(math.MaxInt32 - 3)

		// Store 2 values successfully
		id1 := hs.Store("value1")
		id2 := hs.Store("value2")
		assert.Equal(t, int32(math.MaxInt32-2), id1)
		assert.Equal(t, int32(math.MaxInt32-1), id2)

		// Third store should panic
		assert.Panics(t, func() {
			hs.Store("value3")
		})

		// Clean up
		assert.True(t, hs.Delete(id1))
		assert.True(t, hs.Delete(id2))
	})
}

func TestHandleStore_Concurrency(t *testing.T) {
	hs := newHandleStore()
	const numGoroutines = 10
	const numOpsPerGoroutine = 100

	var wg sync.WaitGroup
	var successCount int64

	// Test concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Each goroutine does store/load/delete operations
			for j := 0; j < numOpsPerGoroutine; j++ {
				value := goroutineID*1000 + j

				// Store
				id := hs.Store(value)

				// Load and verify
				loadedValue, ok := hs.Load(id)
				if ok && loadedValue == value {
					atomic.AddInt64(&successCount, 1)
				}

				// Delete
				hs.Delete(id)
			}
		}(i)
	}

	wg.Wait()

	// Verify all operations succeeded
	expectedSuccesses := int64(numGoroutines * numOpsPerGoroutine)
	assert.Equal(t, expectedSuccesses, successCount)

	// Final cleanup
	hs.Clear()
	assert.Equal(t, 0, hs.Count())
}

func TestHandleStore_ZeroValues(t *testing.T) {
	hs := newHandleStore()

	// Test storing zero values
	zeroValues := []interface{}{
		"",            // empty string
		0,             // zero int
		false,         // false bool
		[]int{},       // empty slice
		map[int]int{}, // empty map
		nil,           // nil value
	}

	for i, zeroValue := range zeroValues {
		id := hs.Store(zeroValue)
		value, ok := hs.Load(id)
		assert.True(t, ok)
		assert.Equal(t, zeroValue, value, "Zero value %d should be stored correctly", i)
		assert.True(t, hs.Delete(id))
	}
}

func TestHandleStore_FunctionTypes(t *testing.T) {
	hs := newHandleStore()

	// Test storing different function types
	fn1 := func() int { return 1 }
	fn2 := func(x int) int { return x * 2 }
	fn3 := func(a, b string) string { return a + b }

	// Store functions
	id1 := hs.Store(fn1)
	id2 := hs.Store(fn2)
	id3 := hs.Store(fn3)

	// Load and verify functions work
	loadedFn1, ok := hs.Load(id1)
	assert.True(t, ok)
	if fn, ok := loadedFn1.(func() int); ok {
		assert.Equal(t, 1, fn())
	}

	loadedFn2, ok := hs.Load(id2)
	assert.True(t, ok)
	if fn, ok := loadedFn2.(func(int) int); ok {
		assert.Equal(t, 10, fn(5))
	}

	loadedFn3, ok := hs.Load(id3)
	assert.True(t, ok)
	if fn, ok := loadedFn3.(func(string, string) string); ok {
		assert.Equal(t, "hello world", fn("hello ", "world"))
	}

	// Clean up
	assert.True(t, hs.Delete(id1))
	assert.True(t, hs.Delete(id2))
	assert.True(t, hs.Delete(id3))
}

func TestHandleStore_ComplexTypes(t *testing.T) {
	hs := newHandleStore()

	// Test complex data structures
	complexData := map[string]interface{}{
		"string":  "hello",
		"number":  42,
		"boolean": true,
		"array":   []int{1, 2, 3},
		"nested": map[string]interface{}{
			"inner": "value",
		},
	}

	id := hs.Store(complexData)
	loaded, ok := hs.Load(id)
	assert.True(t, ok)

	// Use reflect.DeepEqual for complex comparisons
	assert.True(t, reflect.DeepEqual(complexData, loaded))

	assert.True(t, hs.Delete(id))
}
