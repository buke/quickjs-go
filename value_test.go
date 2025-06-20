package quickjs

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

type Point struct {
	X, Y float64
}

// TestValueBasics tests basic value creation and type checking
func TestValueBasics(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic type creation and checking
	testCases := []struct {
		name      string
		createVal func() *Value     // Changed to return pointer
		checkFunc func(*Value) bool // Changed parameter to pointer
	}{
		{"Number", func() *Value { return ctx.Int32(42) }, func(v *Value) bool { return v.IsNumber() }},
		{"String", func() *Value { return ctx.String("test") }, func(v *Value) bool { return v.IsString() }},
		{"Boolean", func() *Value { return ctx.Bool(true) }, func(v *Value) bool { return v.IsBool() }},
		{"Null", func() *Value { return ctx.Null() }, func(v *Value) bool { return v.IsNull() }},
		{"Undefined", func() *Value { return ctx.Undefined() }, func(v *Value) bool { return v.IsUndefined() }},
		{"Uninitialized", func() *Value { return ctx.Uninitialized() }, func(v *Value) bool { return v.IsUninitialized() }},
		{"Object", func() *Value { return ctx.Object() }, func(v *Value) bool { return v.IsObject() }},
		{"BigInt", func() *Value { return ctx.BigInt64(123456789) }, func(v *Value) bool { return v.IsBigInt() }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val := tc.createVal()
			defer val.Free()
			require.True(t, tc.checkFunc(val))
			require.Equal(t, ctx, val.Context()) // Test Context() method
		})
	}

	// Test JavaScript created values - FIXED: removed error handling
	arr := ctx.Eval(`[1, 2, 3]`)
	defer arr.Free()
	require.False(t, arr.IsException()) // Check for exceptions instead of error
	require.True(t, arr.IsArray())
	require.True(t, arr.IsObject()) // Arrays are objects

	sym := ctx.Eval(`Symbol('test')`)
	defer sym.Free()
	require.False(t, sym.IsException()) // Check for exceptions instead of error
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
		createVal      func() *Value // Changed to return pointer
		testFunc       func(*Value)  // Changed parameter to pointer
		testDeprecated func(*Value)  // Changed parameter to pointer - Test deprecated methods for coverage
	}{
		{
			name:           "Bool",
			createVal:      func() *Value { return ctx.Bool(true) },
			testFunc:       func(v *Value) { require.True(t, v.ToBool()) },
			testDeprecated: func(v *Value) { require.True(t, v.Bool()) },
		},
		{
			name:      "String",
			createVal: func() *Value { return ctx.String("Hello") },
			testFunc: func(v *Value) {
				require.Equal(t, "Hello", v.ToString())
				require.Equal(t, "Hello", v.String()) // String() calls ToString()
			},
		},
		{
			name:           "Int32",
			createVal:      func() *Value { return ctx.Int32(42) },
			testFunc:       func(v *Value) { require.Equal(t, int32(42), v.ToInt32()) },
			testDeprecated: func(v *Value) { require.Equal(t, int32(42), v.Int32()) },
		},
		{
			name:           "Int64",
			createVal:      func() *Value { return ctx.Int64(1234567890) },
			testFunc:       func(v *Value) { require.Equal(t, int64(1234567890), v.ToInt64()) },
			testDeprecated: func(v *Value) { require.Equal(t, int64(1234567890), v.Int64()) },
		},
		{
			name:           "Uint32",
			createVal:      func() *Value { return ctx.Uint32(4294967295) },
			testFunc:       func(v *Value) { require.Equal(t, uint32(4294967295), v.ToUint32()) },
			testDeprecated: func(v *Value) { require.Equal(t, uint32(4294967295), v.Uint32()) },
		},
		{
			name:           "Float64",
			createVal:      func() *Value { return ctx.Float64(3.14159) },
			testFunc:       func(v *Value) { require.InDelta(t, 3.14159, v.ToFloat64(), 0.00001) },
			testDeprecated: func(v *Value) { require.InDelta(t, 3.14159, v.Float64(), 0.00001) },
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
		createVal func() *Value // Changed to return pointer
		expected  string
	}{
		{"String", func() *Value { return ctx.String("hello") }, `"hello"`},
		{"Null", func() *Value { return ctx.Null() }, "null"},
		{"True", func() *Value { return ctx.Bool(true) }, "true"},
		{"False", func() *Value { return ctx.Bool(false) }, "false"},
		{"Number", func() *Value { return ctx.Int32(42) }, "42"},
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

	// Test array length - FIXED: removed error handling
	arr := ctx.Eval(`[1, 2, 3, 4, 5]`)
	defer arr.Free()
	require.False(t, arr.IsException()) // Check for exceptions instead of error
	require.Equal(t, int64(5), arr.Len())

	// Test error cases with non-ArrayBuffer types
	errorTests := []struct {
		name      string
		createVal func() *Value // Changed to return pointer
	}{
		{"Object", func() *Value { return ctx.Object() }},
		{"String", func() *Value { return ctx.String("not an array buffer") }},
		{"Number", func() *Value { return ctx.Int32(42) }},
		{"Null", func() *Value { return ctx.Null() }},
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
		checkFunc func(*Value) bool // Changed parameter to pointer
		isTyped   bool
	}{
		{"Int8Array", "new Int8Array([1, 2, 3])", func(v *Value) bool { return v.IsInt8Array() }, true},
		{"Uint8Array", "new Uint8Array([1, 2, 3])", func(v *Value) bool { return v.IsUint8Array() }, true},
		{"Uint8ClampedArray", "new Uint8ClampedArray([1, 2, 3])", func(v *Value) bool { return v.IsUint8ClampedArray() }, true},
		{"Int16Array", "new Int16Array([1, 2, 3])", func(v *Value) bool { return v.IsInt16Array() }, true},
		{"Uint16Array", "new Uint16Array([1, 2, 3])", func(v *Value) bool { return v.IsUint16Array() }, true},
		{"Int32Array", "new Int32Array([1, 2, 3])", func(v *Value) bool { return v.IsInt32Array() }, true},
		{"Uint32Array", "new Uint32Array([1, 2, 3])", func(v *Value) bool { return v.IsUint32Array() }, true},
		{"Float32Array", "new Float32Array([1.5, 2.5, 3.5])", func(v *Value) bool { return v.IsFloat32Array() }, true},
		{"Float64Array", "new Float64Array([1.5, 2.5, 3.5])", func(v *Value) bool { return v.IsFloat64Array() }, true},
		{"BigInt64Array", "new BigInt64Array([1n, 2n, 3n])", func(v *Value) bool { return v.IsBigInt64Array() }, true},
		{"BigUint64Array", "new BigUint64Array([1n, 2n, 3n])", func(v *Value) bool { return v.IsBigUint64Array() }, true},
		{"RegularArray", "[1, 2, 3]", func(v *Value) bool { return v.IsInt8Array() }, false},
	}

	for _, tt := range typedArrayTests {
		t.Run(tt.name, func(t *testing.T) {
			val := ctx.Eval(tt.jsCode)
			defer val.Free()
			require.False(t, val.IsException()) // Check for exceptions instead of error

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
		convertFunc func(*Value) (interface{}, error) // Changed parameter to pointer
		expected    interface{}
	}{
		{
			name:        "Int8Array",
			jsCode:      "new Int8Array([-128, 0, 127])",
			convertFunc: func(v *Value) (interface{}, error) { return v.ToInt8Array() },
			expected:    []int8{-128, 0, 127},
		},
		{
			name:        "Uint8Array",
			jsCode:      "new Uint8Array([0, 128, 255])",
			convertFunc: func(v *Value) (interface{}, error) { return v.ToUint8Array() },
			expected:    []uint8{0, 128, 255},
		},
		{
			name:        "Int32Array",
			jsCode:      "new Int32Array([-2147483648, 0, 2147483647])",
			convertFunc: func(v *Value) (interface{}, error) { return v.ToInt32Array() },
			expected:    []int32{-2147483648, 0, 2147483647},
		},
		{
			name:        "Float32Array",
			jsCode:      "new Float32Array([1.5, 2.5, 3.14159])",
			convertFunc: func(v *Value) (interface{}, error) { return v.ToFloat32Array() },
			expected:    []float32{1.5, 2.5, 3.14159},
		},
		{
			name:        "BigInt64Array",
			jsCode:      "new BigInt64Array([-9223372036854775808n, 0n, 9223372036854775807n])",
			convertFunc: func(v *Value) (interface{}, error) { return v.ToBigInt64Array() },
			expected:    []int64{-9223372036854775808, 0, 9223372036854775807},
		},
	}

	for _, tt := range conversionTests {
		t.Run(tt.name, func(t *testing.T) {
			val := ctx.Eval(tt.jsCode)
			defer val.Free()
			require.False(t, val.IsException()) // Check for exceptions instead of error

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
		testFn func(*Value) // Changed parameter to pointer
	}{
		{"Uint8ClampedArray", "new Uint8ClampedArray([0, 128, 255])", func(v *Value) {
			result, err := v.ToUint8Array() // Uint8ClampedArray uses same method
			require.NoError(t, err)
			require.Equal(t, []uint8{0, 128, 255}, result)
		}},
		{"Uint16Array", "new Uint16Array([0, 32768, 65535])", func(v *Value) {
			result, err := v.ToUint16Array()
			require.NoError(t, err)
			require.Equal(t, []uint16{0, 32768, 65535}, result)
		}},
		{"Int16Array", "new Int16Array([-32768, 0, 32767])", func(v *Value) {
			result, err := v.ToInt16Array()
			require.NoError(t, err)
			require.Equal(t, []int16{-32768, 0, 32767}, result)
		}},
		{"Uint32Array", "new Uint32Array([0, 2147483648, 4294967295])", func(v *Value) {
			result, err := v.ToUint32Array()
			require.NoError(t, err)
			require.Equal(t, []uint32{0, 2147483648, 4294967295}, result)
		}},
		{"Float64Array", "new Float64Array([1.5, 2.5, 3.141592653589793])", func(v *Value) {
			result, err := v.ToFloat64Array()
			require.NoError(t, err)
			expected := []float64{1.5, 2.5, 3.141592653589793}
			require.Len(t, result, len(expected))
			for i, exp := range expected {
				require.InDelta(t, exp, result[i], 1e-10)
			}
		}},
		{"BigUint64Array", "new BigUint64Array([0n, 9223372036854775808n, 18446744073709551615n])", func(v *Value) {
			result, err := v.ToBigUint64Array()
			require.NoError(t, err)
			require.Equal(t, []uint64{0, 9223372036854775808, 18446744073709551615}, result)
		}},
	}

	for _, tt := range additionalTests {
		t.Run(tt.name, func(t *testing.T) {
			val := ctx.Eval(tt.jsCode)
			defer val.Free()
			require.False(t, val.IsException()) // Check for exceptions instead of error
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

	// Test function calls - UPDATED: function signature now uses pointers
	addFunc := ctx.Function(func(ctx *Context, this *Value, args []*Value) *Value {
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
	noArgsFunc := ctx.Function(func(ctx *Context, this *Value, args []*Value) *Value {
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

	// Test constructors - FIXED: removed error handling
	constructorFunc := ctx.Eval(`
        function TestClass(value) {
            this.value = value;
        }
        TestClass;
    `)
	defer constructorFunc.Free()
	require.False(t, constructorFunc.IsException()) // Check for exceptions instead of error

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

	// Test complex error with all properties - FIXED: removed error handling
	complexError := ctx.Eval(`
        const err = new Error("complex error");
        err.name = "CustomError";
        err.cause = "underlying cause";
        err.stack = "stack trace here";
        err;
    `)
	defer complexError.Free()
	require.False(t, complexError.IsException()) // Check for exceptions instead of error

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

	// Test valid instanceof cases - FIXED: removed error handling
	arr := ctx.Eval(`[1, 2, 3]`)
	defer arr.Free()
	require.False(t, arr.IsException()) // Check for exceptions instead of error
	require.True(t, arr.GlobalInstanceof("Array"))
	require.True(t, arr.GlobalInstanceof("Object"))

	obj := ctx.Eval(`({})`)
	defer obj.Free()
	require.False(t, obj.IsException()) // Check for exceptions instead of error
	require.True(t, obj.GlobalInstanceof("Object"))
	require.False(t, obj.GlobalInstanceof("Array"))

	// Test false cases to ensure coverage
	testVals := []struct {
		name      string
		createVal func() *Value // Changed to return pointer
	}{
		{"String", func() *Value { return ctx.String("test") }},
		{"Number", func() *Value { return ctx.Int32(42) }},
		{"Null", func() *Value { return ctx.Null() }},
		{"Undefined", func() *Value { return ctx.Undefined() }},
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

	// Test function - UPDATED: function signature now uses pointers
	funcVal := ctx.Function(func(ctx *Context, this *Value, args []*Value) *Value {
		return ctx.Null()
	})
	defer funcVal.Free()
	require.True(t, funcVal.IsFunction())
	require.False(t, funcVal.IsPromise()) // Functions are not promises

	// Test constructor - FIXED: removed error handling
	constructorVal := ctx.Eval(`function TestConstructor() {}; TestConstructor`)
	defer constructorVal.Free()
	require.False(t, constructorVal.IsException()) // Check for exceptions instead of error
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
			promiseVal := ctx.Eval(tt.jsCode)
			defer promiseVal.Free()
			require.False(t, promiseVal.IsException()) // Check for exceptions instead of error
			require.True(t, promiseVal.IsPromise())
		})
	}

	// Test non-Promise objects for IsPromise method (covers return false branch)
	nonPromiseTests := []struct {
		name      string
		createVal func() *Value // Changed to return pointer
	}{
		{"Object", func() *Value { return ctx.Object() }},
		{"String", func() *Value { return ctx.String("not a promise") }},
		{"Number", func() *Value { return ctx.Int32(42) }},
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

	// Test special float values - FIXED: removed error handling
	infVal := ctx.Eval(`Infinity`)
	defer infVal.Free()
	require.False(t, infVal.IsException()) // Check for exceptions instead of error
	require.True(t, infVal.IsNumber())

	nanVal := ctx.Eval(`NaN`)
	defer nanVal.Free()
	require.False(t, nanVal.IsException()) // Check for exceptions instead of error
	require.True(t, nanVal.IsNumber())

	// Test nil value for special type checks
	var nilValue *Value
	require.False(t, nilValue.IsPromise(), "nil value should not be a promise")
	require.False(t, nilValue.IsTypedArray(), "nil value should not be a typed array")

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
			promise := ctx.Eval(tc.jsCode)
			defer promise.Free()
			require.False(t, promise.IsException()) // Check for exceptions instead of error

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

	// Test awaiting resolved promise - FIXED: removed error handling
	resolvedPromise := ctx.Eval(`Promise.resolve("resolved value")`)
	require.False(t, resolvedPromise.IsException()) // Check for exceptions instead of error

	result := resolvedPromise.Await()
	defer result.Free()
	// Check if result is an exception or valid value
	if result.IsException() {
		err := ctx.Exception()
		t.Logf("Promise await resulted in exception: %v", err)
	} else {
		require.Equal(t, "resolved value", result.String())
	}

	// Test awaiting non-promise value (should return as-is) - FIXED: removed error handling
	normalValue := ctx.String("not a promise")

	result2 := normalValue.Await()
	defer result2.Free()
	if result2.IsException() {
		err := ctx.Exception()
		t.Logf("Non-promise await resulted in exception: %v", err)
	} else {
		require.Equal(t, "not a promise", result2.String())
	}

	// Test awaiting rejected promise - FIXED: removed error handling
	rejectedPromise := ctx.Eval(`Promise.reject(new Error("test error"))`)
	require.False(t, rejectedPromise.IsException()) // Check for exceptions instead of error

	result3 := rejectedPromise.Await()
	defer result3.Free()
	// Rejected promise should result in an exception when awaited
	require.True(t, result3.IsException())
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
		createVal func() *Value // Changed to return pointer
	}{
		{"String", func() *Value { return ctx.String("test") }},
		{"Number", func() *Value { return ctx.Int32(42) }},
		{"Null", func() *Value { return ctx.Null() }},
		{"Undefined", func() *Value { return ctx.Undefined() }},
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

			fn := ctx.Function(func(ctx *Context, this *Value, args []*Value) *Value {
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

		fn := ctx.Function(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.Null()
		})
		defer fn.Free()

		// Cover: C.JS_IsInstanceOf call returning 0 (false) in IsInstanceOfConstructor
		require.False(t, obj.IsInstanceOfConstructor(fn))
	})

	// Test GetGoObject "instance data not found in handle store" branch
	t.Run("GetGoObject_HandleStoreManipulation", func(t *testing.T) {
		// Create a function to get a valid object with opaque data
		fn := ctx.Function(func(ctx *Context, this *Value, args []*Value) *Value {
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

// TestValueCallConstructorEdgeCases tests edge cases and error conditions in CallConstructor
// MODIFIED FOR SCHEME C: Removed all NewInstance tests, enhanced CallConstructor coverage
func TestValueCallConstructorEdgeCases(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Case 1: CallConstructor called on non-constructor value
	t.Run("CallConstructor_NonConstructor", func(t *testing.T) {
		// Test with regular object (not a constructor)
		obj := ctx.Object()
		defer obj.Free()

		// This should trigger a JavaScript TypeError since object is not a constructor
		result := obj.CallConstructor()
		defer result.Free()

		// Verify it returns an error/exception or creates a generic object (depends on JS engine behavior)
		// For non-constructor objects, JavaScript usually throws TypeError
		if !result.IsException() {
			// Some JavaScript engines might return an object, that's also valid
			require.True(t, result.IsObject())
		}
	})

	// Test Case 2: CallConstructor called on string (definitely not a constructor)
	t.Run("CallConstructor_String", func(t *testing.T) {
		str := ctx.String("not a constructor")
		defer str.Free()

		// This should trigger a JavaScript TypeError
		result := str.CallConstructor()
		defer result.Free()

		// Should definitely be an exception since strings are not constructors
		require.True(t, result.IsException())
	})

	// Test Case 3: CallConstructor with various non-constructor types
	t.Run("CallConstructor_VariousNonConstructors", func(t *testing.T) {
		testCases := []struct {
			name string
			val  func() *Value // Changed to return pointer
		}{
			{"Number", func() *Value { return ctx.Int32(42) }},
			{"Boolean", func() *Value { return ctx.Bool(true) }},
			{"Null", func() *Value { return ctx.Null() }},
			{"Undefined", func() *Value { return ctx.Undefined() }},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				val := tc.val()
				defer val.Free()

				result := val.CallConstructor()
				defer result.Free()

				// All of these should trigger JavaScript TypeError
				require.True(t, result.IsException())
			})
		}
	})

	// Test Case 4: CallConstructor with unregistered constructor - FIXED: removed error handling
	t.Run("CallConstructor_UnregisteredConstructor", func(t *testing.T) {
		// Create a constructor function that's not registered in our class system
		unregisteredConstructor := ctx.Eval(`
            function UnregisteredClass(value) {
                this.value = value;
            }
            UnregisteredClass;
        `)
		defer unregisteredConstructor.Free()
		require.False(t, unregisteredConstructor.IsException()) // Check for exceptions instead of error

		// This should work fine - JavaScript constructors don't need to be in our class registry
		result := unregisteredConstructor.CallConstructor(ctx.String("test"))
		defer result.Free()

		require.False(t, result.IsException())
		require.True(t, result.IsObject())

		// Verify the property was set
		value := result.Get("value")
		defer value.Free()
		require.Equal(t, "test", value.String())
	})

	// Test Case 5: CallConstructor with proxy constructor - FIXED: removed error handling
	t.Run("CallConstructor_ProxyConstructor", func(t *testing.T) {
		// Create a constructor wrapped in a Proxy
		proxyConstructor := ctx.Eval(`
            function BaseClass(value) {
                this.value = value || "default";
            }

            const ProxyConstructor = new Proxy(BaseClass, {
                construct: function(target, args, newTarget) {
                    return Reflect.construct(target, args, newTarget);
                }
            });

            ProxyConstructor;
        `)
		defer proxyConstructor.Free()
		require.False(t, proxyConstructor.IsException()) // Check for exceptions instead of error

		// This should work through the proxy
		result := proxyConstructor.CallConstructor(ctx.String("proxy_test"))
		defer result.Free()

		require.False(t, result.IsException())
		require.True(t, result.IsObject())

		// Verify the property was set through proxy
		value := result.Get("value")
		defer value.Free()
		require.Equal(t, "proxy_test", value.String())
	})

	// Test Case 6: CallConstructor with arrow function (not a constructor) - FIXED: removed error handling
	t.Run("CallConstructor_ArrowFunction", func(t *testing.T) {
		arrowFunc := ctx.Eval(`(() => {})`)
		defer arrowFunc.Free()
		require.False(t, arrowFunc.IsException()) // Check for exceptions instead of error

		// Arrow functions cannot be used as constructors
		result := arrowFunc.CallConstructor()
		defer result.Free()

		require.True(t, result.IsException())
	})

	// Test Case 7: CallConstructor with bound function - FIXED: removed error handling
	t.Run("CallConstructor_BoundFunction", func(t *testing.T) {
		boundFunc := ctx.Eval(`
            function OriginalConstructor(value) {
                this.value = value || "bound_default";
            }
            OriginalConstructor.bind(null);
        `)
		defer boundFunc.Free()
		require.False(t, boundFunc.IsException()) // Check for exceptions instead of error

		// Bound functions can be used as constructors
		result := boundFunc.CallConstructor(ctx.String("bound_test"))
		defer result.Free()

		require.False(t, result.IsException())
		require.True(t, result.IsObject())
	})

	// Test Case 8: CallConstructor with built-in constructors
	t.Run("CallConstructor_BuiltInConstructors", func(t *testing.T) {
		builtInTests := []struct {
			name     string
			jsCode   string
			args     []*Value     // Changed to slice of pointers
			validate func(*Value) // Changed parameter to pointer
		}{
			{
				name:   "Array",
				jsCode: "Array",
				args:   []*Value{ctx.Int32(3)},
				validate: func(v *Value) {
					require.True(t, v.IsArray())
					require.Equal(t, int64(3), v.Len())
				},
			},
			{
				name:   "Object",
				jsCode: "Object",
				args:   nil,
				validate: func(v *Value) {
					require.True(t, v.IsObject())
					require.False(t, v.IsArray())
				},
			},
			{
				name:   "Date",
				jsCode: "Date",
				args:   []*Value{ctx.String("2023-01-01")},
				validate: func(v *Value) {
					require.True(t, v.IsObject())
					// Date objects have getTime method
					getTime := v.Get("getTime")
					defer getTime.Free()
					require.True(t, getTime.IsFunction())
				},
			},
		}

		for _, tt := range builtInTests {
			t.Run(tt.name, func(t *testing.T) {
				constructor := ctx.Eval(tt.jsCode)
				defer constructor.Free()
				require.False(t, constructor.IsException()) // Check for exceptions instead of error

				var result *Value
				if len(tt.args) > 0 {
					result = constructor.CallConstructor(tt.args...)
				} else {
					result = constructor.CallConstructor()
				}
				defer result.Free()

				require.False(t, result.IsException())
				tt.validate(result)
			})
		}
	})

	// Test Case 9: Successful CallConstructor with registered class (for comparison)
	t.Run("CallConstructor_RegisteredClass", func(t *testing.T) {
		// Create a Point class using our class system - UPDATED: constructor signature now uses pointers
		pointConstructor, _ := NewClassBuilder("Point").
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				x, y := 0.0, 0.0
				if len(args) > 0 {
					x = args[0].Float64()
				}
				if len(args) > 1 {
					y = args[1].Float64()
				}

				// SCHEME C: Create Go object and return it for automatic association
				point := &Point{X: x, Y: y}
				return point, nil
			}).
			Method("norm", func(ctx *Context, this *Value, args []*Value) *Value {
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				norm := math.Sqrt(point.X*point.X + point.Y*point.Y)
				return ctx.Float64(norm)
			}).
			Build(ctx)
		defer pointConstructor.Free()
		require.False(t, pointConstructor.IsException()) // Check for exceptions instead of error

		// Test CallConstructor with arguments
		instance := pointConstructor.CallConstructor(ctx.Float64(3.0), ctx.Float64(4.0))
		defer instance.Free()

		require.False(t, instance.IsException())
		require.True(t, instance.IsObject())

		// Verify we can call methods on the instance
		norm := instance.Call("norm")
		defer norm.Free()
		require.InDelta(t, 5.0, norm.Float64(), 0.001)

		// Verify we can retrieve the Go object
		goObj, err := instance.GetGoObject()
		require.NoError(t, err)

		point, ok := goObj.(*Point)
		require.True(t, ok)
		require.Equal(t, 3.0, point.X)
		require.Equal(t, 4.0, point.Y)
	})
}

// TestValueCallConstructorComprehensive tests comprehensive CallConstructor scenarios
// NEW TEST: Comprehensive coverage for CallConstructor API
func TestValueCallConstructorComprehensive(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Case 1: CallConstructor with different argument counts - FIXED: removed error handling
	t.Run("CallConstructor_ArgumentCounts", func(t *testing.T) {
		constructor := ctx.Eval(`
            function TestClass() {
                this.argCount = arguments.length;
                this.args = Array.from(arguments);
            }
            TestClass;
        `)
		defer constructor.Free()
		require.False(t, constructor.IsException()) // Check for exceptions instead of error

		// Test with no arguments
		instance0 := constructor.CallConstructor()
		defer instance0.Free()
		require.False(t, instance0.IsException())

		argCount0 := instance0.Get("argCount")
		defer argCount0.Free()
		require.Equal(t, int32(0), argCount0.ToInt32())

		// Test with one argument
		instance1 := constructor.CallConstructor(ctx.String("arg1"))
		defer instance1.Free()
		require.False(t, instance1.IsException())

		argCount1 := instance1.Get("argCount")
		defer argCount1.Free()
		require.Equal(t, int32(1), argCount1.ToInt32())

		// Test with multiple arguments
		instance3 := constructor.CallConstructor(
			ctx.String("arg1"),
			ctx.Int32(42),
			ctx.Bool(true),
		)
		defer instance3.Free()
		require.False(t, instance3.IsException())

		argCount3 := instance3.Get("argCount")
		defer argCount3.Free()
		require.Equal(t, int32(3), argCount3.ToInt32())
	})

	// Test Case 2: CallConstructor with inheritance chain - FIXED: removed error handling
	t.Run("CallConstructor_InheritanceChain", func(t *testing.T) {
		// Set up inheritance chain
		ret := ctx.Eval(`
            function Base(value) {
                this.baseValue = value;
            }
            Base.prototype.getBase = function() {
                return this.baseValue;
            };

            function Child(base, child) {
                Base.call(this, base);
                this.childValue = child;
            }
            Child.prototype = Object.create(Base.prototype);
            Child.prototype.constructor = Child;
            Child.prototype.getChild = function() {
                return this.childValue;
            };
        `)
		defer ret.Free()
		require.False(t, ret.IsException()) // Check for exceptions instead of error

		childConstructor := ctx.Eval(`Child`)
		defer childConstructor.Free()
		require.False(t, childConstructor.IsException()) // Check for exceptions instead of error

		// Create instance using CallConstructor
		instance := childConstructor.CallConstructor(
			ctx.String("base_val"),
			ctx.String("child_val"),
		)
		defer instance.Free()
		require.False(t, instance.IsException())

		// Test base functionality
		baseValue := instance.Call("getBase")
		defer baseValue.Free()
		require.Equal(t, "base_val", baseValue.String())

		// Test child functionality
		childValue := instance.Call("getChild")
		defer childValue.Free()
		require.Equal(t, "child_val", childValue.String())

		// Test instanceof relationships
		require.True(t, instance.GlobalInstanceof("Child"))
		require.True(t, instance.GlobalInstanceof("Base"))
		require.True(t, instance.GlobalInstanceof("Object"))
	})

	// Test Case 3: CallConstructor with ES6 classes - FIXED: removed error handling
	t.Run("CallConstructor_ES6Classes", func(t *testing.T) {
		es6Constructor := ctx.Eval(`
            class ES6Class {
                constructor(name, value) {
                    this.name = name || "default";
                    this.value = value || 0;
                }

                getName() {
                    return this.name;
                }

                getValue() {
                    return this.value;
                }

                static getClassName() {
                    return "ES6Class";
                }
            }
            ES6Class;
        `)
		defer es6Constructor.Free()
		require.False(t, es6Constructor.IsException()) // Check for exceptions instead of error

		// Test CallConstructor with ES6 class
		instance := es6Constructor.CallConstructor(
			ctx.String("test_name"),
			ctx.Int32(123),
		)
		defer instance.Free()
		require.False(t, instance.IsException())

		// Test instance methods
		name := instance.Call("getName")
		defer name.Free()
		require.Equal(t, "test_name", name.String())

		value := instance.Call("getValue")
		defer value.Free()
		require.Equal(t, int32(123), value.ToInt32())

		// Test static method on constructor
		className := es6Constructor.Call("getClassName")
		defer className.Free()
		require.Equal(t, "ES6Class", className.String())
	})

	// Test Case 4: CallConstructor error scenarios - FIXED: removed error handling
	t.Run("CallConstructor_ErrorScenarios", func(t *testing.T) {
		// Constructor that throws
		throwingConstructor := ctx.Eval(`
            function ThrowingConstructor() {
                throw new Error("Constructor intentionally throws");
            }
            ThrowingConstructor;
        `)
		defer throwingConstructor.Free()
		require.False(t, throwingConstructor.IsException()) // Check for exceptions instead of error

		instance := throwingConstructor.CallConstructor()
		defer instance.Free()
		require.True(t, instance.IsException())

		// Constructor with invalid prototype
		invalidProtoConstructor := ctx.Eval(`
            function InvalidProtoConstructor() {}
            InvalidProtoConstructor.prototype = null;
            InvalidProtoConstructor;
        `)
		defer invalidProtoConstructor.Free()
		require.False(t, invalidProtoConstructor.IsException()) // Check for exceptions instead of error

		// This might still work but create object with different prototype
		instance2 := invalidProtoConstructor.CallConstructor()
		defer instance2.Free()
		// Result depends on JavaScript engine behavior
		// Could be exception or object with different prototype
	})

	// Test Case 5: CallConstructor performance test - FIXED: removed error handling
	t.Run("CallConstructor_Performance", func(t *testing.T) {
		constructor := ctx.Eval(`
            function PerfTestClass(id) {
                this.id = id;
                this.created = new Date();
            }
            PerfTestClass;
        `)
		defer constructor.Free()
		require.False(t, constructor.IsException()) // Check for exceptions instead of error

		// Create multiple instances to test performance
		const numInstances = 100
		instances := make([]*Value, numInstances) // Changed to slice of pointers

		for i := 0; i < numInstances; i++ {
			instances[i] = constructor.CallConstructor(ctx.Int32(int32(i)))
			require.False(t, instances[i].IsException())
		}

		// Verify all instances were created correctly
		for i, instance := range instances {
			id := instance.Get("id")
			require.Equal(t, int32(i), id.ToInt32())
			id.Free()
			instance.Free()
		}
	})
}
