package quickjs

import (
	"fmt"
	"runtime/cgo"
	"sync/atomic"
	"unsafe"
)

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

var moduleBuilderIDForceParseFailure atomic.Bool

// getContextAndModuleBuilder retrieves context and ModuleBuilder from module private value.
func getContextAndModuleBuilder(ctx *C.JSContext, m *C.JSModuleDef) (*Context, *ModuleBuilder, error) {
	privateValue := C.JS_GetModulePrivateValue(ctx, m)

	var builderID C.int32_t
	if moduleBuilderIDForceParseFailure.Load() || C.JS_ToInt32(ctx, &builderID, privateValue) < 0 {
		C.JS_FreeValue(ctx, privateValue)
		bestEffortCleanupModuleBuilderHandles(getContextFromJS(ctx))
		return nil, nil, fmt.Errorf("failed to parse module builder id from module private value")
	}
	C.JS_FreeValue(ctx, privateValue)

	goCtx, builderInterface, err := getContextAndObject(ctx, C.int(builderID), errFunctionNotFound)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get context and module builder: %v", err.message)
	}

	builder, ok := builderInterface.(*ModuleBuilder)
	if !ok || builder == nil {
		return nil, nil, fmt.Errorf("failed to get context and module builder: invalid module builder handle")
	}

	return goCtx, builder, nil
}

//export goModuleInitProxy
func goModuleInitProxy(ctx *C.JSContext, m *C.JSModuleDef) C.int {
	goCtx, builder, err := getContextAndModuleBuilder(ctx, m)
	if err != nil {
		return throwModuleError(ctx, err)
	}

	for _, export := range builder.exports {
		if export.Value == nil || !export.Value.hasValidContext() || export.Value.ctx != goCtx {
			return throwModuleError(ctx, fmt.Errorf("invalid module export value: %s", export.Name))
		}

		exportName := C.CString(export.Name)
		val := export.Value.ref
		rc := C.JS_SetModuleExport(ctx, m, exportName, val)
		export.Value.ref = C.JS_NewUndefined()
		C.free(unsafe.Pointer(exportName))
		if rc < 0 {
			return throwModuleError(ctx, fmt.Errorf("failed to set module export: %s", export.Name))
		}
	}

	privateValue := C.JS_GetModulePrivateValue(ctx, m)
	var builderID C.int32_t
	if C.JS_ToInt32(ctx, &builderID, privateValue) >= 0 {
		goCtx.handleStore.Delete(int32(builderID))
	}
	C.JS_FreeValue(ctx, privateValue)

	return C.int(0)
}

func bestEffortCleanupModuleBuilderHandles(ctx *Context) {
	if ctx == nil || ctx.handleStore == nil {
		return
	}

	var ids []int32
	ctx.handleStore.handles.Range(func(key, value interface{}) bool {
		id, ok := key.(int32)
		if !ok || id <= 0 {
			return true
		}
		h, ok := value.(cgo.Handle)
		if !ok {
			return true
		}
		if mb, ok := h.Value().(*ModuleBuilder); ok && mb != nil {
			ids = append(ids, id)
		}
		return true
	})

	for _, id := range ids {
		ctx.handleStore.Delete(id)
	}
}

func forceModuleBuilderIDParseFailureForTest(enable bool) func() {
	old := moduleBuilderIDForceParseFailure.Swap(enable)
	return func() {
		moduleBuilderIDForceParseFailure.Store(old)
	}
}
