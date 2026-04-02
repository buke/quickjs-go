package quickjs

/*
#include "bridge.h"
#include <time.h>
*/
import "C"
import (
	"runtime"
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

// Runtime represents a Javascript runtime with simplified interrupt handling
type Runtime struct {
	mu                     sync.RWMutex
	ref                    *C.JSRuntime
	options                *Options
	interruptHandlerState  atomic.Pointer[interruptHandlerHolder]
	contexts               sync.Map
	constructorRegistry    sync.Map
	closeOnce              sync.Once
	closed                 atomic.Bool
	stdHandlersInitialized bool
}

type Options struct {
	timeout      uint64
	memoryLimit  uint64
	gcThreshold  int64
	maxStackSize uint64
	canBlock     bool
	moduleImport bool
	strip        int
}

type Option func(*Options)

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

// NewRuntime creates a new quickjs runtime with simplified interrupt handling.
func NewRuntime(opts ...Option) *Runtime {
	runtime.LockOSThread() // prevent multiple quickjs runtime from being created

	options := &Options{
		timeout:      0,
		memoryLimit:  0,
		gcThreshold:  -1,
		maxStackSize: 0,
		canBlock:     true,
		moduleImport: false,
		strip:        1,
	}
	for _, opt := range opts {
		opt(options)
	}

	rt := &Runtime{
		ref:     C.JS_NewRuntime(),
		options: options,
	}
	registerRuntime(rt.ref, rt)

	// Configure runtime options
	if rt.options.memoryLimit > 0 {
		rt.SetMemoryLimit(rt.options.memoryLimit)
	}

	if rt.options.gcThreshold >= -1 {
		rt.SetGCThreshold(rt.options.gcThreshold)
	}

	rt.SetMaxStackSize(rt.options.maxStackSize)

	if rt.options.canBlock {
		C.JS_SetCanBlock(rt.ref, C.int(1))
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_RunGC(r.ref)
}

// Close will free the runtime pointer with proper cleanup.
func (r *Runtime) Close() {
	if r == nil {
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
			runtime.UnlockOSThread()
			return
		}

		ref := r.ref
		r.interruptHandlerState.Store(nil)
		C.ClearInterruptHandler(ref)

		r.constructorRegistry.Range(func(key, _ interface{}) bool {
			r.constructorRegistry.Delete(key)
			return true
		})

		unregisterRuntime(ref)
		if r.stdHandlersInitialized {
			C.js_std_free_handlers(ref)
			r.stdHandlersInitialized = false
		}

		C.JS_FreeRuntime(ref)
		r.ref = nil
		runtime.UnlockOSThread()
	})
}

// SetCanBlock will set the runtime's can block; default is true
func (r *Runtime) SetCanBlock(canBlock bool) {
	if r == nil {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}

	if canBlock {
		C.JS_SetCanBlock(r.ref, C.int(1))
	} else {
		C.JS_SetCanBlock(r.ref, C.int(0))
	}
}

// SetMemoryLimit the runtime memory limit; if not set, it will be unlimit.
func (r *Runtime) SetMemoryLimit(limit uint64) {
	if r == nil {
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetStripInfo(r.ref, C.int(strip))
}

// SetModuleImport sets whether the runtime supports module import.
func (r *Runtime) SetModuleImport(moduleImport bool) {
	if r == nil {
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
	codePtr := C.CString(code)
	defer C.free(unsafe.Pointer(codePtr))
	filenamePtr := C.CString("init.js")
	defer C.free(unsafe.Pointer(filenamePtr))

	// Replace evaluation flags with function calls
	evalFlags := C.int(C.GetEvalTypeModule()) | C.int(C.GetEvalFlagCompileOnly())
	init_compile := C.JS_Eval(ctx_ref, codePtr, C.size_t(len(code)), filenamePtr, evalFlags)
	init_run := C.js_std_await(ctx_ref, C.JS_EvalFunction(ctx_ref, init_compile))
	C.JS_FreeValue(ctx_ref, init_run)

	// Create Context with HandleStore for function management
	ctx := &Context{
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
}

func (r *Runtime) unregisterOwnedContext(ctxRef *C.JSContext) {
	if r == nil || ctxRef == nil {
		return
	}
	r.contexts.Delete(ctxRef)
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
