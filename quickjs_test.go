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
	rt := quickjs.NewRuntime(
		quickjs.WithExecuteTimeout(30),
		quickjs.WithMemoryLimit(128*1024),
		quickjs.WithGCThreshold(256*1024),
		quickjs.WithMaxStackSize(65534),
		quickjs.WithCanBlock(true),
		quickjs.WithStripInfo(1),
	)
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
	ctx.Loop()

	asyncRet, _ := ctx.Eval("ret")
	defer asyncRet.Free()

	fmt.Println(asyncRet.String())

	// Output:
	// Hello Javascript!
	// Hello Golang!
	// Hello Async Function!

}

func TestRuntimeGC(t *testing.T) {
	rt := quickjs.NewRuntime(
		quickjs.WithExecuteTimeout(30),
		quickjs.WithMemoryLimit(128*1024),
		quickjs.WithGCThreshold(256*1024),
		quickjs.WithMaxStackSize(65534),
		quickjs.WithCanBlock(true),
	)
	defer rt.Close()

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
	rt.SetMemoryLimit(128 * 1024) //512KB

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
	require.EqualValues(t, true, ctx.Bool(true).ToBool())
	require.EqualValues(t, float64(0.1), ctx.Float64(0.1).ToFloat64())
	require.EqualValues(t, int32(1), ctx.Int32(1).ToInt32())
	require.EqualValues(t, int64(1), ctx.Int64(1).ToInt64())
	require.EqualValues(t, uint32(1), ctx.Uint32(1).ToUint32())

	require.EqualValues(t, big.NewInt(1), ctx.BigInt64(1).ToBigInt())
	require.EqualValues(t, big.NewInt(1), ctx.BigUint64(1).ToBigInt())
	require.EqualValues(t, false, ctx.Float64(0.1).IsBigInt())

	// Array creation is now in the collection package
	// See collection/array_test.go for Array tests

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

func TestEvalFile(t *testing.T) {
	// enable module import
	rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	result, err := ctx.EvalFile("./test/hello_module.js")
	defer result.Free()
	require.NoError(t, err)

	require.EqualValues(t, 55, ctx.Globals().Get("result").ToInt32())

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

	require.EqualValues(t, 55, result.ToInt32())
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

				res <- result.ToInt64()

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
		arg_x := args[0].ToInt32()
		arg_y := args[1].ToInt32()
		return ctx.Int32(arg_x * arg_y)
	}))

	// invoke js function by go
	f_func := test.Get("F")
	defer f_func.Free()

	ret_execute := f_func.Execute(ctx.Null(), ctx.Int32(3), ctx.Int32(3))
	require.True(t, ret_execute.IsNumber() && ret_execute.ToInt32() == 9)

	ret_invoke := ctx.Invoke(f_func, ctx.Null(), ctx.Int32(2), ctx.Int32(3))
	require.True(t, ret_invoke.IsNumber() && ret_invoke.ToInt32() == 6)

	// call js function by go
	F_ret := test.Call("F", ctx.Int32(2), ctx.Int32(3))
	defer F_ret.Free()
	require.True(t, F_ret.IsNumber() && F_ret.ToInt32() == 6)

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

	// wait for job resolve
	ctx.Loop()

	// testAsync
	ret2, _ := ctx.Eval(`
		testAsync('Hello ', 'Async').then(v => ret = ret + v)
	`)
	defer ret2.Free()

	// wait promise execute
	ctx.Loop()

	ret3, _ := ctx.Eval("ret")
	defer ret3.Free()

	require.EqualValues(t, "Hello Async", ret3.String())
}

func TestSetInterruptHandlerContext(t *testing.T) {
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

// This test shows how the interrupt handler is not specific to the context.
// So it is cleaner to set it on the runtime instead.
func TestSetInterruptHandlerContextIsSetToRuntime(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx1 := rt.NewContext()
	defer ctx1.Close()
	ctx2 := rt.NewContext()
	defer ctx2.Close()

	startTime := time.Now().Unix()

	// Set it to context 1
	ctx1.SetInterruptHandler(func() int {
		if time.Now().Unix()-startTime > 1 {
			return 1
		}
		return 0
	})

	// use it in context 2
	ret, err := ctx2.Eval(`while(true){}`)
	defer ret.Free()

	assert.Error(t, err, "expected interrupted by quickjs")
	require.Equal(t, "InternalError: interrupted", err.Error())
}

func TestSetInterruptHandlerRuntime(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	startTime := time.Now().Unix()

	rt.SetInterruptHandler(func() int {
		if time.Now().Unix()-startTime > 1 {
			return 1
		}
		return 0
	})

	ctx := rt.NewContext()
	defer ctx.Close()
	ret, err := ctx.Eval(`while(true){}`)
	defer ret.Free()

	assert.Error(t, err, "expected interrupted by quickjs")
	require.Equal(t, "InternalError: interrupted", err.Error())
}

func TestSetExecuteTimeout(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	rt.SetExecuteTimeout(3)

	ret, err := ctx.Eval(`while(true){}`)
	defer ret.Free()

	assert.Error(t, err, "expected interrupted by quickjs")
	require.Equal(t, "InternalError: interrupted", err.Error())
}

func TestSetTimeout(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	ret, _ := ctx.Eval(`
		var a = false;
		setTimeout(() => {
			a = true;
		}, 50);
	`)
	defer ret.Free()

	ctx.Loop()

	a, _ := ctx.Eval("a")
	defer a.Free()

	require.EqualValues(t, true, a.ToBool())
}

func TestAwait(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// async function bind to global
	ctx.Globals().Set("testAsync", ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) quickjs.Value {
		return promise.Call("resolve", ctx.String(args[0].String()+args[1].String()))
	}))

	// testAwait
	promise, _ := ctx.Eval("testAsync('Hello ', 'Await')")
	require.EqualValues(t, true, promise.IsPromise())

	promiseAwait, _ := ctx.Await(promise)
	require.EqualValues(t, "Hello Await", promiseAwait.String())

	promiseAwaitEval, _ := ctx.Eval("testAsync('Hello ', 'AwaitEval')", quickjs.EvalAwait(true))
	require.EqualValues(t, "Hello AwaitEval", promiseAwaitEval.String())

}

func TestModule(t *testing.T) {
	// enable module import
	rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// eval module
	r1, err := ctx.EvalFile("./test/hello_module.js")
	defer r1.Free()
	require.NoError(t, err)
	require.EqualValues(t, 55, ctx.Globals().Get("result").ToInt32())

	// load module
	r2, err := ctx.LoadModuleFile("./test/fib_module.js", "fib_foo")
	defer r2.Free()
	require.NoError(t, err)

	// call module
	r3, err := ctx.Eval(`
	import {fib} from 'fib_foo';
	globalThis.result = fib(11);
	`)
	defer r3.Free()
	require.NoError(t, err)

	require.EqualValues(t, 89, ctx.Globals().Get("result").ToInt32())

	ctx2 := rt.NewContext()
	defer ctx2.Close()
	// load module from bytecode
	buf, err := ctx2.CompileModule("./test/fib_module.js", "fib_foo2")
	require.NoError(t, err)

	r4, err := ctx2.LoadModuleBytecode(buf)
	defer r4.Free()
	require.NoError(t, err)

	r5, err := ctx2.Eval(`
	import {fib} from 'fib_foo2';
	globalThis.result = fib(12);
	`)
	defer r5.Free()
	require.NoError(t, err)

	require.EqualValues(t, 144, ctx2.Globals().Get("result").ToInt32())

}

func TestModule2(t *testing.T) {
	// enable module import
	rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// load module from bytecode
	buf, err := ctx.CompileModule("./test/fib_module.js", "fib_foo")
	require.NoError(t, err)
	r4, err := ctx.LoadModuleBytecode(buf)
	defer r4.Free()
	require.NoError(t, err)

	r5, err := ctx.Eval(`
	import {fib} from 'fib_foo';
	globalThis.result = fib(12);
	`)
	defer r5.Free()
	require.NoError(t, err)
	require.EqualValues(t, 144, ctx.Globals().Get("result").ToInt32())
}

func TestClassConstructor(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	ret, err := ctx.Eval(`
		class Foo {
			x = 0;	
			constructor(x) {
				this.x = x;
			}
		}
		globalThis.Foo = Foo;
	`)
	defer ret.Free()
	require.NoError(t, err)

	Foo := ctx.Globals().Get("Foo")
	defer Foo.Free()

	fooInstance := Foo.New(ctx.Int32(10))
	defer fooInstance.Free()

	x := fooInstance.Get("x")
	defer x.Free()

	require.EqualValues(t, 10, x.ToInt32())

}
