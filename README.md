# quickjs-go

English | [简体中文](README_zh-cn.md)
### Async Promise with Context Scheduler

The new context-level scheduler helps you resolve or reject JavaScript Promises from other goroutines while ensuring the actual QuickJS calls still happen on the context thread.

```go
package main

import (
    "fmt"
    "time"

    "github.com/buke/quickjs-go"
)

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()

    ctx := rt.NewContext()
    defer ctx.Close()

    ctx.Globals().Set("asyncJob", ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        return ctx.NewPromise(func(resolve, reject func(*quickjs.Value)) {
            go func() {
                time.Sleep(10 * time.Millisecond)

                ctx.Schedule(func(inner *quickjs.Context) {
                    result := inner.NewString("async result")
                    defer result.Free()
                    resolve(result)
                })
            }()
        })
    }))

    promise := ctx.Eval(`asyncJob()`)
    defer promise.Free()

    result := ctx.Await(promise)
    defer result.Free()

    if result.IsException() {
        fmt.Println("error:", ctx.Exception())
        return
    }

    fmt.Println(result.ToString())
}
```


[![Test](https://github.com/buke/quickjs-go/workflows/Test/badge.svg)](https://github.com/buke/quickjs-go/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/quickjs-go/graph/badge.svg?token=8z6vgOaIIS)](https://codecov.io/gh/buke/quickjs-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/buke/quickjs-go)](https://goreportcard.com/report/github.com/buke/quickjs-go)
[![GoDoc](https://pkg.go.dev/badge/github.com/buke/quickjs-go?status.svg)](https://pkg.go.dev/github.com/buke/quickjs-go?tab=doc)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_shield)

Go bindings to QuickJS: a fast, small, and embeddable ES2020 JavaScript interpreter.

**⚠️ This project is not ready for production use yet. Use at your own risk. APIs may change without notice.**

## Features

- Evaluate script
- Compile script into bytecode and Eval from bytecode
- Operate JavaScript values and objects in Go
- Bind Go function to JavaScript async/sync function
- Simple exception throwing and catching
- **Marshal/Unmarshal Go values to/from JavaScript values**
- **Full TypedArray support (Int8Array, Uint8Array, Float32Array, etc.)**
- **Create JavaScript Classes from Go with ClassBuilder**
- **Create JavaScript Modules from Go with ModuleBuilder*o
- **Cross-platform:** Prebuilt QuickJS static libraries for Linux (x64/arm64), Windows (x64/x86), MacOS (x64/arm64).  
  *(See [deps/libs](deps/libs) for details. For Windows build tips, see: https://github.com/buke/quickjs-go/issues/151#issuecomment-2134307728)*


## Guidelines

### Error Handling
- Use `Value.IsException()` or `Context.HasException()` to check for exceptions
- Use `Context.Exception()` to get the exception as a Go error
- Always call `defer value.Free()` for returned values to prevent memory leaks
- Check `Context.HasException()` after operations that might throw

### Memory Management
- Call  `value.Free()` for `*Value` objects you create or receive. QuickJS uses reference counting for memory management, so if a value is referenced by other objects, you only need to ensure the referencing objects are properly freed.
- Runtime and Context objects have their own cleanup methods (`Close()`). Close them once you are done using them.
- Use `runtime.SetFinalizer()` cautiously as it may interfere with QuickJS's GC.

### Performance Tips
- QuickJS is not thread-safe. For concurrency or isolation, use a thread pool pattern with pre-initialized runtimes, or manage separate Runtime/Context instances for different tasks or users (such as : [https://github.com/buke/js-executor](https://github.com/buke/js-executor)).
- Reuse Runtime and Context objects when possible.
- Avoid frequent conversion between Go and JS values.
- Consider using bytecode compilation for frequently executed scripts.

### Best Practices
- Use appropriate `EvalOptions` for different script types.
- Handle both JavaScript exceptions and Go errors appropriately.
- Test memory usage under load to prevent leaks.


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

    ret := ctx.Eval("'Hello ' + 'QuickJS!'")
    defer ret.Free()
    
    if ret.IsException() {
        err := ctx.Exception()
        println(err.Error())
        return
    }
    
    fmt.Println(ret.ToString())
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

    test := ctx.NewObject()
    test.Set("A", ctx.NewString("String A"))
    test.Set("B", ctx.NewString("String B"))
    test.Set("C", ctx.NewString("String C"))
    ctx.Globals().Set("test", test)

    ret := ctx.Eval(`Object.keys(test).map(key => test[key]).join(" ")`)
    defer ret.Free()
    
    if ret.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println(ret.ToString())
}

```

### Bind Go Funtion to Javascript async/sync function

```go
package main

import (
    "fmt"
    "time"

    "github.com/buke/quickjs-go"
)

func main() {
    // Create a new runtime
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // Create a new context
    ctx := rt.NewContext()
    defer ctx.Close()

    // Create a new object
    test := ctx.NewObject()
    // bind properties to the object
    test.Set("A", ctx.NewString("String A"))
    test.Set("B", ctx.NewInt32(0))
    test.Set("C", ctx.NewBool(false))
    // bind go function to js object
    test.Set("hello", ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        return ctx.NewString("Hello " + args[0].ToString())
    }))

    // bind "test" object to global object
    ctx.Globals().Set("test", test)

    // call js function by js
    js_ret := ctx.Eval(`test.hello("Javascript!")`)
    defer js_ret.Free()
    
    if js_ret.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println(js_ret.ToString())

    // call js function by go
    go_ret := test.Call("hello", ctx.NewString("Golang!"))
    defer go_ret.Free()
    
    if go_ret.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println(go_ret.ToString())

    // bind go function to Javascript async function using Function + Promise
    ctx.Globals().Set("testAsync", ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        return ctx.NewPromise(func(resolve, reject func(*quickjs.Value)) {
            go func() {
                time.Sleep(10 * time.Millisecond)

                ctx.Schedule(func(inner *quickjs.Context) {
                    value := inner.NewString("Hello Async Function!")
                    defer value.Free()
                    resolve(value)
                })
            }()
        })
    }))

    promiseResult := ctx.Eval(`testAsync()`)
    defer promiseResult.Free()

    asyncRet := ctx.Await(promiseResult)
    defer asyncRet.Free()

    if asyncRet.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }

    fmt.Println(asyncRet.ToString())

    // Output:
    // Hello Javascript!
    // Hello Golang!
    // Hello Async Function!
}

// NOTE: Always defer interacting with the Context from goroutines until you are
// inside ctx.Schedule. The function passed to ctx.Schedule runs back on the
// Context thread, so QuickJS APIs remain safe to call there.
```

`ctx.Await` drives the pending-job loop internally, so you do not need to call `ctx.Loop()` when you only need the promise result.

### Error Handling

```go
package main

import (
    "fmt"
    "errors"

    "github.com/buke/quickjs-go"
)

func main() {
    // Create a new runtime
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // Create a new context
    ctx := rt.NewContext()
    defer ctx.Close()

    ctx.Globals().SetFunction("A", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        // raise error
        return ctx.ThrowError(errors.New("expected error"))
    })

    result := ctx.Eval("A()")
    defer result.Free()
    
    if result.IsException() {
        actual := ctx.Exception()
        fmt.Println(actual.Error())
    }
}
```

### TypedArray Support

QuickJS-Go provides support for JavaScript TypedArrays, enabling binary data processing between Go and JavaScript.

#### Creating TypedArrays from Go

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

    // Create various TypedArrays from Go slices
    int8Data := []int8{-128, -1, 0, 1, 127}
    int8Array := ctx.NewInt8Array(int8Data)

    uint8Data := []uint8{0, 128, 255}
    uint8Array := ctx.NewUint8Array(uint8Data)

    float32Data := []float32{-3.14, 0.0, 2.718, 100.5}
    float32Array := ctx.NewFloat32Array(float32Data)

    int64Data := []int64{-9223372036854775808, 0, 9223372036854775807}
    bigInt64Array := ctx.NewBigInt64Array(int64Data)

    // Set TypedArrays as global variables
    ctx.Globals().Set("int8Array", int8Array)
    ctx.Globals().Set("uint8Array", uint8Array)
    ctx.Globals().Set("float32Array", float32Array)
    ctx.Globals().Set("bigInt64Array", bigInt64Array)

    // Use in JavaScript
    result := ctx.Eval(`
        // Check types
        const results = {
            int8Type: int8Array instanceof Int8Array,
            uint8Type: uint8Array instanceof Uint8Array,
            float32Type: float32Array instanceof Float32Array,
            bigInt64Type: bigInt64Array instanceof BigInt64Array,
            // Calculate sum of float32 array
            float32Sum: float32Array.reduce((sum, val) => sum + val, 0)
        };
        results;
    `)
    defer result.Free()

    if result.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }

    fmt.Println("Results:", result.JSONStringify())
}
```

#### Converting JavaScript TypedArrays to Go

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

    // Create TypedArrays in JavaScript
    jsTypedArrays := ctx.Eval(`
        ({
            int8: new Int8Array([-128, -1, 0, 1, 127]),
            uint16: new Uint16Array([0, 32768, 65535]),
            float64: new Float64Array([Math.PI, Math.E, 42.5]),
            bigUint64: new BigUint64Array([0n, 18446744073709551615n])
        })
    `)
    defer jsTypedArrays.Free()

    if jsTypedArrays.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }

    // Convert to Go slices
    int8Array := jsTypedArrays.Get("int8")
    defer int8Array.Free()
    if int8Array.IsInt8Array() {
        goInt8Slice, err := int8Array.ToInt8Array()
        if err == nil {
            fmt.Printf("Int8Array: %v\n", goInt8Slice)
        }
    }

    uint16Array := jsTypedArrays.Get("uint16")
    defer uint16Array.Free()
    if uint16Array.IsUint16Array() {
        goUint16Slice, err := uint16Array.ToUint16Array()
        if err == nil {
            fmt.Printf("Uint16Array: %v\n", goUint16Slice)
        }
    }

    float64Array := jsTypedArrays.Get("float64")
    defer float64Array.Free()
    if float64Array.IsFloat64Array() {
        goFloat64Slice, err := float64Array.ToFloat64Array()
        if err == nil {
            fmt.Printf("Float64Array: %v\n", goFloat64Slice)
        }
    }

    bigUint64Array := jsTypedArrays.Get("bigUint64")
    defer bigUint64Array.Free()
    if bigUint64Array.IsBigUint64Array() {
        goBigUint64Slice, err := bigUint64Array.ToBigUint64Array()
        if err == nil {
            fmt.Printf("BigUint64Array: %v\n", goBigUint64Slice)
        }
    }
}
```

#### TypedArray Types Support

| Go Type    | JavaScript TypedArray | Context Method          | Value Method        |
|------------|----------------------|-------------------------|---------------------|
| `[]int8`   | `Int8Array`          | `ctx.NewInt8Array()`       | `val.ToInt8Array()` |
| `[]uint8`  | `Uint8Array`         | `ctx.NewUint8Array()`      | `val.ToUint8Array()` |
| `[]uint8`  | `Uint8ClampedArray`  | `ctx.NewUint8ClampedArray()` | `val.ToUint8Array()` |
| `[]int16`  | `Int16Array`         | `ctx.NewInt16Array()`      | `val.ToInt16Array()` |
| `[]uint16` | `Uint16Array`        | `ctx.NewUint16Array()`     | `val.ToUint16Array()` |
| `[]int32`  | `Int32Array`         | `ctx.NewInt32Array()`      | `val.ToInt32Array()` |
| `[]uint32` | `Uint32Array`        | `ctx.NewUint32Array()`     | `val.ToUint32Array()` |
| `[]float32` | `Float32Array`      | `ctx.NewFloat32Array()`    | `val.ToFloat32Array()` |
| `[]float64` | `Float64Array`      | `ctx.NewFloat64Array()`    | `val.ToFloat64Array()` |
| `[]int64`  | `BigInt64Array`      | `ctx.NewBigInt64Array()`   | `val.ToBigInt64Array()` |
| `[]uint64` | `BigUint64Array`     | `ctx.NewBigUint64Array()`  | `val.ToBigUint64Array()` |
| `[]byte`   | `ArrayBuffer`        | `ctx.NewArrayBuffer()`     | `val.ToByteArray()` |

#### TypedArray Detection

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

    // Create various arrays
    regularArray := ctx.Eval(`[1, 2, 3]`)
    defer regularArray.Free()

    int32Array := ctx.NewInt32Array([]int32{1, 2, 3})
    float64Array := ctx.NewFloat64Array([]float64{1.1, 2.2, 3.3})

    // Set arrays as global variables to be referenced by globals
    ctx.Globals().Set("int32Array", int32Array)
    ctx.Globals().Set("float64Array", float64Array)

    // Detect array types
    fmt.Printf("Regular array IsArray: %v\n", regularArray.IsArray())
    fmt.Printf("Regular array IsTypedArray: %v\n", regularArray.IsTypedArray())

    fmt.Printf("Int32Array IsTypedArray: %v\n", int32Array.IsTypedArray())
    fmt.Printf("Int32Array IsInt32Array: %v\n", int32Array.IsInt32Array())
    fmt.Printf("Int32Array IsFloat64Array: %v\n", int32Array.IsFloat64Array())

    fmt.Printf("Float64Array IsTypedArray: %v\n", float64Array.IsTypedArray())
    fmt.Printf("Float64Array IsFloat64Array: %v\n", float64Array.IsFloat64Array())
    fmt.Printf("Float64Array IsInt32Array: %v\n", float64Array.IsInt32Array())
}
```

#### Binary Data Processing Example

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

    // Process image-like data (simulate RGB pixels)
    imageData := []uint8{
        255, 0, 0,    // Red pixel
        0, 255, 0,    // Green pixel  
        0, 0, 255,    // Blue pixel
        255, 255, 0,  // Yellow pixel
    }

    // Send to JavaScript as Uint8Array
    imageArray := ctx.NewUint8Array(imageData)
    ctx.Globals().Set("imageData", imageArray)

    // Process in JavaScript
    result := ctx.Eval(`
        // Convert RGB to grayscale
        const grayscale = new Uint8Array(imageData.length / 3);
        for (let i = 0; i < imageData.length; i += 3) {
            const r = imageData[i];
            const g = imageData[i + 1];
            const b = imageData[i + 2];
            grayscale[i / 3] = Math.round(0.299 * r + 0.587 * g + 0.114 * b);
        }
        grayscale;
    `)
    defer result.Free()

    if result.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }

    // Convert back to Go
    if result.IsUint8Array() {
        grayscaleData, err := result.ToUint8Array()
        if err == nil {
            fmt.Printf("Original RGB: %v\n", imageData)
            fmt.Printf("Grayscale: %v\n", grayscaleData)
        }
    }
}
```

### Marshal/Unmarshal Go Values

QuickJS-Go provides conversion between Go and JavaScript values through the `Marshal` and `Unmarshal` methods.

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
        // TypedArray will be automatically created for typed slices
        "floatData": []float32{1.1, 2.2, 3.3},
        "intData":   []int32{100, 200, 300},
        "byteData":  []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F}, // "Hello" in bytes
    }

    jsVal, err := ctx.Marshal(data)
    if err != nil {
        panic(err)
    }
    defer jsVal.Free()

    // Use the marshaled value in JavaScript
    ctx.Globals().Set("user", jsVal)
    result := ctx.Eval(`
        const info = user.name + " is " + user.age + " years old";
        const floatArrayType = user.floatData instanceof Float32Array;
        const intArrayType = user.intData instanceof Int32Array;
        const byteArrayType = user.byteData instanceof ArrayBuffer;
        
        ({
            info: info,
            floatArrayType: floatArrayType,
            intArrayType: intArrayType,
            byteArrayType: byteArrayType,
            byteString: new TextDecoder().decode(user.byteData)
        });
    `)
    defer result.Free()
    
    if result.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println("Result:", result.JSONStringify())

    // Unmarshal JavaScript values back to Go
    var userData map[string]interface{}
    err = ctx.Unmarshal(jsVal, &userData)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Unmarshaled: %+v\n", userData)
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
    // TypedArray fields
    Scores    []float32 `js:"scores"`    // Will become Float32Array
    Data      []int32   `js:"data"`      // Will become Int32Array
    Binary    []byte    `js:"binary"`    // Will become ArrayBuffer
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
        Scores:    []float32{95.5, 87.2, 92.0},
        Data:      []int32{1000, 2000, 3000},
        Binary:    []byte{0x41, 0x42, 0x43}, // "ABC"
        password:  "secret123",
        Secret:    "top-secret",
    }

    // Marshal struct to JavaScript
    jsVal, err := ctx.Marshal(user)
    if err != nil {
        panic(err)
    }
    defer jsVal.Free()

    // Check TypedArray types in JavaScript
    ctx.Globals().Set("user", jsVal)
    result := ctx.Eval(`
        ({
            scoresType: user.scores instanceof Float32Array,
            dataType: user.data instanceof Int32Array,
            binaryType: user.binary instanceof ArrayBuffer,
            binaryString: new TextDecoder().decode(user.binary),
            avgScore: user.scores.reduce((sum, score) => sum + score) / user.scores.length
        });
    `)
    defer result.Free()

    if result.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }

    // Modify in JavaScript
    modifyResult := ctx.Eval(`
        user.name = "Alice Smith";
        user.tags.push("moderator");
        // Modify TypedArray data
        user.scores[0] = 98.5;
        user;
    `)
    defer modifyResult.Free()

    if modifyResult.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }

    // Unmarshal back to Go struct
    var updatedUser User
    err = ctx.Unmarshal(modifyResult, &updatedUser)
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
func (c CustomType) MarshalJS(ctx *quickjs.Context) (*quickjs.Value, error) {
    return ctx.NewString("custom:" + c.Value), nil
}

// Implement Unmarshaler interface
func (c *CustomType) UnmarshalJS(ctx *quickjs.Context, val *quickjs.Value) error {
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

    fmt.Println("Marshaled:", jsVal.ToString()) // Output: custom:hello

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
- `[]int8` → JavaScript Int8Array
- `[]uint8` → JavaScript Uint8Array
- `[]int16` → JavaScript Int16Array
- `[]uint16` → JavaScript Uint16Array
- `[]int32` → JavaScript Int32Array
- `[]uint32` → JavaScript Uint32Array
- `[]float32` → JavaScript Float32Array
- `[]float64` → JavaScript Float64Array
- `[]int64` → JavaScript BigInt64Array
- `[]uint64` → JavaScript BigUint64Array
- `slice/array` → JavaScript Array (for non-typed arrays)
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
- JavaScript Int8Array → Go `[]int8`
- JavaScript Uint8Array/Uint8ClampedArray → Go `[]uint8`
- JavaScript Int16Array → Go `[]int16`
- JavaScript Uint16Array → Go `[]uint16`
- JavaScript Int32Array → Go `[]int32`
- JavaScript Uint32Array → Go `[]uint32`
- JavaScript Float32Array → Go `[]float32`
- JavaScript Float64Array → Go `[]float64`
- JavaScript BigInt64Array → Go `[]int64`
- JavaScript BigUint64Array → Go `[]uint64`

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

### Create JavaScript Modules from Go with ModuleBuilder

The ModuleBuilder API allows you to create JavaScript modules from Go code, making Go functions, values, and objects available for standard ES6 import syntax in JavaScript applications.

#### Basic Module Creation

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

    // Create a math module with Go functions and values
    addFunc := ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        if len(args) >= 2 {
            return ctx.NewFloat64(args[0].ToFloat64() + args[1].ToFloat64())
        }
        return ctx.NewFloat64(0)
    })
    defer addFunc.Free()

    // Build the module using fluent API
    module := quickjs.NewModuleBuilder("math").
        Export("PI", ctx.NewFloat64(3.14159)).
        Export("add", addFunc).
        Export("version", ctx.NewString("1.0.0")).
        Export("default", ctx.NewString("Math Module"))

    err := module.Build(ctx)
    if err != nil {
        panic(err)
    }

    // Use the module in JavaScript with standard ES6 import
    result := ctx.Eval(`
        (async function() {
            // Named imports
            const { PI, add, version } = await import('math');
            
            // Use imported functions and values
            const sum = add(PI, 1.0);
            return { sum, version };
        })()
    `, quickjs.EvalAwait(true))
    defer result.Free()

    if result.IsException() {
        err := ctx.Exception()
        panic(err)
    }

    fmt.Println("Module result:", result.JSONStringify())
    // Output: Module result: {"sum":4.14159,"version":"1.0.0"}
}
```

#### Advanced Module Features

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

    // Create a utilities module with complex objects
    config := ctx.NewObject()
    config.Set("appName", ctx.NewString("MyApp"))
    config.Set("version", ctx.NewString("2.0.0"))
    config.Set("debug", ctx.NewBool(true))

    greetFunc := ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        name := "World"
        if len(args) > 0 {
            name = args[0].ToString()
        }
        return ctx.NewString(fmt.Sprintf("Hello, %s!", name))
    })
    defer greetFunc.Free()

    jsonVal := ctx.ParseJSON(`{"MAX": 100, "MIN": 1}`)
    defer jsonVal.Free()

    // Create module with various export types
    module := quickjs.NewModuleBuilder("utils").
        Export("config", config).                    // Object export
        Export("greet", greetFunc).                  // Function export
        Export("constants", jsonVal).                // JSON export
        Export("default", ctx.NewString("Utils Library"))  // Default export

    err := module.Build(ctx)
    if err != nil {
        panic(err)
    }

    // Use mixed imports in JavaScript
    result := ctx.Eval(`
        (async function() {
            // Import from utils module
            const { greet, config, constants } = await import('utils');
            
            // Combine functionality
            const message = greet("JavaScript");
            const info = config.appName + " v" + config.version;
            const limits = "Max: " + constants.MAX + ", Min: " + constants.MIN;
            
            return { message, info, limits };
        })()
    `, quickjs.EvalAwait(true))
    defer result.Free()

    if result.IsException() {
        err := ctx.Exception()
        panic(err)
    }

    fmt.Println("Advanced module result:", result.JSONStringify())
}
```

#### Multiple Module Integration

```go
package main

import (
    "fmt"
    "strings"
    "github.com/buke/quickjs-go"
)

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // Create math module
    addFunc := ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        if len(args) >= 2 {
            return ctx.NewFloat64(args[0].ToFloat64() + args[1].ToFloat64())
        }
        return ctx.NewFloat64(0)
    })
    defer addFunc.Free()

    mathModule := quickjs.NewModuleBuilder("math").
        Export("add", addFunc).
        Export("PI", ctx.NewFloat64(3.14159))

    // Create string utilities module
    upperFunc := ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        if len(args) > 0 {
            return ctx.NewString(strings.ToUpper(args[0].ToString()))
        }
        return ctx.NewString("")
    })
    defer upperFunc.Free()

    stringModule := quickjs.NewModuleBuilder("strings").
        Export("upper", upperFunc)

    // Build both modules
    err := mathModule.Build(ctx)
    if err != nil {
        panic(err)
    }

    err = stringModule.Build(ctx)
    if err != nil {
        panic(err)
    }

    // Use multiple modules together
    result := ctx.Eval(`
        (async function() {
            // Import from multiple modules
            const { add, PI } = await import('math');
            const { upper } = await import('strings');
            
            // Combine functionality
            const sum = add(PI, 1);
            const message = "Result: " + sum.toFixed(2);
            const finalMessage = upper(message);
            
            return finalMessage;
        })()
    `, quickjs.EvalAwait(true))
    defer result.Free()

    if result.IsException() {
        err := ctx.Exception()
        panic(err)
    }

    fmt.Println("Multiple modules result:", result.ToString())
    // Output: Multiple modules result: RESULT: 4.14
}
```

#### ModuleBuilder API Reference

**Core Methods:**
- `NewModuleBuilder(name)` - Create a new module builder with the specified name
- `Export(name, value)` - Add a named export to the module (chainable method)
- `Build(ctx)` - Register the module in the JavaScript context

### Create JavaScript Classes from Go with ClassBuilder

The ClassBuilder API allows you to create JavaScript classes from Go code.

#### Manual Class Creation

Create JavaScript classes manually with control over methods, properties, and accessors:

```go
package main

import (
    "fmt"
    "math"
    "github.com/buke/quickjs-go"
)

type Point struct {
    X, Y float64
    Name string
}

func (p *Point) Distance() float64 {
    return math.Sqrt(p.X*p.X + p.Y*p.Y)
}

func (p *Point) Move(dx, dy float64) {
    p.X += dx
    p.Y += dy
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // Create Point class using ClassBuilder
    pointConstructor, _ := quickjs.NewClassBuilder("Point").
        Constructor(func(ctx *quickjs.Context, instance *quickjs.Value, args []*quickjs.Value) (interface{}, error) {
            x, y := 0.0, 0.0
            name := "Unnamed Point"
            
            if len(args) > 0 { x = args[0].ToFloat64() }
            if len(args) > 1 { y = args[1].ToFloat64() }
            if len(args) > 2 { name = args[2].ToString() }
            
            // Return Go object for automatic association
            return &Point{X: x, Y: y, Name: name}, nil
        }).
        // Accessors provide getter/setter functionality with custom logic
        Accessor("x", 
            func(ctx *quickjs.Context, this *quickjs.Value) *quickjs.Value {
                point, _ := this.GetGoObject()
                return ctx.NewFloat64(point.(*Point).X)
            },
            func(ctx *quickjs.Context, this *quickjs.Value, value *quickjs.Value) *quickjs.Value {
                point, _ := this.GetGoObject()
                point.(*Point).X = value.ToFloat64()
                return ctx.NewUndefined()
            }).
        Accessor("y",
            func(ctx *quickjs.Context, this *quickjs.Value) *quickjs.Value {
                point, _ := this.GetGoObject()
                return ctx.NewFloat64(point.(*Point).Y)
            },
            func(ctx *quickjs.Context, this *quickjs.Value, value *quickjs.Value) *quickjs.Value {
                point, _ := this.GetGoObject()
                point.(*Point).Y = value.ToFloat64()
                return ctx.NewUndefined()
            }).
        // Properties are bound directly to each instance
        Property("version", ctx.NewString("1.0.0")).
        Property("type", ctx.NewString("Point")).
        // Read-only property
        Property("readOnly", ctx.NewBool(true), quickjs.PropertyConfigurable).
        // Instance methods
        Method("distance", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
            point, _ := this.GetGoObject()
            return ctx.NewFloat64(point.(*Point).Distance())
        }).
        Method("move", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
            point, _ := this.GetGoObject()
            dx, dy := 0.0, 0.0
            if len(args) > 0 { dx = args[0].ToFloat64() }
            if len(args) > 1 { dy = args[1].ToFloat64() }
            point.(*Point).Move(dx, dy)
            return ctx.NewUndefined()
        }).
        Method("getName", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
            point, _ := this.GetGoObject()
            return ctx.NewString(point.(*Point).Name)
        }).
        // Static method
        StaticMethod("origin", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
            // Create a new Point at origin
            origin := &Point{X: 0, Y: 0, Name: "Origin"}
            jsVal, _ := ctx.Marshal(origin)
            return jsVal
        }).
        Build(ctx)

    // Register the class
    ctx.Globals().Set("Point", pointConstructor)

    // Use in JavaScript
    result := ctx.Eval(`
        const p = new Point(3, 4, "My Point");
        const dist1 = p.distance();
        p.move(1, 1);
        const dist2 = p.distance();
        
        // Static method usage
        const origin = Point.origin();
        
        ({ 
            // Accessor usage
            x: p.x,
            y: p.y,
            // Property usage
            version: p.version,
            type: p.type,
            readOnly: p.readOnly,
            hasOwnProperty: p.hasOwnProperty('version'), // true for properties
            // Method results
            name: p.getName(),
            initialDistance: dist1,
            finalDistance: dist2,
            // Static method result
            originDistance: Math.sqrt(origin.x * origin.x + origin.y * origin.y)
        });
    `)
    defer result.Free()
    
    if result.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println("Result:", result.JSONStringify())
    
    // Demonstrate the difference between accessors and properties
    propertyTest := ctx.Eval(`
        const p1 = new Point(1, 1);
        const p2 = new Point(2, 2);
        
        // Properties are instance-specific values
        const sameVersion = p1.version === p2.version; // true, same static value
        
        // Accessors provide dynamic values from Go object
        const differentX = p1.x !== p2.x; // true, different values from Go objects
        
        ({ sameVersion, differentX });
    `)
    defer propertyTest.Free()
    
    if propertyTest.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println("Property vs Accessor:", propertyTest.JSONStringify())
}
```

#### Automatic Class Creation with Reflection

Automatically generate JavaScript classes from Go structs using reflection. Go struct fields are automatically converted to JavaScript class accessors, providing getter/setter functionality that directly maps to the underlying Go object fields.

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

type User struct {
    ID        int64     `js:"id"`           // Becomes accessor: user.id
    Name      string    `js:"name"`         // Becomes accessor: user.name
    Email     string    `json:"email_address"` // Becomes accessor: user.email_address
    Age       int       `js:"age"`          // Becomes accessor: user.age
    IsActive  bool      `js:"is_active"`    // Becomes accessor: user.is_active
    Scores    []float32 `js:"scores"`       // Becomes accessor: user.scores (Float32Array)
    private   string    // Not accessible (unexported)
    Secret    string    `js:"-"`            // Explicitly ignored
}

func (u *User) GetFullInfo() string {
    return fmt.Sprintf("%s (%s) - Age: %d", u.Name, u.Email, u.Age)
}

func (u *User) UpdateEmail(newEmail string) {
    u.Email = newEmail
}

func (u *User) AddScore(score float32) {
    u.Scores = append(u.Scores, score)
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // Automatically create User class from struct
    userConstructor, _ := ctx.BindClass(&User{})

    ctx.Globals().Set("User", userConstructor)

    // Use with positional arguments
    result1 := ctx.Eval(`
        const user1 = new User(1, "Alice", "alice@example.com", 25, true, [95.5, 87.2]);
        user1.GetFullInfo();
    `)
    defer result1.Free()
    
    if result1.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println("Positional:", result1.ToString())

    // Use with named arguments (object parameter)
    result2 := ctx.Eval(`
        const user2 = new User({
            id: 2,
            name: "Bob",
            email_address: "bob@example.com",
            age: 30,
            is_active: true,
            scores: [88.0, 92.5, 85.0]
        });
        
        // Call methods
        user2.UpdateEmail("bob.smith@example.com");
        user2.AddScore(95.0);
        
        // Access fields via accessors (directly map to Go struct fields)
        user2.age = 31;        // Setter: modifies the Go struct field
        const newAge = user2.age; // Getter: reads from the Go struct field
        
        ({
            info: user2.GetFullInfo(),
            email: user2.email_address,  // Accessor getter
            age: newAge,                 // Modified via accessor setter
            scoresType: user2.scores instanceof Float32Array,
            scoresLength: user2.scores.length
        });
    `)
    defer result2.Free()
    
    if result2.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println("Named:", result2.JSONStringify())

    // Demonstrate field accessor synchronization
    result3 := ctx.Eval(`
        const user3 = new User(3, "Charlie", "charlie@example.com", 35, true, []);
        
        // Field accessors provide direct access to Go struct fields
        const originalName = user3.name;  // Getter: reads Go struct field
        
        user3.name = "Charles";           // Setter: modifies Go struct field
        const newName = user3.name;       // Getter: reads modified field
        
        // Changes are synchronized with Go object
        const info = user3.GetFullInfo(); // Method sees the changed name
        
        // Verify synchronization by changing multiple fields
        user3.age = 36;
        user3.email_address = "charles.updated@example.com";
        const updatedInfo = user3.GetFullInfo();
        
        ({
            originalName: originalName,
            newName: newName,
            infoAfterNameChange: info,
            finalInfo: updatedInfo,
            // Demonstrate that Go object is synchronized
            goObjectAge: user3.age,
            goObjectEmail: user3.email_address
        });
    `)
    defer result3.Free()
    
    if result3.IsException() {
        err := ctx.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println("Synchronization demonstration:", result3.JSONStringify())
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
    buf, err := ctx.Compile(jsStr)
    if err != nil {
        panic(err)
    }

    // Create a new runtime
    rt2 := quickjs.NewRuntime()
    defer rt2.Close()

    // Create a new context
    ctx2 := rt2.NewContext()
    defer ctx2.Close()

    //Eval bytecode
    result := ctx2.EvalBytecode(buf)
    defer result.Free()
    
    if result.IsException() {
        err := ctx2.Exception()
        fmt.Println("Error:", err.Error())
        return
    }
    
    fmt.Println(result.ToInt32())
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

    result := ctx.Eval(`var array = []; while (true) { array.push(null) }`)
    defer result.Free()
    
    if result.IsException() {
        err := ctx.Exception()
        fmt.Println("Memory limit exceeded:", err.Error())
    }
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
    r1 := ctx.EvalFile("./test/hello_module.js")
    defer r1.Free()
    if r1.IsException() {
        err := ctx.Exception()
        panic(err)
    }

    // load module
    r2 := ctx.LoadModuleFile("./test/fib_module.js", "fib_foo")
    defer r2.Free()
    if r2.IsException() {
        err := ctx.Exception()
        panic(err)
    }

    // call module
    r3 := ctx.Eval(`
    import {fib} from 'fib_foo';
    globalThis.result = fib(9);
    `)
    defer r3.Free()
    if r3.IsException() {
        err := ctx.Exception()
        panic(err)
    }

    result := ctx.Globals().Get("result")
    defer result.Free()
    fmt.Println("Fibonacci result:", result.ToInt32())
}
```

## License

[MIT License](LICENSE)