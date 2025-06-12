#include "_cgo_export.h"
#include "quickjs.h"
#include "quickjs-libc.h"
#include "cutils.h" 
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

// Helper functions for safe opaque data handling
// These avoid pointer arithmetic in Go code
void* IntToOpaque(int32_t id) {
    return (void*)(intptr_t)id;
}

int32_t OpaqueToInt(void* opaque) {
    return (int32_t)(intptr_t)opaque;
}

// Efficient proxy function for regular functions
JSValue GoFunctionProxy(JSContext *ctx, JSValueConst this_val, 
                       int argc, JSValueConst *argv, int magic) {
    return goFunctionProxy(ctx, this_val, argc, argv, magic);
}

// Class-related proxy functions - C layer wrappers for Go exports

// Constructor proxy - handles new_target for inheritance support
// Corresponds to QuickJS JSCFunctionType.constructor_magic
JSValue GoClassConstructorProxy(JSContext *ctx, JSValueConst new_target, 
                               int argc, JSValueConst *argv, int magic) {
    return goClassConstructorProxy(ctx, new_target, argc, argv, magic);
}

// Method proxy - handles both instance and static methods
// Corresponds to QuickJS JSCFunctionType.generic_magic  
JSValue GoClassMethodProxy(JSContext *ctx, JSValueConst this_val,
                          int argc, JSValueConst *argv, int magic) {
    return goClassMethodProxy(ctx, this_val, argc, argv, magic);
}

// Property getter proxy
// Corresponds to QuickJS JSCFunctionType.getter_magic
JSValue GoClassGetterProxy(JSContext *ctx, JSValueConst this_val, int magic) {
    return goClassGetterProxy(ctx, this_val, magic);
}

// Property setter proxy
// Corresponds to QuickJS JSCFunctionType.setter_magic
JSValue GoClassSetterProxy(JSContext *ctx, JSValueConst this_val, 
                          JSValueConst val, int magic) {
    return goClassSetterProxy(ctx, this_val, val, magic);
}

// Finalizer proxy - unified cleanup handler
// Corresponds to QuickJS JSClassDef.finalizer
// Called when JS object is garbage collected
void GoClassFinalizerProxy(JSRuntime *rt, JSValue val) {
    goClassFinalizerProxy(rt, val);
}

// Simplified interrupt handler (no handlerArgs complexity)
int interruptHandler(JSRuntime *rt, void *opaque) {
    JSRuntime *runtimePtr = (JSRuntime*)opaque;
    return goInterruptHandler(runtimePtr);
}

void SetInterruptHandler(JSRuntime *rt) {
    // Use rt itself as opaque parameter for Go lookup
    JS_SetInterruptHandler(rt, interruptHandler, (void*)rt);
}

void ClearInterruptHandler(JSRuntime *rt) {
    JS_SetInterruptHandler(rt, NULL, NULL);
}

// Timeout handler implementation (unchanged but improved cleanup)
typedef struct {
    time_t start;
    time_t timeout;
} TimeoutStruct;

int timeoutHandler(JSRuntime *rt, void *opaque) {
    TimeoutStruct* ts = (TimeoutStruct*)opaque;
    time_t timeout = ts->timeout;
    time_t start = ts->start;
    if (timeout <= 0) {
        free(ts); // Free memory if timeout is disabled
        return 0;
    }

    time_t now = time(NULL);
    if (now - start > timeout) {
        free(ts); // Free memory on timeout
        return 1;
    }

    return 0;
}

void SetExecuteTimeout(JSRuntime *rt, time_t timeout) {
    TimeoutStruct* ts = malloc(sizeof(TimeoutStruct));
    ts->start = time(NULL);
    ts->timeout = timeout;
    JS_SetInterruptHandler(rt, timeoutHandler, ts);
}

// LoadModuleBytecode implementation (unchanged)
JSValue LoadModuleBytecode(JSContext *ctx, const uint8_t *buf, size_t buf_len, int load_only) {
    JSValue obj, val;
    
    obj = JS_ReadObject(ctx, buf, buf_len, JS_READ_OBJ_BYTECODE);
    if (JS_IsException(obj)) {
        return obj;
    }
    
    if (load_only) {
        if (JS_VALUE_GET_TAG(obj) == JS_TAG_MODULE) {
            js_module_set_import_meta(ctx, obj, FALSE, FALSE);
        }
        return obj;
    } else {
        if (JS_VALUE_GET_TAG(obj) == JS_TAG_MODULE) {
            if (JS_ResolveModule(ctx, obj) < 0) {
                JS_FreeValue(ctx, obj);
                return JS_EXCEPTION;
            }
            js_module_set_import_meta(ctx, obj, FALSE, TRUE);
            val = JS_EvalFunction(ctx, obj);
            val = js_std_await(ctx, val);
        } else {
            val = JS_EvalFunction(ctx, obj);
        }
        
        if (JS_IsException(val)) {
            JS_FreeValue(ctx, obj);
            return val;
        }
        
        return val;
    }
}