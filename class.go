package quickjs

/*
#include "bridge.h"
*/
import "C"
import (
	"errors"
	"unsafe"
)

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

// Use function calls for property flags instead of constants
func getPropertyWritableConfigurable() C.int {
	return C.int(C.GetPropertyWritableConfigurable())
}

func getPropertyConfigurable() C.int {
	return C.int(C.GetPropertyConfigurable())
}

// Optional cleanup interface for class instances
// Objects implementing this interface will have Finalize() called automatically
// when the JavaScript object is garbage collected
type ClassFinalizer interface {
	Finalize()
}

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

// ClassBuilder provides a fluent API for building JavaScript classes
// Uses builder pattern for easy and readable class definition
type ClassBuilder struct {
	name        string
	constructor ClassConstructorFunc
	methods     []MethodEntry
	properties  []PropertyEntry
}

// NewClass creates a new ClassBuilder with the specified name
// This is the entry point for building JavaScript classes
func NewClass(name string) *ClassBuilder {
	return &ClassBuilder{
		name:       name,
		methods:    make([]MethodEntry, 0),
		properties: make([]PropertyEntry, 0),
	}
}

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
// Returns the constructor function and classID for CreateInstanceFromNewTarget
func (cb *ClassBuilder) Build(ctx *Context) (Value, uint32, error) {
	return ctx.createClass(cb)
}

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

// createClass implements the core class creation logic
// This method follows the exact pattern from point.c example
func (ctx *Context) createClass(builder *ClassBuilder) (Value, uint32, error) {
	// Step 1: Validate input
	if err := validateClassBuilder(builder); err != nil {
		return Value{}, 0, err
	}

	// Step 2: Create class ID (corresponds to point.c: JS_NewClassID(&js_point_class_id))
	var classID C.JSClassID
	C.JS_NewClassID(&classID)

	// Check QuickJS limits
	if classID >= MaxClassID {
		return Value{}, uint32(classID), errors.New("class ID exceeds maximum value")
	}

	// Step 3: Register class definition (corresponds to point.c: JS_NewClass)
	className := C.CString(builder.name)
	defer C.free(unsafe.Pointer(className))

	classDef := C.JSClassDef{
		class_name: className,
		finalizer:  (*C.JSClassFinalizer)(unsafe.Pointer(C.GoClassFinalizerProxy)),
	}

	result := C.JS_NewClass(ctx.runtime.ref, classID, &classDef)
	if result != 0 {
		return Value{}, 0, errors.New("failed to create class definition")
	}

	// Step 4: Create prototype object (corresponds to point.c: point_proto = JS_NewObject(ctx))
	proto := C.JS_NewObject(ctx.ref)
	if C.JS_IsException(proto) != 0 {
		return Value{}, 0, errors.New("failed to create prototype object")
	}

	// Step 5: Bind instance methods and properties to prototype
	if err := ctx.bindMembersToObject(proto, builder.methods, builder.properties, false); err != nil {
		C.JS_FreeValue(ctx.ref, proto)
		return Value{}, 0, err
	}

	// Step 6: Create constructor function (corresponds to point.c: JS_NewCFunction2)
	constructor, constructorID, err := ctx.createCFunction(
		builder.name,
		builder.constructor,
		C.JSCFunctionEnum(C.GetCFuncConstructorMagic()),
		DefaultConstructorParams,
	)
	if err != nil {
		C.JS_FreeValue(ctx.ref, proto)
		return Value{}, 0, err
	}

	// Step 7: Associate constructor with prototype (corresponds to point.c: JS_SetConstructor)
	C.JS_SetConstructor(ctx.ref, constructor, proto)

	// Step 8: Set class prototype (corresponds to point.c: JS_SetClassProto)
	C.JS_SetClassProto(ctx.ref, classID, proto)

	// Step 9: Bind static methods and properties to constructor
	if err := ctx.bindMembersToObject(constructor, builder.methods, builder.properties, true); err != nil {
		ctx.handleStore.Delete(constructorID)
		C.JS_FreeValue(ctx.ref, constructor)
		return Value{}, 0, err
	}

	// Return constructor and classID
	return Value{ctx: ctx, ref: constructor}, uint32(classID), nil
}

// createCFunction creates a C function with the specified type and parameters
// This is a common helper that reduces code duplication
func (ctx *Context) createCFunction(name string, handler interface{}, funcType C.JSCFunctionEnum, length int) (C.JSValue, int32, error) {
	handlerID := ctx.handleStore.Store(handler)
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var proxy unsafe.Pointer
	constructorMagic := C.JSCFunctionEnum(C.GetCFuncConstructorMagic())
	genericMagic := C.JSCFunctionEnum(C.GetCFuncGenericMagic())
	getterMagic := C.JSCFunctionEnum(C.GetCFuncGetterMagic())
	setterMagic := C.JSCFunctionEnum(C.GetCFuncSetterMagic())

	switch funcType {
	case constructorMagic:
		proxy = unsafe.Pointer(C.GoClassConstructorProxy)
	case genericMagic:
		proxy = unsafe.Pointer(C.GoClassMethodProxy)
	case getterMagic:
		proxy = unsafe.Pointer(C.GoClassGetterProxy)
	case setterMagic:
		proxy = unsafe.Pointer(C.GoClassSetterProxy)
	default:
		ctx.handleStore.Delete(handlerID)
		return C.JS_NewUndefined(), 0, errors.New("unsupported function type")
	}

	jsFunc := C.JS_NewCFunction2(
		ctx.ref,
		(*C.JSCFunction)(proxy),
		cName,
		C.int(length),
		funcType,
		C.int(handlerID),
	)

	if C.JS_IsException(jsFunc) != 0 {
		ctx.handleStore.Delete(handlerID)
		return C.JS_NewUndefined(), 0, errors.New("failed to create function")
	}

	return jsFunc, handlerID, nil
}

// bindMembersToObject binds methods and properties to a JavaScript object
// isStatic determines whether to bind static or instance members
func (ctx *Context) bindMembersToObject(obj C.JSValue, methods []MethodEntry, properties []PropertyEntry, isStatic bool) error {
	// Bind methods
	for _, method := range methods {
		if method.Static == isStatic {
			if err := ctx.bindMethodToObject(obj, method); err != nil {
				return err
			}
		}
	}

	// Bind properties
	for _, prop := range properties {
		if prop.Static == isStatic {
			if err := ctx.bindPropertyToObject(obj, prop); err != nil {
				return err
			}
		}
	}

	return nil
}

// bindMethodToObject binds a method to a JavaScript object (prototype or constructor)
func (ctx *Context) bindMethodToObject(obj C.JSValue, method MethodEntry) error {
	length := method.Length
	if length < 0 {
		length = DefaultMethodParams
	}

	methodFunc, methodID, err := ctx.createCFunction(
		method.Name,
		method.Func,
		C.JSCFunctionEnum(C.GetCFuncGenericMagic()),
		length,
	)
	if err != nil {
		return err
	}

	methodName := C.CString(method.Name)
	defer C.free(unsafe.Pointer(methodName))

	result := C.JS_DefinePropertyValueStr(
		ctx.ref,
		obj,
		methodName,
		methodFunc,
		getPropertyWritableConfigurable(),
	)

	if result < 0 {
		ctx.handleStore.Delete(methodID)
		C.JS_FreeValue(ctx.ref, methodFunc)
		return errors.New("failed to bind method to object")
	}

	return nil
}

// bindPropertyToObject binds a property to a JavaScript object (prototype or constructor)
func (ctx *Context) bindPropertyToObject(obj C.JSValue, prop PropertyEntry) error {
	// Convert property name to JSAtom first
	propName := C.CString(prop.Name)
	defer C.free(unsafe.Pointer(propName))

	propAtom := C.JS_NewAtom(ctx.ref, propName)
	defer C.JS_FreeAtom(ctx.ref, propAtom)

	var getterFunc, setterFunc C.JSValue = C.JS_NewUndefined(), C.JS_NewUndefined()
	var getterID, setterID int32

	// Create getter function if provided
	if prop.Getter != nil {
		var err error
		getterFunc, getterID, err = ctx.createCFunction(
			"get "+prop.Name,
			prop.Getter,
			C.JSCFunctionEnum(C.GetCFuncGetterMagic()),
			DefaultGetterParams,
		)
		if err != nil {
			return err
		}
	}

	// Create setter function if provided
	if prop.Setter != nil {
		var err error
		setterFunc, setterID, err = ctx.createCFunction(
			"set "+prop.Name,
			prop.Setter,
			C.JSCFunctionEnum(C.GetCFuncSetterMagic()),
			DefaultSetterParams,
		)
		if err != nil {
			if prop.Getter != nil {
				ctx.handleStore.Delete(getterID)
				C.JS_FreeValue(ctx.ref, getterFunc)
			}
			return err
		}
	}

	// Bind the property with getter/setter using JSAtom
	result := C.JS_DefinePropertyGetSet(
		ctx.ref,
		obj,
		propAtom,
		getterFunc,
		setterFunc,
		getPropertyConfigurable(),
	)

	if result < 0 {
		if prop.Getter != nil {
			ctx.handleStore.Delete(getterID)
			C.JS_FreeValue(ctx.ref, getterFunc)
		}
		if prop.Setter != nil {
			ctx.handleStore.Delete(setterID)
			C.JS_FreeValue(ctx.ref, setterFunc)
		}
		return errors.New("failed to bind property to object")
	}

	return nil
}
