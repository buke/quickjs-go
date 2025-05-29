package quickjs_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestValueFree tests value memory management.
func TestValueFree(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test that Free() can be called multiple times safely
	val := ctx.String("test")
	val.Free()
	// Second Free() should not crash (though it's not recommended)
}

// TestValueContext tests getting context from value.
func TestValueContext(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val := ctx.String("test")
	defer val.Free()

	valueCtx := val.Context()
	require.NotNil(t, valueCtx)
	require.Equal(t, ctx, valueCtx)
}

// TestValueTypeConversions tests all To* conversion methods.
func TestValueTypeConversions(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test ToBool
	trueVal := ctx.Bool(true)
	defer trueVal.Free()
	require.True(t, trueVal.ToBool())

	falseVal := ctx.Bool(false)
	defer falseVal.Free()
	require.False(t, falseVal.ToBool())

	// Test ToString and String (which calls ToString)
	stringVal := ctx.String("Hello World")
	defer stringVal.Free()
	require.EqualValues(t, "Hello World", stringVal.ToString())
	require.EqualValues(t, "Hello World", stringVal.String())

	// Test ToInt32
	int32Val := ctx.Int32(42)
	defer int32Val.Free()
	require.EqualValues(t, 42, int32Val.ToInt32())

	// Test ToInt64
	int64Val := ctx.Int64(1234567890)
	defer int64Val.Free()
	require.EqualValues(t, 1234567890, int64Val.ToInt64())

	// Test ToUint32
	uint32Val := ctx.Uint32(4294967295)
	defer uint32Val.Free()
	require.EqualValues(t, 4294967295, uint32Val.ToUint32())

	// Test ToFloat64
	floatVal := ctx.Float64(3.14159)
	defer floatVal.Free()
	require.InDelta(t, 3.14159, floatVal.ToFloat64(), 0.00001)

	// Test ToBigInt
	bigIntVal := ctx.BigInt64(9223372036854775807)
	defer bigIntVal.Free()
	expectedBigInt := big.NewInt(9223372036854775807)
	require.Equal(t, expectedBigInt, bigIntVal.ToBigInt())

	// Test ToBigInt with non-BigInt value
	normalIntVal := ctx.Int32(42)
	defer normalIntVal.Free()
	require.Nil(t, normalIntVal.ToBigInt())
}

// TestValueDeprecatedMethods tests deprecated conversion methods.
func TestValueDeprecatedMethods(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test deprecated Bool()
	boolVal := ctx.Bool(true)
	defer boolVal.Free()
	require.True(t, boolVal.Bool())

	// Test deprecated Int32()
	int32Val := ctx.Int32(42)
	defer int32Val.Free()
	require.EqualValues(t, 42, int32Val.Int32())

	// Test deprecated Int64()
	int64Val := ctx.Int64(1234567890)
	defer int64Val.Free()
	require.EqualValues(t, 1234567890, int64Val.Int64())

	// Test deprecated Uint32()
	uint32Val := ctx.Uint32(4294967295)
	defer uint32Val.Free()
	require.EqualValues(t, 4294967295, uint32Val.Uint32())

	// Test deprecated Float64()
	floatVal := ctx.Float64(3.14159)
	defer floatVal.Free()
	require.InDelta(t, 3.14159, floatVal.Float64(), 0.00001)

	// Test deprecated BigInt()
	bigIntVal := ctx.BigInt64(123456789)
	defer bigIntVal.Free()
	expectedBigInt := big.NewInt(123456789)
	require.Equal(t, expectedBigInt, bigIntVal.BigInt())
}

// TestValueJSONStringify tests JSON serialization.
func TestValueJSONStringify(t *testing.T) {
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

	arrJsonStr := arr.JSONStringify()
	require.EqualValues(t, "[1,2,3]", arrJsonStr)

	// Test string JSON stringify
	str := ctx.String("hello")
	defer str.Free()
	require.EqualValues(t, "\"hello\"", str.JSONStringify())
}

// TestValueArrayBuffer tests ArrayBuffer operations.
func TestValueArrayBuffer(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test ArrayBuffer creation and operations
	data := []byte{1, 2, 3, 4, 5}
	arrayBuffer := ctx.ArrayBuffer(data)
	defer arrayBuffer.Free()

	// Test IsByteArray
	require.True(t, arrayBuffer.IsByteArray())

	// Test ByteLen
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
}

// TestValueArrayOperations tests array length operations.
func TestValueArrayOperations(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test array length
	arr, err := ctx.Eval(`[1, 2, 3, 4, 5]`)
	require.NoError(t, err)
	defer arr.Free()

	require.EqualValues(t, 5, arr.Len())

	// Test empty array
	emptyArr, err := ctx.Eval(`[]`)
	require.NoError(t, err)
	defer emptyArr.Free()

	require.EqualValues(t, 0, emptyArr.Len())
}

// TestValuePropertyOperations tests property manipulation.
func TestValuePropertyOperations(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Test Set and Get
	obj.Set("name", ctx.String("test"))
	obj.Set("value", ctx.Int32(42))
	obj.Set("flag", ctx.Bool(true))

	nameVal := obj.Get("name")
	defer nameVal.Free()
	require.EqualValues(t, "test", nameVal.String())

	valueVal := obj.Get("value")
	defer valueVal.Free()
	require.EqualValues(t, 42, valueVal.ToInt32())

	flagVal := obj.Get("flag")
	defer flagVal.Free()
	require.True(t, flagVal.ToBool())

	// Test SetIdx and GetIdx
	obj.SetIdx(0, ctx.String("index0"))
	obj.SetIdx(1, ctx.String("index1"))

	idx0Val := obj.GetIdx(0)
	defer idx0Val.Free()
	require.EqualValues(t, "index0", idx0Val.String())

	idx1Val := obj.GetIdx(1)
	defer idx1Val.Free()
	require.EqualValues(t, "index1", idx1Val.String())

	// Test Has and HasIdx
	require.True(t, obj.Has("name"))
	require.True(t, obj.Has("value"))
	require.False(t, obj.Has("nonexistent"))

	require.True(t, obj.HasIdx(0))
	require.True(t, obj.HasIdx(1))
	require.False(t, obj.HasIdx(99))

	// Test Delete and DeleteIdx
	require.True(t, obj.Delete("flag"))
	require.False(t, obj.Has("flag"))
	require.False(t, obj.Delete("nonexistent"))

	require.True(t, obj.DeleteIdx(0))
	require.False(t, obj.HasIdx(0))
	require.False(t, obj.DeleteIdx(99))
}

// TestValuePropertyNames tests property enumeration.
func TestValuePropertyNames(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Add some properties
	obj.Set("a", ctx.String("value_a"))
	obj.Set("b", ctx.String("value_b"))
	obj.Set("c", ctx.String("value_c"))

	// Test PropertyNames
	names, err := obj.PropertyNames()
	require.NoError(t, err)
	require.Contains(t, names, "a")
	require.Contains(t, names, "b")
	require.Contains(t, names, "c")

	// Test PropertyNames with non-object value
	str := ctx.String("test")
	defer str.Free()
	_, err = str.PropertyNames()
	require.Error(t, err)
}

// TestValueFunctionCalls tests function calling methods.
func TestValueFunctionCalls(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create an object with methods
	obj := ctx.Object()
	defer obj.Free()

	obj.Set("add", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		if len(args) < 2 {
			return ctx.Int32(0)
		}
		return ctx.Int32(args[0].ToInt32() + args[1].ToInt32())
	}))

	obj.Set("multiply", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		if len(args) < 2 {
			return ctx.Int32(1)
		}
		return ctx.Int32(args[0].ToInt32() * args[1].ToInt32())
	}))

	// Test Call method
	result := obj.Call("add", ctx.Int32(3), ctx.Int32(4))
	defer result.Free()
	require.EqualValues(t, 7, result.ToInt32())

	// Test Call with different method
	result2 := obj.Call("multiply", ctx.Int32(3), ctx.Int32(4))
	defer result2.Free()
	require.EqualValues(t, 12, result2.ToInt32())

	// Test Call with non-existent method
	errorResult := obj.Call("nonexistent", ctx.Int32(1))
	defer errorResult.Free()
	require.True(t, errorResult.IsError())

	// Test Call on non-object
	str := ctx.String("test")
	defer str.Free()
	errorResult2 := str.Call("method")
	defer errorResult2.Free()
	require.True(t, errorResult2.IsError())

	// Test Execute method
	addFunc := obj.Get("add")
	defer addFunc.Free()

	execResult := addFunc.Execute(ctx.Null(), ctx.Int32(5), ctx.Int32(6))
	defer execResult.Free()
	require.EqualValues(t, 11, execResult.ToInt32())

	// Test Execute on non-function
	nonFunc := ctx.String("not a function")
	defer nonFunc.Free()
	execError := nonFunc.Execute(ctx.Null())
	defer execError.Free()
	require.True(t, execError.IsError())
}

// TestValueConstructor tests constructor calling.
func TestValueConstructor(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create a constructor function
	result, err := ctx.Eval(`
        function TestClass(value) {
            this.value = value;
        }
        TestClass.prototype.getValue = function() {
            return this.value;
        };
        TestClass;
    `)
	require.NoError(t, err)
	defer result.Free()

	// Test CallConstructor
	instance := result.CallConstructor(ctx.String("test_value"))
	defer instance.Free()
	require.True(t, instance.IsObject())

	valueProperty := instance.Get("value")
	defer valueProperty.Free()
	require.EqualValues(t, "test_value", valueProperty.String())

	// Test New (alias for CallConstructor)
	instance2 := result.New(ctx.String("test_value2"))
	defer instance2.Free()
	require.True(t, instance2.IsObject())

	valueProperty2 := instance2.Get("value")
	defer valueProperty2.Free()
	require.EqualValues(t, "test_value2", valueProperty2.String())

	// Test CallConstructor on non-constructor
	nonConstructor := ctx.String("not a constructor")
	defer nonConstructor.Free()
	errorResult := nonConstructor.CallConstructor()
	defer errorResult.Free()
	require.True(t, errorResult.IsError())
}

// TestValueError tests error handling.
func TestValueError(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create an error value
	testErr := errors.New("test error message")
	errorVal := ctx.Error(testErr)
	defer errorVal.Free()

	// Test ToError method
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

	// Test error with all properties
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
	errorStr := complexConvertedErr.Error()
	require.Contains(t, errorStr, "CustomError")
	require.Contains(t, errorStr, "complex error")
}

// TestValueGlobalInstanceof tests instanceof checking.
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

	// Test Date instanceof
	date, err := ctx.Eval(`new Date()`)
	require.NoError(t, err)
	defer date.Free()
	require.True(t, date.GlobalInstanceof("Date"))
	require.False(t, date.GlobalInstanceof("Array"))

	// Test with non-existent constructor
	str := ctx.String("test")
	defer str.Free()
	require.False(t, str.GlobalInstanceof("NonExistentConstructor"))

	// Test with primitive value
	require.False(t, str.GlobalInstanceof("String"))
}

// TestValueTypeChecking tests all Is* methods.
func TestValueTypeChecking(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test IsNumber
	numVal := ctx.Int32(42)
	defer numVal.Free()
	require.True(t, numVal.IsNumber())

	floatVal := ctx.Float64(3.14)
	defer floatVal.Free()
	require.True(t, floatVal.IsNumber())

	// Test IsBigInt
	bigIntVal := ctx.BigInt64(123456789)
	defer bigIntVal.Free()
	require.True(t, bigIntVal.IsBigInt())
	require.False(t, numVal.IsBigInt())

	// Test IsBool
	boolVal := ctx.Bool(true)
	defer boolVal.Free()
	require.True(t, boolVal.IsBool())
	require.False(t, numVal.IsBool())

	// Test IsNull
	nullVal := ctx.Null()
	defer nullVal.Free()
	require.True(t, nullVal.IsNull())
	require.False(t, numVal.IsNull())

	// Test IsUndefined
	undefinedVal := ctx.Undefined()
	defer undefinedVal.Free()
	require.True(t, undefinedVal.IsUndefined())
	require.False(t, numVal.IsUndefined())

	// Test IsUninitialized
	uninitVal := ctx.Uninitialized()
	defer uninitVal.Free()
	require.True(t, uninitVal.IsUninitialized())
	require.False(t, numVal.IsUninitialized())

	// Test IsString
	strVal := ctx.String("test")
	defer strVal.Free()
	require.True(t, strVal.IsString())
	require.False(t, numVal.IsString())

	// Test IsObject
	objVal := ctx.Object()
	defer objVal.Free()
	require.True(t, objVal.IsObject())
	require.False(t, numVal.IsObject())

	// Test IsArray
	arrVal, err := ctx.Eval(`[1, 2, 3]`)
	require.NoError(t, err)
	defer arrVal.Free()
	require.True(t, arrVal.IsArray())
	require.False(t, objVal.IsArray())

	// Test IsSymbol
	symVal, err := ctx.Eval(`Symbol('test')`)
	require.NoError(t, err)
	defer symVal.Free()
	require.True(t, symVal.IsSymbol())
	require.False(t, strVal.IsSymbol())

	// Test IsError
	errVal := ctx.Error(errors.New("test error"))
	defer errVal.Free()
	require.True(t, errVal.IsError())
	require.False(t, strVal.IsError())

	// Test IsFunction
	funcVal := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.Null()
	})
	defer funcVal.Free()
	require.True(t, funcVal.IsFunction())
	require.False(t, strVal.IsFunction())

	// Test IsConstructor
	constructorVal, err := ctx.Eval(`function TestConstructor() {}; TestConstructor`)
	require.NoError(t, err)
	defer constructorVal.Free()
	require.True(t, constructorVal.IsConstructor())
	require.False(t, strVal.IsConstructor())

	// Test IsException
	exceptionVal, err := ctx.Eval(`throw new Error("test"); "never reached"`)
	require.Error(t, err)
	if exceptionVal.IsException() {
		defer exceptionVal.Free()
		require.True(t, exceptionVal.IsException())
	}

	// Test IsPromise
	promiseVal, err := ctx.Eval(`new Promise((resolve) => resolve("test"))`)
	require.NoError(t, err)
	defer promiseVal.Free()
	require.True(t, promiseVal.IsPromise())
	require.False(t, strVal.IsPromise())
}

// TestValueEdgeCases tests various edge cases and error conditions.
func TestValueEdgeCases(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test empty string operations
	emptyStr := ctx.String("")
	defer emptyStr.Free()
	require.EqualValues(t, "", emptyStr.String())
	require.EqualValues(t, "\"\"", emptyStr.JSONStringify())

	// Test zero values
	zeroInt := ctx.Int32(0)
	defer zeroInt.Free()
	require.EqualValues(t, 0, zeroInt.ToInt32())
	require.False(t, zeroInt.ToBool()) // 0 is falsy

	// Test negative values
	negativeInt := ctx.Int32(-42)
	defer negativeInt.Free()
	require.EqualValues(t, -42, negativeInt.ToInt32())

	negativeFloat := ctx.Float64(-3.14)
	defer negativeFloat.Free()
	require.InDelta(t, -3.14, negativeFloat.ToFloat64(), 0.001)

	// Test large numbers
	largeInt := ctx.Int64(1234567890123456) // Close to max int64 but safe for JS
	defer largeInt.Free()
	require.EqualValues(t, 1234567890123456, largeInt.ToInt64())

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

	obj.Set("123", ctx.String("numeric key"))
	numericKeyVal := obj.Get("123")
	defer numericKeyVal.Free()
	require.EqualValues(t, "numeric key", numericKeyVal.String())

	// Test array with mixed types
	mixedArr, err := ctx.Eval(`[1, "string", true, null, undefined, {}]`)
	require.NoError(t, err)
	defer mixedArr.Free()
	require.True(t, mixedArr.IsArray())
	require.EqualValues(t, 6, mixedArr.Len())

	// Test function call with no arguments
	obj.Set("noArgsFunc", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.String("no args called")
	}))

	noArgsResult := obj.Call("noArgsFunc")
	defer noArgsResult.Free()
	require.EqualValues(t, "no args called", noArgsResult.String())
}

// TestValueComplexOperations tests complex value operations.
func TestValueComplexOperations(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test nested object operations
	nested := ctx.Object()
	defer nested.Free()

	inner := ctx.Object()
	inner.Set("value", ctx.String("inner_value"))

	nested.Set("inner", inner)
	nested.Set("array", func() quickjs.Value {
		arr, _ := ctx.Eval(`[1, 2, 3]`)
		return arr
	}())

	// Get nested property
	retrievedInner := nested.Get("inner")
	defer retrievedInner.Free()
	innerValue := retrievedInner.Get("value")
	defer innerValue.Free()
	require.EqualValues(t, "inner_value", innerValue.String())

	// Get array from object
	retrievedArray := nested.Get("array")
	defer retrievedArray.Free()
	require.True(t, retrievedArray.IsArray())
	require.EqualValues(t, 3, retrievedArray.Len())

	// Test complex function with multiple argument types
	complexFunc := ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		result := ctx.Object()
		result.Set("argCount", ctx.Int32(int32(len(args))))

		if len(args) > 0 {
			result.Set("firstArg", args[0])
		}
		if len(args) > 1 {
			result.Set("secondArg", args[1])
		}

		return result
	})
	defer complexFunc.Free()

	// Call with mixed arguments
	funcResult := complexFunc.Execute(ctx.Null(),
		ctx.String("test"),
		ctx.Int32(42),
		ctx.Bool(true))
	defer funcResult.Free()

	argCount := funcResult.Get("argCount")
	defer argCount.Free()
	require.EqualValues(t, 3, argCount.ToInt32())

	firstArg := funcResult.Get("firstArg")
	defer firstArg.Free()
	require.EqualValues(t, "test", firstArg.String())

	secondArg := funcResult.Get("secondArg")
	defer secondArg.Free()
	require.EqualValues(t, 42, secondArg.ToInt32())
}

// TestValueMemoryManagement tests proper memory management patterns.
func TestValueMemoryManagement(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test creating and freeing many values
	for i := 0; i < 1000; i++ {
		val := ctx.String("test")
		val.Free()
	}

	// Test creating nested structures and freeing them
	for i := 0; i < 100; i++ {
		obj := ctx.Object()
		inner := ctx.Object()
		inner.Set("value", ctx.String("test"))
		obj.Set("inner", inner)

		// Access the nested value
		retrieved := obj.Get("inner").Get("value")
		require.EqualValues(t, "test", retrieved.String())

		// Free all values
		retrieved.Free()
		inner.Free()
		obj.Free()
	}

	// Test that context still works after many operations
	final := ctx.String("final test")
	defer final.Free()
	require.EqualValues(t, "final test", final.String())
}
