package quickjs

import (
	"fmt"
	"unsafe"
)

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

func classObjectOpaque(objectID int32) unsafe.Pointer {
	if objectID >= 0 {
		return nil
	}
	return C.IntToOpaque(C.int32_t(objectID))
}

func resolveClassObjectFromOpaque(ctx *Context, opaque unsafe.Pointer) (*Context, int32, bool) {
	if ctx == nil || ctx.runtime == nil || opaque == nil {
		return nil, 0, false
	}

	objectID := int32(C.OpaqueToInt(opaque))
	if objectID < 0 {
		identity, ok := ctx.runtime.getClassObjectIdentity(objectID)
		if ok {
			ownerCtx := ctx.runtime.getOwnedContextByID(identity.contextID)
			if ownerCtx == nil || ownerCtx.handleStore == nil {
				return nil, 0, false
			}
			return ownerCtx, identity.handleID, true
		}
	}
	return nil, 0, false
}

//export goClassConstructorProxy
func goClassConstructorProxy(ctx *C.JSContext, newTarget C.JSValueConst,
	argc C.int, argv *C.JSValueConst, magic C.int) C.JSValue {

	goCtx, fn, perr := getContextAndObject(ctx, magic, errConstructorNotFound)
	if perr != nil {
		return throwProxyError(ctx, *perr)
	}

	builder, ok := fn.(*ClassBuilder)
	if !ok {
		return throwProxyError(ctx, errInvalidConstructorType)
	}

	classID, exists := getConstructorClassID(goCtx, C.JSValue(newTarget))
	if !exists {
		v := &Value{ctx: goCtx, ref: C.JSValue(newTarget)}
		classID, exists = v.resolveClassIDFromInheritance()
	}
	if !exists {
		return throwProxyError(ctx, proxyError{"InternalError", "Class ID not found for constructor"})
	}

	var instanceProperties []C.PropertyEntry
	var instancePropertyNames []*C.char
	type materializedInstanceProperty struct {
		spec  ValueSpec
		value *Value
	}
	var materializedProperties []materializedInstanceProperty
	defer func() {
		// bridge.c/BindPropertyToObject duplicates property values with JS_DupValue
		// before defining properties on the instance. Free only non-legacy values
		// that were materialized for this constructor invocation.
		for _, p := range materializedProperties {
			if p.value == nil || isContextValueSpec(p.spec) {
				continue
			}
			p.value.Free()
		}
	}()
	defer func() {
		for _, cStr := range instancePropertyNames {
			C.free(unsafe.Pointer(cStr))
		}
	}()

	for _, property := range builder.properties {
		if !property.Static {
			if property.Spec == nil {
				return throwProxyError(ctx, proxyError{"InternalError", fmt.Sprintf("property value is required: %s", property.Name)})
			}
			propertyValue, err := materializeValueSpecSafely(goCtx, property.Spec)
			if err != nil {
				return throwProxyError(ctx, proxyError{"InternalError", fmt.Sprintf("invalid property value: %s (materialize error: %v)", property.Name, err)})
			}
			if propertyValue == nil {
				return throwProxyError(ctx, proxyError{"InternalError", fmt.Sprintf("invalid property value: %s (materialize returned nil)", property.Name)})
			}
			if !propertyValue.belongsTo(goCtx) {
				return throwProxyError(ctx, proxyError{"InternalError", fmt.Sprintf("invalid property value: %s (materialized in a different context)", property.Name)})
			}
			materializedProperties = append(materializedProperties, materializedInstanceProperty{spec: property.Spec, value: propertyValue})

			propertyName := C.CString(property.Name)
			instancePropertyNames = append(instancePropertyNames, propertyName)
			instanceProperties = append(instanceProperties, C.PropertyEntry{
				name:      propertyName,
				value:     propertyValue.ref,
				is_static: C.int(0),
				flags:     C.int(property.Flags),
			})
		}
	}

	var instancePropertiesPtr *C.PropertyEntry
	if len(instanceProperties) > 0 {
		instancePropertiesPtr = &instanceProperties[0]
	}

	instance := C.CreateClassInstance(
		ctx,
		C.JSValue(newTarget),
		C.JSClassID(classID),
		instancePropertiesPtr,
		C.int(len(instanceProperties)),
	)

	if bool(C.JS_IsException(instance)) {
		return instance
	}

	instanceValue := &Value{ctx: goCtx, ref: instance}
	args := convertCArgsToGoValues(argc, argv, goCtx)
	goObj, err := builder.constructor(goCtx, instanceValue, args)
	if err != nil {
		C.JS_FreeValue(ctx, instance)
		errorMsg := C.CString(err.Error())
		defer C.free(unsafe.Pointer(errorMsg))
		return C.ThrowInternalError(ctx, errorMsg)
	}

	if goObj != nil {
		if goCtx.handleStore == nil {
			C.JS_FreeValue(ctx, instance)
			return throwProxyError(ctx, proxyError{"InternalError", "Handle store not available"})
		}
		if goCtx.runtime == nil {
			C.JS_FreeValue(ctx, instance)
			return throwProxyError(ctx, proxyError{"InternalError", "Context runtime not available"})
		}
		handleID := goCtx.handleStore.Store(goObj)
		objectID := goCtx.runtime.registerClassObjectIdentity(goCtx.contextID, handleID)
		if objectID == 0 {
			goCtx.handleStore.Delete(handleID)
			C.JS_FreeValue(ctx, instance)
			return throwProxyError(ctx, proxyError{"InternalError", "Failed to register class object identity"})
		}
		C.JS_SetOpaque(instance, classObjectOpaque(objectID))
	}

	return instance
}

//export goClassFinalizerProxy
func goClassFinalizerProxy(rt *C.JSRuntime, val C.JSValue) {
	goRt := getRuntimeFromJS(rt)
	if goRt == nil {
		return
	}

	classID := C.JS_GetClassID(val)
	opaque := C.JS_GetOpaque(val, classID)
	if opaque == nil {
		return
	}

	objectID := int32(C.OpaqueToInt(opaque))

	var targetCtx *Context
	var handleID int32

	if objectID < 0 {
		identity, ok := goRt.takeClassObjectIdentity(objectID)
		if !ok {
			return
		}
		targetCtx = goRt.getOwnedContextByID(identity.contextID)
		handleID = identity.handleID
	} else {
		return
	}

	if targetCtx == nil || targetCtx.handleStore == nil {
		return
	}

	if goObj, exists := targetCtx.handleStore.Load(handleID); exists {
		if finalizer, ok := goObj.(ClassFinalizer); ok {
			func() {
				defer func() {
					recover()
				}()
				finalizer.Finalize()
			}()
		}
		targetCtx.handleStore.Delete(handleID)
	}
}
