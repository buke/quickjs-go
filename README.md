# quickjs-go


[![codecov](https://codecov.io/gh/buke/quickjs-go/branch/main/graph/badge.svg?token=DW5RGD01AG)](https://codecov.io/gh/buke/quickjs-go)
[![Test](https://github.com/buke/quickjs-go/workflows/Test/badge.svg)](https://github.com/buke/quickjs-go/actions?query=workflow%3ATest)
[![Go Report Card](https://goreportcard.com/badge/github.com/buke/quickjs-go)](https://goreportcard.com/report/github.com/buke/quickjs-go)
[![GoDoc](https://pkg.go.dev/badge/github.com/buke/quickjs-go?status.svg)](https://pkg.go.dev/github.com/buke/quickjs-go?tab=doc)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_shield)




[![Go Report Card](https://goreportcard.com/badge/github.com/gin-gonic/gin)](https://goreportcard.com/report/github.com/gin-gonic/gin)

Go bindings to QuickJS: a fast, small, and embeddable ES2020 JavaScript interpreter.

## Features
* Evaluate script
* Compile script into bytecode and Eval from bytecode
* Operate JavaScript values and objects in Go
* Invoke Go function from JavaScript
* Simple exception throwing and catching

## Guidelines

1. Free `quickjs.Runtime` and `quickjs.Context` once you are done using them.
2. Free `quickjs.Value`'s returned by `Eval()` and `EvalFile()`. All other values do not need to be freed, as they get garbage-collected.
3. You may access the stacktrace of an error returned by `Eval()` or `EvalFile()` by casting it to a `*quickjs.Error`.
4. Make new copies of arguments should you want to return them in functions you created.
5. Make sure to call `runtime.LockOSThread()` to ensure that QuickJS always operates in the exact same thread.

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


### Bind Go Funtion to Javascript
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
    // bind go function to js object
    test.Set("hello", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
        return ctx.String("Hello " + args[0].String())
    }))

    // bind "test" object to global object
    ctx.Globals().Set("test", test)

    // call js function by js
    js_ret, _ := ctx.Eval(`test.hello("Javascript!")`)
    fmt.Println(js_ret.String())
    // Output:
    // Hello Javascript!



    // call js function by go
    go_ret := ctx.Globals().Get("test").Call("hello", ctx.String("Golang!"))
    fmt.Println(go_ret.String())

    // Output:
    // Hello Golang!
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
    rt.SetMemoryLimit(256 * 1024) //256KB
    rt.SetMaxStackSize(65534)

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
* https://github.com/lithdew/quickjs
* https://github.com/wspl/go-quickjs
* https://github.com/quickjs-go/quickjs-go
* https://github.com/ssttevee/go-quickjs
* https://github.com/rosbit/go-quickjs