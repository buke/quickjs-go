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
	test := ctx.NewObject()
	defer test.Free()
	// bind properties to the object
	test.Set("A", ctx.NewString("String A"))
	test.Set("B", ctx.NewInt32(0))
	test.Set("C", ctx.NewBool(false))
	// bind go function to js object - UPDATED: function signature now uses pointers
	test.Set("hello", ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		return ctx.NewString("Hello " + args[0].ToString())
	}))

	// bind "test" object to global object
	ctx.Globals().Set("test", test)

	// call js function by js - FIXED: removed error handling
	js_ret := ctx.Eval(`test.hello("Javascript!")`)
	defer js_ret.Free()
	// Check for exceptions instead of error
	if js_ret.IsException() {
		err := ctx.Exception()
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(js_ret.ToString())

	// call js function by go
	go_ret := ctx.Globals().Get("test").Call("hello", ctx.NewString("Golang!"))
	defer go_ret.Free()
	fmt.Println(go_ret.ToString())

	// bind go function to Javascript async function using Function + Promise - UPDATED: function signature now uses pointers
	ctx.Globals().Set("testAsync", ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		return ctx.NewPromise(func(resolve, reject func(*quickjs.Value)) {
			resolve(ctx.NewString("Hello Async Function!"))
		})
	}))

	ret := ctx.Eval(`
        var ret;
        testAsync().then(v => ret = v)
    `)
	defer ret.Free()
	// Check for exceptions instead of error
	if ret.IsException() {
		err := ctx.Exception()
		fmt.Printf("Error: %v\n", err)
		return
	}

	// wait for promise resolve
	ctx.Loop()

	asyncRet := ctx.Eval("ret")
	defer asyncRet.Free()
	// Check for exceptions instead of error
	if asyncRet.IsException() {
		err := ctx.Exception()
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(asyncRet.ToString())

	// Demonstrate TypedArray functionality
	floatData := []float32{95.5, 87.2, 92.0}
	typedArray := ctx.NewFloat32Array(floatData)
	ctx.Globals().Set("floatData", typedArray)

	arrayResult := ctx.Eval(`floatData instanceof Float32Array`)
	defer arrayResult.Free()
	// Check for exceptions instead of error
	if arrayResult.IsException() {
		err := ctx.Exception()
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println("TypedArray:", arrayResult.ToBool())

	// Demonstrate Marshal/Unmarshal functionality with User struct
	user := User{
		ID:       123,
		Name:     "Alice",
		Email:    "alice@example.com",
		Age:      25,
		IsActive: true,
		Scores:   []float32{95.5, 87.2, 92.0},
	}

	jsVal, err := ctx.Marshal(user)
	if err != nil {
		fmt.Printf("Marshal error: %v\n", err)
		return
	}
	ctx.Globals().Set("userData", jsVal)

	marshalResult := ctx.Eval(`userData.name + " avg: " + (userData.scores.reduce((s,v) => s+v) / userData.scores.length).toFixed(1)`)
	defer marshalResult.Free()
	// Check for exceptions instead of error
	if marshalResult.IsException() {
		err := ctx.Exception()
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println("Marshal:", marshalResult.ToString())

	// Demonstrate Class Binding functionality with the same User struct
	userConstructor, _ := ctx.BindClass(&User{})
	if userConstructor.IsException() {
		defer userConstructor.Free()
		err := ctx.Exception()
		fmt.Printf("BindClass error: %v\n", err)
		return
	}

	ctx.Globals().Set("User", userConstructor)

	classResult := ctx.Eval(`
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
	// Check for exceptions instead of error
	if classResult.IsException() {
		err := ctx.Exception()
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println("Class binding:", classResult.ToString())

	// Output:
	// Hello Javascript!
	// Hello Golang!
	// Hello Async Function!
	// TypedArray: true
	// Marshal: Alice avg: 91.6
	// Class binding: 88.5
}
