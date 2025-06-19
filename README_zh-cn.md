# quickjs-go

[English](README.md) | 简体中文

[![Test](https://github.com/buke/quickjs-go/workflows/Test/badge.svg)](https://github.com/buke/quickjs-go/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/quickjs-go/graph/badge.svg?token=8z6vgOaIIS)](https://codecov.io/gh/buke/quickjs-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/buke/quickjs-go)](https://goreportcard.com/report/github.com/buke/quickjs-go)
[![GoDoc](https://pkg.go.dev/badge/github.com/buke/quickjs-go?status.svg)](https://pkg.go.dev/github.com/buke/quickjs-go?tab=doc)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_shield)

Go 语言的 QuickJS 绑定库：快速、小型、可嵌入的 ES2020 JavaScript 解释器。

**⚠️ 此项目尚未准备好用于生产环境。请自行承担使用风险。API 可能会随时更改。**

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

\* Windows 构建步骤请参考：https://github.com/buke/quickjs-go/issues/151#issuecomment-2134307728

## 版本说明

| quickjs-go | QuickJS     |
| ---------- | ----------- |
| v0.5.x     | v2025-04-26 |
| v0.4.x     | v2024-02-14 |
| v0.3.x     | v2024-01-13 |
| v0.2.x     | v2023-12-09 |
| v0.1.x     | v2021-03-27 |

## 功能特性

- 执行 JavaScript 脚本
- 编译脚本到字节码并执行字节码
- 在 Go 中操作 JavaScript 值和对象
- 绑定 Go 函数到 JavaScript 同步/异步函数
- 简单的异常抛出和捕获
- **Go 值与 JavaScript 值的序列化/反序列化**
- **完整的 TypedArray 支持 (Int8Array, Uint8Array, Float32Array 等)**
- **使用 ClassBuilder 从 Go 创建 JavaScript 类**
- **使用 ModuleBuilder 从 Go 创建 JavaScript 模块**

## 重大变更

### 自 v0.5.10 版本开始

**值类型系统从 Value 改为 *Value**

所有 `Value` 参数和返回值都已从值类型更改为指针类型 (`*Value`)。

#### 受影响的 API

**Context 方法:**
- `Context.Function(fn func(*Context, Value, []Value) Value)` → `Context.Function(fn func(*Context, *Value, []*Value) *Value`
- 所有值创建方法现在返回 `*Value` 而不是 `Value`

**类系统:**
- `ClassConstructorFunc: func(*Context, Value, []Value) (interface{}, error)` → `func(*Context, *Value, []*Value) (interface{}, error)`
- `ClassMethodFunc: func(*Context, Value, []Value) Value` → `func(*Context, *Value, []*Value) *Value`
- `ClassGetterFunc: func(*Context, Value) Value` → `func(*Context, *Value) *Value`
- `ClassSetterFunc: func(*Context, Value, Value) Value` → `func(*Context, *Value, *Value) *Value`

**Value 方法:**
- `Value.Call()`, `Value.Execute()`, `Value.New()` 等现在接受 `[]*Value` 而不是 `[]Value`
- `Value.Set()`, `Value.Get()` 等现在使用 `*Value` 参数

## 使用指南

1. 在使用完毕后，请记得关闭 `quickjs.Runtime` 和 `quickjs.Context`。
2. 在不再需要时手动释放 `quickjs.Value` 以防止内存泄漏。QuickJS 使用引用计数，所以如果一个值被其他对象引用，你只需要确保引用对象被正确释放。
3. 如果你使用了 promise/job，请使用 `ctx.Loop()` 等待 promise/job 结果
4. 如果 `Eval()` 或 `EvalFile()` 返回了错误，可强制转换为 `*quickjs.Error` 以读取错误的堆栈信息。
5. 如果你想在函数中返回参数，请在函数中复制参数。


## 用法

```go
import "github.com/buke/quickjs-go"
```

### 执行脚本

```go
package main

import (
    "fmt"

    "github.com/buke/quickjs-go"
)

func main() {
    // 创建新的运行时
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // 创建新的上下文
    ctx := rt.NewContext()
    defer ctx.Close()

    ret, err := ctx.Eval("'Hello ' + 'QuickJS!'")
    if err != nil {
        println(err.Error())
    }
    defer ret.Free()
    fmt.Println(ret.String())
}
```

### 获取/设置 JavaScript 对象

```go
package main

import (
    "fmt"

    "github.com/buke/quickjs-go"
)

func main() {
    // 创建新的运行时
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // 创建新的上下文
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

### 绑定 Go 函数到 JavaScript 同步/异步函数

```go
package main

import (
    "fmt"
    "github.com/buke/quickjs-go"
)

func main() {
    // 创建新的运行时
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // 创建新的上下文
    ctx := rt.NewContext()
    defer ctx.Close()

    // 创建新对象
    test := ctx.Object()
    // 将属性绑定到对象
    test.Set("A", ctx.String("String A"))
    test.Set("B", ctx.Int32(0))
    test.Set("C", ctx.Bool(false))
    // 将 go 函数绑定到 js 对象
    test.Set("hello", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        return ctx.String("Hello " + args[0].String())
    }))

    // 将 "test" 对象绑定到全局对象
    ctx.Globals().Set("test", test)

    // 通过 js 调用 js 函数
    js_ret, _ := ctx.Eval(`test.hello("Javascript!")`)
    defer js_ret.Free()
    fmt.Println(js_ret.String())

    // 通过 go 调用 js 函数
    go_ret := test.Call("hello", ctx.String("Golang!"))
    defer go_ret.Free()
    fmt.Println(go_ret.String())

    // 使用 Function + Promise 将 Go 函数绑定为 JavaScript 异步函数
    ctx.Globals().Set("testAsync", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        return ctx.Promise(func(resolve, reject func(*quickjs.Value)) {
            resolve(ctx.String("Hello Async Function!"))
        })
    }))

    ret, _ := ctx.Eval(`
            var ret;
            testAsync().then(v => ret = v)
        `)
    defer ret.Free()

    // 等待 promise 解析
    ctx.Loop()

    // 获取 promise 结果
    asyncRet, _ := ctx.Eval("ret")
    defer asyncRet.Free()

    fmt.Println(asyncRet.String())

    // 输出:
    // Hello Javascript!
    // Hello Golang!
    // Hello Async Function!
}
```

### 错误处理

```go
package main

import (
    "fmt"
    "errors"

    "github.com/buke/quickjs-go"
)

func main() {
    // 创建新的运行时
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // 创建新的上下文
    ctx := rt.NewContext()
    defer ctx.Close()

    ctx.Globals().SetFunction("A", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        // 抛出错误
        return ctx.ThrowError(errors.New("expected error"))
    })

    _, actual := ctx.Eval("A()")
    fmt.Println(actual.Error())
}
```

### TypedArray 支持

QuickJS-Go 提供对 JavaScript TypedArray 的支持，实现 Go 和 JavaScript 之间的二进制数据处理。

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

    uint8Data := []uint8{0, 128, 255}
    uint8Array := ctx.Uint8Array(uint8Data)

    float32Data := []float32{-3.14, 0.0, 2.718, 100.5}
    float32Array := ctx.Float32Array(float32Data)

    int64Data := []int64{-9223372036854775808, 0, 9223372036854775807}
    bigInt64Array := ctx.BigInt64Array(int64Data)

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
    float64Array := ctx.Float64Array([]float64{1.1, 2.2, 3.3})

    // 将数组设置为全局变量以便被全局对象引用
    ctx.Globals().Set("int32Array", int32Array)
    ctx.Globals().Set("float64Array", float64Array)

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

QuickJS-Go 通过 `Marshal` 和 `Unmarshal` 方法提供 Go 和 JavaScript 值之间的转换。

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
        // 类型化切片将自动创建 TypedArray
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
func (c CustomType) MarshalJS(ctx *quickjs.Context) (*quickjs.Value, error) {
    return ctx.String("custom:" + c.Value), nil
}

// 实现 Unmarshaler 接口
func (c *CustomType) UnmarshalJS(ctx *quickjs.Context, val *quickjs.Value) error {
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

### 使用 ModuleBuilder 从 Go 创建 JavaScript 模块

ModuleBuilder API 允许您从 Go 代码创建 JavaScript 模块，使 Go 函数、值和对象可以通过标准 ES6 import 语法在 JavaScript 应用程序中使用。

#### 基本模块创建

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

    // 创建包含 Go 函数和值的数学模块
    addFunc := ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        if len(args) >= 2 {
            return ctx.Float64(args[0].Float64() + args[1].Float64())
        }
        return ctx.Float64(0)
    })
    defer addFunc.Free()

    // 使用流畅 API 构建模块
    module := quickjs.NewModuleBuilder("math").
        Export("PI", ctx.Float64(3.14159)).
        Export("add", addFunc).
        Export("version", ctx.String("1.0.0")).
        Export("default", ctx.String("数学模块"))

    err := module.Build(ctx)
    if err != nil {
        panic(err)
    }

    // 在 JavaScript 中使用标准 ES6 import 使用模块
    result, err := ctx.Eval(`
        (async function() {
            // 命名导入
            const { PI, add, version } = await import('math');
            
            // 使用导入的函数和值
            const sum = add(PI, 1.0);
            return { sum, version };
        })()
    `, quickjs.EvalAwait(true))
    defer result.Free()

    if err != nil {
        panic(err)
    }

    fmt.Println("模块结果:", result.JSONStringify())
    // 输出: 模块结果: {"sum":4.14159,"version":"1.0.0"}
}
```

#### 高级模块功能

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

    // 创建包含复杂对象的实用工具模块
    config := ctx.Object()
    config.Set("appName", ctx.String("我的应用"))
    config.Set("version", ctx.String("2.0.0"))
    config.Set("debug", ctx.Bool(true))

    greetFunc := ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        name := "世界"
        if len(args) > 0 {
            name = args[0].String()
        }
        return ctx.String(fmt.Sprintf("你好, %s!", name))
    })
    defer greetFunc.Free()

    jsonVal := ctx.ParseJSON(`{"MAX": 100, "MIN": 1}`)
    defer jsonVal.Free()

    // 创建包含各种导出类型的模块
    module := quickjs.NewModuleBuilder("utils").
        Export("config", config).                    // 对象导出
        Export("greet", greetFunc).                  // 函数导出
        Export("constants", jsonVal).                // JSON 导出
        Export("default", ctx.String("实用工具库"))  // 默认导出

    err := module.Build(ctx)
    if err != nil {
        panic(err)
    }

    // 在 JavaScript 中使用混合导入
    result, err := ctx.Eval(`
        (async function() {
            // 从 utils 模块导入
            const { greet, config, constants } = await import('utils');
            
            // 组合功能
            const message = greet("JavaScript");
            const info = config.appName + " v" + config.version;
            const limits = "Max: " + constants.MAX + ", Min: " + constants.MIN;
            
            return { message, info, limits };
        })()
    `, quickjs.EvalAwait(true))
    defer result.Free()

    if err != nil {
        panic(err)
    }

    fmt.Println("高级模块结果:", result.JSONStringify())
}
```

#### 多模块集成

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

    // 创建数学模块
    addFunc := ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        if len(args) >= 2 {
            return ctx.Float64(args[0].Float64() + args[1].Float64())
        }
        return ctx.Float64(0)
    })
    defer addFunc.Free()

    mathModule := quickjs.NewModuleBuilder("math").
        Export("add", addFunc).
        Export("PI", ctx.Float64(3.14159))

    // 创建字符串实用工具模块
    upperFunc := ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
        if len(args) > 0 {
            return ctx.String(strings.ToUpper(args[0].String()))
        }
        return ctx.String("")
    })
    defer upperFunc.Free()

    stringModule := quickjs.NewModuleBuilder("strings").
        Export("upper", upperFunc)

    // 构建两个模块
    err := mathModule.Build(ctx)
    if err != nil {
        panic(err)
    }

    err = stringModule.Build(ctx)
    if err != nil {
        panic(err)
    }

    // 一起使用多个模块
    result, err := ctx.Eval(`
        (async function() {
            // 从多个模块导入
            const { add, PI } = await import('math');
            const { upper } = await import('strings');
            
            // 组合功能
            const sum = add(PI, 1);
            const message = "结果: " + sum.toFixed(2);
            const finalMessage = upper(message);
            
            return finalMessage;
        })()
    `, quickjs.EvalAwait(true))
    defer result.Free()

    if err != nil {
        panic(err)
    }

    fmt.Println("多模块结果:", result.String())
    // 输出: 多模块结果: 结果: 4.14
}
```

#### ModuleBuilder API 参考

**核心方法:**
- `NewModuleBuilder(name)` - 创建具有指定名称的新模块构建器
- `Export(name, value)` - 向模块添加命名导出（可链式调用的方法）
- `Build(ctx)` - 在 JavaScript 上下文中注册模块

### 使用 ClassBuilder 从 Go 创建 JavaScript 类

ClassBuilder API 允许您从 Go 代码创建 JavaScript 类。

#### 手动类创建

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
        Constructor(func(ctx *quickjs.Context, instance *quickjs.Value, args []*quickjs.Value) (interface{}, error) {
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
            func(ctx *quickjs.Context, this *quickjs.Value) *quickjs.Value {
                point, _ := this.GetGoObject()
                return ctx.Float64(point.(*Point).X)
            },
            func(ctx *quickjs.Context, this *quickjs.Value, value *quickjs.Value) *quickjs.Value {
                point, _ := this.GetGoObject()
                point.(*Point).X = value.Float64()
                return ctx.Undefined()
            }).
        Accessor("y",
            func(ctx *quickjs.Context, this *quickjs.Value) *quickjs.Value {
                point, _ := this.GetGoObject()
                return ctx.Float64(point.(*Point).Y)
            },
            func(ctx *quickjs.Context, this *quickjs.Value, value *quickjs.Value) *quickjs.Value {
                point, _ := this.GetGoObject()
                point.(*Point).Y = value.Float64()
                return ctx.Undefined()
            }).
        // 属性直接绑定到每个实例
        Property("version", ctx.String("1.0.0")).
        Property("type", ctx.String("Point")).
        // 只读属性
        Property("readOnly", ctx.Bool(true), quickjs.PropertyConfigurable).
        // 实例方法
        Method("distance", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
            point, _ := this.GetGoObject()
            return ctx.Float64(point.(*Point).Distance())
        }).
        Method("move", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
            point, _ := this.GetGoObject()
            dx, dy := 0.0, 0.0
            if len(args) > 0 { dx = args[0].Float64() }
            if len(args) > 1 { dy = args[1].Float64() }
            point.(*Point).Move(dx, dy)
            return ctx.Undefined()
        }).
        Method("getName", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
            point, _ := this.GetGoObject()
            return ctx.String(point.(*Point).Name)
        }).
        // 静态方法
        StaticMethod("origin", func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
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
    
    // 演示访问器和属性之间的区别
    propertyTest, _ := ctx.Eval(`
        const p1 = new Point(1, 1);
        const p2 = new Point(2, 2);
        
        // 属性是实例特定的值
        const sameVersion = p1.version === p2.version; // true, 相同的静态值
        
        // 访问器提供来自 Go 对象的动态值
        const differentX = p1.x !== p2.x; // true, 来自 Go 对象的不同值
        
        ({ sameVersion, differentX });
    `)
    defer propertyTest.Free()
    
    fmt.Println("属性 vs 访问器:", propertyTest.JSONStringify())
}
```

#### 使用反射自动创建类

使用反射从 Go 结构体自动生成 JavaScript 类。Go 结构体字段会自动转换为 JavaScript 类访问器，提供 getter/setter 功能，直接映射到底层 Go 对象字段。

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

    // 演示字段访问器同步
    result3, _ := ctx.Eval(`
        const user3 = new User(3, "小刚", "xiaogang@example.com", 35, true, []);
        
        // 字段访问器提供对 Go 结构体字段的直接访问
        const originalName = user3.name;  // Getter: 读取 Go 结构体字段
        
        user3.name = "小刚同学";           // Setter: 修改 Go 结构体字段
        const newName = user3.name;       // Getter: 读取修改后的字段
        
        // 更改与 Go 对象同步
        const info = user3.GetFullInfo(); // 方法看到更改的名称
        
        // 通过更改多个字段验证同步
        user3.age = 36;
        user3.email_address = "xiaogang.updated@example.com";
        const updatedInfo = user3.GetFullInfo();
        
        ({
            originalName: originalName,
            newName: newName,
            infoAfterNameChange: info,
            finalInfo: updatedInfo,
            // 演示 Go 对象已同步
            goObjectAge: user3.age,
            goObjectEmail: user3.email_address
        });
    `)
    defer result3.Free()
    fmt.Println("同步演示:", result3.JSONStringify())
}
```

### 字节码编译器

```go
package main

import (
    "fmt"

    "github.com/buke/quickjs-go"
)

func main() {
    // 创建新的运行时
    rt := quickjs.NewRuntime()
    defer rt.Close()
    // 创建新的上下文
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
    // 将脚本编译为字节码
    buf, _ := ctx.Compile(jsStr)

    // 创建新的运行时
    rt2 := quickjs.NewRuntime()
    defer rt2.Close()

    // 创建新的上下文
    ctx2 := rt2.NewContext()
    defer ctx2.Close()

    // 执行字节码
    result, _ := ctx2.EvalBytecode(buf)
    defer result.Free()
    fmt.Println(result.Int32())
}
```

### 运行时选项：内存、栈、GC 等

```go
package main

import (
    "fmt"

    "github.com/buke/quickjs-go"
)

func main() {
    // 创建新的运行时
    rt := quickjs.NewRuntime()
    defer rt.Close()

    // 设置运行时选项
    rt.SetExecuteTimeout(30) // 设置执行超时为 30 秒
    rt.SetMemoryLimit(256 * 1024) // 设置内存限制为 256KB
    rt.SetMaxStackSize(65534) // 设置最大栈大小为 65534
    rt.SetGCThreshold(256 * 1024) // 设置 GC 阈值为 256KB
    rt.SetCanBlock(true) // 设置可以阻塞为 true

    // 创建新的上下文
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
    // 启用模块导入
    rt := quickjs.NewRuntime(quickjs.WithModuleImport(true))
    defer rt.Close()

    ctx := rt.NewContext()
    defer ctx.Close()

    // 执行模块
    r1, err := ctx.EvalFile("./test/hello_module.js")
    defer r1.Free()
    if err != nil {
        panic(err)
    }

    // 加载模块
    r2, err := ctx.LoadModuleFile("./test/fib_module.js", "fib_foo")
    defer r2.Free()
    if err != nil {
        panic(err)
    }

    // 调用模块
    r3, err := ctx.Eval(`
    import {fib} from 'fib_foo';
    globalThis.result = fib(9);
    `)
    defer r3.Free()
    if err != nil {
        panic(err)
    }

    result := ctx.Globals().Get("result")
    defer result.Free()
    fmt.Println("斐波那契结果:", result.Int32())
}
```

## 文档

Go 语言文档和示例: https://pkg.go.dev/github.com/buke/quickjs-go

## 协议

[MIT](./LICENSE)

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fbuke%2Fquickjs-go?ref=badge_large)

## 相关项目

- https://github.com/buke/quickjs-go-polyfill
