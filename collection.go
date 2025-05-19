package quickjs

import (
	"errors"
)

//
//  Array
//  @Description: simply implement the array structure of js

type Array struct {
	arrayValue Value
	ctx        *Context
}

func NewQjsArray(value Value, ctx *Context) *Array {
	return &Array{
		arrayValue: value,
		ctx:        ctx,
	}
}

// Push
//
//	@Description: add one or more elements after the array,returns the new array length
//	@receiver a :
//	@param elements :
//	@return int64
func (a Array) Push(elements ...Value) int64 {
	ret := a.arrayValue.Call("push", elements...)
	//defer ret.Free()
	return ret.ToInt64()
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

func (a Array) Delete(index int64) (bool, error) {
	if index < 0 {
		return false, errors.New("the input index value is a negative number")
	}
	if index >= a.arrayValue.Len() {
		return false, errors.New("index subscript out of range")
	}
	removeList := a.arrayValue.Call("splice", a.ctx.Int64(index), a.ctx.Int64(1))
	defer removeList.Free()
	return removeList.IsArray(), nil
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

//
//  Map
//  @Description: simply implement the map structure of js

type Map struct {
	mapValue Value
	ctx      *Context
}

func NewQjsMap(value Value, ctx *Context) *Map {
	return &Map{
		mapValue: value,
		ctx:      ctx,
	}
}

// Get
//
//	@Description: get the value by key
//	@receiver m :
//	@param key :
//	@return Value
func (m Map) Get(key Value) Value {
	return m.mapValue.Call("get", key)
}

// Put
//
//	@Description:
//	@receiver m :
//	@param key :
//	@param value :
func (m Map) Put(key Value, value Value) {
	m.mapValue.Call("set", key, value).Free()
}

// Delete
//
//	@Description:delete the value of an element by key
//	@receiver m :
//	@param key :
func (m Map) Delete(key Value) {
	m.mapValue.Call("delete", key).Free()
}

// Has
//
//	@Description:determine whether an element exists
//	@receiver m :
//	@param key :
func (m Map) Has(key Value) bool {
	boolValue := m.mapValue.Call("has", key)
	defer boolValue.Free()
	return boolValue.ToBool()
}

// ForEach
//
//	@Description: iterate map
//	@receiver m :
func (m Map) ForEach(forFn func(key Value, value Value)) {
	forEachFn := m.ctx.Function(func(ctx *Context, this Value, args []Value) Value {
		forFn(args[1], args[0])
		return ctx.Null()
	})
	value := m.mapValue.Call("forEach", forEachFn)
	forEachFn.Free()
	defer value.Free()
}

func (m Map) Free() {
	m.mapValue.Free()
}

func (m Map) ToValue() Value {
	return m.mapValue
}

// Call
//
//	@Description: call some internal methods of js
//	@receiver a :
//	@param funcName :
//	@param values :
//	@return Value
func (m Map) Call(funcName string, values []Value) Value {
	return m.mapValue.Call(funcName, values...)
}

type Set struct {
	setValue Value
	ctx      *Context
}

func NewQjsSet(value Value, ctx *Context) *Set {
	return &Set{
		setValue: value,
		ctx:      ctx,
	}
}

// Add
//
//	@Description: add element
//	@receiver s :
//	@param value :
func (s Set) Add(value Value) {
	v := s.setValue.Call("add", value)
	defer v.Free()
}

// Delete
//
//	@Description: add element
//	@receiver s :
//	@param value :
func (s Set) Delete(value Value) {
	v := s.setValue.Call("delete", value)
	defer v.Free()
}

// Has
//
//	@Description: determine whether an element exists in the set
//	@receiver s :
//	@param value :
//	@return bool
func (s Set) Has(value Value) bool {
	v := s.setValue.Call("has", value)
	return v.ToBool()
}

// ForEach
//
//	@Description: iterate set
//	@receiver m :
func (s Set) ForEach(forFn func(value Value)) {
	forEachFn := s.ctx.Function(func(ctx *Context, this Value, args []Value) Value {
		forFn(args[0])
		return ctx.Null()
	})
	value := s.setValue.Call("forEach", forEachFn)
	forEachFn.Free()
	defer value.Free()
}

func (s Set) Free() {
	s.setValue.Free()
}

func (s Set) ToValue() Value {
	return s.setValue
}
