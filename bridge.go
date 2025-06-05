package quickjs

import (
	"runtime/cgo"
	"sync"
	"unsafe"
)

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

// Global context mapping using sync.Map for lock-free performance
var contextMapping sync.Map // map[*C.JSContext]*Context

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

//export goAsyncProxy
func goAsyncProxy(ctx *C.JSContext, thisVal C.JSValueConst, argc C.int, argv *C.JSValueConst) C.JSValue {
	refs := unsafe.Slice(argv, argc) // Go 1.17 and later

	// get the function
	fnHandler := C.int64_t(0)
	C.JS_ToInt64(ctx, &fnHandler, refs[0])
	asyncFn := cgo.Handle(fnHandler).Value().(func(ctx *Context, this Value, promise Value, args []Value) Value)

	// get ctx
	ctxHandler := C.int64_t(0)
	C.JS_ToInt64(ctx, &ctxHandler, refs[1])
	ctxOrigin := cgo.Handle(ctxHandler).Value().(*Context)

	args := make([]Value, len(refs)-2)
	for i := 0; i < len(args); i++ {
		args[i].ctx = ctxOrigin
		args[i].ref = refs[2+i]
	}
	promise := args[0]

	result := asyncFn(ctxOrigin, Value{ctx: ctxOrigin, ref: thisVal}, promise, args[1:])
	return result.ref

}

//export goInterruptHandler
func goInterruptHandler(rt *C.JSRuntime, handlerArgs unsafe.Pointer) C.int {
	handlerArgsStruct := (*C.handlerArgs)(handlerArgs)

	hFn := cgo.Handle(handlerArgsStruct.fn)
	hFnValue := hFn.Value().(InterruptHandler)
	// defer hFn.Delete()

	return C.int(hFnValue())
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
