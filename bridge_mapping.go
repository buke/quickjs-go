package quickjs

import "sync"

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

var (
	// Global context mapping using sync.Map for lock-free performance.
	contextMapping sync.Map // map[*C.JSContext]*Context

	// Global runtime mapping for interrupt handler access.
	runtimeMapping sync.Map // map[*C.JSRuntime]*Runtime
)

// registerContext registers Go Context with C JSContext (internal use).
func registerContext(cCtx *C.JSContext, goCtx *Context) {
	contextMapping.Store(cCtx, goCtx)
}

// unregisterContext removes mapping when Context is closed (internal use).
func unregisterContext(cCtx *C.JSContext) {
	contextMapping.Delete(cCtx)
}

// getContextFromJS gets Go Context from C JSContext (internal use).
func getContextFromJS(cCtx *C.JSContext) *Context {
	if value, ok := contextMapping.Load(cCtx); ok {
		ctx, ok := value.(*Context)
		if !ok {
			contextMapping.Delete(cCtx)
			return nil
		}
		if !ctx.hasValidRef() {
			contextMapping.Delete(cCtx)
			return nil
		}
		return ctx
	}
	return nil
}

// registerRuntime registers Runtime for interrupt handler access.
func registerRuntime(cRt *C.JSRuntime, goRt *Runtime) {
	runtimeMapping.Store(cRt, goRt)
}

// unregisterRuntime removes Runtime mapping when closed.
func unregisterRuntime(cRt *C.JSRuntime) {
	runtimeMapping.Delete(cRt)
}

// getRuntimeFromJS gets Go Runtime from C JSRuntime (internal use).
func getRuntimeFromJS(cRt *C.JSRuntime) *Runtime {
	if value, ok := runtimeMapping.Load(cRt); ok {
		rt, ok := value.(*Runtime)
		if !ok {
			runtimeMapping.Delete(cRt)
			return nil
		}
		if !rt.isAlive() {
			runtimeMapping.Delete(cRt)
			return nil
		}
		return rt
	}
	return nil
}
