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

| Platform | Arch  | Static Library                                       |
| -------- | ----- | ---------------------------------------------------- |
| Linux    | x64   | [libquickjs.a](deps/libs/linux_amd64/libquickjs.a)   |
| Linux    | arm64 | [libquickjs.a](deps/libs/linux_arm64/libquickjs.a)   |
| Windows  | x64   | [libquickjs.a](deps/libs/windows_amd64/libquickjs.a) |
| Windows  | x86   | [libquickjs.a](deps/libs/windows_386/libquickjs.a)   |
| MacOS    | x64   | [libquickjs.a](deps/libs/darwin_amd64/libquickjs.a)  |
| MacOS    | arm64 | [libquickjs.a](deps/libs/darwin_arm64/libquickjs.a)  |

\* for build on windows, ples see: https://github.com/buke/quickjs-go/issues/151#issuecomment-2134307728

## Version Notes

| quickjs-go | QuickJS     |
| ---------- | ----------- |
| v0.5.x     | v2025-04-26 |
| v0.4.x     | v2024-02-14 |
| v0.3.x     | v2024-01-13 |
| v0.2.x     | v2023-12-09 |
| v0.1.x     | v2021-03-27 |

## Breaking Changes

### v0.5.x

**Collection API Removal**: The main goal of this project is to provide bindings for the QuickJS C API, therefore collection-related APIs have been removed. The following methods are no longer available:

- `ctx.Array()` - Use `ctx.Eval("[]")` or `ctx.Object()` instead
- `ctx.Map()` - Use `ctx.Eval("new Map()")` instead  
- `ctx.Set()` - Use `ctx.Eval("new Set()")` instead
- `value.ToArray()` - Use direct `Value` operations instead
- `value.ToMap()` - Use direct `Value` operations instead
- `value.ToSet()` - Use direct `Value` operations instead
- `value.IsMap()` - Use `value.GlobalInstanceof("Map")` instead
- `value.IsSet()` - Use `value.GlobalInstanceof("Set")` instead

**Migration Guide:**

```go
// Before (v0.4.x and earlier)
arr := ctx.Array()
arr.Set("0", ctx.String("item"))

mapObj := ctx.Map()
mapObj.Set("key", ctx.String("value"))

setObj := ctx.Set()
setObj.Add(ctx.String("item"))

// After (v0.5.x)
arr, _ := ctx.Eval("[]")
arr.Set("0", ctx.String("item"))
arr.Set("length", ctx.Int32(1))

mapObj, _ := ctx.Eval("new Map()")
mapObj.Call("set", ctx.String("key"), ctx.String("value"))

setObj, _ := ctx.Eval("new Set()")
setObj.Call("add", ctx.String("item"))
```

## Features

- Evaluate script
- Compile script into bytecode and Eval from bytecode
- Operate JavaScript values and objects in Go
- Bind Go function to JavaScript async/sync function
- Simple exception throwing and catching
- **Marshal/Unmarshal Go values to/from JavaScript values**

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

### Marshal/Unmarshal Go Values

QuickJS-Go provides seamless conversion between Go and JavaScript values through the `Marshal` and `Unmarshal` methods.

#### Basic Types

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // Marshal Go values to JavaScript
    data := map[string]interface{}{
        "name":    "John Doe",
        "age":     30,
        "active":  true,
        "scores":  []int{85, 92, 78},
        "address": map[string]string{
            "city":    "New York",
            "country": "USA",
        },
    }

    jsVal, err := ctx.Marshal(data)
    if err != nil {
        panic(err)
    }
    defer jsVal.Free()

    // Use the marshaled value in JavaScript
    ctx.Globals().Set("user", jsVal)
    result, _ := ctx.Eval(`
        user.name + " is " + user.age + " years old, scores: " + user.scores.join(", ")
    `)
    defer result.Free()
    fmt.Println(result.String())

    // Unmarshal JavaScript values back to Go
    var userData map[string]interface{}
    err = ctx.Unmarshal(jsVal, &userData)
    if err != nil {
        panic(err)
    }
    fmt.Printf("%+v\n", userData)
}
```

#### Struct Marshaling with Tags

```go
package main

import (
    "fmt"
    "time"
    "github.com/buke/quickjs-go"
)

type User struct {
    ID        int64     `js:"id"`
    Name      string    `js:"name"`
    Email     string    `json:"email_address"`
    CreatedAt time.Time `js:"created_at"`
    IsActive  bool      `js:"is_active"`
    Tags      []string  `js:"tags"`
    // unexported fields are ignored
    password  string
    // Fields with "-" tag are skipped
    Secret    string `js:"-"`
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    user := User{
        ID:        123,
        Name:      "Alice",
        Email:     "alice@example.com",
        CreatedAt: time.Now(),
        IsActive:  true,
        Tags:      []string{"admin", "user"},
        password:  "secret123",
        Secret:    "top-secret",
    }

    // Marshal struct to JavaScript
    jsVal, err := ctx.Marshal(user)
    if err != nil {
        panic(err)
    }
    defer jsVal.Free()

    // Modify in JavaScript
    ctx.Globals().Set("user", jsVal)
    result, _ := ctx.Eval(`
        user.name = "Alice Smith";
        user.tags.push("moderator");
        user;
    `)
    defer result.Free()

    // Unmarshal back to Go struct
    var updatedUser User
    err = ctx.Unmarshal(result, &updatedUser)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Updated user: %+v\n", updatedUser)
    // Note: password and Secret fields remain unchanged (not serialized)
}
```

#### Custom Marshal/Unmarshal

```go
package main

import (
    "fmt"
    "strings"
    "github.com/buke/quickjs-go"
)

type CustomType struct {
    Value string
}

// Implement Marshaler interface
func (c CustomType) MarshalJS(ctx *quickjs.Context) (quickjs.Value, error) {
    return ctx.String("custom:" + c.Value), nil
}

// Implement Unmarshaler interface
func (c *CustomType) UnmarshalJS(ctx *quickjs.Context, val quickjs.Value) error {
    if val.IsString() {
        str := val.ToString()
        if strings.HasPrefix(str, "custom:") {
            c.Value = str[7:] // Remove "custom:" prefix
        } else {
            c.Value = str
        }
    }
    return nil
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // Marshal custom type
    custom := CustomType{Value: "hello"}
    jsVal, err := ctx.Marshal(custom)
    if err != nil {
        panic(err)
    }
    defer jsVal.Free()

    fmt.Println("Marshaled:", jsVal.String()) // Output: custom:hello

    // Unmarshal back
    var result CustomType
    err = ctx.Unmarshal(jsVal, &result)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Unmarshaled: %+v\n", result) // Output: {Value:hello}
}
```

#### Type Mappings

**Go to JavaScript:**
- `bool` → JavaScript boolean
- `int`, `int8`, `int16`, `int32` → JavaScript number (32-bit)
- `int64` → JavaScript number (64-bit)
- `uint`, `uint8`, `uint16`, `uint32` → JavaScript number (32-bit unsigned)
- `uint64` → JavaScript BigInt
- `float32`, `float64` → JavaScript number
- `string` → JavaScript string
- `[]byte` → JavaScript ArrayBuffer
- `slice/array` → JavaScript Array
- `map` → JavaScript Object
- `struct` → JavaScript Object
- `pointer` → recursively marshal pointed value (nil becomes null)

**JavaScript to Go:**
- JavaScript null/undefined → Go nil pointer or zero value
- JavaScript boolean → Go bool
- JavaScript number → Go numeric types (with appropriate conversion)
- JavaScript BigInt → Go `uint64`/`int64`/`*big.Int`
- JavaScript string → Go string
- JavaScript Array → Go slice/array
- JavaScript Object → Go map/struct
- JavaScript ArrayBuffer → Go `[]byte`

When unmarshaling into `interface{}`, the following types are used:
- `nil` for null/undefined
- `bool` for boolean
- `int64` for integer numbers
- `float64` for floating-point numbers
- `string` for string
- `[]interface{}` for Array
- `map[string]interface{}` for Object
- `*big.Int` for BigInt
- `[]byte` for ArrayBuffer

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

### ES6 Module Support

```go

package main

import (
    "fmt"

    "github.com/buke/quickjs-go"
)

func main() {
// enable module import
    rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
    defer rt.Close()

    ctx := rt.NewContext()
    defer ctx.Close()

    // eval module
    r1, err := ctx.EvalFile("./test/hello_module.js")
    defer r1.Free()
    require.NoError(t, err)
    require.EqualValues(t, 55, ctx.Globals().Get("result").Int32())

    // load module
    r2, err := ctx.LoadModuleFile("./test/fib_module.js", "fib_foo")
    defer r2.Free()
    require.NoError(t, err)

    // call module
    r3, err := ctx.Eval(`
    import {fib} from 'fib_foo';
    globalThis.result = fib(9);
    `)
    defer r3.Free()
    require.NoError(t, err)

    require.EqualValues(t, 34, ctx.Globals().Get("result").Int32())
}

```

## Documentation

Go Reference & more examples: https://pkg.go.dev/github.com/buke/quickjs-go

## License

[MIT](./LICENSE)

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_large)

## Related Projects

- https://github.com/buke/quickjs-go-polyfill
