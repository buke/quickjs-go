package quickjs

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

var moduleBuilderIDForceParseFailure atomic.Bool

func cleanupModuleBuilderHandle(ctx *Context, builderID C.int32_t) {
	if ctx == nil || ctx.handleStore == nil {
		return
	}
	ctx.handleStore.Delete(int32(builderID))
}

// getContextAndModuleBuilder retrieves context and ModuleBuilder from module private value.
func getContextAndModuleBuilder(ctx *C.JSContext, m *C.JSModuleDef) (*Context, *ModuleBuilder, C.int32_t, error) {
	privateValue := C.JS_GetModulePrivateValue(ctx, m)

	var builderID C.int32_t
	if moduleBuilderIDForceParseFailure.Load() || C.JS_ToInt32(ctx, &builderID, privateValue) < 0 {
		C.JS_FreeValue(ctx, privateValue)
		return nil, nil, 0, fmt.Errorf("failed to parse module builder id from module private value")
	}
	C.JS_FreeValue(ctx, privateValue)

	goCtx, builderInterface, err := getContextAndObject(ctx, C.int(builderID), errFunctionNotFound)
	if err != nil {
		cleanupModuleBuilderHandle(goCtx, builderID)
		return nil, nil, 0, fmt.Errorf("failed to get context and module builder: %v", err.message)
	}

	builder, ok := builderInterface.(*ModuleBuilder)
	if !ok || builder == nil {
		cleanupModuleBuilderHandle(goCtx, builderID)
		return nil, nil, 0, fmt.Errorf("failed to get context and module builder: invalid module builder handle")
	}

	return goCtx, builder, builderID, nil
}

//export goModuleInitProxy
func goModuleInitProxy(ctx *C.JSContext, m *C.JSModuleDef) C.int {
	goCtx, builder, builderID, err := getContextAndModuleBuilder(ctx, m)
	if err != nil {
		return throwModuleError(ctx, err)
	}
	defer cleanupModuleBuilderHandle(goCtx, builderID)

	type materializedModuleExport struct {
		spec  ValueSpec
		value *Value
	}
	var materializedExports []materializedModuleExport
	defer func() {
		for _, item := range materializedExports {
			if item.value == nil || item.value.ctx != goCtx || isContextValueSpec(item.spec) {
				continue
			}
			item.value.Free()
		}
	}()

	for _, export := range builder.exports {
		if export.Spec == nil {
			return throwModuleError(ctx, fmt.Errorf("invalid module export value: %s", export.Name))
		}

		value, matErr := materializeValueSpecSafely(goCtx, export.Spec)
		if matErr != nil {
			return throwModuleError(ctx, fmt.Errorf("invalid module export value: %s (materialize error: %v)", export.Name, matErr))
		}
		if value != nil && value.ctx == goCtx {
			materializedExports = append(materializedExports, materializedModuleExport{spec: export.Spec, value: value})
		}
		if value == nil {
			return throwModuleError(ctx, fmt.Errorf("invalid module export value: %s (materialize returned nil)", export.Name))
		}
		if !value.belongsTo(goCtx) {
			return throwModuleError(ctx, fmt.Errorf("invalid module export value: %s (materialized in a different context)", export.Name))
		}

		exportName := C.CString(export.Name)
		legacySpec := isContextValueSpec(export.Spec)
		rc := C.JS_SetModuleExport(ctx, m, exportName, value.ref)
		C.free(unsafe.Pointer(exportName))
		if rc < 0 {
			// JS_SetModuleExport frees val on failure, so source handles
			// must be invalidated to avoid a later double free.
			value.ref = C.JS_NewUndefined()
			value.borrowed = false
			return throwModuleError(ctx, fmt.Errorf("failed to set module export: %s", export.Name))
		}
		if legacySpec {
			// Legacy Export(name, *Value) now keeps source values readable after Build.
			// JS_SetModuleExport consumes the original ref, so mark this Go value as a
			// borrowed non-owning alias. Lifetime is held by the module export slot,
			// and Free only invalidates the Go wrapper without decref.
			value.borrowed = true
		} else {
			value.ref = C.JS_NewUndefined()
			value.borrowed = false
		}
	}

	return C.int(0)
}
func forceModuleBuilderIDParseFailureForTest(enable bool) func() {
	old := moduleBuilderIDForceParseFailure.Swap(enable)
	return func() {
		moduleBuilderIDForceParseFailure.Store(old)
	}
}
