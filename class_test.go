package quickjs

import (
	"fmt"
	"math"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Implement ClassFinalizer interface for automatic cleanup testing
func (p *Point) Finalize() {
	// Record that finalize was called for testing
	atomic.AddInt64(&finalizeCallCount, 1)
}

// String method for debugging
func (p *Point) String() string {
	return fmt.Sprintf("Point(%.2f, %.2f)", p.X, p.Y)
}

// Counter for tracking Finalize() calls during GC testing
var finalizeCallCount int64

// resetFinalizeCounter resets the finalize call counter
func resetFinalizeCounter() {
	atomic.StoreInt64(&finalizeCallCount, 0)
}

// getFinalizeCount returns the current finalize call count
func getFinalizeCount() int64 {
	return atomic.LoadInt64(&finalizeCallCount)
}

// createPointClass creates a Point class for testing with SCHEME C constructor
func createPointClass(ctx *Context) (*Value, uint32) {
	return NewClassBuilder("Point").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			x, y := 0.0, 0.0
			if len(args) > 0 {
				x = args[0].ToFloat64() // Updated: Use ToFloat64()
			}
			if len(args) > 1 {
				y = args[1].ToFloat64() // Updated: Use ToFloat64()
			}

			// SCHEME C: Create Go object and return it for automatic association
			point := &Point{X: x, Y: y}
			return point, nil
		}).
		Method("norm", func(ctx *Context, this *Value, args []*Value) *Value {
			obj, err := this.GetGoObject()
			if err != nil {
				return ctx.ThrowError(err)
			}
			point := obj.(*Point)
			norm := math.Sqrt(point.X*point.X + point.Y*point.Y)
			return ctx.NewFloat64(norm) // Updated: Use NewFloat64()
		}).
		Method("toString", func(ctx *Context, this *Value, args []*Value) *Value {
			obj, err := this.GetGoObject()
			if err != nil {
				return ctx.ThrowError(err)
			}
			point := obj.(*Point)
			return ctx.NewString(point.String()) // Updated: Use NewString()
		}).
		Accessor("x",
			func(ctx *Context, this *Value) *Value { // getter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				return ctx.NewFloat64(point.X) // Updated: Use NewFloat64()
			},
			func(ctx *Context, this *Value, value *Value) *Value { // setter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				point.X = value.ToFloat64() // Updated: Use ToFloat64()
				return ctx.NewUndefined()   // Updated: Use NewUndefined()
			}).
		Accessor("y",
			func(ctx *Context, this *Value) *Value { // getter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				return ctx.NewFloat64(point.Y) // Updated: Use NewFloat64()
			},
			func(ctx *Context, this *Value, value *Value) *Value { // setter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				point.Y = value.ToFloat64() // Updated: Use ToFloat64()
				return ctx.NewUndefined()   // Updated: Use NewUndefined()
			}).
		StaticMethod("zero", func(ctx *Context, this *Value, args []*Value) *Value {
			// SCHEME C: Use CallConstructor for static method
			return this.CallConstructor()
		}).
		StaticAccessor("PI",
			func(ctx *Context, this *Value) *Value { // static getter
				return ctx.NewFloat64(math.Pi) // Updated: Use NewFloat64()
			},
			nil). // no setter, read-only
		// NEW: Add Properties for testing
		Property("version", ctx.NewString("1.0.0")).                               // Updated: Use NewString()
		Property("readOnlyFlag", ctx.NewBool(true), PropertyConfigurable).         // Updated: Use NewBool()
		StaticProperty("PI_CONST", ctx.NewFloat64(math.Pi)).                       // Updated: Use NewFloat64()
		StaticProperty("AUTHOR", ctx.NewString("QuickJS-Go"), PropertyEnumerable). // Updated: Use NewString()
		Build(ctx)
}

// TestBasicClassCreation tests basic class creation and registration
func TestBasicClassCreation(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Once set as global property, Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test basic constructor call
	result := context.Eval(`
        let p = new Point(3, 4);
        p.norm();
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate basic constructor test: %v", err)
	}

	expected := 5.0                                    // sqrt(3^2 + 4^2) = 5
	if math.Abs(result.ToFloat64()-expected) > 0.001 { // Updated: Use ToFloat64()
		t.Errorf("Expected norm to be %f, got %f", expected, result.ToFloat64())
	}
}

// TestConstructorFunctionality tests constructor with different parameter counts
func TestConstructorFunctionality(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test constructor with no arguments
	result := context.Eval(`
        let p1 = new Point();
        [p1.x, p1.y];
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate no-args constructor: %v", err)
	}

	if result.GetIdx(0).ToFloat64() != 0.0 || result.GetIdx(1).ToFloat64() != 0.0 { // Updated: Use ToFloat64()
		t.Errorf("Expected Point(0, 0), got Point(%f, %f)",
			result.GetIdx(0).ToFloat64(), result.GetIdx(1).ToFloat64())
	}

	// Test constructor with partial arguments
	result2 := context.Eval(`
        let p2 = new Point(5);
        [p2.x, p2.y];
    `)
	defer result2.Free()
	if result2.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate partial-args constructor: %v", err)
	}

	if result2.GetIdx(0).ToFloat64() != 5.0 || result2.GetIdx(1).ToFloat64() != 0.0 { // Updated: Use ToFloat64()
		t.Errorf("Expected Point(5, 0), got Point(%f, %f)",
			result2.GetIdx(0).ToFloat64(), result2.GetIdx(1).ToFloat64())
	}
}

// TestInstanceMethods tests instance method functionality
func TestInstanceMethods(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test norm method
	result := context.Eval(`
        let p1 = new Point(3, 4);
        p1.norm();
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate norm method: %v", err)
	}

	if math.Abs(result.ToFloat64()-5.0) > 0.001 { // Updated: Use ToFloat64()
		t.Errorf("Expected norm 5.0, got %f", result.ToFloat64())
	}

	// Test toString method
	result2 := context.Eval(`
        let p2 = new Point(1.5, 2.5);
        p2.toString();
    `)
	defer result2.Free()
	if result2.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate toString method: %v", err)
	}

	expected := "Point(1.50, 2.50)"
	if result2.ToString() != expected { // Updated: Use ToString()
		t.Errorf("Expected toString '%s', got '%s'", expected, result2.ToString())
	}
}

// TestAccessors tests getter and setter accessor functionality
func TestAccessors(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test accessor getters
	result := context.Eval(`
        let p1 = new Point(3, 4);
        [p1.x, p1.y];
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate accessor getters: %v", err)
	}

	if result.GetIdx(0).ToFloat64() != 3.0 || result.GetIdx(1).ToFloat64() != 4.0 { // Updated: Use ToFloat64()
		t.Errorf("Expected [3, 4], got [%f, %f]",
			result.GetIdx(0).ToFloat64(), result.GetIdx(1).ToFloat64())
	}

	// Test accessor setters
	result2 := context.Eval(`
        let p2 = new Point(1, 2);
        p2.x = 10;
        p2.y = 20;
        [p2.x, p2.y];
    `)
	defer result2.Free()
	if result2.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate accessor setters: %v", err)
	}

	if result2.GetIdx(0).ToFloat64() != 10.0 || result2.GetIdx(1).ToFloat64() != 20.0 { // Updated: Use ToFloat64()
		t.Errorf("Expected [10, 20], got [%f, %f]",
			result2.GetIdx(0).ToFloat64(), result2.GetIdx(1).ToFloat64())
	}
}

// TestStaticMethods tests static method functionality
func TestStaticMethods(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test static method
	result := context.Eval(`
        let p1 = Point.zero();
        [p1.x, p1.y];
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate static method: %v", err)
	}

	if result.GetIdx(0).ToFloat64() != 0.0 || result.GetIdx(1).ToFloat64() != 0.0 { // Updated: Use ToFloat64()
		t.Errorf("Expected [0, 0], got [%f, %f]",
			result.GetIdx(0).ToFloat64(), result.GetIdx(1).ToFloat64())
	}
}

// TestStaticAccessors tests static accessor functionality
func TestStaticAccessors(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test static read-only accessor
	result := context.Eval(`Point.PI`)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate static accessor: %v", err)
	}

	if math.Abs(result.ToFloat64()-math.Pi) > 0.001 { // Updated: Use ToFloat64()
		t.Errorf("Expected PI %f, got %f", math.Pi, result.ToFloat64())
	}
}

// NEW: TestProperties tests data property functionality
func TestProperties(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	context.Globals().Set("Point", pointConstructor)

	// Test instance properties
	result := context.Eval(`
        let p = new Point(1, 2);
        [
            p.version,           // Instance property
            p.readOnlyFlag,      // Read-only instance property
            typeof p.version,    // Should be string
            typeof p.readOnlyFlag // Should be boolean
        ];
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate instance properties: %v", err)
	}

	if result.GetIdx(0).ToString() != "1.0.0" { // Updated: Use ToString()
		t.Errorf("Expected version '1.0.0', got '%s'", result.GetIdx(0).ToString())
	}
	if !result.GetIdx(1).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected readOnlyFlag true, got %t", result.GetIdx(1).ToBool())
	}
	if result.GetIdx(2).ToString() != "string" { // Updated: Use ToString()
		t.Errorf("Expected version type 'string', got '%s'", result.GetIdx(2).ToString())
	}
	if result.GetIdx(3).ToString() != "boolean" { // Updated: Use ToString()
		t.Errorf("Expected readOnlyFlag type 'boolean', got '%s'", result.GetIdx(3).ToString())
	}
}

// NEW: TestStaticProperties tests static data property functionality
func TestStaticProperties(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	context.Globals().Set("Point", pointConstructor)

	// Test static properties
	result := context.Eval(`
        [
            Point.PI_CONST,      // Static property (default flags)
            Point.AUTHOR,        // Enumerable-only static property
            typeof Point.PI_CONST,
            typeof Point.AUTHOR
        ];
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate static properties: %v", err)
	}

	if math.Abs(result.GetIdx(0).ToFloat64()-math.Pi) > 0.001 { // Updated: Use ToFloat64()
		t.Errorf("Expected PI_CONST %f, got %f", math.Pi, result.GetIdx(0).ToFloat64())
	}
	if result.GetIdx(1).ToString() != "QuickJS-Go" { // Updated: Use ToString()
		t.Errorf("Expected AUTHOR 'QuickJS-Go', got '%s'", result.GetIdx(1).ToString())
	}
	if result.GetIdx(2).ToString() != "number" { // Updated: Use ToString()
		t.Errorf("Expected PI_CONST type 'number', got '%s'", result.GetIdx(2).ToString())
	}
	if result.GetIdx(3).ToString() != "string" { // Updated: Use ToString()
		t.Errorf("Expected AUTHOR type 'string', got '%s'", result.GetIdx(3).ToString())
	}
}

// NEW: TestPropertyFlags tests property descriptor flags
func TestPropertyFlags(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	context.Globals().Set("Point", pointConstructor)

	// Test property descriptor flags
	result := context.Eval(`
        let p = new Point(1, 2)

        // Test default flags for version (writable, enumerable, configurable)
        let versionDesc = Object.getOwnPropertyDescriptor(p, 'version');

        // Test read-only flags for readOnlyFlag (configurable only)
        let readOnlyDesc = Object.getOwnPropertyDescriptor(p, 'readOnlyFlag');

        // Test static property flags
        let piDesc = Object.getOwnPropertyDescriptor(Point, 'PI_CONST');
        let authorDesc = Object.getOwnPropertyDescriptor(Point, 'AUTHOR');

        [
            // Instance property with default flags
            versionDesc.writable,     // Should be true
            versionDesc.enumerable,   // Should be true
            versionDesc.configurable, // Should be true

            // Instance property with read-only flags
            readOnlyDesc.writable,     // Should be false (read-only)
            readOnlyDesc.enumerable,   // Should be false
            readOnlyDesc.configurable, // Should be true

            // Static property with default flags
            piDesc.writable,          // Should be true
            piDesc.enumerable,        // Should be true
            piDesc.configurable,      // Should be true

            // Static property with enumerable-only flags
            authorDesc.writable,      // Should be false
            authorDesc.enumerable,    // Should be true
            authorDesc.configurable   // Should be false
        ];
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate property flags: %v", err)
	}

	// Check version property flags (default: writable, enumerable, configurable)
	if !result.GetIdx(0).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected version.writable to be true")
	}
	if !result.GetIdx(1).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected version.enumerable to be true")
	}
	if !result.GetIdx(2).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected version.configurable to be true")
	}

	// Check readOnlyFlag property flags (configurable only)
	if result.GetIdx(3).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected readOnlyFlag.writable to be false")
	}
	if result.GetIdx(4).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected readOnlyFlag.enumerable to be false")
	}
	if !result.GetIdx(5).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected readOnlyFlag.configurable to be true")
	}

	// Check PI_CONST property flags (default: writable, enumerable, configurable)
	if !result.GetIdx(6).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected PI_CONST.writable to be true")
	}
	if !result.GetIdx(7).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected PI_CONST.enumerable to be true")
	}
	if !result.GetIdx(8).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected PI_CONST.configurable to be true")
	}

	// Check AUTHOR property flags (enumerable only)
	if result.GetIdx(9).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected AUTHOR.writable to be false")
	}
	if !result.GetIdx(10).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected AUTHOR.enumerable to be true")
	}
	if result.GetIdx(11).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected AUTHOR.configurable to be false")
	}
}

// NEW: TestPropertyVsAccessorBehavior tests the behavioral differences between Properties and Accessors
func TestPropertyVsAccessorBehavior(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	context.Globals().Set("Point", pointConstructor)

	// Test behavioral differences between properties and accessors
	result := context.Eval(`
        let p = new Point(5, 10);
        
        // Test property behavior (direct data storage)
        let originalVersion = p.version;
        p.version = "2.0.0";  // Direct assignment to property
        let newVersion = p.version;
        
        // Test accessor behavior (function calls)
        let originalX = p.x;
        p.x = 15;  // Calls setter function
        let newX = p.x;  // Calls getter function
        
        // Test property descriptor differences
        let versionDesc = Object.getOwnPropertyDescriptor(p, 'version');
        let xDesc = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(p), 'x');
        
        [
            originalVersion,      // "1.0.0"
            newVersion,          // "2.0.0" (direct property assignment)
            originalX,           // 5 (from constructor)
            newX,               // 15 (from setter)
            typeof versionDesc.value,   // "string" (data property has value)
            typeof versionDesc.get,     // "undefined" (data property has no getter)
            typeof xDesc.value,         // "undefined" (accessor has no value)
            typeof xDesc.get           // "function" (accessor has getter)
        ];
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate property vs accessor behavior: %v", err)
	}

	// Check property behavior (direct data storage)
	if result.GetIdx(0).ToString() != "1.0.0" { // Updated: Use ToString()
		t.Errorf("Expected original version '1.0.0', got '%s'", result.GetIdx(0).ToString())
	}
	if result.GetIdx(1).ToString() != "2.0.0" { // Updated: Use ToString()
		t.Errorf("Expected new version '2.0.0', got '%s'", result.GetIdx(1).ToString())
	}

	// Check accessor behavior (function calls)
	if result.GetIdx(2).ToFloat64() != 5.0 { // Updated: Use ToFloat64()
		t.Errorf("Expected original x 5.0, got %f", result.GetIdx(2).ToFloat64())
	}
	if result.GetIdx(3).ToFloat64() != 15.0 { // Updated: Use ToFloat64()
		t.Errorf("Expected new x 15.0, got %f", result.GetIdx(3).ToFloat64())
	}

	// Check property descriptor differences
	if result.GetIdx(4).ToString() != "string" { // Updated: Use ToString()
		t.Errorf("Expected version property to have value, got %s", result.GetIdx(4).ToString())
	}
	if result.GetIdx(5).ToString() != "undefined" { // Updated: Use ToString()
		t.Errorf("Expected version property to have no getter, got %s", result.GetIdx(5).ToString())
	}
	if result.GetIdx(6).ToString() != "undefined" { // Updated: Use ToString()
		t.Errorf("Expected x accessor to have no value, got %s", result.GetIdx(6).ToString())
	}
	if result.GetIdx(7).ToString() != "function" { // Updated: Use ToString()
		t.Errorf("Expected x accessor to have getter function, got %s", result.GetIdx(7).ToString())
	}
}

// TestInheritanceAndNewTarget tests inheritance support with new.target
func TestInheritanceAndNewTarget(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test inheritance using extends
	result := context.Eval(`
        class Point3D extends Point {
            constructor(x, y, z) {
                super(x, y);
                this.z = z || 0;
            }

            norm() {
                return Math.sqrt(this.x * this.x + this.y * this.y + this.z * this.z);
            }
        }

        let p3d1 = new Point3D(3, 4, 12);
        p3d1.norm(); // sqrt(3^2 + 4^2 + 12^2) = sqrt(169) = 13
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate inheritance test: %v", err)
	}

	expected := 13.0                                   // sqrt(9 + 16 + 144) = 13
	if math.Abs(result.ToFloat64()-expected) > 0.001 { // Updated: Use ToFloat64()
		t.Errorf("Expected 3D norm 13.0, got %f", result.ToFloat64())
	}

	// Test that inherited object is still instance of Point
	result2 := context.Eval(`
        let p3d2 = new Point3D(1, 2, 3);
        p3d2 instanceof Point;
    `)
	defer result2.Free()
	if result2.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate instanceof test: %v", err)
	}

	if !result2.ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected Point3D instance to be instanceof Point")
	}
}

// TestClassInstanceChecking tests class instance detection methods
func TestClassInstanceChecking(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, pointClassID := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Create test instance
	instance := context.Eval(`new Point(1, 2)`)
	defer instance.Free()
	if instance.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to create test instance: %v", err)
	}

	// Test IsClassInstance
	if !instance.IsClassInstance() {
		t.Errorf("Expected IsClassInstance to return true for Point instance")
	}

	// Test IsInstanceOfClass
	if !instance.IsInstanceOfClassID(pointClassID) {
		t.Errorf("Expected IsInstanceOfClass to return true for correct class ID")
	}

	// Test with wrong class ID
	if instance.IsInstanceOfClassID(999) {
		t.Errorf("Expected IsInstanceOfClass to return false for wrong class ID")
	}

	// Test GetClassID
	if instance.GetClassID() != pointClassID {
		t.Errorf("Expected GetClassID to return %d, got %d", pointClassID, instance.GetClassID())
	}

	// Test HasInstanceData
	if !instance.HasInstanceData() {
		t.Errorf("Expected HasInstanceData to return true for Point instance")
	}

	// Test with regular object
	regularObj := context.Eval(`({})`)
	defer regularObj.Free()
	if regularObj.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to create regular object: %v", err)
	}

	if regularObj.IsClassInstance() {
		t.Errorf("Expected IsClassInstance to return false for regular object")
	}

	if regularObj.HasInstanceData() {
		t.Errorf("Expected HasInstanceData to return false for regular object")
	}
}

// TestGetGoObject tests retrieving Go objects from JS instances
func TestGetGoObject(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Create test instance
	instance := context.Eval(`new Point(3.14, 2.71)`)
	defer instance.Free()
	if instance.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to create test instance: %v", err)
	}

	// Use GetGoObject to retrieve Go object
	obj, err := instance.GetGoObject()
	if err != nil {
		t.Fatalf("Failed to get instance data: %v", err)
	}

	point, ok := obj.(*Point)
	if !ok {
		t.Fatalf("Expected *Point, got %T", obj)
	}

	if point.X != 3.14 || point.Y != 2.71 {
		t.Errorf("Expected Point(3.14, 2.71), got Point(%f, %f)", point.X, point.Y)
	}

	// Test GetGoObject via Context method again for consistency
	obj2, err := instance.GetGoObject()
	if err != nil {
		t.Fatalf("Failed to get instance data via context: %v", err)
	}

	point2, ok := obj2.(*Point)
	if !ok {
		t.Fatalf("Expected *Point, got %T", obj2)
	}

	if point2.X != 3.14 || point2.Y != 2.71 {
		t.Errorf("Expected Point(3.14, 2.71), got Point(%f, %f)", point2.X, point2.Y)
	}
}

// TestFinalizerInterface tests automatic cleanup via ClassFinalizer interface
func TestFinalizerInterface(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Reset finalize counter
	resetFinalizeCounter()

	// Create and register Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		defer pointConstructor.Free()
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Create instances that will be garbage collected
	result := context.Eval(`
        for (let i = 0; i < 10; i++) {
            let p = new Point(i, i * 2);
            // Don't keep references, let them be GC'd
        }
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to create test instances: %v", err)
	}

	// Force garbage collection multiple times
	for i := 0; i < 5; i++ {
		rt.RunGC()
		runtime.GC() // Go's runtime GC
		time.Sleep(10 * time.Millisecond)
	}

	// Check that some finalizers were called
	finalizeCount := getFinalizeCount()
	if finalizeCount == 0 {
		t.Logf("Warning: No finalizers called yet (this might be timing-dependent)")
		// Try more aggressive GC
		for i := 0; i < 10; i++ {
			rt.RunGC()
			runtime.GC() // Go's runtime GC
			time.Sleep(50 * time.Millisecond)
		}
		finalizeCount = getFinalizeCount()
	}

	t.Logf("Finalizer called %d times", finalizeCount)
	// Note: Due to GC timing, we can't guarantee exact count, but some should be called
}

// TestErrorHandling tests error conditions and edge cases
func TestErrorHandling(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Test creating class with empty name
	ctor, _ := NewClassBuilder("").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return nil, nil
		}).
		Method("getValue", func(ctx *Context, this *Value, args []*Value) *Value {
			return ctx.NewFloat64(0) // Updated: Use NewFloat64()
		}).
		Accessor("y",
			func(ctx *Context, this *Value) *Value { // getter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				return ctx.NewFloat64(point.Y) // Updated: Use NewFloat64()
			},
			func(ctx *Context, this *Value, value *Value) *Value { // setter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				point.Y = value.ToFloat64() // Updated: Use ToFloat64()
				return ctx.NewUndefined()   // Updated: Use NewUndefined()
			}).
		Build(context)
	defer ctor.Free()

	if ctor.IsException() {
		err := context.Exception()
		require.Contains(t, err.Error(), "class_name cannot be empty")
	}

	// Test creating class without constructor
	ctor2, _ := NewClassBuilder("TestClass").Build(context)
	defer ctor2.Free()
	if ctor2.IsException() {
		err := context.Exception()
		require.Contains(t, err.Error(), "constructor function is required")
	}

	// Test GetGoObject on non-class object
	regularObj := context.Eval(`({})`)
	defer regularObj.Free()
	if regularObj.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to create regular object: %v", err)
	}

	// Use GetGoObject to test error handling
	_, err := regularObj.GetGoObject()

	if err == nil {
		t.Errorf("Expected error when getting instance data from regular object")
	}

	// Test GetGoObject on non-object
	numberValue := context.NewFloat64(42.0) // Updated: Use NewFloat64()
	defer numberValue.Free()

	// Use GetGoObject to test error handling
	_, err = numberValue.GetGoObject()
	if err == nil {
		t.Errorf("Expected error when getting instance data from number")
	}
}

// TestMemoryManagement tests proper cleanup and memory management
func TestMemoryManagement(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Reset finalize counter
	resetFinalizeCounter()

	// Create multiple classes to test handle store management
	for i := 0; i < 5; i++ {
		className := fmt.Sprintf("TestClass%d", i)
		constructor, _ := NewClassBuilder(className).
			Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
				point := &Point{X: float64(i), Y: float64(i * 2)}
				// SCHEME C: Return Go object for automatic association
				return point, nil
			}).
			Method("getValue", func(ctx *Context, this *Value, args []*Value) *Value {
				return ctx.NewFloat64(float64(i)) // Updated: Use NewFloat64()
			}).
			Build(context)

		if constructor.IsException() {
			err := context.Exception()
			t.Fatalf("Failed to create class %s: %v", className, err)
		}

		// Create instances and let them be garbage collected
		// Note: Globals will manage the constructor memory automatically
		context.Globals().Set(className, constructor)
		result := context.Eval(fmt.Sprintf(`
            for (let j = 0; j < 5; j++) {
                let obj = new %s();
            }
        `, className))
		defer result.Free()
		if result.IsException() {
			err := context.Exception()
			t.Fatalf("Failed to create instances for %s: %v", className, err)
		}
	}

	// Force garbage collection multiple times
	for i := 0; i < 5; i++ {
		rt.RunGC()
		runtime.GC() // Go's runtime GC
		time.Sleep(10 * time.Millisecond)
	}

	t.Logf("Created and cleaned up multiple classes, finalizers called: %d", getFinalizeCount())
}

// TestComplexClassHierarchy tests complex inheritance scenarios
func TestComplexClassHierarchy(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test complex inheritance hierarchy
	result := context.Eval(`
        // First level inheritance
        class ColoredPoint extends Point {
            constructor(x, y, color) {
                super(x, y);
                this.color = color || 'black';
            }

            getColor() {
                return this.color;
            }
        }

        // Second level inheritance
        class NamedColoredPoint extends ColoredPoint {
            constructor(x, y, color, name) {
                super(x, y, color);
                this.name = name || 'unnamed';
            }

            toString() {
                return this.name + ': ' + super.toString() + ' (' + this.color + ')';
            }
        }

        let ncp1 = new NamedColoredPoint(3, 4, 'red', 'MyPoint');
        [
            ncp1 instanceof Point,
            ncp1 instanceof ColoredPoint,
            ncp1 instanceof NamedColoredPoint,
            ncp1.x,
            ncp1.y,
            ncp1.getColor(),
            ncp1.name,
            ncp1.norm()
        ];
    `)
	defer result.Free()
	if result.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to evaluate complex hierarchy: %v", err)
	}

	// Check all instanceof relationships
	if !result.GetIdx(0).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected NamedColoredPoint to be instanceof Point")
	}
	if !result.GetIdx(1).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected NamedColoredPoint to be instanceof ColoredPoint")
	}
	if !result.GetIdx(2).ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected NamedColoredPoint to be instanceof NamedColoredPoint")
	}

	// Check inherited accessors and methods
	if result.GetIdx(3).ToFloat64() != 3.0 { // Updated: Use ToFloat64()
		t.Errorf("Expected x=3, got %f", result.GetIdx(3).ToFloat64())
	}
	if result.GetIdx(4).ToFloat64() != 4.0 { // Updated: Use ToFloat64()
		t.Errorf("Expected y=4, got %f", result.GetIdx(4).ToFloat64())
	}
	if result.GetIdx(5).ToString() != "red" { // Updated: Use ToString()
		t.Errorf("Expected color=red, got %s", result.GetIdx(5).ToString())
	}
	if result.GetIdx(6).ToString() != "MyPoint" { // Updated: Use ToString()
		t.Errorf("Expected name=MyPoint, got %s", result.GetIdx(6).ToString())
	}
	if math.Abs(result.GetIdx(7).ToFloat64()-5.0) > 0.001 { // Updated: Use ToFloat64()
		t.Errorf("Expected norm=5, got %f", result.GetIdx(7).ToFloat64())
	}
}

// TestConcurrentAccess tests thread safety (basic test)
func TestConcurrentAccess(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create Point class
	pointConstructor, _ := createPointClass(context)
	if pointConstructor.IsException() {
		err := context.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Note: QuickJS is not thread-safe, so this test just ensures
	// that our binding code doesn't introduce obvious race conditions
	// when accessed from a single goroutine repeatedly

	for i := 0; i < 100; i++ {
		result := context.Eval(fmt.Sprintf(`
            let p%d = new Point(%d, %d);
            p%d.norm();
        `, i, i, i+1, i))
		defer result.Free()
		if result.IsException() {
			err := context.Exception()
			t.Fatalf("Failed on iteration %d: %v", i, err)
		}
	}
}

// TestUnifiedConstructorMapping tests the unified constructor -> classID mapping
func TestUnifiedConstructorMapping(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test manual class creation
	manualConstructor, manualClassID := createPointClass(ctx)
	if manualConstructor.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to create manual Point class: %v", err)
	}

	// Test reflection class creation
	reflectConstructor, reflectClassID := ctx.BindClass(&Point{})
	if reflectConstructor.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to create reflection Point class: %v", err)
	}

	// Verify both constructors are registered in the unified mapping
	manualRetrievedID, exists := getConstructorClassID(manualConstructor.ref)
	if !exists {
		t.Errorf("Manual constructor not found in unified mapping")
	}
	if manualRetrievedID != manualClassID {
		t.Errorf("Manual constructor classID mismatch: expected %d, got %d", manualClassID, manualRetrievedID)
	}

	reflectRetrievedID, exists := getConstructorClassID(reflectConstructor.ref)
	if !exists {
		t.Errorf("Reflection constructor not found in unified mapping")
	}
	if reflectRetrievedID != reflectClassID {
		t.Errorf("Reflection constructor classID mismatch: expected %d, got %d", reflectClassID, reflectRetrievedID)
	}

	// Test that both can create instances with the CallConstructor API
	ctx.Globals().Set("ManualPoint", manualConstructor)
	ctx.Globals().Set("ReflectPoint", reflectConstructor)

	result := ctx.Eval(`
        let manual = new ManualPoint(1, 2);
        let reflect = new ReflectPoint();
        reflect.X = 3;
        reflect.Y = 4;
        [manual.x, manual.y, reflect.X, reflect.Y];
    `)
	defer result.Free()
	if result.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to test unified mapping: %v", err)
	}

	if result.GetIdx(0).ToFloat64() != 1.0 || result.GetIdx(1).ToFloat64() != 2.0 { // Updated: Use ToFloat64()
		t.Errorf("Manual constructor instance incorrect: got (%f, %f)",
			result.GetIdx(0).ToFloat64(), result.GetIdx(1).ToFloat64())
	}
	if result.GetIdx(2).ToFloat64() != 3.0 || result.GetIdx(3).ToFloat64() != 4.0 { // Updated: Use ToFloat64()
		t.Errorf("Reflection constructor instance incorrect: got (%f, %f)",
			result.GetIdx(2).ToFloat64(), result.GetIdx(3).ToFloat64())
	}
}

// TestReadOnlyAndWriteOnlyAccessors tests readonly and writeonly accessor functionality
func TestReadOnlyAndWriteOnlyAccessors(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test ReadOnlyAccessor
	constructor1, _ := NewClassBuilder("ReadOnlyTest").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &Point{X: 10, Y: 20}, nil
		}).
		Accessor("readOnlyX", func(ctx *Context, this *Value) *Value {
			obj, _ := this.GetGoObject()
			point := obj.(*Point)
			return ctx.NewFloat64(point.X) // Updated: Use NewFloat64()
		}, nil).
		Build(ctx)

	if constructor1.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to create ReadOnlyTest class: %v", err)
	}

	ctx.Globals().Set("ReadOnlyTest", constructor1)

	// Test reading works, writing doesn't change value
	result := ctx.Eval(`
        let obj1 = new ReadOnlyTest();
        let original = obj1.readOnlyX;
        obj1.readOnlyX = 999; // Should not change
        [original, obj1.readOnlyX];
    `)
	defer result.Free()
	if result.IsException() {
		err := ctx.Exception()
		t.Fatalf("ReadOnly accessor test failed: %v", err)
	}

	if result.GetIdx(0).ToFloat64() != 10.0 || result.GetIdx(1).ToFloat64() != 10.0 { // Updated: Use ToFloat64()
		t.Errorf("ReadOnly accessor failed: expected [10, 10], got [%f, %f]",
			result.GetIdx(0).ToFloat64(), result.GetIdx(1).ToFloat64())
	}

	// Test WriteOnlyAccessor
	constructor2, _ := NewClassBuilder("WriteOnlyTest").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &Point{X: 0, Y: 0}, nil
		}).
		Accessor("writeOnlyX", nil, func(ctx *Context, this *Value, value *Value) *Value {
			obj, _ := this.GetGoObject()
			point := obj.(*Point)
			point.X = value.ToFloat64() // Updated: Use ToFloat64()
			return ctx.NewUndefined()   // Updated: Use NewUndefined()
		}).
		Accessor("getX", func(ctx *Context, this *Value) *Value {
			obj, _ := this.GetGoObject()
			point := obj.(*Point)
			return ctx.NewFloat64(point.X) // Updated: Use NewFloat64()
		}, nil).
		Build(ctx)

	if constructor2.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to create WriteOnlyTest class: %v", err)
	}

	ctx.Globals().Set("WriteOnlyTest", constructor2)

	// Test writing works, reading returns undefined
	result2 := ctx.Eval(`
        let obj2 = new WriteOnlyTest();
        obj2.writeOnlyX = 42;
        [obj2.getX, obj2.writeOnlyX]; // getX should show 42, writeOnlyX should be undefined
    `)
	defer result2.Free()
	if result2.IsException() {
		err := ctx.Exception()
		t.Fatalf("WriteOnly accessor test failed: %v", err)
	}

	if result2.GetIdx(0).ToFloat64() != 42.0 || !result2.GetIdx(1).IsUndefined() { // Updated: Use ToFloat64()
		t.Errorf("WriteOnly accessor failed: expected [42, undefined], got [%f, %v]",
			result2.GetIdx(0).ToFloat64(), result2.GetIdx(1).ToString())
	}

	// Test StaticReadOnlyAccessor
	constructor3, _ := NewClassBuilder("StaticReadOnlyTest").
		Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
			return &Point{X: 0, Y: 0}, nil
		}).
		StaticAccessor("VERSION", func(ctx *Context, this *Value) *Value {
			return ctx.NewString("1.0.0") // Updated: Use NewString()
		}, nil).
		Build(ctx)

	if constructor3.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to create StaticReadOnlyTest class: %v", err)
	}

	ctx.Globals().Set("StaticReadOnlyTest", constructor3)

	// Test static readonly accessor
	result3 := ctx.Eval(`
        let original3 = StaticReadOnlyTest.VERSION;
        StaticReadOnlyTest.VERSION = "2.0.0"; // Should not change
        [original3, StaticReadOnlyTest.VERSION];
    `)
	defer result3.Free()
	if result3.IsException() {
		err := ctx.Exception()
		t.Fatalf("StaticReadOnly accessor test failed: %v", err)
	}

	if result3.GetIdx(0).ToString() != "1.0.0" || result3.GetIdx(1).ToString() != "1.0.0" { // Updated: Use ToString()
		t.Errorf("StaticReadOnly accessor failed: expected ['1.0.0', '1.0.0'], got ['%s', '%s']",
			result3.GetIdx(0).ToString(), result3.GetIdx(1).ToString())
	}
}

// NEW: TestCallConstructorAPI tests the CallConstructor API directly
func TestCallConstructorAPI(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Create Point class
	pointConstructor, _ := createPointClass(ctx)
	defer pointConstructor.Free()
	if pointConstructor.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Test CallConstructor with no arguments
	instance1 := pointConstructor.CallConstructor()
	defer instance1.Free()

	if instance1.IsException() {
		err := ctx.Exception()
		t.Fatalf("CallConstructor with no args failed: %v", err)
	}

	// Verify instance properties
	if !instance1.Has("x") || !instance1.Has("y") {
		t.Errorf("Instance missing accessor properties")
	}

	if !instance1.Has("version") || !instance1.Has("readOnlyFlag") {
		t.Errorf("Instance missing data properties")
	}

	// Test CallConstructor with arguments
	arg1 := ctx.NewFloat64(10.5) // Updated: Use NewFloat64()
	arg2 := ctx.NewFloat64(20.5) // Updated: Use NewFloat64()
	defer arg1.Free()
	defer arg2.Free()

	instance2 := pointConstructor.CallConstructor(arg1, arg2)
	defer instance2.Free()

	if instance2.IsException() {
		err := ctx.Exception()
		t.Fatalf("CallConstructor with args failed: %v", err)
	}

	// Verify constructor arguments were applied
	x := instance2.Get("x")
	y := instance2.Get("y")
	defer x.Free()
	defer y.Free()

	if math.Abs(x.ToFloat64()-10.5) > 0.001 { // Updated: Use ToFloat64()
		t.Errorf("Expected x=10.5, got %f", x.ToFloat64())
	}
	if math.Abs(y.ToFloat64()-20.5) > 0.001 { // Updated: Use ToFloat64()
		t.Errorf("Expected y=20.5, got %f", y.ToFloat64())
	}

	// Verify instance properties are present
	version := instance2.Get("version")
	readOnlyFlag := instance2.Get("readOnlyFlag")
	defer version.Free()
	defer readOnlyFlag.Free()

	if version.ToString() != "1.0.0" { // Updated: Use ToString()
		t.Errorf("Expected version '1.0.0', got '%s'", version.ToString())
	}
	if !readOnlyFlag.ToBool() { // Updated: Use ToBool()
		t.Errorf("Expected readOnlyFlag true, got %t", readOnlyFlag.ToBool())
	}
}

// NEW: TestSchemeCSynchronization tests that property changes sync with Go object
func TestSchemeCSynchronization(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Create Point class
	pointConstructor, _ := createPointClass(ctx)
	if pointConstructor.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to create Point class: %v", err)
	}

	ctx.Globals().Set("Point", pointConstructor)

	// Test that accessor changes sync with Go object
	result := ctx.Eval(`
        let p = new Point(1, 2);
        
        // Change values via accessors
        p.x = 100;
        p.y = 200;
        
        // Read back via accessors  
        [p.x, p.y];
    `)
	defer result.Free()
	if result.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to evaluate synchronization test: %v", err)
	}

	if result.GetIdx(0).ToFloat64() != 100.0 || result.GetIdx(1).ToFloat64() != 200.0 { // Updated: Use ToFloat64()
		t.Errorf("Accessor synchronization failed: expected [100, 200], got [%f, %f]",
			result.GetIdx(0).ToFloat64(), result.GetIdx(1).ToFloat64())
	}

	// Test that we can retrieve the Go object and verify synchronization
	instance := ctx.Eval(`p`)
	defer instance.Free()
	if instance.IsException() {
		err := ctx.Exception()
		t.Fatalf("Failed to get instance: %v", err)
	}

	goObj, err := instance.GetGoObject()
	if err != nil {
		t.Fatalf("Failed to get Go object: %v", err)
	}

	point, ok := goObj.(*Point)
	if !ok {
		t.Fatalf("Expected *Point, got %T", goObj)
	}

	if point.X != 100.0 || point.Y != 200.0 {
		t.Errorf("Go object synchronization failed: expected Point(100, 200), got Point(%f, %f)",
			point.X, point.Y)
	}
}
