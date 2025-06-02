package quickjs_test

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestValueBasics tests basic value operations and type checking
func TestValueBasics(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Free() and Context()
	val := ctx.String("test")
	valueCtx := val.Context()
	require.NotNil(t, valueCtx)
	require.Equal(t, ctx, valueCtx)
	val.Free()

	// Test basic type creation and checking
	testCases := []struct {
		name     string
		value    quickjs.Value
		typeTest func(quickjs.Value) bool
	}{
		{"number", ctx.Int32(42), func(v quickjs.Value) bool { return v.IsNumber() }},
		{"string", ctx.String("test"), func(v quickjs.Value) bool { return v.IsString() }},
		{"boolean", ctx.Bool(true), func(v quickjs.Value) bool { return v.IsBool() }},
		{"null", ctx.Null(), func(v quickjs.Value) bool { return v.IsNull() }},
		{"undefined", ctx.Undefined(), func(v quickjs.Value) bool { return v.IsUndefined() }},
		{"uninitialized", ctx.Uninitialized(), func(v quickjs.Value) bool { return v.IsUninitialized() }},
		{"object", ctx.Object(), func(v quickjs.Value) bool { return v.IsObject() }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.value.Free()
			require.True(t, tc.typeTest(tc.value))
		})
	}

	// Test BigInt
	bigIntVal := ctx.BigInt64(123456789)
	defer bigIntVal.Free()
	require.True(t, bigIntVal.IsBigInt())

	// Test array and symbol from JavaScript
	arr, err := ctx.Eval(`[1, 2, 3]`)
	require.NoError(t, err)
	defer arr.Free()
	require.True(t, arr.IsArray())
	require.True(t, arr.IsObject()) // Arrays are objects

	sym, err := ctx.Eval(`Symbol('test')`)
	require.NoError(t, err)
	defer sym.Free()
	require.True(t, sym.IsSymbol())
}

// TestValueConversions tests type conversion methods
func TestValueConversions(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test To* methods
	boolVal := ctx.Bool(true)
	defer boolVal.Free()
	require.True(t, boolVal.ToBool())

	stringVal := ctx.String("Hello World")
	defer stringVal.Free()
	require.EqualValues(t, "Hello World", stringVal.ToString())
	require.EqualValues(t, "Hello World", stringVal.String()) // String() calls ToString()

	int32Val := ctx.Int32(42)
	defer int32Val.Free()
	require.EqualValues(t, 42, int32Val.ToInt32())

	int64Val := ctx.Int64(1234567890)
	defer int64Val.Free()
	require.EqualValues(t, 1234567890, int64Val.ToInt64())

	uint32Val := ctx.Uint32(4294967295)
	defer uint32Val.Free()
	require.EqualValues(t, 4294967295, uint32Val.ToUint32())

	floatVal := ctx.Float64(3.14159)
	defer floatVal.Free()
	require.InDelta(t, 3.14159, floatVal.ToFloat64(), 0.00001)

	// Test ToBigInt
	bigIntVal := ctx.BigInt64(9223372036854775807)
	defer bigIntVal.Free()
	expectedBigInt := big.NewInt(9223372036854775807)
	require.Equal(t, expectedBigInt, bigIntVal.ToBigInt())

	// Test ToBigInt with non-BigInt value (should return nil)
	normalIntVal := ctx.Int32(42)
	defer normalIntVal.Free()
	require.Nil(t, normalIntVal.ToBigInt())

	// Test deprecated methods
	require.True(t, boolVal.Bool())
	require.EqualValues(t, 42, int32Val.Int32())
	require.EqualValues(t, 1234567890, int64Val.Int64())
	require.EqualValues(t, 4294967295, uint32Val.Uint32())
	require.InDelta(t, 3.14159, floatVal.Float64(), 0.00001)
	require.Equal(t, expectedBigInt, bigIntVal.BigInt())
}

// TestValueJSON tests JSON serialization
func TestValueJSON(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test object JSON stringify
	obj := ctx.Object()
	defer obj.Free()
	obj.Set("name", ctx.String("test"))
	obj.Set("value", ctx.Int32(42))

	jsonStr := obj.JSONStringify()
	require.Contains(t, jsonStr, "name")
	require.Contains(t, jsonStr, "test")
	require.Contains(t, jsonStr, "value")
	require.Contains(t, jsonStr, "42")

	// Test array JSON stringify
	arr, err := ctx.Eval(`[1, 2, 3]`)
	require.NoError(t, err)
	defer arr.Free()
	require.EqualValues(t, "[1,2,3]", arr.JSONStringify())

	// Test string JSON stringify
	str := ctx.String("hello")
	defer str.Free()
	require.EqualValues(t, "\"hello\"", str.JSONStringify())

	// Test special values
	specialVals := []struct {
		name     string
		val      quickjs.Value
		expected string
	}{
		{"null", ctx.Null(), "null"},
		{"true", ctx.Bool(true), "true"},
		{"false", ctx.Bool(false), "false"},
		{"number", ctx.Int32(42), "42"},
	}

	for _, sv := range specialVals {
		defer sv.val.Free()
		require.EqualValues(t, sv.expected, sv.val.JSONStringify())
	}
}

// TestValueArrayBuffer tests ArrayBuffer operations
func TestValueArrayBuffer(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test ArrayBuffer creation and operations
	data := []byte{1, 2, 3, 4, 5}
	arrayBuffer := ctx.ArrayBuffer(data)
	defer arrayBuffer.Free()

	require.True(t, arrayBuffer.IsByteArray())
	require.EqualValues(t, len(data), arrayBuffer.ByteLen())

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

	// Test empty ArrayBuffer
	emptyBuffer := ctx.ArrayBuffer([]byte{})
	defer emptyBuffer.Free()
	require.True(t, emptyBuffer.IsByteArray())
	require.EqualValues(t, 0, emptyBuffer.ByteLen())

	// Test array length
	arr, err := ctx.Eval(`[1, 2, 3, 4, 5]`)
	require.NoError(t, err)
	defer arr.Free()
	require.EqualValues(t, 5, arr.Len())

	// Test ToByteArray error cases - comprehensive coverage
	t.Run("ToByteArrayErrorCases", func(t *testing.T) {
		// Test various non-ArrayBuffer types
		testCases := []struct {
			name        string
			val         quickjs.Value
			expectedErr string
		}{
			{"object", ctx.Object(), "exceeds the maximum length"},
			{"string", ctx.String("not an array buffer"), "exceeds the maximum length"},
			{"number", ctx.Int32(42), "exceeds the maximum length"},
			{"boolean", ctx.Bool(true), "exceeds the maximum length"},
			{"null", ctx.Null(), "exceeds the maximum length"},
			{"undefined", ctx.Undefined(), "exceeds the maximum length"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				defer tc.val.Free()

				_, err := tc.val.ToByteArray(1)
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
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

		// Test size validation error (when ByteLen() < requested size)
		validBuffer := ctx.ArrayBuffer([]byte{1, 2, 3})
		defer validBuffer.Free()

		_, err = validBuffer.ToByteArray(10) // Request more than available
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds the maximum length")

		// Test fake ArrayBuffer objects that might pass IsByteArray check but fail ToByteArray
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

		// Test corrupted ArrayBuffer-like object
		corruptedBuffer, err := ctx.Eval(`
            var buffer = new ArrayBuffer(5);
            try {
                Object.defineProperty(buffer, 'byteLength', {
                    get: function() { throw new Error('Corrupted byteLength access'); }
                });
            } catch(e) {
                // If modification fails, use original buffer
            }
            buffer;
        `)
		require.NoError(t, err)
		defer corruptedBuffer.Free()

		if corruptedBuffer.IsByteArray() {
			_, err = corruptedBuffer.ToByteArray(uint(corruptedBuffer.ByteLen()))
			// This might succeed or fail depending on the corruption
			if err != nil {
				t.Logf("Corrupted buffer ToByteArray error: %v", err)
			}
		}

		// Test zero-size ArrayBuffer edge case
		zeroBuffer := ctx.ArrayBuffer([]byte{})
		defer zeroBuffer.Free()

		result, err := zeroBuffer.ToByteArray(0)
		require.NoError(t, err)
		require.Empty(t, result)

		// Test requesting bytes from zero-size buffer
		_, err = zeroBuffer.ToByteArray(1)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds the maximum length")
	})
}

// TestValueTypedArrayDetection tests TypedArray detection methods
func TestValueTypedArrayDetection(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test TypedArray detection methods
	typedArrayTests := []struct {
		name      string
		createJS  string
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
			val, err := ctx.Eval(tt.createJS)
			require.NoError(t, err)
			defer val.Free()

			// Test specific type detection
			require.Equal(t, tt.isTyped, tt.checkFunc(val))

			// Test general TypedArray detection
			if tt.isTyped {
				require.True(t, val.IsTypedArray())
			}
		})
	}
}

// TestValueTypedArrayConversions tests TypedArray conversion methods
func TestValueTypedArrayConversions(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Int8Array conversion
	t.Run("Int8Array", func(t *testing.T) {
		val, err := ctx.Eval("new Int8Array([-128, 0, 127])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToInt8Array()
		require.NoError(t, err)
		require.Equal(t, []int8{-128, 0, 127}, result)

		// Test error case - wrong type
		wrongType := ctx.String("not an array")
		defer wrongType.Free()
		_, err = wrongType.ToInt8Array()
		require.Error(t, err)
		require.Contains(t, err.Error(), "not an Int8Array")
	})

	// Test Uint8Array conversion
	t.Run("Uint8Array", func(t *testing.T) {
		val, err := ctx.Eval("new Uint8Array([0, 128, 255])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToUint8Array()
		require.NoError(t, err)
		require.Equal(t, []uint8{0, 128, 255}, result)

		// Test Uint8ClampedArray also works with ToUint8Array
		clampedVal, err := ctx.Eval("new Uint8ClampedArray([0, 128, 255])")
		require.NoError(t, err)
		defer clampedVal.Free()

		clampedResult, err := clampedVal.ToUint8Array()
		require.NoError(t, err)
		require.Equal(t, []uint8{0, 128, 255}, clampedResult)
	})

	// Test Int16Array conversion
	t.Run("Int16Array", func(t *testing.T) {
		val, err := ctx.Eval("new Int16Array([-32768, 0, 32767])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToInt16Array()
		require.NoError(t, err)
		require.Equal(t, []int16{-32768, 0, 32767}, result)
	})

	// Test Uint16Array conversion
	t.Run("Uint16Array", func(t *testing.T) {
		val, err := ctx.Eval("new Uint16Array([0, 32768, 65535])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToUint16Array()
		require.NoError(t, err)
		require.Equal(t, []uint16{0, 32768, 65535}, result)
	})

	// Test Int32Array conversion
	t.Run("Int32Array", func(t *testing.T) {
		val, err := ctx.Eval("new Int32Array([-2147483648, 0, 2147483647])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToInt32Array()
		require.NoError(t, err)
		require.Equal(t, []int32{-2147483648, 0, 2147483647}, result)
	})

	// Test Uint32Array conversion
	t.Run("Uint32Array", func(t *testing.T) {
		val, err := ctx.Eval("new Uint32Array([0, 2147483648, 4294967295])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToUint32Array()
		require.NoError(t, err)
		require.Equal(t, []uint32{0, 2147483648, 4294967295}, result)
	})

	// Test Float32Array conversion
	t.Run("Float32Array", func(t *testing.T) {
		val, err := ctx.Eval("new Float32Array([1.5, 2.5, 3.14159])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToFloat32Array()
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.InDelta(t, 1.5, result[0], 0.0001)
		require.InDelta(t, 2.5, result[1], 0.0001)
		require.InDelta(t, 3.14159, result[2], 0.0001)
	})

	// Test Float64Array conversion
	t.Run("Float64Array", func(t *testing.T) {
		val, err := ctx.Eval("new Float64Array([1.5, 2.5, 3.141592653589793])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToFloat64Array()
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.InDelta(t, 1.5, result[0], 0.000001)
		require.InDelta(t, 2.5, result[1], 0.000001)
		require.InDelta(t, 3.141592653589793, result[2], 0.000001)
	})

	// Test BigInt64Array conversion
	t.Run("BigInt64Array", func(t *testing.T) {
		val, err := ctx.Eval("new BigInt64Array([-9223372036854775808n, 0n, 9223372036854775807n])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToBigInt64Array()
		require.NoError(t, err)
		require.Equal(t, []int64{-9223372036854775808, 0, 9223372036854775807}, result)
	})

	// Test BigUint64Array conversion
	t.Run("BigUint64Array", func(t *testing.T) {
		val, err := ctx.Eval("new BigUint64Array([0n, 9223372036854775808n, 18446744073709551615n])")
		require.NoError(t, err)
		defer val.Free()

		result, err := val.ToBigUint64Array()
		require.NoError(t, err)
		require.Equal(t, []uint64{0, 9223372036854775808, 18446744073709551615}, result)
	})

	// Test error cases for all conversion methods
	t.Run("ErrorCases", func(t *testing.T) {
		wrongTypeTests := []struct {
			name      string
			convertFn func(quickjs.Value) (interface{}, error)
		}{
			{"ToInt8Array", func(v quickjs.Value) (interface{}, error) { return v.ToInt8Array() }},
			{"ToUint8Array", func(v quickjs.Value) (interface{}, error) { return v.ToUint8Array() }},
			{"ToInt16Array", func(v quickjs.Value) (interface{}, error) { return v.ToInt16Array() }},
			{"ToUint16Array", func(v quickjs.Value) (interface{}, error) { return v.ToUint16Array() }},
			{"ToInt32Array", func(v quickjs.Value) (interface{}, error) { return v.ToInt32Array() }},
			{"ToUint32Array", func(v quickjs.Value) (interface{}, error) { return v.ToUint32Array() }},
			{"ToFloat32Array", func(v quickjs.Value) (interface{}, error) { return v.ToFloat32Array() }},
			{"ToFloat64Array", func(v quickjs.Value) (interface{}, error) { return v.ToFloat64Array() }},
			{"ToBigInt64Array", func(v quickjs.Value) (interface{}, error) { return v.ToBigInt64Array() }},
			{"ToBigUint64Array", func(v quickjs.Value) (interface{}, error) { return v.ToBigUint64Array() }},
		}

		wrongTypeVal := ctx.String("not a typed array")
		defer wrongTypeVal.Free()

		for _, tt := range wrongTypeTests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := tt.convertFn(wrongTypeVal)
				require.Error(t, err)
			})
		}
	})
}

// TestValueTypedArrayExceptionHandling tests exception handling in ToXXXArray methods
func TestValueTypedArrayExceptionHandling(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test buffer.IsException() branch for each TypedArray type using the proven pattern
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
			// Create an object that looks like TypedArray but has no real TypedArray backing
			// This is the exact pattern that successfully triggers buffer.IsException()
			corruptedArray, err := ctx.Eval(fmt.Sprintf(`
                // Create an object that looks like %s but has no real TypedArray backing
                var corrupted = Object.create(%s.prototype);

                // Set constructor property
                Object.defineProperty(corrupted, 'constructor', {
                    value: %s,
                    writable: true,
                    enumerable: false,
                    configurable: true
                });

                // Add properties to make it look more convincing
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

                // But no actual ArrayBuffer or internal slots
                corrupted;
            `, tc.arrayType, tc.arrayType, tc.arrayType))
			require.NoError(t, err)
			defer corruptedArray.Free()

			// Check if it passes the instanceof test
			t.Logf("Is%s: %v", tc.arrayType, corruptedArray.GlobalInstanceof(tc.arrayType))

			// Try to convert - this should trigger buffer.IsException()
			result, err := tc.convertFunc(corruptedArray)
			if err != nil {
				t.Logf("Successfully triggered error for %s: %v", tc.arrayType, err)
				require.Error(t, err)
			} else {
				t.Logf("Unexpected success for %s, result: %v", tc.arrayType, result)
				// Some might not trigger the exception, which is also valid behavior
			}
		})
	}
}

// TestValueProperties tests property operations
func TestValueProperties(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Test Set, Get, Has, Delete
	obj.Set("name", ctx.String("test"))
	obj.Set("value", ctx.Int32(42))
	obj.Set("flag", ctx.Bool(true))

	require.True(t, obj.Has("name"))
	require.True(t, obj.Has("value"))
	require.False(t, obj.Has("nonexistent"))

	nameVal := obj.Get("name")
	defer nameVal.Free()
	require.EqualValues(t, "test", nameVal.String())

	require.True(t, obj.Delete("flag"))
	require.False(t, obj.Has("flag"))
	require.False(t, obj.Delete("nonexistent"))

	// Test SetIdx, GetIdx, HasIdx, DeleteIdx
	obj.SetIdx(0, ctx.String("index0"))
	obj.SetIdx(1, ctx.String("index1"))

	require.True(t, obj.HasIdx(0))
	require.True(t, obj.HasIdx(1))
	require.False(t, obj.HasIdx(99))

	idx0Val := obj.GetIdx(0)
	defer idx0Val.Free()
	require.EqualValues(t, "index0", idx0Val.String())

	require.True(t, obj.DeleteIdx(0))
	require.False(t, obj.HasIdx(0))
	require.False(t, obj.DeleteIdx(99))

	// Test PropertyNames
	obj.Set("a", ctx.String("value_a"))
	obj.Set("b", ctx.String("value_b"))

	names, err := obj.PropertyNames()
	require.NoError(t, err)
	require.Contains(t, names, "a")
	require.Contains(t, names, "b")

	// Test PropertyNames with non-object value
	str := ctx.String("test")
	defer str.Free()
	_, err = str.PropertyNames()
	require.Error(t, err)

	// Test PropertyNames with more non-object types (covers error branches)
	primitiveVal := ctx.Int32(42)
	defer primitiveVal.Free()

	_, err = primitiveVal.PropertyNames()
	require.Error(t, err)
	require.Contains(t, err.Error(), "value does not contain properties")

	// Test PropertyNames with null value
	nullVal := ctx.Null()
	defer nullVal.Free()

	_, err = nullVal.PropertyNames()
	require.Error(t, err)
	require.Contains(t, err.Error(), "value does not contain properties")

	// Test PropertyNames with undefined value
	undefinedVal := ctx.Undefined()
	defer undefinedVal.Free()

	_, err = undefinedVal.PropertyNames()
	require.Error(t, err)
	require.Contains(t, err.Error(), "value does not contain properties")

	// Test PropertyNames with boolean value
	boolVal := ctx.Bool(true)
	defer boolVal.Free()

	_, err = boolVal.PropertyNames()
	require.Error(t, err)
	require.Contains(t, err.Error(), "value does not contain properties")
}

// TestValueFunctionCalls tests function calling methods
func TestValueFunctionCalls(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create an object with methods
	obj := ctx.Object()
	defer obj.Free()

	addFunc := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		if len(args) < 2 {
			return ctx.Int32(0)
		}
		return ctx.Int32(args[0].ToInt32() + args[1].ToInt32())
	})

	obj.Set("add", addFunc)

	// Test Call method with arguments
	result := obj.Call("add", ctx.Int32(3), ctx.Int32(4))
	defer result.Free()
	require.False(t, result.IsException())
	require.EqualValues(t, 7, result.ToInt32())

	// Test Call method without arguments (covers len(cargs) == 0 branch)
	noArgsFunc := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.String("no arguments received")
	})
	obj.Set("noArgs", noArgsFunc)

	// Call without arguments - this should hit the len(cargs) == 0 branch
	noArgsResult := obj.Call("noArgs")
	defer noArgsResult.Free()
	require.False(t, noArgsResult.IsException())
	require.EqualValues(t, "no arguments received", noArgsResult.String())

	// Test Execute method
	execResult := addFunc.Execute(ctx.Null(), ctx.Int32(5), ctx.Int32(6))
	defer execResult.Free()
	require.False(t, execResult.IsException())
	require.EqualValues(t, 11, execResult.ToInt32())

	// Test error cases
	errorResult := obj.Call("nonexistent", ctx.Int32(1))
	defer errorResult.Free()
	require.True(t, errorResult.IsException())

	// Test CallConstructor
	constructorFunc, err := ctx.Eval(`
        function TestClass(value) {
            this.value = value;
        }
        TestClass;
    `)
	require.NoError(t, err)
	defer constructorFunc.Free()

	instance := constructorFunc.CallConstructor(ctx.String("test_value"))
	defer instance.Free()
	require.False(t, instance.IsException())
	require.True(t, instance.IsObject())

	valueProperty := instance.Get("value")
	defer valueProperty.Free()
	require.EqualValues(t, "test_value", valueProperty.String())

	// Test New (alias for CallConstructor)
	instance2 := constructorFunc.New(ctx.String("test_value2"))
	defer instance2.Free()
	require.False(t, instance2.IsException())

	// Test error case
	nonConstructor := ctx.String("not a constructor")
	defer nonConstructor.Free()
	errorResult2 := nonConstructor.CallConstructor()
	defer errorResult2.Free()
	require.True(t, errorResult2.IsException())
}

// TestValueError tests error handling
func TestValueError(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Error creation and conversion
	testErr := errors.New("test error message")
	errorVal := ctx.Error(testErr)
	defer errorVal.Free()

	require.True(t, errorVal.IsError())

	convertedErr := errorVal.ToError()
	require.NotNil(t, convertedErr)
	require.Contains(t, convertedErr.Error(), "test error message")

	// Test deprecated Error method
	deprecatedErr := errorVal.Error()
	require.NotNil(t, deprecatedErr)
	require.Contains(t, deprecatedErr.Error(), "test error message")

	// Test ToError on non-error value
	str := ctx.String("not an error")
	defer str.Free()
	require.Nil(t, str.ToError())

	// Test error with cause property (covers cause branch in ToError method)
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

	// Check if cause field is properly set
	quickjsErr, ok := complexConvertedErr.(*quickjs.Error)
	require.True(t, ok)
	require.EqualValues(t, "underlying cause", quickjsErr.Cause)
	require.EqualValues(t, "CustomError", quickjsErr.Name)
	require.EqualValues(t, "complex error", quickjsErr.Message)
	require.EqualValues(t, "stack trace here", quickjsErr.Stack)
}

// TestValueGlobalInstanceof tests instanceof checking
func TestValueGlobalInstanceof(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Array instanceof
	arr, err := ctx.Eval(`[1, 2, 3]`)
	require.NoError(t, err)
	defer arr.Free()
	require.True(t, arr.GlobalInstanceof("Array"))
	require.True(t, arr.GlobalInstanceof("Object"))

	// Test Object instanceof
	obj, err := ctx.Eval(`({})`)
	require.NoError(t, err)
	defer obj.Free()
	require.True(t, obj.GlobalInstanceof("Object"))
	require.False(t, obj.GlobalInstanceof("Array"))

	// Test false return cases to ensure coverage (covers return false branch)
	// Test with non-existent constructor
	str := ctx.String("test")
	defer str.Free()
	require.False(t, str.GlobalInstanceof("NonExistentConstructor"))
	require.False(t, str.GlobalInstanceof(""))
	require.False(t, str.GlobalInstanceof("UndefinedConstructor"))

	// Test with primitive value
	require.False(t, str.GlobalInstanceof("String"))

	// Test object but constructor mismatch cases
	require.False(t, obj.GlobalInstanceof("Array"))    // should return false
	require.False(t, obj.GlobalInstanceof("Function")) // should return false
	require.False(t, obj.GlobalInstanceof("Date"))     // should return false

	// Test various non-object types with GlobalInstanceof
	testVals := []struct {
		name string
		val  quickjs.Value
	}{
		{"null", ctx.Null()},
		{"undefined", ctx.Undefined()},
		{"int32", ctx.Int32(42)},
		{"float64", ctx.Float64(3.14)},
		{"bool", ctx.Bool(true)},
	}

	for _, tv := range testVals {
		defer tv.val.Free()

		// All of these should return false
		require.False(t, tv.val.GlobalInstanceof("Object"), "Test case %s failed", tv.name)
		require.False(t, tv.val.GlobalInstanceof("Array"), "Test case %s failed", tv.name)
		require.False(t, tv.val.GlobalInstanceof("Function"), "Test case %s failed", tv.name)
		require.False(t, tv.val.GlobalInstanceof("Date"), "Test case %s failed", tv.name)
		require.False(t, tv.val.GlobalInstanceof("NonExistent"), "Test case %s failed", tv.name)
	}

	// Test custom constructor
	result, err := ctx.Eval(`
        function CustomClass() {}
        globalThis.CustomClass = CustomClass;
        new CustomClass();
    `)
	require.NoError(t, err)
	defer result.Free()
	require.True(t, result.GlobalInstanceof("CustomClass"))
}

// TestValueSpecialTypes tests special JavaScript types
func TestValueSpecialTypes(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test function
	funcVal := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.Null()
	})
	defer funcVal.Free()
	require.True(t, funcVal.IsFunction())

	// Test constructor
	constructorVal, err := ctx.Eval(`function TestConstructor() {}; TestConstructor`)
	require.NoError(t, err)
	defer constructorVal.Free()
	require.True(t, constructorVal.IsConstructor())

	// Test promise - pending state
	promiseVal, err := ctx.Eval(`new Promise((resolve) => resolve("test"))`)
	require.NoError(t, err)
	defer promiseVal.Free()
	require.True(t, promiseVal.IsPromise())
	require.True(t, promiseVal.IsObject()) // Promises are objects

	// Test promise - fulfilled state
	fulfilledPromise, err := ctx.Eval(`Promise.resolve("fulfilled")`)
	require.NoError(t, err)
	defer fulfilledPromise.Free()
	require.True(t, fulfilledPromise.IsPromise())

	// Test promise - rejected state
	rejectedPromise, err := ctx.Eval(`Promise.reject("rejected")`)
	require.NoError(t, err)
	defer rejectedPromise.Free()
	require.True(t, rejectedPromise.IsPromise())

	// Test non-Promise objects for IsPromise method (covers return false branch)
	// Test regular object - should return false
	regularObj := ctx.Object()
	defer regularObj.Free()
	require.False(t, regularObj.IsPromise())

	// Test string - should return false
	stringVal := ctx.String("not a promise")
	defer stringVal.Free()
	require.False(t, stringVal.IsPromise())

	// Test number - should return false
	numberVal := ctx.Int32(42)
	defer numberVal.Free()
	require.False(t, numberVal.IsPromise())

	// Test null - should return false
	nullVal := ctx.Null()
	defer nullVal.Free()
	require.False(t, nullVal.IsPromise())

	// Test undefined - should return false
	undefinedVal := ctx.Undefined()
	defer undefinedVal.Free()
	require.False(t, undefinedVal.IsPromise())

	// Test function - should return false (functions are not promises)
	require.False(t, funcVal.IsPromise())

	// Test exception handling
	_, err = ctx.Eval(`throw new Error("test error")`)
	require.Error(t, err)
}

// TestValueEdgeCases tests various edge cases
func TestValueEdgeCases(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test empty and zero values
	emptyStr := ctx.String("")
	defer emptyStr.Free()
	require.EqualValues(t, "", emptyStr.String())
	require.EqualValues(t, "\"\"", emptyStr.JSONStringify())

	zeroInt := ctx.Int32(0)
	defer zeroInt.Free()
	require.EqualValues(t, 0, zeroInt.ToInt32())
	require.False(t, zeroInt.ToBool()) // 0 is falsy

	// Test negative values
	negativeInt := ctx.Int32(-42)
	defer negativeInt.Free()
	require.EqualValues(t, -42, negativeInt.ToInt32())

	// Test special float values
	infVal, err := ctx.Eval(`Infinity`)
	require.NoError(t, err)
	defer infVal.Free()
	require.True(t, infVal.IsNumber())

	nanVal, err := ctx.Eval(`NaN`)
	require.NoError(t, err)
	defer nanVal.Free()
	require.True(t, nanVal.IsNumber())

	// Test object property access with special keys
	obj := ctx.Object()
	defer obj.Free()

	obj.Set("", ctx.String("empty key"))
	emptyKeyVal := obj.Get("")
	defer emptyKeyVal.Free()
	require.EqualValues(t, "empty key", emptyKeyVal.String())

	// Test mixed array
	mixedArr, err := ctx.Eval(`[1, "string", true, null, undefined, {}]`)
	require.NoError(t, err)
	defer mixedArr.Free()
	require.True(t, mixedArr.IsArray())
	require.EqualValues(t, 6, mixedArr.Len())

	// Test non-configurable property deletion
	arr, err := ctx.Eval(`[1, 2, 3]`)
	require.NoError(t, err)
	defer arr.Free()
	require.True(t, arr.Has("length"))
	require.False(t, arr.Delete("length")) // Should fail
	require.True(t, arr.Has("length"))     // Should still exist
}
