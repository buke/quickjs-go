#include "_cgo_export.h"

JSValue InvokeProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv) {
	 return goProxy(ctx, this_val, argc, argv);
}

JSValue InvokeAsyncProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv) {
	return goAsyncProxy(ctx, this_val, argc, argv);
}