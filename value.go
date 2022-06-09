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

type Error struct {
	Cause string
	Stack string
}

func (err Error) Error() string { return err.Cause }

// Object property names and some strings are stored as Atoms (unique strings) to save memory and allow fast comparison. Atoms are represented as a 32 bit integer. Half of the atom range is reserved for immediate integer literals from 0 to 2^{31}-1.
type Atom struct {
	ctx *Context
	ref C.JSAtom
}

// Free the value.
func (a Atom) Free() {
	C.JS_FreeAtom(a.ctx.ref, a.ref)
}

// String returns the string representation of the value.
func (a Atom) String() string {
	ptr := C.JS_AtomToCString(a.ctx.ref, a.ref)
	defer C.JS_FreeCString(a.ctx.ref, ptr)
	return C.GoString(ptr)
}

// Value returns the value of the Atom object.
func (a Atom) Value() Value {
	return Value{ctx: a.ctx, ref: C.JS_AtomToValue(a.ctx.ref, a.ref)}
}

// propertyEnum is a wrapper around JSAtom.
type propertyEnum struct {
	IsEnumerable bool
	atom         Atom
}

//String returns the atom string representation of the value.
func (p propertyEnum) String() string { return p.atom.String() }

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

// Bool returns the boolean value of the value.
func (v Value) Bool() bool {
	return C.JS_ToBool(v.ctx.ref, v.ref) == 1
}

// String returns the string representation of the value.
func (v Value) String() string {
	ptr := C.JS_ToCString(v.ctx.ref, v.ref)
	defer C.JS_FreeCString(v.ctx.ref, ptr)
	return C.GoString(ptr)
}

// Int64 returns the int64 value of the value.
func (v Value) Int64() int64 {
	val := C.int64_t(0)
	C.JS_ToInt64(v.ctx.ref, &val, v.ref)
	return int64(val)
}

// Int32 returns the int32 value of the value.
func (v Value) Int32() int32 {
	val := C.int32_t(0)
	C.JS_ToInt32(v.ctx.ref, &val, v.ref)
	return int32(val)
}

// Uint32 returns the uint32 value of the value.
func (v Value) Uint32() uint32 {
	val := C.uint32_t(0)
	C.JS_ToUint32(v.ctx.ref, &val, v.ref)
	return uint32(val)
}

// Float64 returns the float64 value of the value.
func (v Value) Float64() float64 {
	val := C.double(0)
	C.JS_ToFloat64(v.ctx.ref, &val, v.ref)
	return float64(val)
}

// BigInt returns the big.Int value of the value.
func (v Value) BigInt() *big.Int {
	if !v.IsBigInt() {
		return nil
	}
	val, ok := new(big.Int).SetString(v.String(), 10)
	if !ok {
		return nil
	}
	return val
}

// BigFloat returns the big.Float value of the value.
func (v Value) BigFloat() *big.Float {
	if !v.IsBigDecimal() && !v.IsBigFloat() {
		return nil
	}
	val, ok := new(big.Float).SetString(v.String())
	if !ok {
		return nil
	}
	return val
}

// Len returns the length of the array.
func (v Value) Len() int64 {
	return v.Get("length").Int64()
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
	if !v.IsObject() {
		return v.ctx.Error(errors.New("Object not a object"))
	}

	fn := v.Get(fname) // get the function by name
	defer fn.Free()

	if !fn.IsFunction() {
		return v.ctx.Error(errors.New("Object not a function"))
	}

	cargs := []C.JSValue{}
	for _, x := range args {
		cargs = append(cargs, x.ref)
	}

	return Value{ctx: v.ctx, ref: C.JS_Call(v.ctx.ref, fn.ref, v.ref, C.int(len(cargs)), &cargs[0])}
}

// Error returns the error value of the value.
func (v Value) Error() error {
	if !v.IsError() {
		return nil
	}
	cause := v.String()

	stack := v.Get("stack")
	defer stack.Free()

	if stack.IsUndefined() {
		return &Error{Cause: cause}
	}
	return &Error{Cause: cause, Stack: stack.String()}
}

// propertyEnum is a wrapper around JSValue.
func (v Value) propertyEnum() ([]propertyEnum, error) {
	var (
		ptr  *C.JSPropertyEnum
		size C.uint32_t
	)

	result := int(C.JS_GetOwnPropertyNames(v.ctx.ref, &ptr, &size, v.ref, C.int(1<<0|1<<1|1<<2)))
	if result < 0 {
		return nil, errors.New("value does not contain properties")
	}
	defer C.js_free(v.ctx.ref, unsafe.Pointer(ptr))

	entries := (*[(1 << 29) - 1]C.JSPropertyEnum)(unsafe.Pointer(ptr))

	names := make([]propertyEnum, uint32(size))

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
func (v Value) HasIdx(idx int64) bool {
	prop := v.ctx.AtomIdx(idx)
	defer prop.Free()
	return C.JS_HasProperty(v.ctx.ref, v.ref, prop.ref) == 1
}

// Delete deletes the property with the given name.
func (v Value) Delete(name string) bool {
	prop := v.ctx.Atom(name)
	defer prop.Free()
	return C.JS_DeleteProperty(v.ctx.ref, v.ref, prop.ref, C.int(1)) == 1
}

// DeleteIdx deletes the property with the given index.
func (v Value) DeleteIdx(idx int64) bool {
	return C.JS_DeletePropertyInt64(v.ctx.ref, v.ref, C.int64_t(idx), C.int(1)) == 1
}

func (v Value) IsNumber() bool        { return C.JS_IsNumber(v.ref) == 1 }
func (v Value) IsBigInt() bool        { return C.JS_IsBigInt(v.ctx.ref, v.ref) == 1 }
func (v Value) IsBigFloat() bool      { return C.JS_IsBigFloat(v.ref) == 1 }
func (v Value) IsBigDecimal() bool    { return C.JS_IsBigDecimal(v.ref) == 1 }
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

// func (v Value) IsConstructor() bool   { return C.JS_IsConstructor(v.ctx.ref, v.ref) == 1 }
