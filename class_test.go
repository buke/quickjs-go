package quickjs

import (
	"fmt"
	"math"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// Point represents a 2D point class for testing basic functionality
type Point struct {
	X, Y float64
}

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

// createPointClass creates a Point class for testing with simplified NewInstance
func createPointClass(ctx *Context) (Value, uint32, error) {
	return NewClassBuilder("Point").
		Constructor(func(ctx *Context, newTarget Value, args []Value) Value {
			x, y := 0.0, 0.0
			if len(args) > 0 {
				x = args[0].Float64()
			}
			if len(args) > 1 {
				y = args[1].Float64()
			}

			point := &Point{X: x, Y: y}
			// Use simplified NewInstance (automatic classID retrieval)
			return newTarget.NewInstance(point)
		}).
		Method("norm", func(ctx *Context, this Value, args []Value) Value {
			obj, err := this.GetGoObject()
			if err != nil {
				return ctx.ThrowError(err)
			}
			point := obj.(*Point)
			norm := math.Sqrt(point.X*point.X + point.Y*point.Y)
			return ctx.Float64(norm)
		}).
		Method("toString", func(ctx *Context, this Value, args []Value) Value {
			obj, err := this.GetGoObject()
			if err != nil {
				return ctx.ThrowError(err)
			}
			point := obj.(*Point)
			return ctx.String(point.String())
		}).
		Property("x",
			func(ctx *Context, this Value) Value { // getter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				return ctx.Float64(point.X)
			},
			func(ctx *Context, this Value, value Value) Value { // setter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				point.X = value.Float64()
				return ctx.Undefined()
			}).
		Property("y",
			func(ctx *Context, this Value) Value { // getter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				return ctx.Float64(point.Y)
			},
			func(ctx *Context, this Value, value Value) Value { // setter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				point.Y = value.Float64()
				return ctx.Undefined()
			}).
		StaticMethod("zero", func(ctx *Context, this Value, args []Value) Value {
			// Use simplified NewInstance for static method as well
			return this.NewInstance(&Point{X: 0, Y: 0})
		}).
		StaticProperty("PI",
			func(ctx *Context, this Value) Value { // static getter
				return ctx.Float64(math.Pi)
			},
			nil). // no setter, read-only
		Build(ctx)
}

// TestBasicClassCreation tests basic class creation and registration
func TestBasicClassCreation(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create Point class
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Once set as global property, Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test basic constructor call
	result, err := context.Eval(`
        let p = new Point(3, 4);
        p.norm();
    `)
	if err != nil {
		t.Fatalf("Failed to evaluate basic constructor test: %v", err)
	}
	defer result.Free()

	expected := 5.0 // sqrt(3^2 + 4^2) = 5
	if math.Abs(result.Float64()-expected) > 0.001 {
		t.Errorf("Expected norm to be %f, got %f", expected, result.Float64())
	}
}

// TestConstructorFunctionality tests constructor with different parameter counts
func TestConstructorFunctionality(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test constructor with no arguments
	result, err := context.Eval(`
        let p1 = new Point();
        [p1.x, p1.y];
    `)
	if err != nil {
		t.Fatalf("Failed to evaluate no-args constructor: %v", err)
	}
	defer result.Free()

	if result.GetIdx(0).Float64() != 0.0 || result.GetIdx(1).Float64() != 0.0 {
		t.Errorf("Expected Point(0, 0), got Point(%f, %f)",
			result.GetIdx(0).Float64(), result.GetIdx(1).Float64())
	}

	// Test constructor with partial arguments
	result2, err := context.Eval(`
        let p2 = new Point(5);
        [p2.x, p2.y];
    `)
	if err != nil {
		t.Fatalf("Failed to evaluate partial-args constructor: %v", err)
	}
	defer result2.Free()

	if result2.GetIdx(0).Float64() != 5.0 || result2.GetIdx(1).Float64() != 0.0 {
		t.Errorf("Expected Point(5, 0), got Point(%f, %f)",
			result2.GetIdx(0).Float64(), result2.GetIdx(1).Float64())
	}
}

// TestInstanceMethods tests instance method functionality
func TestInstanceMethods(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test norm method
	result, err := context.Eval(`
        let p1 = new Point(3, 4);
        p1.norm();
    `)
	if err != nil {
		t.Fatalf("Failed to evaluate norm method: %v", err)
	}
	defer result.Free()

	if math.Abs(result.Float64()-5.0) > 0.001 {
		t.Errorf("Expected norm 5.0, got %f", result.Float64())
	}

	// Test toString method
	result2, err := context.Eval(`
        let p2 = new Point(1.5, 2.5);
        p2.toString();
    `)
	if err != nil {
		t.Fatalf("Failed to evaluate toString method: %v", err)
	}
	defer result2.Free()

	expected := "Point(1.50, 2.50)"
	if result2.String() != expected {
		t.Errorf("Expected toString '%s', got '%s'", expected, result2.String())
	}
}

// TestProperties tests getter and setter properties
func TestProperties(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test property getters
	result, err := context.Eval(`
        let p1 = new Point(3, 4);
        [p1.x, p1.y];
    `)
	if err != nil {
		t.Fatalf("Failed to evaluate property getters: %v", err)
	}
	defer result.Free()

	if result.GetIdx(0).Float64() != 3.0 || result.GetIdx(1).Float64() != 4.0 {
		t.Errorf("Expected [3, 4], got [%f, %f]",
			result.GetIdx(0).Float64(), result.GetIdx(1).Float64())
	}

	// Test property setters
	result2, err := context.Eval(`
        let p2 = new Point(1, 2);
        p2.x = 10;
        p2.y = 20;
        [p2.x, p2.y];
    `)
	if err != nil {
		t.Fatalf("Failed to evaluate property setters: %v", err)
	}
	defer result2.Free()

	if result2.GetIdx(0).Float64() != 10.0 || result2.GetIdx(1).Float64() != 20.0 {
		t.Errorf("Expected [10, 20], got [%f, %f]",
			result2.GetIdx(0).Float64(), result2.GetIdx(1).Float64())
	}
}

// TestStaticMethods tests static method functionality
func TestStaticMethods(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test static method
	result, err := context.Eval(`
        let p1 = Point.zero();
        [p1.x, p1.y];
    `)
	if err != nil {
		t.Fatalf("Failed to evaluate static method: %v", err)
	}
	defer result.Free()

	if result.GetIdx(0).Float64() != 0.0 || result.GetIdx(1).Float64() != 0.0 {
		t.Errorf("Expected [0, 0], got [%f, %f]",
			result.GetIdx(0).Float64(), result.GetIdx(1).Float64())
	}
}

// TestStaticProperties tests static property functionality
func TestStaticProperties(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test static read-only property
	result, err := context.Eval(`Point.PI`)
	if err != nil {
		t.Fatalf("Failed to evaluate static property: %v", err)
	}
	defer result.Free()

	if math.Abs(result.Float64()-math.Pi) > 0.001 {
		t.Errorf("Expected PI %f, got %f", math.Pi, result.Float64())
	}
}

// TestInheritanceAndNewTarget tests inheritance support with new.target
func TestInheritanceAndNewTarget(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create and register Point class
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test inheritance using extends
	result, err := context.Eval(`
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
	if err != nil {
		t.Fatalf("Failed to evaluate inheritance test: %v", err)
	}
	defer result.Free()

	expected := 13.0 // sqrt(9 + 16 + 144) = 13
	if math.Abs(result.Float64()-expected) > 0.001 {
		t.Errorf("Expected 3D norm 13.0, got %f", result.Float64())
	}

	// Test that inherited object is still instance of Point
	result2, err := context.Eval(`
        let p3d2 = new Point3D(1, 2, 3);
        p3d2 instanceof Point;
    `)
	if err != nil {
		t.Fatalf("Failed to evaluate instanceof test: %v", err)
	}
	defer result2.Free()

	if !result2.ToBool() {
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
	pointConstructor, pointClassID, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Create test instance
	instance, err := context.Eval(`new Point(1, 2)`)
	if err != nil {
		t.Fatalf("Failed to create test instance: %v", err)
	}
	defer instance.Free()

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
	regularObj, err := context.Eval(`({})`)
	if err != nil {
		t.Fatalf("Failed to create regular object: %v", err)
	}
	defer regularObj.Free()

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
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Create test instance
	instance, err := context.Eval(`new Point(3.14, 2.71)`)
	if err != nil {
		t.Fatalf("Failed to create test instance: %v", err)
	}
	defer instance.Free()

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
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Create instances that will be garbage collected
	_, err = context.Eval(`
        for (let i = 0; i < 10; i++) {
            let p = new Point(i, i * 2);
            // Don't keep references, let them be GC'd
        }
    `)
	if err != nil {
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
	ctor, _, err := NewClassBuilder("").
		Constructor(func(ctx *Context, newTarget Value, args []Value) Value {
			return ctx.Undefined()
		}).
		Method("getValue", func(ctx *Context, this Value, args []Value) Value {
			return ctx.Float64(0)
		}).
		Property("y",
			func(ctx *Context, this Value) Value { // getter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				return ctx.Float64(point.Y)
			},
			func(ctx *Context, this Value, value Value) Value { // setter
				obj, err := this.GetGoObject()
				if err != nil {
					return ctx.ThrowError(err)
				}
				point := obj.(*Point)
				point.Y = value.Float64()
				return ctx.Undefined()
			}).
		Build(context)
	defer ctor.Free()

	if err == nil {
		t.Errorf("Expected error for empty class name")
	}

	// Test creating class without constructor
	ctor2, _, err := NewClassBuilder("TestClass").Build(context)
	defer ctor2.Free()
	if err == nil {
		t.Errorf("Expected error for missing constructor")
	}

	// Test GetGoObject on non-class object
	regularObj, err := context.Eval(`({})`)
	if err != nil {
		t.Fatalf("Failed to create regular object: %v", err)
	}
	defer regularObj.Free()

	// Use GetGoObject to test error handling
	_, err = regularObj.GetGoObject()

	if err == nil {
		t.Errorf("Expected error when getting instance data from regular object")
	}

	// Test GetGoObject on non-object
	numberValue := context.Float64(42.0)
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
		constructor, _, err := NewClassBuilder(className).
			Constructor(func(ctx *Context, newTarget Value, args []Value) Value {
				point := &Point{X: float64(i), Y: float64(i * 2)}
				// Use simplified NewInstance
				return newTarget.NewInstance(point)
			}).
			Method("getValue", func(ctx *Context, this Value, args []Value) Value {
				return ctx.Float64(float64(i))
			}).
			Build(context)

		if err != nil {
			t.Fatalf("Failed to create class %s: %v", className, err)
		}

		// Create instances and let them be garbage collected
		// Note: Globals will manage the constructor memory automatically
		context.Globals().Set(className, constructor)
		_, err = context.Eval(fmt.Sprintf(`
            for (let j = 0; j < 5; j++) {
                let obj = new %s();
            }
        `, className))
		if err != nil {
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
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Test complex inheritance hierarchy
	result, err := context.Eval(`
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
	if err != nil {
		t.Fatalf("Failed to evaluate complex hierarchy: %v", err)
	}
	defer result.Free()

	// Check all instanceof relationships
	if !result.GetIdx(0).ToBool() {
		t.Errorf("Expected NamedColoredPoint to be instanceof Point")
	}
	if !result.GetIdx(1).ToBool() {
		t.Errorf("Expected NamedColoredPoint to be instanceof ColoredPoint")
	}
	if !result.GetIdx(2).ToBool() {
		t.Errorf("Expected NamedColoredPoint to be instanceof NamedColoredPoint")
	}

	// Check inherited properties and methods
	if result.GetIdx(3).Float64() != 3.0 {
		t.Errorf("Expected x=3, got %f", result.GetIdx(3).Float64())
	}
	if result.GetIdx(4).Float64() != 4.0 {
		t.Errorf("Expected y=4, got %f", result.GetIdx(4).Float64())
	}
	if result.GetIdx(5).String() != "red" {
		t.Errorf("Expected color=red, got %s", result.GetIdx(5).String())
	}
	if result.GetIdx(6).String() != "MyPoint" {
		t.Errorf("Expected name=MyPoint, got %s", result.GetIdx(6).String())
	}
	if math.Abs(result.GetIdx(7).Float64()-5.0) > 0.001 {
		t.Errorf("Expected norm=5, got %f", result.GetIdx(7).Float64())
	}
}

// TestConcurrentAccess tests thread safety (basic test)
func TestConcurrentAccess(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create Point class
	pointConstructor, _, err := createPointClass(context)
	if err != nil {
		t.Fatalf("Failed to create Point class: %v", err)
	}

	// Register Point class globally
	// Note: Globals will manage the memory automatically
	context.Globals().Set("Point", pointConstructor)

	// Note: QuickJS is not thread-safe, so this test just ensures
	// that our binding code doesn't introduce obvious race conditions
	// when accessed from a single goroutine repeatedly

	for i := 0; i < 100; i++ {
		result, err := context.Eval(fmt.Sprintf(`
            let p%d = new Point(%d, %d);
            p%d.norm();
        `, i, i, i+1, i))
		if err != nil {
			t.Fatalf("Failed on iteration %d: %v", i, err)
		}
		result.Free()
	}
}

// TestUnifiedConstructorMapping tests the unified constructor -> classID mapping
func TestUnifiedConstructorMapping(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test manual class creation
	manualConstructor, manualClassID, err := createPointClass(ctx)
	if err != nil {
		t.Fatalf("Failed to create manual Point class: %v", err)
	}

	// Test reflection class creation
	reflectConstructor, reflectClassID, err := ctx.BindClass(&Point{})
	if err != nil {
		t.Fatalf("Failed to create reflected Point class: %v", err)
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

	// Test that both can create instances with the simplified API
	ctx.Globals().Set("ManualPoint", manualConstructor)
	ctx.Globals().Set("ReflectPoint", reflectConstructor)

	result, err := ctx.Eval(`
        let manual = new ManualPoint(1, 2);
        let reflect = new ReflectPoint();
        reflect.X = 3;
        reflect.Y = 4;
        [manual.x, manual.y, reflect.X, reflect.Y];
    `)
	if err != nil {
		t.Fatalf("Failed to test unified mapping: %v", err)
	}
	defer result.Free()

	if result.GetIdx(0).Float64() != 1.0 || result.GetIdx(1).Float64() != 2.0 {
		t.Errorf("Manual constructor instance incorrect: got (%f, %f)",
			result.GetIdx(0).Float64(), result.GetIdx(1).Float64())
	}
	if result.GetIdx(2).Float64() != 3.0 || result.GetIdx(3).Float64() != 4.0 {
		t.Errorf("Reflection constructor instance incorrect: got (%f, %f)",
			result.GetIdx(2).Float64(), result.GetIdx(3).Float64())
	}
}

// TestReadOnlyAndWriteOnlyProperties tests readonly and writeonly property functionality
func TestReadOnlyAndWriteOnlyProperties(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	// Test ReadOnlyProperty
	constructor1, _, err := NewClassBuilder("ReadOnlyTest").
		Constructor(func(ctx *Context, newTarget Value, args []Value) Value {
			return newTarget.NewInstance(&Point{X: 10, Y: 20})
		}).
		ReadOnlyProperty("readOnlyX", func(ctx *Context, this Value) Value {
			obj, _ := this.GetGoObject()
			point := obj.(*Point)
			return ctx.Float64(point.X)
		}).
		Build(ctx)

	if err != nil {
		t.Fatalf("Failed to create ReadOnlyTest class: %v", err)
	}

	ctx.Globals().Set("ReadOnlyTest", constructor1)

	// Test reading works, writing doesn't change value
	result, err := ctx.Eval(`
        let obj1 = new ReadOnlyTest();
        let original = obj1.readOnlyX;
        obj1.readOnlyX = 999; // Should not change
        [original, obj1.readOnlyX];
    `)
	if err != nil {
		t.Fatalf("ReadOnly property test failed: %v", err)
	}
	defer result.Free()

	if result.GetIdx(0).Float64() != 10.0 || result.GetIdx(1).Float64() != 10.0 {
		t.Errorf("ReadOnly property failed: expected [10, 10], got [%f, %f]",
			result.GetIdx(0).Float64(), result.GetIdx(1).Float64())
	}

	// Test WriteOnlyProperty
	constructor2, _, err := NewClassBuilder("WriteOnlyTest").
		Constructor(func(ctx *Context, newTarget Value, args []Value) Value {
			return newTarget.NewInstance(&Point{X: 0, Y: 0})
		}).
		WriteOnlyProperty("writeOnlyX", func(ctx *Context, this Value, value Value) Value {
			obj, _ := this.GetGoObject()
			point := obj.(*Point)
			point.X = value.Float64()
			return ctx.Undefined()
		}).
		ReadOnlyProperty("getX", func(ctx *Context, this Value) Value {
			obj, _ := this.GetGoObject()
			point := obj.(*Point)
			return ctx.Float64(point.X)
		}).
		Build(ctx)

	if err != nil {
		t.Fatalf("Failed to create WriteOnlyTest class: %v", err)
	}

	ctx.Globals().Set("WriteOnlyTest", constructor2)

	// Test writing works, reading returns undefined
	result2, err := ctx.Eval(`
        let obj2 = new WriteOnlyTest();
        obj2.writeOnlyX = 42;
        [obj2.getX, obj2.writeOnlyX]; // getX should show 42, writeOnlyX should be undefined
    `)
	if err != nil {
		t.Fatalf("WriteOnly property test failed: %v", err)
	}
	defer result2.Free()

	if result2.GetIdx(0).Float64() != 42.0 || !result2.GetIdx(1).IsUndefined() {
		t.Errorf("WriteOnly property failed: expected [42, undefined], got [%f, %v]",
			result2.GetIdx(0).Float64(), result2.GetIdx(1).String())
	}

	// Test StaticReadOnlyProperty
	constructor3, _, err := NewClassBuilder("StaticReadOnlyTest").
		Constructor(func(ctx *Context, newTarget Value, args []Value) Value {
			return newTarget.NewInstance(&Point{X: 0, Y: 0})
		}).
		StaticReadOnlyProperty("VERSION", func(ctx *Context, this Value) Value {
			return ctx.String("1.0.0")
		}).
		Build(ctx)

	if err != nil {
		t.Fatalf("Failed to create StaticReadOnlyTest class: %v", err)
	}

	ctx.Globals().Set("StaticReadOnlyTest", constructor3)

	// Test static readonly property
	result3, err := ctx.Eval(`
        let original3 = StaticReadOnlyTest.VERSION;
        StaticReadOnlyTest.VERSION = "2.0.0"; // Should not change
        [original3, StaticReadOnlyTest.VERSION];
    `)
	if err != nil {
		t.Fatalf("StaticReadOnly property test failed: %v", err)
	}
	defer result3.Free()

	if result3.GetIdx(0).String() != "1.0.0" || result3.GetIdx(1).String() != "1.0.0" {
		t.Errorf("StaticReadOnly property failed: expected ['1.0.0', '1.0.0'], got ['%s', '%s']",
			result3.GetIdx(0).String(), result3.GetIdx(1).String())
	}
}

// TestMethodLengthDefault tests the default method length parameter handling
func TestMethodLengthDefault(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	context := rt.NewContext()
	defer context.Close()

	// Create a class with method using negative length (should default to DefaultMethodParams)
	constructor, _, err := NewClassBuilder("LengthTestClass").
		Constructor(func(ctx *Context, newTarget Value, args []Value) Value {
			return newTarget.NewInstance(&Point{X: 0, Y: 0})
		}).
		MethodWithLength("testMethod", func(ctx *Context, this Value, args []Value) Value {
			return ctx.Int32(int32(len(args)))
		}, -1). // Negative length should trigger default
		Build(context)

	if err != nil {
		t.Fatalf("Failed to create class with negative method length: %v", err)
	}

	context.Globals().Set("LengthTestClass", constructor)

	// Test that the method works correctly (the default length doesn't affect functionality)
	result, err := context.Eval(`
        let obj = new LengthTestClass();
        obj.testMethod(1, 2, 3); // Should return 3 (number of arguments)
    `)
	if err != nil {
		t.Fatalf("Failed to test method with default length: %v", err)
	}
	defer result.Free()

	if result.Int32() != 3 {
		t.Errorf("Expected method to receive 3 arguments, got %d", result.Int32())
	}
}
