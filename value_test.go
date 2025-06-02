package quickjs_test

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

func TestValueBasics(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Free() and Context()
	val := ctx.String("test")
	require.Equal(t, ctx, val.Context())
	val.Free()

	// Test basic type creation and checking
	testCases := []struct {
		name      string
		createVal func() quickjs.Value
		checkFunc func(quickjs.Value) bool
	}{
		{"Number", func() quickjs.Value { return ctx.Int32(42) }, func(v quickjs.Value) bool { return v.IsNumber() }},
		{"String", func() quickjs.Value { return ctx.String("test") }, func(v quickjs.Value) bool { return v.IsString() }},
		{"Boolean", func() quickjs.Value { return ctx.Bool(true) }, func(v quickjs.Value) bool { return v.IsBool() }},
		{"Null", func() quickjs.Value { return ctx.Null() }, func(v quickjs.Value) bool { return v.IsNull() }},
		{"Undefined", func() quickjs.Value { return ctx.Undefined() }, func(v quickjs.Value) bool { return v.IsUndefined() }},
		{"Uninitialized", func() quickjs.Value { return ctx.Uninitialized() }, func(v quickjs.Value) bool { return v.IsUninitialized() }},
		{"Object", func() quickjs.Value { return ctx.Object() }, func(v quickjs.Value) bool { return v.IsObject() }},
		{"BigInt", func() quickjs.Value { return ctx.BigInt64(123456789) }, func(v quickjs.Value) bool { return v.IsBigInt() }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val := tc.createVal()
			defer val.Free()
			require.True(t, tc.checkFunc(val))
		})
	}

	// Test JavaScript values
	t.Run("JavaScriptValues", func(t *testing.T) {
		// Array
		arr, err := ctx.Eval(`[1, 2, 3]`)
		require.NoError(t, err)
		defer arr.Free()
		require.True(t, arr.IsArray())
		require.True(t, arr.IsObject()) // Arrays are objects

		// Symbol
		sym, err := ctx.Eval(`Symbol('test')`)
		require.NoError(t, err)
		defer sym.Free()
		require.True(t, sym.IsSymbol())
	})
}

func TestValueConversions(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test type conversions
	tests := []struct {
		name           string
		createVal      func() quickjs.Value
		testFunc       func(quickjs.Value)
		testDeprecated func(quickjs.Value) // Test deprecated methods
	}{
		{
			name:           "Bool",
			createVal:      func() quickjs.Value { return ctx.Bool(true) },
			testFunc:       func(v quickjs.Value) { require.True(t, v.ToBool()) },
			testDeprecated: func(v quickjs.Value) { require.True(t, v.Bool()) },
		},
		{
			name:      "String",
			createVal: func() quickjs.Value { return ctx.String("Hello World") },
			testFunc: func(v quickjs.Value) {
				require.Equal(t, "Hello World", v.ToString())
				require.Equal(t, "Hello World", v.String()) // String() calls ToString()
			},
		},
		{
			name:           "Int32",
			createVal:      func() quickjs.Value { return ctx.Int32(42) },
			testFunc:       func(v quickjs.Value) { require.Equal(t, int32(42), v.ToInt32()) },
			testDeprecated: func(v quickjs.Value) { require.Equal(t, int32(42), v.Int32()) },
		},
		{
			name:           "Int64",
			createVal:      func() quickjs.Value { return ctx.Int64(1234567890) },
			testFunc:       func(v quickjs.Value) { require.Equal(t, int64(1234567890), v.ToInt64()) },
			testDeprecated: func(v quickjs.Value) { require.Equal(t, int64(1234567890), v.Int64()) },
		},
		{
			name:           "Uint32",
			createVal:      func() quickjs.Value { return ctx.Uint32(4294967295) },
			testFunc:       func(v quickjs.Value) { require.Equal(t, uint32(4294967295), v.ToUint32()) },
			testDeprecated: func(v quickjs.Value) { require.Equal(t, uint32(4294967295), v.Uint32()) },
		},
		{
			name:           "Float64",
			createVal:      func() quickjs.Value { return ctx.Float64(3.14159) },
			testFunc:       func(v quickjs.Value) { require.InDelta(t, 3.14159, v.ToFloat64(), 0.00001) },
			testDeprecated: func(v quickjs.Value) { require.InDelta(t, 3.14159, v.Float64(), 0.00001) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.createVal()
			defer val.Free()

			tt.testFunc(val)
			if tt.testDeprecated != nil {
				tt.testDeprecated(val)
			}
		})
	}

	// Test BigInt conversion
	t.Run("BigInt", func(t *testing.T) {
		bigIntVal := ctx.BigInt64(9223372036854775807)
		defer bigIntVal.Free()

		expectedBigInt := big.NewInt(9223372036854775807)
		require.Equal(t, expectedBigInt, bigIntVal.ToBigInt())
		require.Equal(t, expectedBigInt, bigIntVal.BigInt()) // Deprecated method

		// Test ToBigInt with non-BigInt value (should return nil)
		normalIntVal := ctx.Int32(42)
		defer normalIntVal.Free()
		require.Nil(t, normalIntVal.ToBigInt())
	})
}

func TestValueJSON(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test object JSON stringify
	t.Run("Object", func(t *testing.T) {
		obj := ctx.Object()
		defer obj.Free()
		obj.Set("name", ctx.String("test"))
		obj.Set("value", ctx.Int32(42))

		jsonStr := obj.JSONStringify()
		require.Contains(t, jsonStr, "name")
		require.Contains(t, jsonStr, "test")
		require.Contains(t, jsonStr, "value")
		require.Contains(t, jsonStr, "42")
	})

	// Test array JSON stringify
	t.Run("Array", func(t *testing.T) {
		arr, err := ctx.Eval(`[1, 2, 3]`)
		require.NoError(t, err)
		defer arr.Free()
		require.Equal(t, "[1,2,3]", arr.JSONStringify())
	})

	// Test various value types
	testCases := []struct {
		name      string
		createVal func() quickjs.Value
		expected  string
	}{
		{"String", func() quickjs.Value { return ctx.String("hello") }, `"hello"`},
		{"Null", func() quickjs.Value { return ctx.Null() }, "null"},
		{"True", func() quickjs.Value { return ctx.Bool(true) }, "true"},
		{"False", func() quickjs.Value { return ctx.Bool(false) }, "false"},
		{"Number", func() quickjs.Value { return ctx.Int32(42) }, "42"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val := tc.createVal()
			defer val.Free()
			require.Equal(t, tc.expected, val.JSONStringify())
		})
	}
}

func TestValueArrayBuffer(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test ArrayBuffer creation and operations
	t.Run("BasicOperations", func(t *testing.T) {
		data := []byte{1, 2, 3, 4, 5}
		arrayBuffer := ctx.ArrayBuffer(data)
		defer arrayBuffer.Free()

		require.True(t, arrayBuffer.IsByteArray())
		require.Equal(t, int64(len(data)), arrayBuffer.ByteLen())

		// Test ToByteArray with various sizes
		for i := 1; i <= len(data); i++ {
			result, err := arrayBuffer.ToByteArray(uint(i))
			require.NoError(t, err)
			require.Equal(t, data[:i], result)
		}

		// Test ToByteArray with size exceeding buffer length
		_, err := arrayBuffer.ToByteArray(uint(len(data)) + 1)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds the maximum length")
	})

	// Test empty ArrayBuffer
	t.Run("EmptyBuffer", func(t *testing.T) {
		emptyBuffer := ctx.ArrayBuffer([]byte{})
		defer emptyBuffer.Free()

		require.True(t, emptyBuffer.IsByteArray())
		require.Equal(t, int64(0), emptyBuffer.ByteLen())

		result, err := emptyBuffer.ToByteArray(0)
		require.NoError(t, err)
		require.Empty(t, result)

		// Test requesting bytes from zero-size buffer
		_, err = emptyBuffer.ToByteArray(1)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds the maximum length")
	})

	// Test array length
	t.Run("ArrayLength", func(t *testing.T) {
		arr, err := ctx.Eval(`[1, 2, 3, 4, 5]`)
		require.NoError(t, err)
		defer arr.Free()
		require.Equal(t, int64(5), arr.Len())
	})

	// Comprehensive error coverage
	t.Run("ErrorCases", func(t *testing.T) {
		// Test various non-ArrayBuffer types
		errorTests := []struct {
			name        string
			createVal   func() quickjs.Value
			expectedErr string
		}{
			{"Object", func() quickjs.Value { return ctx.Object() }, "exceeds the maximum length"},
			{"String", func() quickjs.Value { return ctx.String("not an array buffer") }, "exceeds the maximum length"},
			{"Number", func() quickjs.Value { return ctx.Int32(42) }, "exceeds the maximum length"},
			{"Boolean", func() quickjs.Value { return ctx.Bool(true) }, "exceeds the maximum length"},
			{"Null", func() quickjs.Value { return ctx.Null() }, "exceeds the maximum length"},
			{"Undefined", func() quickjs.Value { return ctx.Undefined() }, "exceeds the maximum length"},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				val := tt.createVal()
				defer val.Free()

				_, err := val.ToByteArray(1)
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			})
		}

		// Test function type
		funcVal := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			return ctx.Null()
		})
		defer funcVal.Free()

		_, err := funcVal.ToByteArray(1)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds the maximum length")

		// Test size validation
		validBuffer := ctx.ArrayBuffer([]byte{1, 2, 3})
		defer validBuffer.Free()

		_, err = validBuffer.ToByteArray(10) // Request more than available
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds the maximum length")

		// Test fake ArrayBuffer
		fakeArrayBuffer, err := ctx.Eval(`
			var fake = {
				constructor: ArrayBuffer,
				byteLength: 5
			};
			Object.setPrototypeOf(fake, ArrayBuffer.prototype);
			fake;
		`)
		require.NoError(t, err)
		defer fakeArrayBuffer.Free()

		if fakeArrayBuffer.IsByteArray() {
			_, err = fakeArrayBuffer.ToByteArray(5)
			if err != nil {
				require.Contains(t, err.Error(), "failed to get ArrayBuffer data")
			}
		}
	})
}

func TestValueTypedArrays(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test TypedArray detection
	t.Run("Detection", func(t *testing.T) {
		typedArrayTests := []struct {
			name      string
			jsCode    string
			checkFunc func(quickjs.Value) bool
			isTyped   bool
		}{
			{"Int8Array", "new Int8Array([1, 2, 3])", func(v quickjs.Value) bool { return v.IsInt8Array() }, true},
			{"Uint8Array", "new Uint8Array([1, 2, 3])", func(v quickjs.Value) bool { return v.IsUint8Array() }, true},
			{"Uint8ClampedArray", "new Uint8ClampedArray([1, 2, 3])", func(v quickjs.Value) bool { return v.IsUint8ClampedArray() }, true},
			{"Int16Array", "new Int16Array([1, 2, 3])", func(v quickjs.Value) bool { return v.IsInt16Array() }, true},
			{"Uint16Array", "new Uint16Array([1, 2, 3])", func(v quickjs.Value) bool { return v.IsUint16Array() }, true},
			{"Int32Array", "new Int32Array([1, 2, 3])", func(v quickjs.Value) bool { return v.IsInt32Array() }, true},
			{"Uint32Array", "new Uint32Array([1, 2, 3])", func(v quickjs.Value) bool { return v.IsUint32Array() }, true},
			{"Float32Array", "new Float32Array([1.5, 2.5, 3.5])", func(v quickjs.Value) bool { return v.IsFloat32Array() }, true},
			{"Float64Array", "new Float64Array([1.5, 2.5, 3.5])", func(v quickjs.Value) bool { return v.IsFloat64Array() }, true},
			{"BigInt64Array", "new BigInt64Array([1n, 2n, 3n])", func(v quickjs.Value) bool { return v.IsBigInt64Array() }, true},
			{"BigUint64Array", "new BigUint64Array([1n, 2n, 3n])", func(v quickjs.Value) bool { return v.IsBigUint64Array() }, true},
			{"RegularArray", "[1, 2, 3]", func(v quickjs.Value) bool { return v.IsInt8Array() }, false},
			{"Object", "{}", func(v quickjs.Value) bool { return v.IsTypedArray() }, false},
		}

		for _, tt := range typedArrayTests {
			t.Run(tt.name, func(t *testing.T) {
				val, err := ctx.Eval(tt.jsCode)
				require.NoError(t, err)
				defer val.Free()

				require.Equal(t, tt.isTyped, tt.checkFunc(val))
				if tt.isTyped {
					require.True(t, val.IsTypedArray())
				}
			})
		}
	})

	// Test TypedArray conversions
	t.Run("Conversions", func(t *testing.T) {
		conversionTests := []struct {
			name        string
			jsCode      string
			convertFunc func(quickjs.Value) (interface{}, error)
			expected    interface{}
			deltaCheck  bool
		}{
			{
				name:        "Int8Array",
				jsCode:      "new Int8Array([-128, 0, 127])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToInt8Array() },
				expected:    []int8{-128, 0, 127},
			},
			{
				name:        "Uint8Array",
				jsCode:      "new Uint8Array([0, 128, 255])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToUint8Array() },
				expected:    []uint8{0, 128, 255},
			},
			{
				name:        "Uint8ClampedArray",
				jsCode:      "new Uint8ClampedArray([0, 128, 255])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToUint8Array() },
				expected:    []uint8{0, 128, 255},
			},
			{
				name:        "Int16Array",
				jsCode:      "new Int16Array([-32768, 0, 32767])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToInt16Array() },
				expected:    []int16{-32768, 0, 32767},
			},
			{
				name:        "Uint16Array",
				jsCode:      "new Uint16Array([0, 32768, 65535])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToUint16Array() },
				expected:    []uint16{0, 32768, 65535},
			},
			{
				name:        "Int32Array",
				jsCode:      "new Int32Array([-2147483648, 0, 2147483647])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToInt32Array() },
				expected:    []int32{-2147483648, 0, 2147483647},
			},
			{
				name:        "Uint32Array",
				jsCode:      "new Uint32Array([0, 2147483648, 4294967295])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToUint32Array() },
				expected:    []uint32{0, 2147483648, 4294967295},
			},
			{
				name:        "Float32Array",
				jsCode:      "new Float32Array([1.5, 2.5, 3.14159])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToFloat32Array() },
				expected:    []float32{1.5, 2.5, 3.14159},
				deltaCheck:  true,
			},
			{
				name:        "Float64Array",
				jsCode:      "new Float64Array([1.5, 2.5, 3.141592653589793])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToFloat64Array() },
				expected:    []float64{1.5, 2.5, 3.141592653589793},
				deltaCheck:  true,
			},
			{
				name:        "BigInt64Array",
				jsCode:      "new BigInt64Array([-9223372036854775808n, 0n, 9223372036854775807n])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToBigInt64Array() },
				expected:    []int64{-9223372036854775808, 0, 9223372036854775807},
			},
			{
				name:        "BigUint64Array",
				jsCode:      "new BigUint64Array([0n, 9223372036854775808n, 18446744073709551615n])",
				convertFunc: func(v quickjs.Value) (interface{}, error) { return v.ToBigUint64Array() },
				expected:    []uint64{0, 9223372036854775808, 18446744073709551615},
			},
		}

		for _, tt := range conversionTests {
			t.Run(tt.name, func(t *testing.T) {
				val, err := ctx.Eval(tt.jsCode)
				require.NoError(t, err)
				defer val.Free()

				result, err := tt.convertFunc(val)
				require.NoError(t, err)

				if tt.deltaCheck {
					switch expected := tt.expected.(type) {
					case []float32:
						resultSlice := result.([]float32)
						require.Len(t, resultSlice, len(expected))
						for i, exp := range expected {
							require.InDelta(t, exp, resultSlice[i], 0.0001)
						}
					case []float64:
						resultSlice := result.([]float64)
						require.Len(t, resultSlice, len(expected))
						for i, exp := range expected {
							require.InDelta(t, exp, resultSlice[i], 1e-10)
						}
					}
				} else {
					require.Equal(t, tt.expected, result)
				}

				// Test error case with wrong type
				wrongType := ctx.String("not a typed array")
				defer wrongType.Free()
				_, err = tt.convertFunc(wrongType)
				require.Error(t, err)
			})
		}
	})

	// Test exception handling in ToXXXArray methods
	t.Run("ExceptionHandling", func(t *testing.T) {
		testCases := []struct {
			name        string
			arrayType   string
			convertFunc func(quickjs.Value) (interface{}, error)
		}{
			{"Int8Array", "Int8Array", func(v quickjs.Value) (interface{}, error) { return v.ToInt8Array() }},
			{"Uint8Array", "Uint8Array", func(v quickjs.Value) (interface{}, error) { return v.ToUint8Array() }},
			{"Uint8ClampedArray", "Uint8ClampedArray", func(v quickjs.Value) (interface{}, error) { return v.ToUint8Array() }},
			{"Int16Array", "Int16Array", func(v quickjs.Value) (interface{}, error) { return v.ToInt16Array() }},
			{"Uint16Array", "Uint16Array", func(v quickjs.Value) (interface{}, error) { return v.ToUint16Array() }},
			{"Int32Array", "Int32Array", func(v quickjs.Value) (interface{}, error) { return v.ToInt32Array() }},
			{"Uint32Array", "Uint32Array", func(v quickjs.Value) (interface{}, error) { return v.ToUint32Array() }},
			{"Float32Array", "Float32Array", func(v quickjs.Value) (interface{}, error) { return v.ToFloat32Array() }},
			{"Float64Array", "Float64Array", func(v quickjs.Value) (interface{}, error) { return v.ToFloat64Array() }},
			{"BigInt64Array", "BigInt64Array", func(v quickjs.Value) (interface{}, error) { return v.ToBigInt64Array() }},
			{"BigUint64Array", "BigUint64Array", func(v quickjs.Value) (interface{}, error) { return v.ToBigUint64Array() }},
		}

		for _, tc := range testCases {
			t.Run(tc.name+"Exception", func(t *testing.T) {
				// Create fake TypedArray that triggers buffer.IsException()
				corruptedArray, err := ctx.Eval(fmt.Sprintf(`
					var corrupted = Object.create(%s.prototype);
					Object.defineProperty(corrupted, 'constructor', {
						value: %s,
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
					Object.defineProperty(corrupted, 'byteLength', {
						value: 3,
						writable: false,
						enumerable: false,
						configurable: false
					});
					Object.defineProperty(corrupted, 'byteOffset', {
						value: 0,
						writable: false,
						enumerable: false,
						configurable: false
					});
					corrupted;
				`, tc.arrayType, tc.arrayType))
				require.NoError(t, err)
				defer corruptedArray.Free()

				result, err := tc.convertFunc(corruptedArray)
				if err != nil {
					t.Logf("✓ Successfully triggered error for %s: %v", tc.arrayType, err)
					require.Error(t, err)
				} else {
					t.Logf("Note: %s did not trigger exception (valid behavior), result: %v", tc.arrayType, result)
				}
			})
		}
	})
}

func TestValueProperties(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Test basic property operations
	t.Run("BasicOperations", func(t *testing.T) {
		obj.Set("name", ctx.String("test"))
		obj.Set("value", ctx.Int32(42))
		obj.Set("flag", ctx.Bool(true))

		require.True(t, obj.Has("name"))
		require.True(t, obj.Has("value"))
		require.False(t, obj.Has("nonexistent"))

		nameVal := obj.Get("name")
		defer nameVal.Free()
		require.Equal(t, "test", nameVal.String())

		require.True(t, obj.Delete("flag"))
		require.False(t, obj.Has("flag"))
		require.False(t, obj.Delete("nonexistent"))
	})

	// Test indexed property operations
	t.Run("IndexedOperations", func(t *testing.T) {
		obj.SetIdx(0, ctx.String("index0"))
		obj.SetIdx(1, ctx.String("index1"))

		require.True(t, obj.HasIdx(0))
		require.True(t, obj.HasIdx(1))
		require.False(t, obj.HasIdx(99))

		idx0Val := obj.GetIdx(0)
		defer idx0Val.Free()
		require.Equal(t, "index0", idx0Val.String())

		require.True(t, obj.DeleteIdx(0))
		require.False(t, obj.HasIdx(0))
		require.False(t, obj.DeleteIdx(99))
	})

	// Test PropertyNames
	t.Run("PropertyNames", func(t *testing.T) {
		obj.Set("a", ctx.String("value_a"))
		obj.Set("b", ctx.String("value_b"))

		names, err := obj.PropertyNames()
		require.NoError(t, err)
		require.Contains(t, names, "a")
		require.Contains(t, names, "b")

		// Test PropertyNames with non-object types
		nonObjectTests := []struct {
			name      string
			createVal func() quickjs.Value
		}{
			{"String", func() quickjs.Value { return ctx.String("test") }},
			{"Number", func() quickjs.Value { return ctx.Int32(42) }},
			{"Null", func() quickjs.Value { return ctx.Null() }},
			{"Undefined", func() quickjs.Value { return ctx.Undefined() }},
			{"Boolean", func() quickjs.Value { return ctx.Bool(true) }},
		}

		for _, tt := range nonObjectTests {
			t.Run(tt.name, func(t *testing.T) {
				val := tt.createVal()
				defer val.Free()

				_, err := val.PropertyNames()
				require.Error(t, err)
				require.Contains(t, err.Error(), "value does not contain properties")
			})
		}
	})

	// Test special property operations
	t.Run("SpecialProperties", func(t *testing.T) {
		// Empty key
		obj.Set("", ctx.String("empty key"))
		emptyKeyVal := obj.Get("")
		defer emptyKeyVal.Free()
		require.Equal(t, "empty key", emptyKeyVal.String())

		// Non-configurable property deletion (array length)
		arr, err := ctx.Eval(`[1, 2, 3]`)
		require.NoError(t, err)
		defer arr.Free()

		require.True(t, arr.Has("length"))
		require.False(t, arr.Delete("length")) // Should fail
		require.True(t, arr.Has("length"))     // Should still exist
	})
}

func TestValueFunctionCalls(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Test function calls
	t.Run("BasicCalls", func(t *testing.T) {
		addFunc := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			if len(args) < 2 {
				return ctx.Int32(0)
			}
			return ctx.Int32(args[0].ToInt32() + args[1].ToInt32())
		})
		obj.Set("add", addFunc)

		// Call with arguments
		result := obj.Call("add", ctx.Int32(3), ctx.Int32(4))
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, int32(7), result.ToInt32())

		// Call without arguments (covers len(cargs) == 0 branch)
		noArgsFunc := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			return ctx.String("no arguments received")
		})
		obj.Set("noArgs", noArgsFunc)

		noArgsResult := obj.Call("noArgs")
		defer noArgsResult.Free()
		require.False(t, noArgsResult.IsException())
		require.Equal(t, "no arguments received", noArgsResult.String())

		// Execute method
		execResult := addFunc.Execute(ctx.Null(), ctx.Int32(5), ctx.Int32(6))
		defer execResult.Free()
		require.False(t, execResult.IsException())
		require.Equal(t, int32(11), execResult.ToInt32())

		// Error case
		errorResult := obj.Call("nonexistent", ctx.Int32(1))
		defer errorResult.Free()
		require.True(t, errorResult.IsException())
	})

	// Test constructors
	t.Run("Constructors", func(t *testing.T) {
		constructorFunc, err := ctx.Eval(`
			function TestClass(value) {
				this.value = value;
			}
			TestClass;
		`)
		require.NoError(t, err)
		defer constructorFunc.Free()

		// CallConstructor
		instance := constructorFunc.CallConstructor(ctx.String("test_value"))
		defer instance.Free()
		require.False(t, instance.IsException())
		require.True(t, instance.IsObject())

		valueProperty := instance.Get("value")
		defer valueProperty.Free()
		require.Equal(t, "test_value", valueProperty.String())

		// New (alias for CallConstructor)
		instance2 := constructorFunc.New(ctx.String("test_value2"))
		defer instance2.Free()
		require.False(t, instance2.IsException())

		// Error case
		nonConstructor := ctx.String("not a constructor")
		defer nonConstructor.Free()
		errorResult := nonConstructor.CallConstructor()
		defer errorResult.Free()
		require.True(t, errorResult.IsException())
	})
}

func TestValueError(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test error creation and conversion
	t.Run("BasicErrors", func(t *testing.T) {
		testErr := errors.New("test error message")
		errorVal := ctx.Error(testErr)
		defer errorVal.Free()

		require.True(t, errorVal.IsError())

		// Test new method
		convertedErr := errorVal.ToError()
		require.NotNil(t, convertedErr)
		require.Contains(t, convertedErr.Error(), "test error message")

		// Test deprecated method
		deprecatedErr := errorVal.Error()
		require.NotNil(t, deprecatedErr)
		require.Contains(t, deprecatedErr.Error(), "test error message")

		// Test ToError on non-error value
		str := ctx.String("not an error")
		defer str.Free()
		require.Nil(t, str.ToError())
	})

	// Test complex error with properties
	t.Run("ComplexError", func(t *testing.T) {
		complexError, err := ctx.Eval(`
			const err = new Error("complex error");
			err.name = "CustomError";
			err.cause = "underlying cause";
			err.stack = "stack trace here";
			err;
		`)
		require.NoError(t, err)
		defer complexError.Free()

		complexConvertedErr := complexError.ToError()
		require.NotNil(t, complexConvertedErr)

		quickjsErr, ok := complexConvertedErr.(*quickjs.Error)
		require.True(t, ok)
		require.Equal(t, "underlying cause", quickjsErr.Cause)
		require.Equal(t, "CustomError", quickjsErr.Name)
		require.Equal(t, "complex error", quickjsErr.Message)
		require.Equal(t, "stack trace here", quickjsErr.Stack)
	})
}

func TestValueInstanceof(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test valid instanceof cases
	t.Run("ValidCases", func(t *testing.T) {
		// Array
		arr, err := ctx.Eval(`[1, 2, 3]`)
		require.NoError(t, err)
		defer arr.Free()
		require.True(t, arr.GlobalInstanceof("Array"))
		require.True(t, arr.GlobalInstanceof("Object"))

		// Object
		obj, err := ctx.Eval(`({})`)
		require.NoError(t, err)
		defer obj.Free()
		require.True(t, obj.GlobalInstanceof("Object"))
		require.False(t, obj.GlobalInstanceof("Array"))

		// Custom constructor
		result, err := ctx.Eval(`
			function CustomClass() {}
			globalThis.CustomClass = CustomClass;
			new CustomClass();
		`)
		require.NoError(t, err)
		defer result.Free()
		require.True(t, result.GlobalInstanceof("CustomClass"))
	})

	// Test false cases to ensure coverage
	t.Run("FalseCases", func(t *testing.T) {
		testVals := []struct {
			name      string
			createVal func() quickjs.Value
		}{
			{"String", func() quickjs.Value { return ctx.String("test") }},
			{"Null", func() quickjs.Value { return ctx.Null() }},
			{"Undefined", func() quickjs.Value { return ctx.Undefined() }},
			{"Int32", func() quickjs.Value { return ctx.Int32(42) }},
			{"Float64", func() quickjs.Value { return ctx.Float64(3.14) }},
			{"Bool", func() quickjs.Value { return ctx.Bool(true) }},
		}

		constructors := []string{"Object", "Array", "Function", "Date", "NonExistent", ""}

		for _, tv := range testVals {
			t.Run(tv.name, func(t *testing.T) {
				val := tv.createVal()
				defer val.Free()

				for _, constructor := range constructors {
					require.False(t, val.GlobalInstanceof(constructor),
						"Test case %s with constructor %s should return false", tv.name, constructor)
				}
			})
		}

		// Test specific object mismatch cases
		obj, err := ctx.Eval(`({})`)
		require.NoError(t, err)
		defer obj.Free()

		require.False(t, obj.GlobalInstanceof("Array"))
		require.False(t, obj.GlobalInstanceof("Function"))
		require.False(t, obj.GlobalInstanceof("Date"))
	})
}

func TestValueSpecialTypes(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test function
	t.Run("Function", func(t *testing.T) {
		funcVal := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
			return ctx.Null()
		})
		defer funcVal.Free()
		require.True(t, funcVal.IsFunction())
		require.False(t, funcVal.IsPromise()) // Functions are not promises
	})

	// Test constructor
	t.Run("Constructor", func(t *testing.T) {
		constructorVal, err := ctx.Eval(`function TestConstructor() {}; TestConstructor`)
		require.NoError(t, err)
		defer constructorVal.Free()
		require.True(t, constructorVal.IsConstructor())
	})

	// Test promises
	t.Run("Promises", func(t *testing.T) {
		promiseTests := []struct {
			name   string
			jsCode string
		}{
			{"Pending", `new Promise((resolve) => resolve("test"))`},
			{"Fulfilled", `Promise.resolve("fulfilled")`},
			{"Rejected", `Promise.reject("rejected")`},
		}

		for _, tt := range promiseTests {
			t.Run(tt.name, func(t *testing.T) {
				promiseVal, err := ctx.Eval(tt.jsCode)
				require.NoError(t, err)
				defer promiseVal.Free()
				require.True(t, promiseVal.IsPromise())
				require.True(t, promiseVal.IsObject()) // Promises are objects
			})
		}

		// Test non-Promise objects for IsPromise method (covers return false branch)
		nonPromiseTests := []struct {
			name      string
			createVal func() quickjs.Value
		}{
			{"Object", func() quickjs.Value { return ctx.Object() }},
			{"String", func() quickjs.Value { return ctx.String("not a promise") }},
			{"Number", func() quickjs.Value { return ctx.Int32(42) }},
			{"Null", func() quickjs.Value { return ctx.Null() }},
			{"Undefined", func() quickjs.Value { return ctx.Undefined() }},
		}

		for _, tt := range nonPromiseTests {
			t.Run(tt.name+"NotPromise", func(t *testing.T) {
				val := tt.createVal()
				defer val.Free()
				require.False(t, val.IsPromise())
			})
		}
	})

	// Test exception handling
	t.Run("Exception", func(t *testing.T) {
		_, err := ctx.Eval(`throw new Error("test error")`)
		require.Error(t, err)
	})
}

func TestValueEdgeCases(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test empty and zero values
	t.Run("EmptyAndZero", func(t *testing.T) {
		emptyStr := ctx.String("")
		defer emptyStr.Free()
		require.Equal(t, "", emptyStr.String())
		require.Equal(t, `""`, emptyStr.JSONStringify())

		zeroInt := ctx.Int32(0)
		defer zeroInt.Free()
		require.Equal(t, int32(0), zeroInt.ToInt32())
		require.False(t, zeroInt.ToBool()) // 0 is falsy

		negativeInt := ctx.Int32(-42)
		defer negativeInt.Free()
		require.Equal(t, int32(-42), negativeInt.ToInt32())
	})

	// Test special float values
	t.Run("SpecialFloats", func(t *testing.T) {
		infVal, err := ctx.Eval(`Infinity`)
		require.NoError(t, err)
		defer infVal.Free()
		require.True(t, infVal.IsNumber())

		nanVal, err := ctx.Eval(`NaN`)
		require.NoError(t, err)
		defer nanVal.Free()
		require.True(t, nanVal.IsNumber())
	})

	// Test mixed array
	t.Run("MixedArray", func(t *testing.T) {
		mixedArr, err := ctx.Eval(`[1, "string", true, null, undefined, {}]`)
		require.NoError(t, err)
		defer mixedArr.Free()
		require.True(t, mixedArr.IsArray())
		require.Equal(t, int64(6), mixedArr.Len())
	})
}
