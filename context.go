package quickjs

import (
	"errors"
	"fmt"
	"os"
	"unsafe"
)

/*
#include <stdint.h> // for uintptr_t
#include "bridge.h"
*/
import "C"

// Context represents a Javascript context (or Realm). Each JSContext has its own global objects and system objects. There can be several JSContexts per JSRuntime and they can share objects, similar to frames of the same origin sharing Javascript objects in a web browser.
type Context struct {
	runtime     *Runtime
	ref         *C.JSContext
	globals     *Value
	handleStore *handleStore //  function handle storage
}

// Runtime returns the runtime of the context.
func (ctx *Context) Runtime() *Runtime {
	return ctx.runtime
}

// Free will free context and all associated objects.
func (ctx *Context) Close() {
	if ctx.globals != nil {
		ctx.globals.Free()
	}

	// Clean up all registered function handles (critical for memory management)
	if ctx.handleStore != nil {
		ctx.handleStore.Clear()
	}

	// Remove from global mapping
	unregisterContext(ctx.ref)

	C.JS_FreeContext(ctx.ref)
}

// Null return a null value.
func (ctx *Context) Null() Value {
	return Value{ctx: ctx, ref: C.JS_NewNull()}
}

// Undefined return a undefined value.
func (ctx *Context) Undefined() Value {
	return Value{ctx: ctx, ref: C.JS_NewUndefined()}
}

// Uninitialized returns a uninitialized value.
func (ctx *Context) Uninitialized() Value {
	return Value{ctx: ctx, ref: C.JS_NewUninitialized()}
}

// Error returns a new exception value with given message.
func (ctx *Context) Error(err error) Value {
	val := Value{ctx: ctx, ref: C.JS_NewError(ctx.ref)}
	val.Set("message", ctx.String(err.Error()))
	return val
}

// Bool returns a bool value with given bool.
func (ctx *Context) Bool(b bool) Value {
	bv := 0
	if b {
		bv = 1
	}
	return Value{ctx: ctx, ref: C.JS_NewBool(ctx.ref, C.int(bv))}
}

// Int32 returns a int32 value with given int32.
func (ctx *Context) Int32(v int32) Value {
	return Value{ctx: ctx, ref: C.JS_NewInt32(ctx.ref, C.int32_t(v))}
}

// Int64 returns a int64 value with given int64.
func (ctx *Context) Int64(v int64) Value {
	return Value{ctx: ctx, ref: C.JS_NewInt64(ctx.ref, C.int64_t(v))}
}

// Uint32 returns a uint32 value with given uint32.
func (ctx *Context) Uint32(v uint32) Value {
	return Value{ctx: ctx, ref: C.JS_NewUint32(ctx.ref, C.uint32_t(v))}
}

// BigInt64 returns a int64 value with given uint64.
func (ctx *Context) BigInt64(v int64) Value {
	return Value{ctx: ctx, ref: C.JS_NewBigInt64(ctx.ref, C.int64_t(v))}
}

// BigUint64 returns a uint64 value with given uint64.
func (ctx *Context) BigUint64(v uint64) Value {
	return Value{ctx: ctx, ref: C.JS_NewBigUint64(ctx.ref, C.uint64_t(v))}
}

// Float64 returns a float64 value with given float64.
func (ctx *Context) Float64(v float64) Value {
	return Value{ctx: ctx, ref: C.JS_NewFloat64(ctx.ref, C.double(v))}
}

// String returns a string value with given string.
func (ctx *Context) String(v string) Value {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))
	return Value{ctx: ctx, ref: C.JS_NewString(ctx.ref, ptr)}
}

// ArrayBuffer returns a ArrayBuffer value with given binary data.
func (ctx *Context) ArrayBuffer(binaryData []byte) Value {
	if len(binaryData) == 0 {
		return Value{ctx: ctx, ref: C.JS_NewArrayBufferCopy(ctx.ref, nil, 0)}
	}
	return Value{ctx: ctx, ref: C.JS_NewArrayBufferCopy(ctx.ref, (*C.uchar)(&binaryData[0]), C.size_t(len(binaryData)))}
}

// createTypedArray is a helper function to create TypedArray with given data and type.
// It creates an ArrayBuffer first, then constructs a TypedArray from it.
func (ctx *Context) createTypedArray(data unsafe.Pointer, elementCount int, elementSize int, arrayType C.JSTypedArrayEnum) Value {
	// Calculate total bytes needed for the data
	totalBytes := elementCount * elementSize

	// Convert raw data pointer to Go byte slice using unsafe.Slice (Go 1.17+)
	var bytes []byte
	if totalBytes > 0 && data != nil {
		bytes = unsafe.Slice((*byte)(data), totalBytes)
	}

	// Create ArrayBuffer from the byte data
	buffer := ctx.ArrayBuffer(bytes)
	defer buffer.Free()

	// Create TypedArray from ArrayBuffer: new TypedArray(buffer, offset, length)
	offset := C.JS_NewInt32(ctx.ref, C.int(0))            // Start from beginning of buffer
	length := C.JS_NewInt32(ctx.ref, C.int(elementCount)) // Number of elements (not bytes)

	// Pack arguments for JS_NewTypedArray call
	args := []C.JSValue{buffer.ref, offset, length}
	return Value{
		ctx: ctx,
		ref: C.JS_NewTypedArray(ctx.ref, C.int(len(args)), &args[0], arrayType),
	}
}

// Int8Array returns a Int8Array value with given int8 slice.
func (ctx *Context) Int8Array(data []int8) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 1, C.JS_TYPED_ARRAY_INT8)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 1, C.JS_TYPED_ARRAY_INT8)
}

// Uint8Array returns a Uint8Array value with given uint8 slice.
func (ctx *Context) Uint8Array(data []uint8) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 1, C.JS_TYPED_ARRAY_UINT8)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 1, C.JS_TYPED_ARRAY_UINT8)
}

// Uint8ClampedArray returns a Uint8ClampedArray value with given uint8 slice.
func (ctx *Context) Uint8ClampedArray(data []uint8) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 1, C.JS_TYPED_ARRAY_UINT8C)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 1, C.JS_TYPED_ARRAY_UINT8C)
}

// Int16Array returns a Int16Array value with given int16 slice.
func (ctx *Context) Int16Array(data []int16) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 2, C.JS_TYPED_ARRAY_INT16)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 2, C.JS_TYPED_ARRAY_INT16)
}

// Uint16Array returns a Uint16Array value with given uint16 slice.
func (ctx *Context) Uint16Array(data []uint16) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 2, C.JS_TYPED_ARRAY_UINT16)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 2, C.JS_TYPED_ARRAY_UINT16)
}

// Int32Array returns a Int32Array value with given int32 slice.
func (ctx *Context) Int32Array(data []int32) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 4, C.JS_TYPED_ARRAY_INT32)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 4, C.JS_TYPED_ARRAY_INT32)
}

// Uint32Array returns a Uint32Array value with given uint32 slice.
func (ctx *Context) Uint32Array(data []uint32) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 4, C.JS_TYPED_ARRAY_UINT32)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 4, C.JS_TYPED_ARRAY_UINT32)
}

// Float32Array returns a Float32Array value with given float32 slice.
func (ctx *Context) Float32Array(data []float32) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 4, C.JS_TYPED_ARRAY_FLOAT32)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 4, C.JS_TYPED_ARRAY_FLOAT32)
}

// Float64Array returns a Float64Array value with given float64 slice.
func (ctx *Context) Float64Array(data []float64) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 8, C.JS_TYPED_ARRAY_FLOAT64)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 8, C.JS_TYPED_ARRAY_FLOAT64)
}

// BigInt64Array returns a BigInt64Array value with given int64 slice.
func (ctx *Context) BigInt64Array(data []int64) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 8, C.JS_TYPED_ARRAY_BIG_INT64)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 8, C.JS_TYPED_ARRAY_BIG_INT64)
}

// BigUint64Array returns a BigUint64Array value with given uint64 slice.
func (ctx *Context) BigUint64Array(data []uint64) Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 8, C.JS_TYPED_ARRAY_BIG_UINT64)
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 8, C.JS_TYPED_ARRAY_BIG_UINT64)
}

// Object returns a new object value.
func (ctx *Context) Object() Value {
	return Value{ctx: ctx, ref: C.JS_NewObject(ctx.ref)}
}

// ParseJson parses given json string and returns a object value.
func (ctx *Context) ParseJSON(v string) Value {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))

	filenamePtr := C.CString("")
	defer C.free(unsafe.Pointer(filenamePtr))

	return Value{ctx: ctx, ref: C.JS_ParseJSON(ctx.ref, ptr, C.size_t(len(v)), filenamePtr)}
}

// Function returns a js function value with given function template
// New implementation using HandleStore and JS_NewCFunction2 with magic parameter
func (ctx *Context) Function(fn func(*Context, Value, []Value) Value) Value {
	// Store function in HandleStore and get int32 ID
	fnID := ctx.handleStore.Store(fn)

	return Value{
		ctx: ctx,
		ref: C.JS_NewCFunction2(
			ctx.ref,
			(*C.JSCFunction)(unsafe.Pointer(C.GoFunctionProxy)),
			nil,                      // name (can be set later)
			0,                        // length (auto-detected)
			C.JS_CFUNC_generic_magic, // enable magic parameter support
			C.int(fnID),              // magic parameter passes function ID
		),
	}
}

// AsyncFunction returns a js async function value with given function template
//
// Deprecated: Use Context.Function + Context.Promise instead for better memory management and thread safety.
// Example:
//
//	asyncFn := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
//	    return ctx.Promise(func(resolve, reject func(quickjs.Value)) {
//	        // async work here
//	        resolve(ctx.String("result"))
//	    })
//	})
func (ctx *Context) AsyncFunction(asyncFn func(ctx *Context, this Value, promise Value, args []Value) Value) Value {
	// New implementation using Function + Promise
	return ctx.Function(func(ctx *Context, this Value, args []Value) Value {
		return ctx.Promise(func(resolve, reject func(Value)) {
			// Create a promise object that has resolve/reject methods
			promiseObj := ctx.Object()
			promiseObj.Set("resolve", ctx.Function(func(ctx *Context, this Value, args []Value) Value {
				if len(args) > 0 {
					resolve(args[0])
				} else {
					resolve(ctx.Undefined())
				}
				return ctx.Undefined()
			}))
			promiseObj.Set("reject", ctx.Function(func(ctx *Context, this Value, args []Value) Value {
				if len(args) > 0 {
					reject(args[0])
				} else {
					errObj := ctx.Error(fmt.Errorf("Promise rejected without reason"))
					defer errObj.Free() // Free the error object
					reject(errObj)
				}
				return ctx.Undefined()
			}))
			defer promiseObj.Free()

			// Call the original async function with the promise object
			result := asyncFn(ctx, this, promiseObj, args)

			// If the function returned a value directly (not using promise.resolve/reject),
			// we resolve with that value
			if !result.IsUndefined() {
				resolve(result)
				result.Free() // Free the result if it's not undefined
			}

		})
	})
}

// getFunction gets function from HandleStore (internal use)
func (ctx *Context) loadFunctionFromHandleID(id int32) interface{} {
	fn, _ := ctx.handleStore.Load(id)
	return fn
}

// SetInterruptHandler sets a interrupt handler.
//
// Deprecated: Use SetInterruptHandler on runtime instead
func (ctx *Context) SetInterruptHandler(handler InterruptHandler) {
	ctx.runtime.SetInterruptHandler(handler)
}

// Atom returns a new Atom value with given string.
func (ctx *Context) Atom(v string) Atom {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))
	return Atom{ctx: ctx, ref: C.JS_NewAtom(ctx.ref, ptr)}
}

// Atom returns a new Atom value with given idx.
func (ctx *Context) AtomIdx(idx uint32) Atom {
	return Atom{ctx: ctx, ref: C.JS_NewAtomUInt32(ctx.ref, C.uint32_t(idx))}
}

// Invoke invokes a function with given this value and arguments.
func (ctx *Context) Invoke(fn Value, this Value, args ...Value) Value {
	cargs := []C.JSValue{}
	for _, x := range args {
		cargs = append(cargs, x.ref)
	}
	var val Value
	if len(cargs) == 0 {
		val = Value{ctx: ctx, ref: C.JS_Call(ctx.ref, fn.ref, this.ref, 0, nil)}
	} else {
		val = Value{ctx: ctx, ref: C.JS_Call(ctx.ref, fn.ref, this.ref, C.int(len(cargs)), &cargs[0])}
	}
	return val
}

type EvalOptions struct {
	js_eval_type_global       bool
	js_eval_type_module       bool
	js_eval_flag_strict       bool
	js_eval_flag_compile_only bool
	filename                  string
	await                     bool
	load_only                 bool
}

type EvalOption func(*EvalOptions)

func EvalFlagGlobal(global bool) EvalOption {
	return func(flags *EvalOptions) {
		flags.js_eval_type_global = global
	}
}

func EvalFlagModule(module bool) EvalOption {
	return func(flags *EvalOptions) {
		flags.js_eval_type_module = module
	}
}

func EvalFlagStrict(strict bool) EvalOption {
	return func(flags *EvalOptions) {
		flags.js_eval_flag_strict = strict
	}
}

func EvalFlagCompileOnly(compileOnly bool) EvalOption {
	return func(flags *EvalOptions) {
		flags.js_eval_flag_compile_only = compileOnly
	}
}

func EvalFileName(filename string) EvalOption {
	return func(flags *EvalOptions) {
		flags.filename = filename
	}
}

func EvalAwait(await bool) EvalOption {
	return func(flags *EvalOptions) {
		flags.await = await
	}
}

func EvalLoadOnly(loadOnly bool) EvalOption {
	return func(flags *EvalOptions) {
		flags.load_only = loadOnly
	}
}

// Eval returns a js value with given code.
// Need call Free() `quickjs.Value`'s returned by `Eval()` and `EvalFile()` and `EvalBytecode()`.
// func (ctx *Context) Eval(code string) (Value, error) { return ctx.EvalFile(code, "code") }
func (ctx *Context) Eval(code string, opts ...EvalOption) (Value, error) {
	options := EvalOptions{
		js_eval_type_global: true,
		filename:            "<input>",
		await:               false,
	}
	for _, fn := range opts {
		fn(&options)
	}

	cFlag := C.int(0)
	if options.js_eval_type_global {
		cFlag |= C.JS_EVAL_TYPE_GLOBAL
	}
	if options.js_eval_type_module {
		cFlag |= C.JS_EVAL_TYPE_MODULE
	}
	if options.js_eval_flag_strict {
		cFlag |= C.JS_EVAL_FLAG_STRICT
	}
	if options.js_eval_flag_compile_only {
		cFlag |= C.JS_EVAL_FLAG_COMPILE_ONLY
	}

	codePtr := C.CString(code)
	defer C.free(unsafe.Pointer(codePtr))

	filenamePtr := C.CString(options.filename)
	defer C.free(unsafe.Pointer(filenamePtr))

	if C.JS_DetectModule(codePtr, C.size_t(len(code))) != 0 {
		cFlag |= C.JS_EVAL_TYPE_MODULE
	}

	var val Value
	if options.await {
		val = Value{ctx: ctx, ref: C.js_std_await(ctx.ref, C.JS_Eval(ctx.ref, codePtr, C.size_t(len(code)), filenamePtr, cFlag))}
	} else {
		val = Value{ctx: ctx, ref: C.JS_Eval(ctx.ref, codePtr, C.size_t(len(code)), filenamePtr, cFlag)}
	}
	if val.IsException() {
		return val, ctx.Exception()
	}

	return val, nil
}

// EvalFile returns a js value with given code and filename.
// Need call Free() `quickjs.Value`'s returned by `Eval()` and `EvalFile()` and `EvalBytecode()`.
func (ctx *Context) EvalFile(filePath string, opts ...EvalOption) (Value, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return ctx.Null(), err
	}
	opts = append(opts, EvalFileName(filePath))
	return ctx.Eval(string(b), opts...)
}

// LoadModule returns a js value with given code and module name.
func (ctx *Context) LoadModule(code string, moduleName string, opts ...EvalOption) (Value, error) {
	options := EvalOptions{
		load_only: false,
	}
	for _, fn := range opts {
		fn(&options)
	}

	codePtr := C.CString(code)
	defer C.free(unsafe.Pointer(codePtr))

	if C.JS_DetectModule(codePtr, C.size_t(len(code))) == 0 {
		return ctx.Null(), fmt.Errorf("not a module")
	}

	codeByte, err := ctx.Compile(code, EvalFlagModule(true), EvalFlagCompileOnly(true), EvalFileName(moduleName))
	if err != nil {
		return ctx.Null(), err
	}

	return ctx.LoadModuleBytecode(codeByte, EvalLoadOnly(options.load_only))

}

// LoadModuleFile returns a js value with given file path and module name.
func (ctx *Context) LoadModuleFile(filePath string, moduleName string) (Value, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return ctx.Null(), err
	}
	return ctx.LoadModule(string(b), moduleName)
}

// CompileModule returns a compiled bytecode with given code and module name.
func (ctx *Context) CompileModule(filePath string, moduleName string, opts ...EvalOption) ([]byte, error) {
	opts = append(opts, EvalFileName(moduleName))
	return ctx.CompileFile(filePath, opts...)
}

// LoadModuleByteCode returns a js value with given bytecode and module name.
func (ctx *Context) LoadModuleBytecode(buf []byte, opts ...EvalOption) (Value, error) {
	if len(buf) == 0 {
		return ctx.Null(), fmt.Errorf("empty bytecode")
	}

	options := EvalOptions{}
	for _, fn := range opts {
		fn(&options)
	}

	var cLoadOnlyFlag C.int = 0
	if options.load_only {
		cLoadOnlyFlag = 1
	}

	// Use our custom LoadModuleBytecode function instead of js_std_eval_binary
	cVal := C.LoadModuleBytecode(ctx.ref, (*C.uint8_t)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)), cLoadOnlyFlag)

	if C.JS_IsException(cVal) == 1 {
		return ctx.Null(), ctx.Exception()
	}

	return Value{ctx: ctx, ref: cVal}, nil
}

// EvalBytecode returns a js value with given bytecode.
// Need call Free() `quickjs.Value`'s returned by `Eval()` and `EvalFile()` and `EvalBytecode()`.
func (ctx *Context) EvalBytecode(buf []byte) (Value, error) {
	cbuf := C.CBytes(buf)
	obj := Value{ctx: ctx, ref: C.JS_ReadObject(ctx.ref, (*C.uint8_t)(cbuf), C.size_t(len(buf)), C.JS_READ_OBJ_BYTECODE)}
	defer C.js_free(ctx.ref, unsafe.Pointer(cbuf))
	if obj.IsException() {
		return obj, ctx.Exception()
	}

	val := Value{ctx: ctx, ref: C.JS_EvalFunction(ctx.ref, obj.ref)}
	if val.IsException() {
		return val, ctx.Exception()
	}

	return val, nil
}

// Compile returns a compiled bytecode with given code.
func (ctx *Context) Compile(code string, opts ...EvalOption) ([]byte, error) {
	opts = append(opts, EvalFlagCompileOnly(true))
	val, err := ctx.Eval(code, opts...)
	if err != nil {
		return nil, err
	}
	defer val.Free()

	var kSize C.size_t = 0
	ptr := C.JS_WriteObject(ctx.ref, &kSize, val.ref, C.JS_WRITE_OBJ_BYTECODE)

	if ptr == nil {
		return nil, ctx.Exception()
	}

	defer C.js_free(ctx.ref, unsafe.Pointer(ptr))

	ret := make([]byte, C.int(kSize))
	if kSize > 0 {
		copy(ret, C.GoBytes(unsafe.Pointer(ptr), C.int(kSize)))
	}

	return ret, nil
}

// Compile returns a compiled bytecode with given filename.
func (ctx *Context) CompileFile(filePath string, opts ...EvalOption) ([]byte, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	options := EvalOptions{}
	for _, fn := range opts {
		fn(&options)
	}
	if options.filename == "" {
		opts = append(opts, EvalFileName(filePath))
	}

	return ctx.Compile(string(b), opts...)
}

// Global returns a context's global object.
func (ctx *Context) Globals() Value {
	if ctx.globals == nil {
		ctx.globals = &Value{
			ctx: ctx,
			ref: C.JS_GetGlobalObject(ctx.ref),
		}
	}
	return *ctx.globals
}

// Throw returns a context's exception value.
func (ctx *Context) Throw(v Value) Value {
	return Value{ctx: ctx, ref: C.JS_Throw(ctx.ref, v.ref)}
}

// ThrowError returns a context's exception value with given error message.
func (ctx *Context) ThrowError(err error) Value {
	return ctx.Throw(ctx.Error(err))
}

// ThrowSyntaxError returns a context's exception value with given error message.
func (ctx *Context) ThrowSyntaxError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowSyntaxError(ctx.ref, causePtr)}
}

// ThrowTypeError returns a context's exception value with given error message.
func (ctx *Context) ThrowTypeError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowTypeError(ctx.ref, causePtr)}
}

// ThrowReferenceError returns a context's exception value with given error message.
func (ctx *Context) ThrowReferenceError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowReferenceError(ctx.ref, causePtr)}
}

// ThrowRangeError returns a context's exception value with given error message.
func (ctx *Context) ThrowRangeError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowRangeError(ctx.ref, causePtr)}
}

// ThrowInternalError returns a context's exception value with given error message.
func (ctx *Context) ThrowInternalError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowInternalError(ctx.ref, causePtr)}
}

// HasException checks if the context has an exception set.
func (ctx *Context) HasException() bool {
	// Check if the context has an exception set
	return C.JS_HasException(ctx.ref) == 1
}

// Exception returns a context's exception value.
func (ctx *Context) Exception() error {
	val := Value{ctx: ctx, ref: C.JS_GetException(ctx.ref)}
	defer val.Free()
	return val.Error()
}

// Loop runs the context's event loop.
func (ctx *Context) Loop() {
	C.js_std_loop(ctx.ref)
}

// Wait for a promise and execute pending jobs while waiting for it. Return the promise result or JS_EXCEPTION in case of promise rejection.
func (ctx *Context) Await(v Value) (Value, error) {
	val := Value{ctx: ctx, ref: C.js_std_await(ctx.ref, v.ref)}
	if val.IsException() {
		return val, ctx.Exception()
	}
	return val, nil
}

// Promise creates a new Promise with executor function
// Executor runs synchronously in current thread for thread safety
func (ctx *Context) Promise(executor func(resolve, reject func(Value))) Value {
	// Create Promise using JavaScript code to avoid complex C API reference management
	promiseSetup, _ := ctx.Eval(`
        (() => {
            let _resolve, _reject;
            const promise = new Promise((resolve, reject) => {
                _resolve = resolve;
                _reject = reject;
            });
            return { promise, resolve: _resolve, reject: _reject };
        })()
    `)

	defer promiseSetup.Free()

	promise := promiseSetup.Get("promise")
	resolveFunc := promiseSetup.Get("resolve")
	rejectFunc := promiseSetup.Get("reject")
	defer resolveFunc.Free()
	defer rejectFunc.Free()

	// Create wrapper functions that call JavaScript resolve/reject
	resolve := func(result Value) {
		resolveFunc.Execute(ctx.Undefined(), result)
	}

	reject := func(reason Value) {
		rejectFunc.Execute(ctx.Undefined(), reason)
	}

	// Execute user function synchronously
	executor(resolve, reject)

	return promise
}

// =============================================================================
// CLASS BINDING METHODS
// =============================================================================

// CreateClass creates and registers a JavaScript class using the ClassBuilder pattern
// This method delegates to the class.go implementation for consistency
func (ctx *Context) CreateClass(builder *ClassBuilder) (Value, uint32, error) {
	return ctx.createClass(builder)
}

// CreateInstanceFromNewTarget creates a class instance using new_target and classID
// This helper method implements the standard pattern from point.c constructor
// and supports inheritance through proper prototype chain setup
func (ctx *Context) CreateInstanceFromNewTarget(newTarget Value, classID uint32, goObj interface{}) Value {
	// Store Go object in HandleStore for automatic memory management
	handleID := ctx.handleStore.Store(goObj)

	// Get prototype from new_target (supports inheritance)
	// This corresponds to point.c: proto = JS_GetPropertyStr(ctx, new_target, "prototype")
	proto := newTarget.Get("prototype")
	if proto.IsException() {
		ctx.handleStore.Delete(handleID)
		return proto
	}
	defer proto.Free()

	// Create JS object with correct prototype and class
	// This corresponds to point.c: obj = JS_NewObjectProtoClass(ctx, proto, js_point_class_id)
	jsObj := C.JS_NewObjectProtoClass(ctx.ref, proto.ref, C.JSClassID(classID))
	if C.JS_IsException(jsObj) != 0 {
		ctx.handleStore.Delete(handleID)
		return Value{ctx: ctx, ref: jsObj}
	}

	// Associate Go object with JS object
	// This corresponds to point.c: JS_SetOpaque(obj, s)
	// Note: Safe conversion of integer ID to opaque pointer for QuickJS storage
	C.JS_SetOpaque(jsObj, unsafe.Pointer(uintptr(handleID)))

	return Value{ctx: ctx, ref: jsObj}
}

// GetInstanceData retrieves Go object from JavaScript class instance
// This method extracts the opaque data stored by CreateInstanceFromNewTarget
func (ctx *Context) GetInstanceData(val Value) (interface{}, error) {
	// First check if the value is an object
	if !val.IsObject() {
		return nil, errors.New("value is not an object")
	}

	// Get class ID to ensure we have a class instance
	classID := C.JS_GetClassID(val.ref)
	if classID == C.JS_INVALID_CLASS_ID {
		return nil, errors.New("value is not a class instance")
	}

	// Use JS_GetOpaque2 for type-safe retrieval with context validation
	// This corresponds to point.c: s = JS_GetOpaque2(ctx, this_val, js_point_class_id)
	opaque := C.JS_GetOpaque2(ctx.ref, val.ref, classID)
	if opaque == nil {
		return nil, errors.New("no instance data found")
	}

	// Convert opaque pointer back to handle ID
	// Note: Safe conversion back to integer ID for HandleStore lookup
	handleID := int32(uintptr(opaque))

	// Retrieve Go object from HandleStore
	if obj, exists := ctx.handleStore.Load(handleID); exists {
		return obj, nil
	}

	return nil, errors.New("instance data not found in handle store")
}

// GetInstanceDataTyped retrieves Go object from JavaScript class instance with type assertion
// This is a convenience method that combines GetInstanceData with type assertion
func (ctx *Context) GetInstanceDataTyped(val Value, expectedClassID uint32) (interface{}, error) {
	// First verify this is the expected class type
	if !val.IsInstanceOfClass(expectedClassID) {
		return nil, errors.New("value is not an instance of expected class")
	}

	// Use JS_GetOpaque2 with specific class ID for maximum type safety
	// This corresponds to point.c pattern: s = JS_GetOpaque2(ctx, this_val, js_point_class_id)
	opaque := C.JS_GetOpaque2(ctx.ref, val.ref, C.JSClassID(expectedClassID))
	if opaque == nil {
		return nil, errors.New("no instance data found for expected class")
	}

	// Convert opaque pointer back to handle ID
	handleID := int32(uintptr(opaque))

	// Retrieve Go object from HandleStore
	if obj, exists := ctx.handleStore.Load(handleID); exists {
		return obj, nil
	}

	return nil, errors.New("instance data not found in handle store")
}

// IsInstanceOf checks if a JavaScript value is an instance of a given class
// This provides a Go-friendly wrapper around JS_GetClassID comparison
func (ctx *Context) IsInstanceOf(val Value, expectedClassID uint32) bool {
	if !val.IsObject() {
		return false
	}

	objClassID := C.JS_GetClassID(val.ref)
	if objClassID == C.JS_INVALID_CLASS_ID {
		return false
	}

	return uint32(objClassID) == expectedClassID
}
