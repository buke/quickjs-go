package quickjs

/*
#include "bridge.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// =============================================================================
// GLOBAL CONSTRUCTOR REGISTRY FOR UNIFIED MAPPING
// =============================================================================

// Global constructor to class ID mapping table for unified management
var globalConstructorRegistry = sync.Map{} // constructor hash -> classID

// Helper function to create a stable key from JSValue
// For constructor functions, we use the object pointer as a unique identifier
func jsValueToKey(jsVal C.JSValue) uint64 {
	// Constructors are JavaScript objects, so we use the object pointer
	// This is stable and unique for each JavaScript object instance
	objPtr := C.JS_VALUE_GET_PTR_Wrapper(jsVal)
	return uint64(uintptr(objPtr))
}

// registerConstructorClassID stores the constructor -> classID mapping
func registerConstructorClassID(constructor C.JSValue, classID uint32) {
	constructorKey := jsValueToKey(constructor)
	globalConstructorRegistry.Store(constructorKey, classID)
}

// getConstructorClassID retrieves the classID for a given constructor
func getConstructorClassID(constructor C.JSValue) (uint32, bool) {
	constructorKey := jsValueToKey(constructor)
	if classIDInterface, ok := globalConstructorRegistry.Load(constructorKey); ok {
		return classIDInterface.(uint32), true
	}
	return 0, false
}

// =============================================================================
// CLASS FINALIZER INTERFACE
// =============================================================================

// Optional cleanup interface for class instances
// Objects implementing this interface will have Finalize() called automatically
// when the JavaScript object is garbage collected
type ClassFinalizer interface {
	Finalize()
}

// =============================================================================
// CLASS-RELATED FUNCTION TYPES
// =============================================================================

// Class-related function types with consistent Class prefix
// These correspond exactly to QuickJS C API function pointer types

// ClassConstructorFunc represents a class constructor function
// newTarget parameter supports inheritance and new.target checking
// Corresponds to QuickJS JSCFunctionType.constructor_magic
type ClassConstructorFunc func(ctx *Context, newTarget Value, args []Value) Value

// ClassMethodFunc represents both instance and static methods
// this parameter represents the object instance for instance methods,
// or the constructor function for static methods
// Corresponds to QuickJS JSCFunctionType.generic_magic
type ClassMethodFunc func(ctx *Context, this Value, args []Value) Value

// ClassGetterFunc represents accessor getter functions
// Corresponds to QuickJS JSCFunctionType.getter_magic
type ClassGetterFunc func(ctx *Context, this Value) Value

// ClassSetterFunc represents accessor setter functions
// Returns the set value or an exception
// Corresponds to QuickJS JSCFunctionType.setter_magic
type ClassSetterFunc func(ctx *Context, this Value, value Value) Value

// =============================================================================
// CLASS BINDING CONFIGURATION STRUCTURES
// =============================================================================

// MethodEntry represents a method binding configuration
type MethodEntry struct {
	Name   string          // Method name in JavaScript
	Func   ClassMethodFunc // Method implementation function
	Static bool            // true for static methods, false for instance methods
	Length int             // Expected parameter count, -1 for auto-detection
}

// AccessorEntry represents an accessor binding configuration
type AccessorEntry struct {
	Name   string          // Accessor name in JavaScript
	Getter ClassGetterFunc // Optional getter function
	Setter ClassSetterFunc // Optional setter function
	Static bool            // true for static accessors, false for instance accessors
}

// =============================================================================
// CLASS BUILDER - FLUENT API FOR BUILDING JAVASCRIPT CLASSES
// =============================================================================

// ClassBuilder provides a fluent API for building JavaScript classes
// Uses builder pattern for easy and readable class definition
type ClassBuilder struct {
	name        string
	constructor ClassConstructorFunc
	methods     []MethodEntry
	accessors   []AccessorEntry
}

// NewClassBuilder creates a new ClassBuilder with the specified name
// This is the entry point for building JavaScript classes
func NewClassBuilder(name string) *ClassBuilder {
	return &ClassBuilder{
		name:      name,
		methods:   make([]MethodEntry, 0),
		accessors: make([]AccessorEntry, 0),
	}
}

// =============================================================================
// CLASSBUILDER FLUENT API METHODS
// =============================================================================

// Constructor sets the constructor function for the class
// The constructor function will be called when creating new instances
func (cb *ClassBuilder) Constructor(fn ClassConstructorFunc) *ClassBuilder {
	cb.constructor = fn
	return cb
}

// Method adds an instance method to the class
// Instance methods are called on object instances
func (cb *ClassBuilder) Method(name string, fn ClassMethodFunc) *ClassBuilder {
	cb.methods = append(cb.methods, MethodEntry{
		Name:   name,
		Func:   fn,
		Static: false,
		Length: 0,
	})
	return cb
}

// StaticMethod adds a static method to the class
// Static methods are called on the constructor function itself
func (cb *ClassBuilder) StaticMethod(name string, fn ClassMethodFunc) *ClassBuilder {
	cb.methods = append(cb.methods, MethodEntry{
		Name:   name,
		Func:   fn,
		Static: true,
		Length: 0,
	})
	return cb
}

// Accessor adds a read-write accessor to the class instance
// Pass nil for getter to create write-only accessor
// Pass nil for setter to create read-only accessor
func (cb *ClassBuilder) Accessor(name string, getter ClassGetterFunc, setter ClassSetterFunc) *ClassBuilder {
	cb.accessors = append(cb.accessors, AccessorEntry{
		Name:   name,
		Getter: getter,
		Setter: setter,
		Static: false,
	})
	return cb
}

// StaticAccessor adds a read-write static accessor to the class constructor
// Pass nil for getter to create write-only accessor
// Pass nil for setter to create read-only accessor
func (cb *ClassBuilder) StaticAccessor(name string, getter ClassGetterFunc, setter ClassSetterFunc) *ClassBuilder {
	cb.accessors = append(cb.accessors, AccessorEntry{
		Name:   name,
		Getter: getter,
		Setter: setter,
		Static: true,
	})
	return cb
}

// Build creates and registers the JavaScript class in the given context
// Returns the constructor function and classID for NewInstance
func (cb *ClassBuilder) Build(ctx *Context) (Value, uint32, error) {
	return ctx.createClass(cb)
}

// =============================================================================
// CLASS CREATION IMPLEMENTATION
// =============================================================================

// validateClassBuilder validates ClassBuilder configuration
func validateClassBuilder(builder *ClassBuilder) error {
	if builder.constructor == nil {
		return errors.New("constructor function is required")
	}
	return nil
}

// createClass implements the core class creation logic using C layer optimization
// This method delegates most work to C layer for better performance
func (ctx *Context) createClass(builder *ClassBuilder) (Value, uint32, error) {
	// Step 1: Input validation (keep in Go layer for business logic)
	if err := validateClassBuilder(builder); err != nil {
		return Value{}, 0, err
	}

	// Step 2: Go layer manages class name and JSClassDef memory
	className := C.CString(builder.name)
	defer C.free(unsafe.Pointer(className))

	classDef := &C.JSClassDef{
		class_name: className,
		finalizer:  (*C.JSClassFinalizer)(unsafe.Pointer(C.GoClassFinalizerProxy)),
	}

	// Step 3: Prepare classID variable for C function to allocate internally
	var classID C.JSClassID

	// Step 4: Store constructor function in handleStore
	constructorID := ctx.handleStore.Store(builder.constructor)

	// Step 5: Prepare method entries for C layer
	var cMethods []C.MethodEntry
	var methodIDs []int32

	for _, method := range builder.methods {
		// Store method function in handleStore
		handlerID := ctx.handleStore.Store(method.Func)
		methodIDs = append(methodIDs, handlerID)

		// Convert method name to C string
		methodName := C.CString(method.Name)
		// Note: Don't defer free as C layer needs these strings during binding

		// Determine length parameter
		length := method.Length

		// Convert static flag
		isStatic := 0
		if method.Static {
			isStatic = 1
		}

		// Create C method entry
		cMethods = append(cMethods, C.MethodEntry{
			name:       methodName,
			handler_id: C.int32_t(handlerID),
			length:     C.int(length),
			is_static:  C.int(isStatic),
		})
	}

	// Step 6: Prepare accessor entries for C layer
	var cAccessors []C.AccessorEntry
	var accessorIDs []int32

	for _, accessor := range builder.accessors {
		// Convert accessor name to C string
		accessorName := C.CString(accessor.Name)
		// Note: Don't defer free as C layer needs these strings during binding

		var getterID, setterID C.int32_t = 0, 0

		// Store getter function if provided
		if accessor.Getter != nil {
			handlerID := ctx.handleStore.Store(accessor.Getter)
			accessorIDs = append(accessorIDs, handlerID)
			getterID = C.int32_t(handlerID)
		}

		// Store setter function if provided
		if accessor.Setter != nil {
			handlerID := ctx.handleStore.Store(accessor.Setter)
			accessorIDs = append(accessorIDs, handlerID)
			setterID = C.int32_t(handlerID)
		}

		// Convert static flag
		isStatic := 0
		if accessor.Static {
			isStatic = 1
		}

		// Create C accessor entry
		cAccessors = append(cAccessors, C.AccessorEntry{
			name:      accessorName,
			getter_id: getterID,
			setter_id: setterID,
			is_static: C.int(isStatic),
		})
	}

	// Step 7: Prepare C array pointers (handle empty arrays)
	var cMethodsPtr *C.MethodEntry
	var cAccessorsPtr *C.AccessorEntry

	if len(cMethods) > 0 {
		cMethodsPtr = &cMethods[0]
	}
	if len(cAccessors) > 0 {
		cAccessorsPtr = &cAccessors[0]
	}

	// Step 8: Call C function to create class (single call does all the work)
	constructor := C.CreateClass(
		ctx.ref,
		&classID, // C function allocates class_id internally
		classDef, // Go layer manages JSClassDef memory
		C.int32_t(constructorID),
		cMethodsPtr,
		C.int(len(cMethods)),
		cAccessorsPtr,
		C.int(len(cAccessors)),
	)

	// Step 9: Error handling - clean up all stored handlers on failure
	if C.JS_IsException(constructor) != 0 {
		fmt.Printf("Failed to create class '%s'\n", builder.name)
		// Clean up constructor handler
		ctx.handleStore.Delete(constructorID)

		// Clean up method handlers
		for _, id := range methodIDs {
			ctx.handleStore.Delete(id)
		}

		// Clean up accessor handlers
		for _, id := range accessorIDs {
			ctx.handleStore.Delete(id)
		}

		// Note: Don't clean up className and classDef - let Go GC handle them
		// The C function failed, so QuickJS isn't using them

		return Value{ctx: ctx, ref: constructor}, 0, ctx.Exception()
	}

	// Step 10: Register constructor -> classID mapping for unified access
	// This enables the global registry for NewInstance lookup
	registerConstructorClassID(constructor, uint32(classID))

	// Success: className, classDef, and classID are all managed properly
	// - className and classDef: Go GC manages lifetime (QuickJS holds references)
	// - classID: returned via pointer from C function
	// - All handlers: stored in handleStore for proper cleanup
	return Value{ctx: ctx, ref: constructor}, uint32(classID), nil
}
