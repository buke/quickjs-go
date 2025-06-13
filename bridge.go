package quickjs

import (
	"sync"
	"unsafe"
)

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

var (
	// Global context mapping using sync.Map for lock-free performance
	contextMapping sync.Map // map[*C.JSContext]*Context

	// Global runtime mapping for interrupt handler access (new addition)
	runtimeMapping sync.Map // map[*C.JSRuntime]*Runtime
)

// registerContext registers Go Context with C JSContext (internal use)
func registerContext(cCtx *C.JSContext, goCtx *Context) {
	contextMapping.Store(cCtx, goCtx)
}

// unregisterContext removes mapping when Context is closed (internal use)
func unregisterContext(cCtx *C.JSContext) {
	contextMapping.Delete(cCtx)
}

// clearContextMapping clears all registered contexts (internal use)
func clearContextMapping() {
	contextMapping = sync.Map{} // Reset the map for efficient cleanup
}

// getContextFromJS gets Go Context from C JSContext (internal use)
func getContextFromJS(cCtx *C.JSContext) *Context {
	if value, ok := contextMapping.Load(cCtx); ok {
		return value.(*Context)
	}
	return nil
}

// registerRuntime registers Runtime for interrupt handler access (new addition)
func registerRuntime(cRt *C.JSRuntime, goRt *Runtime) {
	runtimeMapping.Store(cRt, goRt)
}

// unregisterRuntime removes Runtime mapping when closed (new addition)
func unregisterRuntime(cRt *C.JSRuntime) {
	runtimeMapping.Delete(cRt)
}

// getRuntimeFromJS gets Go Runtime from C JSRuntime (internal use)
func getRuntimeFromJS(cRt *C.JSRuntime) *Runtime {
	if value, ok := runtimeMapping.Load(cRt); ok {
		return value.(*Runtime)
	}
	return nil
}

// convertCArgsToGoValues converts C arguments to Go Value slice (unified helper)
// Reused by all proxy functions for consistent parameter conversion
// Note: Works with both JSValue and JSValueConst since we only read values
func convertCArgsToGoValues(argc C.int, argv *C.JSValue, ctx *Context) []Value {
	if argc == 0 {
		return nil
	}

	// Use unsafe.Slice to convert C array to Go slice (Go 1.17+)
	cArgs := unsafe.Slice(argv, int(argc))
	goArgs := make([]Value, int(argc))

	for i, cArg := range cArgs {
		goArgs[i] = Value{ctx: ctx, ref: cArg}
	}

	return goArgs
}

// Simplified interrupt handler export (no cgo.Handle complexity)
//
//export goInterruptHandler
func goInterruptHandler(runtimePtr *C.JSRuntime) C.int {
	// Get Runtime from mapping instead of unsafe handle operations
	runtime := getRuntimeFromJS(runtimePtr)
	if runtime == nil {
		return C.int(0) // Runtime not found, no interrupt
	}

	r := runtime.callInterruptHandler()

	return C.int(r)
}

// New efficient proxy function for regular functions using HandleStore
//
//export goFunctionProxy
func goFunctionProxy(ctx *C.JSContext, thisVal C.JSValueConst,
	argc C.int, argv *C.JSValueConst, magic C.int) C.JSValue {

	// Get Go Context from global mapping (lock-free with sync.Map)
	goCtx := getContextFromJS(ctx)
	if goCtx == nil {
		msg := C.CString("Context not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Get function from Context's HandleStore using magic parameter
	funcID := int32(magic)
	fn := goCtx.loadFunctionFromHandleID(funcID)
	if fn == nil {
		msg := C.CString("Function not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Type assertion to function signature
	goFn, ok := fn.(func(*Context, Value, []Value) Value)
	if !ok {
		msg := C.CString("Invalid function type")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowTypeError(ctx, msg)
	}

	// Convert arguments using unified helper (optimized)
	// Note: Safe cast from JSValueConst to JSValue since we only read values
	args := convertCArgsToGoValues(argc, (*C.JSValue)(argv), goCtx)

	// Call Go function directly (no intermediate JavaScript execution)
	result := goFn(goCtx, Value{ctx: goCtx, ref: thisVal}, args)
	return result.ref
}

// Class constructor proxy - handles new_target for inheritance support
// Corresponds to QuickJS JSCFunctionType.constructor_magic
//
//export goClassConstructorProxy
func goClassConstructorProxy(ctx *C.JSContext, newTarget C.JSValue,
	argc C.int, argv *C.JSValue, magic C.int) C.JSValue {

	// Get Go Context from global mapping
	goCtx := getContextFromJS(ctx)
	if goCtx == nil {
		msg := C.CString("Context not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Get constructor function from HandleStore using magic parameter
	funcID := int32(magic)
	fn := goCtx.loadFunctionFromHandleID(funcID)
	if fn == nil {
		msg := C.CString("Constructor function not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Type assertion to ClassConstructorFunc signature
	constructor, ok := fn.(ClassConstructorFunc)
	if !ok {
		msg := C.CString("Invalid constructor function type")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowTypeError(ctx, msg)
	}

	// Convert parameters using unified helper
	newTargetValue := Value{ctx: goCtx, ref: newTarget}
	args := convertCArgsToGoValues(argc, argv, goCtx)

	// Call Go constructor function with new_target and arguments
	result := constructor(goCtx, newTargetValue, args)
	return result.ref
}

// Class method proxy - handles both instance and static methods
// Corresponds to QuickJS JSCFunctionType.generic_magic
//
//export goClassMethodProxy
func goClassMethodProxy(ctx *C.JSContext, thisVal C.JSValue,
	argc C.int, argv *C.JSValue, magic C.int) C.JSValue {

	// Get Go Context from global mapping
	goCtx := getContextFromJS(ctx)
	if goCtx == nil {
		msg := C.CString("Context not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Get method function from HandleStore using magic parameter
	funcID := int32(magic)
	fn := goCtx.loadFunctionFromHandleID(funcID)
	if fn == nil {
		msg := C.CString("Method function not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Type assertion to ClassMethodFunc signature
	method, ok := fn.(ClassMethodFunc)
	if !ok {
		msg := C.CString("Invalid method function type")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowTypeError(ctx, msg)
	}

	// Convert parameters using unified helper
	thisValue := Value{ctx: goCtx, ref: thisVal}
	args := convertCArgsToGoValues(argc, argv, goCtx)

	// Call Go method function with this and arguments
	result := method(goCtx, thisValue, args)
	return result.ref
}

// Class property getter proxy
// Corresponds to QuickJS JSCFunctionType.getter_magic
//
//export goClassGetterProxy
func goClassGetterProxy(ctx *C.JSContext, thisVal C.JSValue, magic C.int) C.JSValue {

	// Get Go Context from global mapping
	goCtx := getContextFromJS(ctx)
	if goCtx == nil {
		msg := C.CString("Context not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Get getter function from HandleStore using magic parameter
	funcID := int32(magic)
	fn := goCtx.loadFunctionFromHandleID(funcID)
	if fn == nil {
		msg := C.CString("Getter function not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Type assertion to ClassGetterFunc signature
	getter, ok := fn.(ClassGetterFunc)
	if !ok {
		msg := C.CString("Invalid getter function type")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowTypeError(ctx, msg)
	}

	// Call Go getter function with this value only
	thisValue := Value{ctx: goCtx, ref: thisVal}
	result := getter(goCtx, thisValue)
	return result.ref
}

// Class property setter proxy
// Corresponds to QuickJS JSCFunctionType.setter_magic
//
//export goClassSetterProxy
func goClassSetterProxy(ctx *C.JSContext, thisVal C.JSValue,
	val C.JSValue, magic C.int) C.JSValue {

	// Get Go Context from global mapping
	goCtx := getContextFromJS(ctx)
	if goCtx == nil {
		msg := C.CString("Context not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Get setter function from HandleStore using magic parameter
	funcID := int32(magic)
	fn := goCtx.loadFunctionFromHandleID(funcID)
	if fn == nil {
		msg := C.CString("Setter function not found")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowInternalError(ctx, msg)
	}

	// Type assertion to ClassSetterFunc signature
	setter, ok := fn.(ClassSetterFunc)
	if !ok {
		msg := C.CString("Invalid setter function type")
		defer C.free(unsafe.Pointer(msg))
		return C.ThrowTypeError(ctx, msg)
	}

	// Call Go setter function with this value and new value
	thisValue := Value{ctx: goCtx, ref: thisVal}
	setValue := Value{ctx: goCtx, ref: val}
	result := setter(goCtx, thisValue, setValue)
	return result.ref
}

// Class finalizer proxy - unified cleanup handler
// Corresponds to QuickJS JSClassDef.finalizer
// Called automatically when JS object is garbage collected
//
//export goClassFinalizerProxy
func goClassFinalizerProxy(rt *C.JSRuntime, val C.JSValue) {
	// Get class ID for the object being finalized
	classID := C.JS_GetClassID(val)
	invalidClassID := C.uint32_t(C.GetInvalidClassID())
	if classID == invalidClassID {
		return // Not a class instance
	}

	// Get opaque data from JS object using JS_GetOpaque (like point.c finalizer)
	// This corresponds to point.c: s = JS_GetOpaque(val, js_point_class_id)
	// Note: JS_GetOpaque only needs val and class_id (no context required in finalizer)
	opaque := C.JS_GetOpaque(val, classID)
	if opaque == nil {
		return // Corresponds to point.c: 's' can be NULL
	}

	// Use C helper function to safely convert opaque pointer back to int32
	handleID := int32(C.OpaqueToInt(opaque))

	// Get Context from runtime mapping
	// Note: We need to find the Context that owns this object
	// Since finalizer is called at runtime level, we iterate through contexts
	var targetCtx *Context
	contextMapping.Range(func(key, value interface{}) bool {
		ctx := value.(*Context)
		if ctx.runtime.ref == rt {
			// Check if this context has the handle
			if _, exists := ctx.handleStore.Load(handleID); exists {
				targetCtx = ctx
				return false // Stop iteration
			}
		}
		return true // Continue iteration
	})

	if targetCtx == nil {
		return // Context not found or handle already cleaned
	}

	// Get Go object from HandleStore
	if goObj, exists := targetCtx.handleStore.Load(handleID); exists {
		// Check if object implements optional ClassFinalizer interface
		if finalizer, ok := goObj.(ClassFinalizer); ok {
			// Call user-defined cleanup method
			func() {
				defer func() {
					// Recover from panic in user finalizer to prevent crashes
					if r := recover(); r != nil {
						// Note: Cannot use normal logging here as this is called from GC
						// In production, this could be sent to error monitoring
					}
				}()
				finalizer.Finalize()
			}()
		}

		// Always clean up HandleStore reference (corresponds to point.c: js_free_rt)
		targetCtx.handleStore.Delete(handleID)
	}
}
