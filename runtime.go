package quickjs

/*
#include "bridge.h"
*/
import "C"
import (
	"io"
	"runtime"
)

// Runtime represents a Javascript runtime corresponding to an object heap. Several runtimes can exist at the same time but they cannot exchange objects. Inside a given runtime, no multi-threading is supported.
type Runtime struct {
	ref  *C.JSRuntime
	Loop *Loop // only one loop per runtime
}

// NewRuntime creates a new quickjs runtime.
func NewRuntime() Runtime {
	runtime.LockOSThread() // prevent multiple quickjs runtime from being created
	rt := Runtime{ref: C.JS_NewRuntime(), Loop: NewLoop()}
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

// NewContext creates a new JavaScript context.
// enable BigFloat/BigDecimal support and enable .
// enable operator overloading.
func (r Runtime) NewContext() *Context {
	ref := C.JS_NewContext(r.ref)

	C.JS_AddIntrinsicBigFloat(ref)
	C.JS_AddIntrinsicBigDecimal(ref)
	C.JS_AddIntrinsicOperators(ref)
	C.JS_EnableBignumExt(ref, C.int(1))

	return &Context{ref: ref, runtime: &r}
}

// ExecutePendingJob will execute all pending jobs.
func (r Runtime) ExecutePendingJob() (Context, error) {
	var ctx Context

	err := C.JS_ExecutePendingJob(r.ref, &ctx.ref)
	if err <= 0 {
		if err == 0 {
			return ctx, io.EOF
		}
		return ctx, ctx.Exception()
	}

	return ctx, nil
}

// IsJobPending returns true if there is a pending job.
func (r Runtime) IsJobPending() bool {
	return C.JS_IsJobPending(r.ref) == 1
}

func (r Runtime) ExecuteAllPendingJobs() error {
	var err error
	for r.Loop.IsLoopPending() || r.IsJobPending() {
		// execute loop job
		r.Loop.Run()

		// excute promiIs
		_, err := r.ExecutePendingJob()
		if err == io.EOF {
			err = nil
		}
	}
	return err
}
