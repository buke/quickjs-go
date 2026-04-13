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
	// borrowed indicates ref is an alias to a JSValue owned elsewhere.
	// Free should only invalidate this Go handle, not decrement QuickJS refcount.
	borrowed bool
}

// PropertyDescriptor mirrors QuickJS property descriptor semantics.
// Value/Getter/Setter returned by OwnProperty are owned by caller and must be freed.
type PropertyDescriptor struct {
	Flags  int
	Value  *Value
	Getter *Value
	Setter *Value
}

// Property descriptor flags matching QuickJS JS_PROP_* constants.
const (
	PropConfigurable = 1 << 0
	PropWritable     = 1 << 1
	PropEnumerable   = 1 << 2

	PropHasConfigurable = 1 << 8
	PropHasWritable     = 1 << 9
	PropHasEnumerable   = 1 << 10
	PropHasGet          = 1 << 11
	PropHasSet          = 1 << 12
	PropHasValue        = 1 << 13
)

// hasValidContext reports whether a Value still has a usable context pointer.
func (v *Value) hasValidContext() bool {
	return v != nil && v.ctx != nil && v.ctx.hasValidRef()
}

// isAlive reports whether a Value can safely reach QuickJS.
func (v *Value) isAlive() bool {
	return v != nil && v.ctx != nil && v.ctx.isAlive() && v.ctx.hasValidRef()
}

// belongsTo reports whether a Value belongs to the given live Context.
func (v *Value) belongsTo(ctx *Context) bool {
	return v != nil && ctx != nil && v.ctx == ctx && v.hasValidContext()
}

func sameContextRef(a *Value, b *Value) bool {
	return a != nil && b != nil && a.ctx != nil && b.ctx != nil && a.ctx == b.ctx
}

// sameContext reports whether two values belong to the same live context.
func sameContext(a *Value, b *Value) bool {
	if !sameContextRef(a, b) {
		return false
	}
	return a.hasValidContext() && b.hasValidContext()
}

// Equal returns true if two values are abstractly equal (JS == semantics).
func (v *Value) Equal(other *Value) bool {
	if !sameContext(v, other) {
		return false
	}
	return C.JS_IsEqual(v.ctx.ref, v.ref, other.ref) == 1
}

// StrictEqual returns true if two values are strictly equal (JS === semantics).
func (v *Value) StrictEqual(other *Value) bool {
	if !sameContext(v, other) {
		return false
	}
	return bool(C.JS_IsStrictEqual(v.ctx.ref, v.ref, other.ref))
}

// SameValue returns true if two values are the same according to JS SameValue.
func (v *Value) SameValue(other *Value) bool {
	if !sameContext(v, other) {
		return false
	}
	return bool(C.JS_IsSameValue(v.ctx.ref, v.ref, other.ref))
}

// SameValueZero returns true if two values are the same according to JS SameValueZero.
func (v *Value) SameValueZero(other *Value) bool {
	if !sameContext(v, other) {
		return false
	}
	return bool(C.JS_IsSameValueZero(v.ctx.ref, v.ref, other.ref))
}

// Free the value.
func (v *Value) Free() {
	if !v.hasValidContext() || bool(C.JS_IsUndefined(v.ref)) {
		return // No context or undefined value, nothing to free
	}
	if v.borrowed {
		v.ref = C.JS_NewUndefined()
		v.borrowed = false
		return
	}
	C.JS_FreeValue(v.ctx.ref, v.ref)
	v.ref = C.JS_NewUndefined()
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
	if !v.isAlive() || !val.belongsTo(v.ctx) {
		return
	}
	var namePtr *C.char
	if len(name) > 0 {
		namePtr = (*C.char)(unsafe.Pointer(unsafe.StringData(name)))
	}
	C.SetPropertyByNameLen(v.ctx.ref, v.ref, namePtr, C.size_t(len(name)), val.ref)
}

// SetIdx sets the value of the property with the given index.
func (v *Value) SetIdx(idx int64, val *Value) {
	if !v.isAlive() || !val.belongsTo(v.ctx) {
		return
	}
	C.JS_SetPropertyUint32(v.ctx.ref, v.ref, C.uint32_t(idx), val.ref)
}

// Get returns the value of the property with the given name.
func (v *Value) Get(name string) *Value {
	if !v.hasValidContext() {
		return nil
	}
	var namePtr *C.char
	if len(name) > 0 {
		namePtr = (*C.char)(unsafe.Pointer(unsafe.StringData(name)))
	}
	return &Value{ctx: v.ctx, ref: C.GetPropertyByNameLen(v.ctx.ref, v.ref, namePtr, C.size_t(len(name)))}
}

// GetIdx returns the value of the property with the given index.
func (v *Value) GetIdx(idx int64) *Value {
	if !v.hasValidContext() {
		return nil
	}
	return &Value{ctx: v.ctx, ref: C.JS_GetPropertyUint32(v.ctx.ref, v.ref, C.uint32_t(idx))}
}

// Call calls the function with the given arguments.
func (v *Value) Call(fname string, args ...*Value) *Value {
	if !v.isAlive() {
		return nil
	}
	var fnamePtr *C.char
	if len(fname) > 0 {
		fnamePtr = (*C.char)(unsafe.Pointer(unsafe.StringData(fname)))
	}
	cargs := []C.JSValue{}
	for _, x := range args {
		if !x.belongsTo(v.ctx) {
			return nil
		}
		cargs = append(cargs, x.ref)
	}
	var ref C.JSValue
	if len(cargs) == 0 {
		ref = C.CallPropertyByNameLen(v.ctx.ref, v.ref, fnamePtr, C.size_t(len(fname)), C.int(0), nil)
	} else {
		ref = C.CallPropertyByNameLen(v.ctx.ref, v.ref, fnamePtr, C.size_t(len(fname)), C.int(len(cargs)), &cargs[0])
	}

	return &Value{ctx: v.ctx, ref: ref}
}

// Execute the function with the given arguments.
func (v *Value) Execute(this *Value, args ...*Value) *Value {
	if !v.isAlive() || !this.belongsTo(v.ctx) {
		return nil
	}
	cargs := []C.JSValue{}
	for _, x := range args {
		if !x.belongsTo(v.ctx) {
			return nil
		}
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
	if !v.isAlive() {
		return nil
	}
	cargs := []C.JSValue{}
	for _, x := range args {
		if !x.belongsTo(v.ctx) {
			return nil
		}
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
			IsEnumerable: bool(entries[i].is_enumerable),
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

// DefineProperty defines a property using a full descriptor.
func (v *Value) DefineProperty(name string, desc PropertyDescriptor) bool {
	if !v.isAlive() {
		return false
	}
	atom := v.ctx.NewAtom(name)
	defer atom.Free()
	return v.DefinePropertyAtom(atom, desc)
}

// DefinePropertyAtom defines a property using a full descriptor and a pre-built atom.
func (v *Value) DefinePropertyAtom(atom *Atom, desc PropertyDescriptor) bool {
	if !v.isAlive() || atom == nil || atom.ctx == nil || atom.ctx != v.ctx || !atom.ctx.hasValidRef() {
		return false
	}
	if desc.Value != nil && !desc.Value.belongsTo(v.ctx) {
		return false
	}
	if desc.Getter != nil && !desc.Getter.belongsTo(v.ctx) {
		return false
	}
	if desc.Setter != nil && !desc.Setter.belongsTo(v.ctx) {
		return false
	}

	value := C.JS_NewUndefined()
	if desc.Value != nil {
		// JS_DefineProperty borrows descriptor values; do not transfer caller ownership.
		value = desc.Value.ref
	}
	getter := C.JS_NewUndefined()
	if desc.Getter != nil {
		// Borrowed argument: keep caller-managed getter handle valid after the call.
		getter = desc.Getter.ref
	}
	setter := C.JS_NewUndefined()
	if desc.Setter != nil {
		// Borrowed argument: keep caller-managed setter handle valid after the call.
		setter = desc.Setter.ref
	}

	ret := C.JS_DefineProperty(v.ctx.ref, v.ref, atom.ref, value, getter, setter, C.int(desc.Flags))
	if ret < 0 {
		return false
	}
	return ret == 1
}

// DefinePropertyValue defines a value property by name.
func (v *Value) DefinePropertyValue(name string, value *Value, flags int) bool {
	if !v.isAlive() || !value.belongsTo(v.ctx) {
		return false
	}
	atom := v.ctx.NewAtom(name)
	defer atom.Free()

	// JS_DefinePropertyValue consumes `dup` but does not consume `atom`.
	dup := C.JS_DupValue(v.ctx.ref, value.ref)
	ret := C.JS_DefinePropertyValue(v.ctx.ref, v.ref, atom.ref, dup, C.int(flags))
	if ret < 0 {
		return false
	}
	return ret == 1
}

// DefinePropertyGetSet defines an accessor property by name.
func (v *Value) DefinePropertyGetSet(name string, getter *Value, setter *Value, flags int) bool {
	if !v.isAlive() {
		return false
	}
	if getter != nil && !getter.belongsTo(v.ctx) {
		return false
	}
	if setter != nil && !setter.belongsTo(v.ctx) {
		return false
	}
	atom := v.ctx.NewAtom(name)
	defer atom.Free()

	getterRef := C.JS_NewUndefined()
	setterRef := C.JS_NewUndefined()
	if getter != nil {
		// JS_DefinePropertyGetSet consumes getter/setter values.
		getterRef = C.JS_DupValue(v.ctx.ref, getter.ref)
	}
	if setter != nil {
		// JS_DefinePropertyGetSet consumes getter/setter values.
		setterRef = C.JS_DupValue(v.ctx.ref, setter.ref)
	}

	ret := C.JS_DefinePropertyGetSet(v.ctx.ref, v.ref, atom.ref, getterRef, setterRef, C.int(flags))
	if ret < 0 {
		return false
	}
	return ret == 1
}

// OwnProperty gets a full own property descriptor by name.
func (v *Value) OwnProperty(name string) (*PropertyDescriptor, bool) {
	if !v.isAlive() {
		return nil, false
	}
	atom := v.ctx.NewAtom(name)
	defer atom.Free()

	var desc C.JSPropertyDescriptor
	ret := C.JS_GetOwnProperty(v.ctx.ref, &desc, v.ref, atom.ref)
	if ret <= 0 {
		return nil, false
	}

	result := &PropertyDescriptor{
		Flags:  int(desc.flags),
		Value:  &Value{ctx: v.ctx, ref: desc.value},
		Getter: &Value{ctx: v.ctx, ref: desc.getter},
		Setter: &Value{ctx: v.ctx, ref: desc.setter},
	}
	return result, true
}

// GetAtom gets a property by atom key.
func (v *Value) GetAtom(atom *Atom) *Value {
	if !v.isAlive() || atom == nil || atom.ctx == nil || atom.ctx != v.ctx || !atom.ctx.hasValidRef() {
		return nil
	}
	// JS_GetProperty borrows atom; caller retains atom ownership.
	return &Value{ctx: v.ctx, ref: C.JS_GetProperty(v.ctx.ref, v.ref, atom.ref)}
}

// SetAtom sets a property by atom key.
func (v *Value) SetAtom(atom *Atom, val *Value) bool {
	if !v.isAlive() || atom == nil || atom.ctx == nil || atom.ctx != v.ctx || !atom.ctx.hasValidRef() || !val.belongsTo(v.ctx) {
		return false
	}
	// JS_SetProperty consumes `dup` but borrows `atom`.
	dup := C.JS_DupValue(v.ctx.ref, val.ref)
	ret := C.JS_SetProperty(v.ctx.ref, v.ref, atom.ref, dup)
	if ret < 0 {
		return false
	}
	return ret == 1
}

// GetInt64 gets a property by int64 key.
func (v *Value) GetInt64(idx int64) *Value {
	if !v.hasValidContext() {
		return nil
	}
	return &Value{ctx: v.ctx, ref: C.JS_GetPropertyInt64(v.ctx.ref, v.ref, C.int64_t(idx))}
}

// SetInt64 sets a property by int64 key.
func (v *Value) SetInt64(idx int64, val *Value) bool {
	if !v.isAlive() || !val.belongsTo(v.ctx) {
		return false
	}
	dup := C.JS_DupValue(v.ctx.ref, val.ref)
	ret := C.JS_SetPropertyInt64(v.ctx.ref, v.ref, C.int64_t(idx), dup)
	if ret < 0 {
		return false
	}
	return ret == 1
}

// Prototype returns the object's prototype value.
func (v *Value) Prototype() *Value {
	if !v.isAlive() {
		return nil
	}
	return &Value{ctx: v.ctx, ref: C.JS_GetPrototype(v.ctx.ref, v.ref)}
}

// SetPrototype sets the object's prototype.
func (v *Value) SetPrototype(proto *Value) bool {
	if !v.isAlive() || !proto.belongsTo(v.ctx) {
		return false
	}
	return C.JS_SetPrototype(v.ctx.ref, v.ref, proto.ref) == 1
}

// IsExtensible returns true if new properties can still be added.
func (v *Value) IsExtensible() bool {
	if !v.isAlive() {
		return false
	}
	ret := C.JS_IsExtensible(v.ctx.ref, v.ref)
	if ret < 0 {
		return false
	}
	return ret == 1
}

// PreventExtensions marks object as non-extensible.
func (v *Value) PreventExtensions() bool {
	if !v.isAlive() {
		return false
	}
	ret := C.JS_PreventExtensions(v.ctx.ref, v.ref)
	if ret < 0 {
		return false
	}
	return ret == 1
}

// Seal seals object properties and prevents extensions.
func (v *Value) Seal() bool {
	if !v.isAlive() {
		return false
	}
	ret := C.JS_SealObject(v.ctx.ref, v.ref)
	if ret < 0 {
		return false
	}
	return ret == 1
}

// Freeze freezes object properties and prevents extensions.
func (v *Value) Freeze() bool {
	if !v.isAlive() {
		return false
	}
	ret := C.JS_FreezeObject(v.ctx.ref, v.ref)
	if ret < 0 {
		return false
	}
	return ret == 1
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
// BASIC TYPE CHECKING METHODS
// =============================================================================

func (v *Value) IsNumber() bool        { return v != nil && bool(C.JS_IsNumber(v.ref)) }
func (v *Value) IsBigInt() bool        { return v != nil && bool(C.JS_IsBigInt(v.ref)) }
func (v *Value) IsBool() bool          { return v != nil && bool(C.JS_IsBool(v.ref)) }
func (v *Value) IsNull() bool          { return v != nil && bool(C.JS_IsNull(v.ref)) }
func (v *Value) IsUndefined() bool     { return v != nil && bool(C.JS_IsUndefined(v.ref)) }
func (v *Value) IsException() bool     { return v != nil && bool(C.JS_IsException(v.ref)) }
func (v *Value) IsUninitialized() bool { return v != nil && bool(C.JS_IsUninitialized(v.ref)) }
func (v *Value) IsString() bool        { return v != nil && bool(C.JS_IsString(v.ref)) }
func (v *Value) IsSymbol() bool        { return v != nil && bool(C.JS_IsSymbol(v.ref)) }
func (v *Value) IsObject() bool        { return v != nil && bool(C.JS_IsObject(v.ref)) }
func (v *Value) IsArray() bool         { return v != nil && bool(C.JS_IsArray(v.ref)) }
func (v *Value) IsDate() bool          { return v != nil && bool(C.JS_IsDate(v.ref)) }
func (v *Value) IsError() bool         { return v != nil && bool(C.JS_IsError(v.ref)) }
func (v *Value) IsFunction() bool      { return v != nil && bool(C.JS_IsFunction(v.ctx.ref, v.ref)) }
func (v *Value) IsConstructor() bool   { return v != nil && bool(C.JS_IsConstructor(v.ctx.ref, v.ref)) }

// =============================================================================
// PROMISE SUPPORT METHODS (replaced constants with getter functions)
// =============================================================================

func (v *Value) IsPromise() bool {
	if v == nil {
		return false
	}
	state := C.JS_PromiseState(v.ctx.ref, v.ref)
	pending := C.int(C.JS_PROMISE_PENDING)
	fulfilled := C.int(C.JS_PROMISE_FULFILLED)
	rejected := C.int(C.JS_PROMISE_REJECTED)

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
	case C.JSPromiseStateEnum(C.JS_PROMISE_PENDING):
		return PromisePending
	case C.JSPromiseStateEnum(C.JS_PROMISE_FULFILLED):
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
	if v == nil || !v.hasValidContext() || v.ctx.handleStore == nil || !v.IsObject() {
		return false
	}

	// Get class ID first
	classID := C.JS_GetClassID(v.ref)

	// Use JS_GetOpaque2 for type-safe check (like point.c methods)
	opaque := C.JS_GetOpaque2(v.ctx.ref, v.ref, classID)
	if opaque == nil {
		return false
	}

	ownerCtx, handleID, ok := resolveClassObjectFromOpaque(v.ctx, opaque)
	if !ok || ownerCtx == nil || ownerCtx.handleStore == nil {
		return false
	}
	_, exists := ownerCtx.handleStore.Load(handleID)
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
	if v == nil || v.ctx == nil {
		return nil, errors.New("value context is not available")
	}
	if v.ctx.runtime == nil || !v.ctx.runtime.ensureOwnerAccess() {
		return nil, errOwnerAccessDenied
	}
	if v.ctx.ref == nil || v.ctx.handleStore == nil {
		return nil, errors.New("value context is not available")
	}

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

	ownerCtx, handleID, ok := resolveClassObjectFromOpaque(v.ctx, opaque)
	if !ok || ownerCtx == nil || ownerCtx.handleStore == nil {
		return nil, errors.New("instance data not found in handle store")
	}

	// Retrieve Go object from resolved HandleStore
	if obj, exists := ownerCtx.handleStore.Load(handleID); exists {
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

		if classID, exists := getConstructorClassID(v.ctx, parent.ref); exists {
			return classID, true
		}
	}

	return 0, false
}
