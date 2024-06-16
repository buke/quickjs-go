#include "_cgo_export.h"
#include "quickjs.h"
#include <time.h>


JSValue JS_NewNull() { return JS_NULL; }
JSValue JS_NewUndefined() { return JS_UNDEFINED; }
JSValue JS_NewUninitialized() { return JS_UNINITIALIZED; }

JSValue ThrowSyntaxError(JSContext *ctx, const char *fmt) { return JS_ThrowSyntaxError(ctx, "%s", fmt); }
JSValue ThrowTypeError(JSContext *ctx, const char *fmt) { return JS_ThrowTypeError(ctx, "%s", fmt); }
JSValue ThrowReferenceError(JSContext *ctx, const char *fmt) { return JS_ThrowReferenceError(ctx, "%s", fmt); }
JSValue ThrowRangeError(JSContext *ctx, const char *fmt) { return JS_ThrowRangeError(ctx, "%s", fmt); }
JSValue ThrowInternalError(JSContext *ctx, const char *fmt) { return JS_ThrowInternalError(ctx, "%s", fmt); }

int ValueGetTag(JSValueConst v) {
	return JS_VALUE_GET_TAG(v);
}

JSValue InvokeProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv) {
	 return goProxy(ctx, this_val, argc, argv);
}

JSValue InvokeAsyncProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv) {
	return goAsyncProxy(ctx, this_val, argc, argv);
}

int interruptHandler(JSRuntime *rt, void *handlerArgs) {
	return goInterruptHandler(rt, handlerArgs);
}

void SetInterruptHandler(JSRuntime *rt, void *handlerArgs){
	JS_SetInterruptHandler(rt, &interruptHandler, handlerArgs);
}

typedef struct {
    time_t start;
    time_t timeout;
} TimeoutStruct;

int timeoutHandler(JSRuntime *rt, void *opaque) {
    TimeoutStruct* ts = (TimeoutStruct*)opaque;
    time_t timeout = ts->timeout;
    time_t start = ts->start;
	if (timeout <= 0) {
		return 0;
	}

	time_t now = time(NULL);
	if (now - start > timeout) {
		free(ts);
		return 1;
	}

	return 0;
}

void SetExecuteTimeout(JSRuntime *rt, time_t timeout){
    TimeoutStruct* ts = malloc(sizeof(TimeoutStruct));
    ts->start = time(NULL);
    ts->timeout = timeout;
    JS_SetInterruptHandler(rt, &timeoutHandler, ts);
}