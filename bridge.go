package quickjs

import (
	"runtime/cgo"
	"unsafe"
)

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

//export goProxy
func goProxy(ctx *C.JSContext, thisVal C.JSValueConst, argc C.int, argv *C.JSValueConst) C.JSValue {
	refs := unsafe.Slice(argv, argc) // Go 1.17 and later

	// get the function
	fnHandler := C.int64_t(0)
	C.JS_ToInt64(ctx, &fnHandler, refs[0])
	fn := cgo.Handle(fnHandler).Value().(func(ctx *Context, this Value, args []Value) Value)

	// get ctx
	ctxHandler := C.int64_t(0)
	C.JS_ToInt64(ctx, &ctxHandler, refs[1])
	ctxOrigin := cgo.Handle(ctxHandler).Value().(*Context)

	// refs[0] is the id, refs[1] is the ctx
	args := make([]Value, len(refs)-2)
	for i := 0; i < len(args); i++ {
		args[i].ctx = ctxOrigin
		args[i].ref = refs[2+i]
	}

	result := fn(ctxOrigin, Value{ctx: ctxOrigin, ref: thisVal}, args)

	return result.ref
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
