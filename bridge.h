#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <time.h>
#include "quickjs.h"
#include "quickjs-libc.h"

extern JSValue JS_NewNull();
extern JSValue JS_NewUndefined();
extern JSValue JS_NewUninitialized();
extern JSValue ThrowSyntaxError(JSContext *ctx, const char *fmt);
extern JSValue ThrowTypeError(JSContext *ctx, const char *fmt);
extern JSValue ThrowReferenceError(JSContext *ctx, const char *fmt);
extern JSValue ThrowRangeError(JSContext *ctx, const char *fmt);
extern JSValue ThrowInternalError(JSContext *ctx, const char *fmt);
int JS_DeletePropertyInt64(JSContext *ctx, JSValueConst obj, int64_t idx, int flags);


// New efficient proxy function for regular functions only
extern JSValue GoFunctionProxy(JSContext *ctx, JSValueConst this_val, 
                              int argc, JSValueConst *argv, int magic);

extern int ValueGetTag(JSValueConst v);
extern JSValue LoadModuleBytecode(JSContext *ctx, const uint8_t *buf, size_t buf_len, int load_only);

// Simplified interrupt handler interface (no handlerArgs complexity)
extern void SetInterruptHandler(JSRuntime *rt);
extern void ClearInterruptHandler(JSRuntime *rt);
extern void SetExecuteTimeout(JSRuntime *rt, time_t timeout);