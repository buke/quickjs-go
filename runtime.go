package quickjs

/*
#include "bridge.h"
*/
import "C"
import (
	"runtime"
)

// Runtime represents a Javascript runtime corresponding to an object heap. Several runtimes can exist at the same time but they cannot exchange objects. Inside a given runtime, no multi-threading is supported.
type Runtime struct {
	ref *C.JSRuntime
}

// NewRuntime creates a new quickjs runtime.
func NewRuntime() Runtime {
	runtime.LockOSThread() // prevent multiple quickjs runtime from being created
	rt := Runtime{ref: C.JS_NewRuntime()}
	C.JS_SetCanBlock(rt.ref, C.int(1))
	return rt
}

// RunGC will call quickjs's garbage collector.
func (r Runtime) RunGC() {
	C.JS_RunGC(r.ref)
}

// Close will free the runtime pointer.
func (r Runtime) Close() {
	C.JS_FreeRuntime(r.ref)
}

// SetMemoryLimit the runtime memory limit; if not set, it will be unlimit.
func (r Runtime) SetMemoryLimit(limit uint32) {
	C.JS_SetMemoryLimit(r.ref, C.size_t(limit))
}

// SetGCThreshold the runtime's GC threshold; use -1 to disable automatic GC.
func (r Runtime) SetGCThreshold(threshold int64) {
	C.JS_SetGCThreshold(r.ref, C.size_t(threshold))
}

// SetMaxStackSize will set max runtime's stack size; default is 255
func (r Runtime) SetMaxStackSize(stack_size uint32) {
	C.JS_SetMaxStackSize(r.ref, C.size_t(stack_size))
}

// SetExecuteTimeout will set the runtime's execute timeout; default is 0
func (r Runtime) SetExecuteTimeout(timeout uint32) {
	C.SetExecuteTimeout(r.ref, C.long(timeout))
}

// NewContext creates a new JavaScript context.
// enable BigFloat/BigDecimal support and enable .
// enable operator overloading.
func (r Runtime) NewContext() *Context {
	C.js_std_init_handlers(r.ref)

	// create a new context (heap, global object and context stack
	ctx_ref := C.JS_NewContext(r.ref)

	C.JS_AddIntrinsicBigFloat(ctx_ref)
	C.JS_AddIntrinsicBigDecimal(ctx_ref)
	C.JS_AddIntrinsicOperators(ctx_ref)
	C.JS_EnableBignumExt(ctx_ref, C.int(1))

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
	// C.js_module_set_import_meta(ctx_ref, init_compile, 1, 1)
	init_run := C.JS_EvalFunction(ctx_ref, init_compile)
	C.JS_FreeValue(ctx_ref, init_run)
	// C.js_std_loop(ctx_ref)

	return &Context{ref: ctx_ref, runtime: &r}
}
