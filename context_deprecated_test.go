package quickjs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDeprecatedContextAPIs tests all deprecated Context methods to ensure they still work
// Each deprecated method is called once for test coverage
func TestDeprecatedContextAPIs(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("DeprecatedValueCreation", func(t *testing.T) {
		// Test all deprecated value creation methods
		val1 := ctx.Null()
		defer val1.Free()
		require.True(t, val1.IsNull())

		val2 := ctx.Undefined()
		defer val2.Free()
		require.True(t, val2.IsUndefined())

		val3 := ctx.Uninitialized()
		defer val3.Free()
		require.True(t, val3.IsUninitialized())

		val4 := ctx.Bool(true)
		defer val4.Free()
		require.True(t, val4.IsBool())

		val5 := ctx.Int32(42)
		defer val5.Free()
		require.True(t, val5.IsNumber())

		val6 := ctx.Int64(1234567890)
		defer val6.Free()
		require.True(t, val6.IsNumber())

		val7 := ctx.Uint32(42)
		defer val7.Free()
		require.True(t, val7.IsNumber())

		val8 := ctx.BigInt64(9223372036854775807)
		defer val8.Free()
		require.True(t, val8.IsBigInt())

		val9 := ctx.BigUint64(18446744073709551615)
		defer val9.Free()
		require.True(t, val9.IsBigInt())

		val10 := ctx.Float64(3.14159)
		defer val10.Free()
		require.True(t, val10.IsNumber())

		val11 := ctx.String("test")
		defer val11.Free()
		require.True(t, val11.IsString())

		val12 := ctx.Object()
		defer val12.Free()
		require.True(t, val12.IsObject())

		val13 := ctx.ArrayBuffer([]byte{1, 2, 3})
		defer val13.Free()
		require.True(t, val13.IsByteArray())

		val14 := ctx.Error(errors.New("test error"))
		defer val14.Free()
		require.True(t, val14.IsError())
	})

	t.Run("DeprecatedTypedArrays", func(t *testing.T) {
		// Test all deprecated TypedArray creation methods
		val1 := ctx.Int8Array([]int8{1, 2, 3})
		defer val1.Free()
		require.True(t, val1.IsInt8Array())

		val2 := ctx.Uint8Array([]uint8{1, 2, 3})
		defer val2.Free()
		require.True(t, val2.IsUint8Array())

		val3 := ctx.Uint8ClampedArray([]uint8{1, 2, 3})
		defer val3.Free()
		require.True(t, val3.IsUint8ClampedArray())

		val4 := ctx.Int16Array([]int16{1, 2, 3})
		defer val4.Free()
		require.True(t, val4.IsInt16Array())

		val5 := ctx.Uint16Array([]uint16{1, 2, 3})
		defer val5.Free()
		require.True(t, val5.IsUint16Array())

		val6 := ctx.Int32Array([]int32{1, 2, 3})
		defer val6.Free()
		require.True(t, val6.IsInt32Array())

		val7 := ctx.Uint32Array([]uint32{1, 2, 3})
		defer val7.Free()
		require.True(t, val7.IsUint32Array())

		val8 := ctx.Float32Array([]float32{1.0, 2.0, 3.0})
		defer val8.Free()
		require.True(t, val8.IsFloat32Array())

		val9 := ctx.Float64Array([]float64{1.0, 2.0, 3.0})
		defer val9.Free()
		require.True(t, val9.IsFloat64Array())

		val10 := ctx.BigInt64Array([]int64{1, 2, 3})
		defer val10.Free()
		require.True(t, val10.IsBigInt64Array())

		val11 := ctx.BigUint64Array([]uint64{1, 2, 3})
		defer val11.Free()
		require.True(t, val11.IsBigUint64Array())
	})

	t.Run("DeprecatedFunctions", func(t *testing.T) {
		// Test deprecated Function method
		fn := ctx.Function(func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewString("hello")
		})
		defer fn.Free()
		require.True(t, fn.IsFunction())

		// Test function execution
		result := fn.Execute(ctx.NewNull())
		defer result.Free()
		require.Equal(t, "hello", result.ToString())

		// Test deprecated AsyncFunction method
		asyncFn := ctx.AsyncFunction(func(ctx *Context, this *Value, promise *Value, args []*Value) *Value {
			resolve := promise.Get("resolve")
			defer resolve.Free()
			resolve.Execute(ctx.NewUndefined(), ctx.NewString("async hello"))
			return ctx.NewUndefined()
		})
		defer asyncFn.Free()
		require.True(t, asyncFn.IsFunction())

		// Test deprecated Promise method
		promise := ctx.Promise(func(resolve, reject func(*Value)) {
			resolve(ctx.NewString("promise result"))
		})
		require.True(t, promise.IsPromise())

		// Test promise result
		awaitedResult := promise.Await()
		defer awaitedResult.Free()
		require.Equal(t, "promise result", awaitedResult.ToString())
	})

	t.Run("DeprecatedAtomCreation", func(t *testing.T) {
		// Test deprecated Atom creation methods
		atom1 := ctx.Atom("test")
		defer atom1.Free()
		require.Equal(t, "test", atom1.ToString())

		atom2 := ctx.AtomIdx(123)
		defer atom2.Free()
		require.NotNil(t, atom2)
	})

	t.Run("DeprecatedInvoke", func(t *testing.T) {
		// Test deprecated Invoke method
		fn := ctx.NewFunction(func(ctx *Context, this *Value, args []*Value) *Value {
			if len(args) > 0 {
				return ctx.NewString("invoked with: " + args[0].ToString())
			}
			return ctx.NewString("invoked without args")
		})
		defer fn.Free()

		// Test with arguments
		result1 := ctx.Invoke(fn, ctx.NewNull(), ctx.NewString("test"))
		defer result1.Free()
		require.Equal(t, "invoked with: test", result1.ToString())

		// Test without arguments
		result2 := ctx.Invoke(fn, ctx.NewNull())
		defer result2.Free()
		require.Equal(t, "invoked without args", result2.ToString())
	})
}

// TestDeprecatedContextComplexScenarios tests complex scenarios with deprecated Context APIs
func TestDeprecatedContextComplexScenarios(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("MixedDeprecatedAndNewAPIs", func(t *testing.T) {
		// Mix deprecated and new APIs to ensure compatibility
		oldString := ctx.String("old api")    // deprecated
		newString := ctx.NewString("new api") // new
		defer oldString.Free()
		defer newString.Free()

		require.Equal(t, "old api", oldString.ToString())
		require.Equal(t, "new api", newString.ToString())

		// Both should work the same way
		require.True(t, oldString.IsString())
		require.True(t, newString.IsString())
	})

	t.Run("DeprecatedFunctionWithNewValues", func(t *testing.T) {
		// Use deprecated Function with new value creation methods
		fn := ctx.Function(func(ctx *Context, this *Value, args []*Value) *Value {
			// Use new API inside deprecated function
			return ctx.NewString("mixed usage")
		})
		defer fn.Free()

		result := fn.Execute(ctx.NewNull()) // new API for execution
		defer result.Free()
		require.Equal(t, "mixed usage", result.ToString())
	})

	t.Run("DeprecatedTypedArrayConversions", func(t *testing.T) {
		// Test deprecated TypedArray creation with conversions
		data := []int32{10, 20, 30, 40, 50}
		arr := ctx.Int32Array(data) // deprecated
		defer arr.Free()

		// Convert back using new API methods
		converted, err := arr.ToInt32Array()
		require.NoError(t, err)
		require.Equal(t, data, converted)
	})
}
