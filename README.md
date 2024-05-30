# quickjs-go
English | [简体中文](README_zh-cn.md)

[![Test](https://github.com/buke/quickjs-go/workflows/Test/badge.svg)](https://github.com/buke/quickjs-go/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/quickjs-go/branch/main/graph/badge.svg?token=DW5RGD01AG)](https://codecov.io/gh/buke/quickjs-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/buke/quickjs-go)](https://goreportcard.com/report/github.com/buke/quickjs-go)
[![GoDoc](https://pkg.go.dev/badge/github.com/buke/quickjs-go?status.svg)](https://pkg.go.dev/github.com/buke/quickjs-go?tab=doc)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_shield)

Go bindings to QuickJS: a fast, small, and embeddable ES2020 JavaScript interpreter.

## Platform Support
we prebuilt quickjs static library for the following platforms:

|  Platform | Arch | Static Library |
| --------- | ---- | -------------- |
| Linux     | x64  | [libquickjs.a](deps/libs/linux_amd64/libquickjs.a)  |
| Linux     | arm64| [libquickjs.a](deps/libs/linux_arm64/libquickjs.a)  |
| Windows   | x64  | [libquickjs.a](deps/libs/windows_amd64/libquickjs.a)  |
| Windows   | x86  | [libquickjs.a](deps/libs/windows_386/libquickjs.a)    |
| MacOS     | x64  | [libquickjs.a](deps/libs/darwin_amd64/libquickjs.a) |
| MacOS     | arm64| [libquickjs.a](deps/libs/darwin_arm64/libquickjs.a) |

\* The windows static library is compiled based on mingw32 12.2.0. Please confirm  go version > 1.20.0

## Version Notes

| quickjs-go | QuickJS |
| ---------- | ------- |
| v0.1.x     | v2021-03-27 |
| v0.2.x     | v2023-12-09 |
| v0.3.x     | v2024-01-13 |
| v0.4.x     | v2024-02-14 |

## Features
* Evaluate script
* Compile script into bytecode and Eval from bytecode
* Operate JavaScript values and objects in Go
* Bind Go function to JavaScript async/sync function
* Simple exception throwing and catching

## Guidelines

1. Free `quickjs.Runtime` and `quickjs.Context` once you are done using them.
2. Free `quickjs.Value`'s returned by `Eval()` and `EvalFile()`. All other values do not need to be freed, as they get garbage-collected.
3. Use `ctx.Loop()` wait for promise/job result after you using promise/job
4. You may access the stacktrace of an error returned by `Eval()` or `EvalFile()` by casting it to a `*quickjs.Error`.
5. Make new copies of arguments should you want to return them in functions you created.

## Usage

```go
import "github.com/buke/quickjs-go"
```

### Run a script
```go
package main

import (
	"fmt"
    
	"github.com/buke/quickjs-go"
)

func main() {
    // Create a new runtime
	rt := quickjs.NewRuntime(
		quickjs.WithExecuteTimeout(30),
		quickjs.WithMemoryLimit(128*1024),
		quickjs.WithGCThreshold(256*1024),
		quickjs.WithMaxStackSize(65534),
		quickjs.WithCanBlock(true),
	)
    defer rt.Close()

    // Create a new context
    ctx := rt.NewContext()
    defer ctx.Close()

    ret, err := ctx.Eval("'Hello ' + 'QuickJS!'")
    if err != nil {
        println(err.Error())
    }
    fmt.Println(ret.String())
}
```


### Get/Set Javascript Object
```go
package main

import (
	"fmt"
    
	"github.com/buke/quickjs-go"
)

func main() {
    // Create a new runtime
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // Create a new context
    ctx := rt.NewContext()
    defer ctx.Close()

    test := ctx.Object()
    test.Set("A", ctx.String("String A"))
    test.Set("B", ctx.String("String B"))
    test.Set("C", ctx.String("String C"))
    ctx.Globals().Set("test", test)

    ret, _ := ctx.Eval(`Object.keys(test).map(key => test[key]).join(" ")`)
    defer ret.Free()
    fmt.Println(ret.String())
}

```


### Bind Go Funtion to Javascript async/sync function
```go
package main
import "github.com/buke/quickjs-go"

func main() {
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
	ctx.Globals().Set("testAsync", ctx.AsyncFunction(func(ctx *quickjs.Context, this quickjs.Value, promise quickjs.Value, args []quickjs.Value) {
		promise.Call("resolve", ctx.String("Hello Async Function!"))
	}))

	ret, _ := ctx.Eval(`
			var ret;
			testAsync().then(v => ret = v)
		`)
	defer ret.Free()

	// wait for promise resolve
	ctx.Loop()

    //get promise result
	asyncRet, _ := ctx.Eval("ret")
	defer asyncRet.Free()

	fmt.Println(asyncRet.String())

	// Output:
	// Hello Javascript!
	// Hello Golang!
	// Hello Async Function!
}
```

### Error Handling
```go
package main

import (
	"fmt"
    
	"github.com/buke/quickjs-go"
)

func main() {
    // Create a new runtime
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // Create a new context
    ctx := rt.NewContext()
    defer ctx.Close()

    ctx.Globals().SetFunction("A", func(ctx *Context, this Value, args []Value) Value {
        // raise error
        return ctx.ThrowError(expected)
    })

    _, actual := ctx.Eval("A()")
    fmt.Println(actual.Error())
}
```

### Bytecode Compiler
```go

package main

import (
	"fmt"
    
	"github.com/buke/quickjs-go"
)

func main() {
    // Create a new runtime
    rt := quickjs.NewRuntime()
    defer rt.Close()
    // Create a new context
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
    // Compile the script to bytecode
    buf, _ := ctx.Compile(jsStr)

    // Create a new runtime 
    rt2 := quickjs.NewRuntime()
    defer rt2.Close()

    // Create a new context
    ctx2 := rt2.NewContext()
    defer ctx2.Close()

    //Eval bytecode
    result, _ := ctx2.EvalBytecode(buf)
    fmt.Println(result.Int32())
}
```

### Runtime Options: memory, stack, GC, ...
```go
package main

import (
	"fmt"
    
	"github.com/buke/quickjs-go"
)

func main() {
    // Create a new runtime
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // set runtime options
    rt.SetExecuteTimeout(30) // Set execute timeout to 30 seconds
    rt.SetMemoryLimit(256 * 1024) // Set memory limit to 256KB
    rt.SetMaxStackSize(65534) // Set max stack size to 65534
    rt.SetGCThreshold(256 * 1024) // Set GC threshold to 256KB
    rt.SetCanBlock(true) // Set can block to true

    // Create a new context
    ctx := rt.NewContext()
    defer ctx.Close()

    result, err := ctx.Eval(`var array = []; while (true) { array.push(null) }`)
    defer result.Free()
}
```

## Documentation
Go Reference & more examples: https://pkg.go.dev/github.com/buke/quickjs-go

## License
[MIT](./LICENSE)


[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_large)

## Related Projects 
* https://github.com/buke/quickjs-go-polyfill