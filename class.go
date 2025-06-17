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
// CLASS-RELATED FUNCTION TYPES - MODIFIED FOR SCHEME C
// =============================================================================

// Class-related function types with consistent Class prefix
// These correspond exactly to QuickJS C API function pointer types

// MODIFIED FOR SCHEME C: ClassConstructorFunc signature changed
// Constructor now receives pre-created instance and returns Go object to associate
// This aligns with Scheme C design where instances are pre-created with bound properties
type ClassConstructorFunc func(ctx *Context, instance Value, args []Value) (interface{}, error)

// ClassMethodFunc represents both instance and static methods - unchanged
// this parameter represents the object instance for instance methods,
// or the constructor function for static methods
// Corresponds to QuickJS JSCFunctionType.generic_magic
type ClassMethodFunc func(ctx *Context, this Value, args []Value) Value

// ClassGetterFunc represents accessor getter functions - unchanged
// Corresponds to QuickJS JSCFunctionType.getter_magic
type ClassGetterFunc func(ctx *Context, this Value) Value

// ClassSetterFunc represents accessor setter functions - unchanged
// Returns the set value or an exception
// Corresponds to QuickJS JSCFunctionType.setter_magic
type ClassSetterFunc func(ctx *Context, this Value, value Value) Value

// =============================================================================
// CLASS BINDING CONFIGURATION STRUCTURES
// =============================================================================

// MethodEntry represents a method binding configuration - unchanged
type MethodEntry struct {
	Name   string          // Method name in JavaScript
	Func   ClassMethodFunc // Method implementation function
	Static bool            // true for static methods, false for instance methods
	Length int             // Expected parameter count, 0 for default
}

// AccessorEntry represents an accessor binding configuration - unchanged
type AccessorEntry struct {
	Name   string          // Accessor name in JavaScript
	Getter ClassGetterFunc // Optional getter function
	Setter ClassSetterFunc // Optional setter function
	Static bool            // true for static accessors, false for instance accessors
}

// PropertyEntry represents a property binding configuration - unchanged
type PropertyEntry struct {
	Name   string // Property name in JavaScript
	Value  Value  // Property value (JavaScript Value)
	Static bool   // true for static properties, false for instance properties
	Flags  int    // Property flags (writable, enumerable, configurable)
}

// =============================================================================
// PROPERTY FLAGS CONSTANTS
// =============================================================================

// Property flags constants matching QuickJS - unchanged
const (
	PropertyConfigurable = 1 << 0 // JS_PROP_CONFIGURABLE
	PropertyWritable     = 1 << 1 // JS_PROP_WRITABLE
	PropertyEnumerable   = 1 << 2 // JS_PROP_ENUMERABLE

	// Default property flags (writable, enumerable, configurable)
	PropertyDefault = PropertyConfigurable | PropertyWritable | PropertyEnumerable
)

// =============================================================================
// CLASS BUILDER - FLUENT API FOR BUILDING JAVASCRIPT CLASSES
// =============================================================================

// ClassBuilder provides a fluent API for building JavaScript classes
// Uses builder pattern for easy and readable class definition
// MODIFIED FOR SCHEME C: Now stores complete class definition including instance properties
type ClassBuilder struct {
	name        string
	constructor ClassConstructorFunc // MODIFIED: Uses new signature
	methods     []MethodEntry
	accessors   []AccessorEntry
	properties  []PropertyEntry // Properties field (both static and instance)
}

// NewClassBuilder creates a new ClassBuilder with the specified name
// This is the entry point for building JavaScript classes
func NewClassBuilder(name string) *ClassBuilder {
	return &ClassBuilder{
		name:       name,
		methods:    make([]MethodEntry, 0),
		accessors:  make([]AccessorEntry, 0),
		properties: make([]PropertyEntry, 0),
	}
}

// =============================================================================
// CLASSBUILDER FLUENT API METHODS
// =============================================================================

// Constructor sets the constructor function for the class
// MODIFIED FOR SCHEME C: Now uses new constructor signature
// The constructor function will be called with pre-created instance
func (cb *ClassBuilder) Constructor(fn ClassConstructorFunc) *ClassBuilder {
	cb.constructor = fn
	return cb
}

// Method adds an instance method to the class - unchanged
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

// StaticMethod adds a static method to the class - unchanged
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

// Accessor adds a read-write accessor to the class instance - unchanged
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

// StaticAccessor adds a read-write static accessor to the class constructor - unchanged
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

// =============================================================================
// PROPERTY API METHODS
// =============================================================================

// Property adds a data property to the class instance - unchanged
// Default flags: writable, enumerable, configurable
// SCHEME C: Instance properties will be bound during instance creation
func (cb *ClassBuilder) Property(name string, value Value, flags ...int) *ClassBuilder {
	propFlags := PropertyDefault
	if len(flags) > 0 {
		propFlags = flags[0]
	}

	cb.properties = append(cb.properties, PropertyEntry{
		Name:   name,
		Value:  value,
		Static: false, // Instance property
		Flags:  propFlags,
	})
	return cb
}

// StaticProperty adds a data property to the class constructor - unchanged
// Default flags: writable, enumerable, configurable
func (cb *ClassBuilder) StaticProperty(name string, value Value, flags ...int) *ClassBuilder {
	propFlags := PropertyDefault
	if len(flags) > 0 {
		propFlags = flags[0]
	}

	cb.properties = append(cb.properties, PropertyEntry{
		Name:   name,
		Value:  value,
		Static: true, // Static property
		Flags:  propFlags,
	})
	return cb
}

// Build creates and registers the JavaScript class in the given context
// Returns the constructor function and classID for NewInstance
func (cb *ClassBuilder) Build(ctx *Context) (Value, uint32, error) {
	return ctx.createClass(cb)
}

// =============================================================================
// CLASS CREATION IMPLEMENTATION - MODIFIED FOR SCHEME C
// =============================================================================

// validateClassBuilder validates ClassBuilder configuration - unchanged
func validateClassBuilder(builder *ClassBuilder) error {
	if builder.constructor == nil {
		return errors.New("constructor function is required")
	}
	return nil
}

// createClass implements the core class creation logic using C layer optimization
// MODIFIED FOR SCHEME C: Now stores entire ClassBuilder and separates static/instance properties
func (ctx *Context) createClass(builder *ClassBuilder) (Value, uint32, error) {
	// Step 1: Input validation (keep in Go layer for business logic) - unchanged
	if err := validateClassBuilder(builder); err != nil {
		return Value{}, 0, err
	}

	// Step 2: Go layer manages class name and JSClassDef memory - unchanged
	className := C.CString(builder.name)
	defer C.free(unsafe.Pointer(className))

	classDef := &C.JSClassDef{
		class_name: className,
		finalizer:  (*C.JSClassFinalizer)(unsafe.Pointer(C.GoClassFinalizerProxy)),
	}

	// Step 3: Prepare classID variable for C function to allocate internally - unchanged
	var classID C.JSClassID

	// SCHEME C STEP 4: Store entire ClassBuilder in HandleStore (not just constructor)
	// This allows constructor proxy to access both constructor function and instance properties
	constructorID := ctx.handleStore.Store(builder)

	// Step 5: Prepare method entries for C layer - unchanged logic, same implementation
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

	// Step 6: Prepare accessor entries for C layer - unchanged logic, same implementation
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

	// SCHEME C STEP 7: Prepare property entries - ONLY STATIC PROPERTIES for CreateClass
	// Instance properties are handled separately by constructor proxy
	var cProperties []C.PropertyEntry

	for _, property := range builder.properties {
		// SCHEME C: Only include static properties for CreateClass call
		// Instance properties will be handled by constructor proxy during instance creation
		if property.Static {
			// Convert property name to C string
			propertyName := C.CString(property.Name)
			// Note: Don't defer free as C layer needs these strings during binding

			// Create C property entry for static property only
			cProperties = append(cProperties, C.PropertyEntry{
				name:      propertyName,
				value:     property.Value.ref, // Use JSValue directly
				is_static: C.int(1),           // Always static for CreateClass
				flags:     C.int(property.Flags),
			})
		}
		// Instance properties are stored in ClassBuilder and accessed by constructor proxy
	}

	// Step 8: Prepare C array pointers (handle empty arrays) - unchanged logic
	var cMethodsPtr *C.MethodEntry
	var cAccessorsPtr *C.AccessorEntry
	var cPropertiesPtr *C.PropertyEntry

	if len(cMethods) > 0 {
		cMethodsPtr = &cMethods[0]
	}
	if len(cAccessors) > 0 {
		cAccessorsPtr = &cAccessors[0]
	}
	if len(cProperties) > 0 {
		cPropertiesPtr = &cProperties[0]
	}

	// SCHEME C STEP 9: Call C function to create class - only static properties passed
	// Instance properties are handled by constructor proxy, not by CreateClass
	constructor := C.CreateClass(
		ctx.ref,
		&classID,                 // C function allocates class_id internally
		classDef,                 // Go layer manages JSClassDef memory
		C.int32_t(constructorID), // SCHEME C: Store ClassBuilder, not individual constructor
		cMethodsPtr,
		C.int(len(cMethods)),
		cAccessorsPtr,
		C.int(len(cAccessors)),
		cPropertiesPtr,          // SCHEME C: Only static properties
		C.int(len(cProperties)), // SCHEME C: Only static property count
	)

	// Step 10: Error handling - clean up all stored handlers on failure - unchanged logic
	if C.JS_IsException(constructor) != 0 {
		fmt.Printf("Failed to create class '%s'\n", builder.name)
		// Clean up constructor handler (now stores ClassBuilder)
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

	// SCHEME C STEP 11: Register constructor -> classID mapping for constructor proxy access
	// This enables constructor proxy to extract classID from newTarget
	registerConstructorClassID(constructor, uint32(classID))

	// Success: className, classDef, and classID are all managed properly
	// - className and classDef: Go GC manages lifetime (QuickJS holds references)
	// - classID: returned via pointer from C function
	// - All handlers: stored in handleStore for proper cleanup
	// - ClassBuilder: stored in handleStore for constructor proxy access
	return Value{ctx: ctx, ref: constructor}, uint32(classID), nil
}
