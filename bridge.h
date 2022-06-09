
#include <stdlib.h>
#include <string.h>
#include "quickjs.h"

extern JSValue InvokeProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv);

static JSValue JS_NewNull() { return JS_NULL; }
static JSValue JS_NewUndefined() { return JS_UNDEFINED; }
static JSValue JS_NewUninitialized() { return JS_UNINITIALIZED; }

static JSValue ThrowSyntaxError(JSContext *ctx, const char *fmt) { return JS_ThrowSyntaxError(ctx, "%s", fmt); }
static JSValue ThrowTypeError(JSContext *ctx, const char *fmt) { return JS_ThrowTypeError(ctx, "%s", fmt); }
static JSValue ThrowReferenceError(JSContext *ctx, const char *fmt) { return JS_ThrowReferenceError(ctx, "%s", fmt); }
static JSValue ThrowRangeError(JSContext *ctx, const char *fmt) { return JS_ThrowRangeError(ctx, "%s", fmt); }
static JSValue ThrowInternalError(JSContext *ctx, const char *fmt) { return JS_ThrowInternalError(ctx, "%s", fmt); }

int JS_DeletePropertyInt64(JSContext *ctx, JSValueConst obj, int64_t idx, int flags);