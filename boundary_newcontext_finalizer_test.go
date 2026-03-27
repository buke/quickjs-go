package quickjs

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

type ownershipSentinel struct {
	label string
}

type panicOnFinalize struct{}

func (p *panicOnFinalize) Finalize() {
	panic("panic-on-finalize")
}

func buildOwnershipClass(t *testing.T, ctx *Context, className string, label string) {
	t.Helper()
	constructor, _ := NewClassBuilder(className).
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &ownershipSentinel{label: label}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())
	ctx.Globals().Set(className, constructor)
}

func TestFinalizerOwnershipUsesContextAwareOpaquePayload(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx1 := rt.NewContext()
	require.NotNil(t, ctx1)
	defer ctx1.Close()

	ctx2 := rt.NewContext()
	require.NotNil(t, ctx2)
	defer ctx2.Close()

	buildOwnershipClass(t, ctx1, "CtxOneClass", "one")
	buildOwnershipClass(t, ctx2, "CtxTwoClass", "two")

	obj2 := ctx2.Eval(`new CtxTwoClass()`)
	require.False(t, obj2.IsException())
	defer obj2.Free()

	ctxKey2, handleID2, ok := obj2.classOpaqueContextKeyAndHandleIDForTest()
	require.True(t, ok)
	require.Equal(t, ctx2.cContextKeyForTest(), ctxKey2)
	require.Greater(t, handleID2, int32(0))

	obj1 := ctx1.Eval(`new CtxOneClass()`)
	require.False(t, obj1.IsException())
	ctxKey1, handleID1, ok := obj1.classOpaqueContextKeyAndHandleIDForTest()
	require.True(t, ok)
	require.Equal(t, ctx1.cContextKeyForTest(), ctxKey1)
	require.Greater(t, handleID1, int32(0))
	require.NotEqual(t, ctxKey1, ctxKey2)
	obj1.Free()

	// Try to generate a colliding handle ID on another context; with context-aware
	// opaque payload, collecting it must not affect obj2 ownership resolution.
	collided := handleID1 == handleID2
	for i := 0; i < 64 && !collided; i++ {
		tmp := ctx1.Eval(`new CtxOneClass()`)
		require.False(t, tmp.IsException())
		_, tmpHandleID, ok := tmp.classOpaqueContextKeyAndHandleIDForTest()
		require.True(t, ok)
		collided = tmpHandleID == handleID2
		tmp.Free()
	}
	require.True(t, collided)

	for i := 0; i < 8; i++ {
		rt.RunGC()
		runtime.GC()
	}

	goObj, err := obj2.GetGoObject()
	require.NoError(t, err)
	sentinel, ok := goObj.(*ownershipSentinel)
	require.True(t, ok)
	require.Equal(t, "two", sentinel.label)
}

func TestRuntimeNewContextNilFromCFailClosed(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	setJSNewContextFailForTest(rt, true)
	t.Cleanup(func() {
		setJSNewContextFailForTest(rt, false)
	})

	ctx := rt.NewContext()
	require.Nil(t, ctx)

	setJSNewContextFailForTest(rt, false)
	ctx2 := rt.NewContext()
	require.NotNil(t, ctx2)
	ctx2.Close()
}

func TestClassOpaqueLifecycleStressWithRunGCAndContextClose(t *testing.T) {
	resetClassOpaqueCountersForTest()

	rt := NewRuntime()
	defer rt.Close()

	for cycle := 0; cycle < 30; cycle++ {
		ctx := rt.NewContext()
		require.NotNil(t, ctx)

		className := fmt.Sprintf("OpaqueStressClass%d", cycle)
		buildOwnershipClass(t, ctx, className, "stress")

		for i := 0; i < 240; i++ {
			obj := ctx.Eval("new " + className + "()")
			require.False(t, obj.IsException())
			obj.Free()
		}

		// Keep one object alive only in JS space to exercise close-time finalization
		// without leaking Go-side Value wrappers.
		sticky := ctx.Eval("globalThis.__sticky = new " + className + "(); 1")
		require.False(t, sticky.IsException())
		sticky.Free()

		rt.RunGC()
		runtime.GC()

		require.NotPanics(t, func() {
			ctx.Close()
		})

		rt.RunGC()
		runtime.GC()
	}

	for i := 0; i < 12; i++ {
		rt.RunGC()
		runtime.GC()
	}

	alloc := currentClassOpaqueAllocationCount()
	freed := currentClassOpaqueFreeCount()
	outstanding := currentClassOpaqueOutstandingCount()
	require.Equal(t, alloc, freed)
	require.Equal(t, 0, outstanding)
}

func TestFinalizerFailClosedObservabilityCounters(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	setFinalizerObservabilityForTest(rt, true)
	resetFinalizerObservabilityForTest(rt)
	t.Cleanup(func() {
		setFinalizerObservabilityForTest(rt, false)
		resetFinalizerObservabilityForTest(rt)
	})

	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	defer ctx.Close()

	buildOwnershipClass(t, ctx, "ObserveClass", "observe")

	objs := make([]*Value, 0, 180)
	for i := 0; i < 180; i++ {
		obj := ctx.Eval(`new ObserveClass()`)
		require.False(t, obj.IsException())
		objs = append(objs, obj)
	}

	unregisterContext(ctx.ref)
	for _, obj := range objs {
		obj.Free()
	}
	for i := 0; i < 10; i++ {
		rt.RunGC()
		runtime.GC()
	}
	registerContext(ctx.ref, ctx)

	snapshot := snapshotFinalizerObservabilityForTest(rt)
	require.True(t, snapshot.Enabled)
	require.Greater(t, snapshot.ContextNotFound, uint64(0))
}

func TestRuntimeScopedInjectionAndObservabilityParallel(t *testing.T) {
	const workers = 8

	for i := 0; i < workers; i++ {
		i := i
		t.Run(fmt.Sprintf("worker-%d", i), func(t *testing.T) {
			t.Parallel()

			rt := NewRuntime()
			defer rt.Close()

			setFinalizerObservabilityForTest(rt, true)
			resetFinalizerObservabilityForTest(rt)

			ctx := rt.NewContext()
			require.NotNil(t, ctx)
			defer ctx.Close()

			if i%2 == 0 {
				setJSNewContextFailForTest(rt, true)
				blocked := rt.NewContext()
				require.Nil(t, blocked)
				setJSNewContextFailForTest(rt, false)
			}

			opened := rt.NewContext()
			require.NotNil(t, opened)
			opened.Close()

			className := fmt.Sprintf("ParallelObserveClass%d", i)
			buildOwnershipClass(t, ctx, className, "observe")

			obj := ctx.Eval("new " + className + "()")
			require.False(t, obj.IsException())

			unregisterContext(ctx.ref)
			obj.Free()
			for j := 0; j < 8; j++ {
				rt.RunGC()
				runtime.GC()
			}
			registerContext(ctx.ref, ctx)

			snapshot := snapshotFinalizerObservabilityForTest(rt)
			require.True(t, snapshot.Enabled)
			require.Greater(t, snapshot.ContextNotFound, uint64(0))

			// Runtime-scoped buckets must remain isolated.
			rt2 := NewRuntime()
			defer rt2.Close()
			other := snapshotFinalizerObservabilityForTest(rt2)
			require.Equal(t, uint64(0), other.ContextNotFound)
			require.Equal(t, uint64(0), other.RuntimeMismatch)
			require.Equal(t, uint64(0), other.HandleMissing)
		})
	}
}

func TestFinalizerObservabilityRuntimeMismatchBranch(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	rtOther := NewRuntime()
	defer rtOther.Close()

	setFinalizerObservabilityForTest(rt, true)
	resetFinalizerObservabilityForTest(rt)

	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	defer ctx.Close()

	buildOwnershipClass(t, ctx, "ObserveMismatchClass", "mismatch")

	obj := ctx.Eval(`new ObserveMismatchClass()`)
	require.False(t, obj.IsException())

	mutated := &Context{runtime: rtOther, handleStore: ctx.handleStore}
	contextMapping.Store(ctx.ref, mutated)

	obj.Free()
	for i := 0; i < 10; i++ {
		rt.RunGC()
		runtime.GC()
	}

	registerContext(ctx.ref, ctx)

	snapshot := snapshotFinalizerObservabilityForTest(rt)
	require.Greater(t, snapshot.RuntimeMismatch, uint64(0))

	// Restore normal cleanup path for any handles skipped by mismatch early-return.
	for i := 0; i < 10; i++ {
		rt.RunGC()
		runtime.GC()
	}
}

func TestFinalizerObservabilityContextStateInvalidBranch(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	setFinalizerObservabilityForTest(rt, true)
	resetFinalizerObservabilityForTest(rt)

	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	defer ctx.Close()

	buildOwnershipClass(t, ctx, "ObserveInvalidStateClass", "invalid-state")

	obj := ctx.Eval(`new ObserveInvalidStateClass()`)
	require.False(t, obj.IsException())

	contextMapping.Store(ctx.ref, &Context{handleStore: ctx.handleStore})

	obj.Free()
	for i := 0; i < 10; i++ {
		rt.RunGC()
		runtime.GC()
	}

	registerContext(ctx.ref, ctx)

	snapshot := snapshotFinalizerObservabilityForTest(rt)
	require.Greater(t, snapshot.ContextStateInvalid, uint64(0))

	for i := 0; i < 10; i++ {
		rt.RunGC()
		runtime.GC()
	}
}

func TestFinalizerObservabilityHandleMissingBranch(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	setFinalizerObservabilityForTest(rt, true)
	resetFinalizerObservabilityForTest(rt)

	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	defer ctx.Close()

	buildOwnershipClass(t, ctx, "ObserveHandleMissingClass", "missing")

	obj := ctx.Eval(`new ObserveHandleMissingClass()`)
	require.False(t, obj.IsException())

	_, handleID, ok := obj.classOpaqueContextKeyAndHandleIDForTest()
	require.True(t, ok)
	require.True(t, ctx.handleStore.Delete(handleID))

	obj.Free()
	for i := 0; i < 10; i++ {
		rt.RunGC()
		runtime.GC()
	}

	snapshot := snapshotFinalizerObservabilityForTest(rt)
	require.Greater(t, snapshot.HandleMissing, uint64(0))
}

func TestFinalizerObservabilityExactSingleEventCounts(t *testing.T) {
	t.Run("ContextNotFoundExact", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setFinalizerObservabilityForTest(rt, true)
		resetFinalizerObservabilityForTest(rt)

		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		defer ctx.Close()

		buildOwnershipClass(t, ctx, "ExactContextNotFoundClass", "observe")

		obj := ctx.Eval(`new ExactContextNotFoundClass()`)
		require.False(t, obj.IsException())

		unregisterContext(ctx.ref)
		obj.Free()
		for i := 0; i < 10; i++ {
			rt.RunGC()
			runtime.GC()
		}
		registerContext(ctx.ref, ctx)

		s := snapshotFinalizerObservabilityForTest(rt)
		require.Equal(t, uint64(1), s.ContextNotFound)
		require.Equal(t, uint64(0), s.RuntimeMismatch)
		require.Equal(t, uint64(0), s.HandleMissing)
	})

	t.Run("RuntimeMismatchExact", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		rtOther := NewRuntime()
		defer rtOther.Close()

		setFinalizerObservabilityForTest(rt, true)
		resetFinalizerObservabilityForTest(rt)

		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		defer ctx.Close()

		buildOwnershipClass(t, ctx, "ExactRuntimeMismatchClass", "observe")

		obj := ctx.Eval(`new ExactRuntimeMismatchClass()`)
		require.False(t, obj.IsException())

		mutated := &Context{runtime: rtOther, handleStore: ctx.handleStore}
		contextMapping.Store(ctx.ref, mutated)

		obj.Free()
		for i := 0; i < 10; i++ {
			rt.RunGC()
			runtime.GC()
		}

		registerContext(ctx.ref, ctx)

		s := snapshotFinalizerObservabilityForTest(rt)
		require.Equal(t, uint64(1), s.RuntimeMismatch)
		require.Equal(t, uint64(0), s.ContextNotFound)
		require.Equal(t, uint64(0), s.HandleMissing)
	})

	t.Run("HandleMissingExact", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setFinalizerObservabilityForTest(rt, true)
		resetFinalizerObservabilityForTest(rt)

		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		defer ctx.Close()

		buildOwnershipClass(t, ctx, "ExactHandleMissingClass", "observe")

		obj := ctx.Eval(`new ExactHandleMissingClass()`)
		require.False(t, obj.IsException())

		_, handleID, ok := obj.classOpaqueContextKeyAndHandleIDForTest()
		require.True(t, ok)
		require.True(t, ctx.handleStore.Delete(handleID))

		obj.Free()
		for i := 0; i < 10; i++ {
			rt.RunGC()
			runtime.GC()
		}

		s := snapshotFinalizerObservabilityForTest(rt)
		require.Equal(t, uint64(1), s.HandleMissing)
		require.Equal(t, uint64(0), s.ContextNotFound)
		require.Equal(t, uint64(0), s.RuntimeMismatch)
	})

	t.Run("OpaqueInvalidExact", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setFinalizerObservabilityForTest(rt, true)
		resetFinalizerObservabilityForTest(rt)

		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		defer ctx.Close()

		buildOwnershipClass(t, ctx, "ExactOpaqueInvalidClass", "observe")

		obj := ctx.Eval(`new ExactOpaqueInvalidClass()`)
		require.False(t, obj.IsException())
		require.True(t, obj.corruptClassOpaqueMagicForTest())

		obj.Free()
		for i := 0; i < 10; i++ {
			rt.RunGC()
			runtime.GC()
		}

		s := snapshotFinalizerObservabilityForTest(rt)
		require.Equal(t, uint64(1), s.OpaqueInvalid)
		require.Equal(t, uint64(0), s.ContextNotFound)
		require.Equal(t, uint64(0), s.RuntimeMismatch)
		require.Equal(t, uint64(0), s.HandleMissing)
	})

	t.Run("HandleInvalidExact", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setFinalizerObservabilityForTest(rt, true)
		resetFinalizerObservabilityForTest(rt)

		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		defer ctx.Close()

		buildOwnershipClass(t, ctx, "ExactHandleInvalidClass", "observe")

		obj := ctx.Eval(`new ExactHandleInvalidClass()`)
		require.False(t, obj.IsException())
		require.True(t, obj.setClassOpaqueHandleIDForTest(0))

		obj.Free()
		for i := 0; i < 10; i++ {
			rt.RunGC()
			runtime.GC()
		}

		s := snapshotFinalizerObservabilityForTest(rt)
		require.Equal(t, uint64(1), s.HandleInvalid)
		require.Equal(t, uint64(0), s.ContextNotFound)
		require.Equal(t, uint64(0), s.RuntimeMismatch)
		require.Equal(t, uint64(0), s.HandleMissing)
	})

	t.Run("ContextRefInvalidExact", func(t *testing.T) {
		rt := NewRuntime()
		defer rt.Close()

		setFinalizerObservabilityForTest(rt, true)
		resetFinalizerObservabilityForTest(rt)

		ctx := rt.NewContext()
		require.NotNil(t, ctx)
		defer ctx.Close()

		buildOwnershipClass(t, ctx, "ExactContextRefInvalidClass", "observe")

		obj := ctx.Eval(`new ExactContextRefInvalidClass()`)
		require.False(t, obj.IsException())
		require.True(t, obj.setClassOpaqueContextNullForTest())

		obj.Free()
		for i := 0; i < 10; i++ {
			rt.RunGC()
			runtime.GC()
		}

		s := snapshotFinalizerObservabilityForTest(rt)
		require.Equal(t, uint64(1), s.ContextRefInvalid)
		require.Equal(t, uint64(0), s.ContextNotFound)
		require.Equal(t, uint64(0), s.RuntimeMismatch)
		require.Equal(t, uint64(0), s.HandleMissing)
	})
}

func TestRuntimeFinalizerObservabilityExportHelpers(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	rt.SetFinalizerObservability(true)
	rt.ResetFinalizerObservability()

	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	defer ctx.Close()

	buildOwnershipClass(t, ctx, "ExportObserveClass", "observe")

	obj := ctx.Eval(`new ExportObserveClass()`)
	require.False(t, obj.IsException())

	unregisterContext(ctx.ref)
	obj.Free()
	for i := 0; i < 10; i++ {
		rt.RunGC()
		runtime.GC()
	}
	registerContext(ctx.ref, ctx)

	snapshot := rt.SnapshotAndResetFinalizerObservability()
	require.True(t, snapshot.Enabled)
	require.Equal(t, uint64(1), snapshot.ContextNotFound)

	afterReset := rt.SnapshotFinalizerObservability()
	require.Equal(t, uint64(0), afterReset.ContextNotFound)
	require.Equal(t, uint64(0), afterReset.RuntimeMismatch)
	require.Equal(t, uint64(0), afterReset.HandleMissing)
}

func TestFinalizerPanicRecoveryStillCleansHandle(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	setFinalizerObservabilityForTest(rt, true)
	resetFinalizerObservabilityForTest(rt)

	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	defer ctx.Close()

	constructor, _ := NewClassBuilder("PanicFinalizeClass").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &panicOnFinalize{}, nil
		}).
		Build(ctx)
	require.False(t, constructor.IsException())
	ctx.Globals().Set("PanicFinalizeClass", constructor)

	obj := ctx.Eval(`new PanicFinalizeClass()`)
	require.False(t, obj.IsException())
	obj.Free()

	for i := 0; i < 12; i++ {
		rt.RunGC()
		runtime.GC()
	}

	snapshot := snapshotFinalizerObservabilityForTest(rt)
	require.GreaterOrEqual(t, snapshot.Cleaned, uint64(1))
}

func TestRuntimeScopedInjectionAndObservabilitySoak(t *testing.T) {
	if os.Getenv("QUICKJS_SOAK") != "1" {
		t.Skip("set QUICKJS_SOAK=1 to enable long-running soak")
	}

	const workers = 24
	const rounds = 40

	for i := 0; i < workers; i++ {
		i := i
		t.Run(fmt.Sprintf("soak-worker-%d", i), func(t *testing.T) {
			t.Parallel()

			rt := NewRuntime()
			defer rt.Close()

			setFinalizerObservabilityForTest(rt, true)
			resetFinalizerObservabilityForTest(rt)

			ctx := rt.NewContext()
			require.NotNil(t, ctx)
			defer ctx.Close()

			className := fmt.Sprintf("SoakObserveClass%d", i)
			buildOwnershipClass(t, ctx, className, "observe")

			for r := 0; r < rounds; r++ {
				setJSNewContextFailForTest(rt, true)
				require.Nil(t, rt.NewContext())
				setJSNewContextFailForTest(rt, false)

				obj := ctx.Eval("new " + className + "()")
				require.False(t, obj.IsException())
				unregisterContext(ctx.ref)
				obj.Free()
				rt.RunGC()
				runtime.GC()
				registerContext(ctx.ref, ctx)
			}

			s := snapshotFinalizerObservabilityForTest(rt)
			require.GreaterOrEqual(t, s.ContextNotFound, uint64(rounds))
		})
	}
}
