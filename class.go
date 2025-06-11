package quickjs

/*
#include "bridge.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

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
	return cb.MethodWithLength(name, fn, -1) // Auto-detect parameter count
}

// StaticMethod adds a static method to the class
// Static methods are called on the constructor function itself
func (cb *ClassBuilder) StaticMethod(name string, fn ClassMethodFunc) *ClassBuilder {
	return cb.StaticMethodWithLength(name, fn, -1) // Auto-detect parameter count
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

// createClass implements the core class creation logic
// This method follows the exact pattern from point.c example
func (ctx *Context) createClass(builder *ClassBuilder) (Value, uint32, error) {
	if builder.name == "" {
		return Value{}, 0, errors.New("class name cannot be empty")
	}

	if builder.constructor == nil {
		return Value{}, 0, errors.New("constructor function is required")
	}

	// Step 1: Create class ID (corresponds to point.c: JS_NewClassID(&js_point_class_id))
	// Fix: Pass pointer to classID variable, not nil
	var classID C.JSClassID
	C.JS_NewClassID(&classID)

	// QuickJS hard limit: (1 << 16)
	if classID >= (1 << 16) {
		return Value{}, uint32(classID), errors.New("class ID exceeds maximum value (65535)")
	}

	// Step 2: Register class definition (corresponds to point.c: JS_NewClass)
	className := C.CString(builder.name)
	defer C.free(unsafe.Pointer(className))

	classDef := C.JSClassDef{
		class_name: className,
		finalizer:  (*C.JSClassFinalizer)(unsafe.Pointer(C.GoClassFinalizerProxy)),
	}

	result := C.JS_NewClass(ctx.runtime.ref, classID, &classDef)
	if result != 0 {
		if ctx.HasException() {
			fmt.Printf("Error creating class sssssssssss")
		}
		// C.JS_FreeCString(ctx.ref, className)
		return Value{}, 0, errors.New("failed to create class definition")
	}

	// Step 3: Create prototype object (corresponds to point.c: point_proto = JS_NewObject(ctx))
	proto := C.JS_NewObject(ctx.ref)
	if C.JS_IsException(proto) != 0 {
		return Value{}, 0, errors.New("failed to create prototype object")
	}

	// Step 4: Bind instance methods and properties to prototype
	for _, method := range builder.methods {
		if !method.Static {
			if err := ctx.bindMethodToObject(proto, method); err != nil {
				C.JS_FreeValue(ctx.ref, proto)
				return Value{}, 0, err
			}
		}
	}

	for _, prop := range builder.properties {
		if !prop.Static {
			if err := ctx.bindPropertyToObject(proto, prop); err != nil {
				C.JS_FreeValue(ctx.ref, proto)
				return Value{}, 0, err
			}
		}
	}

	// Step 5: Create constructor function (corresponds to point.c: JS_NewCFunction2)
	constructorID := ctx.handleStore.Store(builder.constructor)
	constructorName := C.CString(builder.name)
	defer C.free(unsafe.Pointer(constructorName))

	constructor := C.JS_NewCFunction2(
		ctx.ref,
		(*C.JSCFunction)(unsafe.Pointer(C.GoClassConstructorProxy)),
		constructorName,
		C.int(2), // Default expected parameter count
		C.JS_CFUNC_constructor_magic,
		C.int(constructorID),
	)

	if C.JS_IsException(constructor) != 0 {
		ctx.handleStore.Delete(constructorID)
		C.JS_FreeValue(ctx.ref, proto)
		return Value{}, 0, errors.New("failed to create constructor function")
	}

	// Step 6: Associate constructor with prototype (corresponds to point.c: JS_SetConstructor)
	C.JS_SetConstructor(ctx.ref, constructor, proto)

	// Step 7: Set class prototype (corresponds to point.c: JS_SetClassProto)
	C.JS_SetClassProto(ctx.ref, classID, proto)

	// Step 8: Bind static methods and properties to constructor
	for _, method := range builder.methods {
		if method.Static {
			if err := ctx.bindMethodToObject(constructor, method); err != nil {
				ctx.handleStore.Delete(constructorID)
				C.JS_FreeValue(ctx.ref, constructor)
				return Value{}, 0, err
			}
		}
	}

	for _, prop := range builder.properties {
		if prop.Static {
			if err := ctx.bindPropertyToObject(constructor, prop); err != nil {
				ctx.handleStore.Delete(constructorID)
				C.JS_FreeValue(ctx.ref, constructor)
				return Value{}, 0, err
			}
		}
	}

	// Return constructor and classID
	return Value{ctx: ctx, ref: constructor}, uint32(classID), nil
}

// bindMethodToObject binds a method to a JavaScript object (prototype or constructor)
func (ctx *Context) bindMethodToObject(obj C.JSValue, method MethodEntry) error {
	methodID := ctx.handleStore.Store(method.Func)
	methodName := C.CString(method.Name)
	defer C.free(unsafe.Pointer(methodName))

	length := method.Length
	if length < 0 {
		length = 0 // Default to 0 if not specified
	}

	methodFunc := C.JS_NewCFunction2(
		ctx.ref,
		(*C.JSCFunction)(unsafe.Pointer(C.GoClassMethodProxy)),
		methodName,
		C.int(length),
		C.JS_CFUNC_generic_magic,
		C.int(methodID),
	)

	if C.JS_IsException(methodFunc) != 0 {
		ctx.handleStore.Delete(methodID)
		return errors.New("failed to create method function")
	}

	result := C.JS_DefinePropertyValueStr(
		ctx.ref,
		obj,
		methodName,
		methodFunc,
		C.JS_PROP_WRITABLE|C.JS_PROP_CONFIGURABLE,
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

	var getterFunc, setterFunc C.JSValue = C.JS_UNDEFINED, C.JS_UNDEFINED

	// Create getter function if provided
	if prop.Getter != nil {
		getterID := ctx.handleStore.Store(prop.Getter)
		getterName := C.CString("get " + prop.Name)
		defer C.free(unsafe.Pointer(getterName))

		getterFunc = C.JS_NewCFunction2(
			ctx.ref,
			(*C.JSCFunction)(unsafe.Pointer(C.GoClassGetterProxy)),
			getterName,
			C.int(0),
			C.JS_CFUNC_getter_magic,
			C.int(getterID),
		)

		if C.JS_IsException(getterFunc) != 0 {
			ctx.handleStore.Delete(getterID)
			return errors.New("failed to create getter function")
		}
	}

	// Create setter function if provided
	if prop.Setter != nil {
		setterID := ctx.handleStore.Store(prop.Setter)
		setterName := C.CString("set " + prop.Name)
		defer C.free(unsafe.Pointer(setterName))

		setterFunc = C.JS_NewCFunction2(
			ctx.ref,
			(*C.JSCFunction)(unsafe.Pointer(C.GoClassSetterProxy)),
			setterName,
			C.int(1),
			C.JS_CFUNC_setter_magic,
			C.int(setterID),
		)

		if C.JS_IsException(setterFunc) != 0 {
			ctx.handleStore.Delete(setterID)
			if C.JS_IsUndefined(getterFunc) == 0 { // 0 means not undefined
				C.JS_FreeValue(ctx.ref, getterFunc)
			}
			return errors.New("failed to create setter function")
		}
	}

	// Bind the property with getter/setter using JSAtom
	result := C.JS_DefinePropertyGetSet(
		ctx.ref,
		obj,
		propAtom,
		getterFunc,
		setterFunc,
		C.JS_PROP_CONFIGURABLE,
	)

	if result < 0 {
		if C.JS_IsUndefined(getterFunc) == 0 { // 0 means not undefined
			C.JS_FreeValue(ctx.ref, getterFunc)
		}
		if C.JS_IsUndefined(setterFunc) == 0 { // 0 means not undefined
			C.JS_FreeValue(ctx.ref, setterFunc)
		}
		return errors.New("failed to bind property to object")
	}

	return nil
}
