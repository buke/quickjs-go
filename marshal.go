// marshal.go
package quickjs

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Marshaler is the interface implemented by types that can marshal themselves into a JavaScript value.
type Marshaler interface {
	MarshalJS(ctx *Context) (Value, error)
}

// Unmarshaler is the interface implemented by types that can unmarshal a JavaScript value into themselves.
type Unmarshaler interface {
	UnmarshalJS(ctx *Context, val Value) error
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
//   - slice/array -> JavaScript Array
//   - map -> JavaScript Object
//   - struct -> JavaScript Object
//   - pointer -> recursively marshal the pointed value (nil becomes null)
//
// Struct fields are marshaled using their field names unless a tag is present.
// The "js" and "json" tags are supported. Fields with tag "-" are ignored.
//
// Types implementing the Marshaler interface are marshaled using their MarshalJS method.
func (ctx *Context) Marshal(v interface{}) (Value, error) {
	if v == nil {
		return ctx.Null(), nil
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
func (ctx *Context) Unmarshal(jsVal Value, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("unmarshal target must be a non-nil pointer")
	}
	return ctx.unmarshal(jsVal, rv.Elem())
}

// marshal recursively marshals a Go value to JavaScript
func (ctx *Context) marshal(rv reflect.Value) (Value, error) {
	// Handle interface{} by getting the concrete value
	if rv.Kind() == reflect.Interface && !rv.IsNil() {
		rv = rv.Elem()
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
			return ctx.Null(), nil
		}
		return ctx.marshal(rv.Elem())
	}

	switch rv.Kind() {
	case reflect.Bool:
		return ctx.Bool(rv.Bool()), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return ctx.Int32(int32(rv.Int())), nil

	case reflect.Int64:
		return ctx.Int64(rv.Int()), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return ctx.Uint32(uint32(rv.Uint())), nil

	case reflect.Uint64:
		return ctx.BigUint64(rv.Uint()), nil

	case reflect.Float32, reflect.Float64:
		return ctx.Float64(rv.Float()), nil

	case reflect.String:
		return ctx.String(rv.String()), nil

	case reflect.Slice:
		return ctx.marshalSlice(rv)

	case reflect.Array:
		return ctx.marshalArray(rv)

	case reflect.Map:
		return ctx.marshalMap(rv)

	case reflect.Struct:
		return ctx.marshalStruct(rv)

	default:
		return ctx.Null(), fmt.Errorf("unsupported type: %v", rv.Type())
	}
}

// marshalSlice marshals Go slice to JavaScript Array
func (ctx *Context) marshalSlice(rv reflect.Value) (Value, error) {
	// Handle []byte as ArrayBuffer
	if rv.Type().Elem().Kind() == reflect.Uint8 {
		bytes := rv.Bytes()
		return ctx.ArrayBuffer(bytes), nil
	}

	// Create JavaScript array
	globals := ctx.Globals()
	arrayClass := globals.Get("Array")
	defer arrayClass.Free()

	arr := arrayClass.New()
	for i := 0; i < rv.Len(); i++ {
		elem, err := ctx.marshal(rv.Index(i))
		if err != nil {
			arr.Free()
			return ctx.Null(), err
		}
		arr.SetIdx(int64(i), elem)
		// Do NOT free elem here - ownership transferred to array
	}
	return arr, nil
}

// marshalArray marshals Go array to JavaScript Array
func (ctx *Context) marshalArray(rv reflect.Value) (Value, error) {
	globals := ctx.Globals()
	arrayClass := globals.Get("Array")
	defer arrayClass.Free()

	arr := arrayClass.New()
	for i := 0; i < rv.Len(); i++ {
		elem, err := ctx.marshal(rv.Index(i))
		if err != nil {
			arr.Free()
			return ctx.Null(), err
		}
		arr.SetIdx(int64(i), elem)
		// Do NOT free elem here - ownership transferred to array
	}
	return arr, nil
}

// marshalMap marshals Go map to JavaScript Object
func (ctx *Context) marshalMap(rv reflect.Value) (Value, error) {
	obj := ctx.Object()
	for _, key := range rv.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		val, err := ctx.marshal(rv.MapIndex(key))
		if err != nil {
			obj.Free()
			return ctx.Null(), err
		}
		obj.Set(keyStr, val)
		// Do NOT free val here - ownership transferred to object
	}
	return obj, nil
}

// marshalStruct marshals Go struct to JavaScript Object
func (ctx *Context) marshalStruct(rv reflect.Value) (Value, error) {
	rt := rv.Type()
	obj := ctx.Object()

	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name from tag or use field name
		name := field.Name
		if tag := field.Tag.Get("js"); tag != "" {
			if tag == "-" {
				continue // Skip this field
			}
			name = tag
		} else if tag := field.Tag.Get("json"); tag != "" {
			if tag == "-" {
				continue // Skip this field
			}
			// Parse json tag (handle "name,omitempty" format)
			if idx := strings.Index(tag, ","); idx != -1 {
				name = tag[:idx]
				// TODO: Handle omitempty option
			} else {
				name = tag
			}
		}

		val, err := ctx.marshal(fieldValue)
		if err != nil {
			obj.Free()
			return ctx.Null(), err
		}
		obj.Set(name, val)
		// Do NOT free val here - ownership transferred to object
	}

	return obj, nil
}

// unmarshal recursively unmarshals a JavaScript value to Go
func (ctx *Context) unmarshal(jsVal Value, rv reflect.Value) error {
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
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go bool", jsVal.String())
		}
		rv.SetBool(jsVal.ToBool())

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		if !jsVal.IsNumber() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go int", jsVal.String())
		}
		rv.SetInt(int64(jsVal.ToInt32()))

	case reflect.Int64:
		if !jsVal.IsNumber() && !jsVal.IsBigInt() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go int64", jsVal.String())
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
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go uint", jsVal.String())
		}
		val := jsVal.ToUint32()
		rv.SetUint(uint64(val))

	case reflect.Uint64:
		if !jsVal.IsNumber() && !jsVal.IsBigInt() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go uint64", jsVal.String())
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
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go float", jsVal.String())
		}
		rv.SetFloat(jsVal.ToFloat64())

	case reflect.String:
		if !jsVal.IsString() {
			return fmt.Errorf("cannot unmarshal JavaScript %s into Go string", jsVal.String())
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

// unmarshalSlice unmarshals JavaScript Array to Go slice
func (ctx *Context) unmarshalSlice(jsVal Value, rv reflect.Value) error {
	// Handle ArrayBuffer as []byte
	if rv.Type().Elem().Kind() == reflect.Uint8 && jsVal.IsByteArray() {
		// ToByteArray() should not fail after IsByteArray() check, but we handle the error for robustness
		bytes, err := jsVal.ToByteArray(uint(jsVal.ByteLen()))
		if err != nil {
			return err // This branch is hard to test as it requires internal QuickJS errors
		}
		rv.SetBytes(bytes)
		return nil
	}

	if !jsVal.IsArray() {
		return fmt.Errorf("expected array, got JavaScript %s", jsVal.String())
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
func (ctx *Context) unmarshalArray(jsVal Value, rv reflect.Value) error {
	if !jsVal.IsArray() {
		return fmt.Errorf("expected array, got JavaScript %s", jsVal.String())
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
func (ctx *Context) unmarshalMap(jsVal Value, rv reflect.Value) error {
	if !jsVal.IsObject() {
		return fmt.Errorf("expected object, got JavaScript %s", jsVal.String())
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
func (ctx *Context) unmarshalStruct(jsVal Value, rv reflect.Value) error {
	if !jsVal.IsObject() {
		return fmt.Errorf("expected object, got JavaScript %s", jsVal.String())
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if !field.IsExported() {
			continue
		}

		// Get field name from tag
		name := field.Name
		if tag := field.Tag.Get("js"); tag != "" {
			if tag == "-" {
				continue
			}
			name = tag
		} else if tag := field.Tag.Get("json"); tag != "" {
			if tag == "-" {
				continue
			}
			if idx := strings.Index(tag, ","); idx != -1 {
				name = tag[:idx]
			} else {
				name = tag
			}
		}

		if jsVal.Has(name) {
			prop := jsVal.Get(name)
			defer prop.Free()

			if err := ctx.unmarshal(prop, fieldValue); err != nil {
				return fmt.Errorf("struct field %s: %v", field.Name, err)
			}
		}
	}

	return nil
}

// unmarshalInterface unmarshals JavaScript value to interface{}
func (ctx *Context) unmarshalInterface(jsVal Value) (interface{}, error) {
	if jsVal.IsFunction() || jsVal.IsSymbol() || jsVal.IsException() || jsVal.IsUninitialized() || jsVal.IsPromise() || jsVal.IsConstructor() {
		return nil, fmt.Errorf("unsupported JavaScript type") //Handle unsupported types first
	} else if jsVal.IsNull() || jsVal.IsUndefined() {
		return nil, nil // Return nil for null/undefined
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
		// ToByteArray () should not fail after IsByteArray() check, but we handle the error for robustness
		bytes, err := jsVal.ToByteArray(uint(jsVal.ByteLen()))
		if err != nil {
			return nil, err // This branch is hard to test as it requires internal QuickJS errors
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
	} else if jsVal.IsObject() {
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
	} else {
		// Should not reach here if all types are handled
		return nil, fmt.Errorf("unhandled JavaScript type: %s", jsVal.String()) // This branch is hard to test as it requires other types
	}
}
