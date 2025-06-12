package quickjs

/*
#include "bridge.h"
*/
import "C"
import (
	"encoding/binary"
	"errors"
	"math"
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
	if v.ctx == nil || C.JS_IsUndefined_Wrapper(v.ref) == 1 {
		return // No context or undefined value, nothing to free
	}
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

// JSONStringify returns the JSON string representation of the value.
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

	if outBuf == nil {
		return nil, errors.New("failed to get ArrayBuffer data")
	}

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

	return val
}

// Deprecated: Use ToError() instead.
func (v Value) Error() error {
	return v.ToError()
}

// ToError returns the error value of the value.
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

// GlobalInstanceof checks if the value is an instance of the given global constructor
func (v Value) GlobalInstanceof(name string) bool {
	ctor := v.ctx.Globals().Get(name)
	defer ctor.Free()
	if ctor.IsUndefined() {
		return false
	}
	return C.JS_IsInstanceOf(v.ctx.ref, v.ref, ctor.ref) == 1
}

// TypedArray detection methods
func (v Value) IsTypedArray() bool {
	typedArrayTypes := []string{
		"Int8Array", "Uint8Array", "Uint8ClampedArray",
		"Int16Array", "Uint16Array", "Int32Array", "Uint32Array",
		"Float32Array", "Float64Array", "BigInt64Array", "BigUint64Array",
	}

	for _, typeName := range typedArrayTypes {
		if v.GlobalInstanceof(typeName) {
			return true
		}
	}
	return false
}

func (v Value) IsInt8Array() bool         { return v.GlobalInstanceof("Int8Array") }
func (v Value) IsUint8Array() bool        { return v.GlobalInstanceof("Uint8Array") }
func (v Value) IsUint8ClampedArray() bool { return v.GlobalInstanceof("Uint8ClampedArray") }
func (v Value) IsInt16Array() bool        { return v.GlobalInstanceof("Int16Array") }
func (v Value) IsUint16Array() bool       { return v.GlobalInstanceof("Uint16Array") }
func (v Value) IsInt32Array() bool        { return v.GlobalInstanceof("Int32Array") }
func (v Value) IsUint32Array() bool       { return v.GlobalInstanceof("Uint32Array") }
func (v Value) IsFloat32Array() bool      { return v.GlobalInstanceof("Float32Array") }
func (v Value) IsFloat64Array() bool      { return v.GlobalInstanceof("Float64Array") }
func (v Value) IsBigInt64Array() bool     { return v.GlobalInstanceof("BigInt64Array") }
func (v Value) IsBigUint64Array() bool    { return v.GlobalInstanceof("BigUint64Array") }

// getTypedArrayInfo is a helper function to extract TypedArray information using C API
func (v Value) getTypedArrayInfo() (buffer Value, byteOffset, byteLength, bytesPerElement int) {
	var cByteOffset, cByteLength, cBytesPerElement C.size_t
	bufferRef := C.JS_GetTypedArrayBuffer(v.ctx.ref, v.ref, &cByteOffset, &cByteLength, &cBytesPerElement)

	return Value{ctx: v.ctx, ref: bufferRef},
		int(cByteOffset), int(cByteLength), int(cBytesPerElement)
}

// ToInt8Array converts the value to int8 slice if it's an Int8Array.
func (v Value) ToInt8Array() ([]int8, error) {
	if !v.IsInt8Array() {
		return nil, errors.New("value is not an Int8Array")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	data := bytes[byteOffset : byteOffset+byteLength]
	result := make([]int8, len(data))
	for i, b := range data {
		result[i] = int8(b)
	}
	return result, nil
}

// ToUint8Array converts the value to uint8 slice if it's a Uint8Array or Uint8ClampedArray.
func (v Value) ToUint8Array() ([]uint8, error) {
	if !v.IsUint8Array() && !v.IsUint8ClampedArray() {
		return nil, errors.New("value is not a Uint8Array or Uint8ClampedArray")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	return bytes[byteOffset : byteOffset+byteLength], nil
}

// ToInt16Array converts the value to int16 slice if it's an Int16Array.
func (v Value) ToInt16Array() ([]int16, error) {
	if !v.IsInt16Array() {
		return nil, errors.New("value is not an Int16Array")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	data := bytes[byteOffset : byteOffset+byteLength]
	result := make([]int16, len(data)/2)
	for i := 0; i < len(result); i++ {
		result[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return result, nil
}

// ToUint16Array converts the value to uint16 slice if it's a Uint16Array.
func (v Value) ToUint16Array() ([]uint16, error) {
	if !v.IsUint16Array() {
		return nil, errors.New("value is not a Uint16Array")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	data := bytes[byteOffset : byteOffset+byteLength]
	result := make([]uint16, len(data)/2)
	for i := 0; i < len(result); i++ {
		result[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	return result, nil
}

// ToInt32Array converts the value to int32 slice if it's an Int32Array.
func (v Value) ToInt32Array() ([]int32, error) {
	if !v.IsInt32Array() {
		return nil, errors.New("value is not an Int32Array")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	data := bytes[byteOffset : byteOffset+byteLength]
	result := make([]int32, len(data)/4)
	for i := 0; i < len(result); i++ {
		result[i] = int32(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return result, nil
}

// ToUint32Array converts the value to uint32 slice if it's a Uint32Array.
func (v Value) ToUint32Array() ([]uint32, error) {
	if !v.IsUint32Array() {
		return nil, errors.New("value is not a Uint32Array")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	data := bytes[byteOffset : byteOffset+byteLength]
	result := make([]uint32, len(data)/4)
	for i := 0; i < len(result); i++ {
		result[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	return result, nil
}

// ToFloat32Array converts the value to float32 slice if it's a Float32Array.
func (v Value) ToFloat32Array() ([]float32, error) {
	if !v.IsFloat32Array() {
		return nil, errors.New("value is not a Float32Array")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	data := bytes[byteOffset : byteOffset+byteLength]
	result := make([]float32, len(data)/4)
	for i := 0; i < len(result); i++ {
		bits := binary.LittleEndian.Uint32(data[i*4:])
		result[i] = math.Float32frombits(bits)
	}
	return result, nil
}

// ToFloat64Array converts the value to float64 slice if it's a Float64Array.
func (v Value) ToFloat64Array() ([]float64, error) {
	if !v.IsFloat64Array() {
		return nil, errors.New("value is not a Float64Array")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	data := bytes[byteOffset : byteOffset+byteLength]
	result := make([]float64, len(data)/8)
	for i := 0; i < len(result); i++ {
		bits := binary.LittleEndian.Uint64(data[i*8:])
		result[i] = math.Float64frombits(bits)
	}
	return result, nil
}

// ToBigInt64Array converts the value to int64 slice if it's a BigInt64Array.
func (v Value) ToBigInt64Array() ([]int64, error) {
	if !v.IsBigInt64Array() {
		return nil, errors.New("value is not a BigInt64Array")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	data := bytes[byteOffset : byteOffset+byteLength]
	result := make([]int64, len(data)/8)
	for i := 0; i < len(result); i++ {
		result[i] = int64(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return result, nil
}

// ToBigUint64Array converts the value to uint64 slice if it's a BigUint64Array.
func (v Value) ToBigUint64Array() ([]uint64, error) {
	if !v.IsBigUint64Array() {
		return nil, errors.New("value is not a BigUint64Array")
	}

	buffer, byteOffset, byteLength, _ := v.getTypedArrayInfo()
	defer buffer.Free()

	totalSize := uint(byteOffset + byteLength)
	bytes, err := buffer.ToByteArray(totalSize)
	if err != nil {
		return nil, err
	}

	data := bytes[byteOffset : byteOffset+byteLength]
	result := make([]uint64, len(data)/8)
	for i := 0; i < len(result); i++ {
		result[i] = binary.LittleEndian.Uint64(data[i*8:])
	}
	return result, nil
}

// =============================================================================
// BASIC TYPE CHECKING METHODS (replaced macros with wrapper functions)
// =============================================================================

func (v Value) IsNumber() bool        { return C.JS_IsNumber_Wrapper(v.ref) == 1 }
func (v Value) IsBigInt() bool        { return C.JS_IsBigInt_Wrapper(v.ctx.ref, v.ref) == 1 }
func (v Value) IsBool() bool          { return C.JS_IsBool_Wrapper(v.ref) == 1 }
func (v Value) IsNull() bool          { return C.JS_IsNull_Wrapper(v.ref) == 1 }
func (v Value) IsUndefined() bool     { return C.JS_IsUndefined_Wrapper(v.ref) == 1 }
func (v Value) IsException() bool     { return C.JS_IsException_Wrapper(v.ref) == 1 }
func (v Value) IsUninitialized() bool { return C.JS_IsUninitialized_Wrapper(v.ref) == 1 }
func (v Value) IsString() bool        { return C.JS_IsString_Wrapper(v.ref) == 1 }
func (v Value) IsSymbol() bool        { return C.JS_IsSymbol_Wrapper(v.ref) == 1 }
func (v Value) IsObject() bool        { return C.JS_IsObject_Wrapper(v.ref) == 1 }
func (v Value) IsArray() bool         { return C.JS_IsArray(v.ctx.ref, v.ref) == 1 }
func (v Value) IsError() bool         { return C.JS_IsError(v.ctx.ref, v.ref) == 1 }
func (v Value) IsFunction() bool      { return C.JS_IsFunction(v.ctx.ref, v.ref) == 1 }
func (v Value) IsConstructor() bool   { return C.JS_IsConstructor(v.ctx.ref, v.ref) == 1 }

// =============================================================================
// PROMISE SUPPORT METHODS (replaced constants with getter functions)
// =============================================================================

func (v Value) IsPromise() bool {
	state := C.JS_PromiseState(v.ctx.ref, v.ref)
	pending := C.GetPromisePending()
	fulfilled := C.GetPromiseFulfilled()
	rejected := C.GetPromiseRejected()

	if C.int(state) == pending || C.int(state) == fulfilled || C.int(state) == rejected {
		return true
	}
	return false
}

// Promise state enumeration matching QuickJS
type PromiseState int

const (
	PromisePending PromiseState = iota
	PromiseFulfilled
	PromiseRejected
)

// PromiseState returns the state of the Promise
func (v Value) PromiseState() PromiseState {
	if !v.IsPromise() {
		return PromisePending
	}

	state := C.JS_PromiseState(v.ctx.ref, v.ref)
	switch state {
	case C.JSPromiseStateEnum(C.GetPromisePending()):
		return PromisePending
	case C.JSPromiseStateEnum(C.GetPromiseFulfilled()):
		return PromiseFulfilled
	default:
		return PromiseRejected
	}
}

// Await waits for promise resolution and executes pending jobs
// Similar to Context.Await but called on Value directly
func (v Value) Await() (Value, error) {
	if !v.IsPromise() {
		// Not a promise, return as-is
		return v, nil
	}

	// Use js_std_await which handles the event loop
	result := Value{ctx: v.ctx, ref: C.js_std_await(v.ctx.ref, v.ref)}
	if result.IsException() {
		return result, v.ctx.Exception()
	}
	return result, nil
}

// =============================================================================
// CLASS INSTANCE SUPPORT METHODS (replaced invalid class ID constant)
// =============================================================================

// IsClassInstance checks if the value is an instance of any user-defined class
// This method uses opaque data validation for maximum reliability
func (v Value) IsClassInstance() bool {
	if !v.IsObject() {
		return false // Only objects can be class instances
	}

	// The most reliable method: check for valid opaque data
	// All our class instances have opaque data pointing to HandleStore entries
	return v.HasInstanceData()
}

// HasInstanceData checks if the value has associated Go object data
// This is the most reliable way to identify our class instances
func (v Value) HasInstanceData() bool {
	if !v.IsObject() {
		return false
	}

	// Get class ID first
	classID := C.JS_GetClassID(v.ref)
	invalidClassID := C.uint32_t(C.GetInvalidClassID())
	if classID == invalidClassID {
		return false
	}

	// Use JS_GetOpaque2 for type-safe check (like point.c methods)
	opaque := C.JS_GetOpaque2(v.ctx.ref, v.ref, classID)
	if opaque == nil {
		return false
	}

	// Validate that the handle ID exists in our HandleStore
	handleID := int32(C.OpaqueToInt(opaque))
	_, exists := v.ctx.handleStore.Load(handleID)
	return exists
}

// IsInstanceOfClass checks if the value is an instance of a specific class ID
// This provides type-safe class instance checking with double validation
func (v Value) IsInstanceOfClass(expectedClassID uint32) bool {
	if !v.IsObject() {
		return false
	}

	// First check: class ID must match
	objClassID := uint32(C.JS_GetClassID(v.ref))
	invalidClassID := uint32(C.GetInvalidClassID())
	if objClassID != expectedClassID || objClassID == invalidClassID {
		return false
	}

	// Second check: must have valid instance data using type-safe retrieval
	opaque := C.JS_GetOpaque2(v.ctx.ref, v.ref, C.JSClassID(expectedClassID))
	if opaque == nil {
		return false
	}

	// Third check: validate handle exists in store
	handleID := int32(C.OpaqueToInt(opaque))
	_, exists := v.ctx.handleStore.Load(handleID)
	return exists
}

// GetClassID returns the class ID of the value if it's a class instance
// Returns JS_INVALID_CLASS_ID (0) if not a class instance
func (v Value) GetClassID() uint32 {
	if !v.IsObject() {
		return uint32(C.GetInvalidClassID())
	}
	return uint32(C.JS_GetClassID(v.ref))
}

// =============================================================================
// SPECIALIZED CLASS TYPE CHECKING METHODS
// =============================================================================

// These methods can be used to check for specific known class types
// They serve as examples of how to implement type checking for custom classes

// IsCustomClass checks if the value is an instance of a class with given name
// This method uses the constructor.name property for identification
func (v Value) IsCustomClass(className string) bool {
	if !v.IsObject() {
		return false
	}

	// Get constructor
	constructor := v.Get("constructor")
	if constructor.IsUndefined() {
		return false
	}
	defer constructor.Free()

	// Get constructor name
	name := constructor.Get("name")
	if name.IsUndefined() {
		return false
	}
	defer name.Free()

	return name.ToString() == className
}

// IsInstanceOfConstructor checks if the value is an instance of a specific constructor
// This uses JavaScript's instanceof operator semantics
func (v Value) IsInstanceOfConstructor(constructor Value) bool {
	if !v.IsObject() || !constructor.IsFunction() {
		return false
	}

	return C.JS_IsInstanceOf(v.ctx.ref, v.ref, constructor.ref) == 1
}

// =============================================================================
// UTILITY METHODS FOR CLASS INSTANCES
// =============================================================================

// CallMethod calls a method on the class instance with given arguments
// This is equivalent to obj.methodName(args...) in JavaScript
func (v Value) CallMethod(methodName string, args ...Value) Value {
	return v.Call(methodName, args...)
}

// GetProperty gets a property value from the class instance
// This is equivalent to obj.propertyName in JavaScript
func (v Value) GetProperty(propertyName string) Value {
	return v.Get(propertyName)
}

// SetProperty sets a property value on the class instance
// This is equivalent to obj.propertyName = value in JavaScript
func (v Value) SetProperty(propertyName string, value Value) {
	v.Set(propertyName, value)
}

// HasMethod checks if the class instance has a method with given name
// This checks if the property exists and is a function
func (v Value) HasMethod(methodName string) bool {
	if !v.Has(methodName) {
		return false
	}

	method := v.Get(methodName)
	defer method.Free()

	return method.IsFunction()
}

// HasProperty checks if the class instance has a property with given name
// This is a wrapper around the existing Has method for consistency
func (v Value) HasProperty(propertyName string) bool {
	return v.Has(propertyName)
}

// =============================================================================
// DEBUGGING AND INSPECTION METHODS
// =============================================================================

// GetClassName returns the class name of the value if it's a class instance
// Returns empty string if not a class instance or name cannot be determined
func (v Value) GetClassName() string {
	if !v.IsObject() {
		return ""
	}

	constructor := v.Get("constructor")
	if constructor.IsUndefined() {
		return ""
	}
	defer constructor.Free()

	name := constructor.Get("name")
	if name.IsUndefined() {
		return ""
	}
	defer name.Free()

	return name.ToString()
}

// GetObjectInfo returns debugging information about the object
// Useful for development and debugging class instances
func (v Value) GetObjectInfo() map[string]interface{} {
	info := make(map[string]interface{})

	info["isObject"] = v.IsObject()
	info["isClassInstance"] = v.IsClassInstance()
	info["hasInstanceData"] = v.HasInstanceData()
	info["classID"] = int(v.GetClassID())
	info["className"] = v.GetClassName()

	if v.IsFunction() {
		info["isFunction"] = true
		info["isConstructor"] = v.IsConstructor()
	}

	return info
}
