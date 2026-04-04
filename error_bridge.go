package quickjs

import "unsafe"

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

// proxyError represents a standardized error for proxy functions.
type proxyError struct {
	errorType string
	message   string
}

// Common proxy errors with consistent error messages.
var (
	errContextNotFound        = proxyError{"InternalError", "Context not found"}
	errFunctionNotFound       = proxyError{"InternalError", "Function not found"}
	errConstructorNotFound    = proxyError{"InternalError", "Constructor function not found"}
	errMethodNotFound         = proxyError{"InternalError", "Method function not found"}
	errGetterNotFound         = proxyError{"InternalError", "Getter function not found"}
	errSetterNotFound         = proxyError{"InternalError", "Setter function not found"}
	errInvalidFunctionType    = proxyError{"TypeError", "Invalid function type"}
	errInvalidConstructorType = proxyError{"TypeError", "Invalid constructor function type"}
	errInvalidMethodType      = proxyError{"TypeError", "Invalid method function type"}
	errInvalidGetterType      = proxyError{"TypeError", "Invalid getter function type"}
	errInvalidSetterType      = proxyError{"TypeError", "Invalid setter function type"}
)

// throwProxyError creates and returns a JavaScript error.
func throwProxyError(ctx *C.JSContext, err proxyError) C.JSValue {
	msg := C.CString(err.message)
	defer C.free(unsafe.Pointer(msg))

	switch err.errorType {
	case "TypeError":
		return C.ThrowTypeError(ctx, msg)
	default:
		return C.ThrowInternalError(ctx, msg)
	}
}

// throwModuleError creates and throws a module initialization error.
func throwModuleError(ctx *C.JSContext, err error) C.int {
	errorMsg := C.CString(err.Error())
	C.ThrowInternalError(ctx, errorMsg)
	C.free(unsafe.Pointer(errorMsg))
	return C.int(-1)
}
