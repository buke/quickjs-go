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
    if (handle_id != 0) {
        JS_SetOpaque(obj, IntToOpaque(handle_id));
    }
    
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
// CLASS CREATION HELPER FUNCTIONS
// ============================================================================

// Forward declarations
JSValue BindMembersToObject(JSContext *ctx, JSValue obj,
                           const MethodEntry *methods, int method_count,
                           const PropertyEntry *properties, int property_count,
                           int is_static);

JSValue BindMethodToObject(JSContext *ctx, JSValue obj, const MethodEntry *method);

JSValue BindPropertyToObject(JSContext *ctx, JSValue obj, const PropertyEntry *prop);

// CreateClass - Complete class creation function
// This function handles all QuickJS class creation steps:
// 1. Allocate class_id internally
// 2. Register class definition (Go layer manages JSClassDef memory)
// 3. Create prototype object
// 4. Bind instance members to prototype
// 5. Create constructor function
// 6. Associate constructor with prototype
// 7. Set class prototype
// 8. Bind static members to constructor
//
// Returns constructor JSValue on success, JS_EXCEPTION on failure
// class_id is returned via pointer parameter
JSValue CreateClass(JSContext *ctx,
                   JSClassID *class_id,        // C function allocates internally
                   JSClassDef *class_def,      // Go layer manages memory
                   int32_t constructor_id,
                   const MethodEntry *methods, int method_count,
                   const PropertyEntry *properties, int property_count) {
    
    JSValue proto, constructor;
    JSRuntime *rt = JS_GetRuntime(ctx);
    
    // Step 1: Input validation
    if (!class_def || !class_def->class_name) {
        return JS_ThrowInternalError(ctx, "class_def or class_name is null");
    }

    // Check for empty class name
    if (strlen(class_def->class_name) == 0) {
        return JS_ThrowInternalError(ctx, "class_name cannot be empty");
    }
    
    // Step 2: Allocate class_id internally (corresponds to point.c: JS_NewClassID(&js_point_class_id))
    JS_NewClassID(class_id);
    
    // Check QuickJS limits
    if (*class_id >= (1 << 16)) {
        return JS_ThrowRangeError(ctx, "class ID exceeds maximum value");
    }
    
    // Step 3: Register class definition (corresponds to point.c: JS_NewClass)
    // Go layer manages class_def memory, we just use it
    int class_result = JS_NewClass(rt, *class_id, class_def);
    if (class_result != 0) {
        return JS_ThrowInternalError(ctx, "JS_NewClass failed: result=%d", class_result);
    }
    
    // Step 4: Create prototype object (corresponds to point.c: point_proto = JS_NewObject(ctx))
    proto = JS_NewObject(ctx);
    if (JS_IsException(proto)) {
        return proto;
    }
    
    // Step 5: Bind instance members to prototype
    JSValue proto_result = BindMembersToObject(ctx, proto, methods, method_count,
                                              properties, property_count, 0);
    if (JS_IsException(proto_result)) {
        JS_FreeValue(ctx, proto);
        return proto_result;
    }
    
    // Step 6: Create constructor function (corresponds to point.c: JS_NewCFunction2)
    constructor = CreateCFunction(ctx, class_def->class_name, 2,
                                 GetCFuncConstructorMagic(), constructor_id);
    if (JS_IsException(constructor)) {
        JS_FreeValue(ctx, proto);
        return constructor;
    }
    
    // Step 7: Associate constructor with prototype (corresponds to point.c: JS_SetConstructor)
    JS_SetConstructor(ctx, constructor, proto);
    
    // Step 8: Set class prototype (corresponds to point.c: JS_SetClassProto)
    JS_SetClassProto(ctx, *class_id, proto);
    
    // Step 9: Bind static members to constructor
    JSValue constructor_result = BindMembersToObject(ctx, constructor, methods, method_count,
                                                    properties, property_count, 1);
    if (JS_IsException(constructor_result)) {
        JS_FreeValue(ctx, constructor);
        return constructor_result;
    }
    
    // Success: class_id has been set via pointer, return constructor
    return constructor;
}

// BindMembersToObject - Bind methods and properties to a JavaScript object
// is_static: 0 for instance members, 1 for static members
JSValue BindMembersToObject(JSContext *ctx, JSValue obj,
                           const MethodEntry *methods, int method_count,
                           const PropertyEntry *properties, int property_count,
                           int is_static) {
    // Bind methods
    for (int i = 0; i < method_count; i++) {
        const MethodEntry *method = &methods[i];
        if (method->is_static == is_static) {
            JSValue method_result = BindMethodToObject(ctx, obj, method);
            if (JS_IsException(method_result)) {
                return method_result;
            }
        }
    }
    
    // Bind properties
    for (int i = 0; i < property_count; i++) {
        const PropertyEntry *prop = &properties[i];
        if (prop->is_static == is_static) {
            JSValue prop_result = BindPropertyToObject(ctx, obj, prop);
            if (JS_IsException(prop_result)) {
                return prop_result;
            }
        }
    }
    
    return JS_UNDEFINED;
}

// BindMethodToObject - Bind a method to a JavaScript object
JSValue BindMethodToObject(JSContext *ctx, JSValue obj, const MethodEntry *method) {
    // Create method function
    JSValue method_func = CreateCFunction(ctx, method->name, method->length,
                                         GetCFuncGenericMagic(), method->handler_id);
    if (JS_IsException(method_func)) {
        return method_func;
    }
    
    // Define property
    int result = JS_DefinePropertyValueStr(ctx, obj, method->name, method_func,
                                          GetPropertyWritableConfigurable());
    if (result < 0) {
        JS_FreeValue(ctx, method_func);
        return JS_ThrowInternalError(ctx, "failed to bind method: %s", method->name);
    }
    
    return JS_UNDEFINED;
}

// BindPropertyToObject - Bind a property to a JavaScript object
JSValue BindPropertyToObject(JSContext *ctx, JSValue obj, const PropertyEntry *prop) {
    JSAtom prop_atom = JS_NewAtom(ctx, prop->name);
    JSValue getter = JS_UNDEFINED;
    JSValue setter = JS_UNDEFINED;
    
    // Create getter
    if (prop->getter_id != 0) {
        getter = CreateCFunction(ctx, prop->name, 0,
                                GetCFuncGetterMagic(), prop->getter_id);
        if (JS_IsException(getter)) {
            JS_FreeAtom(ctx, prop_atom);
            return getter;
        }
    }
    
    // Create setter
    if (prop->setter_id != 0) {
        setter = CreateCFunction(ctx, prop->name, 1,
                                GetCFuncSetterMagic(), prop->setter_id);
        if (JS_IsException(setter)) {
            JS_FreeAtom(ctx, prop_atom);
            if (!JS_IsUndefined(getter)) {
                JS_FreeValue(ctx, getter);
            }
            return setter;
        }
    }
    
    // Define property
    int result = JS_DefinePropertyGetSet(ctx, obj, prop_atom, getter, setter,
                                        GetPropertyConfigurable());
    
    JS_FreeAtom(ctx, prop_atom);
    
    if (result < 0) {
        if (!JS_IsUndefined(getter)) JS_FreeValue(ctx, getter);
        if (!JS_IsUndefined(setter)) JS_FreeValue(ctx, setter);
        return JS_ThrowInternalError(ctx, "failed to bind property: %s", prop->name);
    }
    
    return JS_UNDEFINED;
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