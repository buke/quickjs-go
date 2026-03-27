package quickjs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func moduleBuilderHandleCount(ctx *Context, moduleName string) int {
	count := 0
	ctx.handleStore.handles.Range(func(key, _ interface{}) bool {
		handleID := key.(int32)
		stored, ok := ctx.handleStore.Load(handleID)
		if !ok {
			return true
		}
		builder, ok := stored.(*ModuleBuilder)
		if ok && builder != nil && builder.name == moduleName {
			count++
		}
		return true
	})
	return count
}

func deleteModuleBuilderHandles(ctx *Context, moduleName string) int {
	deleted := 0
	ctx.handleStore.handles.Range(func(key, _ interface{}) bool {
		handleID := key.(int32)
		stored, ok := ctx.handleStore.Load(handleID)
		if !ok {
			return true
		}
		builder, ok := stored.(*ModuleBuilder)
		if ok && builder != nil && builder.name == moduleName {
			if ctx.handleStore.Delete(handleID) {
				deleted++
			}
		}
		return true
	})
	return deleted
}

func assertModuleBuilderAbsent(t *testing.T, ctx *Context, moduleName string, baseHandles int) {
	t.Helper()
	require.Equal(t, 0, moduleBuilderHandleCount(ctx, moduleName))
	require.Equal(t, baseHandles, ctx.handleStore.Count())
}

func assertModuleBuilderRetainedThenCleaned(t *testing.T, ctx *Context, moduleName string, retained int, baseHandles int) {
	t.Helper()
	require.Equal(t, retained, moduleBuilderHandleCount(ctx, moduleName))
	require.Equal(t, retained, deleteModuleBuilderHandles(ctx, moduleName))
	require.Equal(t, 0, moduleBuilderHandleCount(ctx, moduleName))
	require.Equal(t, baseHandles, ctx.handleStore.Count())
}
