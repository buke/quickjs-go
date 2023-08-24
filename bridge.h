#include <stdlib.h>
#include <string.h>
#include "quickjs.h"

extern JSValue JS_NewNull();
extern JSValue JS_NewUndefined();
extern JSValue JS_NewUninitialized();
extern JSValue ThrowSyntaxError(JSContext *ctx, const char *fmt) ;
extern JSValue ThrowTypeError(JSContext *ctx, const char *fmt) ;
extern JSValue ThrowReferenceError(JSContext *ctx, const char *fmt) ;
extern JSValue ThrowRangeError(JSContext *ctx, const char *fmt) ;
extern JSValue ThrowInternalError(JSContext *ctx, const char *fmt) ;
int JS_DeletePropertyInt64(JSContext *ctx, JSValueConst obj, int64_t idx, int flags);

extern JSValue InvokeProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv);
extern JSValue InvokeAsyncProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv);


typedef struct {
    uintptr_t fn;
} handlerArgs;

extern void SetInterruptHandler(JSRuntime *rt, void *handlerArgs);