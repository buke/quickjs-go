package quickjs

import "errors"

type Array struct {
	arrayValue Value
}

func NewQjsArray(value Value) Array {
	return Array{
		arrayValue: value,
	}
}

// Push
//
//	@Description: add one or more elements after the array,returns the new array length
//	@receiver a :
//	@param elements :
//	@return int32
func (a Array) Push(elements ...Value) int32 {
	ret := a.arrayValue.Call("push", elements...)
	defer ret.Free()
	return ret.Int32()
}

// Pop
//
//	@Description: Delete the last element of the array and return the value
//	@receiver a :
//	@return *Context
func (a Array) Pop() *Context {
	ret := a.arrayValue.Call("pop")
	return ret.Context()
}

// Unshift
//
//	@Description: Adds one or more elements to the beginning of the array and
//	returns the new length of the modified array.
//	@receiver a :
//	@param elements :
//	@return int32
func (a Array) Unshift(elements []Value) int32 {
	ret := a.arrayValue.Call("unshift", elements...)
	defer ret.Free()
	return ret.Int32()
}

// Shift
//
//	@Description: remove and return the first element of the array
//	@receiver a :
//	@return *Context
func (a Array) Shift() *Context {
	ret := a.arrayValue.Call("shift")
	return ret.Context()
}

// Get
//
//	@Description: get the specific value by subscript
//	@receiver a :
//	@param index :
//	@return Value
func (a Array) Get(index int64) (Value, error) {
	if index < 0 {
		return Value{}, errors.New("the input index value is a negative number")
	}
	if index >= a.arrayValue.Len() {
		return Value{}, errors.New("index subscript out of range")
	}
	return a.arrayValue.GetIdx(index), nil
}

// Set
//
//	@Description:
//	@receiver a :
//	@param index :
//	@param value :
//	@return error
func (a Array) Set(index int64, value Value) error {
	if index < 0 {
		return errors.New("the input index value is a negative number")
	}
	if index >= a.arrayValue.Len() {
		return errors.New("index subscript out of range")
	}
	a.arrayValue.SetIdx(index, value)
	return nil
}

// SetIdx
//
//	@Description:
//	@receiver a :
//	@param index :
//	@param value :
//	@return error
func (a Array) SetIdx(index int64, value Value) {
	a.arrayValue.SetIdx(index, value)
}

func (a Array) Delete(index int64) (bool, error) {
	if index < 0 {
		return false, errors.New("the input index value is a negative number")
	}
	if index >= a.arrayValue.Len() {
		return false, errors.New("index subscript out of range")
	}
	return a.arrayValue.DeleteIdx(index), nil
}

// Call
//
//	@Description: call some internal methods of js
//	@receiver a :
//	@param funcName :
//	@param values :
//	@return Value
func (a Array) Call(funcName string, values []Value) Value {
	return a.arrayValue.Call(funcName, values...)
}

// Len
//
//	@Description: get the length of the array
//	@receiver a :
//	@return int64
func (a Array) Len() int64 {
	return a.arrayValue.Len()
}

// HasIdx
//
//	@Description: Determine whether there is data at the current subscript position
//	@receiver a :
//	@param i :
//	@return bool
func (a Array) HasIdx(i int64) bool {
	return a.arrayValue.HasIdx(i)
}

// ToValue
//
//	@Description: get the value object of qjs
//	@receiver a :
//	@return Value
func (a Array) ToValue() Value {
	return a.arrayValue
}

func (a Array) Free() {
	a.arrayValue.Free()
}
