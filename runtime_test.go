package quickjs

import (
	"errors"
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
		rt := NewRuntime(WithMaxStackSize(65534))
		defer rt.Close()

		ctx := rt.NewContext()
		require.NotNil(t, ctx)
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
		errMsg := strings.ToLower(err.Error())
		require.True(t,
			strings.Contains(errMsg, "stack overflow") || strings.Contains(errMsg, "maximum call stack size exceeded"),
			"unexpected stack overflow error: %s", err.Error(),
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

func TestAwaitPollSliceMsConfig(t *testing.T) {
	original := GetAwaitPollSliceMs()
	t.Cleanup(func() {
		SetAwaitPollSliceMs(original)
	})

	SetAwaitPollSliceMs(7)
	require.Equal(t, 7, GetAwaitPollSliceMs())

	SetAwaitPollSliceMs(0)
	require.Equal(t, 7, GetAwaitPollSliceMs())

	SetAwaitPollSliceMs(-3)
	require.Equal(t, 7, GetAwaitPollSliceMs())
}

// TestRuntimeInterruptHandler tests interrupt handler functionality and coverage
func TestRuntimeInterruptHandler(t *testing.T) {
	useStableOwnerHooksForLegacySubtests(t)

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

func TestRuntimeContextIDAndClassObjectIdentityRegistry(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx1 := rt.NewContext()
	defer ctx1.Close()
	ctx2 := rt.NewContext()
	defer ctx2.Close()

	require.NotZero(t, ctx1.contextID)
	require.NotZero(t, ctx2.contextID)
	require.NotEqual(t, ctx1.contextID, ctx2.contextID)
	require.Equal(t, ctx1, rt.getOwnedContextByID(ctx1.contextID))
	require.Equal(t, ctx2, rt.getOwnedContextByID(ctx2.contextID))

	objID := rt.registerClassObjectIdentity(ctx1.contextID, 11)
	require.NotZero(t, objID)

	identity, ok := rt.getClassObjectIdentity(objID)
	require.True(t, ok)
	require.Equal(t, ctx1.contextID, identity.contextID)
	require.Equal(t, int32(11), identity.handleID)

	taken, ok := rt.takeClassObjectIdentity(objID)
	require.True(t, ok)
	require.Equal(t, identity, taken)
	_, ok = rt.getClassObjectIdentity(objID)
	require.False(t, ok)

	ctx2Obj1 := rt.registerClassObjectIdentity(ctx2.contextID, 21)
	ctx2Obj2 := rt.registerClassObjectIdentity(ctx2.contextID, 22)
	require.NotZero(t, ctx2Obj1)
	require.NotZero(t, ctx2Obj2)
	rt.cleanupClassObjectIdentitiesByContext(ctx2.contextID)
	_, ok = rt.getClassObjectIdentity(ctx2Obj1)
	require.False(t, ok)
	_, ok = rt.getClassObjectIdentity(ctx2Obj2)
	require.False(t, ok)
}

func TestRuntimeNextClassObjectIDOverflowReturnsZero(t *testing.T) {
	const maxInt32 = int32(^uint32(0) >> 1)

	rt := &Runtime{}
	rt.classObjectIDCounter.Store(maxInt32 - 1)

	first := rt.nextClassObjectID()
	require.Equal(t, -(maxInt32), first)
	second := rt.nextClassObjectID()
	require.Zero(t, second)
}

func TestRuntimeRegisterClassObjectIdentityOverflowReturnsZero(t *testing.T) {
	const maxInt32 = int32(^uint32(0) >> 1)

	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	rt.classObjectIDCounter.Store(maxInt32)
	objectID := rt.registerClassObjectIdentity(ctx.contextID, 1)
	require.Zero(t, objectID)

	seen := false
	rt.classObjectRegistry.Range(func(_, _ interface{}) bool {
		seen = true
		return false
	})
	require.False(t, seen)
}

func TestRuntimeRegisterClassObjectIdentityCorruptedBucketConcurrent(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	rt.classObjectIDsByCtx.Store(ctx.contextID, "corrupted-bucket")

	const workers = 32
	ids := make(chan int32, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ids <- rt.registerClassObjectIdentity(ctx.contextID, int32(i+1))
		}(i)
	}
	wg.Wait()
	close(ids)

	seen := make(map[int32]struct{}, workers)
	for id := range ids {
		require.Less(t, id, int32(0))
		_, exists := seen[id]
		require.False(t, exists)
		seen[id] = struct{}{}

		identity, ok := rt.getClassObjectIdentity(id)
		require.True(t, ok)
		require.Equal(t, ctx.contextID, identity.contextID)
	}
	require.Equal(t, workers, len(seen))
}

func TestRuntimeRegisterClassObjectIdentityCorruptedBucketRepairedBeforeLock(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	rt.classObjectIDsByCtx.Store(ctx.contextID, "corrupted-bucket")

	// Hold the mutex so registerClassObjectIdentity blocks on the corruption path.
	rt.mu.Lock()
	resultCh := make(chan int32, 1)
	go func() {
		resultCh <- rt.registerClassObjectIdentity(ctx.contextID, 99)
	}()

	// Wait until the goroutine has stored identity metadata, which happens before
	// it attempts to take rt.mu in the corruption branch.
	deadline := time.Now().Add(2 * time.Second)
	for {
		ready := false
		rt.classObjectRegistry.Range(func(_, _ interface{}) bool {
			ready = true
			return false
		})
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for identity registration pre-lock state")
		}
	}

	// Repair the bucket before releasing the lock so the double-check path
	// observes a typed *sync.Map value.
	repairedBucket := &sync.Map{}
	rt.classObjectIDsByCtx.Store(ctx.contextID, repairedBucket)
	rt.mu.Unlock()

	objectID := <-resultCh
	require.Less(t, objectID, int32(0))

	identity, ok := rt.getClassObjectIdentity(objectID)
	require.True(t, ok)
	require.Equal(t, ctx.contextID, identity.contextID)
	require.Equal(t, int32(99), identity.handleID)

	_, exists := repairedBucket.Load(objectID)
	require.True(t, exists)
}

func TestRuntimeRegisterClassObjectIdentityCorruptionBranchesDeterministic(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Normal typed-bucket path.
	normalID := rt.registerClassObjectIdentity(ctx.contextID, 1)
	require.NotZero(t, normalID)

	// Corruption replacement path.
	rt.classObjectIDsByCtx.Store(ctx.contextID, "corrupted-bucket")
	replacementID := rt.registerClassObjectIdentity(ctx.contextID, 2)
	require.NotZero(t, replacementID)
	bucketValue, ok := rt.classObjectIDsByCtx.Load(ctx.contextID)
	require.True(t, ok)
	_, typed := bucketValue.(*sync.Map)
	require.True(t, typed)

	// Corruption repaired-before-lock path (existing typed bucket reused).
	rt.classObjectIDsByCtx.Store(ctx.contextID, "corrupted-bucket-again")
	repairedBucket := &sync.Map{}

	rt.mu.Lock()
	resultCh := make(chan int32, 1)
	go func() {
		resultCh <- rt.registerClassObjectIdentity(ctx.contextID, 3)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		found := false
		rt.classObjectRegistry.Range(func(_, value interface{}) bool {
			identity, ok := value.(classObjectIdentity)
			if ok && identity.handleID == 3 {
				found = true
				return false
			}
			return true
		})
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for registerClassObjectIdentity pre-lock state")
		}
	}

	rt.classObjectIDsByCtx.Store(ctx.contextID, repairedBucket)
	rt.mu.Unlock()

	repairedID := <-resultCh
	require.NotZero(t, repairedID)
	_, exists := repairedBucket.Load(repairedID)
	require.True(t, exists)
}

func TestRuntimeIdentityRegistryCorruptionFailClosed(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	rt.contextsByID.Store(uint64(9991), "bad-context-type")
	require.Nil(t, rt.getOwnedContextByID(9991))

	rt.contextsByID.Store(uint64(9992), &Context{})
	require.Nil(t, rt.getOwnedContextByID(9992))

	rt.classObjectRegistry.Store(int32(-7001), "bad-identity-type")
	_, ok := rt.getClassObjectIdentity(-7001)
	require.False(t, ok)

	rt.classObjectRegistry.Store(int32(-7002), classObjectIdentity{})
	_, ok = rt.getClassObjectIdentity(-7002)
	require.False(t, ok)

	rt.classObjectRegistry.Store(int32(-7003), "bad-take-type")
	_, ok = rt.takeClassObjectIdentity(-7003)
	require.False(t, ok)

	rt.classObjectRegistry.Store(int32(-7004), classObjectIdentity{})
	_, ok = rt.takeClassObjectIdentity(-7004)
	require.False(t, ok)

	rt.classObjectIDsByCtx.Store(ctx.contextID, "bad-bucket")
	h := ctx.handleStore.Store("bucket-replacement")
	defer ctx.handleStore.Delete(h)
	objID := rt.registerClassObjectIdentity(ctx.contextID, h)
	require.NotZero(t, objID)

	rt.classObjectIDsByCtx.Store(uint64(8101), "bad-cleanup-bucket")
	rt.cleanupClassObjectIdentitiesByContext(8101)

	bucket := &sync.Map{}
	bucket.Store("bad-key", struct{}{})
	bucket.Store(int32(-9002), struct{}{})
	rt.classObjectRegistry.Store(int32(-9002), classObjectIdentity{contextID: ctx.contextID, handleID: 77})
	rt.classObjectIDsByCtx.Store(uint64(8102), bucket)
	rt.cleanupClassObjectIdentitiesByContext(8102)
	_, ok = rt.getClassObjectIdentity(-9002)
	require.False(t, ok)
}

func TestRuntimeCloseCorruptedOwnedContextsMap(t *testing.T) {
	rt := NewRuntime()
	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	rt.contexts.Store("corrupted-key", "corrupted-value")
	require.NotPanics(t, func() {
		rt.Close()
	})
}

func TestRuntimeCloseClearsInternalRegistries(t *testing.T) {
	rt := NewRuntime()
	ctx := rt.NewContext()
	require.NotNil(t, ctx)

	constructor, _ := NewClassBuilder("CloseCoverageClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return map[string]int{"ok": 1}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())
	ctx.Globals().Set("CloseCoverageClass", constructor)

	instance := ctx.Eval(`globalThis.__close_cov_obj = new CloseCoverageClass(); __close_cov_obj`)
	require.False(t, instance.IsException())
	instance.Free()

	constructorEntries := 0
	rt.constructorRegistry.Range(func(_, _ interface{}) bool {
		constructorEntries++
		return true
	})
	require.Greater(t, constructorEntries, 0)

	classObjectEntries := 0
	rt.classObjectRegistry.Range(func(_, _ interface{}) bool {
		classObjectEntries++
		return true
	})
	require.Greater(t, classObjectEntries, 0)

	orphanContextID := uint64(1 << 62)
	rt.classObjectRegistry.Store(int32(-12345), classObjectIdentity{contextID: orphanContextID, handleID: 1})
	rt.classObjectIDsByCtx.Store(orphanContextID, &sync.Map{})

	require.NotPanics(t, func() {
		rt.Close()
	})
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
	ownerClosed := make(chan struct{})
	closeRequested := make(chan struct{})

	go func() {
		rt := NewRuntime()
		created <- rt
		<-closeRequested
		rt.Close()
		close(ownerClosed)
	}()

	rt := <-created
	require.NotNil(t, rt)

	// Cross-goroutine access is rejected by owner contract.
	require.Nil(t, rt.NewContext())

	// Non-owner close is fail-closed no-op; owner goroutine must close it.
	rt.Close()
	require.NotNil(t, rt.ref)

	close(closeRequested)

	select {
	case <-ownerClosed:
		require.Nil(t, rt.ref)
	case <-time.After(2 * time.Second):
		t.Fatal("owner goroutine close blocked")
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
		require.Nil(t, nilRT.NewBareContext())
		require.Nil(t, nilRT.NewContextWithOptions(DefaultBootstrap()))
		require.Equal(t, 0, nilRT.callInterruptHandler())
		nilRT.registerOwnedContext(nil)
		nilRT.unregisterOwnedContext(nil, 0)
		nilRT.registerConstructorClassID(dummyRef, 1)
		_, _ = nilRT.getConstructorClassID(dummyRef)
		require.Zero(t, nilRT.nextContextID())
		require.Zero(t, nilRT.nextClassObjectID())
		require.Zero(t, nilRT.registerClassObjectIdentity(1, 1))
		_, _ = nilRT.getClassObjectIdentity(1)
		_, _ = nilRT.takeClassObjectIdentity(1)
		require.Nil(t, nilRT.getOwnedContextByID(1))
		nilRT.cleanupClassObjectIdentitiesByContext(1)
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
		zeroRT.unregisterOwnedContext(nil, 0)
		zeroRT.cleanupClassObjectIdentitiesByContext(0)
		require.Zero(t, zeroRT.registerClassObjectIdentity(0, 1))
		require.Zero(t, zeroRT.registerClassObjectIdentity(1, 0))
		require.Nil(t, zeroRT.NewContext())
		require.Nil(t, zeroRT.NewBareContext())
		require.Nil(t, zeroRT.NewContextWithOptions(DefaultBootstrap()))
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

func TestRuntimeNewContextInitFailureHook(t *testing.T) {
	restore := forceRuntimeInitFailureForTest(true)
	defer restore()

	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	require.Nil(t, ctx)

	// A failed initialization should not poison the runtime for future contexts.
	restoreInit := forceRuntimeInitFailureForTest(false)
	defer restoreInit()

	ctx = rt.NewContext()
	require.NotNil(t, ctx)
	ctx.Close()
}

func TestRuntimeContextBootstrapOptions(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	typeOfSetTimeout := func(ctx *Context) string {
		v := ctx.Eval(`typeof globalThis.setTimeout`)
		require.NotNil(t, v)
		defer v.Free()
		require.False(t, v.IsException())
		return v.ToString()
	}

	defaultCtx := rt.NewContext()
	require.NotNil(t, defaultCtx)
	require.Equal(t, "function", typeOfSetTimeout(defaultCtx))
	defaultCtx.Close()

	bareCtx := rt.NewBareContext()
	require.NotNil(t, bareCtx)
	require.Equal(t, "undefined", typeOfSetTimeout(bareCtx))
	require.True(t, BootstrapStdOS(bareCtx))
	require.True(t, BootstrapTimers(bareCtx))
	require.Equal(t, "function", typeOfSetTimeout(bareCtx))
	bareCtx.Close()

	minimalCtx := rt.NewContextWithOptions(MinimalBootstrap())
	require.NotNil(t, minimalCtx)
	require.Equal(t, "undefined", typeOfSetTimeout(minimalCtx))
	require.True(t, BootstrapTimers(minimalCtx))
	require.Equal(t, "function", typeOfSetTimeout(minimalCtx))
	minimalCtx.Close()

	noBootstrapCtx := rt.NewContextWithOptions(NoBootstrap())
	require.NotNil(t, noBootstrapCtx)
	require.Equal(t, "undefined", typeOfSetTimeout(noBootstrapCtx))
	noBootstrapCtx.Close()

	customCtx := rt.NewContextWithOptions(DefaultBootstrap(), WithBootstrapTimers(false))
	require.NotNil(t, customCtx)
	require.Equal(t, "undefined", typeOfSetTimeout(customCtx))
	customCtx.Close()

	autoStdCtx := rt.NewContextWithOptions(WithBootstrapStdOS(false), WithBootstrapTimers(true))
	require.NotNil(t, autoStdCtx)
	require.Equal(t, "function", typeOfSetTimeout(autoStdCtx))
	autoStdCtx.Close()
}

func TestRuntimeContextBootstrapFailClosedAfterClose(t *testing.T) {
	require.False(t, BootstrapStdOS(nil))
	require.False(t, BootstrapTimers(nil))

	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewBareContext()
	require.NotNil(t, ctx)
	ctx.Close()

	require.False(t, BootstrapStdOS(ctx))
	require.False(t, BootstrapTimers(ctx))
}

func TestRuntimeContextBootstrapOwnerDenied(t *testing.T) {
	oldGIDHook := ownerCheckCurrentGoroutineID
	oldThreadHook := ownerCheckCurrentThreadID
	defer func() {
		ownerCheckCurrentGoroutineID = oldGIDHook
		ownerCheckCurrentThreadID = oldThreadHook
	}()

	var gid atomic.Uint64
	gid.Store(71)
	ownerCheckCurrentGoroutineID = func() uint64 { return gid.Load() }
	ownerCheckCurrentThreadID = func() uint64 { return 1 }

	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewBareContext()
	require.NotNil(t, ctx)

	gid.Store(72)
	require.False(t, BootstrapStdOS(ctx))
	require.False(t, BootstrapTimers(ctx))

	gid.Store(71)
	ctx.Close()
}

func TestRuntimeContextBootstrapHooks(t *testing.T) {
	oldStdOSHook := runtimeBootstrapStdOSHook
	oldTimersHook := runtimeBootstrapTimersHook
	defer func() {
		runtimeBootstrapStdOSHook = oldStdOSHook
		runtimeBootstrapTimersHook = oldTimersHook
	}()

	rt := NewRuntime()
	defer rt.Close()

	runtimeBootstrapStdOSHook = func(ctx *Context) bool { return false }
	require.Nil(t, rt.NewContextWithOptions(DefaultBootstrap()))
	runtimeBootstrapStdOSHook = nil

	runtimeBootstrapTimersHook = func(ctx *Context) bool { return false }
	require.Nil(t, rt.NewContextWithOptions(DefaultBootstrap()))
	runtimeBootstrapTimersHook = nil

	ctx := rt.NewContextWithOptions(DefaultBootstrap())
	require.NotNil(t, ctx)
	ctx.Close()
}

func TestRuntimeContextBootstrapStressContract(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	for i := 0; i < 64; i++ {
		ctx := rt.NewBareContext()
		require.NotNil(t, ctx)

		if i%2 == 0 {
			require.True(t, BootstrapStdOS(ctx))
			require.True(t, BootstrapTimers(ctx))
		}

		result := ctx.Eval(`1 + 1`)
		require.NotNil(t, result)
		require.False(t, result.IsException())
		require.EqualValues(t, 2, result.ToInt32())
		result.Free()

		ctx.Close()
		require.False(t, ctx.Schedule(func(*Context) {}))
	}
}

func TestRuntimeNewContextWithOptionsInitFailureHook(t *testing.T) {
	restore := forceRuntimeInitFailureForTest(true)
	defer restore()

	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContextWithOptions(DefaultBootstrap())
	require.Nil(t, ctx)

	ctx = rt.NewContextWithOptions(NoBootstrap())
	require.NotNil(t, ctx)
	ctx.Close()
}

func TestForceRuntimeEvalFailureHookDisable(t *testing.T) {
	restore := forceRuntimeEvalFailureForTest(true)
	defer restore()

	restoreDisable := forceRuntimeEvalFailureForTest(false)
	defer restoreDisable()

	require.Nil(t, runtimeEvalFunctionHook)
}

func TestInitializeContextGlobalsFailurePaths(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	defer ctx.Close()

	t.Run("CompileException", func(t *testing.T) {
		require.False(t, initializeContextGlobals(ctx.ref, "function {", "compile-fail.js"))
	})

	t.Run("EvalException", func(t *testing.T) {
		restore := forceRuntimeEvalFailureForTest(true)
		defer restore()

		require.False(t, initializeContextGlobals(ctx.ref, "globalThis.__evalProbe = 1", "eval-fail.js"))
	})

	t.Run("AwaitException", func(t *testing.T) {
		require.False(t, initializeContextGlobals(ctx.ref, "await Promise.reject(new Error('await fail'))", "await-fail.js"))
	})

	t.Run("Success", func(t *testing.T) {
		require.True(t, initializeContextGlobals(ctx.ref, "globalThis.__initProbe = 1", "success.js"))
	})

	t.Run("HookSuccess", func(t *testing.T) {
		restore := forceRuntimeInitSuccessForTest(true)
		defer restore()

		require.True(t, initializeContextGlobals(ctx.ref, "", "hook-success.js"))
	})

	t.Run("HookSuccessDisable", func(t *testing.T) {
		restore := forceRuntimeInitSuccessForTest(false)
		defer restore()

		require.True(t, initializeContextGlobals(ctx.ref, "globalThis.__initProbeDisabled = 1", "hook-success-disable.js"))
	})
}

func TestRuntimeOwnerCheckHooksAndStrictThreadAffinity(t *testing.T) {
	oldGIDHook := ownerCheckCurrentGoroutineID
	oldThreadHook := ownerCheckCurrentThreadID
	defer func() {
		ownerCheckCurrentGoroutineID = oldGIDHook
		ownerCheckCurrentThreadID = oldThreadHook
	}()

	var gid atomic.Uint64
	var tid atomic.Uint64
	gid.Store(11)
	tid.Store(101)

	ownerCheckCurrentGoroutineID = func() uint64 { return gid.Load() }
	ownerCheckCurrentThreadID = func() uint64 { return tid.Load() }

	var nilRuntime *Runtime
	require.False(t, nilRuntime.ensureOwnerAccess())

	rt := NewRuntime(WithStrictOSThread(true))
	require.NotNil(t, rt)
	defer rt.Close()

	require.True(t, rt.ensureOwnerAccess())
	require.Equal(t, uint64(11), rt.ownerGoroutineID.Load())
	require.Equal(t, uint64(101), rt.ownerThreadID.Load())
	require.NotZero(t, currentGoroutineID())
	require.NotZero(t, currentThreadID())

	gid.Store(0)
	require.False(t, rt.ensureOwnerAccess())
	gid.Store(11)
	tid.Store(0)
	require.False(t, rt.ensureOwnerAccess())
	tid.Store(101)

	gid.Store(12)
	require.False(t, rt.ensureOwnerAccess())

	gid.Store(11)
	tid.Store(102)
	require.False(t, rt.ensureOwnerAccess())

	tid.Store(101)
	require.True(t, rt.ensureOwnerAccess())

	rtNoCheck := NewRuntime()
	require.True(t, rtNoCheck.ensureOwnerAccess())
	rtNoCheck.Close()
}

func TestRuntimeOwnerGoroutineCheckOption(t *testing.T) {
	oldGIDHook := ownerCheckCurrentGoroutineID
	oldThreadHook := ownerCheckCurrentThreadID
	defer func() {
		ownerCheckCurrentGoroutineID = oldGIDHook
		ownerCheckCurrentThreadID = oldThreadHook
	}()

	var gid atomic.Uint64
	var tid atomic.Uint64
	gid.Store(31)
	tid.Store(1)

	ownerCheckCurrentGoroutineID = func() uint64 { return gid.Load() }
	ownerCheckCurrentThreadID = func() uint64 { return tid.Load() }

	defaultRT := NewRuntime()
	require.NotNil(t, defaultRT)
	require.True(t, defaultRT.options.ownerGoroutineCheck)
	require.True(t, defaultRT.ensureOwnerAccess())
	gid.Store(32)
	require.False(t, defaultRT.ensureOwnerAccess())
	gid.Store(31)
	defaultRT.Close()

	unsafeRT := NewRuntime(WithOwnerGoroutineCheck(false))
	require.NotNil(t, unsafeRT)
	require.False(t, unsafeRT.options.ownerGoroutineCheck)
	require.True(t, unsafeRT.ensureOwnerAccess())
	gid.Store(33)
	require.True(t, unsafeRT.ensureOwnerAccess())
	ctx := unsafeRT.NewContext()
	require.NotNil(t, ctx)
	ctx.Close()
	unsafeRT.Close()

	unsafeStrictRT := NewRuntime(WithOwnerGoroutineCheck(false), WithStrictOSThread(true))
	require.NotNil(t, unsafeStrictRT)
	require.True(t, unsafeStrictRT.ensureOwnerAccess())
	tid.Store(2)
	require.False(t, unsafeStrictRT.ensureOwnerAccess())
	tid.Store(1)
	require.True(t, unsafeStrictRT.ensureOwnerAccess())
	unsafeStrictRT.Close()
}

func TestCurrentGoroutineIDFailClosedParser(t *testing.T) {
	oldStack := goroutineStack
	defer func() {
		goroutineStack = oldStack
	}()

	goroutineStack = func(buf []byte, all bool) int {
		const s = "goroutine "
		copy(buf, s)
		return len(s)
	}
	require.Zero(t, currentGoroutineID())

	goroutineStack = func(buf []byte, all bool) int {
		const s = "goroutine x [running]:\n"
		copy(buf, s)
		return len(s)
	}
	require.Zero(t, currentGoroutineID())

	goroutineStack = func(buf []byte, all bool) int {
		const s = "goroutine 424242 [running]:\n"
		copy(buf, s)
		return len(s)
	}
	require.Equal(t, uint64(424242), currentGoroutineID())
}

func TestRuntimeOwnerCheckGatesRuntimeContextAndValuePaths(t *testing.T) {
	oldGIDHook := ownerCheckCurrentGoroutineID
	oldThreadHook := ownerCheckCurrentThreadID
	defer func() {
		ownerCheckCurrentGoroutineID = oldGIDHook
		ownerCheckCurrentThreadID = oldThreadHook
	}()

	var gid atomic.Uint64
	gid.Store(21)
	ownerCheckCurrentGoroutineID = func() uint64 { return gid.Load() }
	ownerCheckCurrentThreadID = func() uint64 { return 1 }

	rt := NewRuntime()
	require.NotNil(t, rt)
	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	defer ctx.Close()
	defer rt.Close()

	obj := ctx.Eval(`({ x: 1, inc: function(){ return this.x + 1; } })`)
	require.NotNil(t, obj)
	require.False(t, obj.IsException())
	defer obj.Free()

	throwVal := ctx.ThrowError(errors.New("owner-check-path"))
	require.NotNil(t, throwVal)
	require.True(t, throwVal.IsException())
	throwVal.Free()

	incFn := obj.Get("inc")
	require.NotNil(t, incFn)
	defer incFn.Free()

	ctor := ctx.Eval(`(function C(){ this.v = 1; })`)
	require.NotNil(t, ctor)
	require.False(t, ctor.IsException())
	defer ctor.Free()

	val := ctx.NewInt32(9)
	require.NotNil(t, val)
	defer val.Free()
	val1 := ctx.NewInt32(1)
	defer val1.Free()
	val2 := ctx.NewInt32(2)
	defer val2.Free()

	adderObj := ctx.Eval(`({ add: function(a, b){ return a + b; } })`)
	require.NotNil(t, adderObj)
	require.False(t, adderObj.IsException())
	defer adderObj.Free()

	adderFn := ctx.Eval(`(function(a, b){ return a + b; })`)
	require.NotNil(t, adderFn)
	require.False(t, adderFn.IsException())
	defer adderFn.Free()
	thisVal := ctx.NewUndefined()
	defer thisVal.Free()

	ctorWithArg := ctx.Eval(`(function D(v){ this.v = v; })`)
	require.NotNil(t, ctorWithArg)
	require.False(t, ctorWithArg.IsException())
	defer ctorWithArg.Free()

	ctxOther := rt.NewContext()
	require.NotNil(t, ctxOther)
	defer ctxOther.Close()
	otherVal := ctxOther.NewInt32(99)
	defer otherVal.Free()

	gid.Store(22)

	require.Nil(t, rt.NewContext())
	rt.RunGC()
	rt.SetMemoryLimit(1024)
	rt.SetGCThreshold(2048)
	rt.SetMaxStackSize(4096)
	rt.SetCanBlock(true)
	rt.SetStripInfo(1)
	rt.SetModuleImport(true)
	rt.SetExecuteTimeout(1)
	rt.SetInterruptHandler(func() int { return 1 })
	rt.ClearInterruptHandler()

	require.Nil(t, ctx.Eval("1 + 1"))
	_, err := ctx.Compile("1 + 1")
	require.ErrorIs(t, err, errOwnerAccessDenied)
	require.Nil(t, ctx.LoadModuleBytecode([]byte{1, 2, 3}))
	require.Nil(t, ctx.EvalBytecode([]byte{1, 2, 3}))
	require.Nil(t, ctx.Globals())
	require.Nil(t, ctx.ThrowSyntaxError("x"))
	require.Nil(t, ctx.ThrowTypeError("x"))
	require.Nil(t, ctx.ThrowReferenceError("x"))
	require.Nil(t, ctx.ThrowRangeError("x"))
	require.Nil(t, ctx.ThrowInternalError("x"))
	require.Nil(t, ctx.ThrowError(errors.New("x")))
	require.Nil(t, ctx.Throw(val))
	require.Nil(t, ctx.NewPromise(func(resolve, reject func(*Value)) {}))
	require.Nil(t, ctx.Await(nil))
	require.False(t, ctx.HasException())
	require.Nil(t, ctx.Exception())
	require.NotPanics(t, func() { ctx.Loop() })

	obj.Set("x", val)
	obj.SetIdx(0, val)
	require.Nil(t, obj.Get("x"))
	require.Nil(t, obj.GetIdx(0))
	require.Nil(t, obj.Call("inc"))
	require.Nil(t, adderObj.Call("add", val1, val2))
	require.Nil(t, incFn.Execute(obj))
	require.Nil(t, adderFn.Execute(thisVal, val1, val2))
	require.Nil(t, adderFn.Execute(thisVal, otherVal))
	require.Nil(t, ctor.CallConstructor())
	require.Nil(t, ctorWithArg.CallConstructor(val1))
	_, err = obj.GetGoObject()
	require.ErrorIs(t, err, errOwnerAccessDenied)

	gid.Store(21)
	rtClose := NewRuntime()
	ctxClose := rtClose.NewContext()
	require.NotNil(t, ctxClose)
	gid.Store(22)
	rtClose.Close()
	require.NotNil(t, rtClose.ref)
	gid.Store(21)
	ctxClose.Close()
	rtClose.Close()
	require.Nil(t, rtClose.ref)

	gid.Store(21)

	extraCtx := rt.NewContext()
	require.NotNil(t, extraCtx)
	extraCtx.Close()
	result := ctx.Eval("1 + 1")
	require.NotNil(t, result)
	require.False(t, result.IsException())
	require.EqualValues(t, 2, result.ToInt32())
	result.Free()

	next := obj.Call("inc")
	require.NotNil(t, next)
	require.False(t, next.IsException())
	require.EqualValues(t, 2, next.ToInt32())
	next.Free()

	emptyNameCall := obj.Call("")
	require.NotNil(t, emptyNameCall)
	emptyNameCall.Free()

	addResult := adderObj.Call("add", val1, val2)
	require.NotNil(t, addResult)
	require.False(t, addResult.IsException())
	require.EqualValues(t, 3, addResult.ToInt32())
	addResult.Free()
	require.Nil(t, adderObj.Call("add", otherVal))

	execResult := adderFn.Execute(thisVal, val1, val2)
	require.NotNil(t, execResult)
	require.False(t, execResult.IsException())
	require.EqualValues(t, 3, execResult.ToInt32())
	execResult.Free()
	require.Nil(t, adderFn.Execute(thisVal, otherVal))
	require.Nil(t, adderFn.Execute(otherVal, val1))

	execZeroArg := incFn.Execute(obj)
	require.NotNil(t, execZeroArg)
	require.False(t, execZeroArg.IsException())
	require.EqualValues(t, 2, execZeroArg.ToInt32())
	execZeroArg.Free()

	instanceNoArg := ctor.CallConstructor()
	require.NotNil(t, instanceNoArg)
	instanceNoArg.Free()

	instance := ctorWithArg.CallConstructor(val1)
	require.NotNil(t, instance)
	require.False(t, instance.IsException())
	require.Nil(t, ctorWithArg.CallConstructor(otherVal))
	got := instance.Get("v")
	require.NotNil(t, got)
	require.EqualValues(t, 1, got.ToInt32())
	got.Free()
	instance.Free()
}
