# quickjs-go

English | [简体中文](README_zh-cn.md)

[![Test](https://github.com/buke/quickjs-go/workflows/Test/badge.svg)](https://github.com/buke/quickjs-go/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/quickjs-go/graph/badge.svg?token=8z6vgOaIIS)](https://codecov.io/gh/buke/quickjs-go)
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


## Features

- Evaluate script
- Compile script into bytecode and Eval from bytecode
- Operate JavaScript values and objects in Go
- Bind Go function to JavaScript async/sync function
- Simple exception throwing and catching
- **Marshal/Unmarshal Go values to/from JavaScript values**
- **Full TypedArray support (Int8Array, Uint8Array, Float32Array, etc.)**
- **Class Binding with manual and automatic reflection-based approaches**

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

    // bind go function to Javascript async function using Function + Promise
    ctx.Globals().Set("testAsync", ctx.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
        return ctx.Promise(func(resolve, reject func(quickjs.Value)) {
            resolve(ctx.String("Hello Async Function!"))
        })
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

### TypedArray Support

QuickJS-Go provides full support for JavaScript TypedArrays, enabling efficient binary data processing between Go and JavaScript.

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
    int8Array := ctx.Int8Array(int8Data)
    defer int8Array.Free()

    uint8Data := []uint8{0, 128, 255}
    uint8Array := ctx.Uint8Array(uint8Data)
    defer uint8Array.Free()

    float32Data := []float32{-3.14, 0.0, 2.718, 100.5}
    float32Array := ctx.Float32Array(float32Data)
    defer float32Array.Free()

    int64Data := []int64{-9223372036854775808, 0, 9223372036854775807}
    bigInt64Array := ctx.BigInt64Array(int64Data)
    defer bigInt64Array.Free()

    // Set TypedArrays as global variables
    ctx.Globals().Set("int8Array", int8Array)
    ctx.Globals().Set("uint8Array", uint8Array)
    ctx.Globals().Set("float32Array", float32Array)
    ctx.Globals().Set("bigInt64Array", bigInt64Array)

    // Use in JavaScript
    result, _ := ctx.Eval(`
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
    jsTypedArrays, _ := ctx.Eval(`
        ({
            int8: new Int8Array([-128, -1, 0, 1, 127]),
            uint16: new Uint16Array([0, 32768, 65535]),
            float64: new Float64Array([Math.PI, Math.E, 42.5]),
            bigUint64: new BigUint64Array([0n, 18446744073709551615n])
        })
    `)
    defer jsTypedArrays.Free()

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
| `[]int8`   | `Int8Array`          | `ctx.Int8Array()`       | `val.ToInt8Array()` |
| `[]uint8`  | `Uint8Array`         | `ctx.Uint8Array()`      | `val.ToUint8Array()` |
| `[]uint8`  | `Uint8ClampedArray`  | `ctx.Uint8ClampedArray()` | `val.ToUint8Array()` |
| `[]int16`  | `Int16Array`         | `ctx.Int16Array()`      | `val.ToInt16Array()` |
| `[]uint16` | `Uint16Array`        | `ctx.Uint16Array()`     | `val.ToUint16Array()` |
| `[]int32`  | `Int32Array`         | `ctx.Int32Array()`      | `val.ToInt32Array()` |
| `[]uint32` | `Uint32Array`        | `ctx.Uint32Array()`     | `val.ToUint32Array()` |
| `[]float32` | `Float32Array`      | `ctx.Float32Array()`    | `val.ToFloat32Array()` |
| `[]float64` | `Float64Array`      | `ctx.Float64Array()`    | `val.ToFloat64Array()` |
| `[]int64`  | `BigInt64Array`      | `ctx.BigInt64Array()`   | `val.ToBigInt64Array()` |
| `[]uint64` | `BigUint64Array`     | `ctx.BigUint64Array()`  | `val.ToBigUint64Array()` |
| `[]byte`   | `ArrayBuffer`        | `ctx.ArrayBuffer()`     | `val.ToByteArray()` |

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
    regularArray, _ := ctx.Eval(`[1, 2, 3]`)
    defer regularArray.Free()

    int32Array := ctx.Int32Array([]int32{1, 2, 3})
    defer int32Array.Free()

    float64Array := ctx.Float64Array([]float64{1.1, 2.2, 3.3})
    defer float64Array.Free()

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
    imageArray := ctx.Uint8Array(imageData)
    ctx.Globals().Set("imageData", imageArray)

    // Process in JavaScript
    result, _ := ctx.Eval(`
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
    result, _ := ctx.Eval(`
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
    result, _ := ctx.Eval(`
        ({
            scoresType: user.scores instanceof Float32Array,
            dataType: user.data instanceof Int32Array,
            binaryType: user.binary instanceof ArrayBuffer,
            binaryString: new TextDecoder().decode(user.binary),
            avgScore: user.scores.reduce((sum, score) => sum + score) / user.scores.length
        });
    `)
    defer result.Free()
    fmt.Println("TypedArray info:", result.JSONStringify())

    // Modify in JavaScript
    modifyResult, _ := ctx.Eval(`
        user.name = "Alice Smith";
        user.tags.push("moderator");
        // Modify TypedArray data
        user.scores[0] = 98.5;
        user;
    `)
    defer modifyResult.Free()

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

### Class Binding

QuickJS-Go provides powerful class binding capabilities that allow you to seamlessly bridge Go structs to JavaScript classes.

#### Manual Class Binding

Create JavaScript classes manually with full control over methods and properties:

```go
package main

import (
    "fmt"
    "math"
    "github.com/buke/quickjs-go"
)

type Point struct {
    X, Y float64
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
    pointConstructor, _, err := quickjs.NewClassBuilder("Point").
        Constructor(func(ctx *quickjs.Context, newTarget quickjs.Value, args []quickjs.Value) quickjs.Value {
            x, y := 0.0, 0.0
            if len(args) > 0 { x = args[0].ToFloat64() }
            if len(args) > 1 { y = args[1].ToFloat64() }
            
            point := &Point{X: x, Y: y}
            return newTarget.NewInstance(point)
        }).
        Property("x", 
            func(ctx *quickjs.Context, this quickjs.Value) quickjs.Value {
                point, _ := this.GetGoObject()
                return ctx.Float64(point.(*Point).X)
            },
            func(ctx *quickjs.Context, this quickjs.Value, value quickjs.Value) quickjs.Value {
                point, _ := this.GetGoObject()
                point.(*Point).X = value.ToFloat64()
                return value
            }).
        Property("y",
            func(ctx *quickjs.Context, this quickjs.Value) quickjs.Value {
                point, _ := this.GetGoObject()
                return ctx.Float64(point.(*Point).Y)
            },
            func(ctx *quickjs.Context, this quickjs.Value, value quickjs.Value) quickjs.Value {
                point, _ := this.GetGoObject()
                point.(*Point).Y = value.ToFloat64()
                return value
            }).
        Method("distance", func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
            point, _ := this.GetGoObject()
            return ctx.Float64(point.(*Point).Distance())
        }).
        Method("move", func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
            point, _ := this.GetGoObject()
            dx, dy := 0.0, 0.0
            if len(args) > 0 { dx = args[0].ToFloat64() }
            if len(args) > 1 { dy = args[1].ToFloat64() }
            point.(*Point).Move(dx, dy)
            return ctx.Undefined()
        }).
        Build(ctx)

    if err != nil {
        panic(err)
    }

    // Register the class
    ctx.Globals().Set("Point", pointConstructor)

    // Use in JavaScript
    result, _ := ctx.Eval(`
        const p = new Point(3, 4);
        const dist1 = p.distance();
        p.move(1, 1);
        const dist2 = p.distance();
        
        ({ 
            initial: dist1, 
            afterMove: dist2, 
            x: p.x, 
            y: p.y 
        });
    `)
    defer result.Free()
    
    fmt.Println("Result:", result.JSONStringify())
}
```

#### Automatic Class Binding with Reflection

Automatically generate JavaScript classes from Go structs using reflection:

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

type User struct {
    ID        int64     `js:"id"`
    Name      string    `js:"name"`
    Email     string    `json:"email_address"`
    Age       int       `js:"age"`
    IsActive  bool      `js:"is_active"`
    Scores    []float32 `js:"scores"`    // Becomes Float32Array
    private   string    // Not accessible
    Secret    string    `js:"-"`         // Explicitly ignored
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
    userConstructor, _, err := ctx.BindClass(&User{})
    if err != nil {
        panic(err)
    }

    ctx.Globals().Set("User", userConstructor)

    // Use with positional arguments
    result1, _ := ctx.Eval(`
        const user1 = new User(1, "Alice", "alice@example.com", 25, true, [95.5, 87.2]);
        user1.GetFullInfo();
    `)
    defer result1.Free()
    fmt.Println("Positional:", result1.String())

    // Use with named arguments (object parameter)
    result2, _ := ctx.Eval(`
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
        
        ({
            info: user2.GetFullInfo(),
            email: user2.email_address,
            scoresType: user2.scores instanceof Float32Array,
            scoresLength: user2.scores.length
        });
    `)
    defer result2.Free()
    fmt.Println("Named:", result2.JSONStringify())
}
```

#### Advanced Reflection Options

Customize automatic binding with filtering and configuration options:

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

type APIClient struct {
    BaseURL    string `js:"baseUrl"`
    APIKey     string `js:"-"`          // Hidden from JavaScript
    Version    string `js:"version"`
    UserAgent  string `js:"userAgent"`
}

func (c *APIClient) Get(endpoint string) string {
    return fmt.Sprintf("GET %s%s", c.BaseURL, endpoint)
}

func (c *APIClient) Post(endpoint string, data interface{}) string {
    return fmt.Sprintf("POST %s%s with data", c.BaseURL, endpoint)
}

func (c *APIClient) InternalMethod() string {
    return "This should be hidden"
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // Create class with custom options
    clientConstructor, _, err := ctx.BindClass(&APIClient{},
        quickjs.WithMethodPrefix("Get"), // Only include Get* and Post* methods
        quickjs.WithIgnoredMethods("InternalMethod"), // Explicitly ignore methods
        quickjs.WithIgnoredFields("APIKey"), // Ignore additional fields beyond tags
    )
    if err != nil {
        panic(err)
    }

    ctx.Globals().Set("APIClient", clientConstructor)

    result, _ := ctx.Eval(`
        const client = new APIClient({
            baseUrl: "https://api.example.com/v1/",
            version: "1.0",
            userAgent: "MyApp/1.0"
        });
        
        ({
            get: client.Get("/users"),
            post: typeof client.Post !== 'undefined' ? client.Post("/users", {name: "test"}) : "undefined",
            hasInternal: typeof client.InternalMethod !== 'undefined',
            hasAPIKey: typeof client.APIKey !== 'undefined'
        });
    `)
    defer result.Free()
    
    fmt.Println("Filtered binding:", result.JSONStringify())
}
```

#### Class Inheritance Support

JavaScript classes can inherit from Go-registered classes:

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

type Vehicle struct {
    Brand string `js:"brand"`
    Model string `js:"model"`
}

func (v *Vehicle) Start() string {
    return fmt.Sprintf("Starting %s %s", v.Brand, v.Model)
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // Register base Vehicle class
    vehicleConstructor, _, _ := ctx.BindClass(&Vehicle{})
    ctx.Globals().Set("Vehicle", vehicleConstructor)

    // Create Car class that extends Vehicle in JavaScript
    _, err := ctx.Eval(`
        class Car extends Vehicle {
            constructor(brand, model, doors) {
                super({ brand, model });
                this.doors = doors;
            }
            
            getInfo() {
                return this.Start() + " with " + this.doors + " doors";
            }
        }
        
        // Test inheritance
        const car = new Car("Toyota", "Camry", 4);
        globalThis.result = car.getInfo();
    `)
    if err != nil {
        panic(err)
    }

    result := ctx.Globals().Get("result")
    defer result.Free()
    fmt.Println("Inheritance:", result.String())
}
```

#### Features

**Manual Class Binding:**
- Full control over class structure and behavior
- Support for instance and static methods/properties
- Read-only, write-only, and read-write properties
- Constructor with `new.target` support for inheritance
- Automatic memory management with finalizers

**Automatic Reflection Binding:**
- Zero-boilerplate class generation from Go structs
- Smart constructor supporting both positional and named parameters
- Automatic property mapping with `js` and `json` tag support
- Method binding with proper parameter/return value conversion
- TypedArray support for numeric slice fields
- Configurable filtering for methods and fields

**Shared Features:**
- Full JavaScript inheritance support
- Seamless integration with Marshal/Unmarshal system
- TypedArray support for binary data
- Automatic memory management
- Thread-safe operation
- Class instance validation and type checking

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
