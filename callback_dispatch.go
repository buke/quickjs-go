package quickjs

import "unsafe"

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

// convertCArgsToGoValues converts C arguments to Go Value slice (unified helper).
// Reused by all proxy functions for consistent parameter conversion.
func convertCArgsToGoValues(argc C.int, argv *C.JSValue, ctx *Context) []*Value {
	if argc == 0 {
		return nil
	}

	cArgs := unsafe.Slice(argv, int(argc))
	goArgs := make([]*Value, int(argc))
	for i, cArg := range cArgs {
		goArgs[i] = &Value{ctx: ctx, ref: cArg}
	}
	return goArgs
}

// getContextAndObject retrieves context and callback target from HandleStore.
func getContextAndObject(ctx *C.JSContext, magic C.int, notFoundErr proxyError) (*Context, interface{}, *proxyError) {
	goCtx := getContextFromJS(ctx)
	if goCtx == nil {
		return nil, nil, &errContextNotFound
	}

	funcID := int32(magic)
	fn := goCtx.loadFunctionFromHandleID(funcID)
	if fn == nil {
		return nil, nil, &notFoundErr
	}

	return goCtx, fn, nil
}

//export goInterruptHandler
func goInterruptHandler(runtimePtr *C.JSRuntime) C.int {
	runtime := getRuntimeFromJS(runtimePtr)
	if runtime == nil {
		return C.int(0)
	}
	return C.int(runtime.callInterruptHandler())
}

//export goFunctionProxy
func goFunctionProxy(ctx *C.JSContext, thisVal C.JSValueConst,
	argc C.int, argv *C.JSValueConst, magic C.int) C.JSValue {

	goCtx, fn, err := getContextAndObject(ctx, magic, errFunctionNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	goFn, ok := fn.(func(*Context, *Value, []*Value) *Value)
	if !ok {
		return throwProxyError(ctx, errInvalidFunctionType)
	}

	args := convertCArgsToGoValues(argc, (*C.JSValue)(argv), goCtx)
	thisValue := &Value{ctx: goCtx, ref: thisVal}
	result := goFn(goCtx, thisValue, args)
	if result == nil {
		return C.JS_NewUndefined()
	}
	return result.ref
}

//export goClassMethodProxy
func goClassMethodProxy(ctx *C.JSContext, thisVal C.JSValue,
	argc C.int, argv *C.JSValue, magic C.int) C.JSValue {

	goCtx, fn, err := getContextAndObject(ctx, magic, errMethodNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	method, ok := fn.(ClassMethodFunc)
	if !ok {
		return throwProxyError(ctx, errInvalidMethodType)
	}

	thisValue := &Value{ctx: goCtx, ref: thisVal}
	args := convertCArgsToGoValues(argc, argv, goCtx)
	result := method(goCtx, thisValue, args)
	if result == nil {
		return C.JS_NewUndefined()
	}
	return result.ref
}

//export goClassGetterProxy
func goClassGetterProxy(ctx *C.JSContext, thisVal C.JSValue, magic C.int) C.JSValue {
	goCtx, fn, err := getContextAndObject(ctx, magic, errGetterNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	getter, ok := fn.(ClassGetterFunc)
	if !ok {
		return throwProxyError(ctx, errInvalidGetterType)
	}

	thisValue := &Value{ctx: goCtx, ref: thisVal}
	result := getter(goCtx, thisValue)
	if result == nil {
		return C.JS_NewUndefined()
	}
	return result.ref
}

//export goClassSetterProxy
func goClassSetterProxy(ctx *C.JSContext, thisVal C.JSValue,
	val C.JSValue, magic C.int) C.JSValue {

	goCtx, fn, err := getContextAndObject(ctx, magic, errSetterNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	setter, ok := fn.(ClassSetterFunc)
	if !ok {
		return throwProxyError(ctx, errInvalidSetterType)
	}

	thisValue := &Value{ctx: goCtx, ref: thisVal}
	setValue := &Value{ctx: goCtx, ref: val}
	result := setter(goCtx, thisValue, setValue)
	if result == nil {
		return C.JS_NewUndefined()
	}
	return result.ref
}
