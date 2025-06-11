package quickjs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// TEST STRUCTS FOR REFLECTION BINDING
// =============================================================================

// Person is a test struct for reflection binding
type Person struct {
	FirstName string  `js:"firstName"`
	LastName  string  `js:"lastName"`
	Age       int     `js:"age"`
	Salary    float64 `js:"salary"`
	IsActive  bool    `js:"isActive"`
	Secret    string  `js:"-"` // Should be ignored
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

// Vehicle tests complex field types
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

// FilteredMethods tests method filtering
type FilteredMethods struct {
	Data string `js:"data"`
}

func (f *FilteredMethods) GetData() string     { return f.Data }
func (f *FilteredMethods) GetInfo() string     { return "info: " + f.Data }
func (f *FilteredMethods) SetData(data string) { f.Data = data }
func (f *FilteredMethods) ProcessData() string { return "processed: " + f.Data }

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

	// Test instance creation
	result, err := ctx.Eval(`
        let person = new Person();
        typeof person;
    `)
	require.NoError(t, err)
	defer result.Free()
	require.Equal(t, "object", result.String())
}

func TestReflectionPropertyAccess(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)

	ctx.Globals().Set("Person", constructor)

	// Test property setting and getting
	result, err := ctx.Eval(`
        let person = new Person();
        person.firstName = "John";
        person.lastName = "Doe";
        person.age = 30;
        person.salary = 50000.0;
        person.isActive = true;
        
        [
            person.firstName,
            person.lastName,
            person.age,
            person.salary,
            person.isActive,
            typeof person.secret // Should be undefined (js:"-")
        ];
    `)
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "John", result.GetIdx(0).String())
	require.Equal(t, "Doe", result.GetIdx(1).String())
	require.Equal(t, int32(30), result.GetIdx(2).Int32())
	require.Equal(t, 50000.0, result.GetIdx(3).Float64())
	require.True(t, result.GetIdx(4).ToBool())
	require.Equal(t, "undefined", result.GetIdx(5).String())
}

func TestReflectionMethodCalls(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	constructor, _, err := ctx.BindClass(&Person{})
	require.NoError(t, err)

	ctx.Globals().Set("Person", constructor)

	// Test method calls
	result, err := ctx.Eval(`
        let person = new Person();
        person.firstName = "Alice";
        person.lastName = "Johnson";
        person.age = 25;
        person.isActive = true;
        
        [
            person.GetFullName(),
            person.IncrementAge(5),
            person.age
        ];
    `)
	require.NoError(t, err)
	defer result.Free()

	require.Equal(t, "Alice Johnson", result.GetIdx(0).String())
	require.Equal(t, int32(30), result.GetIdx(1).Int32())
	require.Equal(t, int32(30), result.GetIdx(2).Int32())
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
// COMPLEX TYPES TESTS
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
            Year: 2023,
            features: {"GPS": true, "Bluetooth": true},
            colors: ["Red", "Blue"],
            engine: {type: "V6", power: 300}
        });
        
        [
            car.brand,
            car.model,
            car.Year,
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

// =============================================================================
// REFLECTION OPTIONS TESTS
// =============================================================================

func TestReflectionOptions(t *testing.T) {
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

func TestReflectionIgnoredMethods(t *testing.T) {
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

func TestReflectionErrorHandling(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	errorCases := []struct {
		name  string
		input interface{}
	}{
		{"non_struct", 42},
		{"nil_input", nil},
		{"slice_input", []string{"test"}},
		{"map_input", map[string]string{"key": "value"}},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ctx.BindClassBuilder(tc.input)
			require.Error(t, err)
		})
	}
}
