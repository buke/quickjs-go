package quickjs

/*
#include "bridge.h"
#include <time.h>
*/
import "C"
import (
	"runtime"
	"unsafe"
)

// InterruptHandler is a function type for interrupt handler.
// Return != 0 if the JS code needs to be interrupted
type InterruptHandler func() int

// Runtime represents a Javascript runtime with simplified interrupt handling
type Runtime struct {
	ref              *C.JSRuntime
	options          *Options
	interruptHandler InterruptHandler // Store interrupt handler directly (no cgo.Handle)
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

	// Set timeout after registration (will override interrupt handler)
	if rt.options.timeout > 0 {
		rt.SetExecuteTimeout(rt.options.timeout)
	}

	// Register runtime for interrupt handler mapping
	registerRuntime(rt.ref, rt)

	return rt
}

// RunGC will call quickjs's garbage collector.
func (r *Runtime) RunGC() {
	C.JS_RunGC(r.ref)
}

// Close will free the runtime pointer with proper cleanup.
func (r *Runtime) Close() {
	// Step 1: Clear interrupt handler before closing
	r.ClearInterruptHandler()

	// Step 2: Clean up global constructor registry safely
	// Use Range + Delete to avoid potential race conditions with map replacement
	globalConstructorRegistry.Range(func(key, value interface{}) bool {
		globalConstructorRegistry.Delete(key)
		return true // continue iteration
	})

	// Step 3: Unregister runtime mapping
	unregisterRuntime(r.ref)

	// Step 4: Clear context mapping
	clearContextMapping()

	// Step 5: Free QuickJS runtime
	C.JS_FreeRuntime(r.ref)

}

// SetCanBlock will set the runtime's can block; default is true
func (r *Runtime) SetCanBlock(canBlock bool) {
	if canBlock {
		C.JS_SetCanBlock(r.ref, C.int(1))
	} else {
		C.JS_SetCanBlock(r.ref, C.int(0))
	}
}

// SetMemoryLimit the runtime memory limit; if not set, it will be unlimit.
func (r *Runtime) SetMemoryLimit(limit uint64) {
	C.JS_SetMemoryLimit(r.ref, C.size_t(limit))
}

// SetGCThreshold the runtime's GC threshold; use -1 to disable automatic GC.
func (r *Runtime) SetGCThreshold(threshold int64) {
	C.JS_SetGCThreshold(r.ref, C.size_t(threshold))
}

// SetMaxStackSize will set max runtime's stack size;
func (r *Runtime) SetMaxStackSize(stack_size uint64) {
	C.JS_SetMaxStackSize(r.ref, C.size_t(stack_size))
}

// SetExecuteTimeout will set the runtime's execute timeout;
// This will override any user interrupt handler (expected behavior)
func (r *Runtime) SetExecuteTimeout(timeout uint64) {
	C.SetExecuteTimeout(r.ref, C.time_t(timeout))
	// Clear user interrupt handler since timeout takes precedence
	r.interruptHandler = nil
}

// SetStripInfo sets the strip info for the runtime.
func (r *Runtime) SetStripInfo(strip int) {
	C.JS_SetStripInfo(r.ref, C.int(strip))
}

// SetModuleImport sets whether the runtime supports module import.
func (r *Runtime) SetModuleImport(moduleImport bool) {
	C.JS_SetModuleLoaderFunc2(r.ref, (*C.JSModuleNormalizeFunc)(unsafe.Pointer(nil)), (*C.JSModuleLoaderFunc2)(C.js_module_loader), (*C.JSModuleCheckSupportedImportAttributes)(C.js_module_check_attributes), unsafe.Pointer(nil))
}

// SetInterruptHandler sets a user interrupt handler using simplified approach.
// This will override any timeout handler (expected behavior)
func (r *Runtime) SetInterruptHandler(handler InterruptHandler) {
	r.interruptHandler = handler

	if handler != nil {
		// Simplified call - no handlerArgs complexity
		C.SetInterruptHandler(r.ref)
	} else {
		C.ClearInterruptHandler(r.ref)
	}
}

// ClearInterruptHandler clears the user interrupt handler
func (r *Runtime) ClearInterruptHandler() {
	r.interruptHandler = nil
	C.ClearInterruptHandler(r.ref)
}

// callInterruptHandler is called from C layer via runtime mapping (internal use)
func (r *Runtime) callInterruptHandler() int {
	if r.interruptHandler != nil {
		return r.interruptHandler()
	}
	return 0 // No interrupt
}

// NewContext creates a new JavaScript context.
func (r *Runtime) NewContext() *Context {
	C.js_std_init_handlers(r.ref)

	// create a new context (heap, global object and context stack
	ctx_ref := C.JS_NewContext(r.ref)

	// import the 'std' and 'os' modules
	C.js_init_module_std(ctx_ref, C.CString("std"))
	C.js_init_module_os(ctx_ref, C.CString("os"))

	// import setTimeout and clearTimeout from 'os' to globalThis
	code := `
    import { setTimeout, clearTimeout } from "os";
    globalThis.setTimeout = setTimeout;
    globalThis.clearTimeout = clearTimeout;
    `

	// Replace evaluation flags with function calls
	evalFlags := C.int(C.GetEvalTypeModule()) | C.int(C.GetEvalFlagCompileOnly())
	init_compile := C.JS_Eval(ctx_ref, C.CString(code), C.size_t(len(code)), C.CString("init.js"), evalFlags)
	init_run := C.js_std_await(ctx_ref, C.JS_EvalFunction(ctx_ref, init_compile))
	C.JS_FreeValue(ctx_ref, init_run)

	// Create Context with HandleStore for function management
	ctx := &Context{
		ref:         ctx_ref,
		runtime:     r,
		handleStore: newHandleStore(), // Initialize HandleStore for function management
	}

	// Register context mapping for C callbacks
	registerContext(ctx_ref, ctx)

	return ctx
}
