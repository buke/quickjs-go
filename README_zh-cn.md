# quickjs-go

[English](README.md) | 简体中文

[![Test](https://github.com/buke/quickjs-go/workflows/Test/badge.svg)](https://github.com/buke/quickjs-go/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/quickjs-go/branch/main/graph/badge.svg?token=DW5RGD01AG)](https://codecov.io/gh/buke/quickjs-go)
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

## 破坏性变更

### v0.5.x

**移除 Collection API**：本项目的主要目标是提供 QuickJS C API 的绑定，因此 collection 相关的 API 将会被移除。以下方法不再可用：

- `ctx.Array()` - 请使用 `ctx.Eval("[]")` 或 `ctx.Object()` 替代
- `ctx.Map()` - 请使用 `ctx.Eval("new Map()")` 替代
- `ctx.Set()` - 请使用 `ctx.Eval("new Set()")` 替代
- `value.ToArray()` - 请直接使用 `Value` 操作替代
- `value.ToMap()` - 请直接使用 `Value` 操作替代
- `value.ToSet()` - 请直接使用 `Value` 操作替代
- `value.IsMap()` - 请使用 `value.GlobalInstanceof("Map")` 替代
- `value.IsSet()` - 请使用 `value.GlobalInstanceof("Set")` 替代

**迁移指南：**

```go
// 之前的版本 (v0.4.x 及更早)
arr := ctx.Array()
arr.Set("0", ctx.String("item"))

mapObj := ctx.Map()
mapObj.Set("key", ctx.String("value"))

setObj := ctx.Set()
setObj.Add(ctx.String("item"))

// 新版本 (v0.5.x)
arr, _ := ctx.Eval("[]")
arr.Set("0", ctx.String("item"))
arr.Set("length", ctx.Int32(1))

mapObj, _ := ctx.Eval("new Map()")
mapObj.Call("set", ctx.String("key"), ctx.String("value"))

setObj, _ := ctx.Eval("new Set()")
setObj.Call("add", ctx.String("item"))
```

## 功能

- 执行 javascript 脚本
- 编译 javascript 脚本到字节码并执行字节码
- 在 Go 中操作 JavaScript 值和对象
- 绑定 Go 函数到 JavaScript 同步函数和异步函数
- 简单的异常抛出和捕获
- **Go 值与 JavaScript 值的序列化和反序列化**
- **完整的 TypedArray 支持 (Int8Array, Uint8Array, Float32Array 等)**

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
