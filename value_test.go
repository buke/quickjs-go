package quickjs

import (
	"errors"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValueBasics tests basic value creation and type checking
func TestValueBasics(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic type creation and checking
	testCases := []struct {
		name      string
		createVal func() Value
		checkFunc func(Value) bool
	}{
		{"Number", func() Value { return ctx.Int32(42) }, func(v Value) bool { return v.IsNumber() }},
		{"String", func() Value { return ctx.String("test") }, func(v Value) bool { return v.IsString() }},
		{"Boolean", func() Value { return ctx.Bool(true) }, func(v Value) bool { return v.IsBool() }},
		{"Null", func() Value { return ctx.Null() }, func(v Value) bool { return v.IsNull() }},
		{"Undefined", func() Value { return ctx.Undefined() }, func(v Value) bool { return v.IsUndefined() }},
		{"Uninitialized", func() Value { return ctx.Uninitialized() }, func(v Value) bool { return v.IsUninitialized() }},
		{"Object", func() Value { return ctx.Object() }, func(v Value) bool { return v.IsObject() }},
		{"BigInt", func() Value { return ctx.BigInt64(123456789) }, func(v Value) bool { return v.IsBigInt() }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val := tc.createVal()
			defer val.Free()
			require.True(t, tc.checkFunc(val))
			require.Equal(t, ctx, val.Context()) // Test Context() method
		})
	}

	// Test JavaScript created values
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

// TestValueConversions tests type conversions including deprecated methods
func TestValueConversions(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic conversions
	tests := []struct {
		name           string
		createVal      func() Value
		testFunc       func(Value)
		testDeprecated func(Value) // Test deprecated methods for coverage
	}{
		{
			name:           "Bool",
			createVal:      func() Value { return ctx.Bool(true) },
			testFunc:       func(v Value) { require.True(t, v.ToBool()) },
			testDeprecated: func(v Value) { require.True(t, v.Bool()) },
		},
		{
			name:      "String",
			createVal: func() Value { return ctx.String("Hello") },
			testFunc: func(v Value) {
				require.Equal(t, "Hello", v.ToString())
				require.Equal(t, "Hello", v.String()) // String() calls ToString()
			},
		},
		{
			name:           "Int32",
			createVal:      func() Value { return ctx.Int32(42) },
			testFunc:       func(v Value) { require.Equal(t, int32(42), v.ToInt32()) },
			testDeprecated: func(v Value) { require.Equal(t, int32(42), v.Int32()) },
		},
		{
			name:           "Int64",
			createVal:      func() Value { return ctx.Int64(1234567890) },
			testFunc:       func(v Value) { require.Equal(t, int64(1234567890), v.ToInt64()) },
			testDeprecated: func(v Value) { require.Equal(t, int64(1234567890), v.Int64()) },
		},
		{
			name:           "Uint32",
			createVal:      func() Value { return ctx.Uint32(4294967295) },
			testFunc:       func(v Value) { require.Equal(t, uint32(4294967295), v.ToUint32()) },
			testDeprecated: func(v Value) { require.Equal(t, uint32(4294967295), v.Uint32()) },
		},
		{
			name:           "Float64",
			createVal:      func() Value { return ctx.Float64(3.14159) },
			testFunc:       func(v Value) { require.InDelta(t, 3.14159, v.ToFloat64(), 0.00001) },
			testDeprecated: func(v Value) { require.InDelta(t, 3.14159, v.Float64(), 0.00001) },
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
	bigIntVal := ctx.BigInt64(9223372036854775807)
	defer bigIntVal.Free()
	expectedBigInt := big.NewInt(9223372036854775807)
	require.Equal(t, expectedBigInt, bigIntVal.ToBigInt())
	require.Equal(t, expectedBigInt, bigIntVal.BigInt()) // Deprecated method

	// Test ToBigInt with non-BigInt value (should return nil)
	normalIntVal := ctx.Int32(42)
	defer normalIntVal.Free()
	require.Nil(t, normalIntVal.ToBigInt())
}

// TestValueJSON tests JSON operations
func TestValueJSON(t *testing.T) {
	rt := NewRuntime()
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
	require.Contains(t, jsonStr, "42")

	// Test various value types
	testCases := []struct {
		name      string
		createVal func() Value
		expected  string
	}{
		{"String", func() Value { return ctx.String("hello") }, `"hello"`},
		{"Null", func() Value { return ctx.Null() }, "null"},
		{"True", func() Value { return ctx.Bool(true) }, "true"},
		{"False", func() Value { return ctx.Bool(false) }, "false"},
		{"Number", func() Value { return ctx.Int32(42) }, "42"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val := tc.createVal()
			defer val.Free()
			require.Equal(t, tc.expected, val.JSONStringify())
		})
	}
}

// TestValueArrayBuffer tests ArrayBuffer operations
func TestValueArrayBuffer(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic ArrayBuffer operations
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

	// Test array length
	arr, err := ctx.Eval(`[1, 2, 3, 4, 5]`)
	require.NoError(t, err)
	defer arr.Free()
	require.Equal(t, int64(5), arr.Len())

	// Test error cases with non-ArrayBuffer types
	errorTests := []struct {
		name      string
		createVal func() Value
	}{
		{"Object", func() Value { return ctx.Object() }},
		{"String", func() Value { return ctx.String("not an array buffer") }},
		{"Number", func() Value { return ctx.Int32(42) }},
		{"Null", func() Value { return ctx.Null() }},
	}

	for _, tt := range errorTests {
		t.Run(tt.name+"Error", func(t *testing.T) {
			val := tt.createVal()
			defer val.Free()
			_, err := val.ToByteArray(1)
			require.Error(t, err)
		})
	}
}

// TestValueTypedArrays tests TypedArray detection and conversion
func TestValueTypedArrays(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test TypedArray detection
	typedArrayTests := []struct {
		name      string
		jsCode    string
		checkFunc func(Value) bool
		isTyped   bool
	}{
		{"Int8Array", "new Int8Array([1, 2, 3])", func(v Value) bool { return v.IsInt8Array() }, true},
		{"Uint8Array", "new Uint8Array([1, 2, 3])", func(v Value) bool { return v.IsUint8Array() }, true},
		{"Uint8ClampedArray", "new Uint8ClampedArray([1, 2, 3])", func(v Value) bool { return v.IsUint8ClampedArray() }, true},
		{"Int16Array", "new Int16Array([1, 2, 3])", func(v Value) bool { return v.IsInt16Array() }, true},
		{"Uint16Array", "new Uint16Array([1, 2, 3])", func(v Value) bool { return v.IsUint16Array() }, true},
		{"Int32Array", "new Int32Array([1, 2, 3])", func(v Value) bool { return v.IsInt32Array() }, true},
		{"Uint32Array", "new Uint32Array([1, 2, 3])", func(v Value) bool { return v.IsUint32Array() }, true},
		{"Float32Array", "new Float32Array([1.5, 2.5, 3.5])", func(v Value) bool { return v.IsFloat32Array() }, true},
		{"Float64Array", "new Float64Array([1.5, 2.5, 3.5])", func(v Value) bool { return v.IsFloat64Array() }, true},
		{"BigInt64Array", "new BigInt64Array([1n, 2n, 3n])", func(v Value) bool { return v.IsBigInt64Array() }, true},
		{"BigUint64Array", "new BigUint64Array([1n, 2n, 3n])", func(v Value) bool { return v.IsBigUint64Array() }, true},
		{"RegularArray", "[1, 2, 3]", func(v Value) bool { return v.IsInt8Array() }, false},
	}

	for _, tt := range typedArrayTests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := ctx.Eval(tt.jsCode)
			require.NoError(t, err)
			defer val.Free()

			require.Equal(t, tt.isTyped, tt.checkFunc(val))
			if tt.isTyped {
				require.True(t, val.IsTypedArray())
			} else {
				require.False(t, val.IsTypedArray())
			}
		})
	}

	// Test TypedArray conversions with selected key types
	conversionTests := []struct {
		name        string
		jsCode      string
		convertFunc func(Value) (interface{}, error)
		expected    interface{}
	}{
		{
			name:        "Int8Array",
			jsCode:      "new Int8Array([-128, 0, 127])",
			convertFunc: func(v Value) (interface{}, error) { return v.ToInt8Array() },
			expected:    []int8{-128, 0, 127},
		},
		{
			name:        "Uint8Array",
			jsCode:      "new Uint8Array([0, 128, 255])",
			convertFunc: func(v Value) (interface{}, error) { return v.ToUint8Array() },
			expected:    []uint8{0, 128, 255},
		},
		{
			name:        "Int32Array",
			jsCode:      "new Int32Array([-2147483648, 0, 2147483647])",
			convertFunc: func(v Value) (interface{}, error) { return v.ToInt32Array() },
			expected:    []int32{-2147483648, 0, 2147483647},
		},
		{
			name:        "Float32Array",
			jsCode:      "new Float32Array([1.5, 2.5, 3.14159])",
			convertFunc: func(v Value) (interface{}, error) { return v.ToFloat32Array() },
			expected:    []float32{1.5, 2.5, 3.14159},
		},
		{
			name:        "BigInt64Array",
			jsCode:      "new BigInt64Array([-9223372036854775808n, 0n, 9223372036854775807n])",
			convertFunc: func(v Value) (interface{}, error) { return v.ToBigInt64Array() },
			expected:    []int64{-9223372036854775808, 0, 9223372036854775807},
		},
	}

	for _, tt := range conversionTests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := ctx.Eval(tt.jsCode)
			require.NoError(t, err)
			defer val.Free()

			result, err := tt.convertFunc(val)
			require.NoError(t, err)

			if tt.name == "Float32Array" {
				resultSlice := result.([]float32)
				expectedSlice := tt.expected.([]float32)
				require.Len(t, resultSlice, len(expectedSlice))
				for i, exp := range expectedSlice {
					require.InDelta(t, exp, resultSlice[i], 0.0001)
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

	// Test remaining conversion methods for coverage
	additionalTests := []struct {
		name   string
		jsCode string
		testFn func(Value)
	}{
		{"Uint8ClampedArray", "new Uint8ClampedArray([0, 128, 255])", func(v Value) {
			result, err := v.ToUint8Array() // Uint8ClampedArray uses same method
			require.NoError(t, err)
			require.Equal(t, []uint8{0, 128, 255}, result)
		}},
		{"Uint16Array", "new Uint16Array([0, 32768, 65535])", func(v Value) {
			result, err := v.ToUint16Array()
			require.NoError(t, err)
			require.Equal(t, []uint16{0, 32768, 65535}, result)
		}},
		{"Int16Array", "new Int16Array([-32768, 0, 32767])", func(v Value) {
			result, err := v.ToInt16Array()
			require.NoError(t, err)
			require.Equal(t, []int16{-32768, 0, 32767}, result)
		}},
		{"Uint32Array", "new Uint32Array([0, 2147483648, 4294967295])", func(v Value) {
			result, err := v.ToUint32Array()
			require.NoError(t, err)
			require.Equal(t, []uint32{0, 2147483648, 4294967295}, result)
		}},
		{"Float64Array", "new Float64Array([1.5, 2.5, 3.141592653589793])", func(v Value) {
			result, err := v.ToFloat64Array()
			require.NoError(t, err)
			expected := []float64{1.5, 2.5, 3.141592653589793}
			require.Len(t, result, len(expected))
			for i, exp := range expected {
				require.InDelta(t, exp, result[i], 1e-10)
			}
		}},
		{"BigUint64Array", "new BigUint64Array([0n, 9223372036854775808n, 18446744073709551615n])", func(v Value) {
			result, err := v.ToBigUint64Array()
			require.NoError(t, err)
			require.Equal(t, []uint64{0, 9223372036854775808, 18446744073709551615}, result)
		}},
	}

	for _, tt := range additionalTests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := ctx.Eval(tt.jsCode)
			require.NoError(t, err)
			defer val.Free()
			tt.testFn(val)
		})
	}
}

// TestValueProperties tests property operations
func TestValueProperties(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Test basic property operations
	obj.Set("name", ctx.String("test"))
	obj.Set("value", ctx.Int32(42))

	require.True(t, obj.Has("name"))
	require.False(t, obj.Has("nonexistent"))

	nameVal := obj.Get("name")
	defer nameVal.Free()
	require.Equal(t, "test", nameVal.String())

	require.True(t, obj.Delete("value"))
	require.False(t, obj.Delete("nonexistent"))

	// Test indexed operations
	obj.SetIdx(0, ctx.String("index0"))
	require.True(t, obj.HasIdx(0))
	require.False(t, obj.HasIdx(99))

	idx0Val := obj.GetIdx(0)
	defer idx0Val.Free()
	require.Equal(t, "index0", idx0Val.String())

	require.True(t, obj.DeleteIdx(0))
	require.False(t, obj.DeleteIdx(99))

	// Test PropertyNames
	obj.Set("a", ctx.String("value_a"))
	obj.Set("b", ctx.String("value_b"))

	names, err := obj.PropertyNames()
	require.NoError(t, err)
	require.Contains(t, names, "a")
	require.Contains(t, names, "b")

	// Test PropertyNames error case
	str := ctx.String("test")
	defer str.Free()
	_, err = str.PropertyNames()
	require.Error(t, err)
	require.Contains(t, err.Error(), "value does not contain properties")
}

// TestValueFunctionCalls tests function calls and constructors
func TestValueFunctionCalls(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Test function calls
	addFunc := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
		if len(args) < 2 {
			return ctx.Int32(0)
		}
		return ctx.Int32(args[0].ToInt32() + args[1].ToInt32())
	})
	obj.Set("add", addFunc)

	// Call with arguments
	result := obj.Call("add", ctx.Int32(3), ctx.Int32(4))
	defer result.Free()
	require.Equal(t, int32(7), result.ToInt32())

	// Call without arguments (covers len(cargs) == 0 branch)
	noArgsFunc := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
		return ctx.String("no arguments")
	})
	obj.Set("noArgs", noArgsFunc)

	noArgsResult := obj.Call("noArgs")
	defer noArgsResult.Free()
	require.Equal(t, "no arguments", noArgsResult.String())

	// Execute method
	execResult := addFunc.Execute(ctx.Null(), ctx.Int32(5), ctx.Int32(6))
	defer execResult.Free()
	require.Equal(t, int32(11), execResult.ToInt32())

	// Test constructors
	constructorFunc, err := ctx.Eval(`
        function TestClass(value) {
            this.value = value;
        }
        TestClass;
    `)
	require.NoError(t, err)
	defer constructorFunc.Free()

	// CallConstructor with arguments
	instance := constructorFunc.CallConstructor(ctx.String("test_value"))
	defer instance.Free()
	require.True(t, instance.IsObject())

	// New (alias for CallConstructor) without arguments
	instance2 := constructorFunc.New()
	defer instance2.Free()
	require.True(t, instance2.IsObject())
}

// TestValueError tests error handling
func TestValueError(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test error creation and conversion
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

	// Test ToError on non-error value
	str := ctx.String("not an error")
	defer str.Free()
	require.Nil(t, str.ToError())

	// Test complex error with all properties
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

	quickjsErr, ok := complexConvertedErr.(*Error)
	require.True(t, ok)
	require.Equal(t, "underlying cause", quickjsErr.Cause)
	require.Equal(t, "CustomError", quickjsErr.Name)
	require.Equal(t, "complex error", quickjsErr.Message)
	require.Equal(t, "stack trace here", quickjsErr.Stack)
}

// TestValueInstanceof tests instanceof operations
func TestValueInstanceof(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test valid instanceof cases
	arr, err := ctx.Eval(`[1, 2, 3]`)
	require.NoError(t, err)
	defer arr.Free()
	require.True(t, arr.GlobalInstanceof("Array"))
	require.True(t, arr.GlobalInstanceof("Object"))

	obj, err := ctx.Eval(`({})`)
	require.NoError(t, err)
	defer obj.Free()
	require.True(t, obj.GlobalInstanceof("Object"))
	require.False(t, obj.GlobalInstanceof("Array"))

	// Test false cases to ensure coverage
	testVals := []struct {
		name      string
		createVal func() Value
	}{
		{"String", func() Value { return ctx.String("test") }},
		{"Number", func() Value { return ctx.Int32(42) }},
		{"Null", func() Value { return ctx.Null() }},
		{"Undefined", func() Value { return ctx.Undefined() }},
	}

	for _, tv := range testVals {
		t.Run(tv.name, func(t *testing.T) {
			val := tv.createVal()
			defer val.Free()
			require.False(t, val.GlobalInstanceof("Array"))
			require.False(t, val.GlobalInstanceof("NonExistent"))
			require.False(t, val.GlobalInstanceof(""))
		})
	}
}

// TestValueSpecialTypes tests special types and edge cases
func TestValueSpecialTypes(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test function
	funcVal := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
		return ctx.Null()
	})
	defer funcVal.Free()
	require.True(t, funcVal.IsFunction())
	require.False(t, funcVal.IsPromise()) // Functions are not promises

	// Test constructor
	constructorVal, err := ctx.Eval(`function TestConstructor() {}; TestConstructor`)
	require.NoError(t, err)
	defer constructorVal.Free()
	require.True(t, constructorVal.IsConstructor())

	// Test promises
	promiseTests := []struct {
		name   string
		jsCode string
	}{
		{"Pending", `new Promise(() => {})`},
		{"Fulfilled", `Promise.resolve("fulfilled")`},
		{"Rejected", `Promise.reject("rejected")`},
	}

	for _, tt := range promiseTests {
		t.Run(tt.name, func(t *testing.T) {
			promiseVal, err := ctx.Eval(tt.jsCode)
			require.NoError(t, err)
			defer promiseVal.Free()
			require.True(t, promiseVal.IsPromise())
		})
	}

	// Test non-Promise objects for IsPromise method (covers return false branch)
	nonPromiseTests := []struct {
		name      string
		createVal func() Value
	}{
		{"Object", func() Value { return ctx.Object() }},
		{"String", func() Value { return ctx.String("not a promise") }},
		{"Number", func() Value { return ctx.Int32(42) }},
	}

	for _, tt := range nonPromiseTests {
		t.Run(tt.name+"NotPromise", func(t *testing.T) {
			val := tt.createVal()
			defer val.Free()
			require.False(t, val.IsPromise())
		})
	}

	// Test edge cases
	emptyStr := ctx.String("")
	defer emptyStr.Free()
	require.Equal(t, "", emptyStr.String())
	require.Equal(t, `""`, emptyStr.JSONStringify())

	zeroInt := ctx.Int32(0)
	defer zeroInt.Free()
	require.False(t, zeroInt.ToBool()) // 0 is falsy

	// Test special float values
	infVal, err := ctx.Eval(`Infinity`)
	require.NoError(t, err)
	defer infVal.Free()
	require.True(t, infVal.IsNumber())

	nanVal, err := ctx.Eval(`NaN`)
	require.NoError(t, err)
	defer nanVal.Free()
	require.True(t, nanVal.IsNumber())
}

// TestPromiseState tests promise state handling
func TestPromiseState(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test all known promise states
	testCases := []struct {
		name     string
		jsCode   string
		expected PromiseState
	}{
		{"Pending", `new Promise(() => {})`, PromisePending},
		{"Fulfilled", `Promise.resolve("test")`, PromiseFulfilled},
		{"Rejected", `Promise.reject("error")`, PromiseRejected},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			promise, err := ctx.Eval(tc.jsCode)
			require.NoError(t, err)
			defer promise.Free()

			require.True(t, promise.IsPromise())
			state := promise.PromiseState()
			require.Equal(t, tc.expected, state)
		})
	}

	// Test non-promise value (covers first if branch)
	nonPromise := ctx.String("not a promise")
	defer nonPromise.Free()
	require.Equal(t, PromisePending, nonPromise.PromiseState())
}

// TestValueAwait tests promise await functionality
func TestValueAwait(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test awaiting resolved promise
	resolvedPromise, err := ctx.Eval(`Promise.resolve("resolved value")`)
	require.NoError(t, err)

	result, err := resolvedPromise.Await()
	require.NoError(t, err)
	defer result.Free()
	require.Equal(t, "resolved value", result.String())

	// Test awaiting non-promise value (should return as-is)
	normalValue := ctx.String("not a promise")

	result2, err := normalValue.Await()
	require.NoError(t, err)
	defer result2.Free()
	require.Equal(t, "not a promise", result2.String())

	// Test awaiting rejected promise
	rejectedPromise, err := ctx.Eval(`Promise.reject(new Error("test error"))`)
	require.NoError(t, err)

	result3, err := rejectedPromise.Await()
	if err != nil {
		require.Error(t, err)
	} else {
		defer result3.Free()
		require.True(t, result3.IsException())
	}
}

// TestValueClassInstanceEdgeCases tests uncovered branches in class instance methods
func TestValueClassInstanceEdgeCases(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test non-object values to cover !v.IsObject() branches
	nonObjects := []struct {
		name      string
		createVal func() Value
	}{
		{"String", func() Value { return ctx.String("test") }},
		{"Number", func() Value { return ctx.Int32(42) }},
		{"Null", func() Value { return ctx.Null() }},
		{"Undefined", func() Value { return ctx.Undefined() }},
	}

	for _, no := range nonObjects {
		t.Run("HasInstanceData_"+no.name, func(t *testing.T) {
			val := no.createVal()
			defer val.Free()
			// Cover: if !v.IsObject() return false branch in HasInstanceData
			require.False(t, val.HasInstanceData())
		})

		t.Run("IsInstanceOfClassID_"+no.name, func(t *testing.T) {
			val := no.createVal()
			defer val.Free()
			// Cover: if !v.IsObject() return false branch in IsInstanceOfClassID
			require.False(t, val.IsInstanceOfClassID(123))
		})

		t.Run("GetGoObject"+no.name, func(t *testing.T) {
			val := no.createVal()
			defer val.Free()
			// Cover: if !v.IsObject() return error branch in GetGoObject
			_, err := val.GetGoObject()
			require.Error(t, err)
			require.Contains(t, err.Error(), "value is not an object")
		})

		t.Run("IsInstanceOfConstructor_NonObject_"+no.name, func(t *testing.T) {
			val := no.createVal()
			defer val.Free()

			fn := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
				return ctx.Null()
			})
			defer fn.Free()

			// Cover: if !v.IsObject() part of condition in IsInstanceOfConstructor
			require.False(t, val.IsInstanceOfConstructor(fn))
		})
	}

	// Test IsInstanceOfConstructor with non-function constructor
	t.Run("IsInstanceOfConstructor_NonFunction", func(t *testing.T) {
		obj := ctx.Object()
		defer obj.Free()

		nonFunc := ctx.String("not a function")
		defer nonFunc.Free()

		// Cover: !constructor.IsFunction() part of condition in IsInstanceOfConstructor
		require.False(t, obj.IsInstanceOfConstructor(nonFunc))
	})

	// Test IsInstanceOfConstructor with valid object and function (no inheritance)
	t.Run("IsInstanceOfConstructor_NoInheritance", func(t *testing.T) {
		obj := ctx.Object()
		defer obj.Free()

		fn := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
			return ctx.Null()
		})
		defer fn.Free()

		// Cover: C.JS_IsInstanceOf call returning 0 (false) in IsInstanceOfConstructor
		require.False(t, obj.IsInstanceOfConstructor(fn))
	})

	// Test GetGoObject "instance data not found in handle store" branch
	t.Run("GetGoObject_HandleStoreManipulation", func(t *testing.T) {
		// Create a function to get a valid object with opaque data
		fn := ctx.Function(func(ctx *Context, this Value, args []Value) Value {
			return ctx.String("test")
		})
		defer fn.Free()

		// Get the handle ID from the function (functions have opaque data)
		var handleID int32
		var originalHandle interface{}
		ctx.handleStore.handles.Range(func(key, value interface{}) bool {
			handleID = key.(int32)
			originalHandle = value
			return false // Stop after first item
		})

		// Remove the handle from the store while keeping the opaque data in the object
		ctx.handleStore.handles.Delete(handleID)

		// Now try to get instance data - should hit "instance data not found in handle store" branch
		_, err := fn.GetGoObject()
		require.Error(t, err)
		require.Contains(t, err.Error(), "instance data not found in handle store")

		// Restore the handle for proper cleanup
		ctx.handleStore.handles.Store(handleID, originalHandle)

		t.Log("Successfully triggered 'instance data not found in handle store' branch")
	})

	// Alternative approach: Test with regular JS objects (no opaque data)
	t.Run("GetGoObject_NoOpaqueData", func(t *testing.T) {
		// Regular objects should have no opaque data, covering "no instance data found"
		obj := ctx.Object()
		defer obj.Free()

		_, err := obj.GetGoObject()
		require.Error(t, err)
		require.Contains(t, err.Error(), "no instance data found")
	})
}
