package quickjs

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type clearRaceFinalizable struct {
	count *atomic.Int64
}

func (c *clearRaceFinalizable) Finalize() {
	if c == nil || c.count == nil {
		return
	}
	c.count.Add(1)
}

func newBoundaryContext(t *testing.T) (*Runtime, *Context) {
	t.Helper()
	rt := NewRuntime()
	ctx := rt.NewContext()
	require.NotNil(t, ctx)
	t.Cleanup(func() {
		ctx.Close()
		rt.Close()
	})
	return rt, ctx
}

func buildAndRegisterContractClass(t *testing.T, ctx *Context, className string, withInstanceProperty bool) {
	t.Helper()
	builder := NewClassBuilder(className).
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &Point{X: 1, Y: 2}, nil
		})

	if withInstanceProperty {
		builder = builder.Property("x", ctx.NewInt32(1))
	}

	constructor, _ := builder.Build(ctx)
	require.False(t, constructor.IsException())
	ctx.Globals().Set(className, constructor)
}

func findClassBuilderHandleByName(t *testing.T, ctx *Context, className string) (int32, *ClassBuilder) {
	t.Helper()
	id, handle := findHandleByPredicate(t, ctx.handleStore, func(v interface{}) bool {
		b, ok := v.(*ClassBuilder)
		return ok && b.name == className
	})
	builder, ok := handle.Value().(*ClassBuilder)
	require.True(t, ok)

	return id, builder
}

func runEvalAndMessage(t *testing.T, ctx *Context, code string) string {
	t.Helper()
	result := ctx.Eval(code)
	defer result.Free()
	if result.IsException() {
		err := ctx.Exception()
		require.NotNil(t, err)
		return err.Error()
	}
	return result.ToString()
}

func importAndRequireMessageContains(t *testing.T, ctx *Context, moduleName string, expected string) {
	t.Helper()
	result := ctx.Eval("import('"+moduleName+"')", EvalAwait(true))
	defer result.Free()
	require.True(t, result.IsException())
	require.Contains(t, ctx.Exception().Error(), expected)
}

func TestBridgeBoundaryContracts(t *testing.T) {
	t.Run("FunctionNilReturnToUndefined", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return nil
		})
		require.NotNil(t, fn)
		ctx.Globals().Set("contractNilFn", fn)

		result := ctx.Eval(`typeof contractNilFn()`)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "undefined", result.ToString())
	})

	t.Run("FunctionContextMissing", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewString("ok")
		})
		ctx.Globals().Set("contractCtxFn", fn)

		unregisterContext(ctx.ref)
		result := ctx.Eval(`
            try {
                contractCtxFn();
            } catch (e) {
                e.toString();
            }
        `)
		registerContext(ctx.ref, ctx)
		defer result.Free()
		if result.IsException() {
			require.Contains(t, ctx.Exception().Error(), "Context not found")
		} else {
			require.Contains(t, result.ToString(), "Context not found")
		}
	})

	t.Run("FunctionContextMappingCorruptedType", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewString("ok")
		})
		ctx.Globals().Set("contractCorruptCtxFn", fn)

		contextMapping.Store(ctx.ref, "not-a-context")
		result := ctx.Eval(`
            try {
                contractCorruptCtxFn();
            } catch (e) {
                e.toString();
            }
        `)
		registerContext(ctx.ref, ctx)
		defer result.Free()

		if result.IsException() {
			require.Contains(t, ctx.Exception().Error(), "Context not found")
		} else {
			require.Contains(t, result.ToString(), "Context not found")
		}
	})

	t.Run("FunctionHandleStoreUnavailable", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewString("ok")
		})
		ctx.Globals().Set("contractStoreFn", fn)

		originalStore := ctx.handleStore
		ctx.handleStore = nil
		msg := runEvalAndMessage(t, ctx, `
            try {
                contractStoreFn();
            } catch (e) {
                e.toString();
            }
        `)
		ctx.handleStore = originalStore

		require.Contains(t, msg, "HandleStore unavailable")
	})

	t.Run("ModuleInvalidExportValueNil", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		module := NewModuleBuilder("contract-nil-export").
			Export("value", ctx.NewString("test"))
		require.NoError(t, module.Build(ctx))
		require.GreaterOrEqual(t, len(module.exports), 1)

		module.exports[0].Value = nil
		importAndRequireMessageContains(t, ctx, "contract-nil-export", "invalid module export value")
	})

	t.Run("ModuleInvalidExportValueForeignContext", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		module := NewModuleBuilder("contract-foreign-export").
			Export("value", ctx.NewString("test"))
		require.NoError(t, module.Build(ctx))
		require.GreaterOrEqual(t, len(module.exports), 1)

		rt2 := NewRuntime()
		defer rt2.Close()
		ctx2 := rt2.NewContext()
		defer ctx2.Close()

		foreign := ctx2.NewString("foreign")
		defer foreign.Free()
		module.exports[0].Value = foreign

		importAndRequireMessageContains(t, ctx, "contract-foreign-export", "invalid module export value")
	})

	t.Run("ModuleInitPanicConvertedToInternalError", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		setModuleInitPanicHookForTest(func(ctx *Context, builder *ModuleBuilder) {
			panic("boom-module-init")
		})
		t.Cleanup(func() {
			setModuleInitPanicHookForTest(nil)
		})

		module := NewModuleBuilder("contract-module-init-panic").
			Export("value", ctx.NewString("ok"))
		require.NoError(t, module.Build(ctx))

		importAndRequireMessageContains(t, ctx, "contract-module-init-panic", "panic in Go callback: boom-module-init")
	})

	t.Run("ConstructorNilInBuilder", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)
		buildAndRegisterContractClass(t, ctx, "ContractCtorNilClass", false)

		constructorID, _ := findClassBuilderHandleByName(t, ctx, "ContractCtorNilClass")

		corrupted := &ClassBuilder{name: "CorruptedClass", constructor: nil}
		restore := replaceHandleWithValueForTest(t, ctx.handleStore, constructorID, corrupted)
		defer restore()

		msg := runEvalAndMessage(t, ctx, `
            try {
                new ContractCtorNilClass();
            } catch (e) {
                e.toString();
            }
        `)
		require.Contains(t, msg, "Constructor function is nil")
	})

	t.Run("ConstructorInvalidInstanceProperty", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)
		buildAndRegisterContractClass(t, ctx, "ContractBadPropClass", true)

		constructorID, builder := findClassBuilderHandleByName(t, ctx, "ContractBadPropClass")
		require.GreaterOrEqual(t, len(builder.properties), 1)

		mutated := *builder
		mutated.properties = append([]PropertyEntry(nil), builder.properties...)
		mutated.properties[0].Value = nil
		restore := replaceHandleWithValueForTest(t, ctx.handleStore, constructorID, &mutated)
		defer restore()

		msg := runEvalAndMessage(t, ctx, `
            try {
                new ContractBadPropClass();
            } catch (e) {
                e.toString();
            }
        `)
		require.Contains(t, msg, "Invalid instance property value")
	})

	t.Run("ClassFinalizerMissingMappingsSafeExit", func(t *testing.T) {
		rt, ctx := newBoundaryContext(t)
		buildAndRegisterContractClass(t, ctx, "FinalizerContractClass", false)

		obj := ctx.Eval(`new FinalizerContractClass()`)
		require.False(t, obj.IsException())
		obj.Free()

		unregisterContext(ctx.ref)
		unregisterRuntime(rt.ref)
		registerContext(ctx.ref, &Context{})

		require.NotPanics(t, func() {
			rt.RunGC()
			runtime.GC()
		})

		registerRuntime(rt.ref, rt)
		registerContext(ctx.ref, ctx)
	})

	t.Run("RuntimeMappingCorruptedTypeSafeExit", func(t *testing.T) {
		rt, ctx := newBoundaryContext(t)

		rt.SetInterruptHandler(func() int { return 1 })
		runtimeMapping.Store(rt.ref, "not-a-runtime")

		require.NotPanics(t, func() {
			result := ctx.Eval(`
                let sum = 0;
                for (let i = 0; i < 50000; i++) {
                    sum += i;
                }
                sum;
            `)
			defer result.Free()
			require.False(t, result.IsException())
		})

		registerRuntime(rt.ref, rt)
	})

	t.Run("FinalizerConcurrentHandleStoreClearStress", func(t *testing.T) {
		rt, ctx := newBoundaryContext(t)

		var finalized atomic.Int64
		constructor, _ := NewClassBuilder("FinalizerClearRaceClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &clearRaceFinalizable{count: &finalized}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())
		ctx.Globals().Set("FinalizerClearRaceClass", constructor)

		for i := 0; i < 300; i++ {
			obj := ctx.Eval(`new FinalizerClearRaceClass()`)
			require.False(t, obj.IsException())
			obj.Free()
		}

		var wg sync.WaitGroup
		wg.Add(1)

		require.NotPanics(t, func() {
			go func() {
				defer wg.Done()
				for i := 0; i < 80; i++ {
					ctx.handleStore.Clear()
				}
			}()

			for i := 0; i < 80; i++ {
				rt.RunGC()
				runtime.GC()
			}

			wg.Wait()
		})

		ctx.handleStore.Clear()
		require.Equal(t, 0, ctx.handleStore.Count())
		require.GreaterOrEqual(t, finalized.Load(), int64(0))
	})

	t.Run("MappingCorruptionWithFinalizerConcurrencyStress", func(t *testing.T) {
		rt, ctx := newBoundaryContext(t)

		var finalized atomic.Int64
		constructor, _ := NewClassBuilder("MappingRaceClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return &clearRaceFinalizable{count: &finalized}, nil
			}).
			Build(ctx)
		require.False(t, constructor.IsException())
		ctx.Globals().Set("MappingRaceClass", constructor)

		for i := 0; i < 200; i++ {
			obj := ctx.Eval(`new MappingRaceClass()`)
			require.False(t, obj.IsException())
			obj.Free()
		}

		var wg sync.WaitGroup
		wg.Add(1)

		require.NotPanics(t, func() {
			go func() {
				defer wg.Done()
				for i := 0; i < 60; i++ {
					contextMapping.Store(ctx.ref, "bad-context")
					runtimeMapping.Store(rt.ref, "bad-runtime")
					ctx.handleStore.Clear()
					registerContext(ctx.ref, ctx)
					registerRuntime(rt.ref, rt)
				}
			}()

			for i := 0; i < 60; i++ {
				rt.RunGC()
				runtime.GC()
			}

			wg.Wait()
		})

		registerContext(ctx.ref, ctx)
		registerRuntime(rt.ref, rt)
		ctx.handleStore.Clear()
		require.Equal(t, 0, ctx.handleStore.Count())
		require.GreaterOrEqual(t, finalized.Load(), int64(0))
	})

	t.Run("FunctionPanicConvertedToInternalError", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			panic("boom-function")
		})
		ctx.Globals().Set("panicFn", fn)

		msg := runEvalAndMessage(t, ctx, `
            try {
                panicFn();
            } catch (e) {
                e.toString();
            }
        `)
		require.Contains(t, msg, "panic in Go callback: boom-function")
	})

	t.Run("ClassConstructorPanicConvertedToInternalError", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		constructor, _ := NewClassBuilder("PanicCtorClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				panic("boom-constructor")
			}).
			Build(ctx)
		require.False(t, constructor.IsException())
		ctx.Globals().Set("PanicCtorClass", constructor)

		msg := runEvalAndMessage(t, ctx, `
            try {
                new PanicCtorClass();
            } catch (e) {
                e.toString();
            }
        `)
		require.Contains(t, msg, "panic in Go callback: boom-constructor")
	})

	t.Run("ClassMethodPanicConvertedToInternalError", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		constructor, _ := NewClassBuilder("PanicMethodClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return struct{}{}, nil
			}).
			Method("explode", func(ctx *Context, this *Value, args []*Value) *Value {
				panic("boom-method")
			}).
			Build(ctx)
		require.False(t, constructor.IsException())
		ctx.Globals().Set("PanicMethodClass", constructor)

		msg := runEvalAndMessage(t, ctx, `
            try {
                new PanicMethodClass().explode();
            } catch (e) {
                e.toString();
            }
        `)
		require.Contains(t, msg, "panic in Go callback: boom-method")
	})

	t.Run("ClassGetterPanicConvertedToInternalError", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		constructor, _ := NewClassBuilder("PanicGetterClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return struct{}{}, nil
			}).
			Accessor("value", func(ctx *Context, this *Value) *Value {
				panic("boom-getter")
			}, nil).
			Build(ctx)
		require.False(t, constructor.IsException())
		ctx.Globals().Set("PanicGetterClass", constructor)

		msg := runEvalAndMessage(t, ctx, `
            try {
                new PanicGetterClass().value;
            } catch (e) {
                e.toString();
            }
        `)
		require.Contains(t, msg, "panic in Go callback: boom-getter")
	})

	t.Run("ClassSetterPanicConvertedToInternalError", func(t *testing.T) {
		_, ctx := newBoundaryContext(t)

		constructor, _ := NewClassBuilder("PanicSetterClass").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				return struct{}{}, nil
			}).
			Accessor("value", nil, func(ctx *Context, this *Value, value *Value) *Value {
				panic("boom-setter")
			}).
			Build(ctx)
		require.False(t, constructor.IsException())
		ctx.Globals().Set("PanicSetterClass", constructor)

		msg := runEvalAndMessage(t, ctx, `
            try {
                const obj = new PanicSetterClass();
                obj.value = 123;
            } catch (e) {
                e.toString();
            }
        `)
		require.Contains(t, msg, "panic in Go callback: boom-setter")
	})

	t.Run("InterruptHandlerPanicNoCrash", func(t *testing.T) {
		rt, ctx := newBoundaryContext(t)

		rt.SetInterruptHandler(func() int {
			panic("boom-interrupt")
		})

		result := ctx.Eval(`
            let sum = 0;
            for (let i = 0; i < 10000; i++) {
                sum += i;
            }
            sum;
        `)
		defer result.Free()
		require.False(t, result.IsException())
		require.Greater(t, result.ToInt32(), int32(0))
	})
}

func TestCallbackPanicSoak(t *testing.T) {
	if os.Getenv("QUICKJS_SOAK") != "1" {
		t.Skip("set QUICKJS_SOAK=1 to enable long-running panic soak")
	}

	const workers = 12
	const rounds = 60

	for i := 0; i < workers; i++ {
		i := i
		t.Run(fmt.Sprintf("panic-soak-worker-%d", i), func(t *testing.T) {
			t.Parallel()

			rt := NewRuntime()
			defer rt.Close()

			ctx := rt.NewContext()
			require.NotNil(t, ctx)
			defer ctx.Close()

			panicFnName := fmt.Sprintf("panicFn%d", i)
			panicClassName := fmt.Sprintf("PanicSoakClass%d", i)

			fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
				panic("soak-function")
			})
			ctx.Globals().Set(panicFnName, fn)

			constructor, _ := NewClassBuilder(panicClassName).
				Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
					return struct{}{}, nil
				}).
				Method("explode", func(ctx *Context, this *Value, args []*Value) *Value {
					panic("soak-method")
				}).
				Build(ctx)
			require.False(t, constructor.IsException())
			ctx.Globals().Set(panicClassName, constructor)

			for r := 0; r < rounds; r++ {
				resFn := ctx.Eval(fmt.Sprintf(`
                    try {
                        %s();
                    } catch (e) {
                        e.toString();
                    }
                `, panicFnName))
				require.False(t, resFn.IsException())
				require.Contains(t, resFn.ToString(), "panic in Go callback: soak-function")
				resFn.Free()

				resMethod := ctx.Eval(fmt.Sprintf(`
                    try {
                        new %s().explode();
                    } catch (e) {
                        e.toString();
                    }
                `, panicClassName))
				require.False(t, resMethod.IsException())
				require.Contains(t, resMethod.ToString(), "panic in Go callback: soak-method")
				resMethod.Free()

				rt.RunGC()
				runtime.GC()
			}
		})
	}
}
