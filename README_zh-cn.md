# quickjs-go

[English](README.md) | 简体中文

[![Test](https://github.com/buke/quickjs-go/workflows/Test/badge.svg)](https://github.com/buke/quickjs-go/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/quickjs-go/graph/badge.svg?token=8z6vgOaIIS)](https://codecov.io/gh/buke/quickjs-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/buke/quickjs-go)](https://goreportcard.com/report/github.com/buke/quickjs-go)
[![GoDoc](https://pkg.go.dev/badge/github.com/buke/quickjs-go?status.svg)](https://pkg.go.dev/github.com/buke/quickjs-go?tab=doc)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_shield)

Go 语言的 QuickJS 绑定库：快速、小型、可嵌入的 ES2020 JavaScript 解释器。

## 平台支持

使用预编译的 quickjs 静态库，支持以下平台：

| 平台    | 架构  | 静态库                                               |
| ------- | ----- | ---------------------------------------------------- |
| Linux   | x64   | [libquickjs.a](deps/libs/linux_amd64/libquickjs.a)   |
| Linux   | arm64 | [libquickjs.a](deps/libs/linux_arm64/libquickjs.a)   |
| Windows | x64   | [libquickjs.a](deps/libs/windows_amd64/libquickjs.a) |
| Windows | x86   | [libquickjs.a](deps/libs/windows_386/libquickjs.a)   |
| MacOS   | x64   | [libquickjs.a](deps/libs/darwin_amd64/libquickjs.a)  |
| MacOS   | arm64 | [libquickjs.a](deps/libs/darwin_arm64/libquickjs.a)  |

\* windows 构建步骤请参考：https://github.com/buke/quickjs-go/issues/151#issuecomment-2134307728

## 版本说明

| quickjs-go | QuickJS     |
| ---------- | ----------- |
| v0.5.x     | v2025-04-26 |
| v0.4.x     | v2024-02-14 |
| v0.3.x     | v2024-01-13 |
| v0.2.x     | v2023-12-09 |
| v0.1.x     | v2021-03-27 |


## 功能

- 执行 javascript 脚本
- 编译 javascript 脚本到字节码并执行字节码
- 在 Go 中操作 JavaScript 值和对象
- 绑定 Go 函数到 JavaScript 同步函数和异步函数
- 简单的异常抛出和捕获
- **Go 值与 JavaScript 值的序列化和反序列化**
- **完整的 TypedArray 支持 (Int8Array, Uint8Array, Float32Array 等)**
- **手动和自动反射的类绑定功能**

## 指南

1. 在使用完毕后，请记得关闭 `quickjs.Runtime` 和 `quickjs.Context`。
2. 请记得关闭由 `Eval()` 和 `EvalFile()` 返回的 `quickjs.Value`。其他值不需要关闭，因为它们会被垃圾回收。
3. 如果你使用了 promise 或 async function，请使用 `ctx.Loop()` 等待所有的 promise/job 结果。
4. 如果`Eval()` 或 `EvalFile()`返回了错误，可强制转换为`*quickjs.Error`以读取错误的堆栈信息。
5. 如果你想在函数中返回参数，请在函数中复制参数。

## 用法

```go
import "github.com/buke/quickjs-go"
```

### 执行 javascript 脚本

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

    // 使用 Function + Promise 绑定 Go 函数为 JavaScript 异步函数
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

### TypedArray 支持

QuickJS-Go 提供对 JavaScript TypedArray 的完整支持，实现 Go 和 JavaScript 之间高效的二进制数据处理。

#### 从 Go 创建 TypedArray

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

    // 从 Go 切片创建各种 TypedArray
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

    // 将 TypedArray 设置为全局变量
    ctx.Globals().Set("int8Array", int8Array)
    ctx.Globals().Set("uint8Array", uint8Array)
    ctx.Globals().Set("float32Array", float32Array)
    ctx.Globals().Set("bigInt64Array", bigInt64Array)

    // 在 JavaScript 中使用
    result, _ := ctx.Eval(`
        // 检查类型
        const results = {
            int8Type: int8Array instanceof Int8Array,
            uint8Type: uint8Array instanceof Uint8Array,
            float32Type: float32Array instanceof Float32Array,
            bigInt64Type: bigInt64Array instanceof BigInt64Array,
            // 计算 float32 数组的和
            float32Sum: float32Array.reduce((sum, val) => sum + val, 0)
        };
        results;
    `)
    defer result.Free()

    fmt.Println("结果:", result.JSONStringify())
}
```

#### 将 JavaScript TypedArray 转换为 Go

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

    // 在 JavaScript 中创建 TypedArray
    jsTypedArrays, _ := ctx.Eval(`
        ({
            int8: new Int8Array([-128, -1, 0, 1, 127]),
            uint16: new Uint16Array([0, 32768, 65535]),
            float64: new Float64Array([Math.PI, Math.E, 42.5]),
            bigUint64: new BigUint64Array([0n, 18446744073709551615n])
        })
    `)
    defer jsTypedArrays.Free()

    // 转换为 Go 切片
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

#### TypedArray 类型支持

| Go 类型    | JavaScript TypedArray | Context 方法            | Value 方法          |
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

#### TypedArray 类型检测

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

    // 创建各种数组
    regularArray, _ := ctx.Eval(`[1, 2, 3]`)
    defer regularArray.Free()

    int32Array := ctx.Int32Array([]int32{1, 2, 3})
    defer int32Array.Free()

    float64Array := ctx.Float64Array([]float64{1.1, 2.2, 3.3})
    defer float64Array.Free()

    // 检测数组类型
    fmt.Printf("普通数组 IsArray: %v\n", regularArray.IsArray())
    fmt.Printf("普通数组 IsTypedArray: %v\n", regularArray.IsTypedArray())

    fmt.Printf("Int32Array IsTypedArray: %v\n", int32Array.IsTypedArray())
    fmt.Printf("Int32Array IsInt32Array: %v\n", int32Array.IsInt32Array())
    fmt.Printf("Int32Array IsFloat64Array: %v\n", int32Array.IsFloat64Array())

    fmt.Printf("Float64Array IsTypedArray: %v\n", float64Array.IsTypedArray())
    fmt.Printf("Float64Array IsFloat64Array: %v\n", float64Array.IsFloat64Array())
    fmt.Printf("Float64Array IsInt32Array: %v\n", float64Array.IsInt32Array())
}
```

#### 二进制数据处理示例

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

    // 处理类似图像的数据 (模拟 RGB 像素)
    imageData := []uint8{
        255, 0, 0,    // 红色像素
        0, 255, 0,    // 绿色像素  
        0, 0, 255,    // 蓝色像素
        255, 255, 0,  // 黄色像素
    }

    // 发送到 JavaScript 作为 Uint8Array
    imageArray := ctx.Uint8Array(imageData)
    ctx.Globals().Set("imageData", imageArray)

    // 在 JavaScript 中处理
    result, _ := ctx.Eval(`
        // 将 RGB 转换为灰度
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

    // 转换回 Go
    if result.IsUint8Array() {
        grayscaleData, err := result.ToUint8Array()
        if err == nil {
            fmt.Printf("原始 RGB: %v\n", imageData)
            fmt.Printf("灰度值: %v\n", grayscaleData)
        }
    }
}
```

### Go 值序列化和反序列化

QuickJS-Go 通过 `Marshal` 和 `Unmarshal` 方法提供 Go 和 JavaScript 值之间的无缝转换。

#### 基本类型

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

    // 将 Go 值序列化为 JavaScript 值
    data := map[string]interface{}{
        "name":    "张三",
        "age":     30,
        "active":  true,
        "scores":  []int{85, 92, 78},
        "address": map[string]string{
            "city":    "北京",
            "country": "中国",
        },
        // TypedArray 将为类型化切片自动创建
        "floatData": []float32{1.1, 2.2, 3.3},
        "intData":   []int32{100, 200, 300},
        "byteData":  []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F}, // "Hello" 的字节
    }

    jsVal, err := ctx.Marshal(data)
    if err != nil {
        panic(err)
    }
    defer jsVal.Free()

    // 在 JavaScript 中使用序列化的值
    ctx.Globals().Set("user", jsVal)
    result, _ := ctx.Eval(`
        const info = user.name + " 今年 " + user.age + " 岁";
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
    fmt.Println("结果:", result.JSONStringify())

    // 将 JavaScript 值反序列化回 Go
    var userData map[string]interface{}
    err = ctx.Unmarshal(jsVal, &userData)
    if err != nil {
        panic(err)
    }
    fmt.Printf("反序列化结果: %+v\n", userData)
}
```

#### 带标签的结构体序列化

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
    // TypedArray 字段
    Scores    []float32 `js:"scores"`    // 将变成 Float32Array
    Data      []int32   `js:"data"`      // 将变成 Int32Array
    Binary    []byte    `js:"binary"`    // 将变成 ArrayBuffer
    // 未导出的字段会被忽略
    password  string
    // 带有 "-" 标签的字段会被跳过
    Secret    string `js:"-"`
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    user := User{
        ID:        123,
        Name:      "小明",
        Email:     "xiaoming@example.com",
        CreatedAt: time.Now(),
        IsActive:  true,
        Tags:      []string{"admin", "user"},
        Scores:    []float32{95.5, 87.2, 92.0},
        Data:      []int32{1000, 2000, 3000},
        Binary:    []byte{0x41, 0x42, 0x43}, // "ABC"
        password:  "secret123",
        Secret:    "top-secret",
    }

    // 将结构体序列化为 JavaScript
    jsVal, err := ctx.Marshal(user)
    if err != nil {
        panic(err)
    }
    defer jsVal.Free()

    // 在 JavaScript 中检查 TypedArray 类型
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
    fmt.Println("TypedArray 信息:", result.JSONStringify())

    // 在 JavaScript 中修改
    modifyResult, _ := ctx.Eval(`
        user.name = "小明同学";
        user.tags.push("moderator");
        // 修改 TypedArray 数据
        user.scores[0] = 98.5;
        user;
    `)
    defer modifyResult.Free()

    // 反序列化回 Go 结构体
    var updatedUser User
    err = ctx.Unmarshal(modifyResult, &updatedUser)
    if err != nil {
        panic(err)
    }

    fmt.Printf("更新后的用户: %+v\n", updatedUser)
    // 注意：password 和 Secret 字段保持不变（未被序列化）
}
```

#### 自定义序列化/反序列化

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

// 实现 Marshaler 接口
func (c CustomType) MarshalJS(ctx *quickjs.Context) (quickjs.Value, error) {
    return ctx.String("custom:" + c.Value), nil
}

// 实现 Unmarshaler 接口
func (c *CustomType) UnmarshalJS(ctx *quickjs.Context, val quickjs.Value) error {
    if val.IsString() {
        str := val.ToString()
        if strings.HasPrefix(str, "custom:") {
            c.Value = str[7:] // 移除 "custom:" 前缀
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

    // 序列化自定义类型
    custom := CustomType{Value: "hello"}
    jsVal, err := ctx.Marshal(custom)
    if err != nil {
        panic(err)
    }
    defer jsVal.Free()

    fmt.Println("序列化结果:", jsVal.String()) // 输出: custom:hello

    // 反序列化
    var result CustomType
    err = ctx.Unmarshal(jsVal, &result)
    if err != nil {
        panic(err)
    }
    fmt.Printf("反序列化结果: %+v\n", result) // 输出: {Value:hello}
}
```

#### 类型映射

**Go 到 JavaScript:**
- `bool` → JavaScript boolean
- `int`, `int8`, `int16`, `int32` → JavaScript number (32位)
- `int64` → JavaScript number (64位)
- `uint`, `uint8`, `uint16`, `uint32` → JavaScript number (32位无符号)
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
- `slice/array` → JavaScript Array (对于非类型化数组)
- `map` → JavaScript Object
- `struct` → JavaScript Object
- `pointer` → 递归序列化指向的值（nil 变为 null）

**JavaScript 到 Go:**
- JavaScript null/undefined → Go nil 指针或零值
- JavaScript boolean → Go bool
- JavaScript number → Go 数值类型（适当转换）
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

当反序列化到 `interface{}` 时，使用以下类型：
- `nil` 对应 null/undefined
- `bool` 对应 boolean
- `int64` 对应整数
- `float64` 对应浮点数
- `string` 对应字符串
- `[]interface{}` 对应 Array
- `map[string]interface{}` 对应 Object
- `*big.Int` 对应 BigInt
- `[]byte` 对应 ArrayBuffer

### 类绑定

QuickJS-Go 提供强大的类绑定功能，允许您无缝地将 Go 结构体桥接到 JavaScript 类，具有自动内存管理和继承支持。

#### 手动类绑定

手动创建 JavaScript 类，完全控制方法、属性和访问器：

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

    // 使用 ClassBuilder 创建 Point 类
    pointConstructor, _, err := quickjs.NewClassBuilder("Point").
        Constructor(func(ctx *quickjs.Context, instance quickjs.Value, args []quickjs.Value) (interface{}, error) {
            x, y := 0.0, 0.0
            name := "未命名点"
            
            if len(args) > 0 { x = args[0].Float64() }
            if len(args) > 1 { y = args[1].Float64() }
            if len(args) > 2 { name = args[2].String() }
            
            // 返回 Go 对象进行自动关联
            return &Point{X: x, Y: y, Name: name}, nil
        }).
        // 访问器提供带有自定义逻辑的 getter/setter 功能
        Accessor("x", 
            func(ctx *quickjs.Context, this quickjs.Value) quickjs.Value {
                point, _ := this.GetGoObject()
                return ctx.Float64(point.(*Point).X)
            },
            func(ctx *quickjs.Context, this quickjs.Value, value quickjs.Value) quickjs.Value {
                point, _ := this.GetGoObject()
                point.(*Point).X = value.Float64()
                return ctx.Undefined()
            }).
        Accessor("y",
            func(ctx *quickjs.Context, this quickjs.Value) quickjs.Value {
                point, _ := this.GetGoObject()
                return ctx.Float64(point.(*Point).Y)
            },
            func(ctx *quickjs.Context, this quickjs.Value, value quickjs.Value) quickjs.Value {
                point, _ := this.GetGoObject()
                point.(*Point).Y = value.Float64()
                return ctx.Undefined()
            }).
        // 属性直接绑定到每个实例（对静态值来说更快）
        Property("version", ctx.String("1.0.0")).
        Property("type", ctx.String("Point")).
        // 只读属性
        Property("readOnly", ctx.Bool(true), quickjs.PropertyConfigurable).
        // 实例方法
        Method("distance", func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
            point, _ := this.GetGoObject()
            return ctx.Float64(point.(*Point).Distance())
        }).
        Method("move", func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
            point, _ := this.GetGoObject()
            dx, dy := 0.0, 0.0
            if len(args) > 0 { dx = args[0].Float64() }
            if len(args) > 1 { dy = args[1].Float64() }
            point.(*Point).Move(dx, dy)
            return ctx.Undefined()
        }).
        Method("getName", func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
            point, _ := this.GetGoObject()
            return ctx.String(point.(*Point).Name)
        }).
        // 静态方法
        StaticMethod("origin", func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
            // 在原点创建新的 Point
            origin := &Point{X: 0, Y: 0, Name: "原点"}
            jsVal, _ := ctx.Marshal(origin)
            return jsVal
        }).
        Build(ctx)

    if err != nil {
        panic(err)
    }

    // 注册类
    ctx.Globals().Set("Point", pointConstructor)

    // 在 JavaScript 中使用
    result, _ := ctx.Eval(`
        const p = new Point(3, 4, "我的点");
        const dist1 = p.distance();
        p.move(1, 1);
        const dist2 = p.distance();
        
        // 静态方法使用
        const origin = Point.origin();
        
        ({ 
            // 访问器使用
            x: p.x,
            y: p.y,
            // 属性使用
            version: p.version,
            type: p.type,
            readOnly: p.readOnly,
            hasOwnProperty: p.hasOwnProperty('version'), // 属性为 true
            // 方法结果
            name: p.getName(),
            initialDistance: dist1,
            finalDistance: dist2,
            // 静态方法结果
            originDistance: Math.sqrt(origin.x * origin.x + origin.y * origin.y)
        });
    `)
    defer result.Free()
    
    fmt.Println("结果:", result.JSONStringify())
    
    // 演示访问器和属性的区别
    propertyTest, _ := ctx.Eval(`
        const p1 = new Point(1, 1);
        const p2 = new Point(2, 2);
        
        // 属性是实例特定的值
        const sameVersion = p1.version === p2.version; // true，相同的静态值
        
        // 访问器提供来自 Go 对象的动态值
        const differentX = p1.x !== p2.x; // true，来自 Go 对象的不同值
        
        ({ sameVersion, differentX });
    `)
    defer propertyTest.Free()
    
    fmt.Println("属性与访问器:", propertyTest.JSONStringify())
}
```

#### 自动反射类绑定

使用反射自动从 Go 结构体生成 JavaScript 类。Go 结构体字段会自动转换为 JavaScript 类访问器，提供 getter/setter 功能，直接映射到底层 Go 对象字段。

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

type User struct {
    ID        int64     `js:"id"`           // 变成访问器: user.id
    Name      string    `js:"name"`         // 变成访问器: user.name
    Email     string    `json:"email_address"` // 变成访问器: user.email_address
    Age       int       `js:"age"`          // 变成访问器: user.age
    IsActive  bool      `js:"is_active"`    // 变成访问器: user.is_active
    Scores    []float32 `js:"scores"`       // 变成访问器: user.scores (Float32Array)
    private   string    // 不可访问（未导出）
    Secret    string    `js:"-"`            // 明确忽略
}

func (u *User) GetFullInfo() string {
    return fmt.Sprintf("%s (%s) - 年龄: %d", u.Name, u.Email, u.Age)
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

    // 自动从结构体创建 User 类
    userConstructor, _, err := ctx.BindClass(&User{})
    if err != nil {
        panic(err)
    }

    ctx.Globals().Set("User", userConstructor)

    // 使用位置参数
    result1, _ := ctx.Eval(`
        const user1 = new User(1, "小明", "xiaoming@example.com", 25, true, [95.5, 87.2]);
        user1.GetFullInfo();
    `)
    defer result1.Free()
    fmt.Println("位置参数:", result1.String())

    // 使用命名参数（对象参数）
    result2, _ := ctx.Eval(`
        const user2 = new User({
            id: 2,
            name: "小红",
            email_address: "xiaohong@example.com",
            age: 30,
            is_active: true,
            scores: [88.0, 92.5, 85.0]
        });
        
        // 调用方法
        user2.UpdateEmail("xiaohong.new@example.com");
        user2.AddScore(95.0);
        
        // 通过访问器访问字段（直接映射到 Go 结构体字段）
        user2.age = 31;        // Setter: 修改 Go 结构体字段
        const newAge = user2.age; // Getter: 从 Go 结构体字段读取
        
        ({
            info: user2.GetFullInfo(),
            email: user2.email_address,  // 访问器 getter
            age: newAge,                 // 通过访问器 setter 修改
            scoresType: user2.scores instanceof Float32Array,
            scoresLength: user2.scores.length
        });
    `)
    defer result2.Free()
    fmt.Println("命名参数:", result2.JSONStringify())

    // 演示字段访问器行为
    result3, _ := ctx.Eval(`
        const user3 = new User(3, "小刚", "xiaogang@example.com", 35, true, []);
        
        // 字段访问器提供对 Go 结构体字段的直接访问
        const originalName = user3.name;  // Getter: 读取 Go 结构体字段
        user3.name = "小刚同学";           // Setter: 修改 Go 结构体字段
        const newName = user3.name;       // Getter: 读取修改后的字段
        
        // 更改会反映在 Go 对象中
        const info = user3.GetFullInfo(); // 方法看到更改的名称
        
        ({
            originalName: originalName,
            newName: newName,
            infoWithNewName: info
        });
    `)
    defer result3.Free()
    fmt.Println("字段访问器:", result3.JSONStringify())
}
```

#### 高级反射选项

使用过滤和配置选项自定义自动绑定：

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

type APIClient struct {
    BaseURL    string `js:"baseUrl"`      // 变成访问器: client.baseUrl
    APIKey     string `js:"-"`            // 从 JavaScript 隐藏
    Version    string `js:"version"`      // 变成访问器: client.version
    UserAgent  string `js:"userAgent"`    // 变成访问器: client.userAgent
}

func (c *APIClient) Get(endpoint string) string {
    return fmt.Sprintf("GET %s%s", c.BaseURL, endpoint)
}

func (c *APIClient) Post(endpoint string, data interface{}) string {
    return fmt.Sprintf("POST %s%s with data", c.BaseURL, endpoint)
}

func (c *APIClient) InternalMethod() string {
    return "这应该被隐藏"
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // 使用自定义选项创建类
    clientConstructor, _, err := ctx.BindClass(&APIClient{},
        quickjs.WithMethodPrefix("Get"), // 只包含 Get* 和 Post* 方法
        quickjs.WithIgnoredMethods("InternalMethod"), // 明确忽略方法
        quickjs.WithIgnoredFields("APIKey"), // 忽略标签之外的额外字段
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
        
        // 字段访问器对所有导出字段都有效
        const originalUrl = client.baseUrl;  // Getter
        client.baseUrl = "https://api.example.com/v2/"; // Setter: 更新 Go 结构体
        const newUrl = client.baseUrl;       // Getter 显示更新的值
        
        ({
            get: client.Get("/users"),
            post: typeof client.Post !== 'undefined' ? client.Post("/users", {name: "test"}) : "undefined",
            hasInternal: typeof client.InternalMethod !== 'undefined',
            hasAPIKey: typeof client.APIKey !== 'undefined',
            originalUrl: originalUrl,
            newUrl: newUrl,
            // 字段访问器更改会反映在方法调用中
            getWithNewUrl: client.Get("/users")
        });
    `)
    defer result.Free()
    
    fmt.Println("过滤绑定:", result.JSONStringify())
}
```

#### 类继承支持

JavaScript 类可以继承 Go 注册的类：

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

type Vehicle struct {
    Brand string `js:"brand"`  // 访问器: vehicle.brand
    Model string `js:"model"`  // 访问器: vehicle.model
}

func (v *Vehicle) Start() string {
    return fmt.Sprintf("启动 %s %s", v.Brand, v.Model)
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // 注册基础 Vehicle 类
    vehicleConstructor, _, _ := ctx.BindClass(&Vehicle{})
    ctx.Globals().Set("Vehicle", vehicleConstructor)

    // 在 JavaScript 中创建继承 Vehicle 的 Car 类
    _, err := ctx.Eval(`
        class Car extends Vehicle {
            constructor(brand, model, doors) {
                super({ brand, model });
                this.doors = doors;
            }
            
            getInfo() {
                // 继承的字段访问器在 JavaScript 子类中工作
                return this.Start() + " 有 " + this.doors + " 个门";
            }
            
            setBrandAndModel(brand, model) {
                // 字段访问器可以用来修改 Go 结构体字段
                this.brand = brand;  // Setter 访问器
                this.model = model;  // Setter 访问器
            }
        }
        
        // 测试字段访问器的继承
        const car = new Car("丰田", "凯美瑞", 4);
        const info1 = car.getInfo();
        
        // 通过继承的字段访问器修改
        car.setBrandAndModel("本田", "雅阁");
        const info2 = car.getInfo();
        
        globalThis.result = { 
            original: info1, 
            modified: info2,
            brand: car.brand,  // Getter 访问器
            model: car.model   // Getter 访问器
        };
    `)
    if err != nil {
        panic(err)
    }

    result := ctx.Globals().Get("result")
    defer result.Free()
    fmt.Println("继承与访问器:", result.JSONStringify())
}
```

#### 直接构造函数调用

使用 `CallConstructor` 从 Go 直接实例化类：

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

type Point struct {
    X, Y float64
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // 创建 Point 类
    pointConstructor, _, err := quickjs.NewClassBuilder("Point").
        Constructor(func(ctx *quickjs.Context, instance quickjs.Value, args []quickjs.Value) (interface{}, error) {
            x, y := 0.0, 0.0
            if len(args) > 0 { x = args[0].Float64() }
            if len(args) > 1 { y = args[1].Float64() }
            return &Point{X: x, Y: y}, nil
        }).
        Method("toString", func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
            point, _ := this.GetGoObject()
            p := point.(*Point)
            return ctx.String(fmt.Sprintf("Point(%g, %g)", p.X, p.Y))
        }).
        Build(ctx)

    if err != nil {
        panic(err)
    }

    // 从 Go 直接调用构造函数
    instance := pointConstructor.CallConstructor(ctx.Float64(10), ctx.Float64(20))
    defer instance.Free()

    // 调用实例方法
    result := instance.Call("toString")
    defer result.Free()

    fmt.Println("直接调用结果:", result.String()) // Point(10, 20)

    // 从 JavaScript 实例获取 Go 对象
    goObj, err := instance.GetGoObject()
    if err == nil {
        point := goObj.(*Point)
        fmt.Printf("Go 对象: {X: %g, Y: %g}\n", point.X, point.Y)
    }
}
```

#### 构造函数错误处理

优雅地处理构造函数错误：

```go
package main

import (
    "fmt"
    "errors"
    "strings"
    "github.com/buke/quickjs-go"
)

type ValidatedUser struct {
    Name  string
    Email string
}

func main() {
    rt := quickjs.NewRuntime()
    defer rt.Close()
    ctx := rt.NewContext()
    defer ctx.Close()

    // 创建带有构造函数验证的类
    userConstructor, _, err := quickjs.NewClassBuilder("ValidatedUser").
        Constructor(func(ctx *quickjs.Context, instance quickjs.Value, args []quickjs.Value) (interface{}, error) {
            if len(args) < 2 {
                return nil, errors.New("ValidatedUser 需要姓名和邮箱")
            }
            
            name := args[0].String()
            email := args[1].String()
            
            if name == "" {
                return nil, errors.New("姓名不能为空")
            }
            
            if email == "" || !strings.Contains(email, "@") {
                return nil, errors.New("无效的邮箱地址")
            }
            
            return &ValidatedUser{Name: name, Email: email}, nil
        }).
        Method("getInfo", func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
            user, _ := this.GetGoObject()
            u := user.(*ValidatedUser)
            return ctx.String(fmt.Sprintf("%s <%s>", u.Name, u.Email))
        }).
        Build(ctx)

    if err != nil {
        panic(err)
    }

    ctx.Globals().Set("ValidatedUser", userConstructor)

    // 测试构造函数错误处理
    result, _ := ctx.Eval(`
        try {
            const user1 = new ValidatedUser("小明", "xiaoming@example.com");
            const info1 = user1.getInfo();
            
            const user2 = new ValidatedUser("", "invalid-email");
            const info2 = user2.getInfo();
            
            [info1, info2];
        } catch (error) {
            error.message;
        }
    `)
    defer result.Free()
    
    fmt.Println("构造函数错误处理:", result.String())
}
```

#### 功能特性

**手动类绑定：**
- 完全控制类结构和行为
- 支持实例和静态方法/属性
- 带有自动实例绑定的数据属性
- 只读、只写和读写访问器
- 带有错误处理和实例预创建的构造函数
- 带有终结器的自动内存管理

**自动反射绑定：**
- 从 Go 结构体零样板代码生成类
- 智能构造函数支持位置和命名参数
- **自动字段到访问器映射**: Go 结构体字段变成 JavaScript 访问器，具有 getter/setter 功能
- 直接字段访问: `obj.field = value` 修改底层 Go 结构体字段
- 带有 `js` 和 `json` 标签支持的自动属性映射
- 方法绑定和正确的参数/返回值转换
- 数值切片字段的 TypedArray 支持
- 方法和字段的可配置过滤

**共享功能：**
- 完整的 JavaScript 继承支持
- 与 Marshal/Unmarshal 系统无缝集成
- 二进制数据的 TypedArray 支持
- 带有 JavaScript 异常的构造函数错误处理
- 自动内存管理和清理
- 线程安全操作
- 类实例验证和类型检查
- 从 Go 代码直接构造函数调用

### Bytecode 编译和执行

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

### 设置内存、栈、GC 等等

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

### ES6 模块支持

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

## 文档

Go 语言文档和示例: https://pkg.go.dev/github.com/buke/quickjs-go

## 协议

[MIT](./LICENSE)

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_large)

## 相关项目

- https://github.com/buke/quickjs-go-polyfill
