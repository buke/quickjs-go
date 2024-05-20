# quickjs-go
[English](README.md) | 简体中文

[![Test](https://github.com/buke/quickjs-go/workflows/Test/badge.svg)](https://github.com/buke/quickjs-go/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/quickjs-go/branch/main/graph/badge.svg?token=DW5RGD01AG)](https://codecov.io/gh/buke/quickjs-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/buke/quickjs-go)](https://goreportcard.com/report/github.com/buke/quickjs-go)
[![GoDoc](https://pkg.go.dev/badge/github.com/buke/quickjs-go?status.svg)](https://pkg.go.dev/github.com/buke/quickjs-go?tab=doc)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_shield)

Go 语言的QuickJS绑定库：快速、小型、可嵌入的ES2020 JavaScript解释器。

## 平台支持
使用预编译的quickjs静态库，支持以下平台：

| 平台 | 架构 | 静态库 |
| -------- | ------------ | ------- |
| Linux    | x64          | [libquickjs.a](deps/libs/linux_amd64/libquickjs.a) |
| Linux    | arm64        | [libquickjs.a](deps/libs/linux_arm64/libquickjs.a) |
| Windows  | x64          | [quickjs.lib](deps/libs/windows_amd64/quickjs.lib) |
| Windows  | x86          | [quickjs.lib](deps/libs/windows_386/quickjs.lib) |
| MacOS    | x64          | [libquickjs.a](deps/libs/darwin_amd64/libquickjs.a) |
| MacOS    | arm64        | [libquickjs.a](deps/libs/darwin_arm64/libquickjs.a) |

## 版本说明

| quickjs-go | QuickJS |
| ---------- | ------- |
| v0.1.x     | v2021-03-27 |
| v0.2.x     | v2023-12-09 |
| v0.3.x     | v2024-01-13 |
| v0.4.x     | v2024-02-14 |

## 功能
* 执行javascript脚本
* 编译javascript脚本到字节码并执行字节码
* 在 Go 中操作 JavaScript 值和对象
* 绑定 Go 函数到 JavaScript 同步函数和异步函数
* 简单的异常抛出和捕获

## 指南

1. 在使用完毕后，请记得关闭 `quickjs.Runtime` 和 `quickjs.Context`。
2. 请记得关闭由 `Eval()` 和 `EvalFile()` 返回的 `quickjs.Value`。其他值不需要关闭，因为它们会被垃圾回收。
3. 如果你使用了promise 或 async function，请使用 `ctx.Loop()` 等待所有的promise/job结果。
4. You may access the stacktrace of an error returned by `Eval()` or `EvalFile()` by casting it to a `*quickjs.Error`.
4.  如果`Eval()` 或 `EvalFile()`返回了错误，可强制转换为`*quickjs.Error`以读取错误的堆栈信息。
5. 如果你想在函数中返回参数，请在函数中复制参数。

## 用法 

```go
import "github.com/buke/quickjs-go"
```

### 执行javascript脚本
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

    ret, err := ctx.Eval("'Hello ' + 'QuickJS!'")
    if err != nil {
        println(err.Error())
    }
    fmt.Println(ret.String())
}
```


### 读取/设置 JavaScript 对象
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


### 函数绑定
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

### 异常抛出和捕获
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

### Bytecode编译和执行
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

### 设置内存、栈、GC等等
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
    rt.SetMemoryLimit(256 * 1024) //256KB
    rt.SetMaxStackSize(65534)

    // Create a new context
    ctx := rt.NewContext()
    defer ctx.Close()

    result, err := ctx.Eval(`var array = []; while (true) { array.push(null) }`)
    defer result.Free()
}
```

## 文档
Go 语言文档和示例: https://pkg.go.dev/github.com/buke/quickjs-go

## 协议
[MIT](./LICENSE)


[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_large)

## 相关项目
* https://github.com/buke/quickjs-go-polyfill