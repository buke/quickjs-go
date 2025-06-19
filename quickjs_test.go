package quickjs_test

import (
	"fmt"

	"github.com/buke/quickjs-go"
)

// User represents a common user struct for demonstrating Marshal and Class features
type User struct {
	ID       int64     `js:"id"`
	Name     string    `js:"name"`
	Email    string    `js:"email"`
	Age      int       `js:"age"`
	IsActive bool      `js:"is_active"`
	Scores   []float32 `js:"scores"` // Demonstrates TypedArray support
}

// GetFullInfo returns the user's full information
func (u *User) GetFullInfo() string {
	return fmt.Sprintf("%s (%s) - Age: %d", u.Name, u.Email, u.Age)
}

// GetAverageScore calculates the average score
func (u *User) GetAverageScore() float64 {
	if len(u.Scores) == 0 {
		return 0
	}
	var sum float32
	for _, score := range u.Scores {
		sum += score
	}
	return float64(sum) / float64(len(u.Scores))
}

func Example() {
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
	test.Set("A", ctx.String("String A"))
	test.Set("B", ctx.Int32(0))
	test.Set("C", ctx.Bool(false))
	// bind go function to js object - UPDATED: function signature now uses pointers
	test.Set("hello", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		return ctx.String("Hello " + args[0].String())
	}))

	// bind "test" object to global object
	ctx.Globals().Set("test", test)

	// call js function by js
	js_ret, _ := ctx.Eval(`test.hello("Javascript!")`)
	defer js_ret.Free()
	fmt.Println(js_ret.String())

	// call js function by go
	go_ret := ctx.Globals().Get("test").Call("hello", ctx.String("Golang!"))
	defer go_ret.Free()
	fmt.Println(go_ret.String())

	// bind go function to Javascript async function using Function + Promise - UPDATED: function signature now uses pointers
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

	// wait for promise resolve
	ctx.Loop()

	asyncRet, _ := ctx.Eval("ret")
	defer asyncRet.Free()
	fmt.Println(asyncRet.String())

	// Demonstrate TypedArray functionality
	floatData := []float32{95.5, 87.2, 92.0}
	typedArray := ctx.Float32Array(floatData)
	ctx.Globals().Set("floatData", typedArray)

	arrayResult, _ := ctx.Eval(`floatData instanceof Float32Array`)
	defer arrayResult.Free()
	fmt.Println("TypedArray:", arrayResult.Bool())

	// Demonstrate Marshal/Unmarshal functionality with User struct
	user := User{
		ID:       123,
		Name:     "Alice",
		Email:    "alice@example.com",
		Age:      25,
		IsActive: true,
		Scores:   []float32{95.5, 87.2, 92.0},
	}

	jsVal, _ := ctx.Marshal(user)
	ctx.Globals().Set("userData", jsVal)

	marshalResult, _ := ctx.Eval(`userData.name + " avg: " + (userData.scores.reduce((s,v) => s+v) / userData.scores.length).toFixed(1)`)
	defer marshalResult.Free()
	fmt.Println("Marshal:", marshalResult.String())

	// Demonstrate Class Binding functionality with the same User struct
	userConstructor, _, _ := ctx.BindClass(&User{})
	ctx.Globals().Set("User", userConstructor)

	classResult, _ := ctx.Eval(`
        const user = new User({
            id: 456,
            name: "Bob",
            email: "bob@example.com",
            age: 30,
            is_active: true,
            scores: [88.0, 92.5, 85.0]
        });
        user.GetAverageScore().toFixed(1)
    `)
	defer classResult.Free()
	fmt.Println("Class binding:", classResult.String())

	// Output:
	// Hello Javascript!
	// Hello Golang!
	// Hello Async Function!
	// TypedArray: true
	// Marshal: Alice avg: 91.6
	// Class binding: 88.5
}
