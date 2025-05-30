package quickjs_test

import (
	"errors"
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

	// 新增：测试 ToByteArray 方法的错误情况 (覆盖 value.go:180.21,182.3)
	// 创建一个非 ArrayBuffer 对象来触发错误
	normalObj := ctx.Object()
	defer normalObj.Free()

	_, err = normalObj.ToByteArray(1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds the maximum length of the current binary arra")

	// 测试其他非 ArrayBuffer 类型
	str := ctx.String("not an array buffer")
	defer str.Free()
	_, err = str.ToByteArray(1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds the maximum length of the current binary arra")

	nullVal := ctx.Null()
	defer nullVal.Free()
	_, err = nullVal.ToByteArray(1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds the maximum length of the current binary arra")
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

	// 新增：测试更多非对象类型的 PropertyNames (覆盖 value.go:255.26,255.3)
	primitiveVal := ctx.Int32(42)
	defer primitiveVal.Free()

	_, err = primitiveVal.PropertyNames()
	require.Error(t, err)
	require.Contains(t, err.Error(), "value does not contain properties")

	// 测试 null 值的 PropertyNames
	nullVal := ctx.Null()
	defer nullVal.Free()

	_, err = nullVal.PropertyNames()
	require.Error(t, err)
	require.Contains(t, err.Error(), "value does not contain properties")

	// 测试 undefined 值的 PropertyNames
	undefinedVal := ctx.Undefined()
	defer undefinedVal.Free()

	_, err = undefinedVal.PropertyNames()
	require.Error(t, err)
	require.Contains(t, err.Error(), "value does not contain properties")

	// 测试 boolean 值的 PropertyNames
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

	// NEW: Test Call method without arguments (covers len(cargs) == 0 branch)
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

	// NEW: Test error with cause property (covers cause branch in ToError method)
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

	// 新增/修改：确保测试覆盖 return false 的情况 (覆盖 value.go:366.2,366.14)
	// Test with non-existent constructor
	str := ctx.String("test")
	defer str.Free()
	require.False(t, str.GlobalInstanceof("NonExistentConstructor"))
	require.False(t, str.GlobalInstanceof(""))
	require.False(t, str.GlobalInstanceof("UndefinedConstructor"))

	// Test with primitive value
	require.False(t, str.GlobalInstanceof("String"))

	// 测试对象但构造函数不匹配的情况
	require.False(t, obj.GlobalInstanceof("Array"))    // 应该返回 false
	require.False(t, obj.GlobalInstanceof("Function")) // 应该返回 false
	require.False(t, obj.GlobalInstanceof("Date"))     // 应该返回 false

	// 测试各种非对象类型的 GlobalInstanceof
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

		// 所有这些都应该返回 false
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

	// NEW: Test non-Promise objects for IsPromise method (covers return false branch)
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
