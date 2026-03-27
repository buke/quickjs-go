package quickjs

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

	moduleInitPanicHookMu                 sync.RWMutex
	moduleInitPanicHookForTest            func(ctx *Context, builder *ModuleBuilder)
	forceInvalidModulePrivateValueForTest atomic.Bool
	forceInvalidModuleBuilderTypeForTest  atomic.Bool
	forceClassOpaqueAllocFailureForTest   atomic.Bool
)

type finalizerObservabilitySnapshot struct {
	Enabled             bool
	OpaqueNil           uint64
	OpaqueInvalid       uint64
	HandleInvalid       uint64
	ContextRefInvalid   uint64
	ContextNotFound     uint64
	ContextStateInvalid uint64
	RuntimeMismatch     uint64
	HandleMissing       uint64
	Cleaned             uint64
}

type finalizerObservabilityCounters struct {
	enabled             atomic.Bool
	opaqueNil           atomic.Uint64
	opaqueInvalid       atomic.Uint64
	handleInvalid       atomic.Uint64
	contextRefInvalid   atomic.Uint64
	contextNotFound     atomic.Uint64
	contextStateInvalid atomic.Uint64
	runtimeMismatch     atomic.Uint64
	handleMissing       atomic.Uint64
	cleaned             atomic.Uint64
}

var finalizerObservabilityByRuntime sync.Map // map[uintptr]*finalizerObservabilityCounters

const (
	finalizerCounterOpaqueNil = iota
	finalizerCounterOpaqueInvalid
	finalizerCounterHandleInvalid
	finalizerCounterContextRefInvalid
	finalizerCounterContextNotFound
	finalizerCounterContextStateInvalid
	finalizerCounterRuntimeMismatch
	finalizerCounterHandleMissing
	finalizerCounterCleaned
)

func runtimeKeyFromCRef(rt *C.JSRuntime) uintptr {
	if rt == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(rt))
}

func getFinalizerObservabilityCounters(rt *C.JSRuntime, create bool) *finalizerObservabilityCounters {
	key := runtimeKeyFromCRef(rt)
	if key == 0 {
		return nil
	}

	if !create {
		raw, ok := finalizerObservabilityByRuntime.Load(key)
		if !ok {
			return nil
		}
		counters, ok := raw.(*finalizerObservabilityCounters)
		if !ok {
			finalizerObservabilityByRuntime.Delete(key)
			return nil
		}
		return counters
	}

	raw, _ := finalizerObservabilityByRuntime.LoadOrStore(key, &finalizerObservabilityCounters{})
	counters, ok := raw.(*finalizerObservabilityCounters)
	if !ok {
		finalizerObservabilityByRuntime.Delete(key)
		return nil
	}
	return counters
}

func observeFinalizerCounter(rt *C.JSRuntime, counterType int) {
	counters := getFinalizerObservabilityCounters(rt, false)
	if counters == nil {
		return
	}
	if !counters.enabled.Load() {
		return
	}

	switch counterType {
	case finalizerCounterOpaqueNil:
		counters.opaqueNil.Add(1)
	case finalizerCounterOpaqueInvalid:
		counters.opaqueInvalid.Add(1)
	case finalizerCounterHandleInvalid:
		counters.handleInvalid.Add(1)
	case finalizerCounterContextRefInvalid:
		counters.contextRefInvalid.Add(1)
	case finalizerCounterContextNotFound:
		counters.contextNotFound.Add(1)
	case finalizerCounterContextStateInvalid:
		counters.contextStateInvalid.Add(1)
	case finalizerCounterRuntimeMismatch:
		counters.runtimeMismatch.Add(1)
	case finalizerCounterHandleMissing:
		counters.handleMissing.Add(1)
	case finalizerCounterCleaned:
		counters.cleaned.Add(1)
	}
}

func resetFinalizerObservabilityForTest(r *Runtime) {
	if r == nil || r.ref == nil {
		return
	}
	counters := getFinalizerObservabilityCounters(r.ref, true)
	if counters == nil {
		return
	}

	counters.opaqueNil.Store(0)
	counters.opaqueInvalid.Store(0)
	counters.handleInvalid.Store(0)
	counters.contextRefInvalid.Store(0)
	counters.contextNotFound.Store(0)
	counters.contextStateInvalid.Store(0)
	counters.runtimeMismatch.Store(0)
	counters.handleMissing.Store(0)
	counters.cleaned.Store(0)
}

func setFinalizerObservabilityForTest(r *Runtime, enabled bool) {
	if r == nil || r.ref == nil {
		return
	}
	counters := getFinalizerObservabilityCounters(r.ref, true)
	if counters == nil {
		return
	}
	counters.enabled.Store(enabled)
}

func snapshotFinalizerObservabilityForTest(r *Runtime) finalizerObservabilitySnapshot {
	if r == nil || r.ref == nil {
		return finalizerObservabilitySnapshot{}
	}
	counters := getFinalizerObservabilityCounters(r.ref, false)
	if counters == nil {
		return finalizerObservabilitySnapshot{}
	}

	return finalizerObservabilitySnapshot{
		Enabled:             counters.enabled.Load(),
		OpaqueNil:           counters.opaqueNil.Load(),
		OpaqueInvalid:       counters.opaqueInvalid.Load(),
		HandleInvalid:       counters.handleInvalid.Load(),
		ContextRefInvalid:   counters.contextRefInvalid.Load(),
		ContextNotFound:     counters.contextNotFound.Load(),
		ContextStateInvalid: counters.contextStateInvalid.Load(),
		RuntimeMismatch:     counters.runtimeMismatch.Load(),
		HandleMissing:       counters.handleMissing.Load(),
		Cleaned:             counters.cleaned.Load(),
	}
}

func snapshotAndResetFinalizerObservabilityForTest(r *Runtime) finalizerObservabilitySnapshot {
	snapshot := snapshotFinalizerObservabilityForTest(r)
	resetFinalizerObservabilityForTest(r)
	return snapshot
}

func clearFinalizerObservabilityForRuntime(rt *C.JSRuntime) {
	key := runtimeKeyFromCRef(rt)
	if key == 0 {
		return
	}
	finalizerObservabilityByRuntime.Delete(key)
}

func resetClassOpaqueCountersForTest() {
	C.ResetClassOpaqueCountersForTest()
}

func currentClassOpaqueAllocationCount() int {
	return int(C.GetClassOpaqueAllocationCount())
}

func currentClassOpaqueFreeCount() int {
	return int(C.GetClassOpaqueFreeCount())
}

func currentClassOpaqueOutstandingCount() int {
	return int(C.GetClassOpaqueOutstandingCount())
}

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

// getContextFromJS gets Go Context from C JSContext (internal use)
func getContextFromJS(cCtx *C.JSContext) *Context {
	if value, ok := contextMapping.Load(cCtx); ok {
		goCtx, castOK := value.(*Context)
		if !castOK {
			contextMapping.Delete(cCtx)
			return nil
		}
		return goCtx
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
	clearFinalizerObservabilityForRuntime(cRt)
}

// getRuntimeFromJS gets Go Runtime from C JSRuntime (internal use)
func getRuntimeFromJS(cRt *C.JSRuntime) *Runtime {
	if value, ok := runtimeMapping.Load(cRt); ok {
		goRt, castOK := value.(*Runtime)
		if !castOK {
			runtimeMapping.Delete(cRt)
			return nil
		}
		return goRt
	}
	return nil
}

// =============================================================================
// COMMON HELPER FUNCTIONS
// =============================================================================

// convertCArgsToGoValues converts C arguments to Go Value slice (unified helper)
// Reused by all proxy functions for consistent parameter conversion
// Note: Works with both JSValue and JSValueConst since we only read values
// MODIFIED: Returns []*Value instead of []Value
func convertCArgsToGoValues(argc C.int, argv *C.JSValue, ctx *Context) []*Value {
	if argc == 0 {
		return nil
	}

	// Use unsafe.Slice to convert C array to Go slice (Go 1.17+)
	cArgs := unsafe.Slice(argv, int(argc))
	goArgs := make([]*Value, int(argc))

	for i, cArg := range cArgs {
		goArgs[i] = &Value{ctx: ctx, ref: cArg}
	}

	return goArgs
}

// normalizeProxyResult prevents nil callback returns from panicking in proxy paths.
// Returning undefined keeps behavior predictable for Go callbacks that intentionally
// don't produce a value.
func normalizeProxyResult(result *Value) C.JSValue {
	if result == nil {
		return C.JS_NewUndefined()
	}
	return result.ref
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
	errHandleStoreUnavailable = proxyError{"InternalError", "HandleStore unavailable"}
	errFunctionNotFound       = proxyError{"InternalError", "Function not found"}
	errConstructorNotFound    = proxyError{"InternalError", "Constructor function not found"}
	errMethodNotFound         = proxyError{"InternalError", "Method function not found"}
	errGetterNotFound         = proxyError{"InternalError", "Getter function not found"}
	errSetterNotFound         = proxyError{"InternalError", "Setter function not found"}
	errInvalidFunctionType    = proxyError{"TypeError", "Invalid function type"}
	errInvalidConstructorType = proxyError{"TypeError", "Invalid constructor function type"}
	errConstructorIsNil       = proxyError{"InternalError", "Constructor function is nil"}
	errInvalidInstanceProp    = proxyError{"InternalError", "Invalid instance property value"}
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
func getContextAndObject(ctx *C.JSContext, magic C.int, notFoundErr proxyError) (*Context, interface{}, *proxyError) {
	// Get Go Context from global mapping
	goCtx := getContextFromJS(ctx)
	if goCtx == nil {
		return nil, nil, &errContextNotFound
	}

	// Get function from HandleStore using magic parameter.
	fn, perr := loadObjectByHandleID(goCtx, int32(magic), notFoundErr)
	if perr != nil {
		return nil, nil, perr
	}

	return goCtx, fn, nil
}

func loadObjectByHandleID(goCtx *Context, handleID int32, notFoundErr proxyError) (interface{}, *proxyError) {
	if goCtx == nil {
		return nil, &errContextNotFound
	}
	if goCtx.handleStore == nil {
		return nil, &errHandleStoreUnavailable
	}
	if handleID <= 0 {
		return nil, &notFoundErr
	}

	fn := goCtx.loadFunctionFromHandleID(handleID)
	if fn == nil {
		return nil, &notFoundErr
	}

	return fn, nil
}

// =============================================================================
// MODULE-RELATED HELPER FUNCTIONS
// =============================================================================

// getContextAndModuleBuilder retrieves context and ModuleBuilder from module private value
func getContextAndModuleBuilder(ctx *C.JSContext, m *C.JSModuleDef) (*Context, *ModuleBuilder, int32, error) {

	// Retrieve ModuleBuilder from module private value
	privateValue := C.JS_GetModulePrivateValue(ctx, m)

	// Extract ModuleBuilder ID from private value using JS_ToInt32
	var builderID C.int32_t
	toInt32Result := C.JS_ToInt32(ctx, &builderID, privateValue)
	if forceInvalidModulePrivateValueForTest.Load() {
		toInt32Result = -1
	}
	if toInt32Result < 0 {
		C.JS_FreeValue(ctx, privateValue)
		return nil, nil, 0, errors.New("invalid module private value")
	}
	C.JS_FreeValue(ctx, privateValue)

	goCtx, builderInterface, err := getContextAndObject(ctx, C.int(builderID), errFunctionNotFound)
	if err != nil {
		return getContextFromJS(ctx), nil, int32(builderID), fmt.Errorf("Failed to get context and ModuleBuilder: %v", err.message)
	}
	if forceInvalidModuleBuilderTypeForTest.Load() {
		builderInterface = "invalid-builder-type"
	}

	// Type assertion to ModuleBuilder
	builder, ok := builderInterface.(*ModuleBuilder)
	if !ok || builder == nil {
		return goCtx, nil, int32(builderID), errors.New("invalid module builder")
	}

	return goCtx, builder, int32(builderID), nil
}

// throwModuleError creates and throws a module initialization error
func throwModuleError(ctx *C.JSContext, err error) C.int {
	errorMsg := C.CString(err.Error())
	C.ThrowInternalError(ctx, errorMsg)
	C.free(unsafe.Pointer(errorMsg))
	return C.int(-1)
}

func panicMessage(value interface{}) string {
	return fmt.Sprintf("panic in Go callback: %v", value)
}

func recoverToJSInternalError(ctx *C.JSContext, ret *C.JSValue) {
	if rec := recover(); rec != nil {
		if ctx == nil {
			*ret = C.JS_NewException()
			return
		}
		msg := C.CString(panicMessage(rec))
		defer C.free(unsafe.Pointer(msg))
		*ret = C.ThrowInternalError(ctx, msg)
	}
}

func recoverToModuleError(ctx *C.JSContext, ret *C.int) {
	if rec := recover(); rec != nil {
		if ctx != nil {
			msg := C.CString(panicMessage(rec))
			C.ThrowInternalError(ctx, msg)
			C.free(unsafe.Pointer(msg))
		}
		*ret = C.int(-1)
	}
}

func recoverToInterruptNoop(ret *C.int) {
	if recover() != nil {
		*ret = C.int(0)
	}
}

func recoverFinalizerPanic() {
	_ = recover()
}

func triggerRecoverToJSInternalErrorNilContextForTest() bool {
	ret := C.JS_NewUndefined()
	func() {
		defer recoverToJSInternalError(nil, &ret)
		panic("bridge-recover-test")
	}()
	return C.JS_IsException_Wrapper(ret) == 1
}

func triggerRecoverToModuleErrorNilContextForTest() int {
	ret := C.int(1)
	func() {
		defer recoverToModuleError(nil, &ret)
		panic("bridge-module-recover-test")
	}()
	return int(ret)
}

func triggerRecoverToInterruptNoopForTest(initial int, shouldPanic bool) int {
	ret := C.int(initial)
	func() {
		defer recoverToInterruptNoop(&ret)
		if shouldPanic {
			panic("bridge-interrupt-recover-test")
		}
	}()
	return int(ret)
}

func setModuleInitPanicHookForTest(hook func(ctx *Context, builder *ModuleBuilder)) {
	moduleInitPanicHookMu.Lock()
	moduleInitPanicHookForTest = hook
	moduleInitPanicHookMu.Unlock()
}

func setForceInvalidModulePrivateValueForTest(enabled bool) {
	forceInvalidModulePrivateValueForTest.Store(enabled)
}

func setForceInvalidModuleBuilderTypeForTest(enabled bool) {
	forceInvalidModuleBuilderTypeForTest.Store(enabled)
}

func setForceClassOpaqueAllocFailureForTest(enabled bool) {
	forceClassOpaqueAllocFailureForTest.Store(enabled)
}

func invokeModuleInitPanicHookForTest(ctx *Context, builder *ModuleBuilder) {
	moduleInitPanicHookMu.RLock()
	hook := moduleInitPanicHookForTest
	moduleInitPanicHookMu.RUnlock()
	if hook != nil {
		hook(ctx, builder)
	}
}

// =============================================================================
// INTERRUPT HANDLER
// =============================================================================

// Simplified interrupt handler export (no cgo.Handle complexity)
//
//export goInterruptHandler
func goInterruptHandler(runtimePtr *C.JSRuntime) (ret C.int) {
	defer recoverToInterruptNoop(&ret)

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
// MODIFIED: Function signature changed to use *Value
//
//export goFunctionProxy
func goFunctionProxy(ctx *C.JSContext, thisVal C.JSValueConst,
	argc C.int, argv *C.JSValueConst, magic C.int) (ret C.JSValue) {
	defer recoverToJSInternalError(ctx, &ret)

	// Get context and function using common helper
	goCtx, fn, err := getContextAndObject(ctx, magic, errFunctionNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	// Type assertion to function signature - MODIFIED: now uses pointers
	goFn, ok := fn.(func(*Context, *Value, []*Value) *Value)
	if !ok {
		return throwProxyError(ctx, errInvalidFunctionType)
	}

	// Convert arguments and call function - MODIFIED: now uses pointer conversion
	args := convertCArgsToGoValues(argc, (*C.JSValue)(argv), goCtx)
	thisValue := &Value{ctx: goCtx, ref: thisVal}
	result := goFn(goCtx, thisValue, args)
	return normalizeProxyResult(result)
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
	argc C.int, argv *C.JSValue, magic C.int) (ret C.JSValue) {
	defer recoverToJSInternalError(ctx, &ret)

	// Get context and ClassBuilder using common helper
	goCtx, fn, perr := getContextAndObject(ctx, magic, errConstructorNotFound)
	if perr != nil {
		return throwProxyError(ctx, *perr)
	}

	// Type assertion to ClassBuilder (SCHEME C: stored entire ClassBuilder, not just constructor)
	builder, ok := fn.(*ClassBuilder)
	if !ok {
		return throwProxyError(ctx, errInvalidConstructorType)
	}
	if builder.constructor == nil {
		return throwProxyError(ctx, errConstructorIsNil)
	}

	// Extract class ID from newTarget for instance creation
	classID, exists := getConstructorClassID(goCtx, newTarget)
	if !exists {
		// This should not happen in normal cases since we register constructors
		// But provide fallback for defensive programming
		// return throwProxyError(ctx, proxyError{"InternalError", "Class ID not found for constructor"})
		v := &Value{ctx: goCtx, ref: newTarget}
		classID, exists = v.resolveClassIDFromInheritance()
	}
	if !exists {
		return throwProxyError(ctx, proxyError{"InternalError", "Class ID not found for constructor"})
	}

	// SCHEME C STEP 1: Validate all instance properties first to avoid
	// leaking C strings on early-return error paths.
	instancePropertyEntries := make([]PropertyEntry, 0)
	for _, property := range builder.properties {
		if property.Static {
			continue
		}
		if property.Value == nil || property.Value.ctx == nil || property.Value.ctx.ref == nil || property.Value.ctx != goCtx {
			return throwProxyError(ctx, errInvalidInstanceProp)
		}
		instancePropertyEntries = append(instancePropertyEntries, property)
	}

	// SCHEME C STEP 2: Build C-side instance properties after validation.
	var instanceProperties []C.PropertyEntry
	var instancePropertyNames []*C.char // Track C strings for cleanup
	for _, property := range instancePropertyEntries {
		propertyName := C.CString(property.Name)
		instancePropertyNames = append(instancePropertyNames, propertyName)

		instanceProperties = append(instanceProperties, C.PropertyEntry{
			name:      propertyName,
			value:     property.Value.ref,
			is_static: C.int(0), // Always instance property
			flags:     C.int(property.Flags),
		})
	}

	// Prepare C array pointer for instance properties
	var instancePropertiesPtr *C.PropertyEntry
	if len(instanceProperties) > 0 {
		instancePropertiesPtr = &instanceProperties[0]
	}

	// SCHEME C STEP 3: Create instance with bound properties using modified CreateClassInstance
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
	if C.JS_IsException_Wrapper(instance) != 0 {
		return instance // Return the exception
	}
	cleanupInstance := true
	defer func() {
		if cleanupInstance {
			C.JS_FreeValue(ctx, instance)
		}
	}()

	// SCHEME C STEP 3: Call constructor function with pre-created instance
	// Constructor receives the pre-created instance and returns Go object to associate
	// MODIFIED: now uses pointer conversion
	instanceValue := &Value{ctx: goCtx, ref: instance}
	args := convertCArgsToGoValues(argc, argv, goCtx)
	goObj, err := builder.constructor(goCtx, instanceValue, args)
	if err != nil {
		errorMsg := C.CString(err.Error())
		defer C.free(unsafe.Pointer(errorMsg))
		return C.ThrowInternalError(ctx, errorMsg)
	}

	// SCHEME C STEP 4: Associate Go object with instance if constructor returned non-nil object
	if goObj != nil {
		handleID := goCtx.handleStore.Store(goObj)
		var opaque unsafe.Pointer
		if !forceClassOpaqueAllocFailureForTest.Load() {
			opaque = C.NewClassOpaque(ctx, C.int32_t(handleID))
		}
		if opaque == nil {
			goCtx.handleStore.Delete(handleID)
			return throwProxyError(ctx, proxyError{"InternalError", "Failed to allocate class opaque payload"})
		}
		C.JS_SetOpaque(instance, opaque)
	}

	cleanupInstance = false
	return instance
}

// Class method proxy - handles both instance and static methods
// Corresponds to QuickJS JSCFunctionType.generic_magic
// MODIFIED: Method signature changed to use *Value
//
//export goClassMethodProxy
func goClassMethodProxy(ctx *C.JSContext, thisVal C.JSValue,
	argc C.int, argv *C.JSValue, magic C.int) (ret C.JSValue) {
	defer recoverToJSInternalError(ctx, &ret)

	// Get context and method using common helper
	goCtx, fn, err := getContextAndObject(ctx, magic, errMethodNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	// Type assertion to ClassMethodFunc signature - MODIFIED: now uses pointers
	method, ok := fn.(ClassMethodFunc)
	if !ok {
		return throwProxyError(ctx, errInvalidMethodType)
	}

	// Convert parameters and call method - MODIFIED: now uses pointer conversion
	thisValue := &Value{ctx: goCtx, ref: thisVal}
	args := convertCArgsToGoValues(argc, argv, goCtx)
	result := method(goCtx, thisValue, args)
	return normalizeProxyResult(result)
}

// Class property getter proxy
// Corresponds to QuickJS JSCFunctionType.getter_magic
// MODIFIED: Getter signature changed to use *Value
//
//export goClassGetterProxy
func goClassGetterProxy(ctx *C.JSContext, thisVal C.JSValue, magic C.int) (ret C.JSValue) {
	defer recoverToJSInternalError(ctx, &ret)

	// Get context and getter using common helper
	goCtx, fn, err := getContextAndObject(ctx, magic, errGetterNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	// Type assertion to ClassGetterFunc signature - MODIFIED: now uses pointers
	getter, ok := fn.(ClassGetterFunc)
	if !ok {
		return throwProxyError(ctx, errInvalidGetterType)
	}

	// Call getter with this value only - MODIFIED: now uses pointer conversion
	thisValue := &Value{ctx: goCtx, ref: thisVal}
	result := getter(goCtx, thisValue)
	return normalizeProxyResult(result)
}

// Class property setter proxy
// Corresponds to QuickJS JSCFunctionType.setter_magic
// MODIFIED: Setter signature changed to use *Value
//
//export goClassSetterProxy
func goClassSetterProxy(ctx *C.JSContext, thisVal C.JSValue,
	val C.JSValue, magic C.int) (ret C.JSValue) {
	defer recoverToJSInternalError(ctx, &ret)

	// Get context and setter using common helper
	goCtx, fn, err := getContextAndObject(ctx, magic, errSetterNotFound)
	if err != nil {
		return throwProxyError(ctx, *err)
	}

	// Type assertion to ClassSetterFunc signature - MODIFIED: now uses pointers
	setter, ok := fn.(ClassSetterFunc)
	if !ok {
		return throwProxyError(ctx, errInvalidSetterType)
	}

	// Call setter with this value and new value - MODIFIED: now uses pointer conversion
	thisValue := &Value{ctx: goCtx, ref: thisVal}
	setValue := &Value{ctx: goCtx, ref: val}
	result := setter(goCtx, thisValue, setValue)
	return normalizeProxyResult(result)
}

// Class finalizer proxy - unified cleanup handler
// Corresponds to QuickJS JSClassDef.finalizer
// Called automatically when JS object is garbage collected
//
//export goClassFinalizerProxy
func goClassFinalizerProxy(rt *C.JSRuntime, val C.JSValue) {
	defer recoverFinalizerPanic()

	// Get class ID for the object being finalized
	classID := C.JS_GetClassID(val)

	// Get opaque data from JS object using JS_GetOpaque (like point.c finalizer)
	// This corresponds to point.c: s = JS_GetOpaque(val, js_point_class_id)
	// Note: JS_GetOpaque only needs val and class_id (no context required in finalizer)
	opaque := C.JS_GetOpaque(val, classID)
	if opaque == nil {
		observeFinalizerCounter(rt, finalizerCounterOpaqueNil)
		return // Corresponds to point.c: 's' can be NULL
	}
	if C.ClassOpaqueIsValid(opaque) == 0 {
		observeFinalizerCounter(rt, finalizerCounterOpaqueInvalid)
		return
	}

	handleID := int32(C.ClassOpaqueHandleID(opaque))
	ctxRef := C.ClassOpaqueContext(opaque)
	C.FreeClassOpaque(opaque)

	if handleID <= 0 {
		observeFinalizerCounter(rt, finalizerCounterHandleInvalid)
		return
	}
	if ctxRef == nil {
		observeFinalizerCounter(rt, finalizerCounterContextRefInvalid)
		return
	}

	targetCtx := getContextFromJS(ctxRef)
	if targetCtx == nil {
		observeFinalizerCounter(rt, finalizerCounterContextNotFound)
		return
	}
	if targetCtx.runtime == nil || targetCtx.runtime.ref == nil || targetCtx.handleStore == nil {
		observeFinalizerCounter(rt, finalizerCounterContextStateInvalid)
		return
	}
	if targetCtx.runtime.ref != rt {
		observeFinalizerCounter(rt, finalizerCounterRuntimeMismatch)
		return
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
		observeFinalizerCounter(rt, finalizerCounterCleaned)
		return
	}
	observeFinalizerCounter(rt, finalizerCounterHandleMissing)
}

// =============================================================================
// MODULE-RELATED PROXY FUNCTIONS - SIMPLIFIED
// =============================================================================

// Module initialization proxy function - Go export for C bridge (SIMPLIFIED)
// This function serves as the bridge between QuickJS C API and Go ModuleBuilder functionality
// Called by QuickJS when a module is being initialized
// Corresponds to JSModuleInitFunc signature: int (*)(JSContext *ctx, JSModuleDef *m)
//
//export goModuleInitProxy
func goModuleInitProxy(ctx *C.JSContext, m *C.JSModuleDef) (ret C.int) {
	defer recoverToModuleError(ctx, &ret)

	// Step 1: Get context and ModuleBuilder using helper
	goCtx, builder, builderID, err := getContextAndModuleBuilder(ctx, m)
	if err != nil {
		if goCtx != nil && goCtx.handleStore != nil && builderID > 0 {
			goCtx.handleStore.Delete(builderID)
		}
		return throwModuleError(ctx, err)
	}
	defer goCtx.handleStore.Delete(builderID)
	invokeModuleInitPanicHookForTest(goCtx, builder)

	// Step 2: Set all export values using JS_SetModuleExport
	for _, export := range builder.exports {
		if export.Value == nil || export.Value.ctx == nil || export.Value.ctx.ref == nil || export.Value.ctx != goCtx {
			return throwModuleError(ctx, fmt.Errorf("invalid module export value: %s", export.Name))
		}

		exportName := C.CString(export.Name)
		// JS_SetModuleExport takes ownership of the JSValue (it will free it on failure).
		// To prevent Go-side double free (issue #688), invalidate the Go Value after
		// handing it off so later export.Value.Free() becomes a no-op.
		val := export.Value.ref
		rc := C.JS_SetModuleExport(ctx, m, exportName, val)
		export.Value.ref = C.JS_NewUndefined()
		C.free(unsafe.Pointer(exportName))
		if rc < 0 {
			return throwModuleError(ctx, fmt.Errorf("failed to set module export: %s", export.Name))
		}
	}

	return C.int(0)
}
