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

// Test types for custom Marshal/Unmarshal interfaces
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
		{"Int32", int32(-32), int32(-32)},
		{"Int64", int64(64), int64(64)},
		{"Uint32", uint32(32), uint32(32)},
		{"Uint64", uint64(1<<63 - 1), uint64(1<<63 - 1)},
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

			// Test round-trip based on type
			switch v := tt.expected.(type) {
			case bool:
				require.Equal(t, v, jsVal.ToBool())
			case string:
				require.Equal(t, v, jsVal.ToString())
			case int32:
				require.Equal(t, v, jsVal.ToInt32())
			case int64:
				require.Equal(t, v, jsVal.ToInt64())
			case uint32:
				require.Equal(t, v, jsVal.ToUint32())
			case uint64:
				if jsVal.IsBigInt() {
					bigInt := jsVal.ToBigInt()
					require.NotNil(t, bigInt)
					require.Equal(t, v, bigInt.Uint64())
				}
			case float64:
				require.InDelta(t, v, jsVal.ToFloat64(), 0.0001)
			}
		})
	}
}

func TestMarshalInterface(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("InterfaceWithNil", func(t *testing.T) {
		var nilInterface interface{} // nil interface{}
		jsVal, err := ctx.Marshal(nilInterface)
		require.NoError(t, err)
		defer jsVal.Free()
		require.True(t, jsVal.IsNull())
	})

	t.Run("InterfaceWithConcreteValue", func(t *testing.T) {
		var iface interface{} = "test string" // non-nil interface{} with concrete value
		jsVal, err := ctx.Marshal(iface)
		require.NoError(t, err)
		defer jsVal.Free()
		require.True(t, jsVal.IsString())
		require.Equal(t, "test string", jsVal.ToString())
	})

	t.Run("InterfaceInSlice", func(t *testing.T) {
		// Test interface{} elements in slice to ensure rv.Elem() coverage
		var iface1 interface{} = "hello"
		var iface2 interface{} = 123
		slice := []interface{}{iface1, iface2}

		jsVal, err := ctx.Marshal(slice)
		require.NoError(t, err)
		defer jsVal.Free()
		require.True(t, jsVal.IsArray())
		require.Equal(t, int64(2), jsVal.Len())

		elem0 := jsVal.GetIdx(0)
		defer elem0.Free()
		require.Equal(t, "hello", elem0.ToString())

		elem1 := jsVal.GetIdx(1)
		defer elem1.Free()
		require.Equal(t, int32(123), elem1.ToInt32())
	})
}

func TestMarshalComplexTypes(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("ByteSlice", func(t *testing.T) {
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

	t.Run("IntSlice", func(t *testing.T) {
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
	})

	t.Run("FixedArray", func(t *testing.T) {
		data := [3]int{1, 2, 3}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsArray())

		var result [3]int
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("Maps", func(t *testing.T) {
		// Test string key map
		stringMap := map[string]string{"key1": "value1", "key2": "value2"}
		jsVal, err := ctx.Marshal(stringMap)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsObject())

		var result map[string]string
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, stringMap, result)

		// Test int key map
		intMap := map[int]string{1: "one", 2: "two"}
		jsVal2, err := ctx.Marshal(intMap)
		require.NoError(t, err)
		defer jsVal2.Free()

		var result2 map[int]string
		err = ctx.Unmarshal(jsVal2, &result2)
		require.NoError(t, err)
		require.Equal(t, intMap, result2)

		// Test nil map
		var nilMap map[string]string
		jsVal3, err := ctx.Marshal(nilMap)
		require.NoError(t, err)
		defer jsVal3.Free()

		var result3 map[string]string
		err = ctx.Unmarshal(jsVal3, &result3)
		require.NoError(t, err)
		require.NotNil(t, result3) // Should create a new map
	})
}

func TestMarshalStructs(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

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

	// Check that correct fields are present
	require.True(t, jsVal.Has("exported"))
	require.True(t, jsVal.Has("json_field"))
	require.True(t, jsVal.Has("js_field"))
	require.True(t, jsVal.Has("NoTagField"))

	// Check that skipped fields are not present
	require.False(t, jsVal.Has("SkippedJSField"))
	require.False(t, jsVal.Has("SkippedJSONField"))

	var result TestStruct
	err = ctx.Unmarshal(jsVal, &result)
	require.NoError(t, err)
	require.Equal(t, data.ExportedField, result.ExportedField)
	require.Equal(t, data.JSONTagField, result.JSONTagField)
	require.Equal(t, data.JSTagField, result.JSTagField)
	require.Equal(t, data.NestedStruct, result.NestedStruct)

	// Test that js tag takes priority over json tag
	t.Run("JSTagPriority", func(t *testing.T) {
		data := struct {
			Field string `js:"js_name" json:"json_name"`
		}{
			Field: "test",
		}

		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.Has("js_name"))
		require.False(t, jsVal.Has("json_name"))
		require.False(t, jsVal.Has("Field"))

		fieldVal := jsVal.Get("js_name")
		defer fieldVal.Free()
		require.Equal(t, "test", fieldVal.ToString())
	})
}

func TestMarshalPointers(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("NonNilPointer", func(t *testing.T) {
		value := "test"
		ptr := &value
		jsVal, err := ctx.Marshal(ptr)
		require.NoError(t, err)
		defer jsVal.Free()

		require.Equal(t, value, jsVal.ToString())

		var result *string
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, value, *result)
	})

	t.Run("NilPointer", func(t *testing.T) {
		var ptr *string
		jsVal, err := ctx.Marshal(ptr)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsNull())

		var result *string
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Nil(t, result)
	})
}

func TestCustomMarshalUnmarshal(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("CustomMarshal", func(t *testing.T) {
		data := CustomMarshalType{Value: "test"}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()
		require.Equal(t, "custom:test", jsVal.ToString())
	})

	t.Run("CustomUnmarshal", func(t *testing.T) {
		jsVal := ctx.String("custom:unmarshal_test")
		defer jsVal.Free()

		var result CustomUnmarshalType
		err := ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, "unmarshal_test", result.Value)
	})

	t.Run("CustomMarshalError", func(t *testing.T) {
		data := ErrorMarshalType{}
		_, err := ctx.Marshal(data)
		require.Error(t, err)
		require.Contains(t, err.Error(), "marshal error")
	})

	t.Run("CustomUnmarshalError", func(t *testing.T) {
		jsVal := ctx.String("test")
		defer jsVal.Free()

		var result ErrorUnmarshalType
		err := ctx.Unmarshal(jsVal, &result)
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

	t.Run("BigInt", func(t *testing.T) {
		testValue := uint64(1 << 62)
		jsVal := ctx.BigUint64(testValue)
		defer jsVal.Free()

		var result interface{}
		err := ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)

		bigInt, ok := result.(*big.Int)
		require.True(t, ok)
		require.Equal(t, testValue, bigInt.Uint64())
	})

	t.Run("ArrayBuffer", func(t *testing.T) {
		data := []byte{1, 2, 3, 4, 5}
		jsVal := ctx.ArrayBuffer(data)
		defer jsVal.Free()

		var result interface{}
		err := ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})
}

func TestUnmarshalTypes(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test uint types
	tests := []struct {
		name     string
		jsCode   string
		target   interface{}
		expected interface{}
	}{
		{"Uint", "42", new(uint), uint(42)},
		{"Uint8", "255", new(uint8), uint8(255)},
		{"Uint16", "65535", new(uint16), uint16(65535)},
		{"Uint32", "4294967295", new(uint32), uint32(4294967295)},
		{"Uint64FromNumber", "12345", new(uint64), uint64(12345)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsVal, err := ctx.Eval(tt.jsCode)
			require.NoError(t, err)
			defer jsVal.Free()

			err = ctx.Unmarshal(jsVal, tt.target)
			require.NoError(t, err)

			rv := reflect.ValueOf(tt.target).Elem()
			require.Equal(t, tt.expected, rv.Interface())
		})
	}

	t.Run("BigIntToUint64", func(t *testing.T) {
		testValue := uint64(1<<63 - 1)
		jsVal := ctx.BigUint64(testValue)
		defer jsVal.Free()

		var result uint64
		err := ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, testValue, result)
	})
}

func TestErrorCases(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test marshal errors
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

	// Test unmarshal errors
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
			{"StringToInt", new(int), `"not a number"`, "cannot unmarshal JavaScript"},
			{"StringToBool", new(bool), `"not a boolean"`, "cannot unmarshal JavaScript"},
			{"StringToFloat", new(float64), `"not a number"`, "cannot unmarshal JavaScript"},
			{"StringToUint", new(uint32), `"not a number"`, "cannot unmarshal JavaScript"},
			{"StringToInt64", new(int64), `"not a number"`, "cannot unmarshal JavaScript"},
			{"StringToUint64", new(uint64), `"not a number"`, "cannot unmarshal JavaScript"},
			{"UnsupportedType", new(complex64), `1.0`, "unsupported type"},
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

	// Test BigInt edge cases
	t.Run("BigIntErrors", func(t *testing.T) {
		// BigInt too large for int64
		jsVal, err := ctx.Eval("BigInt('9223372036854775808')") // 2^63, larger than max int64
		require.NoError(t, err)
		defer jsVal.Free()

		var result int64
		err = ctx.Unmarshal(jsVal, &result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "BigInt value out of range for int64")

		// Negative BigInt to uint64
		jsVal2, err := ctx.Eval("BigInt('-1')")
		require.NoError(t, err)
		defer jsVal2.Free()

		var result2 uint64
		err = ctx.Unmarshal(jsVal2, &result2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "BigInt value out of range for uint64")

		// Negative number to uint64
		jsVal3 := ctx.Float64(-1.0)
		defer jsVal3.Free()

		var result3 uint64
		err = ctx.Unmarshal(jsVal3, &result3)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot unmarshal negative number into Go uint64")

		// Valid BigInt to int64
		testValue := int64(1<<62 - 1)
		jsVal4 := ctx.BigInt64(testValue)
		defer jsVal4.Free()

		var result4 int64
		err = ctx.Unmarshal(jsVal4, &result4)
		require.NoError(t, err)
		require.Equal(t, testValue, result4)
	})

	// Test element errors
	t.Run("ElementErrors", func(t *testing.T) {
		// Slice element error
		jsVal, err := ctx.Eval(`[{"exported": "valid"}, "not_an_object_for_struct"]`)
		require.NoError(t, err)
		defer jsVal.Free()

		var result []TestStruct
		err = ctx.Unmarshal(jsVal, &result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "array element")

		// Array element error
		jsVal2, err := ctx.Eval(`[1, "invalid_for_int", 3]`)
		require.NoError(t, err)
		defer jsVal2.Free()

		var result2 [3]int
		err = ctx.Unmarshal(jsVal2, &result2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "array element")

		// Map value error
		jsVal3, err := ctx.Eval(`({"key": function() {}})`)
		require.NoError(t, err)
		defer jsVal3.Free()

		var result3 map[string]string
		err = ctx.Unmarshal(jsVal3, &result3)
		require.Error(t, err)
		require.Contains(t, err.Error(), "map value")

		// Struct field error
		jsVal4, err := ctx.Eval(`({exported: function() {}})`)
		require.NoError(t, err)
		defer jsVal4.Free()

		var result4 TestStruct
		err = ctx.Unmarshal(jsVal4, &result4)
		require.Error(t, err)
		require.Contains(t, err.Error(), "struct field")
	})

	// Test unsupported JavaScript types
	t.Run("UnsupportedJSTypes", func(t *testing.T) {
		// Symbol
		jsVal, err := ctx.Eval(`Symbol('test')`)
		require.NoError(t, err)
		defer jsVal.Free()

		var result interface{}
		err = ctx.Unmarshal(jsVal, &result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported JavaScript type")

		// Function in array (interface{})
		jsVal2, err := ctx.Eval(`[1, Symbol('test'), 3]`)
		require.NoError(t, err)
		defer jsVal2.Free()

		var result2 interface{}
		err = ctx.Unmarshal(jsVal2, &result2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported JavaScript type")

		// Function in object (interface{})
		jsVal3, err := ctx.Eval(`({"key": function() {}})`)
		require.NoError(t, err)
		defer jsVal3.Free()

		var result3 interface{}
		err = ctx.Unmarshal(jsVal3, &result3)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported JavaScript type")
	})

	// Test numeric string to int key
	t.Run("NumericStringToIntKey", func(t *testing.T) {
		jsVal, err := ctx.Eval(`({abc: "value", "123": "numeric"})`)
		require.NoError(t, err)
		defer jsVal.Free()

		var result map[int]string
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		// Should only have the numeric key
		require.Equal(t, map[int]string{123: "numeric"}, result)
	})

	// Test error cases that are hard to trigger in unmarshalInterface
	t.Run("UnmarshalInterfaceErrors", func(t *testing.T) {
		// Test ToByteArray and PropertyNames error simulation
		// These errors are internal to QuickJS and hard to trigger normally

		// Test normal ArrayBuffer handling
		data := []byte{1, 2, 3, 4, 5}
		jsVal := ctx.ArrayBuffer(data)
		defer jsVal.Free()

		var result interface{}
		err := ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)

		// Test normal object handling
		jsVal2, err := ctx.Eval(`({"key1": "value1", "key2": "value2"})`)
		require.NoError(t, err)
		defer jsVal2.Free()

		var result2 interface{}
		err = ctx.Unmarshal(jsVal2, &result2)
		require.NoError(t, err)
		expected := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}
		require.Equal(t, expected, result2)
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
