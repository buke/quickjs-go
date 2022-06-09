package quickjs

/*
#include "bridge.h"
*/
import "C"

// Runtime represents a Javascript runtime corresponding to an object heap. Several runtimes can exist at the same time but they cannot exchange objects. Inside a given runtime, no multi-threading is supported.
type Runtime struct {
	ref *C.JSRuntime
}

// NewRuntime creates a new quickjs runtime.
func NewRuntime() Runtime {
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
	C.JS_SetMemoryLimit(r.ref, C.ulong(limit))
}

// SetGCThreshold the runtime's GC threshold; use -1 to disable automatic GC.
func (r Runtime) SetGCThreshold(threshold int64) {
	C.JS_SetGCThreshold(r.ref, C.ulong(threshold))
}

// SetMaxStackSize will set max runtime's stack size; default is 255
func (r Runtime) SetMaxStackSize(stack_size uint32) {
	C.JS_SetMaxStackSize(r.ref, C.ulong(stack_size))
}

// NewContext creates a new JavaScript context.
// enable BigFloat/BigDecimal support and enable .
// enable operator overloading.
func (r Runtime) NewContext() *Context {
	ref := C.JS_NewContext(r.ref)

	C.JS_AddIntrinsicBigFloat(ref)
	C.JS_AddIntrinsicBigDecimal(ref)
	C.JS_AddIntrinsicOperators(ref)
	C.JS_EnableBignumExt(ref, C.int(1))

	return &Context{ref: ref}
}
