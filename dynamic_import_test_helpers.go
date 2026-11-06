package quickjs

/*
#include "bridge.h"
*/
import "C"
import "unsafe"

func drainPendingJobsForTest(rt *Runtime) {
	for {
		var jobCtx *C.JSContext
		if C.JS_ExecutePendingJob(rt.ref, &jobCtx) <= 0 {
			return
		}
	}
}

func invokeGoDynamicImportForTest(ctx *Context, basename, filename string) {
	basePtr := C.CString(basename)
	filePtr := C.CString(filename)
	defer C.free(unsafe.Pointer(basePtr))
	defer C.free(unsafe.Pointer(filePtr))
	goDynamicImport(ctx.ref, basePtr, filePtr, C.JS_NewObject(ctx.ref), C.JS_NewUndefined())
}

func forceFinishDynamicImportFailureForTest(enable bool) func() {
	prev := finishDynamicImportResultHook
	if enable {
		finishDynamicImportResultHook = func(ret C.int) C.int { return 1 }
	} else {
		finishDynamicImportResultHook = nil
	}
	return func() { finishDynamicImportResultHook = prev }
}
