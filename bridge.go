package quickjs

import (
	"sync"
	"unsafe"
)

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

var (
	// Global context mapping using sync.Map for lock-free performance
	contextMapping sync.Map // map[*C.JSContext]*Context

	// Global runtime mapping for interrupt handler access
	runtimeMapping sync.Map // map[*C.JSRuntime]*Runtime
)

// =============================================================================
// CONTEXT AND RUNTIME MAPPING FUNCTIONS
// =============================================================================

// registerContext registers Go Context with C JSContext (internal use)
func registerContext(cCtx *C.JSContext, goCtx *Context) {
	contextMapping.Store(cCtx, goCtx)
}

// unregisterContext removes mapping when Context is closed (internal use)
func unregisterContext(cCtx *C.JSContext) {
	contextMapping.Delete(cCtx)
}

// clearContextMapping clears all registered contexts (internal use)
func clearContextMapping() {
	// Clear all mappings
	contextMapping.Range(func(key, value interface{}) bool {
		contextMapping.Delete(key)
		return true // continue iteration
	})
}

// getContextFromJS gets Go Context from C JSContext (internal use)
func getContextFromJS(cCtx *C.JSContext) *Context {
	if value, ok := contextMapping.Load(cCtx); ok {
		return value.(*Context)
	}
	return nil
}

// registerRuntime registers Runtime for interrupt handler access
func registerRuntime(cRt *C.JSRuntime, goRt *Runtime) {
	runtimeMapping.Store(cRt, goRt)
}

// unregisterRuntime removes Runtime mapping when closed
func unregisterRuntime(cRt *C.JSRuntime) {
	runtimeMapping.Delete(cRt)
}

// getRuntimeFromJS gets Go Runtime from C JSRuntime (internal use)
func getRuntimeFromJS(cRt *C.JSRuntime) *Runtime {
	if value, ok := runtimeMapping.Load(cRt); ok {
		return value.(*Runtime)
	}
	return nil
}

// =============================================================================
// COMMON HELPER FUNCTIONS
// =============================================================================

// convertCArgsToGoValues converts C arguments to Go Value slice (unified helper)
// Reused by all proxy functions for consistent parameter conversion
// Note: Works with both JSValue and JSValueConst since we only read values
func convertCArgsToGoValues(argc C.int, argv *C.JSValue, ctx *Context) []Value {
	if argc == 0 {
		return nil
	}

	// Use unsafe.Slice to convert C array to Go slice (Go 1.17+)
	cArgs := unsafe.Slice(argv, int(argc))
	goArgs := make([]Value, int(argc))

	for i, cArg := range cArgs {
		goArgs[i] = Value{ctx: ctx, ref: cArg}
	}

	return goArgs
}

// =============================================================================
// COMMON PROXY HELPER FUNCTIONS
// =============================================================================

// proxyError represents a standardized error for proxy functions
type proxyError struct {
	errorType string
	message   string
}

// Common proxy errors with consistent error messages
var (
	errContextNotFound        = proxyError{"InternalError", "Context not found"}
	errFunctionNotFound       = proxyError{"InternalError", "Function not found"}
	errConstructorNotFound    = proxyError{"InternalError", "Constructor function not found"}
	errMethodNotFound         = proxyError{"InternalError", "Method function not found"}
	errGetterNotFound         = proxyError{"InternalError", "Getter function not found"}
	errSetterNotFound         = proxyError{"InternalError", "Setter function not found"}
	errInvalidFunctionType    = proxyError{"TypeError", "Invalid function type"}
	errInvalidConstructorType = proxyError{"TypeError", "Invalid constructor function type"}
	errInvalidMethodType      = proxyError{"TypeError", "Invalid method function type"}
	errInvalidGetterType      = proxyError{"TypeError", "Invalid getter function type"}
	errInvalidSetterType      = proxyError{"TypeError", "Invalid setter function type"}
)

// throwProxyError creates and returns a JavaScript error
func throwProxyError(ctx *C.JSContext, err proxyError) C.JSValue {
	msg := C.CString(err.message)
	defer C.free(unsafe.Pointer(msg))

	switch err.errorType {
	case "TypeError":
		return C.ThrowTypeError(ctx, msg)
	default:
		return C.ThrowInternalError(ctx, msg)
	}
}

// getContextAndFunction retrieves context and function from HandleStore
// Returns (context, function, error). If error is not nil, caller should return throwProxyError(ctx, error)
func getContextAndFunction(ctx *C.JSContext, magic C.int, notFoundErr proxyError) (*Context, interface{}, *proxyError) {
	// Get Go Context from global mapping
	goCtx := getContextFromJS(ctx)
	if goCtx == nil {
		return nil, nil, &errContextNotFound
	}

	// Get function from HandleStore using magic parameter
	funcID := int32(magic)
	fn := goCtx.loadFunctionFromHandleID(funcID)
	if fn == nil {
		return nil, nil, &notFoundErr
	}

	return goCtx, fn, nil
}

// =============================================================================
// INTERRUPT HANDLER
// =============================================================================

// Simplified interrupt handler export (no cgo.Handle complexity)
//
//export goInterruptHandler
func goInterruptHandler(runtimePtr *C.JSRuntime) C.int {
	// Get Runtime from mapping instead of unsafe handle operations
	runtime := getRuntimeFromJS(runtimePtr)
	if runtime == nil {
		return C.int(0) // Runtime not found, no interrupt
	}

	r := runtime.callInterruptHandler()

	return C.int(r)
}

// =============================================================================
// OPTIMIZED PROXY FUNCTIONS
// =============================================================================

// New efficient proxy function for regular functions using HandleStore
//
//export goFunctionProxy
func goFunctionProxy(ctx *C.JSContext, thisVal C.JSValueConst,
	argc C.int, argv *C.JSValueConst, magic C.int) C.JSValue {

	// Get context and function using common helper
	goCtx, fn, err := getContextAndFunction(ctx, magic, errFunctionNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	// Type assertion to function signature
	goFn, ok := fn.(func(*Context, Value, []Value) Value)
	if !ok {
		return throwProxyError(ctx, errInvalidFunctionType)
	}

	// Convert arguments and call function
	args := convertCArgsToGoValues(argc, (*C.JSValue)(argv), goCtx)
	result := goFn(goCtx, Value{ctx: goCtx, ref: thisVal}, args)
	return result.ref
}

// Class constructor proxy - MODIFIED FOR SCHEME C
// Handles new_target for inheritance support and implements Scheme C logic:
// 1. Gets ClassBuilder from handleStore (not individual constructor function)
// 2. Extracts instance properties from ClassBuilder
// 3. Creates pre-configured instance with bound properties
// 4. Calls constructor function with pre-created instance
// 5. Associates returned Go object with the instance
// Corresponds to QuickJS JSCFunctionType.constructor_magic
//
//export goClassConstructorProxy
func goClassConstructorProxy(ctx *C.JSContext, newTarget C.JSValue,
	argc C.int, argv *C.JSValue, magic C.int) C.JSValue {

	// Get context and ClassBuilder using common helper
	goCtx, fn, perr := getContextAndFunction(ctx, magic, errConstructorNotFound)
	if perr != nil {
		return throwProxyError(ctx, *perr)
	}

	// Type assertion to ClassBuilder (SCHEME C: stored entire ClassBuilder, not just constructor)
	builder, ok := fn.(*ClassBuilder)
	if !ok {
		return throwProxyError(ctx, errInvalidConstructorType)
	}

	// Extract class ID from newTarget for instance creation
	classID, exists := getConstructorClassID(newTarget)
	if !exists {
		// This should not happen in normal cases since we register constructors
		// But provide fallback for defensive programming
		// return throwProxyError(ctx, proxyError{"InternalError", "Class ID not found for constructor"})
		v := Value{ctx: goCtx, ref: newTarget}
		classID, exists = v.resolveClassIDFromInheritance()
	}
	if !exists {
		return throwProxyError(ctx, proxyError{"InternalError", "Class ID not found for constructor"})
	}

	// SCHEME C STEP 1: Extract instance properties from ClassBuilder
	var instanceProperties []C.PropertyEntry
	var instancePropertyNames []*C.char // Track C strings for cleanup

	for _, property := range builder.properties {
		// Only include instance properties (not static properties)
		if !property.Static {
			// Convert property name to C string
			propertyName := C.CString(property.Name)
			instancePropertyNames = append(instancePropertyNames, propertyName)

			// Create C property entry for instance property
			instanceProperties = append(instanceProperties, C.PropertyEntry{
				name:      propertyName,
				value:     property.Value.ref,
				is_static: C.int(0), // Always instance property
				flags:     C.int(property.Flags),
			})
		}
	}

	// Prepare C array pointer for instance properties
	var instancePropertiesPtr *C.PropertyEntry
	if len(instanceProperties) > 0 {
		instancePropertiesPtr = &instanceProperties[0]
	}

	// SCHEME C STEP 2: Create instance with bound properties using modified CreateClassInstance
	instance := C.CreateClassInstance(
		ctx,
		newTarget,
		C.JSClassID(classID),
		instancePropertiesPtr,
		C.int(len(instanceProperties)),
	)

	// Clean up C strings after CreateClassInstance call
	for _, cStr := range instancePropertyNames {
		C.free(unsafe.Pointer(cStr))
	}

	// Check if instance creation failed
	if C.JS_IsException(instance) != 0 {
		return instance // Return the exception
	}

	// SCHEME C STEP 3: Call constructor function with pre-created instance
	// Constructor receives the pre-created instance and returns Go object to associate
	instanceValue := Value{ctx: goCtx, ref: instance}
	args := convertCArgsToGoValues(argc, argv, goCtx)
	goObj, err := builder.constructor(goCtx, instanceValue, args)
	if err != nil {
		C.JS_FreeValue(ctx, instance)
		errorMsg := C.CString(err.Error())
		defer C.free(unsafe.Pointer(errorMsg))
		return C.ThrowInternalError(ctx, errorMsg)
	}

	// SCHEME C STEP 4: Associate Go object with instance if constructor returned non-nil object
	if goObj != nil {
		handleID := goCtx.handleStore.Store(goObj)
		C.JS_SetOpaque(instance, C.IntToOpaque(C.int32_t(handleID)))
	}

	return instance
}

// Class method proxy - handles both instance and static methods
// Corresponds to QuickJS JSCFunctionType.generic_magic
//
//export goClassMethodProxy
func goClassMethodProxy(ctx *C.JSContext, thisVal C.JSValue,
	argc C.int, argv *C.JSValue, magic C.int) C.JSValue {

	// Get context and method using common helper
	goCtx, fn, err := getContextAndFunction(ctx, magic, errMethodNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	// Type assertion to ClassMethodFunc signature
	method, ok := fn.(ClassMethodFunc)
	if !ok {
		return throwProxyError(ctx, errInvalidMethodType)
	}

	// Convert parameters and call method
	thisValue := Value{ctx: goCtx, ref: thisVal}
	args := convertCArgsToGoValues(argc, argv, goCtx)
	result := method(goCtx, thisValue, args)
	return result.ref
}

// Class property getter proxy
// Corresponds to QuickJS JSCFunctionType.getter_magic
//
//export goClassGetterProxy
func goClassGetterProxy(ctx *C.JSContext, thisVal C.JSValue, magic C.int) C.JSValue {

	// Get context and getter using common helper
	goCtx, fn, err := getContextAndFunction(ctx, magic, errGetterNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	// Type assertion to ClassGetterFunc signature
	getter, ok := fn.(ClassGetterFunc)
	if !ok {
		return throwProxyError(ctx, errInvalidGetterType)
	}

	// Call getter with this value only
	thisValue := Value{ctx: goCtx, ref: thisVal}
	result := getter(goCtx, thisValue)
	return result.ref
}

// Class property setter proxy
// Corresponds to QuickJS JSCFunctionType.setter_magic
//
//export goClassSetterProxy
func goClassSetterProxy(ctx *C.JSContext, thisVal C.JSValue,
	val C.JSValue, magic C.int) C.JSValue {

	// Get context and setter using common helper
	goCtx, fn, err := getContextAndFunction(ctx, magic, errSetterNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	// Type assertion to ClassSetterFunc signature
	setter, ok := fn.(ClassSetterFunc)
	if !ok {
		return throwProxyError(ctx, errInvalidSetterType)
	}

	// Call setter with this value and new value
	thisValue := Value{ctx: goCtx, ref: thisVal}
	setValue := Value{ctx: goCtx, ref: val}
	result := setter(goCtx, thisValue, setValue)
	return result.ref
}

// Class finalizer proxy - unified cleanup handler
// Corresponds to QuickJS JSClassDef.finalizer
// Called automatically when JS object is garbage collected
//
//export goClassFinalizerProxy
func goClassFinalizerProxy(rt *C.JSRuntime, val C.JSValue) {
	// Get class ID for the object being finalized
	classID := C.JS_GetClassID(val)

	// Get opaque data from JS object using JS_GetOpaque (like point.c finalizer)
	// This corresponds to point.c: s = JS_GetOpaque(val, js_point_class_id)
	// Note: JS_GetOpaque only needs val and class_id (no context required in finalizer)
	opaque := C.JS_GetOpaque(val, classID)
	if opaque == nil {
		return // Corresponds to point.c: 's' can be NULL
	}

	// Use C helper function to safely convert opaque pointer back to int32
	handleID := int32(C.OpaqueToInt(opaque))

	// Get Context from runtime mapping
	// Note: We need to find the Context that owns this object
	// Since finalizer is called at runtime level, we iterate through contexts
	var targetCtx *Context
	contextMapping.Range(func(key, value interface{}) bool {
		ctx := value.(*Context)
		if ctx.runtime.ref == rt {
			// Check if this context has the handle
			if _, exists := ctx.handleStore.Load(handleID); exists {
				targetCtx = ctx
				return false // Stop iteration
			}
		}
		return true // Continue iteration
	})

	if targetCtx == nil {
		return // Context not found or handle already cleaned
	}

	// Get Go object from HandleStore
	if goObj, exists := targetCtx.handleStore.Load(handleID); exists {
		// Check if object implements optional ClassFinalizer interface
		if finalizer, ok := goObj.(ClassFinalizer); ok {
			// Call user-defined cleanup method
			func() {
				defer func() {
					// Silently recover to prevent GC crashes
					recover()
				}()
				finalizer.Finalize()
			}()
		}

		// Always clean up HandleStore reference (corresponds to point.c: js_free_rt)
		targetCtx.handleStore.Delete(handleID)
	}
}
