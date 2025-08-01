package quickjs

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// Marshaler is the interface implemented by types that can marshal themselves into a JavaScript value.
type Marshaler interface {
	MarshalJS(ctx *Context) (*Value, error)
}

// Unmarshaler is the interface implemented by types that can unmarshal a JavaScript value into themselves.
type Unmarshaler interface {
	UnmarshalJS(ctx *Context, val *Value) error
}

// FieldTagInfo contains parsed field tag information
type FieldTagInfo struct {
	Name      string // JavaScript property name
	Skip      bool   // Whether to skip this field
	OmitEmpty bool   // Whether to omit empty values (for serialization)
}

// parseFieldTag parses struct field tags, handling both "js" and "json" tags
// This function provides unified tag parsing logic for both marshal and class reflection
func parseFieldTag(field reflect.StructField) FieldTagInfo {
	// Check "js" tag first (higher priority)
	if tag := field.Tag.Get("js"); tag != "" {
		return parseTagString(tag, field.Name)
	}

	// Fallback to "json" tag
	if tag := field.Tag.Get("json"); tag != "" {
		return parseTagString(tag, field.Name)
	}

	// No tag found, use camelCase field name
	return FieldTagInfo{
		Name:      fieldNameToCamelCase(field.Name),
		Skip:      false,
		OmitEmpty: false,
	}
}

// parseTagString parses tag string in format "name,option1,option2"
func parseTagString(tag, fieldName string) FieldTagInfo {
	if tag == "-" {
		return FieldTagInfo{Skip: true}
	}

	parts := strings.Split(tag, ",")
	info := FieldTagInfo{
		Name: parts[0],
		Skip: false,
	}

	// If name is empty, use camelCase field name
	if info.Name == "" {
		info.Name = fieldNameToCamelCase(fieldName)
	}

	// Parse options
	for _, option := range parts[1:] {
		switch strings.TrimSpace(option) {
		case "omitempty":
			info.OmitEmpty = true
		}
	}

	return info
}

// fieldNameToCamelCase converts field name to camelCase
func fieldNameToCamelCase(fieldName string) string {
	if len(fieldName) == 0 {
		return ""
	}
	return strings.ToLower(fieldName[:1]) + fieldName[1:]
}

// isEmptyValue checks if a value is empty (for omitempty logic)
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return v.IsNil()
	default:
		return false
	}
}

// Marshal returns the JavaScript value encoding of v.
// It traverses the value v recursively and creates corresponding JavaScript values.
//
// Marshal uses the following type mappings:
//   - bool -> JavaScript boolean
//   - int, int8, int16, int32 -> JavaScript number (32-bit)
//   - int64 -> JavaScript number (64-bit)
//   - uint, uint8, uint16, uint32 -> JavaScript number (32-bit unsigned)
//   - uint64 -> JavaScript BigInt
//   - float32, float64 -> JavaScript number
//   - string -> JavaScript string
//   - []byte -> JavaScript ArrayBuffer
//   - []int8 -> JavaScript Int8Array
//   - []int16 -> JavaScript Int16Array
//   - []uint16 -> JavaScript Uint16Array
//   - []int32 -> JavaScript Int32Array
//   - []uint32 -> JavaScript Uint32Array
//   - []float32 -> JavaScript Float32Array
//   - []float64 -> JavaScript Float64Array
//   - []int64 -> JavaScript BigInt64Array
//   - []uint64 -> JavaScript BigUint64Array
//   - slice/array -> JavaScript Array
//   - map -> JavaScript Object
//   - struct -> JavaScript Object
//   - pointer -> recursively marshal the pointed value (nil becomes null)
//
// Struct fields are marshaled using their field names unless a tag is present.
// The "js" and "json" tags are supported. Fields with tag "-" are ignored.
//
// Types implementing the Marshaler interface are marshaled using their MarshalJS method.
func (ctx *Context) Marshal(v interface{}) (*Value, error) {
	if v == nil {
		return ctx.NewNull(), nil
	}
	return ctx.marshal(reflect.ValueOf(v))
}

// Unmarshal parses the JavaScript value and stores the result in the value pointed to by v.
// If v is nil or not a pointer, Unmarshal returns an error.
//
// Unmarshal uses the inverse of the encodings that Marshal uses, with the following additional rules:
//   - JavaScript null/undefined -> Go nil pointer or zero value
//   - JavaScript Array -> Go slice/array
//   - JavaScript Object -> Go map/struct
//   - JavaScript number -> Go numeric types (with appropriate conversion)
//   - JavaScript BigInt -> Go uint64/int64/*big.Int
//   - JavaScript ArrayBuffer -> Go []byte
//   - JavaScript Int8Array -> Go []int8
//   - JavaScript Int16Array -> Go []int16
//   - JavaScript Uint16Array -> Go []uint16
//   - JavaScript Int32Array -> Go []int32
//   - JavaScript Uint32Array -> Go []uint32
//   - JavaScript Float32Array -> Go []float32
//   - JavaScript Float64Array -> Go []float64
//   - JavaScript BigInt64Array -> Go []int64
//   - JavaScript BigUint64Array -> Go []uint64
//
// When unmarshaling into an interface{}, Unmarshal stores one of:
//   - nil for JavaScript null/undefined
//   - bool for JavaScript boolean
//   - int64 for JavaScript integer numbers
//   - float64 for JavaScript floating-point numbers
//   - string for JavaScript string
//   - []interface{} for JavaScript Array
//   - map[string]interface{} for JavaScript Object
//
// Types implementing the Unmarshaler interface are unmarshaled using their UnmarshalJS method.
func (ctx *Context) Unmarshal(jsVal *Value, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("unmarshal target must be a non-nil pointer")
	}
	return ctx.unmarshal(jsVal, rv.Elem())
}

// marshal recursively marshals a Go value to JavaScript
func (ctx *Context) marshal(rv reflect.Value) (*Value, error) {
	// Handle interface{} by getting the concrete value
	if rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return ctx.NewNull(), nil
		}
		rv = rv.Elem()
	}

	// Handle nil kind (invalid value)
	if !rv.IsValid() || rv.Kind() == reflect.Invalid {
		return ctx.NewNull(), nil
	}

	// Check if type implements Marshaler interface
	if rv.CanInterface() {
		if marshaler, ok := rv.Interface().(Marshaler); ok {
			return marshaler.MarshalJS(ctx)
		}
	}

	// Handle pointer types
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ctx.NewNull(), nil
		}
		return ctx.marshal(rv.Elem())
	}

	switch rv.Kind() {
	case reflect.Bool:
		return ctx.NewBool(rv.Bool()), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return ctx.NewInt32(int32(rv.Int())), nil

	case reflect.Int64:
		return ctx.NewInt64(rv.Int()), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return ctx.NewUint32(uint32(rv.Uint())), nil

	case reflect.Uint64:
		return ctx.NewBigUint64(rv.Uint()), nil

	case reflect.Float32, reflect.Float64:
		return ctx.NewFloat64(rv.Float()), nil

	case reflect.String:
		return ctx.NewString(rv.String()), nil

	case reflect.Slice:
		return ctx.marshalSlice(rv)

	case reflect.Array:
		return ctx.marshalArray(rv)

	case reflect.Map:
		return ctx.marshalMap(rv)

	case reflect.Struct:
		return ctx.marshalStruct(rv)

	default:
		return ctx.NewNull(), fmt.Errorf("unsupported type: %v", rv.Type())
	}
}

// marshalSlice marshals Go slice to JavaScript Array or TypedArray
func (ctx *Context) marshalSlice(rv reflect.Value) (*Value, error) {
	elemKind := rv.Type().Elem().Kind()

	switch elemKind {
	case reflect.Uint8:
		// []byte -> ArrayBuffer (maintain existing behavior)
		bytes := rv.Bytes()
		return ctx.NewArrayBuffer(bytes), nil

	case reflect.Int8:
		// []int8 -> Int8Array
		return ctx.marshalInt8Array(rv)

	case reflect.Int16:
		// []int16 -> Int16Array
		return ctx.marshalInt16Array(rv)

	case reflect.Uint16:
		// []uint16 -> Uint16Array
		return ctx.marshalUint16Array(rv)

	case reflect.Int32:
		// []int32 -> Int32Array
		return ctx.marshalInt32Array(rv)

	case reflect.Uint32:
		// []uint32 -> Uint32Array
		return ctx.marshalUint32Array(rv)

	case reflect.Float32:
		// []float32 -> Float32Array
		return ctx.marshalFloat32Array(rv)

	case reflect.Float64:
		// []float64 -> Float64Array
		return ctx.marshalFloat64Array(rv)

	case reflect.Int64:
		// []int64 -> BigInt64Array
		return ctx.marshalBigInt64Array(rv)

	case reflect.Uint64:
		// []uint64 -> BigUint64Array
		return ctx.marshalBigUint64Array(rv)

	default:
		// Other types -> JavaScript Array
		return ctx.marshalGenericArray(rv)
	}
}

// marshalInt8Array converts []int8 to Int8Array
func (ctx *Context) marshalInt8Array(rv reflect.Value) (*Value, error) {
	slice := rv.Interface().([]int8)
	bytes := make([]byte, len(slice))
	for i, v := range slice {
		bytes[i] = byte(v)
	}

	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	globals := ctx.Globals()
	int8ArrayClass := globals.Get("Int8Array")
	defer int8ArrayClass.Free()

	return int8ArrayClass.New(buffer), nil
}

// marshalInt16Array converts []int16 to Int16Array
func (ctx *Context) marshalInt16Array(rv reflect.Value) (*Value, error) {
	slice := rv.Interface().([]int16)
	bytes := int16SliceToBytes(slice)

	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	globals := ctx.Globals()
	int16ArrayClass := globals.Get("Int16Array")
	defer int16ArrayClass.Free()

	return int16ArrayClass.New(buffer), nil
}

// marshalUint16Array converts []uint16 to Uint16Array
func (ctx *Context) marshalUint16Array(rv reflect.Value) (*Value, error) {
	slice := rv.Interface().([]uint16)
	bytes := uint16SliceToBytes(slice)

	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	globals := ctx.Globals()
	uint16ArrayClass := globals.Get("Uint16Array")
	defer uint16ArrayClass.Free()

	return uint16ArrayClass.New(buffer), nil
}

// marshalInt32Array converts []int32 to Int32Array
func (ctx *Context) marshalInt32Array(rv reflect.Value) (*Value, error) {
	slice := rv.Interface().([]int32)
	bytes := int32SliceToBytes(slice)

	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	globals := ctx.Globals()
	int32ArrayClass := globals.Get("Int32Array")
	defer int32ArrayClass.Free()

	return int32ArrayClass.New(buffer), nil
}

// marshalUint32Array converts []uint32 to Uint32Array
func (ctx *Context) marshalUint32Array(rv reflect.Value) (*Value, error) {
	slice := rv.Interface().([]uint32)
	bytes := uint32SliceToBytes(slice)

	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	globals := ctx.Globals()
	uint32ArrayClass := globals.Get("Uint32Array")
	defer uint32ArrayClass.Free()

	return uint32ArrayClass.New(buffer), nil
}

// marshalFloat32Array converts []float32 to Float32Array
func (ctx *Context) marshalFloat32Array(rv reflect.Value) (*Value, error) {
	slice := rv.Interface().([]float32)
	bytes := float32SliceToBytes(slice)

	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	globals := ctx.Globals()
	float32ArrayClass := globals.Get("Float32Array")
	defer float32ArrayClass.Free()

	return float32ArrayClass.New(buffer), nil
}

// marshalFloat64Array converts []float64 to Float64Array
func (ctx *Context) marshalFloat64Array(rv reflect.Value) (*Value, error) {
	slice := rv.Interface().([]float64)
	bytes := float64SliceToBytes(slice)

	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	globals := ctx.Globals()
	float64ArrayClass := globals.Get("Float64Array")
	defer float64ArrayClass.Free()

	return float64ArrayClass.New(buffer), nil
}

// marshalBigInt64Array converts []int64 to BigInt64Array
func (ctx *Context) marshalBigInt64Array(rv reflect.Value) (*Value, error) {
	slice := rv.Interface().([]int64)
	bytes := int64SliceToBytes(slice)

	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	globals := ctx.Globals()
	bigInt64ArrayClass := globals.Get("BigInt64Array")
	defer bigInt64ArrayClass.Free()

	return bigInt64ArrayClass.New(buffer), nil
}

// marshalBigUint64Array converts []uint64 to BigUint64Array
func (ctx *Context) marshalBigUint64Array(rv reflect.Value) (*Value, error) {
	slice := rv.Interface().([]uint64)
	bytes := uint64SliceToBytes(slice)

	buffer := ctx.NewArrayBuffer(bytes)
	defer buffer.Free()

	globals := ctx.Globals()
	bigUint64ArrayClass := globals.Get("BigUint64Array")
	defer bigUint64ArrayClass.Free()

	return bigUint64ArrayClass.New(buffer), nil
}

// marshalGenericArray marshals Go slice to JavaScript Array (fallback)
func (ctx *Context) marshalGenericArray(rv reflect.Value) (*Value, error) {
	globals := ctx.Globals()
	arrayClass := globals.Get("Array")
	defer arrayClass.Free()

	arr := arrayClass.New()
	for i := 0; i < rv.Len(); i++ {
		elem, err := ctx.marshal(rv.Index(i))
		if err != nil {
			arr.Free()
			return ctx.NewNull(), err
		}
		arr.SetIdx(int64(i), elem)
		// Do NOT free elem here - ownership transferred to array
	}
	return arr, nil
}

// Byte conversion helper functions

// int16SliceToBytes converts []int16 to []byte using little-endian
func int16SliceToBytes(slice []int16) []byte {
	if len(slice) == 0 {
		return nil
	}

	bytes := make([]byte, len(slice)*2)
	for i, v := range slice {
		binary.LittleEndian.PutUint16(bytes[i*2:], uint16(v))
	}
	return bytes
}

// uint16SliceToBytes converts []uint16 to []byte using little-endian
func uint16SliceToBytes(slice []uint16) []byte {
	if len(slice) == 0 {
		return nil
	}

	bytes := make([]byte, len(slice)*2)
	for i, v := range slice {
		binary.LittleEndian.PutUint16(bytes[i*2:], v)
	}
	return bytes
}

// int32SliceToBytes converts []int32 to []byte using little-endian
func int32SliceToBytes(slice []int32) []byte {
	if len(slice) == 0 {
		return nil
	}

	bytes := make([]byte, len(slice)*4)
	for i, v := range slice {
		binary.LittleEndian.PutUint32(bytes[i*4:], uint32(v))
	}
	return bytes
}

// uint32SliceToBytes converts []uint32 to []byte using little-endian
func uint32SliceToBytes(slice []uint32) []byte {
	if len(slice) == 0 {
		return nil
	}

	bytes := make([]byte, len(slice)*4)
	for i, v := range slice {
		binary.LittleEndian.PutUint32(bytes[i*4:], v)
	}
	return bytes
}

// float32SliceToBytes converts []float32 to []byte using little-endian
func float32SliceToBytes(slice []float32) []byte {
	if len(slice) == 0 {
		return nil
	}

	bytes := make([]byte, len(slice)*4)
	for i, v := range slice {
		binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(v))
	}
	return bytes
}

// float64SliceToBytes converts []float64 to []byte using little-endian
func float64SliceToBytes(slice []float64) []byte {
	if len(slice) == 0 {
		return nil
	}

	bytes := make([]byte, len(slice)*8)
	for i, v := range slice {
		binary.LittleEndian.PutUint64(bytes[i*8:], math.Float64bits(v))
	}
	return bytes
}

// int64SliceToBytes converts []int64 to []byte using little-endian
func int64SliceToBytes(slice []int64) []byte {
	if len(slice) == 0 {
		return nil
	}

	bytes := make([]byte, len(slice)*8)
	for i, v := range slice {
		binary.LittleEndian.PutUint64(bytes[i*8:], uint64(v))
	}
	return bytes
}

// uint64SliceToBytes converts []uint64 to []byte using little-endian
func uint64SliceToBytes(slice []uint64) []byte {
	if len(slice) == 0 {
		return nil
	}

	bytes := make([]byte, len(slice)*8)
	for i, v := range slice {
		binary.LittleEndian.PutUint64(bytes[i*8:], v)
	}
	return bytes
}

// marshalArray marshals Go array to JavaScript Array
func (ctx *Context) marshalArray(rv reflect.Value) (*Value, error) {
	globals := ctx.Globals()
	arrayClass := globals.Get("Array")
	defer arrayClass.Free()

	arr := arrayClass.New()
	for i := 0; i < rv.Len(); i++ {
		elem, err := ctx.marshal(rv.Index(i))
		if err != nil {
			arr.Free()
			return ctx.NewNull(), err
		}
		arr.SetIdx(int64(i), elem)
		// Do NOT free elem here - ownership transferred to array
	}
	return arr, nil
}

// marshalMap marshals Go map to JavaScript Object
func (ctx *Context) marshalMap(rv reflect.Value) (*Value, error) {
	obj := ctx.NewObject()
	for _, key := range rv.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		val, err := ctx.marshal(rv.MapIndex(key))
		if err != nil {
			obj.Free()
			return ctx.NewNull(), err
		}
		obj.Set(keyStr, val)
		// Do NOT free val here - ownership transferred to object
	}
	return obj, nil
}

// marshalStruct marshals Go struct to JavaScript Object
func (ctx *Context) marshalStruct(rv reflect.Value) (*Value, error) {
	rt := rv.Type()
	obj := ctx.NewObject()

	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Use unified tag parsing
		tagInfo := parseFieldTag(field)
		if tagInfo.Skip {
			continue
		}

		// Handle omitempty option
		if tagInfo.OmitEmpty && isEmptyValue(fieldValue) {
			continue
		}

		val, err := ctx.marshal(fieldValue)
		if err != nil {
			obj.Free()
			return ctx.NewNull(), err
		}
		obj.Set(tagInfo.Name, val)
		// Do NOT free val here - ownership transferred to object
	}

	return obj, nil
}

// unmarshal recursively unmarshals a JavaScript value to Go
func (ctx *Context) unmarshal(jsVal *Value, rv reflect.Value) error {
	// Check if type implements Unmarshaler interface
	if rv.CanAddr() {
		if unmarshaler, ok := rv.Addr().Interface().(Unmarshaler); ok {
			return unmarshaler.UnmarshalJS(ctx, jsVal)
		}
	}

	// Handle pointer types
	if rv.Kind() == reflect.Ptr {
		if jsVal.IsNull() || jsVal.IsUndefined() {
			rv.Set(reflect.Zero(rv.Type()))
			return nil
		}

		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		return ctx.unmarshal(jsVal, rv.Elem())
	}

	switch rv.Kind() {
	case reflect.Bool:
		if !jsVal.IsBool() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go bool", jsVal.ToString())
		}
		rv.SetBool(jsVal.ToBool())

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		if !jsVal.IsNumber() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go int", jsVal.ToString())
		}
		rv.SetInt(int64(jsVal.ToInt32()))

	case reflect.Int64:
		if !jsVal.IsNumber() && !jsVal.IsBigInt() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go int64", jsVal.ToString())
		}
		if jsVal.IsBigInt() {
			bigInt := jsVal.ToBigInt()
			if bigInt != nil && bigInt.IsInt64() {
				rv.SetInt(bigInt.Int64())
			} else {
				return fmt.Errorf("BigInt value out of range for int64")
			}
		} else {
			rv.SetInt(jsVal.ToInt64())
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		if !jsVal.IsNumber() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go uint", jsVal.ToString())
		}
		val := jsVal.ToUint32()
		rv.SetUint(uint64(val))

	case reflect.Uint64:
		if !jsVal.IsNumber() && !jsVal.IsBigInt() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go uint64", jsVal.ToString())
		}
		if jsVal.IsBigInt() {
			bigInt := jsVal.ToBigInt()
			if bigInt != nil && bigInt.IsUint64() {
				rv.SetUint(bigInt.Uint64())
			} else {
				return fmt.Errorf("BigInt value out of range for uint64")
			}
		} else {
			val := jsVal.ToInt64()
			if val < 0 {
				return fmt.Errorf("cannot unmarshal negative number into Go uint64")
			}
			rv.SetUint(uint64(val))
		}

	case reflect.Float32, reflect.Float64:
		if !jsVal.IsNumber() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go float", jsVal.ToString())
		}
		rv.SetFloat(jsVal.ToFloat64())

	case reflect.String:
		if !jsVal.IsString() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go string", jsVal.ToString())
		}
		rv.SetString(jsVal.ToString())

	case reflect.Slice:
		return ctx.unmarshalSlice(jsVal, rv)

	case reflect.Array:
		return ctx.unmarshalArray(jsVal, rv)

	case reflect.Map:
		return ctx.unmarshalMap(jsVal, rv)

	case reflect.Struct:
		return ctx.unmarshalStruct(jsVal, rv)

	case reflect.Interface:
		// Handle interface{} by determining the best Go type
		val, err := ctx.unmarshalInterface(jsVal)
		if err != nil {
			return err
		}
		// Handle nil values properly
		if val == nil {
			rv.Set(reflect.Zero(rv.Type()))
		} else {
			rv.Set(reflect.ValueOf(val))
		}

	default:
		return fmt.Errorf("unsupported type: %v", rv.Type())
	}

	return nil
}

// unmarshalSlice unmarshals JavaScript Array/TypedArray to Go slice
func (ctx *Context) unmarshalSlice(jsVal *Value, rv reflect.Value) error {
	elemKind := rv.Type().Elem().Kind()

	// Check for corresponding TypedArray types first
	switch elemKind {
	case reflect.Uint8:
		// Handle ArrayBuffer as []byte (priority)
		if jsVal.IsByteArray() {
			bytes, err := jsVal.ToByteArray(uint(jsVal.ByteLen()))
			if err != nil {
				return err
			}
			rv.SetBytes(bytes)
			return nil
		}
		// Handle Uint8Array/Uint8ClampedArray as []byte
		if jsVal.IsUint8Array() || jsVal.IsUint8ClampedArray() {
			data, err := jsVal.ToUint8Array()
			if err != nil {
				return err
			}
			rv.SetBytes(data)
			return nil
		}

	case reflect.Int8:
		if jsVal.IsInt8Array() {
			data, err := jsVal.ToInt8Array()
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(data))
			return nil
		}

	case reflect.Int16:
		if jsVal.IsInt16Array() {
			data, err := jsVal.ToInt16Array()
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(data))
			return nil
		}

	case reflect.Uint16:
		if jsVal.IsUint16Array() {
			data, err := jsVal.ToUint16Array()
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(data))
			return nil
		}

	case reflect.Int32:
		if jsVal.IsInt32Array() {
			data, err := jsVal.ToInt32Array()
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(data))
			return nil
		}

	case reflect.Uint32:
		if jsVal.IsUint32Array() {
			data, err := jsVal.ToUint32Array()
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(data))
			return nil
		}

	case reflect.Float32:
		if jsVal.IsFloat32Array() {
			data, err := jsVal.ToFloat32Array()
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(data))
			return nil
		}

	case reflect.Float64:
		if jsVal.IsFloat64Array() {
			data, err := jsVal.ToFloat64Array()
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(data))
			return nil
		}

	case reflect.Int64:
		if jsVal.IsBigInt64Array() {
			data, err := jsVal.ToBigInt64Array()
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(data))
			return nil
		}

	case reflect.Uint64:
		if jsVal.IsBigUint64Array() {
			data, err := jsVal.ToBigUint64Array()
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(data))
			return nil
		}
	}

	// If not a corresponding TypedArray, handle as regular array
	if !jsVal.IsArray() {
		return fmt.Errorf("expected array, got JavaScript %s", jsVal.ToString())
	}

	length := jsVal.Len()
	slice := reflect.MakeSlice(rv.Type(), int(length), int(length))

	for i := int64(0); i < length; i++ {
		elem := jsVal.GetIdx(i)
		defer elem.Free()

		if err := ctx.unmarshal(elem, slice.Index(int(i))); err != nil {
			return fmt.Errorf("array element %d: %v", i, err)
		}
	}

	rv.Set(slice)
	return nil
}

// unmarshalArray unmarshals JavaScript Array to Go array
func (ctx *Context) unmarshalArray(jsVal *Value, rv reflect.Value) error {
	if !jsVal.IsArray() {
		return fmt.Errorf("expected array, got JavaScript %s", jsVal.ToString())
	}

	length := jsVal.Len()
	arrayLen := rv.Len()

	// Use the smaller of the two lengths to avoid index out of bounds
	maxLen := int(length)
	if arrayLen < maxLen {
		maxLen = arrayLen
	}

	for i := 0; i < maxLen; i++ {
		elem := jsVal.GetIdx(int64(i))
		defer elem.Free()

		if err := ctx.unmarshal(elem, rv.Index(i)); err != nil {
			return fmt.Errorf("array element %d: %v", i, err)
		}
	}

	return nil
}

// unmarshalMap unmarshals JavaScript Object to Go map
func (ctx *Context) unmarshalMap(jsVal *Value, rv reflect.Value) error {
	if !jsVal.IsObject() {
		return fmt.Errorf("expected object, got JavaScript %s", jsVal.ToString())
	}

	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}

	props, err := jsVal.PropertyNames()
	if err != nil {
		return err
	}

	keyType := rv.Type().Key()
	valueType := rv.Type().Elem()

	for _, prop := range props {
		val := jsVal.Get(prop)
		defer val.Free()

		// Convert property name to the map's key type
		keyVal := reflect.New(keyType).Elem()
		switch keyType.Kind() {
		case reflect.String:
			keyVal.SetString(prop)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if intVal, err := strconv.ParseInt(prop, 10, 64); err == nil {
				keyVal.SetInt(intVal)
			} else {
				continue // Skip non-numeric keys for numeric key types
			}
		default:
			return fmt.Errorf("unsupported map key type: %v", keyType)
		}

		// Unmarshal the value
		valueVal := reflect.New(valueType).Elem()
		if err := ctx.unmarshal(val, valueVal); err != nil {
			return fmt.Errorf("map value for key %s: %v", prop, err)
		}

		rv.SetMapIndex(keyVal, valueVal)
	}

	return nil
}

// unmarshalStruct unmarshals JavaScript Object to Go struct
func (ctx *Context) unmarshalStruct(jsVal *Value, rv reflect.Value) error {
	if !jsVal.IsObject() {
		return fmt.Errorf("expected object, got JavaScript %s", jsVal.ToString())
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if !field.IsExported() {
			continue
		}

		// Use unified tag parsing
		tagInfo := parseFieldTag(field)
		if tagInfo.Skip {
			continue
		}

		if jsVal.Has(tagInfo.Name) {
			prop := jsVal.Get(tagInfo.Name)
			defer prop.Free()

			if err := ctx.unmarshal(prop, fieldValue); err != nil {
				return fmt.Errorf("struct field %s: %v", field.Name, err)
			}
		}
	}

	return nil
}

// unmarshalInterface unmarshals JavaScript value to interface{}
func (ctx *Context) unmarshalInterface(jsVal *Value) (interface{}, error) {
	if jsVal.IsFunction() || jsVal.IsSymbol() || jsVal.IsException() || jsVal.IsUninitialized() || jsVal.IsPromise() || jsVal.IsConstructor() {
		return nil, fmt.Errorf("unsupported JavaScript type")
	} else if jsVal.IsNull() || jsVal.IsUndefined() {
		return nil, nil
	} else if jsVal.IsBool() {
		return jsVal.ToBool(), nil
	} else if jsVal.IsString() {
		return jsVal.ToString(), nil
	} else if jsVal.IsNumber() {
		// Try to determine if it's an integer or float
		f := jsVal.ToFloat64()
		if f == float64(int64(f)) {
			return int64(f), nil
		}
		return f, nil
	} else if jsVal.IsBigInt() {
		return jsVal.ToBigInt(), nil
	} else if jsVal.IsByteArray() {
		bytes, err := jsVal.ToByteArray(uint(jsVal.ByteLen()))
		if err != nil {
			return nil, err
		}
		return bytes, nil
	} else if jsVal.IsArray() {
		length := jsVal.Len()
		slice := make([]interface{}, length)
		for i := int64(0); i < length; i++ {
			elem := jsVal.GetIdx(i)
			defer elem.Free()

			val, err := ctx.unmarshalInterface(elem)
			if err != nil {
				return nil, err
			}
			slice[i] = val
		}
		return slice, nil
	} else {
		// Default case: treat as object (covers IsObject() and any edge cases)
		result := make(map[string]interface{})

		props, err := jsVal.PropertyNames()
		if err != nil {
			return nil, err
		}

		for _, prop := range props {
			val := jsVal.Get(prop)
			defer val.Free()

			goVal, err := ctx.unmarshalInterface(val)
			if err != nil {
				return nil, err
			}
			result[prop] = goVal
		}
		return result, nil
	}
}
