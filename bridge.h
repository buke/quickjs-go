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


// Function proxy for regular functions
extern JSValue GoFunctionProxy(JSContext *ctx, JSValueConst this_val, 
                              int argc, JSValueConst *argv, int magic);

// Class-related proxy functions
// Constructor proxy - handles new_target for inheritance support
extern JSValue GoClassConstructorProxy(JSContext *ctx, JSValueConst new_target, 
                                      int argc, JSValueConst *argv, int magic);

// Method proxy - handles both instance and static methods  
extern JSValue GoClassMethodProxy(JSContext *ctx, JSValueConst this_val,
                                 int argc, JSValueConst *argv, int magic);

// Property getter proxy
extern JSValue GoClassGetterProxy(JSContext *ctx, JSValueConst this_val, int magic);

// Property setter proxy  
extern JSValue GoClassSetterProxy(JSContext *ctx, JSValueConst this_val, 
                                 JSValueConst val, int magic);

// Finalizer proxy - unified cleanup handler
extern void GoClassFinalizerProxy(JSRuntime *rt, JSValue val);

extern int ValueGetTag(JSValueConst v);
extern JSValue LoadModuleBytecode(JSContext *ctx, const uint8_t *buf, size_t buf_len, int load_only);

// Simplified interrupt handler interface (no handlerArgs complexity)
extern void SetInterruptHandler(JSRuntime *rt);
extern void ClearInterruptHandler(JSRuntime *rt);
extern void SetExecuteTimeout(JSRuntime *rt, time_t timeout);