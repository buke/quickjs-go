package quickjs

import (
	"fmt"
	"os"
	"sync/atomic"
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
	jobQueue    chan func(*Context)
	jobClosed   chan struct{}
}

const defaultJobQueueSize = 1024

// awaitPromiseStateHook and awaitExecutePendingJobHook are used only in tests to
// force specific Await code paths; they must remain nil in production.
var (
	awaitPromiseStateHook      func(ctx *Context, promise *Value, current int) (int, bool)
	awaitExecutePendingJobHook func(ctx *Context, promise *Value, current int) (int, bool)
)

func (ctx *Context) initScheduler() {
	ctx.jobQueue = make(chan func(*Context), defaultJobQueueSize)
	ctx.jobClosed = make(chan struct{})
}

// Schedule enqueues a job to be executed on the Context's thread.
// Jobs run sequentially via ProcessJobs to keep QuickJS access single-threaded.
func (ctx *Context) Schedule(job func(*Context)) bool {
	if ctx == nil || ctx.jobQueue == nil || job == nil {
		return false
	}
	select {
	case <-ctx.jobClosed:
		return false
	default:
	}
	select {
	case ctx.jobQueue <- job:
		return true
	case <-ctx.jobClosed:
		return false
	}
}

// ProcessJobs drains all pending scheduled jobs.
// Call this regularly (e.g., inside Loop or Await) to allow resolve/reject handlers to run.
func (ctx *Context) ProcessJobs() {
	if ctx == nil || ctx.jobQueue == nil {
		return
	}
	for {
		select {
		case job := <-ctx.jobQueue:
			if job != nil {
				job(ctx)
			}
		default:
			return
		}
	}
}

func (ctx *Context) drainJobs() {
	if ctx == nil || ctx.jobQueue == nil {
		return
	}
	for {
		select {
		case <-ctx.jobQueue:
			continue
		default:
			return
		}
	}
}

func (ctx *Context) duplicateValue(val *Value) *Value {
	if val == nil || val.ctx == nil {
		return nil
	}
	return &Value{ctx: ctx, ref: C.JS_DupValue_Go(ctx.ref, val.ref)}
}

func (ctx *Context) wrapPromiseCallback(fn *Value) (func(*Value), func()) {
	if fn == nil {
		noop := func(*Value) {}
		return noop, func() {}
	}
	fnRef := ctx.duplicateValue(fn)
	var consumed atomic.Bool
	release := func() {
		if !consumed.Swap(true) && fnRef != nil {
			fnRef.Free()
		}
	}
	callback := func(arg *Value) {
		if consumed.Swap(true) {
			return
		}
		dupArg := ctx.duplicateValue(arg)
		job := func(inner *Context) {
			defer fnRef.Free()
			var callArg *Value
			if dupArg != nil {
				callArg = dupArg
				defer dupArg.Free()
			} else {
				callArg = inner.NewUndefined()
				defer callArg.Free()
			}

			thisVal := inner.NewUndefined()
			defer thisVal.Free()

			result := fnRef.Execute(thisVal, callArg)
			if result != nil {
				result.Free()
			}
		}

		if !ctx.Schedule(job) {
			if dupArg != nil {
				dupArg.Free()
			}
			fnRef.Free()
		}
	}
	return callback, release
}

// Runtime returns the runtime of the context.
func (ctx *Context) Runtime() *Runtime {
	return ctx.runtime
}

// Free will free context and all associated objects.
func (ctx *Context) Close() {
	if ctx.jobClosed != nil {
		select {
		case <-ctx.jobClosed:
		default:
			close(ctx.jobClosed)
		}
	}
	ctx.drainJobs()

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

// NewNull returns a null value.
func (ctx *Context) NewNull() *Value {
	return &Value{ctx: ctx, ref: C.JS_NewNull()}
}

// Null return a null value.
// Deprecated: Use NewNull() instead.
func (ctx *Context) Null() *Value {
	return ctx.NewNull()
}

// NewUndefined returns a undefined value.
func (ctx *Context) NewUndefined() *Value {
	return &Value{ctx: ctx, ref: C.JS_NewUndefined()}
}

// Undefined return a undefined value.
// Deprecated: Use NewUndefined() instead.
func (ctx *Context) Undefined() *Value {
	return ctx.NewUndefined()
}

// NewUninitialized returns a uninitialized value.
func (ctx *Context) NewUninitialized() *Value {
	return &Value{ctx: ctx, ref: C.JS_NewUninitialized()}
}

// Uninitialized returns a uninitialized value.
// Deprecated: Use NewUninitialized() instead.
func (ctx *Context) Uninitialized() *Value {
	return ctx.NewUninitialized()
}

// NewError returns a new exception value with given message.
func (ctx *Context) NewError(err error) *Value {
	val := &Value{ctx: ctx, ref: C.JS_NewError(ctx.ref)}
	val.Set("message", ctx.NewString(err.Error()))
	return val
}

// Error returns a new exception value with given message.
// Deprecated: Use NewError() instead.
func (ctx *Context) Error(err error) *Value {
	return ctx.NewError(err)
}

// NewBool returns a bool value with given bool.
func (ctx *Context) NewBool(b bool) *Value {
	bv := 0
	if b {
		bv = 1
	}
	return &Value{ctx: ctx, ref: C.JS_NewBool(ctx.ref, C.int(bv))}
}

// Bool returns a bool value with given bool.
// Deprecated: Use NewBool() instead.
func (ctx *Context) Bool(b bool) *Value {
	return ctx.NewBool(b)
}

// NewInt32 returns a int32 value with given int32.
func (ctx *Context) NewInt32(v int32) *Value {
	return &Value{ctx: ctx, ref: C.JS_NewInt32(ctx.ref, C.int32_t(v))}
}

// Int32 returns a int32 value with given int32.
// Deprecated: Use NewInt32() instead.
func (ctx *Context) Int32(v int32) *Value {
	return ctx.NewInt32(v)
}

// NewInt64 returns a int64 value with given int64.
func (ctx *Context) NewInt64(v int64) *Value {
	return &Value{ctx: ctx, ref: C.JS_NewInt64(ctx.ref, C.int64_t(v))}
}

// Int64 returns a int64 value with given int64.
// Deprecated: Use NewInt64() instead.
func (ctx *Context) Int64(v int64) *Value {
	return ctx.NewInt64(v)
}

// NewUint32 returns a uint32 value with given uint32.
func (ctx *Context) NewUint32(v uint32) *Value {
	return &Value{ctx: ctx, ref: C.JS_NewUint32(ctx.ref, C.uint32_t(v))}
}

// Uint32 returns a uint32 value with given uint32.
// Deprecated: Use NewUint32() instead.
func (ctx *Context) Uint32(v uint32) *Value {
	return ctx.NewUint32(v)
}

// NewBigInt64 returns a int64 value with given uint64.
func (ctx *Context) NewBigInt64(v int64) *Value {
	return &Value{ctx: ctx, ref: C.JS_NewBigInt64(ctx.ref, C.int64_t(v))}
}

// BigInt64 returns a int64 value with given uint64.
// Deprecated: Use NewBigInt64() instead.
func (ctx *Context) BigInt64(v int64) *Value {
	return ctx.NewBigInt64(v)
}

// NewBigUint64 returns a uint64 value with given uint64.
func (ctx *Context) NewBigUint64(v uint64) *Value {
	return &Value{ctx: ctx, ref: C.JS_NewBigUint64(ctx.ref, C.uint64_t(v))}
}

// BigUint64 returns a uint64 value with given uint64.
// Deprecated: Use NewBigUint64() instead.
func (ctx *Context) BigUint64(v uint64) *Value {
	return ctx.NewBigUint64(v)
}

// NewFloat64 returns a float64 value with given float64.
func (ctx *Context) NewFloat64(v float64) *Value {
	return &Value{ctx: ctx, ref: C.JS_NewFloat64(ctx.ref, C.double(v))}
}

// Float64 returns a float64 value with given float64.
// Deprecated: Use NewFloat64() instead.
func (ctx *Context) Float64(v float64) *Value {
	return ctx.NewFloat64(v)
}

// NewString returns a string value with given string.
func (ctx *Context) NewString(v string) *Value {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))
	return &Value{ctx: ctx, ref: C.JS_NewString(ctx.ref, ptr)}
}

// String returns a string value with given string.
// Deprecated: Use NewString() instead.
func (ctx *Context) String(v string) *Value {
	return ctx.NewString(v)
}

// NewArrayBuffer returns a ArrayBuffer value with given binary data.
func (ctx *Context) NewArrayBuffer(binaryData []byte) *Value {
	if len(binaryData) == 0 {
		return &Value{ctx: ctx, ref: C.JS_NewArrayBufferCopy(ctx.ref, nil, 0)}
	}
	return &Value{ctx: ctx, ref: C.JS_NewArrayBufferCopy(ctx.ref, (*C.uchar)(&binaryData[0]), C.size_t(len(binaryData)))}
}

// ArrayBuffer returns a ArrayBuffer value with given binary data.
// Deprecated: Use NewArrayBuffer() instead.
func (ctx *Context) ArrayBuffer(binaryData []byte) *Value {
	return ctx.NewArrayBuffer(binaryData)
}

// createTypedArray is a helper function to create TypedArray with given data and type.
// It creates an ArrayBuffer first, then constructs a TypedArray from it.
func (ctx *Context) createTypedArray(data unsafe.Pointer, elementCount int, elementSize int, arrayType C.JSTypedArrayEnum) *Value {
	// Calculate total bytes needed for the data
	totalBytes := elementCount * elementSize

	// Convert raw data pointer to Go byte slice using unsafe.Slice (Go 1.17+)
	var bytes []byte
	if totalBytes > 0 && data != nil {
		bytes = unsafe.Slice((*byte)(data), totalBytes)
	}

	// Create ArrayBuffer from the byte data
	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	// Create TypedArray from ArrayBuffer: new TypedArray(buffer, offset, length)
	offset := C.JS_NewInt32(ctx.ref, C.int(0))            // Start from beginning of buffer
	length := C.JS_NewInt32(ctx.ref, C.int(elementCount)) // Number of elements (not bytes)

	// Pack arguments for JS_NewTypedArray call
	args := []C.JSValue{buffer.ref, offset, length}
	return &Value{
		ctx: ctx,
		ref: C.JS_NewTypedArray(ctx.ref, C.int(len(args)), &args[0], arrayType),
	}
}

// NewInt8Array returns a Int8Array value with given int8 slice.
func (ctx *Context) NewInt8Array(data []int8) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 1, C.JSTypedArrayEnum(C.GetTypedArrayInt8()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 1, C.JSTypedArrayEnum(C.GetTypedArrayInt8()))
}

// Int8Array returns a Int8Array value with given int8 slice.
// Deprecated: Use NewInt8Array() instead.
func (ctx *Context) Int8Array(data []int8) *Value {
	return ctx.NewInt8Array(data)
}

// NewUint8Array returns a Uint8Array value with given uint8 slice.
func (ctx *Context) NewUint8Array(data []uint8) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 1, C.JSTypedArrayEnum(C.GetTypedArrayUint8()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 1, C.JSTypedArrayEnum(C.GetTypedArrayUint8()))
}

// Uint8Array returns a Uint8Array value with given uint8 slice.
// Deprecated: Use NewUint8Array() instead.
func (ctx *Context) Uint8Array(data []uint8) *Value {
	return ctx.NewUint8Array(data)
}

// NewUint8ClampedArray returns a Uint8ClampedArray value with given uint8 slice.
func (ctx *Context) NewUint8ClampedArray(data []uint8) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 1, C.JSTypedArrayEnum(C.GetTypedArrayUint8C()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 1, C.JSTypedArrayEnum(C.GetTypedArrayUint8C()))
}

// Uint8ClampedArray returns a Uint8ClampedArray value with given uint8 slice.
// Deprecated: Use NewUint8ClampedArray() instead.
func (ctx *Context) Uint8ClampedArray(data []uint8) *Value {
	return ctx.NewUint8ClampedArray(data)
}

// NewInt16Array returns a Int16Array value with given int16 slice.
func (ctx *Context) NewInt16Array(data []int16) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 2, C.JSTypedArrayEnum(C.GetTypedArrayInt16()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 2, C.JSTypedArrayEnum(C.GetTypedArrayInt16()))
}

// Int16Array returns a Int16Array value with given int16 slice.
// Deprecated: Use NewInt16Array() instead.
func (ctx *Context) Int16Array(data []int16) *Value {
	return ctx.NewInt16Array(data)
}

// NewUint16Array returns a Uint16Array value with given uint16 slice.
func (ctx *Context) NewUint16Array(data []uint16) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 2, C.JSTypedArrayEnum(C.GetTypedArrayUint16()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 2, C.JSTypedArrayEnum(C.GetTypedArrayUint16()))
}

// Uint16Array returns a Uint16Array value with given uint16 slice.
// Deprecated: Use NewUint16Array() instead.
func (ctx *Context) Uint16Array(data []uint16) *Value {
	return ctx.NewUint16Array(data)
}

// NewInt32Array returns a Int32Array value with given int32 slice.
func (ctx *Context) NewInt32Array(data []int32) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 4, C.JSTypedArrayEnum(C.GetTypedArrayInt32()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 4, C.JSTypedArrayEnum(C.GetTypedArrayInt32()))
}

// Int32Array returns a Int32Array value with given int32 slice.
// Deprecated: Use NewInt32Array() instead.
func (ctx *Context) Int32Array(data []int32) *Value {
	return ctx.NewInt32Array(data)
}

// NewUint32Array returns a Uint32Array value with given uint32 slice.
func (ctx *Context) NewUint32Array(data []uint32) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 4, C.JSTypedArrayEnum(C.GetTypedArrayUint32()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 4, C.JSTypedArrayEnum(C.GetTypedArrayUint32()))
}

// Uint32Array returns a Uint32Array value with given uint32 slice.
// Deprecated: Use NewUint32Array() instead.
func (ctx *Context) Uint32Array(data []uint32) *Value {
	return ctx.NewUint32Array(data)
}

// NewFloat32Array returns a Float32Array value with given float32 slice.
func (ctx *Context) NewFloat32Array(data []float32) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 4, C.JSTypedArrayEnum(C.GetTypedArrayFloat32()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 4, C.JSTypedArrayEnum(C.GetTypedArrayFloat32()))
}

// Float32Array returns a Float32Array value with given float32 slice.
// Deprecated: Use NewFloat32Array() instead.
func (ctx *Context) Float32Array(data []float32) *Value {
	return ctx.NewFloat32Array(data)
}

// NewFloat64Array returns a Float64Array value with given float64 slice.
func (ctx *Context) NewFloat64Array(data []float64) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 8, C.JSTypedArrayEnum(C.GetTypedArrayFloat64()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 8, C.JSTypedArrayEnum(C.GetTypedArrayFloat64()))
}

// Float64Array returns a Float64Array value with given float64 slice.
// Deprecated: Use NewFloat64Array() instead.
func (ctx *Context) Float64Array(data []float64) *Value {
	return ctx.NewFloat64Array(data)
}

// NewBigInt64Array returns a BigInt64Array value with given int64 slice.
func (ctx *Context) NewBigInt64Array(data []int64) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 8, C.JSTypedArrayEnum(C.GetTypedArrayBigInt64()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 8, C.JSTypedArrayEnum(C.GetTypedArrayBigInt64()))
}

// BigInt64Array returns a BigInt64Array value with given int64 slice.
// Deprecated: Use NewBigInt64Array() instead.
func (ctx *Context) BigInt64Array(data []int64) *Value {
	return ctx.NewBigInt64Array(data)
}

// NewBigUint64Array returns a BigUint64Array value with given uint64 slice.
func (ctx *Context) NewBigUint64Array(data []uint64) *Value {
	if len(data) == 0 {
		return ctx.createTypedArray(nil, 0, 8, C.JSTypedArrayEnum(C.GetTypedArrayBigUint64()))
	}
	return ctx.createTypedArray(unsafe.Pointer(&data[0]), len(data), 8, C.JSTypedArrayEnum(C.GetTypedArrayBigUint64()))
}

// BigUint64Array returns a BigUint64Array value with given uint64 slice.
// Deprecated: Use NewBigUint64Array() instead.
func (ctx *Context) BigUint64Array(data []uint64) *Value {
	return ctx.NewBigUint64Array(data)
}

// NewObject returns a new object value.
func (ctx *Context) NewObject() *Value {
	return &Value{ctx: ctx, ref: C.JS_NewObject(ctx.ref)}
}

// Object returns a new object value.
// Deprecated: Use NewObject() instead.
func (ctx *Context) Object() *Value {
	return ctx.NewObject()
}

// ParseJson parses given json string and returns a object value.
func (ctx *Context) ParseJSON(v string) *Value {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))

	filenamePtr := C.CString("")
	defer C.free(unsafe.Pointer(filenamePtr))

	return &Value{ctx: ctx, ref: C.JS_ParseJSON(ctx.ref, ptr, C.size_t(len(v)), filenamePtr)}
}

// NewFunction returns a js function value with given function template
// New implementation using HandleStore and JS_NewCFunction2 with magic parameter
func (ctx *Context) NewFunction(fn func(*Context, *Value, []*Value) *Value) *Value {
	// Store function in HandleStore and get int32 ID
	fnID := ctx.handleStore.Store(fn)

	return &Value{
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

// Function returns a js function value with given function template
// New implementation using HandleStore and JS_NewCFunction2 with magic parameter
// Deprecated: Use NewFunction() instead.
func (ctx *Context) Function(fn func(*Context, *Value, []*Value) *Value) *Value {
	return ctx.NewFunction(fn)
}

// NewAsyncFunction returns a js async function value with given function template
//
// Deprecated: Use Context.NewFunction + Context.NewPromise instead for better memory management and thread safety.
// Example:
//
//	asyncFn := ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
//	    return ctx.NewPromise(func(resolve, reject func(*quickjs.Value)) {
//	        // async work here
//	        resolve(ctx.NewString("result"))
//	    })
//	})
func (ctx *Context) NewAsyncFunction(asyncFn func(ctx *Context, this *Value, promise *Value, args []*Value) *Value) *Value {
	// New implementation using Function + Promise
	return ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.NewPromise(func(resolve, reject func(*Value)) {
			// Create a promise object that has resolve/reject methods
			promiseObj := ctx.NewObject()
			promiseObj.Set("resolve", ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
				if len(args) > 0 {
					resolve(args[0])
				} else {
					resolve(ctx.NewUndefined())
				}
				return ctx.NewUndefined()
			}))
			promiseObj.Set("reject", ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
				if len(args) > 0 {
					reject(args[0])
				} else {
					errObj := ctx.NewError(fmt.Errorf("Promise rejected without reason"))
					defer errObj.Free() // Free the error object
					reject(errObj)
				}
				return ctx.NewUndefined()
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

// AsyncFunction returns a js async function value with given function template
//
// Deprecated: Use Context.NewFunction + Context.NewPromise instead for better memory management and thread safety.
// Example:
//
//	asyncFn := ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
//	    return ctx.NewPromise(func(resolve, reject func(*quickjs.Value)) {
//	        // async work here
//	        resolve(ctx.NewString("result"))
//	    })
//	})
func (ctx *Context) AsyncFunction(asyncFn func(ctx *Context, this *Value, promise *Value, args []*Value) *Value) *Value {
	return ctx.NewAsyncFunction(asyncFn)
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

// NewAtom returns a new Atom value with given string.
func (ctx *Context) NewAtom(v string) *Atom {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))
	return &Atom{ctx: ctx, ref: C.JS_NewAtom(ctx.ref, ptr)}
}

// Atom returns a new Atom value with given string.
// Deprecated: Use NewAtom() instead.
func (ctx *Context) Atom(v string) *Atom {
	return ctx.NewAtom(v)
}

// NewAtomIdx returns a new Atom value with given idx.
func (ctx *Context) NewAtomIdx(idx uint32) *Atom {
	return &Atom{ctx: ctx, ref: C.JS_NewAtomUInt32(ctx.ref, C.uint32_t(idx))}
}

// AtomIdx returns a new Atom value with given idx.
// Deprecated: Use NewAtomIdx() instead.
func (ctx *Context) AtomIdx(idx uint32) *Atom {
	return ctx.NewAtomIdx(idx)
}

// Invoke invokes a function with given this value and arguments.
// Deprecated: Use Value.Execute() instead for better API consistency.
func (ctx *Context) Invoke(fn *Value, this *Value, args ...*Value) *Value {
	cargs := []C.JSValue{}
	for _, x := range args {
		cargs = append(cargs, x.ref)
	}
	var val *Value
	if len(cargs) == 0 {
		val = &Value{ctx: ctx, ref: C.JS_Call(ctx.ref, fn.ref, this.ref, 0, nil)}
	} else {
		val = &Value{ctx: ctx, ref: C.JS_Call(ctx.ref, fn.ref, this.ref, C.int(len(cargs)), &cargs[0])}
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
// func (ctx *Context) Eval(code string) (*Value, error) { return ctx.EvalFile(code, "code") }
func (ctx *Context) Eval(code string, opts ...EvalOption) *Value {
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
		cFlag |= C.int(C.GetEvalTypeGlobal())
	}
	if options.js_eval_type_module {
		cFlag |= C.int(C.GetEvalTypeModule())
	}
	if options.js_eval_flag_strict {
		cFlag |= C.int(C.GetEvalFlagStrict())
	}
	if options.js_eval_flag_compile_only {
		cFlag |= C.int(C.GetEvalFlagCompileOnly())
	}

	codePtr := C.CString(code)
	defer C.free(unsafe.Pointer(codePtr))

	filenamePtr := C.CString(options.filename)
	defer C.free(unsafe.Pointer(filenamePtr))

	if C.JS_DetectModule(codePtr, C.size_t(len(code))) != 0 {
		cFlag |= C.int(C.GetEvalTypeModule())
	}

	var val *Value
	if options.await {
		val = &Value{ctx: ctx, ref: C.js_std_await(ctx.ref, C.JS_Eval(ctx.ref, codePtr, C.size_t(len(code)), filenamePtr, cFlag))}
	} else {
		val = &Value{ctx: ctx, ref: C.JS_Eval(ctx.ref, codePtr, C.size_t(len(code)), filenamePtr, cFlag)}
	}

	return val
}

// EvalFile returns a js value with given code and filename.
// Need call Free() `quickjs.Value`'s returned by `Eval()` and `EvalFile()` and `EvalBytecode()`.
func (ctx *Context) EvalFile(filePath string, opts ...EvalOption) *Value {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return ctx.ThrowError(err)
	}
	opts = append(opts, EvalFileName(filePath))
	return ctx.Eval(string(b), opts...)
}

// LoadModule returns a js value with given code and module name.
func (ctx *Context) LoadModule(code string, moduleName string, opts ...EvalOption) *Value {
	options := EvalOptions{
		load_only: false,
	}
	for _, fn := range opts {
		fn(&options)
	}

	codePtr := C.CString(code)
	defer C.free(unsafe.Pointer(codePtr))

	if C.JS_DetectModule(codePtr, C.size_t(len(code))) == 0 {
		return ctx.ThrowSyntaxError("not a module: %s", moduleName)
	}

	codeByte, err := ctx.Compile(code, EvalFlagModule(true), EvalFlagCompileOnly(true), EvalFileName(moduleName))
	if err != nil {
		return ctx.ThrowError(err)
	}

	return ctx.LoadModuleBytecode(codeByte, EvalLoadOnly(options.load_only))

}

// LoadModuleFile returns a js value with given file path and module name.
func (ctx *Context) LoadModuleFile(filePath string, moduleName string) *Value {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return ctx.ThrowError(err)
	}
	return ctx.LoadModule(string(b), moduleName)
}

// CompileModule returns a compiled bytecode with given code and module name.
func (ctx *Context) CompileModule(filePath string, moduleName string, opts ...EvalOption) ([]byte, error) {
	opts = append(opts, EvalFileName(moduleName))
	return ctx.CompileFile(filePath, opts...)
}

// LoadModuleByteCode returns a js value with given bytecode and module name.
func (ctx *Context) LoadModuleBytecode(buf []byte, opts ...EvalOption) *Value {
	if len(buf) == 0 {
		return ctx.ThrowSyntaxError("empty bytecode")
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

	return &Value{ctx: ctx, ref: cVal}
}

// EvalBytecode returns a js value with given bytecode.
// Need call Free() `quickjs.Value`'s returned by `Eval()` and `EvalFile()` and `EvalBytecode()`.
func (ctx *Context) EvalBytecode(buf []byte) *Value {
	cbuf := C.CBytes(buf)
	obj := &Value{ctx: ctx, ref: C.JS_ReadObject(ctx.ref, (*C.uint8_t)(cbuf), C.size_t(len(buf)), C.int(C.GetReadObjBytecode()))}
	defer C.js_free(ctx.ref, unsafe.Pointer(cbuf))
	if obj.IsException() {
		return obj
	}

	return &Value{ctx: ctx, ref: C.JS_EvalFunction(ctx.ref, obj.ref)}
}

// Compile returns a compiled bytecode with given code.
func (ctx *Context) Compile(code string, opts ...EvalOption) ([]byte, error) {
	opts = append(opts, EvalFlagCompileOnly(true))
	val := ctx.Eval(code, opts...)
	defer val.Free()

	var kSize C.size_t = 0
	ptr := C.JS_WriteObject(ctx.ref, &kSize, val.ref, C.int(C.GetWriteObjBytecode()))

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
func (ctx *Context) Globals() *Value {
	if ctx.globals == nil {
		ctx.globals = &Value{
			ctx: ctx,
			ref: C.JS_GetGlobalObject(ctx.ref),
		}
	}
	return ctx.globals
}

// Throw returns a context's exception value.
func (ctx *Context) Throw(v *Value) *Value {
	return &Value{ctx: ctx, ref: C.JS_Throw(ctx.ref, v.ref)}
}

// ThrowError returns a context's exception value with given error message.
func (ctx *Context) ThrowError(err error) *Value {
	return ctx.Throw(ctx.NewError(err))
}

// ThrowSyntaxError returns a context's exception value with given error message.
func (ctx *Context) ThrowSyntaxError(format string, args ...interface{}) *Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return &Value{ctx: ctx, ref: C.ThrowSyntaxError(ctx.ref, causePtr)}
}

// ThrowTypeError returns a context's exception value with given error message.
func (ctx *Context) ThrowTypeError(format string, args ...interface{}) *Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return &Value{ctx: ctx, ref: C.ThrowTypeError(ctx.ref, causePtr)}
}

// ThrowReferenceError returns a context's exception value with given error message.
func (ctx *Context) ThrowReferenceError(format string, args ...interface{}) *Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return &Value{ctx: ctx, ref: C.ThrowReferenceError(ctx.ref, causePtr)}
}

// ThrowRangeError returns a context's exception value with given error message.
func (ctx *Context) ThrowRangeError(format string, args ...interface{}) *Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return &Value{ctx: ctx, ref: C.ThrowRangeError(ctx.ref, causePtr)}
}

// ThrowInternalError returns a context's exception value with given error message.
func (ctx *Context) ThrowInternalError(format string, args ...interface{}) *Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return &Value{ctx: ctx, ref: C.ThrowInternalError(ctx.ref, causePtr)}
}

// HasException checks if the context has an exception set.
func (ctx *Context) HasException() bool {
	// Check if the context has an exception set
	return C.JS_HasException(ctx.ref) == 1
}

// Exception returns a context's exception value.
func (ctx *Context) Exception() error {
	val := &Value{ctx: ctx, ref: C.JS_GetException(ctx.ref)}
	defer val.Free()
	return val.Error()
}

// Loop runs the context's event loop.
func (ctx *Context) Loop() {
	ctx.ProcessJobs()
	C.js_std_loop(ctx.ref)
	ctx.ProcessJobs()
}

// Wait for a promise and execute pending jobs while waiting for it. Return the promise result or JS_EXCEPTION in case of promise rejection.
func (ctx *Context) Await(v *Value) *Value {
	if v == nil || !v.IsPromise() {
		return v
	}

	// Transfer ownership of the JSValue so the original handle no longer leaks references.
	promise := &Value{ctx: ctx, ref: v.ref}
	v.ref = C.JS_NewUndefined()
	defer promise.Free()

	pendingState := C.JSPromiseStateEnum(C.GetPromisePending())
	fulfilledState := C.JSPromiseStateEnum(C.GetPromiseFulfilled())
	rejectedState := C.JSPromiseStateEnum(C.GetPromiseRejected())
	runtimeRef := ctx.runtime.ref

	for {
		ctx.ProcessJobs()
		state := C.JS_PromiseState(ctx.ref, promise.ref)
		if hook := awaitPromiseStateHook; hook != nil {
			if override, ok := hook(ctx, promise, int(state)); ok {
				state = C.JSPromiseStateEnum(override)
			}
		}
		switch state {
		case fulfilledState:
			return &Value{ctx: ctx, ref: C.JS_PromiseResult(ctx.ref, promise.ref)}
		case rejectedState:
			reason := C.JS_PromiseResult(ctx.ref, promise.ref)
			return &Value{ctx: ctx, ref: C.JS_Throw(ctx.ref, reason)}
		case pendingState:
			executed := C.JS_ExecutePendingJob(runtimeRef, nil)
			if hook := awaitExecutePendingJobHook; hook != nil {
				if override, ok := hook(ctx, promise, int(executed)); ok {
					executed = C.int(override)
				}
			}
			if executed < 0 {
				return ctx.ThrowInternalError("failed to execute pending job")
			}
			if executed == 0 {
				C.js_std_loop(ctx.ref)
			}
		default:
			return v
		}
	}
}

// NewPromise creates a new Promise with the provided executor function and
// keeps all QuickJS interactions on the context thread.
//
// The executor itself runs synchronously, so you can resolve/reject
// immediately:
//
//	ctx.NewPromise(func(resolve, reject func(*quickjs.Value)) {
//	    resolve(ctx.NewString("sync value"))
//	})
//
// For asynchronous work, perform the slow operation in another goroutine and
// use ctx.Schedule inside that goroutine so the actual resolve happens on the
// JS thread. Do not invoke other Context methods from the goroutine directly;
// the function passed to ctx.Schedule runs on the context thread, which keeps
// QuickJS access safe:
//
//	ctx.NewPromise(func(resolve, reject func(*quickjs.Value)) {
//	    go func() {
//	        result := doWork()
//	        ctx.Schedule(func(inner *quickjs.Context) {
//	            val := inner.NewString(result)
//	            defer val.Free()
//	            resolve(val)
//	        })
//	    }()
//	})
//
// The resolver helpers ensure only the first resolve/reject wins, matching
// native Promise semantics.
func (ctx *Context) NewPromise(executor func(resolve, reject func(*Value))) *Value {
	// Create Promise using JavaScript code to avoid complex C API reference management
	promiseSetup := ctx.Eval(`
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

	// Create wrapper functions that schedule resolve/reject back onto the JS thread
	settled := int32(0)
	resolveCallback, releaseResolve := ctx.wrapPromiseCallback(resolveFunc)
	rejectCallback, releaseReject := ctx.wrapPromiseCallback(rejectFunc)
	wrap := func(target int32, callback func(*Value), releaseOther func()) func(*Value) {
		return func(val *Value) {
			if atomic.CompareAndSwapInt32(&settled, 0, target) {
				callback(val)
				releaseOther()
			}
		}
	}
	resolve := wrap(1, resolveCallback, releaseReject)
	reject := wrap(2, rejectCallback, releaseResolve)

	// Execute user function synchronously and flush any immediate resolve/reject work
	executor(resolve, reject)
	ctx.ProcessJobs()

	return promise
}

// Promise creates a new Promise with executor function
// Executor runs synchronously in current thread for thread safety
// Deprecated: Use NewPromise() instead.
func (ctx *Context) Promise(executor func(resolve, reject func(*Value))) *Value {
	return ctx.NewPromise(executor)
}
