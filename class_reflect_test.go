package quickjs

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// TEST STRUCTS FOR REFLECTION BINDING
// =============================================================================

// Person is a test struct for reflection binding
type Person struct {
	private   string  // Private field - not accessible
	FirstName string  `js:"firstName,omitempty"`
	LastName  string  `js:",omitempty"`
	Age       int     `json:"age,omitempty"`
	Salary    float64 `json:",omitempty"`
	IsActive  bool    `js:"isActive"`
	Secret    string  `js:"-"`   // Should be ignored
	Secret2   string  `json:"-"` // Should be ignored
}

func (p *Person) GetFullName() string {
	return fmt.Sprintf("%s %s", p.FirstName, p.LastName)
}

func (p *Person) IncrementAge(years int) int {
	p.Age += years
	return p.Age
}

func (p *Person) GetProfile() (string, int, bool) {
	return p.GetFullName(), p.Age, p.IsActive
}

func (p *Person) MultipleArgs(a int, b string, c bool) string {
	return fmt.Sprintf("%d-%s-%t", a, b, c)
}

// Private method - not accessible through Go reflection
func (p *Person) getSecret() string {
	return p.private
}

// Vehicle tests complex field types and nested structures
type Vehicle struct {
	Brand    string          `js:"brand"`
	Model    string          `json:"model"` // json tag fallback
	Year     int             // no tag - use field name
	Features map[string]bool `js:"features"`
	Colors   []string        `js:"colors"`
	Engine   *EngineSpec     `js:"engine"`
}

type EngineSpec struct {
	Type  string `js:"type"`
	Power int    `js:"power"`
}

func (v *Vehicle) GetDescription() string {
	return fmt.Sprintf("%d %s %s", v.Year, v.Brand, v.Model)
}

// FilteredMethods tests method filtering options
type FilteredMethods struct {
	Data string `js:"data"`
}

func (f *FilteredMethods) GetData() string     { return f.Data }
func (f *FilteredMethods) GetInfo() string     { return "info: " + f.Data }
func (f *FilteredMethods) SetData(data string) { f.Data = data }
func (f *FilteredMethods) ProcessData() string { return "processed: " + f.Data }

// ReflectionTestStruct for field filtering tests
type ReflectionTestStruct struct {
	Field1     string `js:"field1"`
	Field2     string `js:"field2"`
	IgnoredOne string `js:"ignoredOne"`
	IgnoredTwo string `js:"ignoredTwo"`
}

// MethodTestStruct for method argument testing
type MethodTestStruct struct {
	Value int `js:"value"`
}

func (m *MethodTestStruct) NoArgs() string {
	return "no args"
}

func (m *MethodTestStruct) OneArg(arg int) int {
	return arg * 2
}

func (m *MethodTestStruct) MultipleArgs(a int, b string, c bool) string {
	return fmt.Sprintf("%d-%s-%t", a, b, c)
}

// VoidStruct for testing void methods
type VoidStruct struct{}

func (v *VoidStruct) VoidMethod() {} // No return values

// PrivateStruct for testing private fields only
type PrivateStruct struct {
	private1 string
	private2 int
}

// EmptyStruct for testing empty structures
type EmptyStruct struct{}

// ProblematicStruct for testing marshal/unmarshal errors
type ProblematicStruct struct {
	Channel chan int `js:"channel"` // Channel cannot be marshaled
}

func (p *ProblematicStruct) GetChannel() chan int {
	return make(chan int) // Return unsupported type
}

func (p *ProblematicStruct) MultipleProblematicReturns() (chan int, func(), interface{}) {
	return make(chan int), func() {}, make(map[interface{}]interface{})
}

// EdgeCaseStruct for testing edge cases in tag parsing
type EdgeCaseStruct struct {
	Field1 string `js:",omitempty"`   // Empty name before comma
	Field2 string `json:",omitempty"` // Empty name before comma
	Field3 string `js:""`             // Empty js tag
	Field4 string `json:""`           // Empty json tag
}

// =============================================================================
// BASIC REFLECTION TESTS
// =============================================================================

func TestReflectionBasicBinding(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test basic class binding
	constructor, classID := ctx.BindClass(&Person{})
	require.False(t, constructor.IsException())
	require.NotEqual(t, uint32(0), classID)

	ctx.Globals().Set("Person", constructor)

	// Test instance creation and accessor access
	result := ctx.Eval(`
        let person = new Person();
        person.firstName = "John";
        person.lastName = "Doe";
        person.age = 30;
        person.salary = 50000.0;
        person.isActive = true;
        
        [
            typeof person,
            person.firstName,
            person.lastName,
            person.age,
            person.salary,
            person.isActive,
            typeof person.secret,  // Should be undefined (js:"-")
            typeof person.private  // Should be undefined (private field)
        ];
    `)
	defer result.Free()

	require.False(t, result.IsException())

	require.Equal(t, "object", result.GetIdx(0).ToString())
	require.Equal(t, "John", result.GetIdx(1).ToString())
	require.Equal(t, "Doe", result.GetIdx(2).ToString())
	require.Equal(t, int32(30), result.GetIdx(3).ToInt32())
	require.Equal(t, 50000.0, result.GetIdx(4).ToFloat64())
	require.True(t, result.GetIdx(5).ToBool())
	require.Equal(t, "undefined", result.GetIdx(6).ToString()) // Changed: String() → ToString()
	require.Equal(t, "undefined", result.GetIdx(7).ToString()) // Changed: String() → ToString()
}

func TestReflectionMethodCalls(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&Person{})
	require.False(t, constructor.IsException())

	ctx.Globals().Set("Person", constructor)

	// Test method calls and private method accessibility
	result := ctx.Eval(`
        let person = new Person();
        person.firstName = "Alice";
        person.lastName = "Johnson";
        person.age = 25;
        person.isActive = true;
        
        [
            person.GetFullName(),
            person.IncrementAge(5),
            person.age,
            typeof person.getSecret  // Should be undefined (private method)
        ];
    `)
	defer result.Free()

	require.False(t, result.IsException())

	require.Equal(t, "Alice Johnson", result.GetIdx(0).ToString()) // Changed: String() → ToString()
	require.Equal(t, int32(30), result.GetIdx(1).ToInt32())        // Changed: Int32() → ToInt32()
	require.Equal(t, int32(30), result.GetIdx(2).ToInt32())        // Changed: Int32() → ToInt32()
	require.Equal(t, "undefined", result.GetIdx(3).ToString())     // Changed: String() → ToString()
}

func TestReflectionMultipleReturnValues(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&Person{})
	require.False(t, constructor.IsException())

	ctx.Globals().Set("Person", constructor)

	// Test method with multiple return values
	result := ctx.Eval(`
        let person = new Person();
        person.firstName = "Bob";
        person.lastName = "Wilson";
        person.age = 35;
        person.isActive = true;
        
        let profile = person.GetProfile(); // Returns array [fullName, age, isActive]
        [profile[0], profile[1], profile[2]];
    `)
	defer result.Free()

	require.False(t, result.IsException())

	require.Equal(t, "Bob Wilson", result.GetIdx(0).ToString()) // Changed: String() → ToString()
	require.Equal(t, int32(35), result.GetIdx(1).ToInt32())     // Changed: Int32() → ToInt32()
	require.True(t, result.GetIdx(2).ToBool())                  // Function already exists, no change
}

// =============================================================================
// CONSTRUCTOR TESTS
// =============================================================================

func TestReflectionConstructorModes(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&Person{})
	require.False(t, constructor.IsException())

	ctx.Globals().Set("Person", constructor)

	testCases := []struct {
		name string
		js   string
		want []interface{} // [firstName, lastName, age, salary, isActive]
	}{
		{
			"positional_args",
			`new Person("Alice", "Smith", 28, 60000.0, true)`,
			[]interface{}{"Alice", "Smith", int32(28), 60000.0, true},
		},
		{
			"named_args",
			`new Person({firstName: "Bob", lastName: "Jones", age: 32, salary: 70000.0, isActive: false})`,
			[]interface{}{"Bob", "Jones", int32(32), 70000.0, false},
		},
		{
			"partial_args",
			`new Person("Carol", "Brown")`,
			[]interface{}{"Carol", "Brown", int32(0), 0.0, false},
		},
		{
			"empty_constructor",
			`new Person()`,
			[]interface{}{"", "", int32(0), 0.0, false},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ctx.Eval(fmt.Sprintf(`
                (function() {
                    let person = %s;
                    return [person.firstName, person.lastName, person.age, person.salary, person.isActive];
                })();
            `, tc.js))
			defer result.Free()
			require.False(t, result.IsException())

			for i, expected := range tc.want {
				switch exp := expected.(type) {
				case string:
					require.Equal(t, exp, result.GetIdx(int64(i)).ToString()) // Changed: String() → ToString()
				case int32:
					require.Equal(t, exp, result.GetIdx(int64(i)).ToInt32()) // Changed: Int32() → ToInt32()
				case float64:
					require.Equal(t, exp, result.GetIdx(int64(i)).ToFloat64()) // Changed: Float64() → ToFloat64()
				case bool:
					require.Equal(t, exp, result.GetIdx(int64(i)).ToBool()) // Function already exists, no change
				}
			}
		})
	}
}

// =============================================================================
// REFLECTION OPTIONS TESTS
// =============================================================================

func TestReflectionWithIgnoredFields(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test WithIgnoredFields option
	constructor, _ := ctx.BindClass(&ReflectionTestStruct{}, WithIgnoredFields("IgnoredOne", "IgnoredTwo"))
	require.False(t, constructor.IsException())

	ctx.Globals().Set("ReflectionTestStruct", constructor)

	result := ctx.Eval(`
        let obj = new ReflectionTestStruct();
        [
            typeof obj.field1,      // Should exist
            typeof obj.field2,      // Should exist
            typeof obj.ignoredOne,  // Should be undefined (ignored)
            typeof obj.ignoredTwo   // Should be undefined (ignored)
        ];
    `)
	defer result.Free()

	require.False(t, result.IsException())

	require.Equal(t, "string", result.GetIdx(0).ToString())    // Changed: String() → ToString()
	require.Equal(t, "string", result.GetIdx(1).ToString())    // Changed: String() → ToString()
	require.Equal(t, "undefined", result.GetIdx(2).ToString()) // Changed: String() → ToString()
	require.Equal(t, "undefined", result.GetIdx(3).ToString()) // Changed: String() → ToString()
}

func TestReflectionWithMethodPrefix(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test method prefix filtering
	builder, err := ctx.BindClassBuilder(&FilteredMethods{}, WithMethodPrefix("Get"))
	require.NoError(t, err)

	constructor, _ := builder.Build(ctx)
	require.False(t, constructor.IsException())

	ctx.Globals().Set("Filtered", constructor)

	result := ctx.Eval(`
        let obj = new Filtered();
        [
            typeof obj.GetData,      // Should exist
            typeof obj.GetInfo,      // Should exist
            typeof obj.SetData,      // Should be undefined (no Get prefix)
            typeof obj.ProcessData   // Should be undefined (no Get prefix)
        ];
    `)
	defer result.Free()

	require.False(t, result.IsException())

	require.Equal(t, "function", result.GetIdx(0).ToString())  // Changed: String() → ToString()
	require.Equal(t, "function", result.GetIdx(1).ToString())  // Changed: String() → ToString()
	require.Equal(t, "undefined", result.GetIdx(2).ToString()) // Changed: String() → ToString()
	require.Equal(t, "undefined", result.GetIdx(3).ToString()) // Changed: String() → ToString()
}

func TestReflectionWithIgnoredMethods(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test ignored methods
	builder, err := ctx.BindClassBuilder(&FilteredMethods{}, WithIgnoredMethods("GetInfo", "ProcessData"))
	require.NoError(t, err)

	constructor, _ := builder.Build(ctx)
	require.False(t, constructor.IsException())

	ctx.Globals().Set("Filtered", constructor)

	result := ctx.Eval(`
        let obj = new Filtered();
        [
            typeof obj.GetData,      // Should exist
            typeof obj.GetInfo,      // Should be undefined (ignored)
            typeof obj.SetData,      // Should exist
            typeof obj.ProcessData   // Should be undefined (ignored)
        ];
    `)
	defer result.Free()

	require.False(t, result.IsException())

	require.Equal(t, "function", result.GetIdx(0).ToString())  // Changed: String() → ToString()
	require.Equal(t, "undefined", result.GetIdx(1).ToString()) // Changed: String() → ToString()
	require.Equal(t, "function", result.GetIdx(2).ToString())  // Changed: String() → ToString()
	require.Equal(t, "undefined", result.GetIdx(3).ToString()) // Changed: String() → ToString()
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

func TestReflectionInputValidation(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test reflect.Type input with non-struct type
	t.Run("NonStructReflectType", func(t *testing.T) {
		intType := reflect.TypeOf(42)
		_, err := ctx.BindClassBuilder(intType)
		require.Error(t, err)
		require.Contains(t, err.Error(), "type must be a struct type")
	})

	// Test pointer to non-struct
	t.Run("PointerToNonStruct", func(t *testing.T) {
		intPtr := new(int)
		_, err := ctx.BindClassBuilder(intPtr)
		require.Error(t, err)
		require.Contains(t, err.Error(), "value must be a struct or pointer to struct")
	})

	// Test non-struct direct value
	t.Run("NonStructValue", func(t *testing.T) {
		_, err := ctx.BindClassBuilder(42)
		require.Error(t, err)
		require.Contains(t, err.Error(), "value must be a struct or pointer to struct")
	})

	// Test nil input
	t.Run("NilInput", func(t *testing.T) {
		_, err := ctx.BindClassBuilder(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot get type from nil value")
	})

	// Test anonymous struct types - these should fail because they have no class name
	t.Run("AnonymousStructValue", func(t *testing.T) {
		anonymousStruct := struct {
			Name string `js:"name"`
			Age  int    `js:"age"`
		}{}

		ret, _ := ctx.BindClass(anonymousStruct)
		require.True(t, ret.IsException())
		require.Contains(t, ctx.Exception().Error(), "cannot determine class name from anonymous type")
	})

	t.Run("AnonymousStructPointer", func(t *testing.T) {
		anonymousStructPtr := &struct {
			Name string `js:"name"`
			Age  int    `js:"age"`
		}{}

		_, err := ctx.BindClassBuilder(anonymousStructPtr)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot determine class name from anonymous type")
	})

	t.Run("AnonymousStructReflectType", func(t *testing.T) {
		anonymousStruct := struct {
			Name string `js:"name"`
			Age  int    `js:"age"`
		}{}

		typ := reflect.TypeOf(anonymousStruct)

		_, err := ctx.BindClassBuilder(typ)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot determine class name from anonymous type")
	})

	// Test other invalid types
	errorCases := []struct {
		name  string
		input interface{}
	}{
		{"slice_input", []string{"test"}},
		{"map_input", map[string]string{"key": "value"}},
		{"channel_input", make(chan int)},
		{"function_input", func() {}},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ctx.BindClassBuilder(tc.input)
			require.Error(t, err)
		})
	}
}

func TestReflectionMethodArgumentErrors(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&MethodTestStruct{})
	require.False(t, constructor.IsException())

	ctx.Globals().Set("MethodTest", constructor)

	// Test too many arguments
	t.Run("TooManyArguments", func(t *testing.T) {
		result := ctx.Eval(`
            (function() {
                let obj = new MethodTest();
                try {
                    obj.NoArgs("extra", "args");
                    return "should_not_reach_here";
                } catch(e) {
                    return e.toString();
                }
            })();
        `)
		defer result.Free()
		require.False(t, result.IsException())
		require.Contains(t, result.ToString(), "too many arguments") // Changed: String() → ToString()
	})

	// Test argument conversion errors
	t.Run("ArgumentConversionError", func(t *testing.T) {
		result := ctx.Eval(`
            (function() {
                let obj = new MethodTest();
                try {
                    obj.OneArg("not_a_number"); // Should fail to convert to int
                    return "should_not_reach_here";
                } catch(e) {
                    return e.toString();
                }
            })();
        `)
		defer result.Free()
		require.False(t, result.IsException())
		require.Contains(t, result.ToString(), "failed to convert argument") // Changed: String() → ToString()
	})

	// Test missing arguments (zero value filling)
	t.Run("MissingArguments", func(t *testing.T) {
		result := ctx.Eval(`
            (function() {
                let obj = new MethodTest();
                // Call MultipleArgs with only 1 argument, others should use zero values
                return obj.MultipleArgs(42); // Should be "42--false"
            })();
        `)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "42--false", result.ToString()) // Changed: String() → ToString()
	})
}

// =============================================================================
// ERROR COVERAGE TESTS - NEW TESTS TO COVER MISSING BRANCHES
// =============================================================================

func TestReflectionConstructorErrors(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&Person{})
	require.False(t, constructor.IsException())
	ctx.Globals().Set("Person", constructor)

	// Test positional argument conversion error
	t.Run("PositionalArgumentError", func(t *testing.T) {
		ret := ctx.Eval(`
            try {
                // Pass an object where an int is expected for age
                new Person("John", "Doe", {invalid: "object"}, 50000, true);
            } catch(e) {
                throw e;
            }
        `)
		defer ret.Free()
		require.True(t, ret.IsException())

		err := ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "constructor initialization failed")
	})

	// Test object argument conversion error
	t.Run("ObjectArgumentError", func(t *testing.T) {
		ret := ctx.Eval(`
            try {
                // Pass an object where an int is expected for age accessor
                new Person({
                    firstName: "John",
                    lastName: "Doe",
                    age: {invalid: "object"}
                });
            } catch(e) {
                throw e;
            }
        `)
		defer ret.Free()
		require.True(t, ret.IsException())

		err := ctx.Exception()
		require.Error(t, err)
		require.Contains(t, err.Error(), "constructor initialization failed")
	})

	// Test positional args with unexported fields - covers the continue branch
	t.Run("PositionalArgsSkipUnexportedFields", func(t *testing.T) {
		// Person struct has: FirstName, LastName, Age, Salary, IsActive (exported)
		// and private (unexported) - this should be skipped during positional initialization
		result := ctx.Eval(`
            (function() {
                // Pass 5 arguments for the 5 exported fields
                // The private field should be skipped (continue branch)
                let person = new Person("Alice", "Smith", 30, 75000.0, true);
                return [
                    person.firstName,
                    person.lastName, 
                    person.age,
                    person.salary,
                    person.isActive,
                    typeof person.private  // Should be undefined (unexported)
                ];
            })();
        `)
		defer result.Free()
		require.False(t, result.IsException())

		require.Equal(t, "Alice", result.GetIdx(0).ToString())     // Changed: String() → ToString()
		require.Equal(t, "Smith", result.GetIdx(1).ToString())     // Changed: String() → ToString()
		require.Equal(t, int32(30), result.GetIdx(2).ToInt32())    // Changed: Int32() → ToInt32()
		require.Equal(t, 75000.0, result.GetIdx(3).ToFloat64())    // Changed: Float64() → ToFloat64()
		require.True(t, result.GetIdx(4).ToBool())                 // Function already exists, no change
		require.Equal(t, "undefined", result.GetIdx(5).ToString()) // Changed: String() → ToString()
	})
}

func TestReflectionAccessorSetterErrors(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&Person{})
	require.False(t, constructor.IsException())
	ctx.Globals().Set("Person", constructor)

	// Test accessor setter conversion error
	result := ctx.Eval(`
        (function() {
            let person = new Person();
            try {
                // Try to set age to an object instead of a number
                person.age = {complex: "object", nested: true};
                return "should_not_reach";
            } catch(e) {
                return e.toString();
            }
        })();
    `)
	defer result.Free()
	require.False(t, result.IsException())
	require.Contains(t, result.ToString(), "failed to unmarshal value for field") // Changed: String() → ToString()
}

func TestReflectionMarshalErrors(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&ProblematicStruct{})
	require.False(t, constructor.IsException())
	ctx.Globals().Set("ProblematicStruct", constructor)

	// Test accessor getter marshal error
	result := ctx.Eval(`
        (function() {
            let obj = new ProblematicStruct();
            try {
                let value = obj.channel; // Try to get unsupported type
                return "should_not_reach";
            } catch(e) {
                return e.toString();
            }
        })();
    `)
	defer result.Free()
	require.False(t, result.IsException())
	require.Contains(t, result.ToString(), "failed to marshal field") // Changed: String() → ToString()

	// Test method return value marshal error
	result2 := ctx.Eval(`
        (function() {
            let obj = new ProblematicStruct();
            try {
                obj.GetChannel(); // Method returns unsupported type
                return "should_not_reach";
            } catch(e) {
                return e.toString();
            }
        })();
    `)
	defer result2.Free()
	require.False(t, result2.IsException())
	require.Contains(t, result2.ToString(), "failed to marshal return value") // Changed: String() → ToString()

	// Test multiple return values marshal error
	result3 := ctx.Eval(`
        (function() {
            let obj = new ProblematicStruct();
            try {
                obj.MultipleProblematicReturns(); // Multiple unsupported return types
                return "should_not_reach";
            } catch(e) {
                return e.toString();
            }
        })();
    `)
	defer result3.Free()
	require.False(t, result3.IsException())
	require.Contains(t, result3.ToString(), "failed to marshal return values") // Changed: String() → ToString()
}

func TestReflectionFieldTagEdgeCases(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&EdgeCaseStruct{})
	require.False(t, constructor.IsException())
	ctx.Globals().Set("EdgeCaseStruct", constructor)

	// Test accessor names with empty tag names
	result := ctx.Eval(`
        (function() {
            let obj = new EdgeCaseStruct();
            obj.field1 = "test1";  // js:",omitempty" -> should use camelCase field name
            obj.field2 = "test2";  // json:",omitempty" -> should use camelCase field name
            obj.field3 = "test3";  // js:"" -> should use camelCase field name
            obj.field4 = "test4";  // json:"" -> should use camelCase field name
            
            return [obj.field1, obj.field2, obj.field3, obj.field4];
        })();
    `)
	defer result.Free()
	require.False(t, result.IsException())

	require.Equal(t, "test1", result.GetIdx(0).ToString()) // Changed: String() → ToString()
	require.Equal(t, "test2", result.GetIdx(1).ToString()) // Changed: String() → ToString()
	require.Equal(t, "test3", result.GetIdx(2).ToString()) // Changed: String() → ToString()
	require.Equal(t, "test4", result.GetIdx(3).ToString()) // Changed: String() → ToString()
}

// =============================================================================
// COMPLEX TYPES AND EDGE CASES
// =============================================================================

func TestReflectionComplexTypes(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&Vehicle{})
	require.False(t, constructor.IsException())

	ctx.Globals().Set("Vehicle", constructor)

	// Test complex struct with nested objects, maps, and arrays
	result := ctx.Eval(`
        let car = new Vehicle({
            brand: "Toyota",
            model: "Camry",
            year: 2023,
            features: {"GPS": true, "Bluetooth": true},
            colors: ["Red", "Blue"],
            engine: {type: "V6", power: 300}
        });
        
        [
            car.brand,
            car.model,
            car.year,
            car.features.GPS,
            car.colors.length,
            car.engine.type,
            car.engine.power,
            car.GetDescription()
        ];
    `)
	defer result.Free()
	require.False(t, result.IsException())

	require.Equal(t, "Toyota", result.GetIdx(0).ToString())            // Changed: String() → ToString()
	require.Equal(t, "Camry", result.GetIdx(1).ToString())             // Changed: String() → ToString()
	require.Equal(t, int32(2023), result.GetIdx(2).ToInt32())          // Changed: Int32() → ToInt32()
	require.True(t, result.GetIdx(3).ToBool())                         // Function already exists, no change
	require.Equal(t, int32(2), result.GetIdx(4).ToInt32())             // Changed: Int32() → ToInt32()
	require.Equal(t, "V6", result.GetIdx(5).ToString())                // Changed: String() → ToString()
	require.Equal(t, int32(300), result.GetIdx(6).ToInt32())           // Changed: Int32() → ToInt32()
	require.Equal(t, "2023 Toyota Camry", result.GetIdx(7).ToString()) // Changed: String() → ToString()
}

func TestReflectionEdgeCases(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test empty struct
	t.Run("EmptyStruct", func(t *testing.T) {
		constructor, _ := ctx.BindClass(&EmptyStruct{})
		require.False(t, constructor.IsException())

		ctx.Globals().Set("EmptyStruct", constructor)

		result := ctx.Eval(`
            (function() {
                let obj = new EmptyStruct();
                return typeof obj;
            })();
        `)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "object", result.ToString()) // Changed: String() → ToString()
	})

	// Test struct with only private fields
	t.Run("OnlyPrivateFields", func(t *testing.T) {
		constructor, _ := ctx.BindClass(&PrivateStruct{})
		require.False(t, constructor.IsException())

		ctx.Globals().Set("PrivateStruct", constructor)

		result := ctx.Eval(`
            (function() {
                let obj = new PrivateStruct();
                return Object.keys(obj).length; // Should have no enumerable accessors
            })();
        `)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, int32(0), result.ToInt32()) // Changed: Int32() → ToInt32()
	})

	// Test method with zero return values
	t.Run("VoidMethod", func(t *testing.T) {
		constructor, _ := ctx.BindClass(&VoidStruct{})
		require.False(t, constructor.IsException())

		ctx.Globals().Set("VoidStruct", constructor)

		result := ctx.Eval(`
            (function() {
                let obj = new VoidStruct();
                let returnValue = obj.VoidMethod();
                return typeof returnValue; // Should be undefined
            })();
        `)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "undefined", result.ToString()) // Changed: String() → ToString()
	})

	// Test valid reflect.Type input
	t.Run("ValidReflectType", func(t *testing.T) {
		personType := reflect.TypeOf(Person{})
		constructor, _ := ctx.BindClass(personType)
		require.False(t, constructor.IsException())

		ctx.Globals().Set("PersonFromType", constructor)

		result := ctx.Eval(`
            (function() {
                let person = new PersonFromType();
                return typeof person;
            })();
        `)
		defer result.Free()
		require.False(t, result.IsException())
		require.Equal(t, "object", result.ToString()) // Changed: String() → ToString()
	})
}

func TestReflectionMethodOnNonClassInstance(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&Person{})
	require.False(t, constructor.IsException())
	ctx.Globals().Set("Person", constructor)

	// Test calling class method on wrong object type
	result := ctx.Eval(`
        (function() {
            let person = new Person();
            let fakeObj = {};
            
            try {
                // Try to call method on wrong object
                person.GetFullName.call(fakeObj);
                return "should_not_reach";
            } catch(e) {
                return e.toString();
            }
        })();
    `)
	defer result.Free()
	require.False(t, result.IsException())
	require.Contains(t, result.ToString(), "failed to get instance data") // Changed: String() → ToString()
}

func TestReflectionAccessorGetterOnNonClassInstance(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&Person{})
	require.False(t, constructor.IsException())
	ctx.Globals().Set("Person", constructor)

	// Test accessing accessor getter on wrong object type - covers createFieldGetter error branch
	result := ctx.Eval(`
        (function() {
            let person = new Person();
            let fakeObj = {};
            
            try {
                // Get the accessor descriptor and call getter on wrong object
                let desc = Object.getOwnPropertyDescriptor(Person.prototype, 'firstName');
                desc.get.call(fakeObj);
                return "should_not_reach";
            } catch(e) {
                return e.toString();
            }
        })();
    `)
	defer result.Free()
	require.False(t, result.IsException())
	require.Contains(t, result.ToString(), "failed to get instance data") // Changed: String() → ToString()
}

func TestReflectionAccessorSetterOnNonClassInstance(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _ := ctx.BindClass(&Person{})
	require.False(t, constructor.IsException())
	ctx.Globals().Set("Person", constructor)

	// Test accessing accessor setter on wrong object type - covers createFieldSetter error branch
	result := ctx.Eval(`
        (function() {
            let person = new Person();
            let fakeObj = {};
            
            try {
                // Get the accessor descriptor and call setter on wrong object
                let desc = Object.getOwnPropertyDescriptor(Person.prototype, 'firstName');
                desc.set.call(fakeObj, "test");
                return "should_not_reach";
            } catch(e) {
                return e.toString();
            }
        })();
    `)
	defer result.Free()
	require.False(t, result.IsException())
	require.Contains(t, result.ToString(), "failed to get instance data") // Changed: String() → ToString()
}
