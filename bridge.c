#include "_cgo_export.h"
#include "quickjs.h"
#include "quickjs-libc.h"
#include "cutils.h" 
#include <time.h>

// ============================================================================
// MACRO WRAPPER FUNCTIONS
// Convert QuickJS macros to functions for Windows cgo compatibility
// ============================================================================

// Value creation macros -> functions
JSValue JS_NewNull() { return JS_NULL; }
JSValue JS_NewUndefined() { return JS_UNDEFINED; }
JSValue JS_NewUninitialized() { return JS_UNINITIALIZED; }
JSValue JS_NewException() { return JS_EXCEPTION; }
JSValue JS_NewTrue() { return JS_TRUE; }
JSValue JS_NewFalse() { return JS_FALSE; }

// Error throwing macros -> functions
JSValue ThrowSyntaxError(JSContext *ctx, const char *fmt) { return JS_ThrowSyntaxError(ctx, "%s", fmt); }
JSValue ThrowTypeError(JSContext *ctx, const char *fmt) { return JS_ThrowTypeError(ctx, "%s", fmt); }
JSValue ThrowReferenceError(JSContext *ctx, const char *fmt) { return JS_ThrowReferenceError(ctx, "%s", fmt); }
JSValue ThrowRangeError(JSContext *ctx, const char *fmt) { return JS_ThrowRangeError(ctx, "%s", fmt); }
JSValue ThrowInternalError(JSContext *ctx, const char *fmt) { return JS_ThrowInternalError(ctx, "%s", fmt); }

// Type checking macros -> functions (these are heavily used in Go code)
int JS_IsNumber_Wrapper(JSValue val) { return JS_IsNumber(val); }
int JS_IsBigInt_Wrapper(JSContext *ctx, JSValue val) { return JS_IsBigInt(ctx, val); }
int JS_IsBool_Wrapper(JSValue val) { return JS_IsBool(val); }
int JS_IsNull_Wrapper(JSValue val) { return JS_IsNull(val); }
int JS_IsUndefined_Wrapper(JSValue val) { return JS_IsUndefined(val); }
int JS_IsException_Wrapper(JSValue val) { return JS_IsException(val); }
int JS_IsUninitialized_Wrapper(JSValue val) { return JS_IsUninitialized(val); }
int JS_IsString_Wrapper(JSValue val) { return JS_IsString(val); }
int JS_IsSymbol_Wrapper(JSValue val) { return JS_IsSymbol(val); }
int JS_IsObject_Wrapper(JSValue val) { return JS_IsObject(val); }

// Value tag access macro -> function
int ValueGetTag(JSValueConst v) {
    return JS_VALUE_GET_TAG(v);
}

// Value pointer access macro -> function
void* JS_VALUE_GET_PTR_Wrapper(JSValue val) {
    return JS_VALUE_GET_PTR(val);
}

// Property flags (For class.go)
int GetPropertyWritableConfigurable() { return JS_PROP_WRITABLE | JS_PROP_CONFIGURABLE; }
int GetPropertyConfigurable() { return JS_PROP_CONFIGURABLE; }

// TypedArray enum values (For context.go)
int GetTypedArrayInt8() { return JS_TYPED_ARRAY_INT8; }
int GetTypedArrayUint8() { return JS_TYPED_ARRAY_UINT8; }
int GetTypedArrayUint8C() { return JS_TYPED_ARRAY_UINT8C; }
int GetTypedArrayInt16() { return JS_TYPED_ARRAY_INT16; }
int GetTypedArrayUint16() { return JS_TYPED_ARRAY_UINT16; }
int GetTypedArrayInt32() { return JS_TYPED_ARRAY_INT32; }
int GetTypedArrayUint32() { return JS_TYPED_ARRAY_UINT32; }
int GetTypedArrayFloat32() { return JS_TYPED_ARRAY_FLOAT32; }
int GetTypedArrayFloat64() { return JS_TYPED_ARRAY_FLOAT64; }
int GetTypedArrayBigInt64() { return JS_TYPED_ARRAY_BIG_INT64; }
int GetTypedArrayBigUint64() { return JS_TYPED_ARRAY_BIG_UINT64; }

// Evaluation flags (For context.go)
int GetEvalTypeGlobal() { return JS_EVAL_TYPE_GLOBAL; }
int GetEvalTypeModule() { return JS_EVAL_TYPE_MODULE; }
int GetEvalFlagStrict() { return JS_EVAL_FLAG_STRICT; }
int GetEvalFlagCompileOnly() { return JS_EVAL_FLAG_COMPILE_ONLY; }

// Read/Write object flags
int GetReadObjBytecode() { return JS_READ_OBJ_BYTECODE; }
int GetWriteObjBytecode() { return JS_WRITE_OBJ_BYTECODE; }

// Function type enums (For class.go)
int GetCFuncGeneric() { return JS_CFUNC_generic; }
int GetCFuncGenericMagic() { return JS_CFUNC_generic_magic; }
int GetCFuncConstructor() { return JS_CFUNC_constructor; }
int GetCFuncConstructorMagic() { return JS_CFUNC_constructor_magic; }
int GetCFuncGetterMagic() { return JS_CFUNC_getter_magic; }
int GetCFuncSetterMagic() { return JS_CFUNC_setter_magic; }

// Promise states (For value.go)
int GetPromisePending() { return JS_PROMISE_PENDING; }
int GetPromiseFulfilled() { return JS_PROMISE_FULFILLED; }
int GetPromiseRejected() { return JS_PROMISE_REJECTED; }

// Class ID
int GetInvalidClassID() { return JS_INVALID_CLASS_ID; }

// ============================================================================
// HELPER FUNCTIONS 
// ============================================================================

// Helper functions for safe opaque data handling
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

// ============================================================================
// NEWINSTANCE HELPER FUNCTION
// ============================================================================

// CreateClassInstance - encapsulates the object creation logic from NewInstance
// This function handles:
// 1. Getting prototype from constructor
// 2. Creating JS object with correct prototype and class
// 3. Setting opaque data
// 4. Error handling and cleanup
// 
// Returns JS_EXCEPTION on any error, proper JSValue on success
// This corresponds to the logic in point.c example
JSValue CreateClassInstance(JSContext *ctx, JSValue constructor, 
                           JSClassID class_id, int32_t handle_id) {
    JSValue proto, obj;
    
    // Get prototype from constructor 
    // Corresponds to point.c: proto = JS_GetPropertyStr(ctx, new_target, "prototype")
    proto = JS_GetPropertyStr(ctx, constructor, "prototype");
    if (JS_IsException(proto)) {
        // Return the exception directly, caller will handle cleanup
        return proto;
    }
    
    // Create JS object with correct prototype and class
    // Corresponds to point.c: obj = JS_NewObjectProtoClass(ctx, proto, js_point_class_id)
    obj = JS_NewObjectProtoClass(ctx, proto, class_id);
    
    // Free prototype reference (always needed, regardless of obj creation result)
    JS_FreeValue(ctx, proto);
    
    if (JS_IsException(obj)) {
        // Return the exception directly, caller will handle cleanup
        return obj;
    }
    
    // Associate Go object with JS object
    // Corresponds to point.c: JS_SetOpaque(obj, s)
    // Use helper function to safely convert int32 to opaque pointer
    JS_SetOpaque(obj, IntToOpaque(handle_id));
    
    return obj;
}

// ============================================================================
// CREATECFUNCTION HELPER FUNCTION
// ============================================================================

// CreateCFunction - encapsulates C function creation logic
// This function handles:
// 1. Function type validation and proxy selection
// 2. JS_NewCFunction2 call with proper parameters
// 3. Error handling
// 
// Parameters match JS_NewCFunction2: ctx, name, length, cproto, magic
// Returns JS_EXCEPTION on any error, proper JSValue on success
JSValue CreateCFunction(JSContext *ctx, const char *name, 
                       int length, int func_type, int32_t handler_id) {
    // Get magic enum values for comparison
    int constructor_magic = GetCFuncConstructorMagic();
    int generic_magic = GetCFuncGenericMagic();
    int getter_magic = GetCFuncGetterMagic();
    int setter_magic = GetCFuncSetterMagic();
    
    // Create the C function based on type - each type needs proper casting
    JSValue jsFunc;
    
    if (func_type == constructor_magic) {
        // Constructor function: JSValue (*)(JSContext *, JSValueConst, int, JSValueConst *, int)
        jsFunc = JS_NewCFunction2(ctx, (JSCFunction *)GoClassConstructorProxy, name, length, 
                                 (JSCFunctionEnum)func_type, handler_id);
    } else if (func_type == generic_magic) {
        // Generic method: JSValue (*)(JSContext *, JSValueConst, int, JSValueConst *, int)
        jsFunc = JS_NewCFunction2(ctx, (JSCFunction *)GoClassMethodProxy, name, length, 
                                 (JSCFunctionEnum)func_type, handler_id);
    } else if (func_type == getter_magic) {
        // Getter function: JSValue (*)(JSContext *, JSValueConst, int)
        // Note: QuickJS will handle the signature mismatch internally based on the JSCFunctionEnum
        jsFunc = JS_NewCFunction2(ctx, (JSCFunction *)GoClassGetterProxy, name, length, 
                                 (JSCFunctionEnum)func_type, handler_id);
    } else if (func_type == setter_magic) {
        // Setter function: JSValue (*)(JSContext *, JSValueConst, JSValueConst, int)
        // Note: QuickJS will handle the signature mismatch internally based on the JSCFunctionEnum
        jsFunc = JS_NewCFunction2(ctx, (JSCFunction *)GoClassSetterProxy, name, length, 
                                 (JSCFunctionEnum)func_type, handler_id);
    } else {
        // Return exception for unsupported function type
        return JS_ThrowTypeError(ctx, "unsupported function type: %d", func_type);
    }
    
    // JS_NewCFunction2 returns JS_EXCEPTION on failure
    // No need to check explicitly, just return the result
    return jsFunc;
}

// ============================================================================
// INTERRUPT HANDLERS
// ============================================================================

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

// ============================================================================
// MODULE LOADING
// ============================================================================

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