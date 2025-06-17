#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <time.h>
#include "quickjs.h"
#include "quickjs-libc.h"

// Value creation functions
extern JSValue JS_NewNull();
extern JSValue JS_NewUndefined();
extern JSValue JS_NewUninitialized();
extern JSValue JS_NewException();
extern JSValue JS_NewTrue();
extern JSValue JS_NewFalse();

// Error throwing functions
extern JSValue ThrowSyntaxError(JSContext *ctx, const char *fmt);
extern JSValue ThrowTypeError(JSContext *ctx, const char *fmt);
extern JSValue ThrowReferenceError(JSContext *ctx, const char *fmt);
extern JSValue ThrowRangeError(JSContext *ctx, const char *fmt);
extern JSValue ThrowInternalError(JSContext *ctx, const char *fmt);

// Type checking functions
extern int JS_IsNumber_Wrapper(JSValue val);
extern int JS_IsBigInt_Wrapper(JSContext *ctx, JSValue val);
extern int JS_IsBool_Wrapper(JSValue val);
extern int JS_IsNull_Wrapper(JSValue val);
extern int JS_IsUndefined_Wrapper(JSValue val);
extern int JS_IsException_Wrapper(JSValue val);
extern int JS_IsUninitialized_Wrapper(JSValue val);
extern int JS_IsString_Wrapper(JSValue val);
extern int JS_IsSymbol_Wrapper(JSValue val);
extern int JS_IsObject_Wrapper(JSValue val);

// Constant getters
extern int GetPropertyWritableConfigurable();
extern int GetPropertyConfigurable();
extern int GetPropertyWritable();
extern int GetPropertyEnumerable();
extern int GetPropertyDefault();
extern int GetTypedArrayInt8();
extern int GetTypedArrayUint8();
extern int GetTypedArrayUint8C();
extern int GetTypedArrayInt16();
extern int GetTypedArrayUint16();
extern int GetTypedArrayInt32();
extern int GetTypedArrayUint32();
extern int GetTypedArrayFloat32();
extern int GetTypedArrayFloat64();
extern int GetTypedArrayBigInt64();
extern int GetTypedArrayBigUint64();
extern int GetEvalTypeGlobal();
extern int GetEvalTypeModule();
extern int GetEvalFlagStrict();
extern int GetEvalFlagCompileOnly();
extern int GetReadObjBytecode();
extern int GetWriteObjBytecode();
extern int GetCFuncGeneric();
extern int GetCFuncGenericMagic();
extern int GetCFuncConstructor();
extern int GetCFuncConstructorMagic();
extern int GetCFuncGetterMagic();
extern int GetCFuncSetterMagic();
extern int GetPromisePending();
extern int GetPromiseFulfilled();
extern int GetPromiseRejected();
extern int GetInvalidClassID();

// Helper functions
extern void* IntToOpaque(int32_t id);
extern int32_t OpaqueToInt(void* opaque);
extern int ValueGetTag(JSValueConst v);
extern void* JS_VALUE_GET_PTR_Wrapper(JSValue val); 
extern int JS_DeletePropertyInt64(JSContext *ctx, JSValueConst obj, int64_t idx, int flags);

// Class creation structures
// Method configuration structure
typedef struct {
    const char *name;
    int32_t handler_id;
    int length;
    int is_static;
} MethodEntry;

// Accessor configuration structure
typedef struct {
    const char *name;
    int32_t getter_id;
    int32_t setter_id;
    int is_static;
} AccessorEntry;

// Property configuration structure for data properties
typedef struct {
    const char *name;
    JSValue value;
    int is_static;
    int flags;
} PropertyEntry;

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

// Accessor getter proxy
extern JSValue GoClassGetterProxy(JSContext *ctx, JSValueConst this_val, int magic);

// Accessor setter proxy  
extern JSValue GoClassSetterProxy(JSContext *ctx, JSValueConst this_val, 
                                 JSValueConst val, int magic);

// Finalizer proxy - unified cleanup handler
extern void GoClassFinalizerProxy(JSRuntime *rt, JSValue val);


// Complete class creation function
// C function allocates class_id internally and returns it via pointer
// Go layer manages JSClassDef and class name memory
extern JSValue CreateClass(JSContext *ctx,
                          JSClassID *class_id,        // C function allocates internally
                          JSClassDef *class_def,      // Go layer manages memory
                          int32_t constructor_id,
                          const MethodEntry *methods, int method_count,
                          const AccessorEntry *accessors, int accessor_count,
                          const PropertyEntry *properties, int property_count);

// MODIFIED FOR SCHEME C: CreateClassInstance with instance property binding support
// This function handles the instance creation and property binding for Scheme C:
// 1. Getting prototype from constructor
// 2. Creating JS object with correct prototype and class
// 3. Binding instance properties to the created object (NEW)
// 4. Error handling and cleanup
// 
// Note: Go object association is now handled by constructor proxy, not here
// Returns JS_EXCEPTION on any error, proper JSValue on success
extern JSValue CreateClassInstance(JSContext *ctx, JSValue constructor, 
                                  JSClassID class_id,
                                  const PropertyEntry *instance_properties,
                                  int instance_property_count);

// CreateCFunction - encapsulates C function creation logic
// Parameters match JS_NewCFunction2: ctx, name, length, cproto, magic
// Returns JS_EXCEPTION on any error, proper JSValue on success
extern JSValue CreateCFunction(JSContext *ctx, const char *name, 
                              int length, int func_type, int32_t handler_id);


extern int ValueGetTag(JSValueConst v);
extern JSValue LoadModuleBytecode(JSContext *ctx, const uint8_t *buf, size_t buf_len, int load_only);

// Simplified interrupt handler interface (no handlerArgs complexity)
extern void SetInterruptHandler(JSRuntime *rt);
extern void ClearInterruptHandler(JSRuntime *rt);
extern void SetExecuteTimeout(JSRuntime *rt, time_t timeout);