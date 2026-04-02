#include "_cgo_export.h"
#include "quickjs.h"
#include "quickjs-libc.h"
#include "cutils.h" 
#include <pthread.h>
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

// Value tag access macro -> function
int ValueGetTag(JSValueConst v) {
    return JS_VALUE_GET_TAG(v);
}

// Value pointer access macro -> function
void* JS_VALUE_GET_PTR_Wrapper(JSValue val) {
    return JS_VALUE_GET_PTR(val);
}

// ============================================================================
// HELPER FUNCTIONS 
// ============================================================================

static int isExecuteTimeoutExceeded(JSRuntime *rt);
static int ThrowUnhandledPromiseRejectionIfAny(JSContext *ctx);

static int g_await_poll_slice_ms = 10;
static pthread_mutex_t g_await_poll_slice_mu = PTHREAD_MUTEX_INITIALIZER;

int GetAwaitPollSliceMs(void) {
    int value;
    pthread_mutex_lock(&g_await_poll_slice_mu);
    value = g_await_poll_slice_ms;
    pthread_mutex_unlock(&g_await_poll_slice_mu);
    return value;
}

void SetAwaitPollSliceMs(int timeout_ms) {
    if (timeout_ms <= 0) {
        return;
    }

    pthread_mutex_lock(&g_await_poll_slice_mu);
    g_await_poll_slice_ms = timeout_ms;
    pthread_mutex_unlock(&g_await_poll_slice_mu);
}

typedef struct RejectionEntry {
    JSValue promise;
    JSValue reason;
    struct RejectionEntry *next;
} RejectionEntry;

typedef struct RejectionStateEntry {
    JSRuntime *rt;
    RejectionEntry *head;
    RejectionEntry *tail;
    struct RejectionStateEntry *next;
} RejectionStateEntry;

static RejectionStateEntry *g_rejection_states = NULL;
static pthread_mutex_t g_rejection_states_mu = PTHREAD_MUTEX_INITIALIZER;

static void freeRejectionEntry(JSRuntime *rt, RejectionEntry *entry) {
    if (!entry) {
        return;
    }
    JS_FreeValueRT(rt, entry->promise);
    JS_FreeValueRT(rt, entry->reason);
    free(entry);
}

static RejectionStateEntry *getRejectionState(JSRuntime *rt, int create) {
    RejectionStateEntry *current = g_rejection_states;
    while (current) {
        if (current->rt == rt) {
            return current;
        }
        current = current->next;
    }

    if (!create) {
        return NULL;
    }

    RejectionStateEntry *state = malloc(sizeof(RejectionStateEntry));
    if (!state) {
        return NULL;
    }

    state->rt = rt;
    state->head = NULL;
    state->tail = NULL;
    state->next = g_rejection_states;
    g_rejection_states = state;
    return state;
}

static void clearRejectionState(JSRuntime *rt) {
    pthread_mutex_lock(&g_rejection_states_mu);

    RejectionStateEntry *prev = NULL;
    RejectionStateEntry *current = g_rejection_states;
    while (current) {
        if (current->rt == rt) {
            RejectionEntry *entry = current->head;
            while (entry) {
                RejectionEntry *next = entry->next;
                freeRejectionEntry(rt, entry);
                entry = next;
            }

            if (prev) {
                prev->next = current->next;
            } else {
                g_rejection_states = current->next;
            }
            free(current);
            break;
        }
        prev = current;
        current = current->next;
    }

    pthread_mutex_unlock(&g_rejection_states_mu);
}

static void QuickjsGoPromiseRejectionTracker(JSContext *ctx,
                                             JSValueConst promise,
                                             JSValueConst reason,
                                             bool is_handled,
                                             void *opaque) {
    JSRuntime *rt = JS_GetRuntime(ctx);
    (void)opaque;

    pthread_mutex_lock(&g_rejection_states_mu);
    RejectionStateEntry *state = getRejectionState(rt, 1);
    if (!state) {
        pthread_mutex_unlock(&g_rejection_states_mu);
        return;
    }

    RejectionEntry *prev = NULL;
    RejectionEntry *current = state->head;
    while (current) {
        if (JS_IsSameValue(ctx, current->promise, promise)) {
            break;
        }
        prev = current;
        current = current->next;
    }

    if (is_handled) {
        if (current) {
            if (prev) {
                prev->next = current->next;
            } else {
                state->head = current->next;
            }
            if (state->tail == current) {
                state->tail = prev;
            }
            freeRejectionEntry(rt, current);
        }
        pthread_mutex_unlock(&g_rejection_states_mu);
        return;
    }

    if (current) {
        JS_FreeValueRT(rt, current->reason);
        current->reason = JS_DupValueRT(rt, reason);
        pthread_mutex_unlock(&g_rejection_states_mu);
        return;
    }

    RejectionEntry *entry = malloc(sizeof(RejectionEntry));
    if (!entry) {
        pthread_mutex_unlock(&g_rejection_states_mu);
        return;
    }

    entry->promise = JS_DupValueRT(rt, promise);
    entry->reason = JS_DupValueRT(rt, reason);
    entry->next = NULL;

    if (state->tail) {
        state->tail->next = entry;
    } else {
        state->head = entry;
    }
    state->tail = entry;

    pthread_mutex_unlock(&g_rejection_states_mu);
}

void SetPromiseRejectionTracker(JSRuntime *rt, int enabled) {
    if (!rt) {
        return;
    }

    if (enabled) {
        JS_SetHostPromiseRejectionTracker(rt, QuickjsGoPromiseRejectionTracker, NULL);
        return;
    }

    JS_SetHostPromiseRejectionTracker(rt, NULL, NULL);
    clearRejectionState(rt);
}

static int popUnhandledPromiseRejection(JSContext *ctx, JSValue *reason_out) {
    JSRuntime *rt = JS_GetRuntime(ctx);

    pthread_mutex_lock(&g_rejection_states_mu);
    RejectionStateEntry *state = getRejectionState(rt, 0);
    if (!state || !state->head) {
        pthread_mutex_unlock(&g_rejection_states_mu);
        return 0;
    }

    RejectionEntry *entry = state->head;
    state->head = entry->next;
    if (!state->head) {
        state->tail = NULL;
    }

    *reason_out = entry->reason;
    JS_FreeValueRT(rt, entry->promise);
    free(entry);

    pthread_mutex_unlock(&g_rejection_states_mu);
    return 1;
}

static int ThrowUnhandledPromiseRejectionIfAny(JSContext *ctx) {
    JSValue reason;
    if (!popUnhandledPromiseRejection(ctx, &reason)) {
        return 0;
    }

    JS_Throw(ctx, reason);
    return 1;
}

// Helper functions for safe opaque data handling
void* IntToOpaque(int32_t id) {
    return (void*)(intptr_t)id;
}

int32_t OpaqueToInt(void* opaque) {
    return (int32_t)(intptr_t)opaque;
}

int SetPropertyByNameLen(JSContext *ctx, JSValueConst obj, const char *name, size_t name_len, JSValue val) {
    JSAtom atom = JS_NewAtomLen(ctx, name, name_len);
    if (atom == JS_ATOM_NULL) {
        JS_FreeValue(ctx, val);
        return -1;
    }
    int rc = JS_SetProperty(ctx, obj, atom, val);
    JS_FreeAtom(ctx, atom);
    return rc;
}

JSValue GetPropertyByNameLen(JSContext *ctx, JSValueConst obj, const char *name, size_t name_len) {
    JSAtom atom = JS_NewAtomLen(ctx, name, name_len);
    if (atom == JS_ATOM_NULL) {
        return JS_EXCEPTION;
    }
    JSValue ret = JS_GetProperty(ctx, obj, atom);
    JS_FreeAtom(ctx, atom);
    return ret;
}

JSValue CallPropertyByNameLen(JSContext *ctx, JSValueConst obj, const char *name, size_t name_len, int argc, JSValue *argv) {
    JSAtom atom = JS_NewAtomLen(ctx, name, name_len);
    if (atom == JS_ATOM_NULL) {
        return JS_EXCEPTION;
    }

    JSValue fn = JS_GetProperty(ctx, obj, atom);
    JS_FreeAtom(ctx, atom);
    if (JS_IsException(fn)) {
        return fn;
    }

    JSValue ret = JS_Call(ctx, fn, obj, argc, argv);
    JS_FreeValue(ctx, fn);
    return ret;
}

int DetectModuleSourceWithProbe(JSContext *ctx, const char *code, size_t code_len) {
    if (!JS_DetectModule(code, code_len)) {
        return 0;
    }

    static const char *probe_filename = "<module-detect>";
    int probe_flags = JS_EVAL_TYPE_GLOBAL | JS_EVAL_FLAG_COMPILE_ONLY;
    JSValue probe = JS_Eval(ctx, code, code_len, probe_filename, probe_flags);
    if (JS_IsException(probe)) {
        JSValue exception = JS_GetException(ctx);
        JS_FreeValue(ctx, exception);
        return 1;
    }

    JS_FreeValue(ctx, probe);
    return 0;
}

/*
 * quickjs-go local await helper.
 *
 * Unlike js_std_await from quickjs-libc, this keeps polling in bounded slices
 * and explicitly polls interrupt state directly so
 * execute-timeout/interrupt handlers can abort permanently pending promises.
 */
JSValue AwaitValue(JSContext *ctx, JSValue obj) {
    JSRuntime *rt = JS_GetRuntime(ctx);

    for (;;) {
        if (ThrowUnhandledPromiseRejectionIfAny(ctx)) {
            JS_FreeValue(ctx, obj);
            return JS_EXCEPTION;
        }

        if (isExecuteTimeoutExceeded(rt)) {
            JS_FreeValue(ctx, obj);
            JS_ThrowInternalError(ctx, "interrupted");
            return JS_EXCEPTION;
        }

        int state = JS_PromiseState(ctx, obj);
        if (state == JS_PROMISE_FULFILLED) {
            JSValue ret = JS_PromiseResult(ctx, obj);
            JS_FreeValue(ctx, obj);
            return ret;
        }

        if (state == JS_PROMISE_REJECTED) {
            JSValue ret = JS_Throw(ctx, JS_PromiseResult(ctx, obj));
            JS_FreeValue(ctx, obj);
            return ret;
        }

        if (state != JS_PROMISE_PENDING) {
            return obj;
        }

        JSContext *ctx1 = NULL;
        int err = JS_ExecutePendingJob(rt, &ctx1);
        if (err < 0) {
            if (ctx1 != NULL && ctx1 != ctx) {
                JSValue ex = JS_GetException(ctx1);
                JS_Throw(ctx, ex);
            }
            JS_FreeValue(ctx, obj);
            return JS_EXCEPTION;
        }

        /* Bound host IO polling to avoid an uninterruptible blocking wait. */
        if (err == 0) {
            /*
             * Drive timers/microtasks so promises resolved by setTimeout can
             * progress while awaiting.
             */
            int loop_once_ret = js_std_loop_once(ctx);
            if (loop_once_ret == -2) {
                JS_FreeValue(ctx, obj);
                return JS_EXCEPTION;
            }
            if (loop_once_ret == 0) {
                continue;
            }

            int poll_timeout_ms = GetAwaitPollSliceMs();
            if (loop_once_ret > 0 && loop_once_ret < poll_timeout_ms) {
                poll_timeout_ms = loop_once_ret;
            }

            int poll_ret = js_std_poll_io(ctx, poll_timeout_ms);
            if (poll_ret < 0) {
                JS_FreeValue(ctx, obj);
                return JS_EXCEPTION;
            }

            if (QuickjsGoPollInterrupt(ctx) < 0) {
                JS_FreeValue(ctx, obj);
                return JS_EXCEPTION;
            }
        }
    }
}

JSValue EvalAndAwait(JSContext *ctx, const char *input, size_t input_len, const char *filename, int eval_flags) {
    JSValue eval_result = JS_Eval(ctx, input, input_len, filename, eval_flags);
    return AwaitValue(ctx, eval_result);
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

// Accessor getter proxy
// Corresponds to QuickJS JSCFunctionType.getter_magic
JSValue GoClassGetterProxy(JSContext *ctx, JSValueConst this_val, int magic) {
    return goClassGetterProxy(ctx, this_val, magic);
}

// Accessor setter proxy
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
    int constructor_magic = JS_CFUNC_constructor_magic;
    int generic_magic = JS_CFUNC_generic_magic;
    int getter_magic = JS_CFUNC_getter_magic;
    int setter_magic = JS_CFUNC_setter_magic;
    
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
                           const AccessorEntry *accessors, int accessor_count,
                           const PropertyEntry *properties, int property_count,
                           int is_static);

JSValue BindMethodToObject(JSContext *ctx, JSValue obj, const MethodEntry *method);

JSValue BindAccessorToObject(JSContext *ctx, JSValue obj, const AccessorEntry *accessor);

JSValue BindPropertyToObject(JSContext *ctx, JSValue obj, const PropertyEntry *property);

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
                   const AccessorEntry *accessors, int accessor_count,
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
    
    // Step 2: Allocate class_id internally (corresponds to point.c: JS_NewClassID(rt, &js_point_class_id))
    JS_NewClassID(rt, class_id);
    
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
                                              accessors, accessor_count,
                                              properties, property_count, 0);
    if (JS_IsException(proto_result)) {
        JS_FreeValue(ctx, proto);
        return proto_result;
    }
    
    // Step 6: Create constructor function (corresponds to point.c: JS_NewCFunction2)
    constructor = CreateCFunction(ctx, class_def->class_name, 2,
                                 JS_CFUNC_constructor_magic, constructor_id);
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
                                                    accessors, accessor_count,
                                                    properties, property_count, 1);
    if (JS_IsException(constructor_result)) {
        JS_FreeValue(ctx, constructor);
        return constructor_result;
    }
    
    // Success: class_id has been set via pointer, return constructor
    return constructor;
}

// BindMembersToObject - Bind methods, accessors, and properties to a JavaScript object
// is_static: 0 for instance members, 1 for static members
JSValue BindMembersToObject(JSContext *ctx, JSValue obj,
                           const MethodEntry *methods, int method_count,
                           const AccessorEntry *accessors, int accessor_count,
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
    
    // Bind accessors
    for (int i = 0; i < accessor_count; i++) {
        const AccessorEntry *accessor = &accessors[i];
        if (accessor->is_static == is_static) {
            JSValue accessor_result = BindAccessorToObject(ctx, obj, accessor);
            if (JS_IsException(accessor_result)) {
                return accessor_result;
            }
        }
    }
    
    // Bind properties (data properties support)
    for (int i = 0; i < property_count; i++) {
        const PropertyEntry *property = &properties[i];
        if (property->is_static == is_static) {
            JSValue property_result = BindPropertyToObject(ctx, obj, property);
            if (JS_IsException(property_result)) {
                return property_result;
            }
        }
    }
    
    return JS_UNDEFINED;
}

// BindMethodToObject - Bind a method to a JavaScript object
JSValue BindMethodToObject(JSContext *ctx, JSValue obj, const MethodEntry *method) {
    // Create method function
    JSValue method_func = CreateCFunction(ctx, method->name, method->length,
                                         JS_CFUNC_generic_magic, method->handler_id);
    if (JS_IsException(method_func)) {
        return method_func;
    }
    
    // Define property
    int result = JS_DefinePropertyValueStr(ctx, obj, method->name, method_func,
                                          JS_PROP_WRITABLE | JS_PROP_CONFIGURABLE);
    if (result < 0) {
        JS_FreeValue(ctx, method_func);
        return JS_ThrowInternalError(ctx, "failed to bind method: %s", method->name);
    }
    
    return JS_UNDEFINED;
}

// BindAccessorToObject - Bind an accessor to a JavaScript object
JSValue BindAccessorToObject(JSContext *ctx, JSValue obj, const AccessorEntry *accessor) {
    JSAtom accessor_atom = JS_NewAtom(ctx, accessor->name);
    JSValue getter = JS_UNDEFINED;
    JSValue setter = JS_UNDEFINED;
    
    // Create getter
    if (accessor->getter_id != 0) {
        getter = CreateCFunction(ctx, accessor->name, 0,
                                JS_CFUNC_getter_magic, accessor->getter_id);
        if (JS_IsException(getter)) {
            JS_FreeAtom(ctx, accessor_atom);
            return getter;
        }
    }
    
    // Create setter
    if (accessor->setter_id != 0) {
        setter = CreateCFunction(ctx, accessor->name, 1,
                                JS_CFUNC_setter_magic, accessor->setter_id);
        if (JS_IsException(setter)) {
            JS_FreeAtom(ctx, accessor_atom);
            if (!JS_IsUndefined(getter)) {
                JS_FreeValue(ctx, getter);
            }
            return setter;
        }
    }
    
    // Define accessor
    int result = JS_DefinePropertyGetSet(ctx, obj, accessor_atom, getter, setter,
                                        JS_PROP_CONFIGURABLE);
    
    JS_FreeAtom(ctx, accessor_atom);
    
    if (result < 0) {
        if (!JS_IsUndefined(getter)) JS_FreeValue(ctx, getter);
        if (!JS_IsUndefined(setter)) JS_FreeValue(ctx, setter);
        return JS_ThrowInternalError(ctx, "failed to bind accessor: %s", accessor->name);
    }
    
    return JS_UNDEFINED;
}

// BindPropertyToObject - Bind a data property to a JavaScript object
// This creates real data properties using JS_DefinePropertyValueStr
JSValue BindPropertyToObject(JSContext *ctx, JSValue obj, const PropertyEntry *property) {
    // Duplicate the value to ensure proper reference counting
    // JS_DefinePropertyValueStr takes ownership of the value
    JSValue property_value = JS_DupValue(ctx, property->value);
    
    // Define the data property using QuickJS API
    // This creates a real data property, not an accessor property
    int result = JS_DefinePropertyValueStr(ctx, obj, property->name, 
                                          property_value, property->flags);
    
    if (result < 0) {
        // If define failed, we need to free the duplicated value
        JS_FreeValue(ctx, property_value);
        return JS_ThrowInternalError(ctx, "failed to bind property: %s", property->name);
    }
    
    // Success: property_value ownership has been transferred to QuickJS
    return JS_UNDEFINED;
}



// ============================================================================
// SCHEME C: CREATECLASSINSTANCE HELPER FUNCTION - MODIFIED
// ============================================================================

// CreateClassInstance - encapsulates the object creation logic for Scheme C
// MODIFIED FOR SCHEME C: This function now handles instance property binding
// This function handles:
// 1. Getting prototype from constructor
// 2. Creating JS object with correct prototype and class
// 3. Binding instance properties to the created object (NEW for Scheme C)
// 4. Error handling and cleanup
// 
// Note: Go object association is now handled by constructor proxy, not here
// Returns JS_EXCEPTION on any error, proper JSValue on success
// This corresponds to the logic in point.c example with instance property support
JSValue CreateClassInstance(JSContext *ctx, JSValue constructor, 
                           JSClassID class_id,
                           const PropertyEntry *instance_properties,
                           int instance_property_count) {
    JSValue proto, obj;

    // Check QuickJS limits
    if (class_id >= (1 << 16)) {
        return JS_ThrowRangeError(ctx, "class ID exceeds maximum value");
    }
    
    // Step 1: Get prototype from constructor 
    // Corresponds to point.c: proto = JS_GetPropertyStr(ctx, new_target, "prototype")
    proto = JS_GetPropertyStr(ctx, constructor, "prototype");
    if (JS_IsException(proto)) {
        // Return the exception directly, caller will handle cleanup
        return proto;
    }
    
    // Step 2: Create JS object with correct prototype and class
    // Corresponds to point.c: obj = JS_NewObjectProtoClass(ctx, proto, js_point_class_id)
    obj = JS_NewObjectProtoClass(ctx, proto, class_id);
    
    // Free prototype reference (always needed, regardless of obj creation result)
    JS_FreeValue(ctx, proto);
    
    if (JS_IsException(obj)) {
        // Return the exception directly, caller will handle cleanup
        return obj;
    }
    
    // Step 3: NEW FOR SCHEME C - Bind instance properties before constructor call
    // This ensures instance properties are available when the constructor function runs
    if (instance_properties && instance_property_count > 0) {
        for (int i = 0; i < instance_property_count; i++) {
            const PropertyEntry *property = &instance_properties[i];
            
            // Only process instance properties (static properties handled elsewhere)
            if (property->is_static == 0) {
                JSValue property_result = BindPropertyToObject(ctx, obj, property);
                if (JS_IsException(property_result)) {
                    // Clean up object on property binding failure
                    JS_FreeValue(ctx, obj);
                    return property_result;
                }
            }
        }
    }
    
    // Step 4: Return instance (Go object association handled by constructor proxy)
    // In Scheme C, the constructor proxy will call this function, then call the 
    // constructor function, and finally associate the returned Go object
    return obj;
}

// ============================================================================
// INTERRUPT HANDLERS
// ============================================================================

static void clearTimeoutState(JSRuntime *rt);

// Simplified interrupt handler (no handlerArgs complexity)
int interruptHandler(JSRuntime *rt, void *opaque) {
    JSRuntime *runtimePtr = (JSRuntime*)opaque;
    return goInterruptHandler(runtimePtr);
}

void SetInterruptHandler(JSRuntime *rt) {
    // Use rt itself as opaque parameter for Go lookup
    JS_SetInterruptHandler(rt, interruptHandler, (void*)rt);
    clearTimeoutState(rt);
}

void ClearInterruptHandler(JSRuntime *rt) {
    JS_SetInterruptHandler(rt, NULL, NULL);
    clearTimeoutState(rt);
}

// Timeout handler implementation (unchanged but improved cleanup)
typedef struct {
    time_t start;
    time_t timeout;
} TimeoutStruct;

typedef struct TimeoutStateEntry {
    JSRuntime *rt;
    TimeoutStruct *state;
    struct TimeoutStateEntry *next;
} TimeoutStateEntry;

static TimeoutStateEntry *g_timeout_states = NULL;
static pthread_mutex_t g_timeout_states_mu = PTHREAD_MUTEX_INITIALIZER;

static int isExecuteTimeoutExceeded(JSRuntime *rt) {
    time_t start = 0;
    time_t timeout = 0;
    int found = 0;

    pthread_mutex_lock(&g_timeout_states_mu);
    TimeoutStateEntry *current = g_timeout_states;
    while (current) {
        if (current->rt == rt && current->state != NULL) {
            start = current->state->start;
            timeout = current->state->timeout;
            found = 1;
            break;
        }
        current = current->next;
    }
    pthread_mutex_unlock(&g_timeout_states_mu);

    if (!found || timeout <= 0) {
        return 0;
    }

    return (time(NULL) - start) > timeout;
}
static TimeoutStruct *takeTimeoutState(JSRuntime *rt) {
    pthread_mutex_lock(&g_timeout_states_mu);

    TimeoutStateEntry *prev = NULL;
    TimeoutStateEntry *current = g_timeout_states;
    while (current) {
        if (current->rt == rt) {
            TimeoutStruct *state = current->state;
            if (prev) {
                prev->next = current->next;
            } else {
                g_timeout_states = current->next;
            }
            free(current);
            pthread_mutex_unlock(&g_timeout_states_mu);
            return state;
        }
        prev = current;
        current = current->next;
    }

    pthread_mutex_unlock(&g_timeout_states_mu);
    return NULL;
}

static int setTimeoutState(JSRuntime *rt, TimeoutStruct *state) {
    pthread_mutex_lock(&g_timeout_states_mu);

    TimeoutStateEntry *current = g_timeout_states;
    while (current) {
        if (current->rt == rt) {
            if (current->state != NULL && current->state != state) {
                free(current->state);
            }
            current->state = state;
            pthread_mutex_unlock(&g_timeout_states_mu);
            return 0;
        }
        current = current->next;
    }

    TimeoutStateEntry *entry = malloc(sizeof(TimeoutStateEntry));
    if (!entry) {
        pthread_mutex_unlock(&g_timeout_states_mu);
        return -1;
    }
    entry->rt = rt;
    entry->state = state;
    entry->next = g_timeout_states;
    g_timeout_states = entry;

    pthread_mutex_unlock(&g_timeout_states_mu);
    return 0;
}

static void clearTimeoutState(JSRuntime *rt) {
    TimeoutStruct *state = takeTimeoutState(rt);
    if (state) {
        free(state);
    }
}

int GetTimeoutOpaqueCount(void) {
    int count = 0;

    pthread_mutex_lock(&g_timeout_states_mu);
    TimeoutStateEntry *current = g_timeout_states;
    while (current) {
        if (current->state != NULL) {
            count++;
        }
        current = current->next;
    }
    pthread_mutex_unlock(&g_timeout_states_mu);

    return count;
}

int timeoutHandler(JSRuntime *rt, void *opaque) {
    TimeoutStruct* ts = (TimeoutStruct*)opaque;
    time_t timeout = ts->timeout;
    time_t start = ts->start;
    if (timeout <= 0) {
        return 0;
    }

    time_t now = time(NULL);
    if (now - start > timeout) {
        return 1;
    }

    return 0;
}

void SetExecuteTimeout(JSRuntime *rt, time_t timeout) {
    if (timeout <= 0) {
        JS_SetInterruptHandler(rt, NULL, NULL);
        clearTimeoutState(rt);
        return;
    }

    TimeoutStruct* ts = malloc(sizeof(TimeoutStruct));
    if (!ts) {
        JS_SetInterruptHandler(rt, NULL, NULL);
        clearTimeoutState(rt);
        return;
    }

    ts->start = time(NULL);
    ts->timeout = timeout;
    JS_SetInterruptHandler(rt, timeoutHandler, ts);

    if (setTimeoutState(rt, ts) != 0) {
        JS_SetInterruptHandler(rt, NULL, NULL);
        free(ts);
        clearTimeoutState(rt);
    }
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
            js_module_set_import_meta(ctx, obj, false, false);
        }
        return obj;
    } else {
        if (JS_VALUE_GET_TAG(obj) == JS_TAG_MODULE) {
            if (JS_ResolveModule(ctx, obj) < 0) {
                JS_FreeValue(ctx, obj);
                return JS_EXCEPTION;
            }
            js_module_set_import_meta(ctx, obj, false, false);
            val = JS_EvalFunction(ctx, obj);
            val = AwaitValue(ctx, val);
        } else {
            val = JS_EvalFunction(ctx, obj);
        }
        
        return val;
    }
}

// ============================================================================
// MODULE-RELATED FUNCTIONS - NEW FOR MODULE BUILDER
// ============================================================================


// CreateModule - encapsulates QuickJS module creation logic
// This function handles all the C API calls needed to create a JavaScript module:
// 1. Create C module with initialization function (JS_NewCModule)
// 2. Pre-declare all exports (JS_AddModuleExport)
// 3. Set module private value for initialization access (JS_SetModulePrivateValue)
// 
// Parameters:
// - ctx: JavaScript context
// - module_name: Module name (C string)
// - export_names: Array of export names (C strings)
// - export_count: Number of exports
// - builder_id: ModuleBuilder ID for initialization access
//
// Returns:
// - 0 on success
// - -1 on failure (JS exception will be set)
int CreateModule(JSContext *ctx, const char *module_name,
                const char **export_names, int export_count,
                int32_t builder_id) {
    JSModuleDef *module;
    JSValue builder_value;
    
    // Input validation
    if (!ctx || !module_name || !export_names) {
        JS_ThrowInternalError(ctx, "CreateModule: invalid parameters");
        return -1;
    }
    
    if (strlen(module_name) == 0) {
        JS_ThrowInternalError(ctx, "CreateModule: module name cannot be empty");
        return -1;
    }
    
    // Step 1: Create C module with initialization function
    // Corresponds to JS_NewCModule(ctx, module_name, GoModuleInitProxy)
    module = JS_NewCModule(ctx, module_name, GoModuleInitProxy);
    if (!module) {
        JS_ThrowInternalError(ctx, "CreateModule: failed to create C module: %s", module_name);
        return -1;
    }
    
    // Step 2: Pre-declare all exports (JS_AddModuleExport phase)
    // This must be done before module instantiation
    for (int i = 0; i < export_count; i++) {
        const char *export_name = export_names[i];
        
        // Validate export name
        if (!export_name || strlen(export_name) == 0) {
            JS_ThrowInternalError(ctx, "CreateModule: export name cannot be empty at index %d", i);
            return -1;
        }
        
        // Add module export declaration
        int result = JS_AddModuleExport(ctx, module, export_name);
        if (result < 0) {
            JS_ThrowInternalError(ctx, "CreateModule: failed to add module export: %s", export_name);
            return -1;
        }
    }
    
    // Step 3: Set module private value for initialization access
    // Create JSValue from builder_id for storage
    builder_value = JS_NewInt32(ctx, builder_id);
    if (JS_IsException(builder_value)) {
        JS_ThrowInternalError(ctx, "CreateModule: failed to create builder value");
        return -1;
    }
    
    // Store builder_id as module private value
    JS_SetModulePrivateValue(ctx, module, builder_value);
    
    // Success
    return 0;
}

// Module initialization proxy function - C wrapper for Go export
// This function serves as a bridge between QuickJS C API and Go ModuleBuilder functionality
// Called by QuickJS when a module is being initialized
// Corresponds to JSModuleInitFunc signature: int (*)(JSContext *ctx, JSModuleDef *m)
int GoModuleInitProxy(JSContext *ctx, JSModuleDef *m) {
    // Call the Go export function which handles the actual module initialization logic
    // The Go function will:
    // 1. Retrieve the ModuleBuilder from module private value
    // 2. Set all export values using JS_SetModuleExport
    // 3. Call user initialization function if provided
    // 4. Handle error cases and resource cleanup
    return goModuleInitProxy(ctx, m);
}