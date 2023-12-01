package quickjs_test

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Example() {

	// Create a new runtime
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Create a new context
	ctx := rt.NewContext()
	defer ctx.Close()

	// Create a new object
	test := ctx.Object()
	defer test.Free()
	// bind properties to the object
	test.Set("A", test.Context().String("String A"))
	test.Set("B", ctx.Int32(0))
	test.Set("C", ctx.Bool(false))
	// bind go function to js object
	test.Set("hello", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.String("Hello " + args[0].String())
	}))

	// bind "test" object to global object
	ctx.Globals().Set("test", test)

	// call js function by js
	js_ret, _ := ctx.Eval(`test.hello("Javascript!")`)
	fmt.Println(js_ret.String())

	// call js function by go
	go_ret := ctx.Globals().Get("test").Call("hello", ctx.String("Golang!"))
	fmt.Println(go_ret.String())

	//bind go function to Javascript async function
	ctx.Globals().Set("testAsync", ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		return promise.Call("resolve", ctx.String("Hello Async Function!"))
	}))

	ret, _ := ctx.Eval(`
			var ret;
			testAsync().then(v => ret = v)
		`)
	defer ret.Free()

	// wait for promise resolve
	rt.ExecuteAllPendingJobs()

	asyncRet, _ := ctx.Eval("ret")
	defer asyncRet.Free()

	fmt.Println(asyncRet.String())

	// Output:
	// Hello Javascript!
	// Hello Golang!
	// Hello Async Function!

}

func TestRuntimeGC(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// set runtime options
	rt.SetGCThreshold(256 * 1024)

	ctx := rt.NewContext()
	defer ctx.Close()

	rt.RunGC()

	result, _ := ctx.Eval(`"Hello GC!"`)
	defer result.Free()

	require.EqualValues(t, "Hello GC!", result.String())
}

func TestRuntimeMemoryLimit(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// set runtime options
	rt.SetMemoryLimit(256 * 1024) //512KB

	ctx := rt.NewContext()
	defer ctx.Close()

	result, err := ctx.Eval(`var array = []; while (true) { array.push(null) }`)
	defer result.Free()

	if assert.Error(t, err, "expected a memory limit violation") {
		require.Equal(t, "InternalError: out of memory", err.Error())
	}

}

func TestRuntimeStackSize(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	rt.SetMaxStackSize(65534)

	ctx := rt.NewContext()
	defer ctx.Close()

	result, err := ctx.Eval(`
	function fib(n)
	{
		if (n <= 0)
			return 0;
		else if (n == 1)
			return 1;
		else
			return fib(n - 1) + fib(n - 2);
	}
	fib(128)
	`)
	defer result.Free()

	if assert.Error(t, err, "expected a memory limit violation") {
		require.Equal(t, "InternalError: stack overflow", err.Error())
	}
}

func TestThrowError(t *testing.T) {
	expected := errors.New("custom error")

	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.Globals().Set("A", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowError(expected)
	}))

	_, actual := ctx.Eval("A()")
	require.Error(t, actual)
	require.EqualValues(t, "Error: "+expected.Error(), actual.Error())
}

func TestThrowInternalError(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.Globals().Set("A", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowInternalError("%s", "custom error")
	}))

	_, actual := ctx.Eval("A()")
	require.Error(t, actual)
	require.EqualValues(t, "InternalError: custom error", actual.Error())
}

func TestThrowRangeError(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.Globals().Set("A", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowRangeError("%s", "custom error")
	}))

	_, actual := ctx.Eval("A()")
	require.Error(t, actual)
	require.EqualValues(t, "RangeError: custom error", actual.Error())
}

func TestThrowReferenceError(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.Globals().Set("A", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowReferenceError("%s", "custom error")
	}))

	_, actual := ctx.Eval("A()")
	require.Error(t, actual)
	require.EqualValues(t, "ReferenceError: custom error", actual.Error())
}

func TestThrowSyntaxError(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.Globals().Set("A", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowSyntaxError("%s", "custom error")
	}))

	_, actual := ctx.Eval("A()")
	require.Error(t, actual)
	require.EqualValues(t, "SyntaxError: custom error", actual.Error())
}

func TestThrowTypeError(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.Globals().Set("A", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.ThrowTypeError("%s", "custom error")
	}))

	_, actual := ctx.Eval("A()")
	require.Error(t, actual)
	require.EqualValues(t, "TypeError: custom error", actual.Error())
}

func TestValue(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// require.EqualValues(t, big.NewInt(1), ctx.BigUint64(uint64(1)).)
	require.EqualValues(t, true, ctx.Bool(true).IsBool())
	require.EqualValues(t, true, ctx.Bool(true).Bool())
	require.EqualValues(t, float64(0.1), ctx.Float64(0.1).Float64())
	require.EqualValues(t, int32(1), ctx.Int32(1).Int32())
	require.EqualValues(t, int64(1), ctx.Int64(1).Int64())
	require.EqualValues(t, uint32(1), ctx.Uint32(1).Uint32())

	require.EqualValues(t, big.NewInt(1), ctx.BigInt64(1).BigInt())
	require.EqualValues(t, big.NewInt(1), ctx.BigUint64(1).BigInt())

	require.EqualValues(t, false, ctx.Float64(0.1).IsBigDecimal())
	require.EqualValues(t, false, ctx.Float64(0.1).IsBigFloat())
	require.EqualValues(t, false, ctx.Float64(0.1).IsBigInt())

	a := ctx.Array()
	defer a.Free()
	//require.True(t, a.IsArray())

	o := ctx.Object()
	defer o.Free()
	require.True(t, o.IsObject())

	s := ctx.String("hello")
	defer s.Free()
	require.EqualValues(t, true, s.IsString())

	n := ctx.Null()
	defer n.Free()
	require.True(t, n.IsNull())

	ud := ctx.Undefined()
	defer ud.Free()
	require.True(t, ud.IsUndefined())

	ui := ctx.Uninitialized()
	defer ui.Free()
	require.True(t, ui.IsUninitialized())

	sym, _ := ctx.Eval("Symbol()")
	defer sym.Free()
	require.True(t, sym.IsSymbol())

	err := ctx.Error(errors.New("error"))
	defer err.Free()
	require.True(t, err.IsError())
}

func TestEvalBytecode(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()
	jsStr := `
	function fib(n)
	{
		if (n <= 0)
			return 0;
		else if (n == 1)
			return 1;
		else
			return fib(n - 1) + fib(n - 2);
	}
	fib(10)
	`
	buf, err := ctx.Compile(jsStr)
	require.NoError(t, err)

	rt2 := quickjs.NewRuntime()
	defer rt2.Close()

	ctx2 := rt2.NewContext()
	defer ctx2.Close()

	result, err := ctx2.EvalBytecode(buf)
	require.NoError(t, err)

	require.EqualValues(t, 55, result.Int32())
}
func TestBadSyntax(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	_, err := ctx.Compile(`"bad syntax'`)
	require.Error(t, err)

}

func TestBadBytecode(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	buf := make([]byte, 1)
	_, err := ctx.EvalBytecode(buf)
	require.Error(t, err)

}

func TestArrayBuffer(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	binaryData := []uint8{1, 2, 3, 4, 5}
	value := ctx.ArrayBuffer(binaryData)
	defer value.Free()
	for i := 1; i <= len(binaryData); i++ {
		data, err := value.ToByteArray(uint(i))
		assert.NoError(t, err)
		//fmt.Println(data)
		assert.EqualValues(t, data, binaryData[:i])
	}
	_, err := value.ToByteArray(uint(len(binaryData)) + 1)
	assert.Error(t, err)
	assert.True(t, value.IsByteArray())
	binaryLen := len(binaryData)
	assert.Equal(t, value.ByteLen(), int64(binaryLen))
}

func TestConcurrency(t *testing.T) {
	n := 32
	m := 10000

	var wg sync.WaitGroup
	wg.Add(n)

	req := make(chan struct{}, n)
	res := make(chan int64, m)

	for i := 0; i < n; i++ {
		go func() {

			defer wg.Done()

			rt := quickjs.NewRuntime()
			defer rt.Close()

			ctx := rt.NewContext()
			defer ctx.Close()

			for range req {
				result, err := ctx.Eval(`new Date().getTime()`)
				require.NoError(t, err)

				res <- result.Int64()

				result.Free()
			}
		}()
	}

	for i := 0; i < m; i++ {
		req <- struct{}{}
	}
	close(req)

	wg.Wait()

	for i := 0; i < m; i++ {
		<-res
	}
}

func TestJson(t *testing.T) {
	// Create a new runtime
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Create a new context
	ctx := rt.NewContext()
	defer ctx.Close()

	// Create a  object from json
	fooObj := ctx.ParseJSON(`{"foo":"bar"}`)
	defer fooObj.Free()

	// JSONStringify
	jsonStr := fooObj.JSONStringify()
	require.EqualValues(t, "{\"foo\":\"bar\"}", jsonStr)
}

func TestObject(t *testing.T) {
	// Create a new runtime
	rt := quickjs.NewRuntime()
	defer rt.Close()

	// Create a new context
	ctx := rt.NewContext()
	defer ctx.Close()

	// Create a new object
	test := ctx.Object()
	test.Set("A", test.Context().String("String A"))
	test.Set("B", ctx.Int32(0))
	test.Set("C", ctx.Bool(false))
	ctx.Globals().Set("test", test)

	result, err := ctx.Eval(`Object.keys(test).map(key => test[key]).join(",")`)
	require.NoError(t, err)
	defer result.Free()

	// eval js code
	require.EqualValues(t, "String A,0,false", result.String())

	// set function
	test.Set("F", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		arg_x := args[0].Int32()
		arg_y := args[1].Int32()
		return ctx.Int32(arg_x * arg_y)
	}))

	// call js function by go
	F_ret := test.Call("F", ctx.Int32(2), ctx.Int32(3))
	defer F_ret.Free()
	require.True(t, F_ret.IsNumber() && F_ret.Int32() == 6)

	// invoke js function by go
	f_func := test.Get("F")
	defer f_func.Free()
	ret := ctx.Invoke(f_func, ctx.Null(), ctx.Int32(2), ctx.Int32(3))
	require.True(t, ret.IsNumber() && ret.Int32() == 6)

	// test error call
	F_ret_err := test.Call("A", ctx.Int32(2), ctx.Int32(3))
	defer F_ret_err.Free()
	require.Error(t, F_ret_err.Error())

	// get object property
	require.True(t, test.Has("A"))
	require.True(t, test.Get("A").String() == "String A")

	// get object all property
	pNames, _ := test.PropertyNames()
	require.True(t, strings.Join(pNames[:], ",") == "A,B,C,F")

	// delete object property
	test.Delete("C")
	pNames, _ = test.PropertyNames()
	require.True(t, strings.Join(pNames[:], ",") == "A,B,F")

}

func TestArray(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	test := ctx.Array()
	for i := int64(0); i < 3; i++ {
		test.Push(ctx.String(fmt.Sprintf("test %d", i)))
		require.True(t, test.HasIdx(i))
	}
	require.EqualValues(t, 3, test.Len())

	for i := int64(0); int64(i) < test.Len(); i++ {
		require.EqualValues(t, fmt.Sprintf("test %d", i), test.ToValue().GetIdx(i).String())
	}

	ctx.Globals().Set("test", test.ToValue())

	result, err := ctx.Eval(`test.map(v => v.toUpperCase())`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, `TEST 0,TEST 1,TEST 2`, result.String())

	dFlag, _ := test.Delete(0)
	require.True(t, dFlag)
	result, err = ctx.Eval(`test.map(v => v.toUpperCase())`)
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, `TEST 1,TEST 2`, result.String())

	first, err := test.Get(0)
	if err != nil {
		fmt.Println(err)
	}
	require.EqualValues(t, first.String(), "test 1")

	test.Push([]quickjs.Value{ctx.Int32(34), ctx.Bool(false), ctx.String("445")}...)

	require.Equal(t, int(test.Len()), 5)

	err = test.Set(test.Len()-1, ctx.Int32(2))
	require.NoError(t, err)

	require.EqualValues(t, test.ToValue().String(), "test 1,test 2,34,false,2")

}

func TestMap(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	test := ctx.Map()
	defer test.Free()
	require.True(t, test.ToValue().IsMap())

	for i := int64(0); i < 3; i++ {
		test.Put(ctx.Int64(i), ctx.String(fmt.Sprintf("test %d", i)))
		require.True(t, test.Has(ctx.Int64(i)))
		testValue := test.Get(ctx.Int64(i))
		require.EqualValues(t, testValue.String(), fmt.Sprintf("test %d", i))
		//testValue.Free()
	}

	count := 0
	test.ForEach(func(key quickjs.Value, value quickjs.Value) {
		count++
		fmt.Println(fmt.Sprintf("key:%s value:%s", key.String(), value.String()))
	})
	require.EqualValues(t, count, 3)

	test.Put(ctx.Int64(3), ctx.Int64(4))
	fmt.Println("\nput after the content inside")
	count = 0
	test.ForEach(func(key quickjs.Value, value quickjs.Value) {
		count++
		fmt.Println(fmt.Sprintf("key:%s value:%s", key.String(), value.String()))
	})
	require.EqualValues(t, count, 4)

	count = 0
	test.Delete(ctx.Int64(3))
	fmt.Println("\ndelete after the content inside")
	test.ForEach(func(key quickjs.Value, value quickjs.Value) {
		if key.String() == "3" {
			panic(errors.New("map did not delete the key"))
		}
		count++
		fmt.Println(fmt.Sprintf("key:%s value:%s", key.String(), value.String()))
	})
	require.EqualValues(t, count, 3)
}

func TestSet(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	test := ctx.Set()
	defer test.Free()
	require.True(t, test.ToValue().IsSet())

	for i := int64(0); i < 3; i++ {
		test.Add(ctx.Int64(i))
		require.True(t, test.Has(ctx.Int64(i)))
	}

	count := 0
	test.ForEach(func(key quickjs.Value) {
		count++
		fmt.Println(fmt.Sprintf("value:%s", key.String()))
	})
	require.EqualValues(t, count, 3)

	test.Delete(ctx.Int64(0))
	require.True(t, !test.Has(ctx.Int64(0)))
}

func TestFunction(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.Globals().Set("test", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		return ctx.String("Hello " + args[0].String() + args[1].String())
	}))

	ret, _ := ctx.Eval(`
		test('Go ', 'JS')
	`)
	defer ret.Free()

	require.EqualValues(t, "Hello Go JS", ret.String())
}

func TestAsyncFunction(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	ctx.Globals().Set("testAsync", ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		return promise.Call("resolve", ctx.String(args[0].String()+args[1].String()))
	}))

	ret1, _ := ctx.Eval(`
		var ret = "";
	`)
	defer ret1.Free()

	ctx.ScheduleJob(func() {
		ret2, _ := ctx.Eval(`ret = ret + "Job Done: ";`)
		defer ret2.Free()
	})

	// wait for job resolve
	rt.ExecuteAllPendingJobs()

	// testAsync
	ret3, _ := ctx.Eval(`
		testAsync('Hello ', 'Async').then(v => ret = ret + v)
	`)
	defer ret3.Free()

	// wait promise execute
	rt.ExecuteAllPendingJobs()

	ret4, _ := ctx.Eval("ret")
	defer ret4.Free()

	require.EqualValues(t, "Job Done: Hello Async", ret4.String())
}

func TestSetInterruptHandler(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	startTime := time.Now().Unix()

	ctx.SetInterruptHandler(func() int {
		if time.Now().Unix()-startTime > 1 {
			return 1
		}
		return 0
	})

	ret, err := ctx.Eval(`while(true){}`)
	defer ret.Free()

	assert.Error(t, err, "expected interrupted by quickjs")
	require.Equal(t, "InternalError: interrupted", err.Error())
}
