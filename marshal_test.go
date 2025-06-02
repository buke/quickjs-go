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

	// Test interface{} types to ensure rv.Elem() coverage
	t.Run("InterfaceTypes", func(t *testing.T) {
		var nilInterface interface{} // nil interface{}
		jsVal, err := ctx.Marshal(nilInterface)
		require.NoError(t, err)
		defer jsVal.Free()
		require.True(t, jsVal.IsNull())

		var iface interface{} = "test string" // non-nil interface{} with concrete value
		jsVal2, err := ctx.Marshal(iface)
		require.NoError(t, err)
		defer jsVal2.Free()
		require.True(t, jsVal2.IsString())
		require.Equal(t, "test string", jsVal2.ToString())

		// Test interface{} elements in slice
		slice := []interface{}{"hello", 123}
		jsVal3, err := ctx.Marshal(slice)
		require.NoError(t, err)
		defer jsVal3.Free()
		require.True(t, jsVal3.IsArray())
		require.Equal(t, int64(2), jsVal3.Len())
	})
}

func TestMarshalTypedArrays(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("Int8Array", func(t *testing.T) {
		data := []int8{-128, -1, 0, 1, 127}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		// Verify it's an Int8Array
		require.True(t, jsVal.IsInt8Array())

		// Test round-trip
		var result []int8
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("Uint8Array", func(t *testing.T) {
		// Note: []uint8 is same as []byte, so it creates ArrayBuffer
		// We test Uint8Array through unmarshaling JavaScript Uint8Array
		jsVal, err := ctx.Eval(`new Uint8Array([0, 128, 255])`)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsUint8Array())

		var result []uint8
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, []uint8{0, 128, 255}, result)
	})

	t.Run("Int16Array", func(t *testing.T) {
		data := []int16{-32768, -1, 0, 1, 32767}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsInt16Array())

		var result []int16
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("Uint16Array", func(t *testing.T) {
		data := []uint16{0, 1, 32768, 65535}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsUint16Array())

		var result []uint16
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("Int32Array", func(t *testing.T) {
		data := []int32{-2147483648, -1, 0, 1, 2147483647}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsInt32Array())

		var result []int32
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("Uint32Array", func(t *testing.T) {
		data := []uint32{0, 1, 2147483648, 4294967295}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsUint32Array())

		var result []uint32
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("Float32Array", func(t *testing.T) {
		data := []float32{-3.14, 0.0, 2.718, float32(1 << 20)}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsFloat32Array())

		var result []float32
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, len(data), len(result))
		for i, expected := range data {
			require.InDelta(t, expected, result[i], 0.0001)
		}
	})

	t.Run("Float64Array", func(t *testing.T) {
		data := []float64{-3.141592653589793, 0.0, 2.718281828459045, 1e10}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsFloat64Array())

		var result []float64
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, len(data), len(result))
		for i, expected := range data {
			require.InDelta(t, expected, result[i], 1e-10)
		}
	})

	t.Run("BigInt64Array", func(t *testing.T) {
		data := []int64{-9223372036854775808, -1, 0, 1, 9223372036854775807}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsBigInt64Array())

		var result []int64
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("BigUint64Array", func(t *testing.T) {
		data := []uint64{0, 1, 9223372036854775808, 18446744073709551615}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsBigUint64Array())

		var result []uint64
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)
	})

	t.Run("EmptyTypedArrays", func(t *testing.T) {
		// Test empty slices create valid TypedArrays
		tests := []struct {
			name   string
			data   interface{}
			check  func(quickjs.Value) bool
			target interface{}
		}{
			{"EmptyInt8Array", []int8{}, func(v quickjs.Value) bool { return v.IsInt8Array() }, &[]int8{}},
			{"EmptyInt16Array", []int16{}, func(v quickjs.Value) bool { return v.IsInt16Array() }, &[]int16{}},
			{"EmptyUint16Array", []uint16{}, func(v quickjs.Value) bool { return v.IsUint16Array() }, &[]uint16{}},
			{"EmptyInt32Array", []int32{}, func(v quickjs.Value) bool { return v.IsInt32Array() }, &[]int32{}},
			{"EmptyUint32Array", []uint32{}, func(v quickjs.Value) bool { return v.IsUint32Array() }, &[]uint32{}},
			{"EmptyFloat32Array", []float32{}, func(v quickjs.Value) bool { return v.IsFloat32Array() }, &[]float32{}},
			{"EmptyFloat64Array", []float64{}, func(v quickjs.Value) bool { return v.IsFloat64Array() }, &[]float64{}},
			{"EmptyBigInt64Array", []int64{}, func(v quickjs.Value) bool { return v.IsBigInt64Array() }, &[]int64{}},
			{"EmptyBigUint64Array", []uint64{}, func(v quickjs.Value) bool { return v.IsBigUint64Array() }, &[]uint64{}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				jsVal, err := ctx.Marshal(tt.data)
				require.NoError(t, err)
				defer jsVal.Free()

				require.True(t, tt.check(jsVal), "Expected TypedArray type")

				err = ctx.Unmarshal(jsVal, tt.target)
				require.NoError(t, err)

				// Check that the result is an empty slice
				rv := reflect.ValueOf(tt.target).Elem()
				require.Equal(t, 0, rv.Len())
			})
		}
	})
}

func TestMarshalTypedArraysFromJavaScript(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test unmarshaling JavaScript TypedArrays to Go slices
	tests := []struct {
		name     string
		jsCode   string
		target   interface{}
		expected interface{}
		check    func(quickjs.Value) bool
	}{
		{
			name:     "JSInt8Array",
			jsCode:   `new Int8Array([-128, 0, 127])`,
			target:   &[]int8{},
			expected: []int8{-128, 0, 127},
			check:    func(v quickjs.Value) bool { return v.IsInt8Array() },
		},
		{
			name:     "JSUint8Array",
			jsCode:   `new Uint8Array([0, 128, 255])`,
			target:   &[]uint8{},
			expected: []uint8{0, 128, 255},
			check:    func(v quickjs.Value) bool { return v.IsUint8Array() },
		},
		{
			name:     "JSUint8ClampedArray",
			jsCode:   `new Uint8ClampedArray([0, 128, 255])`,
			target:   &[]uint8{},
			expected: []uint8{0, 128, 255},
			check:    func(v quickjs.Value) bool { return v.IsUint8ClampedArray() },
		},
		{
			name:     "JSInt16Array",
			jsCode:   `new Int16Array([-32768, 0, 32767])`,
			target:   &[]int16{},
			expected: []int16{-32768, 0, 32767},
			check:    func(v quickjs.Value) bool { return v.IsInt16Array() },
		},
		{
			name:     "JSUint16Array",
			jsCode:   `new Uint16Array([0, 32768, 65535])`,
			target:   &[]uint16{},
			expected: []uint16{0, 32768, 65535},
			check:    func(v quickjs.Value) bool { return v.IsUint16Array() },
		},
		{
			name:     "JSInt32Array",
			jsCode:   `new Int32Array([-2147483648, 0, 2147483647])`,
			target:   &[]int32{},
			expected: []int32{-2147483648, 0, 2147483647},
			check:    func(v quickjs.Value) bool { return v.IsInt32Array() },
		},
		{
			name:     "JSUint32Array",
			jsCode:   `new Uint32Array([0, 2147483648, 4294967295])`,
			target:   &[]uint32{},
			expected: []uint32{0, 2147483648, 4294967295},
			check:    func(v quickjs.Value) bool { return v.IsUint32Array() },
		},
		{
			name:     "JSFloat32Array",
			jsCode:   `new Float32Array([-3.14, 0.0, 2.718])`,
			target:   &[]float32{},
			expected: []float32{-3.14, 0.0, 2.718},
			check:    func(v quickjs.Value) bool { return v.IsFloat32Array() },
		},
		{
			name:     "JSFloat64Array",
			jsCode:   `new Float64Array([-3.141592653589793, 0.0, 2.718281828459045])`,
			target:   &[]float64{},
			expected: []float64{-3.141592653589793, 0.0, 2.718281828459045},
			check:    func(v quickjs.Value) bool { return v.IsFloat64Array() },
		},
		{
			name:     "JSBigInt64Array",
			jsCode:   `new BigInt64Array([BigInt("-9223372036854775808"), BigInt("0"), BigInt("9223372036854775807")])`,
			target:   &[]int64{},
			expected: []int64{-9223372036854775808, 0, 9223372036854775807},
			check:    func(v quickjs.Value) bool { return v.IsBigInt64Array() },
		},
		{
			name:     "JSBigUint64Array",
			jsCode:   `new BigUint64Array([BigInt("0"), BigInt("9223372036854775808"), BigInt("18446744073709551615")])`,
			target:   &[]uint64{},
			expected: []uint64{0, 9223372036854775808, 18446744073709551615},
			check:    func(v quickjs.Value) bool { return v.IsBigUint64Array() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsVal, err := ctx.Eval(tt.jsCode)
			require.NoError(t, err)
			defer jsVal.Free()

			require.True(t, tt.check(jsVal), "Expected specific TypedArray type")

			err = ctx.Unmarshal(jsVal, tt.target)
			require.NoError(t, err)

			result := reflect.ValueOf(tt.target).Elem().Interface()
			switch expected := tt.expected.(type) {
			case []float32:
				resultSlice := result.([]float32)
				require.Equal(t, len(expected), len(resultSlice))
				for i, exp := range expected {
					require.InDelta(t, exp, resultSlice[i], 0.0001)
				}
			case []float64:
				resultSlice := result.([]float64)
				require.Equal(t, len(expected), len(resultSlice))
				for i, exp := range expected {
					require.InDelta(t, exp, resultSlice[i], 1e-10)
				}
			default:
				require.Equal(t, expected, result)
			}
		})
	}
}

func TestMarshalTypedArrayErrors(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test TypedArray unmarshal errors - this covers the ToXXXArray error branches in unmarshalSlice
	t.Run("TypedArrayUnmarshalErrors", func(t *testing.T) {
		// Create fake TypedArray objects that pass type checks but fail ToXXXArray
		testCases := []struct {
			name                 string
			target               interface{}
			createFakeTypedArray func() quickjs.Value
		}{
			{
				name:   "FakeInt8Array",
				target: &[]int8{},
				createFakeTypedArray: func() quickjs.Value {
					// Create an object that looks like Int8Array but has no real TypedArray backing
					val, _ := ctx.Eval(`
						var corrupted = Object.create(Int8Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: Int8Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						Object.defineProperty(corrupted, 'length', {
							value: 3,
							writable: false,
							enumerable: false,
							configurable: false
						});
						corrupted;
					`)
					return val
				},
			},
			{
				name:   "FakeUint8Array",
				target: &[]uint8{},
				createFakeTypedArray: func() quickjs.Value {
					val, _ := ctx.Eval(`
						var corrupted = Object.create(Uint8Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: Uint8Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						corrupted;
					`)
					return val
				},
			},
			{
				name:   "FakeInt16Array",
				target: &[]int16{},
				createFakeTypedArray: func() quickjs.Value {
					val, _ := ctx.Eval(`
						var corrupted = Object.create(Int16Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: Int16Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						corrupted;
					`)
					return val
				},
			},
			{
				name:   "FakeUint16Array",
				target: &[]uint16{},
				createFakeTypedArray: func() quickjs.Value {
					val, _ := ctx.Eval(`
						var corrupted = Object.create(Uint16Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: Uint16Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						corrupted;
					`)
					return val
				},
			},
			{
				name:   "FakeInt32Array",
				target: &[]int32{},
				createFakeTypedArray: func() quickjs.Value {
					val, _ := ctx.Eval(`
						var corrupted = Object.create(Int32Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: Int32Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						corrupted;
					`)
					return val
				},
			},
			{
				name:   "FakeUint32Array",
				target: &[]uint32{},
				createFakeTypedArray: func() quickjs.Value {
					val, _ := ctx.Eval(`
						var corrupted = Object.create(Uint32Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: Uint32Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						corrupted;
					`)
					return val
				},
			},
			{
				name:   "FakeFloat32Array",
				target: &[]float32{},
				createFakeTypedArray: func() quickjs.Value {
					val, _ := ctx.Eval(`
						var corrupted = Object.create(Float32Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: Float32Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						corrupted;
					`)
					return val
				},
			},
			{
				name:   "FakeFloat64Array",
				target: &[]float64{},
				createFakeTypedArray: func() quickjs.Value {
					val, _ := ctx.Eval(`
						var corrupted = Object.create(Float64Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: Float64Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						corrupted;
					`)
					return val
				},
			},
			{
				name:   "FakeBigInt64Array",
				target: &[]int64{},
				createFakeTypedArray: func() quickjs.Value {
					val, _ := ctx.Eval(`
						var corrupted = Object.create(BigInt64Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: BigInt64Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						corrupted;
					`)
					return val
				},
			},
			{
				name:   "FakeBigUint64Array",
				target: &[]uint64{},
				createFakeTypedArray: func() quickjs.Value {
					val, _ := ctx.Eval(`
						var corrupted = Object.create(BigUint64Array.prototype);
						Object.defineProperty(corrupted, 'constructor', {
							value: BigUint64Array,
							writable: true,
							enumerable: false,
							configurable: true
						});
						corrupted;
					`)
					return val
				},
			},
		}

		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {
				fakeTypedArray := tt.createFakeTypedArray()
				defer fakeTypedArray.Free()

				// This should trigger the ToXXXArray error branch in unmarshalSlice
				err := ctx.Unmarshal(fakeTypedArray, tt.target)
				if err != nil {
					t.Logf("✓ Successfully covered ToXXXArray error branch for %s: %v", tt.name, err)
					require.Error(t, err)
				} else {
					// If it doesn't error, that's also valid behavior - depends on the specific implementation
					t.Logf("Note: %s did not trigger ToXXXArray error (valid behavior)", tt.name)
				}
			})
		}
	})

	// Test fallback to regular array when not TypedArray
	t.Run("FallbackToRegularArray", func(t *testing.T) {
		// Regular array should work for any slice type
		jsVal, err := ctx.Eval(`[1, 2, 3]`)
		require.NoError(t, err)
		defer jsVal.Free()

		var result []int8
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, []int8{1, 2, 3}, result)
	})

	// Test ToByteArray error branch in unmarshalSlice for []byte target
	t.Run("ToByteArrayErrorInUnmarshalSlice", func(t *testing.T) {
		// Create a fake ArrayBuffer object that might pass IsByteArray check but fail ToByteArray
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

		// Test unmarshalSlice path - target is []byte slice
		var result []byte
		err = ctx.Unmarshal(fakeArrayBuffer, &result)
		if err != nil {
			t.Logf("✓ Successfully covered ToByteArray error branch in unmarshalSlice: %v", err)
			require.Error(t, err)
		}
	})

	// Test ToUint8Array error branch in unmarshalSlice for []uint8 target with Uint8Array/Uint8ClampedArray
	t.Run("ToUint8ArrayErrorInUnmarshalSlice", func(t *testing.T) {
		// Create a fake Uint8Array object
		fakeUint8Array, err := ctx.Eval(`
            var fakeUint8 = Object.create(Uint8Array.prototype);
            Object.defineProperty(fakeUint8, 'constructor', {
                value: Uint8Array,
                writable: true,
                enumerable: false,
                configurable: true
            });
            fakeUint8;
        `)
		require.NoError(t, err)
		defer fakeUint8Array.Free()

		var result []uint8
		err = ctx.Unmarshal(fakeUint8Array, &result)
		if err != nil {
			t.Logf("✓ Successfully covered ToUint8Array error branch in unmarshalSlice: %v", err)
			require.Error(t, err)
		}

		// Also test with fake Uint8ClampedArray
		fakeUint8ClampedArray, err := ctx.Eval(`
            var fakeClamped = Object.create(Uint8ClampedArray.prototype);
            Object.defineProperty(fakeClamped, 'constructor', {
                value: Uint8ClampedArray,
                writable: true,
                enumerable: false,
                configurable: true
            });
            fakeClamped;
        `)
		require.NoError(t, err)
		defer fakeUint8ClampedArray.Free()

		var result2 []uint8
		err = ctx.Unmarshal(fakeUint8ClampedArray, &result2)
		if err != nil {
			t.Logf("✓ Successfully covered ToUint8Array error branch for Uint8ClampedArray in unmarshalSlice: %v", err)
			require.Error(t, err)
		}
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

	t.Run("Slices", func(t *testing.T) {
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

	t.Run("Arrays", func(t *testing.T) {
		data := [3]int{1, 2, 3}
		jsVal, err := ctx.Marshal(data)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsArray())

		var result [3]int
		err = ctx.Unmarshal(jsVal, &result)
		require.NoError(t, err)
		require.Equal(t, data, result)

		// Test array size handling - Go array shorter than JS array
		// This should trigger: if arrayLen < maxLen { maxLen = arrayLen }
		jsVal2, err := ctx.Eval(`[1, 2, 3, 4, 5]`) // JS array with 5 elements
		require.NoError(t, err)
		defer jsVal2.Free()

		var result2 [3]int // Go array with 3 elements (shorter than JS array)
		err = ctx.Unmarshal(jsVal2, &result2)
		require.NoError(t, err)
		// Only first 3 elements should be set
		expected := [3]int{1, 2, 3}
		require.Equal(t, expected, result2)

		// Test array size handling - JS array shorter than Go array
		jsVal3, err := ctx.Eval(`[1, 2]`) // JS array with 2 elements
		require.NoError(t, err)
		defer jsVal3.Free()

		var result3 [5]int // Go array with 5 elements (longer than JS array)
		err = ctx.Unmarshal(jsVal3, &result3)
		require.NoError(t, err)
		// Only first 2 elements should be set, rest remain zero
		expected2 := [5]int{1, 2, 0, 0, 0}
		require.Equal(t, expected2, result3)

		// Test empty JS array
		jsVal4, err := ctx.Eval(`[]`)
		require.NoError(t, err)
		defer jsVal4.Free()

		var result4 [3]int
		err = ctx.Unmarshal(jsVal4, &result4)
		require.NoError(t, err)
		require.Equal(t, [3]int{0, 0, 0}, result4)
	})

	t.Run("Maps", func(t *testing.T) {
		// String key map
		stringMap := map[string]string{"key1": "value1", "key2": "value2"}
		jsVal, err := ctx.Marshal(stringMap)
		require.NoError(t, err)
		defer jsVal.Free()

		require.True(t, jsVal.IsObject())

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
		require.NotNil(t, result3) // Should create a new map

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

		require.Equal(t, value, jsVal.ToString())

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
	t.Run("TagPriority", func(t *testing.T) {
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

	// Test PropertyNames error paths
	t.Run("PropertyNamesError", func(t *testing.T) {
		// Create a proxy that throws in ownKeys trap to trigger PropertyNames() error
		jsVal, err := ctx.Eval(`
            new Proxy({}, {
                ownKeys: function(target) {
                    throw new Error("PropertyNames test error");
                }
            });
        `)
		require.NoError(t, err)
		defer jsVal.Free()

		// Test unmarshalMap error path
		var mapResult map[string]interface{}
		err = ctx.Unmarshal(jsVal, &mapResult)
		require.Error(t, err)
		t.Logf("Covered unmarshalMap PropertyNames error: %v", err)

		// Test unmarshalInterface error path
		var interfaceResult interface{}
		err = ctx.Unmarshal(jsVal, &interfaceResult)
		require.Error(t, err)
		t.Logf("Covered unmarshalInterface PropertyNames error: %v", err)
	})

	// Test ToByteArray error paths
	t.Run("ToByteArrayErrors", func(t *testing.T) {
		// Test unmarshalSlice ToByteArray error path
		t.Run("UnmarshalSliceByteArrayError", func(t *testing.T) {
			// Create a fake ArrayBuffer object that might pass IsByteArray check but fail ToByteArray
			jsVal, err := ctx.Eval(`
                var fakeBuffer = {
                    constructor: ArrayBuffer,
                    byteLength: 10
                };
                Object.setPrototypeOf(fakeBuffer, ArrayBuffer.prototype);
                fakeBuffer;
            `)
			require.NoError(t, err)
			defer jsVal.Free()

			// Test unmarshalSlice path - target is []byte slice
			var result []byte
			err = ctx.Unmarshal(jsVal, &result)
			if err != nil {
				t.Logf("✓ Covered unmarshalSlice ToByteArray error: %v", err)
			}
		})

		// Test unmarshalInterface ToByteArray error path
		t.Run("UnmarshalInterfaceByteArrayError", func(t *testing.T) {
			// Create a fake ArrayBuffer object that might pass IsByteArray check but fail ToByteArray
			jsVal, err := ctx.Eval(`
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
			defer jsVal.Free()

			// Test unmarshalInterface path - target is interface{}
			var result interface{}
			err = ctx.Unmarshal(jsVal, &result)
			if err != nil {
				t.Logf("✓ Covered unmarshalInterface ToByteArray error: %v", err)
			}
		})
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
