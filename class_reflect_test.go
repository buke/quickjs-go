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
	constructor, classID, err := ctx.BindClass(&Person{})
	require.NoError(t, err)
	require.NotEqual(t, uint32(0), classID)

	ctx.Globals().Set("Person", constructor)

	// Test instance creation and accessor access
	result, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "object", result.GetIdx(0).String())
	require.Equal(t, "John", result.GetIdx(1).String())
	require.Equal(t, "Doe", result.GetIdx(2).String())
	require.Equal(t, int32(30), result.GetIdx(3).Int32())
	require.Equal(t, 50000.0, result.GetIdx(4).Float64())
	require.True(t, result.GetIdx(5).ToBool())
	require.Equal(t, "undefined", result.GetIdx(6).String()) // js:"-" tag
	require.Equal(t, "undefined", result.GetIdx(7).String()) // private field
}

func TestReflectionMethodCalls(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)

	ctx.Globals().Set("Person", constructor)

	// Test method calls and private method accessibility
	result, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "Alice Johnson", result.GetIdx(0).String())
	require.Equal(t, int32(30), result.GetIdx(1).Int32())
	require.Equal(t, int32(30), result.GetIdx(2).Int32())
	require.Equal(t, "undefined", result.GetIdx(3).String()) // private method not accessible
}

func TestReflectionMultipleReturnValues(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)

	ctx.Globals().Set("Person", constructor)

	// Test method with multiple return values
	result, err := ctx.Eval(`
        let person = new Person();
        person.firstName = "Bob";
        person.lastName = "Wilson";
        person.age = 35;
        person.isActive = true;
        
        let profile = person.GetProfile(); // Returns array [fullName, age, isActive]
        [profile[0], profile[1], profile[2]];
    `)
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "Bob Wilson", result.GetIdx(0).String())
	require.Equal(t, int32(35), result.GetIdx(1).Int32())
	require.True(t, result.GetIdx(2).ToBool())
}

// =============================================================================
// CONSTRUCTOR TESTS
// =============================================================================

func TestReflectionConstructorModes(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)

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
			result, err := ctx.Eval(fmt.Sprintf(`
                (function() {
                    let person = %s;
                    return [person.firstName, person.lastName, person.age, person.salary, person.isActive];
                })();
            `, tc.js))
			require.NoError(t, err)
			defer result.Free()

			for i, expected := range tc.want {
				switch exp := expected.(type) {
				case string:
					require.Equal(t, exp, result.GetIdx(int64(i)).String())
				case int32:
					require.Equal(t, exp, result.GetIdx(int64(i)).Int32())
				case float64:
					require.Equal(t, exp, result.GetIdx(int64(i)).Float64())
				case bool:
					require.Equal(t, exp, result.GetIdx(int64(i)).ToBool())
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
	constructor, _, err := ctx.BindClass(&ReflectionTestStruct{}, WithIgnoredFields("IgnoredOne", "IgnoredTwo"))
	require.NoError(t, err)

	ctx.Globals().Set("ReflectionTestStruct", constructor)

	result, err := ctx.Eval(`
        let obj = new ReflectionTestStruct();
        [
            typeof obj.field1,      // Should exist
            typeof obj.field2,      // Should exist
            typeof obj.ignoredOne,  // Should be undefined (ignored)
            typeof obj.ignoredTwo   // Should be undefined (ignored)
        ];
    `)
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "string", result.GetIdx(0).String())
	require.Equal(t, "string", result.GetIdx(1).String())
	require.Equal(t, "undefined", result.GetIdx(2).String()) // Ignored field
	require.Equal(t, "undefined", result.GetIdx(3).String()) // Ignored field
}

func TestReflectionWithMethodPrefix(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test method prefix filtering
	builder, err := ctx.BindClassBuilder(&FilteredMethods{}, WithMethodPrefix("Get"))
	require.NoError(t, err)

	constructor, _, err := builder.Build(ctx)
	require.NoError(t, err)

	ctx.Globals().Set("Filtered", constructor)

	result, err := ctx.Eval(`
        let obj = new Filtered();
        [
            typeof obj.GetData,      // Should exist
            typeof obj.GetInfo,      // Should exist
            typeof obj.SetData,      // Should be undefined (no Get prefix)
            typeof obj.ProcessData   // Should be undefined (no Get prefix)
        ];
    `)
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "function", result.GetIdx(0).String())
	require.Equal(t, "function", result.GetIdx(1).String())
	require.Equal(t, "undefined", result.GetIdx(2).String())
	require.Equal(t, "undefined", result.GetIdx(3).String())
}

func TestReflectionWithIgnoredMethods(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test ignored methods
	builder, err := ctx.BindClassBuilder(&FilteredMethods{}, WithIgnoredMethods("GetInfo", "ProcessData"))
	require.NoError(t, err)

	constructor, _, err := builder.Build(ctx)
	require.NoError(t, err)

	ctx.Globals().Set("Filtered", constructor)

	result, err := ctx.Eval(`
        let obj = new Filtered();
        [
            typeof obj.GetData,      // Should exist
            typeof obj.GetInfo,      // Should be undefined (ignored)
            typeof obj.SetData,      // Should exist
            typeof obj.ProcessData   // Should be undefined (ignored)
        ];
    `)
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "function", result.GetIdx(0).String())
	require.Equal(t, "undefined", result.GetIdx(1).String())
	require.Equal(t, "function", result.GetIdx(2).String())
	require.Equal(t, "undefined", result.GetIdx(3).String())
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

		_, _, err := ctx.BindClass(anonymousStruct)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot determine class name from anonymous type")
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

	constructor, _, err := ctx.BindClass(&MethodTestStruct{})
	require.NoError(t, err)

	ctx.Globals().Set("MethodTest", constructor)

	// Test too many arguments
	t.Run("TooManyArguments", func(t *testing.T) {
		result, err := ctx.Eval(`
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
		require.NoError(t, err)
		defer result.Free()
		require.Contains(t, result.String(), "too many arguments")
	})

	// Test argument conversion errors
	t.Run("ArgumentConversionError", func(t *testing.T) {
		result, err := ctx.Eval(`
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
		require.NoError(t, err)
		defer result.Free()
		require.Contains(t, result.String(), "failed to convert argument")
	})

	// Test missing arguments (zero value filling)
	t.Run("MissingArguments", func(t *testing.T) {
		result, err := ctx.Eval(`
            (function() {
                let obj = new MethodTest();
                // Call MultipleArgs with only 1 argument, others should use zero values
                return obj.MultipleArgs(42); // Should be "42--false"
            })();
        `)
		require.NoError(t, err)
		defer result.Free()
		require.Equal(t, "42--false", result.String())
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

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)
	ctx.Globals().Set("Person", constructor)

	// Test positional argument conversion error
	t.Run("PositionalArgumentError", func(t *testing.T) {
		_, err := ctx.Eval(`
            try {
                // Pass an object where an int is expected for age
                new Person("John", "Doe", {invalid: "object"}, 50000, true);
            } catch(e) {
                throw e;
            }
        `)
		require.Error(t, err)
		require.Contains(t, err.Error(), "constructor initialization failed")
	})

	// Test object argument conversion error
	t.Run("ObjectArgumentError", func(t *testing.T) {
		_, err := ctx.Eval(`
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
		require.Error(t, err)
		require.Contains(t, err.Error(), "constructor initialization failed")
	})

	// Test positional args with unexported fields - covers the continue branch
	t.Run("PositionalArgsSkipUnexportedFields", func(t *testing.T) {
		// Person struct has: FirstName, LastName, Age, Salary, IsActive (exported)
		// and private (unexported) - this should be skipped during positional initialization
		result, err := ctx.Eval(`
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
		require.NoError(t, err)
		defer result.Free()

		require.Equal(t, "Alice", result.GetIdx(0).String())
		require.Equal(t, "Smith", result.GetIdx(1).String())
		require.Equal(t, int32(30), result.GetIdx(2).Int32())
		require.Equal(t, 75000.0, result.GetIdx(3).Float64())
		require.True(t, result.GetIdx(4).ToBool())
		require.Equal(t, "undefined", result.GetIdx(5).String()) // private field not accessible
	})
}

func TestReflectionAccessorSetterErrors(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)
	ctx.Globals().Set("Person", constructor)

	// Test accessor setter conversion error
	result, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result.Free()
	require.Contains(t, result.String(), "failed to unmarshal value for field")
}

func TestReflectionMarshalErrors(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&ProblematicStruct{})
	require.NoError(t, err)
	ctx.Globals().Set("ProblematicStruct", constructor)

	// Test accessor getter marshal error
	result, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result.Free()
	require.Contains(t, result.String(), "failed to marshal field")

	// Test method return value marshal error
	result2, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result2.Free()
	require.Contains(t, result2.String(), "failed to marshal return value")

	// Test multiple return values marshal error
	result3, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result3.Free()
	require.Contains(t, result3.String(), "failed to marshal return values")
}

func TestReflectionFieldTagEdgeCases(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&EdgeCaseStruct{})
	require.NoError(t, err)
	ctx.Globals().Set("EdgeCaseStruct", constructor)

	// Test accessor names with empty tag names
	result, err := ctx.Eval(`
        (function() {
            let obj = new EdgeCaseStruct();
            obj.field1 = "test1";  // js:",omitempty" -> should use camelCase field name
            obj.field2 = "test2";  // json:",omitempty" -> should use camelCase field name
            obj.field3 = "test3";  // js:"" -> should use camelCase field name
            obj.field4 = "test4";  // json:"" -> should use camelCase field name
            
            return [obj.field1, obj.field2, obj.field3, obj.field4];
        })();
    `)
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "test1", result.GetIdx(0).String())
	require.Equal(t, "test2", result.GetIdx(1).String())
	require.Equal(t, "test3", result.GetIdx(2).String())
	require.Equal(t, "test4", result.GetIdx(3).String())
}

// =============================================================================
// COMPLEX TYPES AND EDGE CASES
// =============================================================================

func TestReflectionComplexTypes(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Vehicle{})
	require.NoError(t, err)

	ctx.Globals().Set("Vehicle", constructor)

	// Test complex struct with nested objects, maps, and arrays
	result, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "Toyota", result.GetIdx(0).String())
	require.Equal(t, "Camry", result.GetIdx(1).String())
	require.Equal(t, int32(2023), result.GetIdx(2).Int32())
	require.True(t, result.GetIdx(3).ToBool())
	require.Equal(t, int32(2), result.GetIdx(4).Int32())
	require.Equal(t, "V6", result.GetIdx(5).String())
	require.Equal(t, int32(300), result.GetIdx(6).Int32())
	require.Equal(t, "2023 Toyota Camry", result.GetIdx(7).String())
}

func TestReflectionEdgeCases(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test empty struct
	t.Run("EmptyStruct", func(t *testing.T) {
		constructor, _, err := ctx.BindClass(&EmptyStruct{})
		require.NoError(t, err)

		ctx.Globals().Set("EmptyStruct", constructor)

		result, err := ctx.Eval(`
            (function() {
                let obj = new EmptyStruct();
                return typeof obj;
            })();
        `)
		require.NoError(t, err)
		defer result.Free()
		require.Equal(t, "object", result.String())
	})

	// Test struct with only private fields
	t.Run("OnlyPrivateFields", func(t *testing.T) {
		constructor, _, err := ctx.BindClass(&PrivateStruct{})
		require.NoError(t, err)

		ctx.Globals().Set("PrivateStruct", constructor)

		result, err := ctx.Eval(`
            (function() {
                let obj = new PrivateStruct();
                return Object.keys(obj).length; // Should have no enumerable accessors
            })();
        `)
		require.NoError(t, err)
		defer result.Free()
		require.Equal(t, int32(0), result.Int32())
	})

	// Test method with zero return values
	t.Run("VoidMethod", func(t *testing.T) {
		constructor, _, err := ctx.BindClass(&VoidStruct{})
		require.NoError(t, err)

		ctx.Globals().Set("VoidStruct", constructor)

		result, err := ctx.Eval(`
            (function() {
                let obj = new VoidStruct();
                let returnValue = obj.VoidMethod();
                return typeof returnValue; // Should be undefined
            })();
        `)
		require.NoError(t, err)
		defer result.Free()
		require.Equal(t, "undefined", result.String())
	})

	// Test valid reflect.Type input
	t.Run("ValidReflectType", func(t *testing.T) {
		personType := reflect.TypeOf(Person{})
		constructor, _, err := ctx.BindClass(personType)
		require.NoError(t, err)

		ctx.Globals().Set("PersonFromType", constructor)

		result, err := ctx.Eval(`
            (function() {
                let person = new PersonFromType();
                return typeof person;
            })();
        `)
		require.NoError(t, err)
		defer result.Free()
		require.Equal(t, "object", result.String())
	})
}

func TestReflectionMethodOnNonClassInstance(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)
	ctx.Globals().Set("Person", constructor)

	// Test calling class method on wrong object type
	result, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result.Free()
	require.Contains(t, result.String(), "failed to get instance data")
}

func TestReflectionAccessorGetterOnNonClassInstance(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)
	ctx.Globals().Set("Person", constructor)

	// Test accessing accessor getter on wrong object type - covers createFieldGetter error branch
	result, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result.Free()
	require.Contains(t, result.String(), "failed to get instance data")
}

func TestReflectionAccessorSetterOnNonClassInstance(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)
	ctx.Globals().Set("Person", constructor)

	// Test accessing accessor setter on wrong object type - covers createFieldSetter error branch
	result, err := ctx.Eval(`
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
	require.NoError(t, err)
	defer result.Free()
	require.Contains(t, result.String(), "failed to get instance data")
}
