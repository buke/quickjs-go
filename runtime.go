package quickjs

/*
#include "bridge.h"
#include <time.h>
*/
import "C"
import (
	"errors"
	goruntime "runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

// InterruptHandler is a function type for interrupt handler.
// Return != 0 if the JS code needs to be interrupted
type InterruptHandler func() int

type interruptHandlerHolder struct {
	fn InterruptHandler
}

// runtimeNewContextHook is used in tests to force JS_NewContext failure paths.
// It must remain nil in production.
var runtimeNewContextHook func(rt *C.JSRuntime) *C.JSContext

// runtimeInitContextHook is used in tests to force context initialization failure paths.
// It must remain nil in production.
var runtimeInitContextHook func(ctx *C.JSContext) C.JSValue

// runtimeEvalFunctionHook is used in tests to force JS_EvalFunction failure paths.
// It must remain nil in production.
var runtimeEvalFunctionHook func(ctx *C.JSContext, compiled C.JSValue) C.JSValue

var errOwnerAccessDenied = errors.New("quickjs: owner access denied; runtime/context/value APIs must be called from the owner goroutine; if strict OS thread mode is enabled, also bind that goroutine with runtime.LockOSThread()")

var ownerCheckCurrentGoroutineID = currentGoroutineID
var ownerCheckCurrentThreadID = currentThreadID
var goroutineStack = goruntime.Stack

type classObjectIdentity struct {
	contextID uint64
	handleID  int32
}

// Runtime represents a Javascript runtime with simplified interrupt handling
type Runtime struct {
	mu                     sync.RWMutex
	ref                    *C.JSRuntime
	options                *Options
	ownerGoroutineID       atomic.Uint64
	ownerThreadID          atomic.Uint64
	interruptHandlerState  atomic.Pointer[interruptHandlerHolder]
	contexts               sync.Map
	contextsByID           sync.Map
	contextIDCounter       atomic.Uint64
	constructorRegistry    sync.Map
	classObjectRegistry    sync.Map
	classObjectIDsByCtx    sync.Map
	classObjectIDCounter   atomic.Int32
	closeOnce              sync.Once
	closed                 atomic.Bool
	stdHandlersInitialized bool
}

// isAlive reports whether the runtime still has a valid native handle and
// has not started closing.
func (r *Runtime) isAlive() bool {
	return r != nil && r.ref != nil && !r.closed.Load()
}

type Options struct {
	timeout              uint64
	memoryLimit          uint64
	gcThreshold          int64
	maxStackSize         uint64
	canBlock             bool
	moduleImport         bool
	strip                int
	ownerGoroutineCheck  bool
	strictThreadAffinity bool
}

type Option func(*Options)

func (r *Runtime) ensureOwnerAccess() bool {
	if r == nil {
		return false
	}

	if r.options == nil || r.options.ownerGoroutineCheck {
		gid := ownerCheckCurrentGoroutineID()
		if gid == 0 {
			return false
		}
		if !r.claimOrVerifyOwnerGoroutine(gid) {
			return false
		}
	}

	if r.options != nil && r.options.strictThreadAffinity {
		tid := ownerCheckCurrentThreadID()
		if tid == 0 {
			return false
		}
		if !r.claimOrVerifyOwnerThread(tid) {
			return false
		}
	}

	return true
}

func (r *Runtime) claimOrVerifyOwnerGoroutine(current uint64) bool {
	owner := r.ownerGoroutineID.Load()
	if owner == 0 {
		if r.ownerGoroutineID.CompareAndSwap(0, current) {
			return true
		}
		owner = r.ownerGoroutineID.Load()
	}
	return owner == current
}

func (r *Runtime) claimOrVerifyOwnerThread(current uint64) bool {
	owner := r.ownerThreadID.Load()
	if owner == 0 {
		r.ownerThreadID.CompareAndSwap(0, current)
		owner = r.ownerThreadID.Load()
	}
	return owner == current
}

func currentGoroutineID() uint64 {
	const prefix = "goroutine "

	var buf [128]byte
	n := goroutineStack(buf[:], false)
	if n <= len(prefix) {
		return 0
	}

	idx := len(prefix)
	var id uint64
	hasDigit := false
	for idx < n {
		c := buf[idx]
		if c < '0' || c > '9' {
			break
		}
		hasDigit = true
		id = id*10 + uint64(c-'0')
		idx++
	}

	if !hasDigit {
		return 0
	}
	return id
}

func currentThreadID() uint64 {
	return uint64(C.CurrentThreadID())
}

// WithExecuteTimeout will set the runtime's execute timeout; default is 0
func WithExecuteTimeout(timeout uint64) Option {
	return func(o *Options) {
		o.timeout = timeout
	}
}

// WithMemoryLimit will set the runtime memory limit; if not set, it will be unlimit.
func WithMemoryLimit(memoryLimit uint64) Option {
	return func(o *Options) {
		o.memoryLimit = memoryLimit
	}
}

// WithGCThreshold will set the runtime's GC threshold; default is -1 to disable automatic GC.
func WithGCThreshold(gcThreshold int64) Option {
	return func(o *Options) {
		o.gcThreshold = gcThreshold
	}
}

// WithMaxStackSize will set max runtime's stack size; default is 0 disable maximum stack size check
func WithMaxStackSize(maxStackSize uint64) Option {
	return func(o *Options) {
		o.maxStackSize = maxStackSize
	}
}

// WithCanBlock will set the runtime's can block; default is true
func WithCanBlock(canBlock bool) Option {
	return func(o *Options) {
		o.canBlock = canBlock
	}
}

func WithModuleImport(moduleImport bool) Option {
	return func(o *Options) {
		o.moduleImport = moduleImport
	}
}

func WithStripInfo(strip int) Option {
	return func(o *Options) {
		o.strip = strip
	}
}

// WithOwnerGoroutineCheck enables/disables owner-goroutine checks.
// WARNING: disabling this check is unsafe and may cause data races or memory corruption.
func WithOwnerGoroutineCheck(enabled bool) Option {
	return func(o *Options) {
		o.ownerGoroutineCheck = enabled
	}
}

// WithStrictOSThread enables strict OS-thread affinity checks.
func WithStrictOSThread(enabled bool) Option {
	return func(o *Options) {
		o.strictThreadAffinity = enabled
	}
}

// NewRuntime creates a new quickjs runtime with simplified interrupt handling.
func NewRuntime(opts ...Option) *Runtime {
	options := &Options{
		timeout:              0,
		memoryLimit:          0,
		gcThreshold:          -1,
		maxStackSize:         0,
		canBlock:             true,
		moduleImport:         false,
		strip:                1,
		ownerGoroutineCheck:  true,
		strictThreadAffinity: false,
	}
	for _, opt := range opts {
		opt(options)
	}

	rt := &Runtime{
		ref:     C.JS_NewRuntime(),
		options: options,
	}
	registerRuntime(rt.ref, rt)
	C.SetPromiseRejectionTracker(rt.ref, 1)

	// Configure runtime options
	if rt.options.memoryLimit > 0 {
		rt.SetMemoryLimit(rt.options.memoryLimit)
	}

	if rt.options.gcThreshold >= -1 {
		rt.SetGCThreshold(rt.options.gcThreshold)
	}

	rt.SetMaxStackSize(rt.options.maxStackSize)

	if rt.options.canBlock {
		C.JS_SetCanBlock(rt.ref, C.bool(true))
	}

	if rt.options.strip > 0 {
		rt.SetStripInfo(rt.options.strip)
	}

	if rt.options.moduleImport {
		rt.SetModuleImport(rt.options.moduleImport)
	}

	// Set timeout after other options (will override interrupt handler)
	if rt.options.timeout > 0 {
		rt.SetExecuteTimeout(rt.options.timeout)
	}

	return rt
}

// RunGC will call quickjs's garbage collector.
func (r *Runtime) RunGC() {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_RunGC(r.ref)
}

// SetAwaitPollSliceMs configures AwaitValue idle poll slice duration in milliseconds.
// Values <= 0 are ignored.
func SetAwaitPollSliceMs(timeoutMs int) {
	if timeoutMs <= 0 {
		return
	}
	C.SetAwaitPollSliceMs(C.int(timeoutMs))
}

// GetAwaitPollSliceMs returns AwaitValue idle poll slice duration in milliseconds.
func GetAwaitPollSliceMs() int {
	return int(C.GetAwaitPollSliceMs())
}

// Close will free the runtime pointer with proper cleanup.
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}

	r.closeOnce.Do(func() {
		r.closed.Store(true)

		var contexts []*Context
		r.contexts.Range(func(_, value interface{}) bool {
			if ctx, ok := value.(*Context); ok {
				contexts = append(contexts, ctx)
			}
			return true
		})
		for _, ctx := range contexts {
			ctx.Close()
		}

		r.mu.Lock()
		defer r.mu.Unlock()
		if r.ref == nil {
			return
		}

		ref := r.ref
		r.interruptHandlerState.Store(nil)
		C.ClearInterruptHandler(ref)
		C.SetPromiseRejectionTracker(ref, 0)

		r.constructorRegistry.Range(func(key, _ interface{}) bool {
			r.constructorRegistry.Delete(key)
			return true
		})

		r.classObjectRegistry.Range(func(key, _ interface{}) bool {
			r.classObjectRegistry.Delete(key)
			return true
		})
		r.classObjectIDsByCtx.Range(func(key, _ interface{}) bool {
			r.classObjectIDsByCtx.Delete(key)
			return true
		})
		r.contextsByID.Range(func(key, _ interface{}) bool {
			r.contextsByID.Delete(key)
			return true
		})

		unregisterRuntime(ref)
		if r.stdHandlersInitialized {
			C.js_std_free_handlers(ref)
			r.stdHandlersInitialized = false
		}

		C.JS_FreeRuntime(ref)
		r.ref = nil
	})
}

// SetCanBlock will set the runtime's can block; default is true
func (r *Runtime) SetCanBlock(canBlock bool) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}

	C.JS_SetCanBlock(r.ref, C.bool(canBlock))
}

// SetMemoryLimit the runtime memory limit; if not set, it will be unlimit.
func (r *Runtime) SetMemoryLimit(limit uint64) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetMemoryLimit(r.ref, C.size_t(limit))
}

// SetGCThreshold the runtime's GC threshold; use -1 to disable automatic GC.
func (r *Runtime) SetGCThreshold(threshold int64) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetGCThreshold(r.ref, C.size_t(threshold))
}

// SetMaxStackSize will set max runtime's stack size;
func (r *Runtime) SetMaxStackSize(stack_size uint64) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetMaxStackSize(r.ref, C.size_t(stack_size))
}

// SetExecuteTimeout will set the runtime's execute timeout;
// This will override any user interrupt handler (expected behavior)
func (r *Runtime) SetExecuteTimeout(timeout uint64) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.SetExecuteTimeout(r.ref, C.time_t(timeout))
	// Clear user interrupt handler since timeout takes precedence
	r.interruptHandlerState.Store(nil)
}

// SetStripInfo sets the strip info for the runtime.
func (r *Runtime) SetStripInfo(strip int) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	// quickjs-ng does not expose a runtime-level JS_SetStripInfo API.
	_ = strip
}

// SetModuleImport sets whether the runtime supports module import.
func (r *Runtime) SetModuleImport(moduleImport bool) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetModuleLoaderFunc2(r.ref, (*C.JSModuleNormalizeFunc)(unsafe.Pointer(nil)), (*C.JSModuleLoaderFunc2)(C.js_module_loader), (*C.JSModuleCheckSupportedImportAttributes)(C.js_module_check_attributes), unsafe.Pointer(nil))
}

// SetInterruptHandler sets a user interrupt handler using simplified approach.
// This will override any timeout handler (expected behavior)
func (r *Runtime) SetInterruptHandler(handler InterruptHandler) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	if handler != nil {
		r.interruptHandlerState.Store(&interruptHandlerHolder{fn: handler})
	} else {
		r.interruptHandlerState.Store(nil)
	}

	if handler != nil {
		// Simplified call - no handlerArgs complexity
		C.SetInterruptHandler(r.ref)
	} else {
		C.ClearInterruptHandler(r.ref)
	}
}

// ClearInterruptHandler clears the user interrupt handler
func (r *Runtime) ClearInterruptHandler() {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ref == nil {
		return
	}
	r.interruptHandlerState.Store(nil)
	C.ClearInterruptHandler(r.ref)
}

// callInterruptHandler is called from C layer via runtime mapping (internal use)
func (r *Runtime) callInterruptHandler() int {
	if r == nil {
		return 0
	}
	holder := r.interruptHandlerState.Load()
	if holder != nil && holder.fn != nil {
		return holder.fn()
	}
	return 0 // No interrupt
}

// NewContext creates a new JavaScript context.
func (r *Runtime) NewContext() *Context {
	if r == nil {
		return nil
	}
	if !r.ensureOwnerAccess() {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() || r.ref == nil {
		return nil
	}

	if !r.stdHandlersInitialized {
		C.js_std_init_handlers(r.ref)
		r.stdHandlersInitialized = true
	}

	// create a new context (heap, global object and context stack
	var ctx_ref *C.JSContext
	if runtimeNewContextHook != nil {
		ctx_ref = runtimeNewContextHook(r.ref)
	} else {
		ctx_ref = C.JS_NewContext(r.ref)
	}
	if ctx_ref == nil {
		return nil
	}

	// import the 'std' and 'os' modules
	stdModuleName := C.CString("std")
	defer C.free(unsafe.Pointer(stdModuleName))
	osModuleName := C.CString("os")
	defer C.free(unsafe.Pointer(osModuleName))
	C.js_init_module_std(ctx_ref, stdModuleName)
	C.js_init_module_os(ctx_ref, osModuleName)

	// import setTimeout and clearTimeout from 'os' to globalThis
	code := `
    import { setTimeout, clearTimeout } from "os";
    globalThis.setTimeout = setTimeout;
    globalThis.clearTimeout = clearTimeout;
    `
	if !initializeContextGlobals(ctx_ref, code, "init.js") {
		C.JS_FreeContext(ctx_ref)
		return nil
	}

	// Create Context with HandleStore for function management
	ctx := &Context{
		contextID:   r.nextContextID(),
		ref:         ctx_ref,
		runtime:     r,
		handleStore: newHandleStore(), // Initialize HandleStore for function management
	}
	ctx.initScheduler()

	// Register context mapping for C callbacks
	registerContext(ctx_ref, ctx)
	r.registerOwnedContext(ctx)

	return ctx
}

func (r *Runtime) registerOwnedContext(ctx *Context) {
	if r == nil || ctx == nil || ctx.ref == nil {
		return
	}
	r.contexts.Store(ctx.ref, ctx)
	if ctx.contextID != 0 {
		r.contextsByID.Store(ctx.contextID, ctx)
	}
}

func (r *Runtime) unregisterOwnedContext(ctxRef *C.JSContext, contextID uint64) {
	if r == nil {
		return
	}
	if ctxRef != nil {
		r.contexts.Delete(ctxRef)
	}
	if contextID != 0 {
		r.contextsByID.Delete(contextID)
		r.cleanupClassObjectIdentitiesByContext(contextID)
	}
}

func (r *Runtime) nextContextID() uint64 {
	if r == nil {
		return 0
	}
	for {
		id := r.contextIDCounter.Add(1)
		if id != 0 {
			return id
		}
	}
}

func (r *Runtime) nextClassObjectID() int32 {
	if r == nil {
		return 0
	}
	id := r.classObjectIDCounter.Add(1)
	if id <= 0 {
		return 0
	}
	return -id
}

func (r *Runtime) getOwnedContextByID(contextID uint64) *Context {
	if r == nil || contextID == 0 {
		return nil
	}
	v, ok := r.contextsByID.Load(contextID)
	if !ok {
		return nil
	}
	ctx, ok := v.(*Context)
	if !ok {
		r.contextsByID.Delete(contextID)
		return nil
	}
	if !ctx.hasValidRef() {
		return nil
	}
	return ctx
}

func (r *Runtime) registerClassObjectIdentity(contextID uint64, handleID int32) int32 {
	if r == nil || contextID == 0 || handleID <= 0 {
		return 0
	}

	objectID := r.nextClassObjectID()
	if objectID == 0 {
		return 0
	}
	identity := classObjectIdentity{contextID: contextID, handleID: handleID}
	r.classObjectRegistry.Store(objectID, identity)

	bucketValue, _ := r.classObjectIDsByCtx.LoadOrStore(contextID, &sync.Map{})
	bucket, ok := bucketValue.(*sync.Map)
	if !ok {
		// Corruption path: serialize replacement to avoid concurrent overwrite.
		r.mu.Lock()
		currentBucket, loaded := r.classObjectIDsByCtx.Load(contextID)
		if loaded {
			if existing, typed := currentBucket.(*sync.Map); typed {
				bucket = existing
				ok = true
			}
		}
		if !ok {
			replacement := &sync.Map{}
			r.classObjectIDsByCtx.Store(contextID, replacement)
			bucket = replacement
		}
		r.mu.Unlock()
	}
	bucket.Store(objectID, struct{}{})

	return objectID
}

func (r *Runtime) getClassObjectIdentity(objectID int32) (classObjectIdentity, bool) {
	if r == nil || objectID == 0 {
		return classObjectIdentity{}, false
	}
	v, ok := r.classObjectRegistry.Load(objectID)
	if !ok {
		return classObjectIdentity{}, false
	}
	identity, ok := v.(classObjectIdentity)
	if !ok {
		r.classObjectRegistry.Delete(objectID)
		return classObjectIdentity{}, false
	}
	if identity.contextID == 0 || identity.handleID <= 0 {
		r.classObjectRegistry.Delete(objectID)
		return classObjectIdentity{}, false
	}
	return identity, true
}

func (r *Runtime) takeClassObjectIdentity(objectID int32) (classObjectIdentity, bool) {
	if r == nil || objectID == 0 {
		return classObjectIdentity{}, false
	}
	v, ok := r.classObjectRegistry.LoadAndDelete(objectID)
	if !ok {
		return classObjectIdentity{}, false
	}
	identity, ok := v.(classObjectIdentity)
	if !ok {
		return classObjectIdentity{}, false
	}
	if identity.contextID == 0 || identity.handleID <= 0 {
		return classObjectIdentity{}, false
	}

	if bucketValue, ok := r.classObjectIDsByCtx.Load(identity.contextID); ok {
		if bucket, ok := bucketValue.(*sync.Map); ok {
			bucket.Delete(objectID)
		}
	}

	return identity, true
}

func (r *Runtime) cleanupClassObjectIdentitiesByContext(contextID uint64) {
	if r == nil || contextID == 0 {
		return
	}
	bucketValue, ok := r.classObjectIDsByCtx.LoadAndDelete(contextID)
	if !ok {
		return
	}
	bucket, ok := bucketValue.(*sync.Map)
	if !ok {
		return
	}
	bucket.Range(func(key, _ interface{}) bool {
		objectID, ok := key.(int32)
		if !ok {
			return true
		}
		r.classObjectRegistry.Delete(objectID)
		return true
	})
}

func (r *Runtime) registerConstructorClassID(constructor C.JSValue, classID uint32) {
	if r == nil {
		return
	}
	r.constructorRegistry.Store(jsValueToKey(constructor), classID)
}

func (r *Runtime) getConstructorClassID(constructor C.JSValue) (uint32, bool) {
	if r == nil {
		return 0, false
	}

	constructorKey := jsValueToKey(constructor)
	if classIDInterface, ok := r.constructorRegistry.Load(constructorKey); ok {
		classID, ok := classIDInterface.(uint32)
		if !ok {
			r.constructorRegistry.Delete(constructorKey)
			return 0, false
		}
		return classID, true
	}
	return 0, false
}

func timeoutOpaqueCount() int {
	return int(C.GetTimeoutOpaqueCount())
}

func initializeContextGlobals(ctx *C.JSContext, code string, filename string) bool {
	if runtimeInitContextHook != nil {
		initRun := runtimeInitContextHook(ctx)
		if bool(C.JS_IsException(initRun)) {
			C.JS_FreeValue(ctx, initRun)
			return false
		}
		C.JS_FreeValue(ctx, initRun)
		return true
	}

	codeBuf := zeroTerminatedBytes(code)
	codePtr := (*C.char)(unsafe.Pointer(&codeBuf[0]))
	filenameBuf := zeroTerminatedBytes(filename)
	filenamePtr := (*C.char)(unsafe.Pointer(&filenameBuf[0]))

	var pinner goruntime.Pinner
	pinner.Pin(&codeBuf[0])
	pinner.Pin(&filenameBuf[0])
	defer pinner.Unpin()

	evalFlags := C.int(C.JS_EVAL_TYPE_MODULE) | C.int(C.JS_EVAL_FLAG_COMPILE_ONLY)
	initCompile := C.JS_Eval(ctx, codePtr, C.size_t(len(code)), filenamePtr, evalFlags)
	goruntime.KeepAlive(codeBuf)
	goruntime.KeepAlive(filenameBuf)
	if bool(C.JS_IsException(initCompile)) {
		C.JS_FreeValue(ctx, initCompile)
		return false
	}

	initEval := initCompile
	if runtimeEvalFunctionHook != nil {
		initEval = runtimeEvalFunctionHook(ctx, initCompile)
	} else {
		initEval = C.JS_EvalFunction(ctx, initCompile)
	}
	if bool(C.JS_IsException(initEval)) {
		C.JS_FreeValue(ctx, initEval)
		return false
	}

	initRun := C.AwaitValue(ctx, initEval)
	if bool(C.JS_IsException(initRun)) {
		C.JS_FreeValue(ctx, initRun)
		return false
	}

	C.JS_FreeValue(ctx, initRun)
	return true
}

func forceRuntimeNewContextFailureForTest(enable bool) func() {
	oldHook := runtimeNewContextHook
	if enable {
		runtimeNewContextHook = func(rt *C.JSRuntime) *C.JSContext {
			return nil
		}
	} else {
		runtimeNewContextHook = nil
	}
	return func() {
		runtimeNewContextHook = oldHook
	}
}

func forceRuntimeInitFailureForTest(enable bool) func() {
	oldHook := runtimeInitContextHook
	if enable {
		runtimeInitContextHook = func(ctx *C.JSContext) C.JSValue {
			msg := C.CString("forced init failure")
			defer C.free(unsafe.Pointer(msg))
			return C.ThrowInternalError(ctx, msg)
		}
	} else {
		runtimeInitContextHook = nil
	}
	return func() {
		runtimeInitContextHook = oldHook
	}
}

func forceRuntimeInitSuccessForTest(enable bool) func() {
	oldHook := runtimeInitContextHook
	if enable {
		runtimeInitContextHook = func(ctx *C.JSContext) C.JSValue {
			return C.JS_NewUndefined()
		}
	} else {
		runtimeInitContextHook = nil
	}
	return func() {
		runtimeInitContextHook = oldHook
	}
}

func forceRuntimeEvalFailureForTest(enable bool) func() {
	oldHook := runtimeEvalFunctionHook
	if enable {
		runtimeEvalFunctionHook = func(ctx *C.JSContext, compiled C.JSValue) C.JSValue {
			C.JS_FreeValue(ctx, compiled)
			msg := C.CString("forced eval failure")
			defer C.free(unsafe.Pointer(msg))
			return C.ThrowInternalError(ctx, msg)
		}
	} else {
		runtimeEvalFunctionHook = nil
	}
	return func() {
		runtimeEvalFunctionHook = oldHook
	}
}
