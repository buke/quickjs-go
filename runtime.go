package quickjs

/*
#include "bridge.h"
#include <time.h>
*/
import "C"
import (
	"runtime"
	"runtime/cgo"
	"unsafe"
)

// Runtime represents a Javascript runtime corresponding to an object heap. Several runtimes can exist at the same time but they cannot exchange objects. Inside a given runtime, no multi-threading is supported.
type Runtime struct {
	ref     *C.JSRuntime
	options *Options
	pinner  runtime.Pinner
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

// NewRuntime creates a new quickjs runtime.
func NewRuntime(opts ...Option) Runtime {
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

	rt := Runtime{ref: C.JS_NewRuntime(), options: options}

	if rt.options.timeout > 0 {
		rt.SetExecuteTimeout(rt.options.timeout)
	}

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
	return rt
}

// RunGC will call quickjs's garbage collector.
func (r Runtime) RunGC() {
	C.JS_RunGC(r.ref)
}

// Close will free the runtime pointer.
func (r Runtime) Close() {
	C.JS_FreeRuntime(r.ref)
	r.pinner.Unpin()
}

// SetCanBlock will set the runtime's can block; default is true
func (r Runtime) SetCanBlock(canBlock bool) {
	if canBlock {
		C.JS_SetCanBlock(r.ref, C.int(1))
	} else {
		C.JS_SetCanBlock(r.ref, C.int(0))
	}
}

// SetMemoryLimit the runtime memory limit; if not set, it will be unlimit.
func (r Runtime) SetMemoryLimit(limit uint64) {
	C.JS_SetMemoryLimit(r.ref, C.size_t(limit))
}

// SetGCThreshold the runtime's GC threshold; use -1 to disable automatic GC.
func (r Runtime) SetGCThreshold(threshold int64) {
	C.JS_SetGCThreshold(r.ref, C.size_t(threshold))
}

// SetMaxStackSize will set max runtime's stack size;
func (r Runtime) SetMaxStackSize(stack_size uint64) {
	C.JS_SetMaxStackSize(r.ref, C.size_t(stack_size))
}

// SetExecuteTimeout will set the runtime's execute timeout;
func (r Runtime) SetExecuteTimeout(timeout uint64) {
	C.SetExecuteTimeout(r.ref, C.time_t(timeout))
}

func (r Runtime) SetStripInfo(strip int) {
	C.JS_SetStripInfo(r.ref, C.int(strip))
}

// SetInterruptHandler sets a interrupt handler.
func (r *Runtime) SetInterruptHandler(handler InterruptHandler) {
	handlerArgsPtr := &C.handlerArgs{
		fn: (C.uintptr_t)(cgo.NewHandle(handler)),
	}

	// Ensure the C.handlerArgs instance is never moved to a different place or GCed.
	r.pinner.Pin(handlerArgsPtr)

	C.SetInterruptHandler(r.ref, unsafe.Pointer(handlerArgsPtr))
}

// NewContext creates a new JavaScript context.
// enable BigFloat/BigDecimal support and enable .
// enable operator overloading.
func (r Runtime) NewContext() *Context {
	C.js_std_init_handlers(r.ref)

	// create a new context (heap, global object and context stack
	ctx_ref := C.JS_NewContext(r.ref)

	// set the module loader for support dynamic import
	if r.options.moduleImport {
		C.JS_SetModuleLoaderFunc2(r.ref, (*C.JSModuleNormalizeFunc)(unsafe.Pointer(nil)), (*C.JSModuleLoaderFunc2)(C.js_module_loader), (*C.JSModuleCheckSupportedImportAttributes)(C.js_module_check_attributes), unsafe.Pointer(nil))
	}

	// import the 'std' and 'os' modules
	C.js_init_module_std(ctx_ref, C.CString("std"))
	C.js_init_module_os(ctx_ref, C.CString("os"))

	// import setTimeout and clearTimeout from 'os' to globalThis
	code := `
	import { setTimeout, clearTimeout } from "os";
	globalThis.setTimeout = setTimeout;
	globalThis.clearTimeout = clearTimeout;
	`
	init_compile := C.JS_Eval(ctx_ref, C.CString(code), C.size_t(len(code)), C.CString("init.js"), C.JS_EVAL_TYPE_MODULE|C.JS_EVAL_FLAG_COMPILE_ONLY)
	init_run := C.js_std_await(ctx_ref, C.JS_EvalFunction(ctx_ref, init_compile))
	C.JS_FreeValue(ctx_ref, init_run)
	// C.js_std_loop(ctx_ref)

	return &Context{ref: ctx_ref, runtime: &r}
}
