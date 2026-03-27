package quickjs

/*
#include "bridge.h"
#include <time.h>
*/
import "C"
import (
	"fmt"
	"runtime"
	"sync/atomic"
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
	handlersInit     bool
	osThreadLocked   bool
}

const (
	runtimeNewContextFailStageNone int32 = iota
	runtimeNewContextFailStageCompile
	runtimeNewContextFailStageExec
	runtimeNewContextFailStageAwait
)

var runtimeNewContextFailStageForTest atomic.Int32
var runtimeNewContextInitCodeForTest atomic.Value

const runtimeNewContextDefaultInitCode = `
	import { setTimeout, clearTimeout } from "os";
	globalThis.setTimeout = setTimeout;
	globalThis.clearTimeout = clearTimeout;
	`

// FinalizerObservabilitySnapshot captures fail-closed branches observed by class finalizers.
// This is intended for tests and diagnostics.
type FinalizerObservabilitySnapshot struct {
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
	runtime.LockOSThread() // bind runtime creation path to a dedicated OS thread

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
		ref:            C.JS_NewRuntime_Go(),
		options:        options,
		osThreadLocked: true,
	}

	if rt.ref == nil {
		runtime.UnlockOSThread()
		rt.osThreadLocked = false
		return rt
	}
	C.RegisterRuntimeOwnerThread(rt.ref)

	// Configure runtime options
	if rt.options.memoryLimit > 0 {
		rt.SetMemoryLimit(rt.options.memoryLimit)
	}

	if rt.options.gcThreshold >= -1 {
		rt.SetGCThreshold(rt.options.gcThreshold)
	}

	rt.SetMaxStackSize(rt.options.maxStackSize)

	if rt.options.canBlock {
		C.SetCanBlock(rt.ref, C.int(1))
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

func requireRuntimeOwnerThread(r *Runtime, op string) {
	if r == nil || r.ref == nil {
		return
	}
	if C.IsRuntimeOwnerThread(r.ref) == 0 {
		panic(fmt.Sprintf("quickjs: %s must be called on the runtime owner thread", op))
	}
}

// RunGC will call quickjs's garbage collector.
func (r *Runtime) RunGC() {
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.RunGC")
	C.JS_RunGC(r.ref)
}

// Close will free the runtime pointer with proper cleanup.
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if r.ref == nil {
		r.releaseOSThread()
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.Close")

	// Step 1: Clear interrupt handler before closing
	r.ClearInterruptHandler()
	C.SetJSNewContextFailForTest(r.ref, C.int(0))

	// Step 2: Clean up runtime-scoped constructor registry
	clearConstructorRegistryForRuntime(r)

	// Step 3: Unregister runtime mapping
	unregisterRuntime(r.ref)
	C.UnregisterRuntimeOwnerThread(r.ref)

	// Step 4: Free QuickJS runtime
	if r.handlersInit {
		C.js_std_free_handlers(r.ref)
		r.handlersInit = false
	}
	C.JS_FreeRuntime(r.ref)
	r.ref = nil
	r.releaseOSThread()

}

func (r *Runtime) releaseOSThread() {
	if r == nil || !r.osThreadLocked {
		return
	}
	defer func() {
		_ = recover()
	}()
	runtime.UnlockOSThread()
	r.osThreadLocked = false
}

// SetCanBlock will set the runtime's can block; default is true
func (r *Runtime) SetCanBlock(canBlock bool) {
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.SetCanBlock")
	if canBlock {
		C.SetCanBlock(r.ref, C.int(1))
	} else {
		C.SetCanBlock(r.ref, C.int(0))
	}
}

// SetMemoryLimit the runtime memory limit; if not set, it will be unlimit.
func (r *Runtime) SetMemoryLimit(limit uint64) {
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.SetMemoryLimit")
	C.JS_SetMemoryLimit(r.ref, C.size_t(limit))
}

// SetGCThreshold the runtime's GC threshold; use -1 to disable automatic GC.
func (r *Runtime) SetGCThreshold(threshold int64) {
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.SetGCThreshold")
	C.JS_SetGCThreshold(r.ref, C.size_t(threshold))
}

// SetMaxStackSize will set max runtime's stack size;
func (r *Runtime) SetMaxStackSize(stack_size uint64) {
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.SetMaxStackSize")
	C.JS_SetMaxStackSize(r.ref, C.size_t(stack_size))
}

// SetExecuteTimeout will set the runtime's execute timeout;
// This will override any user interrupt handler (expected behavior)
func (r *Runtime) SetExecuteTimeout(timeout uint64) {
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.SetExecuteTimeout")
	C.SetExecuteTimeout(r.ref, C.time_t(timeout))
	// Clear user interrupt handler since timeout takes precedence
	r.interruptHandler = nil
}

// SetStripInfo sets the strip info for the runtime.
func (r *Runtime) SetStripInfo(strip int) {
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.SetStripInfo")
	C.SetStripInfo(r.ref, C.int(strip))
}

// SetModuleImport sets whether the runtime supports module import.
func (r *Runtime) SetModuleImport(moduleImport bool) {
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.SetModuleImport")
	C.JS_SetModuleLoaderFunc2(r.ref, (*C.JSModuleNormalizeFunc)(unsafe.Pointer(nil)), (*C.JSModuleLoaderFunc2)(C.js_module_loader), (*C.JSModuleCheckSupportedImportAttributes)(C.js_module_check_attributes), unsafe.Pointer(nil))
}

func currentTimeoutAllocationCount() int {
	return int(C.GetTimeoutAllocationCount())
}

func currentTimeoutRegistryEntryCount() int {
	return int(C.GetTimeoutRegistryEntryCount())
}

func setJSNewContextFailForTest(r *Runtime, enabled bool) {
	if r == nil || r.ref == nil {
		return
	}
	if enabled {
		C.SetJSNewContextFailForTest(r.ref, C.int(1))
		return
	}
	C.SetJSNewContextFailForTest(r.ref, C.int(0))
}

func setJSNewRuntimeFailForTest(enabled bool) {
	if enabled {
		C.SetJSNewRuntimeFailForTest(C.int(1))
		return
	}
	C.SetJSNewRuntimeFailForTest(C.int(0))
}

func setRuntimeNewContextFailStageForTest(stage int32) {
	runtimeNewContextFailStageForTest.Store(stage)
}

func setRuntimeNewContextInitCodeForTest(code string) {
	runtimeNewContextInitCodeForTest.Store(code)
}

func toExportedFinalizerObservabilitySnapshot(in finalizerObservabilitySnapshot) FinalizerObservabilitySnapshot {
	return FinalizerObservabilitySnapshot{
		Enabled:             in.Enabled,
		OpaqueNil:           in.OpaqueNil,
		OpaqueInvalid:       in.OpaqueInvalid,
		HandleInvalid:       in.HandleInvalid,
		ContextRefInvalid:   in.ContextRefInvalid,
		ContextNotFound:     in.ContextNotFound,
		ContextStateInvalid: in.ContextStateInvalid,
		RuntimeMismatch:     in.RuntimeMismatch,
		HandleMissing:       in.HandleMissing,
		Cleaned:             in.Cleaned,
	}
}

// SetFinalizerObservability enables or disables runtime-scoped finalizer branch observability.
// This is intended for tests and diagnostics.
func (r *Runtime) SetFinalizerObservability(enabled bool) {
	setFinalizerObservabilityForTest(r, enabled)
}

// ResetFinalizerObservability clears runtime-scoped finalizer branch counters.
// This is intended for tests and diagnostics.
func (r *Runtime) ResetFinalizerObservability() {
	resetFinalizerObservabilityForTest(r)
}

// SnapshotFinalizerObservability returns runtime-scoped finalizer branch counters.
// This is intended for tests and diagnostics.
func (r *Runtime) SnapshotFinalizerObservability() FinalizerObservabilitySnapshot {
	return toExportedFinalizerObservabilitySnapshot(snapshotFinalizerObservabilityForTest(r))
}

// SnapshotAndResetFinalizerObservability returns current counters and then clears them.
// This is intended for tests and diagnostics.
func (r *Runtime) SnapshotAndResetFinalizerObservability() FinalizerObservabilitySnapshot {
	return toExportedFinalizerObservabilitySnapshot(snapshotAndResetFinalizerObservabilityForTest(r))
}

// SetInterruptHandler sets a user interrupt handler using simplified approach.
// This will override any timeout handler (expected behavior)
func (r *Runtime) SetInterruptHandler(handler InterruptHandler) {
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.SetInterruptHandler")
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
	if r == nil || r.ref == nil {
		return
	}
	requireRuntimeOwnerThread(r, "Runtime.ClearInterruptHandler")
	r.interruptHandler = nil
	C.ClearInterruptHandler(r.ref)
}

// callInterruptHandler is called from C layer via runtime mapping (internal use)
func (r *Runtime) callInterruptHandler() int {
	if r == nil {
		return 0
	}
	if r.interruptHandler != nil {
		return r.interruptHandler()
	}
	return 0 // No interrupt
}

// NewContext creates a new JavaScript context.
func (r *Runtime) NewContext() *Context {
	if r == nil || r.ref == nil {
		return nil
	}
	requireRuntimeOwnerThread(r, "Runtime.NewContext")
	if !r.handlersInit {
		C.js_std_init_handlers(r.ref)
		r.handlersInit = true
	}

	// create a new context (heap, global object and context stack
	ctx_ref := C.JS_NewContext_Go(r.ref)
	if ctx_ref == nil {
		return nil
	}

	// import the 'std' and 'os' modules
	stdName := C.CString("std")
	defer C.free(unsafe.Pointer(stdName))
	osName := C.CString("os")
	defer C.free(unsafe.Pointer(osName))
	C.js_init_module_std(ctx_ref, stdName)
	C.js_init_module_os(ctx_ref, osName)

	// import setTimeout and clearTimeout from 'os' to globalThis
	code := runtimeNewContextDefaultInitCode
	if override, ok := runtimeNewContextInitCodeForTest.Load().(string); ok && override != "" {
		code = override
	}

	// Replace evaluation flags with function calls
	evalFlags := C.int(C.GetEvalTypeModule()) | C.int(C.GetEvalFlagCompileOnly())
	codePtr := C.CString(code)
	defer C.free(unsafe.Pointer(codePtr))
	filenamePtr := C.CString("init.js")
	defer C.free(unsafe.Pointer(filenamePtr))
	init_compile := C.JS_Eval(ctx_ref, codePtr, C.size_t(len(code)), filenamePtr, evalFlags)
	if runtimeNewContextFailStageForTest.Load() == runtimeNewContextFailStageCompile {
		C.JS_FreeValue(ctx_ref, init_compile)
		C.JS_FreeContext(ctx_ref)
		return nil
	}
	if C.JS_IsException_Wrapper(init_compile) == 1 {
		C.JS_FreeValue(ctx_ref, init_compile)
		C.JS_FreeContext(ctx_ref)
		return nil
	}

	init_exec := C.JS_EvalFunction(ctx_ref, init_compile)
	if runtimeNewContextFailStageForTest.Load() == runtimeNewContextFailStageExec {
		C.JS_FreeValue(ctx_ref, init_exec)
		C.JS_FreeContext(ctx_ref)
		return nil
	}
	if C.JS_IsException_Wrapper(init_exec) == 1 {
		C.JS_FreeValue(ctx_ref, init_exec)
		C.JS_FreeContext(ctx_ref)
		return nil
	}

	init_run := C.js_std_await(ctx_ref, init_exec)
	if runtimeNewContextFailStageForTest.Load() == runtimeNewContextFailStageAwait {
		C.JS_FreeValue(ctx_ref, init_run)
		C.JS_FreeContext(ctx_ref)
		return nil
	}
	if C.JS_IsException_Wrapper(init_run) == 1 {
		C.JS_FreeValue(ctx_ref, init_run)
		C.JS_FreeContext(ctx_ref)
		return nil
	}
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

	return ctx
}
