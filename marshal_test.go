package quickjs_test

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// Custom types for testing marshal/unmarshal interfaces
type CustomMarshalType struct {
	Value string
}

func (c CustomMarshalType) MarshalJS(ctx *quickjs.Context) (quickjs.Value, error) {
	return ctx.String("custom:" + c.Value), nil
}

type CustomUnmarshalType struct {
	Value string
}

func (c *CustomUnmarshalType) UnmarshalJS(ctx *quickjs.Context, val quickjs.Value) error {
	if val.IsString() {
		str := val.ToString()
		if len(str) > 7 && str[:7] == "custom:" {
			c.Value = str[7:]
		} else {
			c.Value = str
		}
	}
	return nil
}

type ErrorMarshalType struct{}

func (e ErrorMarshalType) MarshalJS(ctx *quickjs.Context) (quickjs.Value, error) {
	return ctx.Null(), errors.New("marshal error")
}

type ErrorUnmarshalType struct{}

func (e *ErrorUnmarshalType) UnmarshalJS(ctx *quickjs.Context, val quickjs.Value) error {
	return errors.New("unmarshal error")
}

// Test struct with various field types and tags
type TestStruct struct {
	ExportedField    string            `js:"exported"`
	unexportedField  string            // Should be skipped
	JSONTagField     int               `json:"json_field"`
	JSTagField       bool              `js:"js_field"`
	SkippedJSField   string            `js:"-"`
	SkippedJSONField string            `json:"-"`
	CommaTag         string            `json:"comma_tag,omitempty"`
	NoTagField       float64           // Should use field name
	NestedStruct     NestedStruct      `js:"nested"`
	MapField         map[string]string `js:"map_field"`
	SliceField       []int             `js:"slice_field"`
}

type NestedStruct struct {
	Name  string `js:"name"`
	Value int    `js:"value"`
}

// Time wrapper for custom marshal/unmarshal
type TimeWrapper struct {
	time.Time
}

func (t TimeWrapper) MarshalJS(ctx *quickjs.Context) (quickjs.Value, error) {
	return ctx.String(t.Format(time.RFC3339)), nil
}

func (t *TimeWrapper) UnmarshalJS(ctx *quickjs.Context, val quickjs.Value) error {
	if val.IsString() {
		parsed, err := time.Parse(time.RFC3339, val.ToString())
		if err != nil {
			return err
		}
		t.Time = parsed
	}
	return nil
}

func TestMarshalBasicTypes(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"Nil", nil, nil},
		{"Bool", true, true},
		{"Int", int(-42), int(-42)},
		{"Int8", int8(-8), int8(-8)},
		{"Int16", int16(-16), int16(-16)},
		{"Int32", int32(-32), int32(-32)},
		{"Int64", int64(64), int64(64)},
		{"Uint", uint(42), uint(42)},
		{"Uint8", uint8(8), uint8(8)},
		{"Uint16", uint16(16), uint16(16)},
		{"Uint32", uint32(32), uint32(32)},
		{"Uint64", uint64(1<<63 - 1), uint64(1<<63 - 1)},
		{"Float32", float32(3.14), float32(3.14)},
		{"Float64", float64(2.718), float64(2.718)},
		{"String", "hello world", "hello world"},
		{"EmptyString", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsVal, err := ctx.Marshal(tt.input)
			require.NoError(t, err)
			defer jsVal.Free()

			if tt.input == nil {
				require.True(t, jsVal.IsNull())
				return
			}

			// Test round-trip
			target := reflect.New(reflect.TypeOf(tt.input)).Interface()
			err = ctx.Unmarshal(jsVal, target)
			require.NoError(t, err)
			result := reflect.ValueOf(target).Elem().Interface()

			switch expected := tt.expected.(type) {
			case float32:
				require.InDelta(t, expected, result.(float32), 0.0001)
			case float64:
				require.InDelta(t, expected, result.(float64), 0.0001)
			default:
				require.Equal(t, expected, result)
			}
		})
	}

	// Test interface{} types to ensure rv.Elem() coverage
	t.Run("InterfaceTypes", func(t *testing.T) {
		var nilInterface interface{} = nil
		jsVal, err := ctx.Marshal(nilInterface)
		require.NoError(t, err)
		defer jsVal.Free()
		require.True(t, jsVal.IsNull())

		var iface interface{} = "test string"
		jsVal2, err := ctx.Marshal(iface)
		require.NoError(t, err)
		defer jsVal2.Free()
		require.True(t, jsVal2.IsString())
		require.Equal(t, "test string", jsVal2.ToString())
	})
}

func TestTypedArrays(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Helper function for TypedArray round-trip tests
	testTypedArrayRoundTrip := func(t *testing.T, name string, data interface{}, checkFunc func(quickjs.Value) bool) {
		t.Run(name, func(t *testing.T) {
			jsVal, err := ctx.Marshal(data)
			require.NoError(t, err)
			defer jsVal.Free()

			require.True(t, checkFunc(jsVal), "Expected specific TypedArray type")

			// Test round-trip
			targetType := reflect.TypeOf(data)
			result := reflect.New(targetType).Interface()
			err = ctx.Unmarshal(jsVal, result)
			require.NoError(t, err)

			resultVal := reflect.ValueOf(result).Elem().Interface()
			switch expected := data.(type) {
			case []float32:
				resultSlice := resultVal.([]float32)
				require.Equal(t, len(expected), len(resultSlice))
				for i, exp := range expected {
					require.InDelta(t, exp, resultSlice[i], 0.0001)
				}
			case []float64:
				resultSlice := resultVal.([]float64)
				require.Equal(t, len(expected), len(resultSlice))
				for i, exp := range expected {
					require.InDelta(t, exp, resultSlice[i], 1e-10)
				}
			default:
				require.Equal(t, expected, resultVal)
			}
		})
	}

	// Test all TypedArray types
	testTypedArrayRoundTrip(t, "Int8Array", []int8{-128, -1, 0, 1, 127}, func(v quickjs.Value) bool { return v.IsInt8Array() })
	testTypedArrayRoundTrip(t, "Int16Array", []int16{-32768, -1, 0, 1, 32767}, func(v quickjs.Value) bool { return v.IsInt16Array() })
	testTypedArrayRoundTrip(t, "Uint16Array", []uint16{0, 1, 32768, 65535}, func(v quickjs.Value) bool { return v.IsUint16Array() })
	testTypedArrayRoundTrip(t, "Int32Array", []int32{-2147483648, -1, 0, 1, 2147483647}, func(v quickjs.Value) bool { return v.IsInt32Array() })
	testTypedArrayRoundTrip(t, "Uint32Array", []uint32{0, 1, 2147483648, 4294967295}, func(v quickjs.Value) bool { return v.IsUint32Array() })
	testTypedArrayRoundTrip(t, "Float32Array", []float32{-3.14, 0.0, 2.718, float32(1 << 20)}, func(v quickjs.Value) bool { return v.IsFloat32Array() })
	testTypedArrayRoundTrip(t, "Float64Array", []float64{-3.141592653589793, 0.0, 2.718281828459045, 1e10}, func(v quickjs.Value) bool { return v.IsFloat64Array() })
	testTypedArrayRoundTrip(t, "BigInt64Array", []int64{-9223372036854775808, -1, 0, 1, 9223372036854775807}, func(v quickjs.Value) bool { return v.IsBigInt64Array() })
	testTypedArrayRoundTrip(t, "BigUint64Array", []uint64{0, 1, 9223372036854775808, 18446744073709551615}, func(v quickjs.Value) bool { return v.IsBigUint64Array() })

	// Test []byte -> ArrayBuffer (special case)
	t.Run("ByteSliceToArrayBuffer", func(t *testing.T) {
		data := []byte{1, 2, 3, 4, 5}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsByteArray())
		require.Equal(t, int64(len(data)), jsVal.ByteLen())

		var result []byte
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	// Test empty TypedArrays
	t.Run("EmptyTypedArrays", func(t *testing.T) {
		emptyTests := []struct {
			name  string
			data  interface{}
			check func(quickjs.Value) bool
		}{
			{"EmptyInt8Array", []int8{}, func(v quickjs.Value) bool { return v.IsInt8Array() }},
			{"EmptyInt16Array", []int16{}, func(v quickjs.Value) bool { return v.IsInt16Array() }},
			{"EmptyUint16Array", []uint16{}, func(v quickjs.Value) bool { return v.IsUint16Array() }},
			{"EmptyInt32Array", []int32{}, func(v quickjs.Value) bool { return v.IsInt32Array() }},
			{"EmptyUint32Array", []uint32{}, func(v quickjs.Value) bool { return v.IsUint32Array() }},
			{"EmptyFloat32Array", []float32{}, func(v quickjs.Value) bool { return v.IsFloat32Array() }},
			{"EmptyFloat64Array", []float64{}, func(v quickjs.Value) bool { return v.IsFloat64Array() }},
			{"EmptyBigInt64Array", []int64{}, func(v quickjs.Value) bool { return v.IsBigInt64Array() }},
			{"EmptyBigUint64Array", []uint64{}, func(v quickjs.Value) bool { return v.IsBigUint64Array() }},
		}

		for _, tt := range emptyTests {
			t.Run(tt.name, func(t *testing.T) {
				jsVal, err := ctx.Marshal(tt.data)
				require.NoError(t, err)
				defer jsVal.Free()

				require.True(t, tt.check(jsVal), "Expected TypedArray type")

				targetType := reflect.TypeOf(tt.data)
				result := reflect.New(targetType).Interface()
				err = ctx.Unmarshal(jsVal, result)
				require.NoError(t, err)

				rv := reflect.ValueOf(result).Elem()
				require.Equal(t, 0, rv.Len())
			})
		}
	})

	// Test JavaScript TypedArrays to Go slices
	t.Run("JavaScriptTypedArrays", func(t *testing.T) {
		jsTests := []struct {
			name     string
			jsCode   string
			target   interface{}
			expected interface{}
		}{
			{"JSUint8Array", `new Uint8Array([0, 128, 255])`, &[]uint8{}, []uint8{0, 128, 255}},
			{"JSUint8ClampedArray", `new Uint8ClampedArray([0, 128, 255])`, &[]uint8{}, []uint8{0, 128, 255}},
			{"JSBigInt64Array", `new BigInt64Array([BigInt("-9223372036854775808"), BigInt("0"), BigInt("9223372036854775807")])`, &[]int64{}, []int64{-9223372036854775808, 0, 9223372036854775807}},
			{"JSBigUint64Array", `new BigUint64Array([BigInt("0"), BigInt("9223372036854775808"), BigInt("18446744073709551615")])`, &[]uint64{}, []uint64{0, 9223372036854775808, 18446744073709551615}},
		}

		for _, tt := range jsTests {
			t.Run(tt.name, func(t *testing.T) {
				jsVal, err := ctx.Eval(tt.jsCode)
				require.NoError(t, err)
				defer jsVal.Free()

				err = ctx.Unmarshal(jsVal, tt.target)
				require.NoError(t, err)

				result := reflect.ValueOf(tt.target).Elem().Interface()
				require.Equal(t, tt.expected, result)
			})
		}
	})
}

func TestTypedArrayErrors(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Helper function to create fake TypedArray objects
	createFakeTypedArray := func(typeName string) quickjs.Value {
		jsCode := fmt.Sprintf(`
            var corrupted = Object.create(%s.prototype);
            Object.defineProperty(corrupted, 'constructor', {
                value: %s,
                writable: true,
                enumerable: false,
                configurable: true
            });
            corrupted;
        `, typeName, typeName)
		val, _ := ctx.Eval(jsCode)
		return val
	}

	// Test ToXXXArray error branches in unmarshalSlice
	errorTests := []struct {
		name     string
		target   interface{}
		typeName string
	}{
		{"FakeInt8Array", &[]int8{}, "Int8Array"},
		{"FakeUint8Array", &[]uint8{}, "Uint8Array"},
		{"FakeInt16Array", &[]int16{}, "Int16Array"},
		{"FakeUint16Array", &[]uint16{}, "Uint16Array"},
		{"FakeInt32Array", &[]int32{}, "Int32Array"},
		{"FakeUint32Array", &[]uint32{}, "Uint32Array"},
		{"FakeFloat32Array", &[]float32{}, "Float32Array"},
		{"FakeFloat64Array", &[]float64{}, "Float64Array"},
		{"FakeBigInt64Array", &[]int64{}, "BigInt64Array"},
		{"FakeBigUint64Array", &[]uint64{}, "BigUint64Array"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			fakeTypedArray := createFakeTypedArray(tt.typeName)
			defer fakeTypedArray.Free()

			err := ctx.Unmarshal(fakeTypedArray, tt.target)
			if err != nil {
				t.Logf("✓ Covered ToXXXArray error branch for %s: %v", tt.name, err)
			}
		})
	}

	// Test specific error cases for byte arrays
	t.Run("ByteArrayErrors", func(t *testing.T) {
		fakeArrayBuffer, err := ctx.Eval(`
            var fakeBuffer = {
                constructor: ArrayBuffer,
                byteLength: 10
            };
            Object.setPrototypeOf(fakeBuffer, ArrayBuffer.prototype);
            fakeBuffer;
        `)
		require.NoError(t, err)
		defer fakeArrayBuffer.Free()

		var result []byte
		err = ctx.Unmarshal(fakeArrayBuffer, &result)
		if err != nil {
			t.Logf("✓ Covered ToByteArray error branch: %v", err)
		}
	})

	// Test fallback to regular array
	t.Run("FallbackToRegularArray", func(t *testing.T) {
		jsVal, err := ctx.Eval(`[1, 2, 3]`)
		require.NoError(t, err)
		defer jsVal.Free()

		var result []int8
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, []int8{1, 2, 3}, result)
	})
}

func TestComplexTypes(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("Slices", func(t *testing.T) {
		// Test generic slice (fallback to regular array)
		data := []int{1, 2, 3}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsArray())
		require.Equal(t, int64(len(data)), jsVal.Len())

		var result []int
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)

		// Test slice with interface{} elements
		slice := []interface{}{"hello", 123}
		jsVal2, err := ctx.Marshal(slice)
		require.NoError(t, err)
		defer jsVal2.Free()
		require.True(t, jsVal2.IsArray())
		require.Equal(t, int64(2), jsVal2.Len())
	})

	t.Run("Arrays", func(t *testing.T) {
		// Test array marshal/unmarshal to cover marshalArray
		data := [3]int{1, 2, 3}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsArray())
		require.Equal(t, int64(3), jsVal.Len())

		var result [3]int
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)

		// Test array size edge cases
		tests := []struct {
			name     string
			jsCode   string
			target   interface{}
			expected interface{}
		}{
			{"SameSize", `[1, 2, 3]`, &[3]int{}, [3]int{1, 2, 3}},
			{"JSLarger", `[1, 2, 3, 4, 5]`, &[3]int{}, [3]int{1, 2, 3}},
			{"GoLarger", `[1, 2]`, &[5]int{}, [5]int{1, 2, 0, 0, 0}},
			{"Empty", `[]`, &[3]int{}, [3]int{0, 0, 0}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				jsVal, err := ctx.Eval(tt.jsCode)
				require.NoError(t, err)
				defer jsVal.Free()

				err = ctx.Unmarshal(jsVal, tt.target)
				require.NoError(t, err)

				result := reflect.ValueOf(tt.target).Elem().Interface()
				require.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("Maps", func(t *testing.T) {
		// String key map
		stringMap := map[string]string{"key1": "value1", "key2": "value2"}
		jsVal, err := ctx.Marshal(stringMap)
		require.NoError(t, err)
		defer jsVal.Free()

		var result map[string]string
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, stringMap, result)

		// Int key map
		intMap := map[int]string{1: "one", 2: "two"}
		jsVal2, err := ctx.Marshal(intMap)
		require.NoError(t, err)
		defer jsVal2.Free()

		var result2 map[int]string
		err = ctx.Unmarshal(jsVal2, &result2)
		require.NoError(t, err)
		require.Equal(t, intMap, result2)

		// Nil map
		var nilMap map[string]string
		jsVal3, err := ctx.Marshal(nilMap)
		require.NoError(t, err)
		defer jsVal3.Free()

		var result3 map[string]string
		err = ctx.Unmarshal(jsVal3, &result3)
		require.NoError(t, err)
		require.NotNil(t, result3)

		// Mixed key types (numeric string to int key)
		jsVal4, err := ctx.Eval(`({abc: "value", "123": "numeric"})`)
		require.NoError(t, err)
		defer jsVal4.Free()

		var result4 map[int]string
		err = ctx.Unmarshal(jsVal4, &result4)
		require.NoError(t, err)
		require.Equal(t, map[int]string{123: "numeric"}, result4)
	})

	t.Run("Pointers", func(t *testing.T) {
		// Non-nil pointer
		value := "test"
		ptr := &value
		jsVal, err := ctx.Marshal(ptr)
		require.NoError(t, err)
		defer jsVal.Free()

		var result *string
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, value, *result)

		// Nil pointer
		var nilPtr *string
		jsVal2, err := ctx.Marshal(nilPtr)
		require.NoError(t, err)
		defer jsVal2.Free()
		require.True(t, jsVal2.IsNull())

		var result2 *string
		err = ctx.Unmarshal(jsVal2, &result2)
		require.NoError(t, err)
		require.Nil(t, result2)
	})
}

func TestStructsAndCustomTypes(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("Structs", func(t *testing.T) {
		data := TestStruct{
			ExportedField:    "exported",
			unexportedField:  "should be ignored",
			JSONTagField:     42,
			JSTagField:       true,
			SkippedJSField:   "should be skipped",
			SkippedJSONField: "should be skipped",
			CommaTag:         "comma",
			NoTagField:       3.14,
			NestedStruct:     NestedStruct{Name: "nested", Value: 123},
			MapField:         map[string]string{"key": "value"},
			SliceField:       []int{1, 2, 3},
		}

		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsObject())
		require.True(t, jsVal.Has("exported"))
		require.True(t, jsVal.Has("json_field"))
		require.True(t, jsVal.Has("js_field"))
		require.False(t, jsVal.Has("SkippedJSField"))
		require.False(t, jsVal.Has("SkippedJSONField"))

		var result TestStruct
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data.ExportedField, result.ExportedField)
		require.Equal(t, data.JSONTagField, result.JSONTagField)
		require.Equal(t, data.JSTagField, result.JSTagField)
		require.Equal(t, data.NestedStruct, result.NestedStruct)

		// Test tag priority
		tagData := struct {
			Field string `js:"js_name" json:"json_name"`
		}{Field: "test"}

		jsVal2, err := ctx.Marshal(tagData)
		require.NoError(t, err)
		defer jsVal2.Free()

		require.True(t, jsVal2.Has("js_name"))
		require.False(t, jsVal2.Has("json_name"))
	})

	t.Run("CustomMarshalUnmarshal", func(t *testing.T) {
		// Test custom marshal
		data := CustomMarshalType{Value: "test"}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()
		require.Equal(t, "custom:test", jsVal.ToString())

		// Test custom unmarshal
		jsVal2 := ctx.String("custom:unmarshal_test")
		defer jsVal2.Free()

		var result CustomUnmarshalType
		err = ctx.Unmarshal(jsVal2, &result)
		require.NoError(t, err)
		require.Equal(t, "unmarshal_test", result.Value)

		// Test errors
		_, err = ctx.Marshal(ErrorMarshalType{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "marshal error")

		var errorResult ErrorUnmarshalType
		err = ctx.Unmarshal(jsVal2, &errorResult)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unmarshal error")
	})
}

func TestUnmarshalInterface(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	tests := []struct {
		name     string
		jsCode   string
		expected interface{}
	}{
		{"Null", "null", nil},
		{"Undefined", "undefined", nil},
		{"Boolean", "true", true},
		{"String", `"hello"`, "hello"},
		{"Integer", "42", int64(42)},
		{"Float", "3.14", 3.14},
		{"Array", "[1, 2, 3]", []interface{}{int64(1), int64(2), int64(3)}},
		{"Object", `({"name": "test", "value": 42})`, map[string]interface{}{"name": "test", "value": int64(42)}},
		{"EmptyArray", "[]", []interface{}{}},
		{"EmptyObject", "({})", map[string]interface{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var jsVal quickjs.Value
			var err error

			if tt.jsCode == "undefined" {
				jsVal = ctx.Undefined()
			} else {
				jsVal, err = ctx.Eval(tt.jsCode)
				require.NoError(t, err)
			}
			defer jsVal.Free()

			var result interface{}
			err = ctx.Unmarshal(jsVal, &result)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}

	// Test special cases
	t.Run("SpecialCases", func(t *testing.T) {
		// BigInt
		testValue := uint64(1 << 62)
		jsVal := ctx.BigUint64(testValue)
		defer jsVal.Free()

		var result interface{}
		err := ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)

		bigInt, ok := result.(*big.Int)
		require.True(t, ok)
		require.Equal(t, testValue, bigInt.Uint64())

		// ArrayBuffer
		data := []byte{1, 2, 3, 4, 5}
		jsVal2 := ctx.ArrayBuffer(data)
		defer jsVal2.Free()

		var result2 interface{}
		err = ctx.Unmarshal(jsVal2, &result2)
		require.NoError(t, err)
		require.Equal(t, data, result2)
	})
}

func TestErrorCases(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("MarshalErrors", func(t *testing.T) {
		errorTests := []interface{}{
			make(chan int),                                      // unsupported channel
			func() {},                                           // unsupported function
			[]chan int{make(chan int)},                          // slice with unsupported element
			[1]chan int{make(chan int)},                         // array with unsupported element
			map[string]chan int{"key": make(chan int)},          // map with unsupported value
			struct{ UnsupportedField chan int }{make(chan int)}, // struct with unsupported field
		}

		for i, data := range errorTests {
			t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
				_, err := ctx.Marshal(data)
				require.Error(t, err)
				require.Contains(t, err.Error(), "unsupported type")
			})
		}
	})

	t.Run("UnmarshalErrors", func(t *testing.T) {
		tests := []struct {
			name        string
			target      interface{}
			jsCode      string
			expectedErr string
		}{
			{"NonPointerTarget", "not a pointer", `"test"`, "must be a non-nil pointer"},
			{"NilPointerTarget", nil, `"test"`, "must be a non-nil pointer"},
			{"WrongTypeForSlice", &[]int{}, `"not an array"`, "expected array"},
			{"WrongTypeForMap", &map[string]int{}, `"not an object"`, "expected object"},
			{"WrongTypeForStruct", &TestStruct{}, `"not an object"`, "expected object"},
			{"UnsupportedMapKeyType", &map[float64]string{}, `({"key": "value"})`, "unsupported map key type"},
			{"NonArrayToArray", &[3]int{}, `"not an array"`, "expected array"},
			{"UnsupportedType", new(complex64), `1.0`, "unsupported type"},
			{"StringToInt", new(int), `"not a number"`, "cannot unmarshal JavaScript"},
			{"StringToBool", new(bool), `"not a boolean"`, "cannot unmarshal JavaScript"},
			{"StringToFloat", new(float64), `"not a number"`, "cannot unmarshal JavaScript"},
			{"StringToUint", new(uint32), `"not a number"`, "cannot unmarshal JavaScript"},
			{"StringToInt64", new(int64), `"not a number"`, "cannot unmarshal JavaScript"},
			{"StringToUint64", new(uint64), `"not a number"`, "cannot unmarshal JavaScript"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				jsVal, err := ctx.Eval(tt.jsCode)
				require.NoError(t, err)
				defer jsVal.Free()

				err = ctx.Unmarshal(jsVal, tt.target)
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			})
		}
	})

	t.Run("SpecificErrorPaths", func(t *testing.T) {
		// PropertyNames error
		jsVal, err := ctx.Eval(`
            new Proxy({}, {
                ownKeys: function(target) {
                    throw new Error("PropertyNames test error");
                }
            });
        `)
		require.NoError(t, err)
		defer jsVal.Free()

		var mapResult map[string]interface{}
		err = ctx.Unmarshal(jsVal, &mapResult)
		require.Error(t, err)

		var interfaceResult interface{}
		err = ctx.Unmarshal(jsVal, &interfaceResult)
		require.Error(t, err)

		// BigInt range errors
		jsVal2, err := ctx.Eval("BigInt('9223372036854775808')")
		require.NoError(t, err)
		defer jsVal2.Free()

		var result int64
		err = ctx.Unmarshal(jsVal2, &result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "BigInt value out of range for int64")

		// Negative BigInt to uint64
		jsVal3, err := ctx.Eval("BigInt('-1')")
		require.NoError(t, err)
		defer jsVal3.Free()

		var result2 uint64
		err = ctx.Unmarshal(jsVal3, &result2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "BigInt value out of range for uint64")

		// Negative number to uint64
		jsVal4 := ctx.Float64(-1.0)
		defer jsVal4.Free()

		var result3 uint64
		err = ctx.Unmarshal(jsVal4, &result3)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot unmarshal negative number into Go uint64")

		// Unsupported JavaScript types
		jsVal5, err := ctx.Eval(`Symbol('test')`)
		require.NoError(t, err)
		defer jsVal5.Free()

		var result4 interface{}
		err = ctx.Unmarshal(jsVal5, &result4)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported JavaScript type")
	})

	t.Run("ElementErrors", func(t *testing.T) {
		errorCases := []struct {
			name        string
			jsCode      string
			target      interface{}
			expectedErr string
		}{
			{"SliceElement", `[{"exported": "valid"}, "not_an_object"]`, &[]TestStruct{}, "array element"},
			{"ArrayElement", `[1, "invalid", 3]`, &[3]int{}, "array element"},
			{"MapValue", `({"key": function() {}})`, &map[string]string{}, "map value"},
			{"StructField", `({exported: function() {}})`, &TestStruct{}, "struct field"},
			{"UnsupportedInArray", `[1, Symbol('test'), 3]`, &[]interface{}{}, "unsupported JavaScript type"},
			{"UnsupportedInObject", `({"key": function() {}})`, &map[string]interface{}{}, "unsupported JavaScript type"},
		}

		for _, tt := range errorCases {
			t.Run(tt.name, func(t *testing.T) {
				jsVal, err := ctx.Eval(tt.jsCode)
				require.NoError(t, err)
				defer jsVal.Free()

				err = ctx.Unmarshal(jsVal, tt.target)
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			})
		}
	})

	// Test ToByteArray error in unmarshalInterface
	t.Run("ToByteArrayErrorInInterface", func(t *testing.T) {
		fakeArrayBuffer, err := ctx.Eval(`
            var fakeArrayBuffer = {
                constructor: ArrayBuffer,
                byteLength: 10,
                toString: function() { return "[object ArrayBuffer]"; }
            };
            Object.setPrototypeOf(fakeArrayBuffer, ArrayBuffer.prototype);
            Object.defineProperty(fakeArrayBuffer, Symbol.toStringTag, {
                value: "ArrayBuffer",
                configurable: true
            });
            fakeArrayBuffer;
        `)
		require.NoError(t, err)
		defer fakeArrayBuffer.Free()

		var result interface{}
		err = ctx.Unmarshal(fakeArrayBuffer, &result)
		if err != nil {
			t.Logf("✓ Covered ToByteArray error in unmarshalInterface: %v", err)
		}
	})
}

// New test functions to cover missing branches
func TestBigIntUnmarshaling(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("ValidBigIntToInt64", func(t *testing.T) {
		// Test valid BigInt that can be converted to int64
		jsVal, err := ctx.Eval("BigInt('123456789')")
		require.NoError(t, err)
		defer jsVal.Free()

		var result int64
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, int64(123456789), result)
	})

	t.Run("ValidPositiveNumberToUint64", func(t *testing.T) {
		// Test positive number that can be converted to uint64
		jsVal := ctx.Float64(123456789.0)
		defer jsVal.Free()

		var result uint64
		err := ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, uint64(123456789), result)
	})

	t.Run("ValidBigIntToUint64", func(t *testing.T) {
		// Test valid BigInt that can be converted to uint64
		jsVal, err := ctx.Eval("BigInt('18446744073709551615')") // max uint64
		require.NoError(t, err)
		defer jsVal.Free()

		var result uint64
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, uint64(18446744073709551615), result)
	})
}

func TestArrayMarshalCoverage(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("ArrayWithErrorElements", func(t *testing.T) {
		// Test array with elements that cause marshal errors
		// This should cover the error path in marshalArray where arr.Free() is called
		data := [2]interface{}{"valid", make(chan int)} // channel is unsupported

		_, err := ctx.Marshal(data)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported type")
	})

	t.Run("SliceWithErrorElements", func(t *testing.T) {
		// Test slice with elements that cause marshal errors
		// This should cover the error path in marshalGenericArray where arr.Free() is called
		data := []interface{}{"valid", make(chan int)} // channel is unsupported

		_, err := ctx.Marshal(data)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported type")
	})
}

func TestUnmarshalInterfaceErrors(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("ArrayWithErrorElements", func(t *testing.T) {
		// Create an array with elements that will cause unmarshal errors
		jsVal, err := ctx.Eval(`[1, Symbol('error'), 3]`)
		require.NoError(t, err)
		defer jsVal.Free()

		var result interface{}
		err = ctx.Unmarshal(jsVal, &result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported JavaScript type")
	})

	t.Run("ObjectWithErrorValues", func(t *testing.T) {
		// Create an object with values that will cause unmarshal errors
		jsVal, err := ctx.Eval(`({valid: 1, error: Symbol('error')})`)
		require.NoError(t, err)
		defer jsVal.Free()

		var result interface{}
		err = ctx.Unmarshal(jsVal, &result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported JavaScript type")
	})
}

func TestIntegrationExample(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	type User struct {
		ID        int64       `js:"id"`
		Name      string      `js:"name"`
		IsActive  bool        `js:"is_active"`
		Tags      []string    `js:"tags"`
		CreatedAt TimeWrapper `js:"created_at"`
	}

	user := User{
		ID:        123,
		Name:      "John Doe",
		IsActive:  true,
		Tags:      []string{"admin", "user"},
		CreatedAt: TimeWrapper{Time: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
	}

	// Marshal Go -> JavaScript
	jsVal, err := ctx.Marshal(user)
	require.NoError(t, err)

	// Use in JavaScript
	ctx.Globals().Set("user", jsVal)
	result, err := ctx.Eval(`
        user.name = "Jane Doe";
        user.tags.push("moderator");
        user;
    `)
	require.NoError(t, err)
	defer result.Free()

	// Unmarshal JavaScript -> Go
	var updatedUser User
	err = ctx.Unmarshal(result, &updatedUser)
	require.NoError(t, err)

	// Verify changes
	require.Equal(t, "Jane Doe", updatedUser.Name)
	require.Contains(t, updatedUser.Tags, "moderator")
	require.Equal(t, user.CreatedAt.Time, updatedUser.CreatedAt.Time)
}
