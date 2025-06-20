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

// Value represents a Javascript value which can be a primitive type or an object.
// Reference counting is used, so it is important to explicitly duplicate (JS_DupValue(),
// increment the reference count) or free (JS_FreeValue(), decrement the reference count) JSValues.
type Value struct {
	ctx *Context
	ref C.JSValue
}

// Free the value.
func (v *Value) Free() {
	if v.ctx == nil || C.JS_IsUndefined_Wrapper(v.ref) == 1 {
		return // No context or undefined value, nothing to free
	}
	C.JS_FreeValue(v.ctx.ref, v.ref)
}

// Context returns the context of the value.
func (v *Value) Context() *Context {
	return v.ctx
}

// Deprecated: Use ToBool instead.
func (v *Value) Bool() bool {
	return v.ToBool()
}

// ToBool returns the boolean value of the value.
func (v *Value) ToBool() bool {
	return C.JS_ToBool(v.ctx.ref, v.ref) == 1
}

// String returns the string representation of the value.
// This method implements the fmt.Stringer interface.
func (v *Value) String() string {
	return v.ToString()
}

// ToString returns the string representation of the value.
func (v *Value) ToString() string {
	ptr := C.JS_ToCString(v.ctx.ref, v.ref)
	defer C.JS_FreeCString(v.ctx.ref, ptr)
	return C.GoString(ptr)
}

// JSONStringify returns the JSON string representation of the value.
func (v *Value) JSONStringify() string {
	ref := C.JS_JSONStringify(v.ctx.ref, v.ref, C.JS_NewNull(), C.JS_NewNull())
	ptr := C.JS_ToCString(v.ctx.ref, ref)
	defer C.JS_FreeCString(v.ctx.ref, ptr)
	return C.GoString(ptr)
}

func (v *Value) ToByteArray(size uint) ([]byte, error) {
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

// Deprecated: Use ToInt64 instead.
func (v *Value) Int64() int64 {
	return v.ToInt64()
}

// ToInt64 returns the int64 value of the value.
func (v *Value) ToInt64() int64 {
	val := C.int64_t(0)
	C.JS_ToInt64(v.ctx.ref, &val, v.ref)
	return int64(val)
}

// Deprecated: Use ToInt32 instead.
func (v *Value) Int32() int32 {
	return v.ToInt32()
}

// ToInt32 returns the int32 value of the value.
func (v *Value) ToInt32() int32 {
	val := C.int32_t(0)
	C.JS_ToInt32(v.ctx.ref, &val, v.ref)
	return int32(val)
}

// Deprecated: Use ToUint32 instead.
func (v *Value) Uint32() uint32 {
	return v.ToUint32()
}

// ToUint32 returns the uint32 value of the value.
func (v *Value) ToUint32() uint32 {
	val := C.uint32_t(0)
	C.JS_ToUint32(v.ctx.ref, &val, v.ref)
	return uint32(val)
}

// Deprecated: Use ToFloat64 instead.
func (v *Value) Float64() float64 {
	return v.ToFloat64()
}

// ToFloat64 returns the float64 value of the value.
func (v *Value) ToFloat64() float64 {
	val := C.double(0)
	C.JS_ToFloat64(v.ctx.ref, &val, v.ref)
	return float64(val)
}

// Deprecated: Use ToBigInt instead.
func (v *Value) BigInt() *big.Int {
	return v.ToBigInt()
}

// ToBigInt returns the big.Int value of the value.
func (v *Value) ToBigInt() *big.Int {
	if !v.IsBigInt() {
		return nil
	}
	val, _ := new(big.Int).SetString(v.ToString(), 10)
	return val
}

// Len returns the length of the array.
func (v *Value) Len() int64 {
	length := v.Get("length")
	defer length.Free()
	return length.ToInt64()
}

// ByteLen returns the length of the ArrayBuffer.
func (v *Value) ByteLen() int64 {
	byteLength := v.Get("byteLength")
	defer byteLength.Free()
	return byteLength.ToInt64()
}

// Set sets the value of the property with the given name.
func (v *Value) Set(name string, val *Value) {
	namePtr := C.CString(name)
	defer C.free(unsafe.Pointer(namePtr))
	C.JS_SetPropertyStr(v.ctx.ref, v.ref, namePtr, val.ref)
}

// SetIdx sets the value of the property with the given index.
func (v *Value) SetIdx(idx int64, val *Value) {
	C.JS_SetPropertyUint32(v.ctx.ref, v.ref, C.uint32_t(idx), val.ref)
}

// Get returns the value of the property with the given name.
func (v *Value) Get(name string) *Value {
	namePtr := C.CString(name)
	defer C.free(unsafe.Pointer(namePtr))
	return &Value{ctx: v.ctx, ref: C.JS_GetPropertyStr(v.ctx.ref, v.ref, namePtr)}
}

// GetIdx returns the value of the property with the given index.
func (v *Value) GetIdx(idx int64) *Value {
	return &Value{ctx: v.ctx, ref: C.JS_GetPropertyUint32(v.ctx.ref, v.ref, C.uint32_t(idx))}
}

// Call calls the function with the given arguments.
func (v *Value) Call(fname string, args ...*Value) *Value {
	fn := v.Get(fname) // get the function by name
	defer fn.Free()

	cargs := []C.JSValue{}
	for _, x := range args {
		cargs = append(cargs, x.ref)
	}
	var val *Value
	if len(cargs) == 0 {
		val = &Value{ctx: v.ctx, ref: C.JS_Call(v.ctx.ref, fn.ref, v.ref, C.int(0), nil)}
	} else {
		val = &Value{ctx: v.ctx, ref: C.JS_Call(v.ctx.ref, fn.ref, v.ref, C.int(len(cargs)), &cargs[0])}
	}

	return val
}

// Execute the function with the given arguments.
func (v *Value) Execute(this *Value, args ...*Value) *Value {
	cargs := []C.JSValue{}
	for _, x := range args {
		cargs = append(cargs, x.ref)
	}
	var val *Value
	if len(cargs) == 0 {
		val = &Value{ctx: v.ctx, ref: C.JS_Call(v.ctx.ref, v.ref, this.ref, C.int(0), nil)}
	} else {
		val = &Value{ctx: v.ctx, ref: C.JS_Call(v.ctx.ref, v.ref, this.ref, C.int(len(cargs)), &cargs[0])}
	}

	return val
}

// New calls the constructor with the given arguments.
func (v *Value) New(args ...*Value) *Value {
	return v.CallConstructor(args...)
}

// CallConstructor calls the constructor with the given arguments.
// SCHEME C: For class instances, use this method to create instances.
// The class constructor function will receive a pre-created instance and initialize it.
// Instance properties declared in ClassBuilder.Property() are automatically bound to the instance.
//
// Example usage:
//
//	constructor := ctx.Eval("MyClass")
//	instance := constructor.CallConstructor(arg1, arg2)
//
// This replaces the previous NewInstance method and provides automatic property binding
// and simplified constructor semantics where constructors work with pre-created instances.
func (v *Value) CallConstructor(args ...*Value) *Value {
	cargs := []C.JSValue{}
	for _, x := range args {
		cargs = append(cargs, x.ref)
	}
	var val *Value
	if len(cargs) == 0 {
		val = &Value{ctx: v.ctx, ref: C.JS_CallConstructor(v.ctx.ref, v.ref, C.int(0), nil)}
	} else {
		val = &Value{ctx: v.ctx, ref: C.JS_CallConstructor(v.ctx.ref, v.ref, C.int(len(cargs)), &cargs[0])}
	}

	return val
}

// Deprecated: Use ToError() instead.
func (v *Value) Error() error {
	return v.ToError()
}

// ToError returns the error value of the value.
func (v *Value) ToError() error {
	if !v.IsError() {
		return nil
	}

	err := &Error{}

	name := v.Get("name")
	defer name.Free()
	if !name.IsUndefined() {
		err.Name = name.ToString()
	}

	message := v.Get("message")
	defer message.Free()
	if !message.IsUndefined() {
		err.Message = message.ToString()
	}

	cause := v.Get("cause")
	defer cause.Free()
	if !cause.IsUndefined() {
		err.Cause = cause.ToString()
	}

	stack := v.Get("stack")
	defer stack.Free()
	if !stack.IsUndefined() {
		err.Stack = stack.ToString()
	}

	jsonString := v.JSONStringify()
	if jsonString != "" {
		err.JSONString = jsonString
	}

	return err
}

// propertyEnum is a wrapper around JSValue.
func (v *Value) propertyEnum() ([]*propertyEnum, error) {
	var ptr *C.JSPropertyEnum
	var size C.uint32_t

	result := int(C.JS_GetOwnPropertyNames(v.ctx.ref, &ptr, &size, v.ref, C.int(1<<0|1<<1|1<<2)))
	if result < 0 {
		return nil, errors.New("value does not contain properties")
	}
	defer C.js_free(v.ctx.ref, unsafe.Pointer(ptr))

	entries := unsafe.Slice(ptr, size) // Go 1.17 and later
	names := make([]*propertyEnum, len(entries))
	for i := 0; i < len(names); i++ {
		names[i] = &propertyEnum{
			IsEnumerable: entries[i].is_enumerable == 1,
			atom:         &Atom{ctx: v.ctx, ref: entries[i].atom},
		}
		names[i].atom.Free()
	}

	return names, nil
}

// PropertyNames returns the names of the properties of the value.
func (v *Value) PropertyNames() ([]string, error) {
	pList, err := v.propertyEnum()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(pList))
	for i := 0; i < len(names); i++ {
		names[i] = pList[i].ToString()
	}
	return names, nil
}

// Has returns true if the value has the property with the given name.
func (v *Value) Has(name string) bool {
	prop := v.ctx.NewAtom(name)
	defer prop.Free()
	return C.JS_HasProperty(v.ctx.ref, v.ref, prop.ref) == 1
}

// HasIdx returns true if the value has the property with the given index.
func (v *Value) HasIdx(idx uint32) bool {
	prop := v.ctx.NewAtomIdx(idx)
	defer prop.Free()
	return C.JS_HasProperty(v.ctx.ref, v.ref, prop.ref) == 1
}

// Delete deletes the property with the given name.
func (v *Value) Delete(name string) bool {
	if !v.Has(name) {
		return false // Property does not exist, nothing to delete
	}
	prop := v.ctx.NewAtom(name)
	defer prop.Free()
	return C.JS_DeleteProperty(v.ctx.ref, v.ref, prop.ref, C.int(1)) == 1
}

// DeleteIdx deletes the property with the given index.
func (v *Value) DeleteIdx(idx uint32) bool {
	if !v.HasIdx(idx) {
		return false // Property does not exist, nothing to delete
	}
	return C.JS_DeletePropertyInt64(v.ctx.ref, v.ref, C.int64_t(idx), C.int(1)) == 1
}

// GlobalInstanceof checks if the value is an instance of the given global constructor
func (v *Value) GlobalInstanceof(name string) bool {
	ctor := v.ctx.Globals().Get(name)
	defer ctor.Free()
	if ctor.IsUndefined() {
		return false
	}
	return C.JS_IsInstanceOf(v.ctx.ref, v.ref, ctor.ref) == 1
}

// getTypedArrayInfo is a helper function to extract TypedArray information using C API
func (v *Value) getTypedArrayInfo() (buffer *Value, byteOffset, byteLength, bytesPerElement int) {
	var cByteOffset, cByteLength, cBytesPerElement C.size_t
	bufferRef := C.JS_GetTypedArrayBuffer(v.ctx.ref, v.ref, &cByteOffset, &cByteLength, &cBytesPerElement)

	return &Value{ctx: v.ctx, ref: bufferRef},
		int(cByteOffset), int(cByteLength), int(cBytesPerElement)
}

// ToInt8Array converts the value to int8 slice if it's an Int8Array.
func (v *Value) ToInt8Array() ([]int8, error) {
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
func (v *Value) ToUint8Array() ([]uint8, error) {
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
func (v *Value) ToInt16Array() ([]int16, error) {
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
func (v *Value) ToUint16Array() ([]uint16, error) {
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
func (v *Value) ToInt32Array() ([]int32, error) {
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
func (v *Value) ToUint32Array() ([]uint32, error) {
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
func (v *Value) ToFloat32Array() ([]float32, error) {
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
func (v *Value) ToFloat64Array() ([]float64, error) {
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
func (v *Value) ToBigInt64Array() ([]int64, error) {
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
func (v *Value) ToBigUint64Array() ([]uint64, error) {
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

func (v *Value) IsNumber() bool        { return v != nil && C.JS_IsNumber_Wrapper(v.ref) == 1 }
func (v *Value) IsBigInt() bool        { return v != nil && C.JS_IsBigInt_Wrapper(v.ctx.ref, v.ref) == 1 }
func (v *Value) IsBool() bool          { return v != nil && C.JS_IsBool_Wrapper(v.ref) == 1 }
func (v *Value) IsNull() bool          { return v != nil && C.JS_IsNull_Wrapper(v.ref) == 1 }
func (v *Value) IsUndefined() bool     { return v != nil && C.JS_IsUndefined_Wrapper(v.ref) == 1 }
func (v *Value) IsException() bool     { return v != nil && C.JS_IsException_Wrapper(v.ref) == 1 }
func (v *Value) IsUninitialized() bool { return v != nil && C.JS_IsUninitialized_Wrapper(v.ref) == 1 }
func (v *Value) IsString() bool        { return v != nil && C.JS_IsString_Wrapper(v.ref) == 1 }
func (v *Value) IsSymbol() bool        { return v != nil && C.JS_IsSymbol_Wrapper(v.ref) == 1 }
func (v *Value) IsObject() bool        { return v != nil && C.JS_IsObject_Wrapper(v.ref) == 1 }
func (v *Value) IsArray() bool         { return v != nil && C.JS_IsArray(v.ctx.ref, v.ref) == 1 }
func (v *Value) IsError() bool         { return v != nil && C.JS_IsError(v.ctx.ref, v.ref) == 1 }
func (v *Value) IsFunction() bool      { return v != nil && C.JS_IsFunction(v.ctx.ref, v.ref) == 1 }
func (v *Value) IsConstructor() bool   { return v != nil && C.JS_IsConstructor(v.ctx.ref, v.ref) == 1 }

// =============================================================================
// PROMISE SUPPORT METHODS (replaced constants with getter functions)
// =============================================================================

func (v *Value) IsPromise() bool {
	if v == nil {
		return false
	}
	state := C.JS_PromiseState(v.ctx.ref, v.ref)
	pending := C.GetPromisePending()
	fulfilled := C.GetPromiseFulfilled()
	rejected := C.GetPromiseRejected()

	return C.int(state) == pending || C.int(state) == fulfilled || C.int(state) == rejected
}

// Promise state enumeration matching QuickJS
type PromiseState int

const (
	PromisePending PromiseState = iota
	PromiseFulfilled
	PromiseRejected
)

// PromiseState returns the state of the Promise
func (v *Value) PromiseState() PromiseState {
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
func (v *Value) Await() *Value {
	return v.ctx.Await(v)
}

// =============================================================================
// CLASS INSTANCE SUPPORT METHODS (replaced invalid class ID constant)
// =============================================================================

// IsClassInstance checks if the value is an instance of any user-defined class
// This method uses opaque data validation for maximum reliability
func (v *Value) IsClassInstance() bool {
	return v != nil && v.HasInstanceData()
}

// HasInstanceData checks if the value has associated Go object data
// This is the most reliable way to identify our class instances
func (v *Value) HasInstanceData() bool {
	if v == nil || !v.IsObject() {
		return false
	}

	// Get class ID first
	classID := C.JS_GetClassID(v.ref)

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

// IsInstanceOfClassID checks if the value is an instance of a specific class ID
// This provides type-safe class instance checking with double validation
func (v *Value) IsInstanceOfClassID(expectedClassID uint32) bool {
	if v == nil || !v.IsObject() {
		return false
	}

	// Only check class ID match - no opaque data requirement
	objClassID := uint32(C.JS_GetClassID(v.ref))
	return objClassID == expectedClassID
}

// GetClassID returns the class ID of the value if it's a class instance
// Returns JS_INVALID_CLASS_ID (0) if not a class instance
func (v *Value) GetClassID() uint32 {
	return uint32(C.JS_GetClassID(v.ref))
}

// GetGoObject retrieves Go object from JavaScript class instance
// This method extracts the opaque data stored by the constructor proxy
func (v *Value) GetGoObject() (interface{}, error) {
	// First check if the value is an object
	if !v.IsObject() {
		return nil, errors.New("value is not an object")
	}

	// Get class ID to ensure we have a class instance
	classID := C.JS_GetClassID(v.ref)

	// Use JS_GetOpaque2 for type-safe retrieval with context validation
	// This corresponds to point.c: s = JS_GetOpaque2(ctx, this_val, js_point_class_id)
	opaque := C.JS_GetOpaque2(v.ctx.ref, v.ref, classID)
	if opaque == nil {
		return nil, errors.New("no instance data found")
	}

	// Use C helper function to safely convert opaque pointer back to int32
	handleID := int32(C.OpaqueToInt(opaque))

	// Retrieve Go object from HandleStore
	if obj, exists := v.ctx.handleStore.Load(handleID); exists {
		return obj, nil
	}

	return nil, errors.New("instance data not found in handle store")
}

// =============================================================================
// SPECIALIZED CLASS TYPE CHECKING METHODS
// =============================================================================

// IsInstanceOfConstructor checks if the value is an instance of a specific constructor
// This uses JavaScript's instanceof operator semantics
func (v *Value) IsInstanceOfConstructor(constructor *Value) bool {
	return v != nil && constructor != nil && v.IsObject() && constructor.IsFunction() &&
		C.JS_IsInstanceOf(v.ctx.ref, v.ref, constructor.ref) == 1
}

// TypedArray detection methods
func (v *Value) IsTypedArray() bool {
	if v == nil {
		return false
	}
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

func (v *Value) IsInt8Array() bool  { return v != nil && v.GlobalInstanceof("Int8Array") }
func (v *Value) IsUint8Array() bool { return v != nil && v.GlobalInstanceof("Uint8Array") }
func (v *Value) IsUint8ClampedArray() bool {
	return v != nil && v.GlobalInstanceof("Uint8ClampedArray")
}
func (v *Value) IsInt16Array() bool     { return v != nil && v.GlobalInstanceof("Int16Array") }
func (v *Value) IsUint16Array() bool    { return v != nil && v.GlobalInstanceof("Uint16Array") }
func (v *Value) IsInt32Array() bool     { return v != nil && v.GlobalInstanceof("Int32Array") }
func (v *Value) IsUint32Array() bool    { return v != nil && v.GlobalInstanceof("Uint32Array") }
func (v *Value) IsFloat32Array() bool   { return v != nil && v.GlobalInstanceof("Float32Array") }
func (v *Value) IsFloat64Array() bool   { return v != nil && v.GlobalInstanceof("Float64Array") }
func (v *Value) IsBigInt64Array() bool  { return v != nil && v.GlobalInstanceof("BigInt64Array") }
func (v *Value) IsBigUint64Array() bool { return v != nil && v.GlobalInstanceof("BigUint64Array") }

// IsByteArray returns true if the value is array buffer
func (v *Value) IsByteArray() bool {
	return v != nil && v.IsObject() && (v.GlobalInstanceof("ArrayBuffer") || v.ToString() == "[object ArrayBuffer]")
}

// resolveClassIDFromInheritance attempts to resolve classID by checking if this constructor
// extends a registered class and should use the parent's classID
func (v *Value) resolveClassIDFromInheritance() (uint32, bool) {
	// Simple and efficient approach: use JavaScript to traverse the prototype chain
	script := `
        (function(child) {
            // Walk up the prototype chain and collect all parent constructors
            let constructors = [];
            let current = child;
            
            // Traverse up to 10 levels to prevent infinite loops
            for (let i = 0; i < 10; i++) {
                if (!current || !current.prototype) break;
                
                let parentProto = Object.getPrototypeOf(current.prototype);
                if (!parentProto || parentProto === Object.prototype) break;
                
                let parentConstructor = parentProto.constructor;
                if (!parentConstructor || parentConstructor === current) break;
                
                constructors.push(parentConstructor);
                current = parentConstructor;
            }
            
            return constructors;
        })
    `

	traverser := v.ctx.Eval(script)
	defer traverser.Free()

	// Get all parent constructors
	undefinedVal := v.ctx.NewUndefined()
	defer undefinedVal.Free()
	parents := traverser.Execute(undefinedVal, v)
	defer parents.Free()

	// Check each parent to see if it's registered
	lengthVal := parents.Get("length")
	defer lengthVal.Free()

	length := int(lengthVal.ToInt32())
	for i := 0; i < length; i++ {
		parent := parents.GetIdx(int64(i))
		defer parent.Free()

		if classID, exists := getConstructorClassID(parent.ref); exists {
			return classID, true
		}
	}

	return 0, false
}
