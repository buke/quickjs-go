package quickjs

/*
#include "bridge.h"
*/
import "C"
import (
	"errors"
	"math/big"
	"unsafe"
)

// JSValue represents a Javascript value which can be a primitive type or an object. Reference counting is used, so it is important to explicitly duplicate (JS_DupValue(), increment the reference count) or free (JS_FreeValue(), decrement the reference count) JSValues.
type Value struct {
	ctx *Context
	ref C.JSValue
}

// Free the value.
func (v Value) Free() {
	C.JS_FreeValue(v.ctx.ref, v.ref)
}

// Context represents a Javascript context.
func (v Value) Context() *Context {
	return v.ctx
}

// Deprecated: Use ToBool instead.
func (v Value) Bool() bool {
	return v.ToBool()
}

// ToBool returns the boolean value of the value.
func (v Value) ToBool() bool {
	return C.JS_ToBool(v.ctx.ref, v.ref) == 1
}

// String returns the string representation of the value.
// This method implements the fmt.Stringer interface.
func (v Value) String() string {
	return v.ToString()
}

// ToString returns the string representation of the value.
func (v Value) ToString() string {
	ptr := C.JS_ToCString(v.ctx.ref, v.ref)
	defer C.JS_FreeCString(v.ctx.ref, ptr)
	return C.GoString(ptr)
}

// JSONString returns the JSON string representation of the value.
func (v Value) JSONStringify() string {
	ref := C.JS_JSONStringify(v.ctx.ref, v.ref, C.JS_NewNull(), C.JS_NewNull())
	ptr := C.JS_ToCString(v.ctx.ref, ref)
	defer C.JS_FreeCString(v.ctx.ref, ptr)
	return C.GoString(ptr)
}

func (v Value) ToByteArray(size uint) ([]byte, error) {
	if v.ByteLen() < int64(size) {
		return nil, errors.New("exceeds the maximum length of the current binary array")
	}
	cSize := C.size_t(size)
	outBuf := C.JS_GetArrayBuffer(v.ctx.ref, &cSize, v.ref)
	return C.GoBytes(unsafe.Pointer(outBuf), C.int(size)), nil
}

// IsByteArray return true if the value is array buffer
func (v Value) IsByteArray() bool {
	return v.IsObject() && v.GlobalInstanceof("ArrayBuffer") || v.String() == "[object ArrayBuffer]"
}

// Deprecated: Use ToInt64 instead.
func (v Value) Int64() int64 {
	return v.ToInt64()
}

// ToInt64 returns the int64 value of the value.
func (v Value) ToInt64() int64 {
	val := C.int64_t(0)
	C.JS_ToInt64(v.ctx.ref, &val, v.ref)
	return int64(val)
}

// Deprecated: Use ToInt32 instead.
func (v Value) Int32() int32 {
	return v.ToInt32()
}

// ToInt32 returns the int32 value of the value.
func (v Value) ToInt32() int32 {
	val := C.int32_t(0)
	C.JS_ToInt32(v.ctx.ref, &val, v.ref)
	return int32(val)
}

// Deprecated: Use ToUint32 instead.
func (v Value) Uint32() uint32 {
	return v.ToUint32()
}

// ToUint32 returns the uint32 value of the value.
func (v Value) ToUint32() uint32 {
	val := C.uint32_t(0)
	C.JS_ToUint32(v.ctx.ref, &val, v.ref)
	return uint32(val)
}

// Deprecated: Use ToFloat64 instead.
func (v Value) Float64() float64 {
	return v.ToFloat64()
}

// ToFloat64 returns the float64 value of the value.
func (v Value) ToFloat64() float64 {
	val := C.double(0)
	C.JS_ToFloat64(v.ctx.ref, &val, v.ref)
	return float64(val)
}

// Deprecated: Use ToBigInt instead.
func (v Value) BigInt() *big.Int {
	return v.ToBigInt()
}

// ToBigInt returns the big.Int value of the value.
func (v Value) ToBigInt() *big.Int {
	if !v.IsBigInt() {
		return nil
	}
	val, _ := new(big.Int).SetString(v.String(), 10)
	return val
}

// Len returns the length of the array.
func (v Value) Len() int64 {
	return v.Get("length").Int64()
}

// ByteLen returns the length of the ArrayBuffer.
func (v Value) ByteLen() int64 {
	return v.Get("byteLength").Int64()
}

// Set sets the value of the property with the given name.
func (v Value) Set(name string, val Value) {
	namePtr := C.CString(name)
	defer C.free(unsafe.Pointer(namePtr))
	C.JS_SetPropertyStr(v.ctx.ref, v.ref, namePtr, val.ref)
}

// SetIdx sets the value of the property with the given index.
func (v Value) SetIdx(idx int64, val Value) {
	C.JS_SetPropertyUint32(v.ctx.ref, v.ref, C.uint32_t(idx), val.ref)
}

// Get returns the value of the property with the given name.
func (v Value) Get(name string) Value {
	namePtr := C.CString(name)
	defer C.free(unsafe.Pointer(namePtr))
	return Value{ctx: v.ctx, ref: C.JS_GetPropertyStr(v.ctx.ref, v.ref, namePtr)}
}

// GetIdx returns the value of the property with the given index.
func (v Value) GetIdx(idx int64) Value {
	return Value{ctx: v.ctx, ref: C.JS_GetPropertyUint32(v.ctx.ref, v.ref, C.uint32_t(idx))}
}

// Call calls the function with the given arguments.
func (v Value) Call(fname string, args ...Value) Value {
	fn := v.Get(fname) // get the function by name
	defer fn.Free()

	cargs := []C.JSValue{}
	for _, x := range args {
		cargs = append(cargs, x.ref)
	}
	var val Value
	if len(cargs) == 0 {
		val = Value{ctx: v.ctx, ref: C.JS_Call(v.ctx.ref, fn.ref, v.ref, C.int(0), nil)}
	} else {
		val = Value{ctx: v.ctx, ref: C.JS_Call(v.ctx.ref, fn.ref, v.ref, C.int(len(cargs)), &cargs[0])}
	}

	if v.ctx.HasException() {
		return Value{ctx: v.ctx, ref: C.JS_GetException(v.ctx.ref)}
	}

	return val

}

// Execute the function with the given arguments.
func (v Value) Execute(this Value, args ...Value) Value {
	cargs := []C.JSValue{}
	for _, x := range args {
		cargs = append(cargs, x.ref)
	}
	var val Value
	if len(cargs) == 0 {
		val = Value{ctx: v.ctx, ref: C.JS_Call(v.ctx.ref, v.ref, this.ref, C.int(0), nil)}
	} else {
		val = Value{ctx: v.ctx, ref: C.JS_Call(v.ctx.ref, v.ref, this.ref, C.int(len(cargs)), &cargs[0])}
	}

	if v.ctx.HasException() {
		return Value{ctx: v.ctx, ref: C.JS_GetException(v.ctx.ref)}
	}

	return val
}

// Call Class Constructor
func (v Value) New(args ...Value) Value {
	return v.CallConstructor(args...)
}

// Call calls the constructor with the given arguments.
func (v Value) CallConstructor(args ...Value) Value {
	cargs := []C.JSValue{}
	for _, x := range args {
		cargs = append(cargs, x.ref)
	}
	var val Value
	if len(cargs) == 0 {
		val = Value{ctx: v.ctx, ref: C.JS_CallConstructor(v.ctx.ref, v.ref, C.int(0), nil)}
	} else {
		val = Value{ctx: v.ctx, ref: C.JS_CallConstructor(v.ctx.ref, v.ref, C.int(len(cargs)), &cargs[0])}
	}

	if v.ctx.HasException() {
		return Value{ctx: v.ctx, ref: C.JS_GetException(v.ctx.ref)}
	}
	return val
}

// Deprecated: Use ToError() instead.
func (v Value) Error() error {
	return v.ToError()
}

// Error returns the error value of the value.
func (v Value) ToError() error {
	if !v.IsError() {
		return nil
	}

	err := &Error{}

	name := v.Get("name")
	defer name.Free()
	if !name.IsUndefined() {
		err.Name = name.String()
	}

	message := v.Get("message")
	defer message.Free()
	if !message.IsUndefined() {
		err.Message = message.String()
	}

	cause := v.Get("cause")
	defer cause.Free()
	if !cause.IsUndefined() {
		err.Cause = cause.String()
	}

	stack := v.Get("stack")
	defer stack.Free()
	if !stack.IsUndefined() {
		err.Stack = stack.String()
	}

	jsonString := v.JSONStringify()
	if jsonString != "" {
		err.JSONString = jsonString
	}

	return err
}

// propertyEnum is a wrapper around JSValue.
func (v Value) propertyEnum() ([]propertyEnum, error) {
	var ptr *C.JSPropertyEnum
	var size C.uint32_t

	result := int(C.JS_GetOwnPropertyNames(v.ctx.ref, &ptr, &size, v.ref, C.int(1<<0|1<<1|1<<2)))
	if result < 0 {
		return nil, errors.New("value does not contain properties")
	}
	defer C.js_free(v.ctx.ref, unsafe.Pointer(ptr))

	entries := unsafe.Slice(ptr, size) // Go 1.17 and later
	names := make([]propertyEnum, len(entries))
	for i := 0; i < len(names); i++ {
		names[i].IsEnumerable = entries[i].is_enumerable == 1
		names[i].atom = Atom{ctx: v.ctx, ref: entries[i].atom}
		names[i].atom.Free()
	}

	return names, nil
}

// PropertyNames returns the names of the properties of the value.
func (v Value) PropertyNames() ([]string, error) {
	pList, err := v.propertyEnum()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(pList))
	for i := 0; i < len(names); i++ {
		names[i] = pList[i].String()
	}
	return names, nil
}

// Has returns true if the value has the property with the given name.
func (v Value) Has(name string) bool {
	prop := v.ctx.Atom(name)
	defer prop.Free()
	return C.JS_HasProperty(v.ctx.ref, v.ref, prop.ref) == 1
}

// HasIdx returns true if the value has the property with the given index.
func (v Value) HasIdx(idx uint32) bool {
	prop := v.ctx.AtomIdx(idx)
	defer prop.Free()
	return C.JS_HasProperty(v.ctx.ref, v.ref, prop.ref) == 1
}

// Delete deletes the property with the given name.
func (v Value) Delete(name string) bool {
	if !v.Has(name) {
		return false // Property does not exist, nothing to delete
	}
	prop := v.ctx.Atom(name)
	defer prop.Free()
	return C.JS_DeleteProperty(v.ctx.ref, v.ref, prop.ref, C.int(1)) == 1
}

// DeleteIdx deletes the property with the given index.
func (v Value) DeleteIdx(idx uint32) bool {
	if !v.HasIdx(idx) {
		return false // Property does not exist, nothing to delete
	}
	return C.JS_DeletePropertyInt64(v.ctx.ref, v.ref, C.int64_t(idx), C.int(1)) == 1
}

// globalInstanceof checks if the value is an instance of the given global constructor
func (v Value) GlobalInstanceof(name string) bool {
	ctor := v.ctx.Globals().Get(name)
	defer ctor.Free()
	if ctor.IsUndefined() {
		return false
	}
	return C.JS_IsInstanceOf(v.ctx.ref, v.ref, ctor.ref) == 1
}

func (v Value) IsNumber() bool        { return C.JS_IsNumber(v.ref) == 1 }
func (v Value) IsBigInt() bool        { return C.JS_IsBigInt(v.ctx.ref, v.ref) == 1 }
func (v Value) IsBool() bool          { return C.JS_IsBool(v.ref) == 1 }
func (v Value) IsNull() bool          { return C.JS_IsNull(v.ref) == 1 }
func (v Value) IsUndefined() bool     { return C.JS_IsUndefined(v.ref) == 1 }
func (v Value) IsException() bool     { return C.JS_IsException(v.ref) == 1 }
func (v Value) IsUninitialized() bool { return C.JS_IsUninitialized(v.ref) == 1 }
func (v Value) IsString() bool        { return C.JS_IsString(v.ref) == 1 }
func (v Value) IsSymbol() bool        { return C.JS_IsSymbol(v.ref) == 1 }
func (v Value) IsObject() bool        { return C.JS_IsObject(v.ref) == 1 }
func (v Value) IsArray() bool         { return C.JS_IsArray(v.ctx.ref, v.ref) == 1 }
func (v Value) IsError() bool         { return C.JS_IsError(v.ctx.ref, v.ref) == 1 }
func (v Value) IsFunction() bool      { return C.JS_IsFunction(v.ctx.ref, v.ref) == 1 }
func (v Value) IsPromise() bool {
	state := C.JS_PromiseState(v.ctx.ref, v.ref)
	if state == C.JS_PROMISE_PENDING || state == C.JS_PROMISE_FULFILLED || state == C.JS_PROMISE_REJECTED {
		return true
	}
	return false
}

func (v Value) IsConstructor() bool { return C.JS_IsConstructor(v.ctx.ref, v.ref) == 1 }
