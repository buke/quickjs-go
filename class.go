package quickjs

/*
#include "bridge.h"
*/
import "C"
import (
	"errors"
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
// CLASS BINDING CONFIGURATION CONSTANTS
// =============================================================================

// Constants for class binding configuration
const (
	// Default parameter counts for different function types
	DefaultConstructorParams = 2 // newTarget + arguments
	DefaultMethodParams      = 0 // Auto-detect
	DefaultGetterParams      = 0 // No parameters for getters
	DefaultSetterParams      = 1 // One parameter for setters

	// QuickJS limits
	MaxClassID = 1 << 16 // QuickJS class ID hard limit
)

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

// ClassGetterFunc represents property getter functions
// Corresponds to QuickJS JSCFunctionType.getter_magic
type ClassGetterFunc func(ctx *Context, this Value) Value

// ClassSetterFunc represents property setter functions
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

// PropertyEntry represents a property binding configuration
type PropertyEntry struct {
	Name   string          // Property name in JavaScript
	Getter ClassGetterFunc // Optional getter function
	Setter ClassSetterFunc // Optional setter function
	Static bool            // true for static properties, false for instance properties
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
	properties  []PropertyEntry
}

// NewClassBuilder creates a new ClassBuilder with the specified name
// This is the entry point for building JavaScript classes
func NewClassBuilder(name string) *ClassBuilder {
	return &ClassBuilder{
		name:       name,
		methods:    make([]MethodEntry, 0),
		properties: make([]PropertyEntry, 0),
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
	return cb.MethodWithLength(name, fn, DefaultMethodParams)
}

// StaticMethod adds a static method to the class
// Static methods are called on the constructor function itself
func (cb *ClassBuilder) StaticMethod(name string, fn ClassMethodFunc) *ClassBuilder {
	return cb.StaticMethodWithLength(name, fn, DefaultMethodParams)
}

// MethodWithLength adds an instance method with explicit parameter count
// Useful for optimization when parameter count is known
func (cb *ClassBuilder) MethodWithLength(name string, fn ClassMethodFunc, length int) *ClassBuilder {
	cb.methods = append(cb.methods, MethodEntry{
		Name:   name,
		Func:   fn,
		Static: false,
		Length: length,
	})
	return cb
}

// StaticMethodWithLength adds a static method with explicit parameter count
func (cb *ClassBuilder) StaticMethodWithLength(name string, fn ClassMethodFunc, length int) *ClassBuilder {
	cb.methods = append(cb.methods, MethodEntry{
		Name:   name,
		Func:   fn,
		Static: true,
		Length: length,
	})
	return cb
}

// Property adds a read-write property to the class instance
// Both getter and setter must be provided for read-write properties
func (cb *ClassBuilder) Property(name string, getter ClassGetterFunc, setter ClassSetterFunc) *ClassBuilder {
	cb.properties = append(cb.properties, PropertyEntry{
		Name:   name,
		Getter: getter,
		Setter: setter,
		Static: false,
	})
	return cb
}

// ReadOnlyProperty adds a read-only property to the class instance
// Only getter is provided, property cannot be modified from JavaScript
func (cb *ClassBuilder) ReadOnlyProperty(name string, getter ClassGetterFunc) *ClassBuilder {
	cb.properties = append(cb.properties, PropertyEntry{
		Name:   name,
		Getter: getter,
		Setter: nil,
		Static: false,
	})
	return cb
}

// WriteOnlyProperty adds a write-only property to the class instance
// Only setter is provided, property cannot be read from JavaScript
func (cb *ClassBuilder) WriteOnlyProperty(name string, setter ClassSetterFunc) *ClassBuilder {
	cb.properties = append(cb.properties, PropertyEntry{
		Name:   name,
		Getter: nil,
		Setter: setter,
		Static: false,
	})
	return cb
}

// StaticProperty adds a read-write static property to the class constructor
func (cb *ClassBuilder) StaticProperty(name string, getter ClassGetterFunc, setter ClassSetterFunc) *ClassBuilder {
	cb.properties = append(cb.properties, PropertyEntry{
		Name:   name,
		Getter: getter,
		Setter: setter,
		Static: true,
	})
	return cb
}

// StaticReadOnlyProperty adds a read-only static property to the class constructor
func (cb *ClassBuilder) StaticReadOnlyProperty(name string, getter ClassGetterFunc) *ClassBuilder {
	cb.properties = append(cb.properties, PropertyEntry{
		Name:   name,
		Getter: getter,
		Setter: nil,
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
	if builder.name == "" {
		return errors.New("class name cannot be empty")
	}
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
		if length < 0 {
			length = DefaultMethodParams
		}

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

	// Step 6: Prepare property entries for C layer
	var cProperties []C.PropertyEntry
	var propertyIDs []int32

	for _, prop := range builder.properties {
		// Convert property name to C string
		propName := C.CString(prop.Name)
		// Note: Don't defer free as C layer needs these strings during binding

		var getterID, setterID C.int32_t = 0, 0

		// Store getter function if provided
		if prop.Getter != nil {
			handlerID := ctx.handleStore.Store(prop.Getter)
			propertyIDs = append(propertyIDs, handlerID)
			getterID = C.int32_t(handlerID)
		}

		// Store setter function if provided
		if prop.Setter != nil {
			handlerID := ctx.handleStore.Store(prop.Setter)
			propertyIDs = append(propertyIDs, handlerID)
			setterID = C.int32_t(handlerID)
		}

		// Convert static flag
		isStatic := 0
		if prop.Static {
			isStatic = 1
		}

		// Create C property entry
		cProperties = append(cProperties, C.PropertyEntry{
			name:      propName,
			getter_id: getterID,
			setter_id: setterID,
			is_static: C.int(isStatic),
		})
	}

	// Step 7: Prepare C array pointers (handle empty arrays)
	var cMethodsPtr *C.MethodEntry
	var cPropertiesPtr *C.PropertyEntry

	if len(cMethods) > 0 {
		cMethodsPtr = &cMethods[0]
	}
	if len(cProperties) > 0 {
		cPropertiesPtr = &cProperties[0]
	}

	// Step 8: Call C function to create class (single call does all the work)
	constructor := C.CreateClass(
		ctx.ref,
		&classID, // C function allocates class_id internally
		classDef, // Go layer manages JSClassDef memory
		C.int32_t(constructorID),
		cMethodsPtr,
		C.int(len(cMethods)),
		cPropertiesPtr,
		C.int(len(cProperties)),
	)

	// Step 9: Error handling - clean up all stored handlers on failure
	if C.JS_IsException(constructor) != 0 {
		// Clean up constructor handler
		ctx.handleStore.Delete(constructorID)

		// Clean up method handlers
		for _, id := range methodIDs {
			ctx.handleStore.Delete(id)
		}

		// Clean up property handlers
		for _, id := range propertyIDs {
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

// // createClass implements the core class creation logic
// // This method follows the exact pattern from point.c example
// func (ctx *Context) createClass2(builder *ClassBuilder) (Value, uint32, error) {
// 	// Step 1: Validate input
// 	if err := validateClassBuilder(builder); err != nil {
// 		return Value{}, 0, err
// 	}

// 	// Step 2: Create class ID (corresponds to point.c: JS_NewClassID(&js_point_class_id))
// 	var classID C.JSClassID
// 	C.JS_NewClassID(&classID)

// 	// Check QuickJS limits
// 	if classID >= MaxClassID {
// 		return Value{}, uint32(classID), errors.New("class ID exceeds maximum value")
// 	}

// 	// Step 3: Register class definition (corresponds to point.c: JS_NewClass)
// 	className := C.CString(builder.name)
// 	defer C.free(unsafe.Pointer(className))

// 	classDef := C.JSClassDef{
// 		class_name: className,
// 		finalizer:  (*C.JSClassFinalizer)(unsafe.Pointer(C.GoClassFinalizerProxy)),
// 	}

// 	result := C.JS_NewClass(ctx.runtime.ref, classID, &classDef)
// 	if result != 0 {
// 		return Value{}, 0, errors.New("failed to create class definition")
// 	}

// 	// Step 4: Create prototype object (corresponds to point.c: point_proto = JS_NewObject(ctx))
// 	proto := C.JS_NewObject(ctx.ref)
// 	if C.JS_IsException(proto) != 0 {
// 		return Value{}, 0, errors.New("failed to create prototype object")
// 	}

// 	// Step 5: Bind instance methods and properties to prototype
// 	if err := ctx.bindMembersToObject(proto, builder.methods, builder.properties, false); err != nil {
// 		C.JS_FreeValue(ctx.ref, proto)
// 		return Value{}, 0, err
// 	}

// 	// Step 6: Create constructor function (corresponds to point.c: JS_NewCFunction2)
// 	constructor, constructorID, err := ctx.createCFunction(
// 		builder.name,
// 		builder.constructor,
// 		uint32(C.GetCFuncConstructorMagic()),
// 		DefaultConstructorParams,
// 	)
// 	if err != nil {
// 		C.JS_FreeValue(ctx.ref, proto)
// 		return Value{}, 0, err
// 	}

// 	// Step 7: Associate constructor with prototype (corresponds to point.c: JS_SetConstructor)
// 	C.JS_SetConstructor(ctx.ref, constructor.ref, proto)

// 	// Step 8: Set class prototype (corresponds to point.c: JS_SetClassProto)
// 	C.JS_SetClassProto(ctx.ref, classID, proto)

// 	// Step 9: Bind static methods and properties to constructor
// 	if err := ctx.bindMembersToObject(constructor.ref, builder.methods, builder.properties, true); err != nil {
// 		ctx.handleStore.Delete(constructorID)
// 		constructor.Free()
// 		return Value{}, 0, err
// 	}

// 	// Step 10: Register constructor -> classID mapping for unified access
// 	registerConstructorClassID(constructor.ref, uint32(classID))

// 	// Return constructor and classID
// 	return Value{ctx: ctx, ref: constructor.ref}, uint32(classID), nil
// }

// =============================================================================
// HELPER FUNCTIONS FOR CLASS CREATION
// =============================================================================

// // createCFunction creates a C function with the specified type and parameters
// // This is a common helper that reduces code duplication
// func (ctx *Context) createCFunction(name string, handler interface{}, funcType uint32, length int) (Value, int32, error) {
// 	handlerID := ctx.handleStore.Store(handler)
// 	cName := C.CString(name)
// 	defer C.free(unsafe.Pointer(cName))

// 	// Call C function to handle the complex logic
// 	// Parameters match JS_NewCFunction2: ctx, name, length, cproto, magic
// 	jsFunc := C.CreateCFunction(
// 		ctx.ref,
// 		cName,
// 		C.int(length),
// 		C.int(funcType),
// 		C.int32_t(handlerID),
// 	)

// 	// Check if C function returned an exception
// 	if C.JS_IsException(jsFunc) != 0 {
// 		ctx.handleStore.Delete(handlerID)
// 		return Value{ctx: ctx, ref: jsFunc}, 0, ctx.Exception()
// 	}

// 	return Value{ctx: ctx, ref: jsFunc}, handlerID, nil
// }

// // bindMembersToObject binds methods and properties to a JavaScript object
// // isStatic determines whether to bind static or instance members
// func (ctx *Context) bindMembersToObject(obj C.JSValue, methods []MethodEntry, properties []PropertyEntry, isStatic bool) error {
// 	// Bind methods
// 	for _, method := range methods {
// 		if method.Static == isStatic {
// 			if err := ctx.bindMethodToObject(obj, method); err != nil {
// 				return err
// 			}
// 		}
// 	}

// 	// Bind properties
// 	for _, prop := range properties {
// 		if prop.Static == isStatic {
// 			if err := ctx.bindPropertyToObject(obj, prop); err != nil {
// 				return err
// 			}
// 		}
// 	}

// 	return nil
// }

// // bindMethodToObject binds a method to a JavaScript object (prototype or constructor)
// func (ctx *Context) bindMethodToObject(obj C.JSValue, method MethodEntry) error {
// 	length := method.Length
// 	if length < 0 {
// 		length = DefaultMethodParams
// 	}

// 	methodFunc, methodID, err := ctx.createCFunction(
// 		method.Name,
// 		method.Func,
// 		uint32(C.GetCFuncGenericMagic()),
// 		length,
// 	)
// 	if err != nil {
// 		return err
// 	}

// 	methodName := C.CString(method.Name)
// 	defer C.free(unsafe.Pointer(methodName))

// 	result := C.JS_DefinePropertyValueStr(
// 		ctx.ref,
// 		obj,
// 		methodName,
// 		methodFunc.ref,
// 		getPropertyWritableConfigurable(),
// 	)

// 	if result < 0 {
// 		ctx.handleStore.Delete(methodID)
// 		C.JS_FreeValue(ctx.ref, methodFunc.ref)
// 		return errors.New("failed to bind method to object")
// 	}

// 	return nil
// }

// // bindPropertyToObject binds a property to a JavaScript object (prototype or constructor)
// func (ctx *Context) bindPropertyToObject(obj C.JSValue, prop PropertyEntry) error {
// 	// Convert property name to JSAtom first
// 	propName := C.CString(prop.Name)
// 	defer C.free(unsafe.Pointer(propName))

// 	propAtom := C.JS_NewAtom(ctx.ref, propName)
// 	defer C.JS_FreeAtom(ctx.ref, propAtom)

// 	// var getterFunc, setterFunc C.JSValue = C.JS_NewUndefined(), C.JS_NewUndefined()
// 	var getterFunc, setterFunc Value = ctx.Undefined(), ctx.Undefined()
// 	var getterID, setterID int32

// 	// Create getter function if provided
// 	if prop.Getter != nil {
// 		var err error
// 		getterFunc, getterID, err = ctx.createCFunction(
// 			"get "+prop.Name,
// 			prop.Getter,
// 			uint32(C.GetCFuncGetterMagic()),
// 			DefaultGetterParams,
// 		)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	// Create setter function if provided
// 	if prop.Setter != nil {
// 		var err error
// 		setterFunc, setterID, err = ctx.createCFunction(
// 			"set "+prop.Name,
// 			prop.Setter,
// 			uint32(C.GetCFuncSetterMagic()),
// 			DefaultSetterParams,
// 		)
// 		if err != nil {
// 			if prop.Getter != nil {
// 				ctx.handleStore.Delete(getterID)
// 				getterFunc.Free()
// 			}
// 			return err
// 		}
// 	}

// 	// Bind the property with getter/setter using JSAtom
// 	result := C.JS_DefinePropertyGetSet(
// 		ctx.ref,
// 		obj,
// 		propAtom,
// 		getterFunc.ref,
// 		setterFunc.ref,
// 		getPropertyConfigurable(),
// 	)

// 	if result < 0 {
// 		if prop.Getter != nil {
// 			ctx.handleStore.Delete(getterID)
// 			getterFunc.Free()
// 		}
// 		if prop.Setter != nil {
// 			ctx.handleStore.Delete(setterID)
// 			setterFunc.Free()
// 		}
// 		return errors.New("failed to bind property to object")
// 	}

// 	return nil
// }
