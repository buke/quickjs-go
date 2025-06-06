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
	// Clear all mappings
	contextMapping.Range(func(key, value interface{}) bool {
		contextMapping.Delete(key)
		return true // continue iteration
	})
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

	// Convert arguments efficiently
	var args []Value
	if argc > 0 && argv != nil {
		refs := unsafe.Slice(argv, argc)
		args = make([]Value, argc)
		for i := 0; i < int(argc); i++ {
			args[i] = Value{ctx: goCtx, ref: refs[i]}
		}
	}

	// Call Go function directly (no intermediate JavaScript execution)
	result := goFn(goCtx, Value{ctx: goCtx, ref: thisVal}, args)
	return result.ref
}
